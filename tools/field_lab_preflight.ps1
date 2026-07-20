param(
    [switch]$RunFocusedTests
)

$ErrorActionPreference = 'Stop'
$sourceRoot = Split-Path -Parent $PSScriptRoot
$requiredFiles = @(
    'go.mod',
    'wails.json',
    'runtime_character_panel.go',
    'formula_sampler_app.go',
    'formula_sampler_scan.go',
    'frontend\package.json',
    'resources\patch_core.dll'
)

Write-Host 'GBFR field calibration preflight'
Write-Host "Source: $sourceRoot"

$missing = @()
foreach ($relativePath in $requiredFiles) {
    $candidate = Join-Path $sourceRoot $relativePath
    if (-not (Test-Path -LiteralPath $candidate)) {
        $missing += $relativePath
    }
}
if ($missing.Count -gt 0) {
    throw "Missing required files: $($missing -join ', ')"
}

function Show-ToolVersion {
    param(
        [string]$Name,
        [string[]]$Arguments
    )
    $command = Get-Command $Name -ErrorAction SilentlyContinue
    if ($null -eq $command) {
        Write-Warning "$Name is not installed or is not on PATH"
        return
    }
    $version = & $command.Source @Arguments 2>&1 | Select-Object -First 2
    Write-Host "$Name`: $($version -join ' ')"
}

Show-ToolVersion -Name 'git' -Arguments @('--version')
Show-ToolVersion -Name 'go' -Arguments @('version')
Show-ToolVersion -Name 'node' -Arguments @('--version')
Show-ToolVersion -Name 'npm' -Arguments @('--version')
Show-ToolVersion -Name 'wails' -Arguments @('version')

$webViewClients = @(
    'HKLM:\SOFTWARE\Microsoft\EdgeUpdate\Clients\{F3017226-FE2A-4295-8BDF-00C3A9A7E4C5}',
    'HKLM:\SOFTWARE\WOW6432Node\Microsoft\EdgeUpdate\Clients\{F3017226-FE2A-4295-8BDF-00C3A9A7E4C5}',
    'HKCU:\SOFTWARE\Microsoft\EdgeUpdate\Clients\{F3017226-FE2A-4295-8BDF-00C3A9A7E4C5}'
)
$webViewFound = $false
foreach ($registryPath in $webViewClients) {
    if (Test-Path -LiteralPath $registryPath) {
        $webViewFound = $true
        $version = (Get-ItemProperty -LiteralPath $registryPath -ErrorAction SilentlyContinue).pv
        Write-Host "WebView2 Runtime: $version"
        break
    }
}
if (-not $webViewFound) {
    Write-Warning 'WebView2 Runtime was not found in the common machine-wide registry locations.'
}

$game = Get-Process -Name 'granblue_fantasy_relink' -ErrorAction SilentlyContinue
if ($null -eq $game) {
    Write-Warning 'The game process is not running. Start it before live calibration.'
} else {
    Write-Host "Game process detected: PID $($game.Id)"
}

if ($RunFocusedTests) {
    Push-Location $sourceRoot
    try {
        & go test . -run 'FormulaSampler|RuntimeCharacterPanel' -count=1
        if ($LASTEXITCODE -ne 0) {
            throw "Focused Go tests failed with exit code $LASTEXITCODE"
        }
        Push-Location (Join-Path $sourceRoot 'frontend')
        try {
            & node --test src/formulaSamplerBindingSync.test.js src/formulaSamplerUi.test.js
            if ($LASTEXITCODE -ne 0) {
                throw "Focused frontend tests failed with exit code $LASTEXITCODE"
            }
        } finally {
            Pop-Location
        }
    } finally {
        Pop-Location
    }
}

Write-Host 'Preflight complete.'
