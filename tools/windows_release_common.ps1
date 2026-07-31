$ErrorActionPreference = 'Stop'

function Get-GBFRSha256Hex {
    param([Parameter(Mandatory = $true)][string]$LiteralPath)

    $stream = [System.IO.File]::OpenRead($LiteralPath)
    $sha256 = [System.Security.Cryptography.SHA256]::Create()
    try {
        return ([System.BitConverter]::ToString($sha256.ComputeHash($stream))).Replace('-', '')
    }
    finally {
        $sha256.Dispose()
        $stream.Dispose()
    }
}

function Get-GBFRPathKey {
    param([Parameter(Mandatory = $true)][string]$LiteralPath)

    $normalized = [System.IO.Path]::GetFullPath($LiteralPath).TrimEnd(
        [System.IO.Path]::DirectorySeparatorChar,
        [System.IO.Path]::AltDirectorySeparatorChar
    ).ToUpperInvariant()
    $bytes = [System.Text.Encoding]::UTF8.GetBytes($normalized)
    $sha256 = [System.Security.Cryptography.SHA256]::Create()
    try {
        $digest = ([System.BitConverter]::ToString($sha256.ComputeHash($bytes))).Replace('-', '').ToLowerInvariant()
        return $digest.Substring(0, 24)
    }
    finally {
        $sha256.Dispose()
    }
}

function Get-GBFRRepositoryLockPath {
    param(
        [Parameter(Mandatory = $true)][string]$RepositoryRoot,
        [Parameter(Mandatory = $true)][ValidatePattern('^[a-z0-9-]+$')][string]$Scope
    )

    $key = Get-GBFRPathKey -LiteralPath $RepositoryRoot
    return Join-Path ([System.IO.Path]::GetTempPath()) "gbfr-$Scope-$key.lock"
}

function Enter-GBFRExclusiveLease {
    param(
        [Parameter(Mandatory = $true)][string]$LockPath,
        [Parameter(Mandatory = $true)][string]$Description
    )

    $parent = Split-Path -Parent $LockPath
    if (-not [string]::IsNullOrWhiteSpace($parent)) {
        [System.IO.Directory]::CreateDirectory($parent) | Out-Null
    }
    try {
        $stream = [System.IO.FileStream]::new(
            $LockPath,
            [System.IO.FileMode]::OpenOrCreate,
            [System.IO.FileAccess]::ReadWrite,
            [System.IO.FileShare]::None,
            4096,
            [System.IO.FileOptions]::DeleteOnClose
        )
    }
    catch [System.IO.IOException] {
        throw "Another $Description is already running. Wait for it to finish before starting another one."
    }

    try {
        $stream.SetLength(0)
        $payload = [System.Text.Encoding]::UTF8.GetBytes(
            "pid=$PID`r`nstarted=$([DateTime]::UtcNow.ToString('o'))`r`n"
        )
        $stream.Write($payload, 0, $payload.Length)
        $stream.Flush($true)
        return $stream
    }
    catch {
        $stream.Dispose()
        throw
    }
}

function Exit-GBFRExclusiveLease {
    param([System.IO.FileStream]$Lease)

    if ($null -ne $Lease) {
        $Lease.Dispose()
    }
}

function Remove-GBFRStaleNamedPaths {
    param(
        [Parameter(Mandatory = $true)][string]$Directory,
        [Parameter(Mandatory = $true)][string]$ExactNamePattern,
        [Parameter(Mandatory = $true)][ValidateSet('File', 'Directory')][string]$Kind
    )

    if (-not (Test-Path -LiteralPath $Directory -PathType Container)) {
        return
    }
    foreach ($entry in @(Get-ChildItem -LiteralPath $Directory -Force)) {
        if ($entry.Name -notmatch $ExactNamePattern) {
            continue
        }
        if ($Kind -eq 'File' -and $entry.PSIsContainer) {
            continue
        }
        if ($Kind -eq 'Directory' -and -not $entry.PSIsContainer) {
            continue
        }
        if ($entry.PSIsContainer) {
            Remove-Item -LiteralPath $entry.FullName -Recurse -Force
        }
        else {
            Remove-Item -LiteralPath $entry.FullName -Force
        }
    }
}

