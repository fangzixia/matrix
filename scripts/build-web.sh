#!/usr/bin/env bash
set -euo pipefail
cd "$(dirname "$0")/.."

echo "Building Matrix Web..."

command -v go >/dev/null || { echo "Go 1.24+ required"; exit 1; }
command -v npm >/dev/null || { echo "Node.js 22+ required"; exit 1; }

echo "[1/2] Frontend..."
(cd frontend && npm install && npm run build)

echo "[2/2] Backend..."
mkdir -p build
go build -o build/matrix ./cmd/web

echo
echo "Build successful!"
echo "Run: ./build/matrix -config config/config.yml"
