#!/data/data/com.termux/files/usr/bin/bash
# ============================================
# yuhua-termux-install.sh — 羽化面板 ZeroTermux 原生部署器
# 特性: Python 3.12 / 3.14 可选共存，自动处理 LD_PRELOAD/SSL 证书/proot 兜底
# 用法: bash yuhua-termux-install.sh
# ============================================
set -eu

INSTALL_DIR=${INSTALL_DIR:-$HOME/yuhua-panel}
MIRROR=${GITHUB_PROXY:-https://ghproxy.net}
REPO=${RELEASE_REPO:-yuhualhh/yuhua-panel}

banner() { echo "\n===== 羽化面板 ZeroTermux 部署器 ====="; }

# ---------- 1. 环境检测 ----------
banner
echo "[1/6] 检测环境..."
command -v pkg >/dev/null || { echo "❌ 请在 Termux/ZeroTermux 中运行"; exit 1; }
ARCH=$(uname -m); echo "  架构: $ARCH"

# ---------- 2. 安装基础依赖 ----------
echo "[2/6] 安装基础依赖..."
pkg install -y python python-pip openssl curl tar proot nodejs-lts 2>/dev/null || \
  apt install -y python python-pip openssl curl tar proot nodejs-lts

# ---------- 3. Python 版本选择 ----------
echo "\n[3/6] Python 环境选择（多版本共存）"
PY312=$(command -v python3.12 || true)
PY314=$(command -v python3.14 || true)
[ -z "$PY312" ] && echo "  未检测到 python3.12" && read -rp "  是否安装 python 3.12? [y/N]: " A12
[ -z "$PY314" ] && echo "  未检测到 python3.14" && read -rp "  是否安装 python 3.14? [y/N]: " A14
# ZeroTermux 安装特定版本: pkg install python3.12 / python3.14（仓库有对应包名时）
if [ "${A12:-}" = "y" ] && [ -z "$PY312" ]; then
  pkg search python 2>/dev/null | grep -q python3.12 && pkg install -y python3.12 || \
    echo "  ⚠️ 仓库无 python3.12 包，稍后用 uv/pyenv 方式兜底"
  PY312=$(command -v python3.12 || true)
fi
if [ "${A14:-}" = "y" ] && [ -z "$PY314" ]; then
  pkg search python 2>/dev/null | grep -q python3.14 && pkg install -y python3.14 || \
    echo "  ⚠️ 仓库无 python3.14 包，稍后用 uv/pyenv 方式兜底"
  PY314=$(command -v python3.14 || true)
fi
echo "  可用版本: ${PY312:+3.12 }${PY314:+3.14}"
if [ -n "$PY312" ] && [ -n "$PY314" ]; then
  read -rp "  插件默认使用哪个版本? [12/14，回车=12]: " CHOICE
  [ "$CHOICE" = "14" ] && PYBIN=python3.14 || PYBIN=python3.12
else
  PYBIN=$([ -n "$PY314" ] && echo python3.14 || echo python3.12)
fi
echo "  ✓ 默认插件解释器: $PYBIN"

# uv 兜底（Termux 仓库缺版本时用 uv 装指定 Python）
if ! command -v $PYBIN >/dev/null; then
  echo "  → 用 uv 安装 $PYBIN ..."
  pip install -q uv 2>/dev/null || pipx install uv
  uv python install "$([ "$PYBIN" = python3.14 ] && echo 3.14 || echo 3.12)"
  UV_PY=$(uv python find "$([ "$PYBIN" = python3.14 ] && echo 3.14 || echo 3.12)")
  ln -sf "$UV_PY" "$PREFIX/bin/$PYBIN" 2>/dev/null || true
fi

# ---------- 4. 下载面板主程序 ----------
echo "[4/6] 获取面板主程序..."
mkdir -p "$INSTALL_DIR"; cd "$INSTALL_DIR"
if [ -x ./yuhua-panel ]; then
  echo "  已存在主程序，跳过（删除 $INSTALL_DIR/yuhua-panel 可强制重下）"
else
  TAG=$(curl -fsSL --max-time 30 "$MIRROR/https://raw.githubusercontent.com/$REPO/main/VERSION" | tr -d ' \r\n')
  case "$ARCH" in aarch64|arm64) AA=arm64;; *) AA=amd64;; esac
  URL="$MIRROR/https://github.com/$REPO/releases/download/v$TAG/yuhua-panel-linux-$AA-v$TAG.tar.gz"
  echo "  下载: $URL"
  curl -fSL --max-time 600 --retry 2 "$URL" -o /tmp/panel.tgz
  tar -xzf /tmp/panel.tgz -C . && rm -f /tmp/panel.tgz
  F=$(find . -maxdepth 1 -name 'yuhua-panel-*' | head -1); [ -n "$F" ] && mv -f "$F" ./yuhua-panel
  chmod +x ./yuhua-panel
