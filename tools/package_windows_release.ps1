param(
    [Parameter(Mandatory = $true)]
    [ValidatePattern('^v\d+\.\d+\.\d+$')]
    [string]$Version,

    [string]$OutputDirectory,

    [string]$Commit
)

$ErrorActionPreference = 'Stop'
$repositoryRoot = (Resolve-Path (Join-Path $PSScriptRoot '..')).Path
$plainVersion = $Version.Substring(1)
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

$buildScript = Join-Path $repositoryRoot 'build-windows.bat'
& $buildScript
if ($LASTEXITCODE -ne 0) {
    throw "Production build failed with exit code $LASTEXITCODE."
}
$dirtyAfterBuild = @(& git -C $repositoryRoot status --porcelain=v1 --untracked-files=all)
if ($LASTEXITCODE -ne 0) {
    throw 'Unable to inspect the worktree after the production build.'
}
if ($dirtyAfterBuild.Count -gt 0) {
    throw "The production build changed tracked or untracked source files; packaging stopped to preserve commit provenance.`n$($dirtyAfterBuild -join "`n")"
}

$builtExecutable = Join-Path $repositoryRoot 'build\bin\GBFR PE Patch Tool.exe'
if (-not (Test-Path -LiteralPath $builtExecutable -PathType Leaf)) {
    throw "Production executable is missing after the clean build: $builtExecutable."
}

if ([string]::IsNullOrWhiteSpace($OutputDirectory)) {
    $OutputDirectory = Join-Path (Split-Path $repositoryRoot -Parent) "release-$Version"
}
$outputPath = [System.IO.Path]::GetFullPath($OutputDirectory)
if (Test-Path -LiteralPath $outputPath) {
    throw "Release output already exists: $outputPath. Choose a new empty directory."
}

$archiveBaseName = "GBFR-PE-Patch-Tool-$Version-windows-amd64"
$stagePath = Join-Path $outputPath 'package'
$nativeLicensePath = Join-Path $stagePath 'licenses\native'
New-Item -ItemType Directory -Path $nativeLicensePath -Force | Out-Null

$releaseExecutable = Join-Path $outputPath "$archiveBaseName.exe"
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
$nativeHelperHash = (Get-FileHash -Algorithm SHA256 -LiteralPath (Join-Path $repositoryRoot 'internal\backend\resources\patch_core.dll')).Hash.ToLowerInvariant()
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

$archivePath = Join-Path $outputPath "$archiveBaseName.zip"
Compress-Archive -Path (Join-Path $stagePath '*') -DestinationPath $archivePath -CompressionLevel Optimal

$checksums = @(
    Get-FileHash -Algorithm SHA256 -LiteralPath $releaseExecutable
    Get-FileHash -Algorithm SHA256 -LiteralPath $archivePath
)
$checksumPath = Join-Path $outputPath "SHA256SUMS-$Version.txt"
$checksumLines = $checksums | ForEach-Object {
    "$($_.Hash.ToLowerInvariant())  $(Split-Path $_.Path -Leaf)"
}
[System.IO.File]::WriteAllLines($checksumPath, $checksumLines, [System.Text.UTF8Encoding]::new($false))

Write-Output "Release package: $outputPath"
Write-Output "Executable: $releaseExecutable"
Write-Output "Archive: $archivePath"
Write-Output "Checksums: $checksumPath"