if (-not ([System.Management.Automation.PSTypeName]'GBFRAtomicFilePublisher').Type) {
    Add-Type -TypeDefinition @'
using System;
using System.ComponentModel;
using System.Runtime.InteropServices;

public static class GBFRAtomicFilePublisher
{
    private const int MoveFileReplaceExisting = 0x1;
    private const int MoveFileWriteThrough = 0x8;

    [DllImport("kernel32.dll", CharSet = CharSet.Unicode, SetLastError = true)]
    private static extern bool MoveFileEx(string existingFile, string destinationFile, int flags);

    public static void Replace(string source, string destination)
    {
        if (!MoveFileEx(source, destination, MoveFileReplaceExisting | MoveFileWriteThrough))
        {
            throw new Win32Exception(Marshal.GetLastWin32Error());
        }
    }

    public static void Move(string source, string destination)
    {
        if (!MoveFileEx(source, destination, MoveFileWriteThrough))
        {
            throw new Win32Exception(Marshal.GetLastWin32Error());
        }
    }
}
'@
}

function Copy-GBFRVerifiedFile {
    param(
        [Parameter(Mandatory = $true)][string]$Source,
        [Parameter(Mandatory = $true)][string]$Destination,
        [string]$ExpectedHash = ''
    )

    Copy-Item -LiteralPath $Source -Destination $Destination
    $sourceHash = if ([string]::IsNullOrWhiteSpace($ExpectedHash)) {
        Get-GBFRSha256Hex -LiteralPath $Source
    }
    else {
        $ExpectedHash
    }
    $destinationHash = Get-GBFRSha256Hex -LiteralPath $Destination
    if ($sourceHash -ne $destinationHash) {
        throw "Verified copy hash mismatch: '$Source' -> '$Destination'."
    }
    return $sourceHash
}

function Restore-GBFRFileFromVerifiedBackup {
    param(
        [Parameter(Mandatory = $true)][string]$BackupPath,
        [Parameter(Mandatory = $true)][string]$DestinationPath,
        [Parameter(Mandatory = $true)][string]$ExpectedHash
    )

    if (-not (Test-Path -LiteralPath $BackupPath -PathType Leaf)) {
        throw "Rollback backup is missing: $BackupPath"
    }
    if ((Get-GBFRSha256Hex -LiteralPath $BackupPath) -ne $ExpectedHash) {
        throw "Rollback backup failed integrity verification: $BackupPath"
    }

    $destinationDirectory = Split-Path -Parent $DestinationPath
    [System.IO.Directory]::CreateDirectory($destinationDirectory) | Out-Null
    $restoreTemporary = Join-Path $destinationDirectory (
        '.patch_core-' + [guid]::NewGuid().ToString('N') + '.tmp'
    )
    try {
        Copy-GBFRVerifiedFile -Source $BackupPath -Destination $restoreTemporary -ExpectedHash $ExpectedHash | Out-Null
        [GBFRAtomicFilePublisher]::Replace($restoreTemporary, $DestinationPath)
        if ((Get-GBFRSha256Hex -LiteralPath $DestinationPath) -ne $ExpectedHash) {
            throw "Atomically restored file failed read-back verification: $DestinationPath"
        }
    }
    finally {
        if (Test-Path -LiteralPath $restoreTemporary) {
            Remove-Item -LiteralPath $restoreTemporary -Force
        }
    }
}

