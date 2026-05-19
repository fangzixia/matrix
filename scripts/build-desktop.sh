#!/bin/bash
set -e

echo "Building Matrix Desktop..."

# 检查 Wails CLI 是否安装
if ! command -v wails &> /dev/null; then
    echo "Error: Wails CLI not found. Please install it first:"
    echo "go install github.com/wailsapp/wails/v2/cmd/wails@latest"
    exit 1
fi

# 构建应用
wails build -clean

echo ""
echo "Build successful!"
echo "Output: build/bin/matrix"
