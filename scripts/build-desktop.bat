@echo off
echo Building Matrix Desktop for Windows...

REM 检查 Wails CLI 是否安装
where wails >nul 2>nul
if %ERRORLEVEL% NEQ 0 (
    echo Error: Wails CLI not found. Please install it first:
    echo go install github.com/wailsapp/wails/v2/cmd/wails@latest
    exit /b 1
)

REM 构建应用
wails build -clean

if %ERRORLEVEL% EQU 0 (
    echo.
    echo Build successful!
    echo Output: build\bin\matrix.exe
) else (
    echo.
    echo Build failed!
    exit /b 1
)