function Remove-GBFRPublishedFileForRollback {
    param([Parameter(Mandatory = $true)][string]$DestinationPath)

    if (-not (Test-Path -LiteralPath $DestinationPath -PathType Leaf)) {
        return
    }
    $destinationDirectory = Split-Path -Parent $DestinationPath
    $quarantine = Join-Path $destinationDirectory (
        '.patch_core-' + [guid]::NewGuid().ToString('N') + '.tmp'
    )
    [GBFRAtomicFilePublisher]::Move($DestinationPath, $quarantine)
    if (Test-Path -LiteralPath $DestinationPath) {
        throw "Rollback could not remove the newly published file: $DestinationPath"
    }
    try {
        Remove-Item -LiteralPath $quarantine -Force
    }
    catch {
        Write-Warning "Rollback removed the published file, but its quarantined temporary remains for the next safe cleanup: $quarantine"
    }
}

function Get-GBFRPatchCoreRecoveryPaths {
    param([Parameter(Mandatory = $true)][string]$RepositoryRoot)

    $key = Get-GBFRPathKey -LiteralPath $RepositoryRoot
    $prefix = "gbfr-windows-build-$key-patch-core"
    return [pscustomobject]@{
        Key = $key
        Prefix = $prefix
        JournalPath = Join-Path ([System.IO.Path]::GetTempPath()) "$prefix.recovery.json"
        BackupNamePattern = '^' + [regex]::Escape($prefix) + '-[0-9a-fA-F]{32}\.backup$'
        ClearedNamePattern = '^' + [regex]::Escape($prefix) + '\.cleared-[0-9a-fA-F]{32}\.tmp$'
        JournalTemporaryNamePattern = '^' + [regex]::Escape($prefix) + '\.journal-[0-9a-fA-F]{32}\.tmp$'
    }
}

function Assert-GBFRBuildLeaseHeld {
    param(
        [Parameter(Mandatory = $true)][System.IO.FileStream]$Lease,
        [Parameter(Mandatory = $true)][string]$ExpectedLockPath
    )

    if ($Lease.SafeFileHandle.IsClosed -or
        -not $Lease.Name.Equals(
            [System.IO.Path]::GetFullPath($ExpectedLockPath),
            [System.StringComparison]::OrdinalIgnoreCase
        )) {
        throw 'Patch-core recovery requires the current repository build lease.'
    }
}

function Sync-GBFRFile {
    param([Parameter(Mandatory = $true)][string]$LiteralPath)

    $stream = [System.IO.FileStream]::new(
        $LiteralPath,
        [System.IO.FileMode]::Open,
        [System.IO.FileAccess]::ReadWrite,
        [System.IO.FileShare]::Read
    )
    try {
        $stream.Flush($true)
    }
    finally {
        $stream.Dispose()
    }
}

function Write-GBFRDurableJsonFileAtomic {
    param(
        [Parameter(Mandatory = $true)][string]$LiteralPath,
        [Parameter(Mandatory = $true)]$Value,
        [Parameter(Mandatory = $true)][string]$TemporaryPrefix
    )

    $directory = Split-Path -Parent $LiteralPath
    [System.IO.Directory]::CreateDirectory($directory) | Out-Null
    $temporary = Join-Path $directory (
        $TemporaryPrefix + [guid]::NewGuid().ToString('N') + '.tmp'
    )
    $bytes = [System.Text.UTF8Encoding]::new($false).GetBytes(
        ($Value | ConvertTo-Json -Depth 6)
    )
    $stream = $null
    try {
        $stream = [System.IO.FileStream]::new(
            $temporary,
            [System.IO.FileMode]::CreateNew,
            [System.IO.FileAccess]::Write,
            [System.IO.FileShare]::None
        )
        $stream.Write($bytes, 0, $bytes.Length)
        $stream.Flush($true)
        $stream.Dispose()
        $stream = $null
        [GBFRAtomicFilePublisher]::Replace($temporary, $LiteralPath)
    }
    finally {
        if ($null -ne $stream) {
            $stream.Dispose()
        }
        if (Test-Path -LiteralPath $temporary) {
            Remove-Item -LiteralPath $temporary -Force
        }
    }
}

