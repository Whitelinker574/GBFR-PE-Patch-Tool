param(
    [ValidateSet('Debug', 'Release')]
    [string]$Configuration = 'Release',
    [string]$LibmemRootOverride = '',
    [switch]$DependenciesOnly
)

$ErrorActionPreference = 'Stop'
$repoRoot = Split-Path -Parent $PSScriptRoot
$project = Join-Path $repoRoot 'src_dll\patch_core\patch_core.vcxproj'
$vswhere = Join-Path ${env:ProgramFiles(x86)} 'Microsoft Visual Studio\Installer\vswhere.exe'
$libmemRoot = if ($LibmemRootOverride) { [System.IO.Path]::GetFullPath($LibmemRootOverride) } else { Join-Path $repoRoot 'src_dll\thirdparty\libmem' }
$libmemArchiveUrl = 'https://github.com/rdbo/libmem/releases/download/5.1.5/libmem-5.1.5-x86_64-windows-msvc-static-mt.tar.gz'
$libmemArchiveSha256 = '40C0F5BF2851C51E7424AE9DD891DB1F004A99D7A574E091072CDC4112B51024'
$libmemHashes = @{
    Debug = '81869606237334DD7F74682003A6A3EB6F4E60D0D5A27E040C917D910F76D62F'
    Release = 'F75D281164AC40C761F314049DEB30D2925D8BDC2CB5317EF71680B54BEF2150'
}

function Get-Sha256Hex {
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

function Ensure-LibmemLibraries {
    $selectedLibrary = Join-Path $libmemRoot "lib\$($Configuration.ToLowerInvariant())\libmem.lib"
    if (Test-Path -LiteralPath $selectedLibrary) {
        $actual = Get-Sha256Hex -LiteralPath $selectedLibrary
        if ($actual -ne $libmemHashes[$Configuration]) {
            throw "Existing libmem $Configuration library failed integrity verification: $actual"
        }
        return
    }

    $tempRoot = Join-Path ([System.IO.Path]::GetTempPath()) ("gbfr-libmem-" + [guid]::NewGuid().ToString('N'))
    $archive = Join-Path $tempRoot 'libmem.tar.gz'
    $extractRoot = Join-Path $tempRoot 'extract'
    New-Item -ItemType Directory -Path $extractRoot -Force | Out-Null
    try {
        Write-Host "Downloading pinned libmem 5.1.5 build dependency..."
        Invoke-WebRequest -Uri $libmemArchiveUrl -OutFile $archive
        $archiveHash = Get-Sha256Hex -LiteralPath $archive
        if ($archiveHash -ne $libmemArchiveSha256) {
            throw "Downloaded libmem archive failed integrity verification: $archiveHash"
        }
        & tar -xf $archive -C $extractRoot
        if ($LASTEXITCODE -ne 0) {
            throw "libmem archive extraction failed with exit code $LASTEXITCODE"
        }
        $packageRoot = Join-Path $extractRoot 'libmem-5.1.5-x86_64-windows-msvc-static-mt'
        foreach ($name in @('Debug', 'Release')) {
            $source = Join-Path $packageRoot "lib\$($name.ToLowerInvariant())\libmem.lib"
            $expected = $libmemHashes[$name]
            if (-not (Test-Path -LiteralPath $source)) {
                throw "Downloaded libmem package is missing the $name library."
            }
            $actual = Get-Sha256Hex -LiteralPath $source
            if ($actual -ne $expected) {
                throw "Downloaded libmem $name library failed integrity verification: $actual"
            }
            $destinationDirectory = Join-Path $libmemRoot "lib\$($name.ToLowerInvariant())"
            New-Item -ItemType Directory -Path $destinationDirectory -Force | Out-Null
            Copy-Item -LiteralPath $source -Destination (Join-Path $destinationDirectory 'libmem.lib') -Force
        }
    }
    finally {
        if (Test-Path -LiteralPath $tempRoot) {
            Remove-Item -LiteralPath $tempRoot -Recurse -Force
        }
    }
}

Ensure-LibmemLibraries
if ($DependenciesOnly) {
    Write-Host "Verified libmem dependencies in $libmemRoot"
    return
}

if (-not (Test-Path -LiteralPath $vswhere)) {
    throw 'Visual Studio Build Tools with the Desktop development with C++ workload is required.'
}

$installation = & $vswhere -latest -products '*' -requires Microsoft.Component.MSBuild -property installationPath
if (-not $installation) {
    throw 'MSBuild was not found through Visual Studio Installer.'
}
$msbuild = Join-Path $installation 'MSBuild\Current\Bin\MSBuild.exe'
if (-not (Test-Path -LiteralPath $msbuild)) {
    throw "MSBuild is missing: $msbuild"
}

& $msbuild $project /m /t:Build "/p:Configuration=$Configuration" /p:Platform=x64 /verbosity:minimal
if ($LASTEXITCODE -ne 0) {
    throw "patch_core.dll build failed with exit code $LASTEXITCODE"
}

$built = Join-Path $repoRoot "src_dll\patch_core\x64\$Configuration\patch_core.dll"
$embedded = Join-Path $repoRoot 'internal\backend\resources\patch_core.dll'
if (-not (Test-Path -LiteralPath $built)) {
    throw "Native runtime output is missing: $built"
}
Copy-Item -LiteralPath $built -Destination $embedded -Force

$builtHash = Get-Sha256Hex -LiteralPath $built
$embeddedHash = Get-Sha256Hex -LiteralPath $embedded
if ($builtHash -ne $embeddedHash) {
    throw 'Embedded patch_core.dll does not match the native build output.'
}
Write-Host "Embedded patch_core.dll: $embeddedHash"
