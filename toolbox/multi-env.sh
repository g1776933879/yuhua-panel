#!/usr/bin/env bash
# ============================================
# multi-env.sh — 多容器 Ubuntu 环境管理器
# 用途：创建/管理多个独立 Ubuntu 容器，自动安装依赖，可部署任意项目
# 用法见底部 usage()
# ============================================
set -euo pipefail

WORKDIR=${YUHUA_TOOLBOX_DIR:-/root/yuhua-toolbox}
mkdir -p "$WORKDIR"

# 依赖预设：可自定义，逗号分隔
PRESET_PYTHON="python3 python3-pip python3-venv"
PRESET_NODE="curl ca-certificates"
PRESET_BASE="curl ca-certificates git unzip tzdata"

need_docker() {
  command -v docker >/dev/null 2>&1 || { echo "[错误] 未安装 docker"; exit 1; }
}

# 创建容器：multi-env create <名字> [预设:base|python|node] [宿主机挂载目录]
cmd_create() {
  local name=$1 preset=${2:-base} hostdir=${3:-$WORKDIR/$name}
  need_docker
  mkdir -p "$hostdir"
  local pkgs="$PRESET_BASE"
  case "$preset" in
    python) pkgs="$pkgs $PRESET_PYTHON" ;;
    node)   pkgs="$pkgs $PRESET_NODE" ;;
  esac
  docker run -d --name "$name" --restart unless-stopped \
    -v "$hostdir:/workspace" -w /workspace ubuntu:24.04 sleep infinity
  echo "[OK] 容器 $name 已创建，正在安装依赖（首次较慢）..."
  docker exec "$name" bash -c "apt-get update -qq && DEBIAN_FRONTEND=noninteractive apt-get install -y -qq $pkgs"
  echo "[OK] 依赖安装完成。进入容器: docker exec -it $name bash"
  echo "     挂载目录: $hostdir <-> /workspace"
}

cmd_list() {
  echo "=== 工具箱容器 ==="
  docker ps -a --filter "name=" --format '{{.Names}}\t{{.Status}}\t{{.Image}}' | grep -E "^[a-zA-Z0-9_-]+" || true
}

cmd_exec() { docker exec -it "$1" bash; }
cmd_stop()  { docker stop "$1"; }
cmd_start() { docker start "$1"; }
cmd_rm()    { docker rm -f "$1" && echo "[OK] 已删除 $1（挂载目录保留: $WORKDIR/$1）"; }

# 在容器里部署项目：multi-env deploy <容器名> <git仓库地址或zip URL>
cmd_deploy() {
  local name=$1 url=$2
  docker exec "$name" bash -c "cd /workspace && (git clone '$url' || (curl -fsSL '$url' -o project.zip && unzip -o project.zip))"
  echo "[OK] 项目已部署到容器 $name:/workspace，进入容器查看"
}

usage() {
  cat <<EOF
用法:
  multi-env create <名字> [base|python|node] [挂载目录]  创建Ubuntu容器并装依赖
  multi-env list                                         列出所有容器
  multi-env exec <名字>                                  进入容器
  multi-env deploy <名字> <git地址或zip链接>              部署项目到容器
  multi-env stop|start|rm <名字>                         停止/启动/删除容器
示例:
  multi-env create myapp python
  multi-env deploy myapp https://github.com/user/proj.git
  multi-env exec myapp
EOF
}

case "${1:-}" in
  create) shift; cmd_create "$@" ;;
  list)   cmd_list ;;
  exec)   shift; cmd_exec "$@" ;;
  deploy) shift; cmd_deploy "$@" ;;
  stop)   shift; cmd_stop "$@" ;;
  start)  shift; cmd_start "$@" ;;
  rm)     shift; cmd_rm "$@" ;;
  *)      usage ;;
esac
