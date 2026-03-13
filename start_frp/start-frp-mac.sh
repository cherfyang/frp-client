#!/usr/bin/env bash

set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
CONFIG_PATH="${1:-$ROOT_DIR/frpc.toml}"
LOCAL_BIN="$ROOT_DIR/.tools/frp/bin/frpc"
VERSION_FILE="$ROOT_DIR/.tools/frp/VERSION"

# 检查当前系统
CURRENT_OS="$(uname -s)"
if [ "$CURRENT_OS" != "Darwin" ]; then
  echo "错误：当前系统是 $CURRENT_OS，请使用 Linux 系统对应的启动脚本 (start-frp-linux.sh)。" >&2
  exit 1
fi

# 检查当前系统架构
get_current_platform() {
  case "$(uname -s)" in
    Darwin) printf "darwin" ;;
    Linux) printf "linux" ;;
    *) echo "不支持的系统：$(uname -s)" >&2; exit 1 ;;
  esac
}

get_current_arch() {
  case "$(uname -m)" in
    x86_64|amd64) printf "amd64" ;;
    arm64|aarch64) printf "arm64" ;;
    *) echo "不支持的架构：$(uname -m)" >&2; exit 1 ;;
  esac
}

CURRENT_PLATFORM="$(get_current_platform)"
CURRENT_ARCH="$(get_current_arch)"

# 检查 frpc 是否需要重新下载
need_reinstall() {
  # 文件不存在
  [ ! -x "$LOCAL_BIN" ] && return 0
  
  # 版本文件不存在
  [ ! -f "$VERSION_FILE" ] && return 0
  
  # 检查架构是否匹配 (通过文件类型判断)
  local file_info
  file_info="$(file "$LOCAL_BIN" 2>/dev/null || true)"
  
  case "$CURRENT_PLATFORM-$CURRENT_ARCH" in
    darwin-arm64)
      echo "$file_info" | grep -q "arm64" || return 0
      ;;
    darwin-amd64)
      echo "$file_info" | grep -q "x86_64" || return 0
      ;;
    linux-amd64)
      echo "$file_info" | grep -q "x86_64" || return 0
      ;;
    linux-arm64)
      echo "$file_info" | grep -q "aarch64\|arm64" || return 0
      ;;
    *)
      return 0
      ;;
  esac
  
  return 1
}

if need_reinstall; then
  echo "当前平台架构：$CURRENT_PLATFORM/$CURRENT_ARCH"
  echo "frpc 不存在或架构不匹配，正在自动安装..."
  bash "$ROOT_DIR/setup-frpc.sh"
fi

if [ -n "${FRPC_BIN:-}" ]; then
  FRPC_COMMAND="$FRPC_BIN"
elif [ -x "$LOCAL_BIN" ]; then
  FRPC_COMMAND="$LOCAL_BIN"
elif command -v frpc >/dev/null 2>&1; then
  FRPC_COMMAND="$(command -v frpc)"
else
  echo "未找到 frpc 可执行文件。请先运行 ./setup-frpc.sh 或设置 FRPC_BIN。" >&2
  exit 1
fi

echo "使用配置文件：$CONFIG_PATH"
exec "$FRPC_COMMAND" -c "$CONFIG_PATH"
