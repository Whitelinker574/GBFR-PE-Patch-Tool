$ErrorActionPreference = 'Stop'

. (Join-Path $PSScriptRoot 'windows_release_common.ps1')

function Assert-GBFR {
    param(
        [Parameter(Mandatory = $true)][bool]$Condition,
        [Parameter(Mandatory = $true)][string]$Message
    )

    if (-not $Condition) {
        throw $Message
    }
}

$testRoot = Join-Path ([System.IO.Path]::GetTempPath()) (
    'gbfr-windows-release-safety-' + [guid]::NewGuid().ToString('N')
)
[System.IO.Directory]::CreateDirectory($testRoot) | Out-Null
try {
    $lockPath = Join-Path $testRoot 'exclusive.lock'
    $firstLease = Enter-GBFRExclusiveLease -LockPath $lockPath -Description 'test build'
    try {
        $secondRejected = $false
        try {
            $secondLease = Enter-GBFRExclusiveLease -LockPath $lockPath -Description 'test build'
            Exit-GBFRExclusiveLease -Lease $secondLease
        }
        catch {
            $secondRejected = $_.Exception.Message -like 'Another test build is already running*'
        }
        Assert-GBFR -Condition $secondRejected -Message 'A concurrent build lease was not rejected.'
    }
    finally {
        Exit-GBFRExclusiveLease -Lease $firstLease
    }
    $replacementLease = Enter-GBFRExclusiveLease -LockPath $lockPath -Description 'test build'
    Exit-GBFRExclusiveLease -Lease $replacementLease

    $repositoryRoot = [System.IO.Path]::GetFullPath((Join-Path $PSScriptRoot '..'))
    $buildLockPath = Get-GBFRRepositoryLockPath -RepositoryRoot $repositoryRoot -Scope 'windows-build'
    $buildLease = Enter-GBFRExclusiveLease -LockPath $buildLockPath -Description 'test production build'
    try {
        & (Join-Path $PSScriptRoot 'build_patch_core.ps1') `
            -Configuration Release `
            -DependenciesOnly `
            -InheritedBuildLease $buildLease `
            -InheritedBuildLockPath $buildLockPath

        $nativeBuildRejected = $false
        try {
            & (Join-Path $PSScriptRoot 'build_patch_core.ps1') -Configuration Release -DependenciesOnly
        }
        catch {
            $nativeBuildRejected = $_.Exception.Message -like 'Another Windows production or native build is already running*'
        }
        Assert-GBFR -Condition $nativeBuildRejected -Message 'The native build entry point bypassed the repository build lease.'

        $concurrentPackageRejected = $false
        try {
            & (Join-Path $PSScriptRoot 'package_windows_release.ps1') `
                -Version 'v2.0.3' `
                -OutputDirectory (Join-Path $testRoot 'concurrent-package')
        }
        catch {
            $concurrentPackageRejected = $_.Exception.Message -like 'Another Windows production, native, or release build is already running*'
        }
        Assert-GBFR -Condition $concurrentPackageRejected -Message 'The release packager did not share the repository build gate.'
    }
    finally {
        Exit-GBFRExclusiveLease -Lease $buildLease
    }

    $recoveryRepository = Join-Path $testRoot 'interrupted-build-repository'
    $recoveryDestination = Join-Path $recoveryRepository 'internal\backend\resources\patch_core.dll'
    [System.IO.Directory]::CreateDirectory((Split-Path -Parent $recoveryDestination)) | Out-Null
    [System.IO.File]::WriteAllText($recoveryDestination, 'verified original native bytes')
    $recoveryOriginalHash = Get-GBFRSha256Hex -LiteralPath $recoveryDestination
    $recoveryLockPath = Get-GBFRRepositoryLockPath -RepositoryRoot $recoveryRepository -Scope 'windows-build'
    $interruptedLease = Enter-GBFRExclusiveLease -LockPath $recoveryLockPath -Description 'interrupted test build'
    $interruptedTransaction = Start-GBFRPatchCoreRecoveryTransaction `
        -RepositoryRoot $recoveryRepository `
        -DestinationPath $recoveryDestination `
        -Lease $interruptedLease `
        -LockPath $recoveryLockPath
    $replacement = Join-Path (Split-Path -Parent $recoveryDestination) '.patch_core-0123456789abcdef0123456789abcdef.tmp'
    [System.IO.File]::WriteAllText($replacement, 'interrupted replacement native bytes')
    [GBFRAtomicFilePublisher]::Replace($replacement, $recoveryDestination)
    Exit-GBFRExclusiveLease -Lease $interruptedLease

    $recoveryPaths = Get-GBFRPatchCoreRecoveryPaths -RepositoryRoot $recoveryRepository
    Assert-GBFR -Condition (Test-Path -LiteralPath $recoveryPaths.JournalPath) -Message 'Interrupted publication did not leave a durable recovery journal.'
    Assert-GBFR -Condition ((Get-GBFRSha256Hex -LiteralPath $recoveryDestination) -ne $recoveryOriginalHash) -Message 'Interrupted publication did not replace the formal file for the recovery probe.'

    $nextBuildLease = Enter-GBFRExclusiveLease -LockPath $recoveryLockPath -Description 'next test build'
    try {
        $recovered = Invoke-GBFRPendingPatchCoreRecovery `
            -RepositoryRoot $recoveryRepository `
            -DestinationPath $recoveryDestination `
            -Lease $nextBuildLease `
            -LockPath $recoveryLockPath
        Assert-GBFR -Condition $recovered -Message 'The next build did not replay an interrupted recovery journal.'
    }
    finally {
        Exit-GBFRExclusiveLease -Lease $nextBuildLease
    }
    Assert-GBFR -Condition (Test-Path -LiteralPath $recoveryDestination) -Message 'Interrupted-build recovery deleted the formal patch_core.dll.'
    Assert-GBFR `
        -Condition ((Get-GBFRSha256Hex -LiteralPath $recoveryDestination) -eq $recoveryOriginalHash) `
        -Message 'Interrupted-build recovery did not restore the original formal DLL.'
    Assert-GBFR -Condition (-not (Test-Path -LiteralPath $recoveryPaths.JournalPath)) -Message 'Completed interrupted-build recovery retained its live journal.'

    $embeddedDirectory = Join-Path $testRoot 'embedded'
    [System.IO.Directory]::CreateDirectory($embeddedDirectory) | Out-Null
    $stalePatchTemporary = Join-Path $embeddedDirectory '.patch_core-0123456789abcdef0123456789abcdef.tmp'
    $formalPatchCore = Join-Path $embeddedDirectory 'patch_core.dll'
    $similarPatchTemporary = Join-Path $embeddedDirectory '.patch_core-not-a-guid.tmp'
    [System.IO.File]::WriteAllText($stalePatchTemporary, 'stale')
    [System.IO.File]::WriteAllText($formalPatchCore, 'formal')
    [System.IO.File]::WriteAllText($similarPatchTemporary, 'keep')
    Remove-GBFRStaleNamedPaths `
        -Directory $embeddedDirectory `
        -ExactNamePattern '^\.patch_core-[0-9a-fA-F]{32}\.tmp$' `
        -Kind File
    Assert-GBFR -Condition (-not (Test-Path -LiteralPath $stalePatchTemporary)) -Message 'Exact stale patch_core temporary was not removed.'
    Assert-GBFR -Condition (Test-Path -LiteralPath $formalPatchCore) -Message 'Formal patch_core.dll was removed by stale cleanup.'
    Assert-GBFR -Condition (Test-Path -LiteralPath $similarPatchTemporary) -Message 'A similarly named non-tool file was removed.'

    $releaseParent = Join-Path $testRoot 'releases'
    [System.IO.Directory]::CreateDirectory($releaseParent) | Out-Null
    $stalePartial = Join-Path $releaseParent '.release-v2.0.3.partial-0123456789abcdef0123456789abcdef'
    $formalRelease = Join-Path $releaseParent 'release-v2.0.3'
    $similarPartial = Join-Path $releaseParent '.release-v2.0.3.partial-not-a-guid'
    [System.IO.Directory]::CreateDirectory($stalePartial) | Out-Null
    [System.IO.Directory]::CreateDirectory($formalRelease) | Out-Null
    [System.IO.Directory]::CreateDirectory($similarPartial) | Out-Null
    Remove-GBFRStaleNamedPaths `
        -Directory $releaseParent `
        -ExactNamePattern '^\.release-v2\.0\.3\.partial-[0-9a-fA-F]{32}$' `
        -Kind Directory
    Assert-GBFR -Condition (-not (Test-Path -LiteralPath $stalePartial)) -Message 'Exact stale release partial was not removed.'
    Assert-GBFR -Condition (Test-Path -LiteralPath $formalRelease) -Message 'Formal release directory was removed by stale cleanup.'
    Assert-GBFR -Condition (Test-Path -LiteralPath $similarPartial) -Message 'A similarly named non-tool directory was removed.'

    $packageProbeParent = Join-Path $testRoot 'package-script-probe'
    [System.IO.Directory]::CreateDirectory($packageProbeParent) | Out-Null
    $packageProbeOutput = Join-Path $packageProbeParent 'release-v2.0.3'
    $packageProbeStale = Join-Path $packageProbeParent '.release-v2.0.3.partial-fedcba9876543210fedcba9876543210'
    [System.IO.Directory]::CreateDirectory($packageProbeOutput) | Out-Null
    [System.IO.Directory]::CreateDirectory($packageProbeStale) | Out-Null
    $formalOutputRejected = $false
    try {
        & (Join-Path $PSScriptRoot 'package_windows_release.ps1') `
            -Version 'v2.0.3' `
            -OutputDirectory $packageProbeOutput
    }
    catch {
        $formalOutputRejected = $_.Exception.Message -like 'Release output already exists*'
    }
    Assert-GBFR -Condition $formalOutputRejected -Message 'The release packager did not reject an existing formal output.'
    Assert-GBFR -Condition (-not (Test-Path -LiteralPath $packageProbeStale)) -Message 'The release packager did not clean its exact stale partial.'
    Assert-GBFR -Condition (Test-Path -LiteralPath $packageProbeOutput) -Message 'The release packager removed a formal output during stale cleanup.'

    $rollbackDirectory = Join-Path $testRoot 'rollback'
    [System.IO.Directory]::CreateDirectory($rollbackDirectory) | Out-Null
    $destination = Join-Path $rollbackDirectory 'patch_core.dll'
    $backup = Join-Path $testRoot 'verified-backup.dll'
    [System.IO.File]::WriteAllText($destination, 'new native bytes')
    [System.IO.File]::WriteAllText($backup, 'old native bytes')
    $expectedHash = Get-GBFRSha256Hex -LiteralPath $backup
    Restore-GBFRFileFromVerifiedBackup `
        -BackupPath $backup `
        -DestinationPath $destination `
        -ExpectedHash $expectedHash
    Assert-GBFR `
        -Condition ((Get-GBFRSha256Hex -LiteralPath $destination) -eq $expectedHash) `
        -Message 'Atomic rollback did not restore the verified bytes.'

    [System.IO.File]::WriteAllText($destination, 'current native bytes')
    [System.IO.File]::WriteAllText($backup, 'corrupted backup')
    $currentHash = Get-GBFRSha256Hex -LiteralPath $destination
    $corruptRejected = $false
    try {
        Restore-GBFRFileFromVerifiedBackup `
            -BackupPath $backup `
            -DestinationPath $destination `
            -ExpectedHash $expectedHash
    }
    catch {
        $corruptRejected = $_.Exception.Message -like 'Rollback backup failed integrity verification*'
    }
    Assert-GBFR -Condition $corruptRejected -Message 'A corrupted rollback backup was accepted.'
    Assert-GBFR `
        -Condition ((Get-GBFRSha256Hex -LiteralPath $destination) -eq $currentHash) `
        -Message 'A rejected rollback changed the destination.'
}
finally {
    if (Test-Path -LiteralPath $testRoot) {
        Remove-Item -LiteralPath $testRoot -Recurse -Force
    }
}

Write-Output 'Windows release safety tests passed.'
