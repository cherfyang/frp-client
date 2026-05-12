#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT_DIR"

echo "=== 编译 frp-client 多平台应用 ==="

# macOS
echo ""
echo "--- macOS amd64 ---"
wails build -platform darwin/amd64 -clean -o frp-client-darwin-amd64

echo ""
echo "--- macOS arm64 ---"
wails build -platform darwin/arm64 -clean -o frp-client-darwin-arm64

# Windows
echo ""
echo "--- Windows amd64 ---"
wails build -platform windows/amd64 -clean -o frp-client-windows-amd64.exe

# Linux
echo ""
echo "--- Linux amd64 ---"
wails build -platform linux/amd64 -clean -o frp-client-linux-amd64

echo ""
echo "=== 编译完成 ==="
ls -la build/bin/
