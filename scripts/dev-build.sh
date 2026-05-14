#!/bin/bash
set -euo pipefail

APP_DIR="build/bin/frp-client.app"
GOPATH_BIN="${GOPATH:-$HOME/go}/bin"

echo "==> 清理旧构建..."
rm -rf "$APP_DIR"

echo "==> 编译..."
"$GOPATH_BIN/wails" build

echo "==> 启动..."
open "$APP_DIR"
echo "完成"
