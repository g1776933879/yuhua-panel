# 羽化面板专属适配器逆向规格（从 v1.1.2 amd64 二进制提取）

## 证据等级说明
confirmed = 二进制字符串+类型符号双重印证 | probable = 强单面证据

## 1. qx — 千寻微信 Pro（adapters/qx/main.go 单文件）
### 配置项（confirmed，前端 BotsView 原文）
qx.enable / qx.http_api(默认 http://127.0.0.1:7777/qianxun/httpapi) / qx.safe_key /
qx.webhook_key / qx.bot_wxid / qx.auto_add_friend / qx.auto_add_friend_duration /
qx.reply_at / qx.debug / qx.admin_only
### 本面板暴露路由（confirmed）
GET /qx/webhook?key=<qx.webhook_key> — 接收千寻框架回调（receiveWebhook）
### 千寻侧 HTTP API（probable，QianXun HTTPAPI 插件标准接口）
- 发文本: POST {http_api}/sendtext?wxid=<bot>&to_wxid=<to>&msg=<text>
- 发图片: POST {http_api}/sendimage?wxid=...&imageurl=...
- 通用动作参数在 safe_key 校验（qianxunResponse/qianxunEvent/qianxunMsgData 结构体 confirmed）
### 协议要点（confirmed by Go symbols）
qianxunEvent/qianxunMsgData/qianxunResponse 结构体；CQ 码解析(parseCQParams/decodeCQValue)；
emoji↔unicode 转义；支付消息去重(newPayDedup/parsePayXML)；媒体落盘(qianxunMediaPath)

## 2. kpeng — 鲲鹏微信机器人（adapters/kpeng/main.go 单文件）
### 配置项（confirmed）
kpeng.enable / kpeng.mode(http|ws) / kpeng.http_api(默认 http://127.0.0.1:2022/KP) /
kpeng.http_key / kpeng.webhook_key / kpeng.ws_url(默认 ws://127.0.0.1:2023) / kpeng.bot_wxid /
kpeng.auto_add_friend(+duration) / kpeng.reply_at / kpeng.debug / kpeng.admin_only
### 路由（confirmed）
GET /kpeng/webhook?key=<kpeng.webhook_key>
### 协议要点（confirmed by Go symbols）
computeSign（签名校验，http_key 参与）；双模式 HTTP/WS；
kpengResponse 结构；CQ 码解析；emoji 转换(kpEmojiToUnicode/kpUnicodeToEmoji)；
支付 XML 解析(parsePayMsg/parsePayXML/normalizeMoney)；媒体下载与公共 URL(媒体外链/本地双路径)

## 3. qw — 企业微信 AiBot（adapters/qw/main.go 单文件）
### 配置项（confirmed）
qw.enable / qw.bot_id / qw.secret / qw.hostname / qw.debug / qw.admin_only
### 协议要点（confirmed by Go symbols）
企业微信智能机器人（WeCom AiBot）协议：wecomClient(run 长连接/轮询) + convertWecomMessage；
markdown 转义(wecomMarkdownEscape)；媒体源处理(wecomMediaSource/SaveLocalImage)；
需要公网 hostname 提供媒体回链(wecomPublicHostname)

## 实现路线（建议顺序）
1. 参考 adapters/web + adapters/qq 的 Adapter 生命周期骨架（Register/Start/Stop/Destroy + srpc）
2. 先做 qx（协议最简单：单向 HTTP 回调 + HTTPAPI 发送）
3. 再做 kpeng（多一套 ws 模式 + 签名）
4. 最后 qw（企业微信 SDK 语义，字段映射最多）
5. 前端 BotsView.vue 已有配置表单（三大平台 UI 已在 sillyGirl 开源版中缺位，需按上方配置项补齐）

## 风险提示
发送侧字段名（robot_wxid 等）基于千寻/鲲鹏生态通用约定（probable），
首个实机联调时打开 qx.debug/kpeng.debug 对拍日志修正。
