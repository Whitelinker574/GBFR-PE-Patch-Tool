param(
    [System.IO.FileStream]$InheritedBuildLease,
    [string]$InheritedBuildLockPath = '',
    $InheritedRecoveryTransaction
)

$ErrorActionPreference = 'Stop'

$repoRoot = [System.IO.Path]::GetFullPath((Join-Path $PSScriptRoot '..'))
. (Join-Path $PSScriptRoot 'windows_release_common.ps1')

function Invoke-GBFRCheckedNativeCommand {
    param(
        [Parameter(Mandatory = $true)][string]$Executable,
        [Parameter(Mandatory = $true)][string[]]$Arguments,
        [Parameter(Mandatory = $true)][string]$Description
    )

    & $Executable @Arguments
    if ($LASTEXITCODE -ne 0) {
        throw "$Description failed with exit code $LASTEXITCODE."
    }
}

$buildLease = $null
$ownsBuildLease = $false
$recoveryTransaction = $null
$ownsRecoveryTransaction = $false
$transactionCompleted = $false
$patchCoreResource = Join-Path $repoRoot 'internal\backend\resources\patch_core.dll'
$patchCoreDirectory = Split-Path -Parent $patchCoreResource
$buildLockPath = Get-GBFRRepositoryLockPath -RepositoryRoot $repoRoot -Scope 'windows-build'

