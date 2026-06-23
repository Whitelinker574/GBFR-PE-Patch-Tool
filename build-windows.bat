@echo off
setlocal

echo [1/2] Building frontend...
cd /d "%~dp0frontend" || exit /b %errorlevel%
call npm run build
if errorlevel 1 exit /b %errorlevel%

echo [2/2] Building Wails app...
cd /d "%~dp0" || exit /b %errorlevel%
wails build -s
if errorlevel 1 exit /b %errorlevel%

echo Build complete.
exit /b 0
