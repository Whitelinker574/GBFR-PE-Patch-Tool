@echo off
setlocal

cd /d "%~dp0" || exit /b %errorlevel%

echo [1/5] Rebuilding the integrated native runtime...
powershell -NoProfile -ExecutionPolicy Bypass -File "%~dp0tools\build_patch_core.ps1" -Configuration Release
if errorlevel 1 exit /b %errorlevel%

echo [2/5] Checking required embedded resources...
if not exist "internal\backend\resources\patch_core.dll" (
	echo ERROR: Missing internal\backend\resources\patch_core.dll. Build src_dll\patch_core as Release x64 first.
	exit /b 1
)
if not exist "frontend\dist" mkdir "frontend\dist"
if not exist "frontend\dist\.embed-placeholder" echo Wails embed placeholder>"frontend\dist\.embed-placeholder"

echo [3/5] Generating Wails bindings...
wails generate module
if errorlevel 1 exit /b %errorlevel%

echo [4/5] Building frontend...
cd /d "%~dp0frontend" || exit /b %errorlevel%
if not exist "node_modules\pinyin-pro\package.json" (
	echo Installing frontend dependencies...
	call npm ci
	if errorlevel 1 exit /b %errorlevel%
)
call npm run build
if errorlevel 1 exit /b %errorlevel%

echo [5/5] Building clean Windows amd64 release...
cd /d "%~dp0" || exit /b %errorlevel%
wails build -clean -platform windows/amd64 -s
if errorlevel 1 exit /b %errorlevel%
if not exist "build\bin\GBFR PE Patch Tool.exe" (
	echo ERROR: Wails returned without producing build\bin\GBFR PE Patch Tool.exe.
	exit /b 1
)

echo Build complete.
exit /b 0
