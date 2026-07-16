@echo off
setlocal
powershell.exe -NoProfile -ExecutionPolicy Bypass -File "%~dp0run_qnaigc_cutouts.ps1" %*
endlocal
