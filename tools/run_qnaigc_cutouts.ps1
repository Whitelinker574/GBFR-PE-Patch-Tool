$ErrorActionPreference = 'Stop'

$toolDir = Split-Path -Parent $MyInvocation.MyCommand.Path
$script = Join-Path $toolDir 'qnaigc_image_edit.py'
$manifest = Join-Path $toolDir 'qnaigc_cutouts.json'

$secureKey = Read-Host '请输入 QNAIGC API Key（输入内容不会显示，也不会保存）' -AsSecureString
$keyPtr = [Runtime.InteropServices.Marshal]::SecureStringToBSTR($secureKey)

try {
    $env:QNAIGC_API_KEY = [Runtime.InteropServices.Marshal]::PtrToStringBSTR($keyPtr)
    python $script --manifest $manifest @args
    if ($LASTEXITCODE -ne 0) { throw "图片生成脚本退出，代码 $LASTEXITCODE" }
    Write-Host ''
    Write-Host '全部生成完成。可以关闭此窗口。' -ForegroundColor Green
}
finally {
    Remove-Item Env:QNAIGC_API_KEY -ErrorAction SilentlyContinue
    [Runtime.InteropServices.Marshal]::ZeroFreeBSTR($keyPtr)
}

Read-Host '按回车键关闭'
