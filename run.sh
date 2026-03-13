#!/usr/bin/env bash

set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "$ROOT_DIR"

if ! command -v pnpm >/dev/null 2>&1; then
  if command -v corepack >/dev/null 2>&1; then
    echo "pnpm 未找到，尝试使用 corepack 启用 pnpm..."
    corepack enable pnpm >/dev/null 2>&1 || true
  fi
fi

if ! command -v pnpm >/dev/null 2>&1; then
  echo "未检测到 pnpm，请先安装 pnpm 后重试。"
  exit 1
fi

if [ ! -d node_modules ]; then
  echo "未检测到 node_modules，正在安装依赖..."
  pnpm install
fi

echo "启动 frpc.toml 配置页：http://0.0.0.0:6633"
pnpm run build
exec env PORT=6633 HOST=0.0.0.0 pnpm run serve
