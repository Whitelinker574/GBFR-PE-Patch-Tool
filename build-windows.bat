@echo off
setlocal

powershell.exe -NoProfile -ExecutionPolicy Bypass -File "%~dp0tools\build_windows.ps1"
set "BUILD_EXIT=%ERRORLEVEL%"
exit /b %BUILD_EXIT%
