@echo off
setlocal

echo [1/3] Generating Wails bindings...
cd /d "%~dp0" || exit /b %errorlevel%
wails generate module
if errorlevel 1 exit /b %errorlevel%

echo [2/3] Building frontend...
cd /d "%~dp0frontend" || exit /b %errorlevel%
if not exist "node_modules\pinyin-pro\package.json" (
	echo Installing frontend dependencies...
	call npm install
	if errorlevel 1 exit /b %errorlevel%
)
call npm run build
if errorlevel 1 exit /b %errorlevel%

echo [3/3] Building Wails app...
cd /d "%~dp0" || exit /b %errorlevel%
wails build -s
if errorlevel 1 exit /b %errorlevel%

echo Build complete.
exit /b 0