function Test-GBFRProcessIdentityAlive {
    param(
        [Parameter(Mandatory = $true)][int]$OwnerPid,
        [Parameter(Mandatory = $true)][string]$OwnerStartedUtc
    )

    try {
        $process = [System.Diagnostics.Process]::GetProcessById($OwnerPid)
        return $process.StartTime.ToUniversalTime().ToString('o') -eq $OwnerStartedUtc
    }
    catch {
        return $false
    }
}

function Assert-GBFRPatchCoreRecoveryTransaction {
    param(
        [Parameter(Mandatory = $true)]$Transaction,
        [Parameter(Mandatory = $true)][string]$RepositoryRoot,
        [Parameter(Mandatory = $true)][string]$DestinationPath
    )

    $expectedRepository = [System.IO.Path]::GetFullPath($RepositoryRoot)
    $expectedDestination = [System.IO.Path]::GetFullPath($DestinationPath)
    if ([int]$Transaction.schemaVersion -ne 1) {
        throw 'Patch-core recovery journal has an unsupported schema.'
    }
    if (-not ([string]$Transaction.repositoryRoot).Equals(
        $expectedRepository,
        [System.StringComparison]::OrdinalIgnoreCase
    )) {
        throw 'Patch-core recovery journal belongs to another repository.'
    }
    if (-not ([string]$Transaction.destinationPath).Equals(
        $expectedDestination,
        [System.StringComparison]::OrdinalIgnoreCase
    )) {
        throw 'Patch-core recovery journal targets an unexpected destination.'
    }
    if ([int]$Transaction.ownerPid -le 0 -or [string]::IsNullOrWhiteSpace([string]$Transaction.ownerStartedUtc)) {
        throw 'Patch-core recovery journal has no valid owner identity.'
    }

    if ([bool]$Transaction.originalExisted) {
        if ([string]::IsNullOrWhiteSpace([string]$Transaction.originalHash) -or
            [string]::IsNullOrWhiteSpace([string]$Transaction.backupPath)) {
            throw 'Patch-core recovery journal is missing its verified original.'
        }
        $paths = Get-GBFRPatchCoreRecoveryPaths -RepositoryRoot $RepositoryRoot
        $backupPath = [System.IO.Path]::GetFullPath([string]$Transaction.backupPath)
        $temporaryRoot = [System.IO.Path]::GetFullPath([System.IO.Path]::GetTempPath())
        if (-not (Split-Path -Parent $backupPath).Equals(
            $temporaryRoot.TrimEnd(
                [System.IO.Path]::DirectorySeparatorChar,
                [System.IO.Path]::AltDirectorySeparatorChar
            ),
            [System.StringComparison]::OrdinalIgnoreCase
        ) -or (Split-Path $backupPath -Leaf) -notmatch $paths.BackupNamePattern) {
            throw 'Patch-core recovery journal references an untrusted backup path.'
        }
    }
}

