@echo off
setlocal

cd /d "%~dp0" || exit /b %errorlevel%

echo [1/4] Checking required embedded resources...
if not exist "resources\patch_core.dll" (
	echo ERROR: Missing resources\patch_core.dll. Build src_dll\patch_core as Release x64 first.
	exit /b 1
)
if not exist "frontend\dist" mkdir "frontend\dist"
if not exist "frontend\dist\.embed-placeholder" echo Wails embed placeholder>"frontend\dist\.embed-placeholder"

echo [2/4] Generating Wails bindings...
wails generate module
if errorlevel 1 exit /b %errorlevel%

echo [3/4] Building frontend...
cd /d "%~dp0frontend" || exit /b %errorlevel%
if not exist "node_modules\pinyin-pro\package.json" (
	echo Installing frontend dependencies...
	call npm ci
	if errorlevel 1 exit /b %errorlevel%
)
call npm run build
if errorlevel 1 exit /b %errorlevel%

echo [4/4] Building clean Windows amd64 release...
cd /d "%~dp0" || exit /b %errorlevel%
wails build -clean -platform windows/amd64 -s
if errorlevel 1 exit /b %errorlevel%
if not exist "build\bin\GBFR PE Patch Tool.exe" (
	echo ERROR: Wails returned without producing build\bin\GBFR PE Patch Tool.exe.
	exit /b 1
)

echo Build complete.
exit /b 0
