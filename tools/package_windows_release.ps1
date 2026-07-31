param(
    [Parameter(Mandatory = $true)]
    [ValidatePattern('^v\d+\.\d+\.\d+$')]
    [string]$Version,

    [string]$OutputDirectory,

    [string]$Commit
)

$ErrorActionPreference = 'Stop'
$repositoryRoot = (Resolve-Path (Join-Path $PSScriptRoot '..')).Path
. (Join-Path $PSScriptRoot 'windows_release_common.ps1')

$plainVersion = $Version.Substring(1)
if ([string]::IsNullOrWhiteSpace($OutputDirectory)) {
    $OutputDirectory = Join-Path (Split-Path $repositoryRoot -Parent) "release-$Version"
}
$outputPath = [System.IO.Path]::GetFullPath($OutputDirectory)
$repositoryPrefix = $repositoryRoot.TrimEnd(
    [System.IO.Path]::DirectorySeparatorChar,
    [System.IO.Path]::AltDirectorySeparatorChar
) + [System.IO.Path]::DirectorySeparatorChar
if ($outputPath.Equals($repositoryRoot, [System.StringComparison]::OrdinalIgnoreCase) -or
    $outputPath.StartsWith($repositoryPrefix, [System.StringComparison]::OrdinalIgnoreCase)) {
    throw "Release output must be outside the Git repository: $outputPath"
}

$outputParent = Split-Path -Parent $outputPath
[System.IO.Directory]::CreateDirectory($outputParent) | Out-Null
$outputLeaf = Split-Path $outputPath -Leaf
$temporaryOutput = Join-Path $outputParent (
    '.' + $outputLeaf + '.partial-' + [guid]::NewGuid().ToString('N')
)
$partialNamePattern = '^\.' + [regex]::Escape($outputLeaf) + '\.partial-[0-9a-fA-F]{32}$'
$packageLockPath = Get-GBFRRepositoryLockPath -RepositoryRoot $repositoryRoot -Scope 'windows-build'
$packageLease = $null
$preserveTemporaryOutput = $false
$recoveryTransaction = $null
$transactionCompleted = $false
$embeddedResource = Join-Path $repositoryRoot 'internal\backend\resources\patch_core.dll'