function Start-GBFRPatchCoreRecoveryTransaction {
    param(
        [Parameter(Mandatory = $true)][string]$RepositoryRoot,
        [Parameter(Mandatory = $true)][string]$DestinationPath,
        [Parameter(Mandatory = $true)][System.IO.FileStream]$Lease,
        [Parameter(Mandatory = $true)][string]$LockPath
    )

    Assert-GBFRBuildLeaseHeld -Lease $Lease -ExpectedLockPath $LockPath
    $paths = Get-GBFRPatchCoreRecoveryPaths -RepositoryRoot $RepositoryRoot
    if (Test-Path -LiteralPath $paths.JournalPath) {
        throw "A pending patch-core recovery journal must be replayed first: $($paths.JournalPath)"
    }

    $destination = [System.IO.Path]::GetFullPath($DestinationPath)
    $originalExisted = Test-Path -LiteralPath $destination -PathType Leaf
    $originalHash = ''
    $backupPath = ''
    if ($originalExisted) {
        $originalHash = Get-GBFRSha256Hex -LiteralPath $destination
        $backupPath = Join-Path ([System.IO.Path]::GetTempPath()) (
            $paths.Prefix + '-' + [guid]::NewGuid().ToString('N') + '.backup'
        )
        Copy-GBFRVerifiedFile `
            -Source $destination `
            -Destination $backupPath `
            -ExpectedHash $originalHash | Out-Null
        Sync-GBFRFile -LiteralPath $backupPath
        if ((Get-GBFRSha256Hex -LiteralPath $backupPath) -ne $originalHash) {
            Remove-Item -LiteralPath $backupPath -Force
            throw 'Durable patch-core rollback backup failed read-back verification.'
        }
    }

    $ownerProcess = [System.Diagnostics.Process]::GetCurrentProcess()
    $transaction = [ordered]@{
        schemaVersion = 1
        repositoryRoot = [System.IO.Path]::GetFullPath($RepositoryRoot)
        destinationPath = $destination
        originalExisted = $originalExisted
        originalHash = $originalHash
        backupPath = $backupPath
        ownerPid = $PID
        ownerStartedUtc = $ownerProcess.StartTime.ToUniversalTime().ToString('o')
        createdAtUtc = [DateTime]::UtcNow.ToString('o')
    }
    try {
        Write-GBFRDurableJsonFileAtomic `
            -LiteralPath $paths.JournalPath `
            -Value $transaction `
            -TemporaryPrefix ($paths.Prefix + '.journal-')
    }
    catch {
        if (-not [string]::IsNullOrWhiteSpace($backupPath) -and (Test-Path -LiteralPath $backupPath)) {
            Remove-Item -LiteralPath $backupPath -Force
        }
        throw
    }
    return [pscustomobject]$transaction
}

function Complete-GBFRPatchCoreRecoveryTransaction {
    param(
        [Parameter(Mandatory = $true)][string]$RepositoryRoot,
        [Parameter(Mandatory = $true)]$Transaction,
        [Parameter(Mandatory = $true)][System.IO.FileStream]$Lease,
        [Parameter(Mandatory = $true)][string]$LockPath
    )

    Assert-GBFRBuildLeaseHeld -Lease $Lease -ExpectedLockPath $LockPath
    $paths = Get-GBFRPatchCoreRecoveryPaths -RepositoryRoot $RepositoryRoot
    $clearedJournal = ''
    if (Test-Path -LiteralPath $paths.JournalPath -PathType Leaf) {
        $clearedJournal = Join-Path ([System.IO.Path]::GetTempPath()) (
            $paths.Prefix + '.cleared-' + [guid]::NewGuid().ToString('N') + '.tmp'
        )
        [GBFRAtomicFilePublisher]::Move($paths.JournalPath, $clearedJournal)
    }

    $backupPath = [string]$Transaction.backupPath
    if (-not [string]::IsNullOrWhiteSpace($backupPath) -and (Test-Path -LiteralPath $backupPath)) {
        try {
            Remove-Item -LiteralPath $backupPath -Force
        }
        catch {
            Write-Warning "The completed recovery backup remains for the next safe cleanup: $backupPath"
        }
    }
    if (-not [string]::IsNullOrWhiteSpace($clearedJournal) -and (Test-Path -LiteralPath $clearedJournal)) {
        try {
            Remove-Item -LiteralPath $clearedJournal -Force
        }
        catch {
            Write-Warning "The cleared recovery journal remains for the next safe cleanup: $clearedJournal"
        }
    }
}

