@echo off
setlocal
cd /d "%~dp0.."

echo Building Matrix Web...

where go >nul 2>nul
if %ERRORLEVEL% NEQ 0 (
    echo Error: Go not found. Install Go 1.24+ first.
    exit /b 1
)

where npm >nul 2>nul
if %ERRORLEVEL% NEQ 0 (
    echo Error: npm not found. Install Node.js 22+ first.
    exit /b 1
)

echo [1/2] Frontend...
pushd frontend
call npm install
if %ERRORLEVEL% NEQ 0 exit /b 1
call npm run build
if %ERRORLEVEL% NEQ 0 exit /b 1
popd

echo [2/2] Backend...
if not exist build mkdir build
go build -o build\matrix.exe .\cmd\web
if %ERRORLEVEL% NEQ 0 exit /b 1

echo.
echo Build successful!
echo Run: build\matrix.exe -config config\config.yml
endlocal
