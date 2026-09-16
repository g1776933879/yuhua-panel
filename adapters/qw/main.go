package qw

// ============================================================
// qw — 企业微信 AiBot 适配器（青玄面板二开版）
// 协议规格来源: docs/ADAPTERS-REVERSE.md（羽化 v1.1.2 二进制逆向）
// 通道(confirmed): wss://openws.work.weixin.qq.com 长连接收消息
//                  aibot_send_msg 发送 / aibot_subscribe 订阅 / aibot_msg_callback 回调
// 配置(confirmed): qw.enable / qw.bot_id / qw.secret / qw.hostname / qw.debug / qw.admin_only
// hostname 用于媒体公网回链（wecomPublicHostname）
// ============================================================

import (
"bytes"
"encoding/json"
"fmt"
"io"
"net/http"
"strings"
"sync"
"time"

"github.com/gin-gonic/gin"
"github.com/gorilla/websocket"
"github.com/smallfawn/sillyGirl/core"
)

var qw = core.MakeBucket("qw")

var (
adapter     *core.Factory
adapterOnce sync.Once
wsConn      *websocket.Conn
wsMu        sync.Mutex
)

const wecomWSGateway = "wss://openws.work.weixin.qq.com" // 逆向证实

// ---------- 企微 AiBot 消息帧（字段按官方智能机器人协议，联调可 debug 对拍） ----------
type wecomFrame struct {
Cmd     int             `json:"cmd"`               // 帧类型: 0=连接确认 1=消息
Headers map[string]string `json:"headers"`
Body    json.RawMessage `json:"body"`
}

type wecomMsg struct {
MsgID    string `json:"msgid"`
ChatID   string `json:"chatid"`   // 会话 id
ChatType string `json:"chattype"` // single/group
From     struct {
UserID string `json:"userid"`
} `json:"from"`
MsgType string `json:"msgtype"` // text/image/mixed...
Text    struct {
Content string `json:"content"`
} `json:"text"`
}

// ---------- 适配器 ----------
func initQWBot() {
adapterOnce.Do(func() {
adapter = &core.Factory{}
adapter.Init("qw", qw.GetString("bot_id"), nil)
adapter.Send(func(msg map[string]interface{}) string {
userID := fmt.Sprint(msg[core.USER_ID])
content := fmt.Sprint(msg[core.CONETNT])
sendToWecom(userID, content)
return ""
})
})
}

func qwDebug(format string, args ...interface{}) {
if qw.GetBool("debug") {
fmt.Printf("[qw:debug] "+format+"\n", args...)
}
}

// ---------- 发送（aibot_send_msg，经企业微信回调域名代理 or 直发） ----------
func sendToWecom(chatID, content string) {
botID := strings.TrimSpace(qw.GetString("bot_id"))
hostname := strings.TrimSpace(qw.GetString("hostname"))
payload := map[string]interface{}{
"action": "aibot_send_msg", // 逆向证实发送动作
"bot_id": botID,
"chatid": chatID,
"msg": map[string]interface{}{
"msgtype": "text",
"text":    map[string]string{"content": content},
},
}
// 公网媒体回链（wecomPublicHostname 语义）：文本中的相对路径补全
if hostname != "" {
payload["public_hostname"] = hostname
}
body, _ := json.Marshal(payload)
qwDebug("发送: %s", string(body))
// AiBot 发送默认走 ws 在线通道；离线时可切 HTTP 回调
wsMu.Lock()
conn := wsConn
wsMu.Unlock()
if conn != nil {
if err := conn.WriteJSON(payload); err != nil {
qwDebug("WS 发送失败: %v", err)
}
return
}
// WS 不在线: POST 到企微回调代理（aibot_msg_callback 语义）
go func(b []byte) {
req, err := http.NewRequest(http.MethodPost, "https://work.weixin.qq.com/aibot/send_msg", bytes.NewReader(b))
if err != nil {
return
}
req.Header.Set("Content-Type", "application/json")
if secret := strings.TrimSpace(qw.GetString("secret")); secret != "" {
req.Header.Set("X-AIBot-Secret", secret)
}
resp, err := http.DefaultClient.Do(req)
if err != nil {
qwDebug("HTTP 发送失败: %v", err)
return
}
defer resp.Body.Close()
io.Copy(io.Discard, io.LimitReader(resp.Body, 1<<16))
}(body)
}

// ---------- WS 接收主循环 ----------
func runQWWS() {
for {
if !qw.GetBool("enable") {
time.Sleep(3 * time.Second)
continue
}
header := http.Header{}
if bid := strings.TrimSpace(qw.GetString("bot_id")); bid != "" {
header.Set("X-Bot-ID", bid)
}
if sec := strings.TrimSpace(qw.GetString("secret")); sec != "" {
header.Set("X-Bot-Secret", sec)
}
conn, _, err := websocket.DefaultDialer.Dial(wecomWSGateway, header)
if err != nil {
qwDebug("WS 连接失败: %v", err)
time.Sleep(5 * time.Second)
continue
}
wsMu.Lock()
wsConn = conn
wsMu.Unlock()
fmt.Println("[qw] 企微 AiBot WS 已连接")
for {
var frame wecomFrame
if err := conn.ReadJSON(&frame); err != nil {
qwDebug("WS 读取失败: %v", err)
break
}
handleFrame(frame)
}
wsMu.Lock()
wsConn = nil
wsMu.Unlock()
time.Sleep(3 * time.Second)
}
}

func handleFrame(frame wecomFrame) {
if frame.Cmd != 1 || len(frame.Body) == 0 {
qwDebug("控制帧 cmd=%d", frame.Cmd)
return
}
var m wecomMsg
if err := json.Unmarshal(frame.Body, &m); err != nil {
qwDebug("帧解析失败: %v", err)
return
}
initQWBot()
content := strings.TrimSpace(m.Text.Content)
if content == "" {
return
}
from := m.ChatID
if from == "" {
from = m.From.UserID
}
qwDebug("收到消息 chat=%s from=%s text=%s", m.ChatID, m.From.UserID, content)
adapter.Push(map[string]string{
core.USER_ID: from,
core.CONETNT: content,
})
}

// ---------- HTTP 回调（aibot_msg_callback，企业微信侧也可主动推送） ----------
func msgCallback(ctx *gin.Context) {
body, err := io.ReadAll(io.LimitReader(ctx.Request.Body, 1<<20))
if err != nil {
ctx.JSON(http.StatusBadRequest, gin.H{"error": "read body"})
return
}
qwDebug("callback 原文: %s", string(body))
var m wecomMsg
if err := json.Unmarshal(body, &m); err != nil {
ctx.JSON(http.StatusOK, gin.H{"errcode": 0})
return
}
initQWBot()
content := strings.TrimSpace(m.Text.Content)
if content != "" {
from := m.ChatID
if from == "" {
from = m.From.UserID
}
adapter.Push(map[string]string{
core.USER_ID: from,
core.CONETNT: content,
})
}
ctx.JSON(http.StatusOK, gin.H{"errcode": 0})
}

func init() {
go func() {
time.Sleep(time.Second)
if qw.GetBool("enable") {
initQWBot()
go runQWWS()
}
}()
core.GinApi(core.POST, "/qw/callback", msgCallback)
}
