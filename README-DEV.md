# sillygirl-dev 分支 · 二开指南

## 本分支是什么
羽化面板的开源等价基座：smallfawn/sillyGirl v1.1.9 完整源码。
主仓库 main 分支保留羽化官方插件与 toolbox 工具，本分支承载二开代码。

## 已验证（2026-09-16）
- [x] Go 1.26.5 arm64 编译通过（GOPROXY=https://goproxy.cn）
- [x] 启动成功，admin(8080/admin) HTTP 200
- [x] bolt 存储 + web 机器人默认可用

## 快速开始
```bash
export PATH=/usr/local/go/bin:$PATH GOPROXY=https://goproxy.cn,direct
go build -o sillygirl .
SSL_CERT_FILE=/etc/ssl/certs/ca-certificates.crt SILLYGIRL_PORT=8080 ./sillygirl
# 后台: http://localhost:8080/admin
```

## 二开路线图
1. [ ] 对齐羽化缺失的适配器: kpeng(鲲鹏)/qx(千寻)/qw(微信) — 参考 adapters/web 结构实现
2. [ ] 插件市场功能（对应羽化 /admin 的市场页）
3. [ ] 品牌化: 前端 frontend/ (Vue3+antd) 改名替换
4. [ ] Python 3.12/3.14 共存调度（对齐 toolbox/yuhua-termux-install.sh 的环境变量约定）

## 提交规范
直接提交本分支；稳定后合入 main 或打 tag 发布二进制。