function Rollback-GBFRPatchCoreRecoveryTransaction {
    param(
        [Parameter(Mandatory = $true)][string]$RepositoryRoot,
        [Parameter(Mandatory = $true)]$Transaction,
        [Parameter(Mandatory = $true)][System.IO.FileStream]$Lease,
        [Parameter(Mandatory = $true)][string]$LockPath
    )

    Assert-GBFRBuildLeaseHeld -Lease $Lease -ExpectedLockPath $LockPath
    $destination = [string]$Transaction.destinationPath
    if ([bool]$Transaction.originalExisted) {
        Restore-GBFRFileFromVerifiedBackup `
            -BackupPath ([string]$Transaction.backupPath) `
            -DestinationPath $destination `
            -ExpectedHash ([string]$Transaction.originalHash)
    }
    else {
        Remove-GBFRPublishedFileForRollback -DestinationPath $destination
    }
    Complete-GBFRPatchCoreRecoveryTransaction `
        -RepositoryRoot $RepositoryRoot `
        -Transaction $Transaction `
        -Lease $Lease `
        -LockPath $LockPath
}

function Remove-GBFROrphanedPatchCoreRecoveryArtifacts {
    param(
        [Parameter(Mandatory = $true)][string]$RepositoryRoot,
        [Parameter(Mandatory = $true)][System.IO.FileStream]$Lease,
        [Parameter(Mandatory = $true)][string]$LockPath
    )

    Assert-GBFRBuildLeaseHeld -Lease $Lease -ExpectedLockPath $LockPath
    $paths = Get-GBFRPatchCoreRecoveryPaths -RepositoryRoot $RepositoryRoot
    $temporaryRoot = [System.IO.Path]::GetTempPath()
    Remove-GBFRStaleNamedPaths -Directory $temporaryRoot -ExactNamePattern $paths.BackupNamePattern -Kind File
    Remove-GBFRStaleNamedPaths -Directory $temporaryRoot -ExactNamePattern $paths.ClearedNamePattern -Kind File
    Remove-GBFRStaleNamedPaths -Directory $temporaryRoot -ExactNamePattern $paths.JournalTemporaryNamePattern -Kind File
}

function Invoke-GBFRPendingPatchCoreRecovery {
    param(
        [Parameter(Mandatory = $true)][string]$RepositoryRoot,
        [Parameter(Mandatory = $true)][string]$DestinationPath,
        [Parameter(Mandatory = $true)][System.IO.FileStream]$Lease,
        [Parameter(Mandatory = $true)][string]$LockPath
    )

    Assert-GBFRBuildLeaseHeld -Lease $Lease -ExpectedLockPath $LockPath
    $paths = Get-GBFRPatchCoreRecoveryPaths -RepositoryRoot $RepositoryRoot
    if (-not (Test-Path -LiteralPath $paths.JournalPath -PathType Leaf)) {
        Remove-GBFROrphanedPatchCoreRecoveryArtifacts `
            -RepositoryRoot $RepositoryRoot `
            -Lease $Lease `
            -LockPath $LockPath
        return $false
    }

    try {
        $transaction = Get-Content -LiteralPath $paths.JournalPath -Raw | ConvertFrom-Json
    }
    catch {
        throw "Patch-core recovery journal is unreadable and was preserved: $($paths.JournalPath)"
    }
    Assert-GBFRPatchCoreRecoveryTransaction `
        -Transaction $transaction `
        -RepositoryRoot $RepositoryRoot `
        -DestinationPath $DestinationPath

    $oldOwnerAlive = Test-GBFRProcessIdentityAlive `
        -OwnerPid ([int]$transaction.ownerPid) `
        -OwnerStartedUtc ([string]$transaction.ownerStartedUtc)
    if ($oldOwnerAlive -and [int]$transaction.ownerPid -ne $PID) {
        Write-Warning 'The previous host process still exists, but the current exclusive build lease proves that its build transaction no longer owns publication.'
    }

    Rollback-GBFRPatchCoreRecoveryTransaction `
        -RepositoryRoot $RepositoryRoot `
        -Transaction $transaction `
        -Lease $Lease `
        -LockPath $LockPath
    Remove-GBFROrphanedPatchCoreRecoveryArtifacts `
        -RepositoryRoot $RepositoryRoot `
        -Lease $Lease `
        -LockPath $LockPath
    Write-Warning 'Recovered the embedded patch_core.dll from an interrupted previous build.'
    return $true
}
