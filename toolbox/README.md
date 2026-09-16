# 羽化面板工具箱（二开伴生工具）

面板本体为闭源二进制，本目录提供伴随部署工具，与羽化面板共存于同一 Docker 环境。

## 1. multi-env.sh — 多容器 Ubuntu 环境管理器
创建多个独立 Ubuntu 容器，自动安装依赖，可部署任意项目：

```bash
bash toolbox/multi-env.sh create myapp python    # 创建容器并装 Python 环境
bash toolbox/multi-env.sh deploy myapp https://github.com/user/proj.git  # 部署项目
bash toolbox/multi-env.sh exec myapp             # 进入容器
bash toolbox/multi-env.sh list                   # 列出所有容器
bash toolbox/multi-env.sh rm myapp               # 删除容器（数据目录保留）
```
- 预设: `base`(git/curl) `python`(python3+pip) `node`
- 数据挂载: `/root/yuhua-toolbox/<名字>` <-> 容器 `/workspace`

## 2. plugin-download.sh — 插件下载器
```bash
bash toolbox/plugin-download.sh list             # 列出官方插件
bash toolbox/plugin-download.sh get 美团领券PLUS.py   # 下载单个
bash toolbox/plugin-download.sh all              # 下载全部
# 从自己仓库下载:
YUHUA_PLUGIN_REPO=g1776933879/yuhua-panel bash toolbox/plugin-download.sh all
```

## 3. ZeroTermux 一键部署（双 Python 可选）
```bash
bash toolbox/yuhua-termux-install.sh          # 交互式: 选 3.12/3.14 共存
PYBIN=python3.14 ~/yuhua-panel/start.sh       # 运行时切版本
USE_PROOT=1 ~/yuhua-panel/start.sh            # SIGSYS 兜底模式
```

## 4. 组合编排示例（docker-compose.yml）
面板 + 多个业务容器统一管理，参考仓库根目录 `docker-compose.yml`。
