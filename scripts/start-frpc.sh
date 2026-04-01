#!/usr/bin/env bash

set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
CONFIG_PATH="${1:-$ROOT_DIR/config/frpc.toml}"
TOOLS_DIR="$ROOT_DIR/.tools"

resolve_platform() {
  case "$(uname -s)" in
    Darwin)  printf "darwin" ;;
    Linux)   printf "linux" ;;
    MINGW*|MSYS*|CYGWIN*) printf "windows" ;;
    *) echo "不支持的系统：$(uname -s)" >&2; exit 1 ;;
  esac
}

resolve_arch() {
  case "$(uname -m)" in
    x86_64|amd64)  printf "amd64" ;;
    arm64|aarch64) printf "arm64" ;;
    *) echo "不支持的架构：$(uname -m)" >&2; exit 1 ;;
  esac
}

CURRENT_PLATFORM="$(resolve_platform)"
CURRENT_ARCH="$(resolve_arch)"

BIN_DIR="$TOOLS_DIR/$CURRENT_PLATFORM"
BIN_NAME="frpc"
[ "$CURRENT_PLATFORM" = "windows" ] && BIN_NAME="frpc.exe"
FRPC_BIN="$BIN_DIR/$BIN_NAME"

if [ ! -x "$FRPC_BIN" ]; then
  echo "未找到 $CURRENT_PLATFORM/$CURRENT_ARCH 的 frpc，正在安装..."
  bash "$ROOT_DIR/scripts/setup-frpc.sh"
fi

if [ ! -x "$FRPC_BIN" ]; then
  echo "错误：安装后仍未找到 $FRPC_BIN" >&2
  exit 1
fi

echo "使用配置文件：$CONFIG_PATH"
echo "frpc 路径：$FRPC_BIN"
exec "$FRPC_BIN" -c "$CONFIG_PATH"
