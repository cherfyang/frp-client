#!/usr/bin/env bash

set -euo pipefail

ROOT_DIR="$(cd "$(dirname "$0")" && pwd)"
TOOLS_DIR="$ROOT_DIR/.tools/frp"
BIN_DIR="$TOOLS_DIR/bin"
VERSION_FILE="$TOOLS_DIR/VERSION"
INSTALL_BIN="$BIN_DIR/frpc"
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

resolve_platform() {
  case "$(uname -s)" in
    Darwin)
      printf '%s\n' "darwin"
      ;;
    Linux)
      printf '%s\n' "linux"
      ;;
    *)
      echo "暂不支持当前系统: $(uname -s)" >&2
      exit 1
      ;;
  esac
}

resolve_arch() {
  case "$(uname -m)" in
    x86_64|amd64)
      printf '%s\n' "amd64"
      ;;
    arm64|aarch64)
      printf '%s\n' "arm64"
      ;;
    *)
      echo "暂不支持当前架构: $(uname -m)" >&2
      exit 1
      ;;
  esac
}

require_command curl
require_command tar
require_command node

PLATFORM="$(resolve_platform)"
ARCH="$(resolve_arch)"

if [ -n "${FRPC_VERSION:-}" ]; then
  case "$FRPC_VERSION" in
    v*)
      RELEASE_TAG="$FRPC_VERSION"
      ;;
    *)
      RELEASE_TAG="v$FRPC_VERSION"
      ;;
  esac
  RELEASE_API_URL="https://api.github.com/repos/fatedier/frp/releases/tags/$RELEASE_TAG"
else
  RELEASE_API_URL="https://api.github.com/repos/fatedier/frp/releases/latest"
fi

echo "正在查询 frp 官方发布信息..."
RELEASE_JSON="$(curl -fsSL "$RELEASE_API_URL")"

RELEASE_META="$(
  printf '%s' "$RELEASE_JSON" \
    | node -e '
      let input = "";
      process.stdin.on("data", (chunk) => {
        input += chunk;
      });
      process.stdin.on("end", () => {
        const release = JSON.parse(input);
        const platform = process.argv[1];
        const arch = process.argv[2];
        const asset = release.assets.find((item) =>
          new RegExp(`frp_[^/]+_${platform}_${arch}\\.tar\\.gz$`).test(item.browser_download_url),
        );

        if (!asset) {
          console.error(`未找到匹配 ${platform}/${arch} 的 frpc 安装包。`);
          process.exit(2);
        }

        process.stdout.write(`${release.tag_name}\n${asset.browser_download_url}\n`);
      });
    ' "$PLATFORM" "$ARCH"
)"

RELEASE_TAG="$(printf '%s\n' "$RELEASE_META" | sed -n '1p')"
DOWNLOAD_URL="$(printf '%s\n' "$RELEASE_META" | sed -n '2p')"

mkdir -p "$BIN_DIR"

if [ -x "$INSTALL_BIN" ] && [ -f "$VERSION_FILE" ] && [ "$(cat "$VERSION_FILE")" = "$RELEASE_TAG" ]; then
  echo "frpc 已安装，版本: $RELEASE_TAG"
  echo "可执行文件: $INSTALL_BIN"
  exit 0
fi

ARCHIVE_PATH="$TMP_DIR/frp.tar.gz"

echo "正在下载 $RELEASE_TAG ($PLATFORM/$ARCH)..."
curl -fL "$DOWNLOAD_URL" -o "$ARCHIVE_PATH"

echo "正在解压 frpc..."
tar -xzf "$ARCHIVE_PATH" -C "$TMP_DIR"

FOUND_BIN="$(find "$TMP_DIR" -type f -name 'frpc' | head -n 1)"
if [ -z "$FOUND_BIN" ]; then
  echo "压缩包中未找到 frpc 可执行文件。" >&2
  exit 1
fi

cp "$FOUND_BIN" "$INSTALL_BIN"
chmod +x "$INSTALL_BIN"
printf '%s\n' "$RELEASE_TAG" > "$VERSION_FILE"

echo "frpc 安装完成"
echo "版本: $RELEASE_TAG"
echo "路径: $INSTALL_BIN"