try {
    if ($null -ne $InheritedBuildLease) {
        if ([string]::IsNullOrWhiteSpace($InheritedBuildLockPath)) {
            throw 'An inherited Windows build lease requires its lock path.'
        }
        Assert-GBFRBuildLeaseHeld -Lease $InheritedBuildLease -ExpectedLockPath $buildLockPath
        if (-not [System.IO.Path]::GetFullPath($InheritedBuildLockPath).Equals(
            [System.IO.Path]::GetFullPath($buildLockPath),
            [System.StringComparison]::OrdinalIgnoreCase
        )) {
            throw 'The inherited Windows build lease belongs to an unexpected lock path.'
        }
        $buildLease = $InheritedBuildLease
    }
    else {
        if (-not [string]::IsNullOrWhiteSpace($InheritedBuildLockPath) -or $null -ne $InheritedRecoveryTransaction) {
            throw 'Inherited build state cannot be supplied without the open repository build lease.'
        }
        $buildLease = Enter-GBFRExclusiveLease -LockPath $buildLockPath -Description 'Windows production, native, or release build'
        $ownsBuildLease = $true
    }
    [System.IO.Directory]::CreateDirectory($patchCoreDirectory) | Out-Null
    Invoke-GBFRPendingPatchCoreRecovery `
        -RepositoryRoot $repoRoot `
        -DestinationPath $patchCoreResource `
        -Lease $buildLease `
        -LockPath $buildLockPath | Out-Null
    Remove-GBFRStaleNamedPaths `
        -Directory $patchCoreDirectory `
        -ExactNamePattern '^\.patch_core-[0-9a-fA-F]{32}\.tmp$' `
        -Kind File
    if ($null -ne $InheritedRecoveryTransaction) {
        Assert-GBFRPatchCoreRecoveryTransaction `
            -Transaction $InheritedRecoveryTransaction `
            -RepositoryRoot $repoRoot `
            -DestinationPath $patchCoreResource
        $recoveryTransaction = $InheritedRecoveryTransaction
    }
    else {
        $recoveryTransaction = Start-GBFRPatchCoreRecoveryTransaction `
            -RepositoryRoot $repoRoot `
            -DestinationPath $patchCoreResource `
            -Lease $buildLease `
            -LockPath $buildLockPath
        $ownsRecoveryTransaction = $true
    }

    Push-Location -LiteralPath $repoRoot
    try {
        Write-Host '[1/5] Rebuilding the integrated native runtime...'
        & (Join-Path $PSScriptRoot 'build_patch_core.ps1') `
            -Configuration Release `
            -InheritedBuildLease $buildLease `
            -InheritedBuildLockPath $buildLockPath

        Write-Host '[2/5] Checking required embedded resources...'
        if (-not (Test-Path -LiteralPath $patchCoreResource -PathType Leaf)) {
            throw "Missing embedded native runtime: $patchCoreResource"
        }
        $frontendDist = Join-Path $repoRoot 'frontend\dist'
        [System.IO.Directory]::CreateDirectory($frontendDist) | Out-Null
        $embedPlaceholder = Join-Path $frontendDist '.embed-placeholder'
        if (-not (Test-Path -LiteralPath $embedPlaceholder)) {
            [System.IO.File]::WriteAllText(
                $embedPlaceholder,
                'Wails embed placeholder',
                [System.Text.UTF8Encoding]::new($false)
            )
        }

        Write-Host '[3/5] Generating Wails bindings...'
        Invoke-GBFRCheckedNativeCommand `
            -Executable 'wails' `
            -Arguments @('generate', 'module') `
            -Description 'Wails binding generation'

        Write-Host '[4/5] Building frontend...'
        $frontendDirectory = Join-Path $repoRoot 'frontend'
        Push-Location -LiteralPath $frontendDirectory
        try {
            if (-not (Test-Path -LiteralPath (Join-Path $frontendDirectory 'node_modules\pinyin-pro\package.json') -PathType Leaf)) {
                Write-Host 'Installing frontend dependencies...'
                Invoke-GBFRCheckedNativeCommand `
                    -Executable 'npm.cmd' `
                    -Arguments @('ci') `
                    -Description 'Frontend dependency installation'
            }
            Invoke-GBFRCheckedNativeCommand `
                -Executable 'npm.cmd' `
                -Arguments @('run', 'build') `
                -Description 'Frontend production build'
        }
        finally {
            Pop-Location
        }

        Write-Host '[5/5] Building clean Windows amd64 release...'
        Invoke-GBFRCheckedNativeCommand `
            -Executable 'wails' `
            -Arguments @('build', '-clean', '-platform', 'windows/amd64', '-s') `
            -Description 'Windows amd64 Wails build'
        $builtExecutable = Join-Path $repoRoot 'build\bin\GBFR PE Patch Tool.exe'
        if (-not (Test-Path -LiteralPath $builtExecutable -PathType Leaf)) {
            throw "Wails returned without producing: $builtExecutable"
        }
    }
    finally {
        Pop-Location
    }

    if ($ownsRecoveryTransaction) {
        Complete-GBFRPatchCoreRecoveryTransaction `
            -RepositoryRoot $repoRoot `
            -Transaction $recoveryTransaction `
            -Lease $buildLease `
            -LockPath $buildLockPath
        $transactionCompleted = $true
    }
    Write-Host 'Build complete.'
}
catch {
    $buildFailure = $_
    if ($ownsRecoveryTransaction -and $null -ne $recoveryTransaction -and -not $transactionCompleted) {
        try {
            Rollback-GBFRPatchCoreRecoveryTransaction `
                -RepositoryRoot $repoRoot `
                -Transaction $recoveryTransaction `
                -Lease $buildLease `
                -LockPath $buildLockPath
            $transactionCompleted = $true
        }
        catch {
            $rollbackFailure = $_
            $paths = Get-GBFRPatchCoreRecoveryPaths -RepositoryRoot $repoRoot
            throw "Windows production build failed: $($buildFailure.Exception.Message) Rollback also failed: $($rollbackFailure.Exception.Message) The durable recovery journal was preserved at '$($paths.JournalPath)'."
        }
        throw "Windows production build failed; the embedded native runtime was atomically restored and hash-verified. $($buildFailure.Exception.Message)"
    }
    throw
}
finally {
    if ($ownsBuildLease) {
        Exit-GBFRExclusiveLease -Lease $buildLease
    }
}