try {
    $packageLease = Enter-GBFRExclusiveLease -LockPath $packageLockPath -Description 'Windows production, native, or release build'
    Remove-GBFRStaleNamedPaths `
        -Directory $outputParent `
        -ExactNamePattern $partialNamePattern `
        -Kind Directory
    if (Test-Path -LiteralPath $outputPath) {
        throw "Release output already exists: $outputPath. Choose a new empty directory."
    }

    if (-not (Test-Path -LiteralPath $embeddedResource -PathType Leaf)) {
        throw "Embedded native runtime is missing before the release build: $embeddedResource"
    }

    $headCommit = (& git -C $repositoryRoot rev-parse HEAD).Trim()
    if ($LASTEXITCODE -ne 0 -or [string]::IsNullOrWhiteSpace($headCommit)) {
        throw 'Unable to resolve the repository HEAD commit.'
    }
    $requestedCommit = $headCommit
    if (-not [string]::IsNullOrWhiteSpace($Commit)) {
        $requestedCommit = (& git -C $repositoryRoot rev-parse "$Commit^{commit}").Trim()
        if ($LASTEXITCODE -ne 0 -or [string]::IsNullOrWhiteSpace($requestedCommit)) {
            throw "Unable to resolve requested commit '$Commit'."
        }
    }
    if ($headCommit -ne $requestedCommit) {
        throw "HEAD '$headCommit' does not match requested release commit '$requestedCommit'."
    }
    $dirtyBeforeBuild = @(& git -C $repositoryRoot status --porcelain=v1 --untracked-files=all)
    if ($LASTEXITCODE -ne 0) {
        throw 'Unable to inspect the repository worktree.'
    }
    if ($dirtyBeforeBuild.Count -gt 0) {
        throw "Release packaging requires a clean worktree. Commit or remove pending files first.`n$($dirtyBeforeBuild -join "`n")"
    }

    $metadata = Get-Content (Join-Path $repositoryRoot 'wails.json') -Raw | ConvertFrom-Json
    if ($metadata.info.productVersion -ne $plainVersion) {
        throw "wails.json productVersion '$($metadata.info.productVersion)' does not match requested release '$plainVersion'."
    }

    $recoveryTransaction = Start-GBFRPatchCoreRecoveryTransaction `
        -RepositoryRoot $repositoryRoot `
        -DestinationPath $embeddedResource `
        -Lease $packageLease `
        -LockPath $packageLockPath
    $buildScript = Join-Path $repositoryRoot 'tools\build_windows.ps1'
    & $buildScript `
        -InheritedBuildLease $packageLease `
        -InheritedBuildLockPath $packageLockPath `
        -InheritedRecoveryTransaction $recoveryTransaction
    $dirtyAfterBuild = @(& git -C $repositoryRoot status --porcelain=v1 --untracked-files=all)
    if ($LASTEXITCODE -ne 0) {
        throw 'Unable to inspect the worktree after the production build.'
    }
    if ($dirtyAfterBuild.Count -gt 0) {
        throw "The production build changed tracked or untracked source files; packaging stopped to preserve commit provenance.`n$($dirtyAfterBuild -join "`n")"
    }
    Complete-GBFRPatchCoreRecoveryTransaction `
        -RepositoryRoot $repositoryRoot `
        -Transaction $recoveryTransaction `
        -Lease $packageLease `
        -LockPath $packageLockPath
    $transactionCompleted = $true

    $builtExecutable = Join-Path $repositoryRoot 'build\bin\GBFR PE Patch Tool.exe'
    if (-not (Test-Path -LiteralPath $builtExecutable -PathType Leaf)) {
        throw "Production executable is missing after the clean build: $builtExecutable."
    }

    $archiveBaseName = "GBFR-PE-Patch-Tool-$Version-windows-amd64"
    $stagePath = Join-Path $temporaryOutput 'package'
    $nativeLicensePath = Join-Path $stagePath 'licenses\native'
    [System.IO.Directory]::CreateDirectory($nativeLicensePath) | Out-Null

    $releaseExecutable = Join-Path $temporaryOutput "$archiveBaseName.exe"
    $stagedExecutable = Join-Path $stagePath "$archiveBaseName.exe"
    $releaseNotesSource = Join-Path $repositoryRoot "docs\RELEASE_NOTES_$Version.md"
    if (-not (Test-Path -LiteralPath $releaseNotesSource -PathType Leaf)) {
        throw "Release notes are missing: $releaseNotesSource"
    }
    Copy-Item -LiteralPath $builtExecutable -Destination $releaseExecutable
    Copy-Item -LiteralPath $builtExecutable -Destination $stagedExecutable
    Copy-Item -LiteralPath (Join-Path $repositoryRoot 'THIRD_PARTY_NOTICES.md') -Destination $stagePath
    Copy-Item -LiteralPath $releaseNotesSource -Destination (Join-Path $stagePath 'RELEASE_NOTES.md')
    Copy-Item -Path (Join-Path $repositoryRoot 'src_dll\thirdparty\libmem\licenses\*.txt') -Destination $nativeLicensePath

    $executableHash = (Get-FileHash -Algorithm SHA256 -LiteralPath $releaseExecutable).Hash.ToLowerInvariant()
    $nativeHelperHash = (Get-FileHash -Algorithm SHA256 -LiteralPath $embeddedResource).Hash.ToLowerInvariant()
    $branchName = (& git -C $repositoryRoot symbolic-ref --quiet --short HEAD).Trim()
    if ($LASTEXITCODE -ne 0) {
        $branchName = ''
    }
    $provenance = [ordered]@{
        schemaVersion = 1
        version = $Version
        commit = $headCommit
        branch = $branchName
        builtAtUtc = [DateTime]::UtcNow.ToString('o')
        executable = [ordered]@{
            file = "$archiveBaseName.exe"
            sha256 = $executableHash
        }
        embeddedNativeHelper = [ordered]@{
            file = 'patch_core.dll'
            sha256 = $nativeHelperHash
        }
    }
    $provenancePath = Join-Path $stagePath 'BUILD_PROVENANCE.json'
    [System.IO.File]::WriteAllText(
        $provenancePath,
        ($provenance | ConvertTo-Json -Depth 5),
        [System.Text.UTF8Encoding]::new($false)
    )

    $archivePath = Join-Path $temporaryOutput "$archiveBaseName.zip"
    Compress-Archive -Path (Join-Path $stagePath '*') -DestinationPath $archivePath -CompressionLevel Optimal

    $checksums = @(
        Get-FileHash -Algorithm SHA256 -LiteralPath $releaseExecutable
        Get-FileHash -Algorithm SHA256 -LiteralPath $archivePath
    )
    $checksumPath = Join-Path $temporaryOutput "SHA256SUMS-$Version.txt"
    $checksumLines = $checksums | ForEach-Object {
        "$($_.Hash.ToLowerInvariant())  $(Split-Path $_.Path -Leaf)"
    }
    [System.IO.File]::WriteAllLines($checksumPath, $checksumLines, [System.Text.UTF8Encoding]::new($false))

    $finalHead = (& git -C $repositoryRoot rev-parse HEAD).Trim()
    if ($LASTEXITCODE -ne 0 -or $finalHead -ne $headCommit) {
        throw 'Repository HEAD changed while the release package was being assembled.'
    }
    $finalDirty = @(& git -C $repositoryRoot status --porcelain=v1 --untracked-files=all)
    if ($LASTEXITCODE -ne 0 -or $finalDirty.Count -gt 0) {
        throw "Repository inputs changed while the release package was being assembled.`n$($finalDirty -join "`n")"
    }
    if ((Get-GBFRSha256Hex -LiteralPath $embeddedResource) -ne $nativeHelperHash -or
        (Get-GBFRSha256Hex -LiteralPath $builtExecutable) -ne $executableHash -or
        (Get-GBFRSha256Hex -LiteralPath $releaseExecutable) -ne $executableHash) {
        throw 'Release inputs or executable copies changed after provenance was recorded.'
    }

    [System.IO.Directory]::Move($temporaryOutput, $outputPath)
}
catch {
    $packageFailure = $_
    if ($null -ne $recoveryTransaction -and -not $transactionCompleted) {
        try {
            Rollback-GBFRPatchCoreRecoveryTransaction `
                -RepositoryRoot $repositoryRoot `
                -Transaction $recoveryTransaction `
                -Lease $packageLease `
                -LockPath $packageLockPath
            $transactionCompleted = $true
        }
        catch {
            $preserveTemporaryOutput = $true
            $paths = Get-GBFRPatchCoreRecoveryPaths -RepositoryRoot $repositoryRoot
            throw "Release packaging failed: $($packageFailure.Exception.Message) Embedded-runtime rollback also failed: $($_.Exception.Message) The durable recovery journal was preserved at '$($paths.JournalPath)'."
        }
    }
    throw
}
finally {
    if (-not $preserveTemporaryOutput -and (Test-Path -LiteralPath $temporaryOutput)) {
        Remove-Item -LiteralPath $temporaryOutput -Recurse -Force
    }
    Exit-GBFRExclusiveLease -Lease $packageLease
}

Write-Output "Release package: $outputPath"
Write-Output "Executable: $(Join-Path $outputPath "$archiveBaseName.exe")"
Write-Output "Archive: $(Join-Path $outputPath "$archiveBaseName.zip")"
Write-Output "Checksums: $(Join-Path $outputPath "SHA256SUMS-$Version.txt")"
