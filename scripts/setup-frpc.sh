#!/usr/bin/env bash

set -euo pipefail

ROOT_DIR="$(cd "$(dirname "$0")/.." && pwd)"
TOOLS_DIR="$ROOT_DIR/.tools"
VERSION_FILE="$TOOLS_DIR/VERSION"
TMP_DIR="$(mktemp -d)"

cleanup() {
  rm -rf "$TMP_DIR"
}

trap cleanup EXIT

require_command() {
  if ! command -v "$1" >/dev/null 2>&1; then
    echo "缺少依赖命令: $1" >&2
    exit 1
  fi
}

require_command curl
require_command tar

DEFAULT_VERSION="v0.68.0"

if [ -n "${FRPC_VERSION:-}" ]; then
  case "$FRPC_VERSION" in
    v*) RELEASE_TAG="$FRPC_VERSION" ;;
    *)  RELEASE_TAG="v$FRPC_VERSION" ;;
  esac
else
  RELEASE_TAG="$DEFAULT_VERSION"
fi

if [ -f "$VERSION_FILE" ] && [ "$(cat "$VERSION_FILE")" = "$RELEASE_TAG" ]; then
  echo "frpc 已安装，版本: $RELEASE_TAG"
  echo "路径: $TOOLS_DIR/{darwin,linux,windows}/"
  exit 0
fi

VERSION_NUM="${RELEASE_TAG#v}"

download_unix() {
  local platform="$1"
  local arch="$2"
  local bin_dir="$TOOLS_DIR/$platform"
  local bin_name="frpc"
  local asset_url="https://github.com/fatedier/frp/releases/download/${RELEASE_TAG}/frp_${VERSION_NUM}_${platform}_${arch}.tar.gz"
  local archive_path="$TMP_DIR/frp.tar.gz"

  mkdir -p "$bin_dir"

  echo "  下载 ${platform}/${arch}..."
  if ! curl -fSL "$asset_url" -o "$archive_path"; then
    echo "  跳过 ${platform}/${arch}（下载失败）"
    return 0
  fi

  echo "  解压 ${platform}/${arch}..."
  tar -xzf "$archive_path" -C "$TMP_DIR"

  local found
  found="$(find "$TMP_DIR" -type f -name 'frpc' | head -n 1)"
  if [ -z "$found" ]; then
    echo "  警告：压缩包中未找到 frpc。" >&2
    return 0
  fi

  cp "$found" "$bin_dir/$bin_name"
  chmod +x "$bin_dir/$bin_name"
  rm -rf "$TMP_DIR"/frp_*
  echo "  完成 -> $bin_dir/$bin_name"
}

download_windows() {
  local arch="$1"
  local bin_dir="$TOOLS_DIR/windows"
  local bin_name="frpc.exe"
  local asset_url="https://github.com/fatedier/frp/releases/download/${RELEASE_TAG}/frp_${VERSION_NUM}_windows_${arch}.zip"
  local archive_path="$TMP_DIR/frp.zip"

  mkdir -p "$bin_dir"

  echo "  下载 windows/${arch}..."
  if ! curl -fSL "$asset_url" -o "$archive_path"; then
    echo "  跳过 windows/${arch}（下载失败）"
    return 0
  fi

  echo "  解压 windows/${arch}..."
  unzip -qo "$archive_path" -d "$TMP_DIR"

  local found
  found="$(find "$TMP_DIR" -type f -name 'frpc.exe' | head -n 1)"
  if [ -z "$found" ]; then
    echo "  警告：压缩包中未找到 frpc.exe。" >&2
    return 0
  fi

  cp "$found" "$bin_dir/$bin_name"
  rm -rf "$TMP_DIR"/frp_*
  echo "  完成 -> $bin_dir/$bin_name"
}

echo "开始下载全平台 frpc ($RELEASE_TAG)..."

for arch in amd64 arm64; do
  download_unix "darwin" "$arch"
  download_unix "linux" "$arch"
done

download_windows "amd64"

printf '%s\n' "$RELEASE_TAG" > "$VERSION_FILE"

echo ""
echo "frpc 全平台安装完成"
echo "版本: $RELEASE_TAG"
echo "路径: $TOOLS_DIR/{darwin,linux,windows}/"