fi

# ---------- 5. 数据目录与插件环境 ----------
echo "[5/6] 初始化数据目录（双 Python 环境）..."
mkdir -p "$INSTALL_DIR/data/plugins/python_packages"
export PIPX_HOME="$INSTALL_DIR/data/plugins/python_packages"
export PIPX_BIN_DIR="$PIPX_HOME/bin"
# 用选定的 Python 初始化 proto3 路径
export PYTHONPATH="$INSTALL_DIR/proto3"
$PYBIN -m pip install -q --upgrade pip 2>/dev/null || true

# ---------- 6. 生成启动脚本 ----------
echo "[6/6] 生成启动脚本（含 Termux 兼容修复）..."
cat > "$INSTALL_DIR/start.sh" << LAUNCH
#!/data/data/com.termux/files/usr/bin/bash
cd "$INSTALL_DIR"
# Termux 兼容三件套:
# 1) TLS 证书链（Go 程序必需）
export SSL_CERT_FILE="\$PREFIX/etc/tls/cert.pem"
export SSL_CERT_DIR="\$PREFIX/etc/tls/certs"
# 2) 数据目录重定向
export SILLYGIRL_DATA_PATH="$INSTALL_DIR/data"
export SILLYGIRL_NODE_PATH="$PREFIX/lib/node_modules"
# 3) Python 双版本选择（启动时可覆盖: PYBIN=python3.14 ./start.sh）
export SILLYGIRL_PYTHON_BIN="\${PYBIN:-$PYBIN}"
export SILLYGIRL_PYTHON_PATH="$INSTALL_DIR/proto3"
export PYTHONPATH="$INSTALL_DIR/proto3"
export PIPX_HOME="$PIPX_HOME"; export PIPX_BIN_DIR="$PIPX_BIN_DIR"
export NODE_PATH="\$SILLYGIRL_NODE_PATH"
# 4) 免疫 termux-exec LD_PRELOAD（Go GC worker SIGSEGV 修复）
if [ "\${USE_PROOT:-0}" = "1" ]; then
  echo "[启动] proot 模式（SIGSYS 兜底）..."
  exec proot -0 "\$INSTALL_DIR/yuhua-panel"
else
  echo "[启动] 原生模式（PYBIN=\$SILLYGIRL_PYTHON_BIN）..."
  env --unset=LD_PRELOAD nohup ./yuhua-panel > panel.log 2>&1 &
  echo "  PID: \$!  日志: tail -f $INSTALL_DIR/panel.log"
  echo "  停止: pkill -x yuhua-panel"
fi
LAUNCH
chmod +x "$INSTALL_DIR/start.sh"

echo "\n✅ 部署完成!"
echo "  启动:   ~/yuhua-panel/start.sh"
echo "  切版本: PYBIN=python3.14 ~/yuhua-panel/start.sh"
echo "  proot:  USE_PROOT=1 ~/yuhua-panel/start.sh  (SIGSYS 时用)"
echo "  访问:   http://127.0.0.1:6060/admin"
