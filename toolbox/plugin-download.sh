#!/usr/bin/env bash
# ============================================
# plugin-download.sh — 羽化面板插件下载器
# 从 GitHub 仓库的 plugins/ 目录下载插件到本地
# 支持任意仓库（默认官方 yuhualhh/yuhua-panel）
# ============================================
set -euo pipefail

REPO=${YUHUA_PLUGIN_REPO:-yuhualhh/yuhua-panel}
BRANCH=${YUHUA_PLUGIN_BRANCH:-main}
MIRROR=${YUHUA_MIRROR:-https://ghproxy.net}   # GitHub 镜像，直连可设为空
DEST=${YUHUA_PLUGIN_DIR:-./plugins-downloaded}
mkdir -p "$DEST"

raw_url() { echo "${MIRROR:+$MIRROR/}https://raw.githubusercontent.com/$REPO/$BRANCH/$1"; }

list_plugins() {
  local api="https://api.github.com/repos/$REPO/contents/plugins?ref=$BRANCH"
  curl -fsSL -H "User-Agent: yuhua-toolbox" "$api" | grep -o '"name": *"[^"]*\.py"' | sed 's/.*"\(.*\.py\)"/\1/'
}

case "${1:-}" in
  list) list_plugins ;;
  all)
    echo "下载 $REPO 全部插件到 $DEST ..."
    for f in $(list_plugins); do curl -fsSL "$(raw_url "plugins/$f")" -o "$DEST/$f" && echo "  ✓ $f"; done
    echo "完成: $(ls "$DEST" | wc -l) 个插件"
    ;;
  get)
    shift
    for f in "$@"; do
      curl -fsSL "$(raw_url "plugins/$f")" -o "$DEST/$f" && echo "✓ $f -> $DEST/$f" || echo "✗ $f 下载失败"
    done
    ;;
  *)
  cat <<EOF
用法:
  plugin-download.sh list            列出远程仓库可用插件
  plugin-download.sh all             下载全部插件
  plugin-download.sh get 插件名.py   下载指定插件（可多个）
环境变量:
  YUHUA_PLUGIN_REPO=yuhualhh/yuhua-panel  换源仓库
  YUHUA_MIRROR=https://ghproxy.net        GitHub镜像（空=直连）
  YUHUA_PLUGIN_DIR=./plugins-downloaded   本地保存目录
示例:
  YUHUA_PLUGIN_REPO=g1776933879/yuhua-panel plugin-download.sh get 美团领券PLUS.py
EOF
  ;;
esac
