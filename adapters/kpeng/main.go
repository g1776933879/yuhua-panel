package kpeng

// ============================================================
// kpeng — 鲲鹏微信机器人适配器（青玄面板二开版）
// 协议规格来源: docs/ADAPTERS-REVERSE.md（羽化 v1.1.2 二进制逆向）
// 双模式: http(POST {http_api}, X-KP-Key 头) / ws({ws_url} 长连接)
// 回调: GET|POST /kpeng/webhook?key=<kpeng.webhook_key>
// 消息字段(逆向证实): robotWxid/from_wxid/from_name/roomid/nickname/
//   msg/event_type/msgId/final_from_wxid/final_from_name/sign/timestamp/nonce
// 动作码(逆向证实): K10030/K10033/K10034/K10035/K10043 (+K10005 文本族)
// 响应包装(逆向证实): {"result": ...}
// ============================================================

import (
	"bytes"
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"github.com/smallfawn/sillyGirl/core"
)

var kpeng = core.MakeBucket("kpeng")

var (
	adapter     *core.Factory
	adapterOnce sync.Once
	wsConn      *websocket.Conn
	wsMu        sync.Mutex
)

// ---------- 消息结构（字段名全部来自二进制反射元数据，confirmed） ----------
type kpengEvent struct {
	EventType     string `json:"event_type"` // 事件/动作码: K10005 文本 等
	RobotWxid     string `json:"robotWxid"`  // 机器人 wxid
	FromWxid      string `json:"from_wxid"`  // 来源 wxid/群 id
	FromName      string `json:"from_name"`  // 来源昵称
	ToWxid        string `json:"to_wxid"`    // 目标
	Roomid        string `json:"roomid"`     // 群聊房间 id（非空=群消息）
	Nickname      string `json:"nickname"`
	Msg           string `json:"msg"` // 消息内容（含 CQ 码）
	MsgID         string `json:"msgId"`
	Sign          string `json:"sign"` // 签名
	Timestamp     int64  `json:"timestamp"`
	Nonce         string `json:"nonce"`
	FinalFromWxid string `json:"final_from_wxid"` // 群内实际发送人（逆向证实）
	FinalFromName string `json:"final_from_name"`
}

type kpengResponse struct {
	Result interface{} `json:"result"`
}

// ---------- CQ 码（与 qx 同源逻辑） ----------
var (
	kpCQImagePattern = regexp.MustCompile(`\[CQ:image,file=([^,\]]+),url=([^\]]+)\]`)
	kpCQAtPattern    = regexp.MustCompile(`\[CQ:at,qq=([^\]]+)\]`)
	kpCQAllPattern   = regexp.MustCompile(`\[CQ:[a-z]+[^\]]*\]`)
)

func decodeCQValue(v string) string {
	return strings.NewReplacer("&amp;", "&", "&#91;", "[", "&#93;", "]", "&#44;", ",").Replace(v)
}

func stripCQ(content string) string {
	content = kpCQImagePattern.ReplaceAllString(content, "")
	content = kpCQAtPattern.ReplaceAllString(content, "@$1")
	return strings.TrimSpace(kpCQAllPattern.ReplaceAllString(content, ""))
}

// ---------- 签名（computeSign: 参数字典序 + key 拼 MD5，联调期可在 debug 对拍修正） ----------
func computeSign(params map[string]string, key string) string {
	keys := make([]string, 0, len(params))
	for k := range params {
		if k == "sign" || k == "" {
			continue
		}
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var b strings.Builder
	for _, k := range keys {
		b.WriteString(k)
		b.WriteString("=")
		b.WriteString(params[k])
		b.WriteString("&")
	}
	b.WriteString("key=")
	b.WriteString(key)
	hash := md5.Sum([]byte(b.String()))
	return hex.EncodeToString(hash[:])
}

// ---------- 配置 ----------
func kpHTTPAPI() string {
	base := strings.TrimSpace(kpeng.GetString("http_api"))
	if base == "" {
		base = "http://127.0.0.1:2022/KP"
	}
	return strings.TrimRight(base, "/")
}

func kpWSURL() string {
	u := strings.TrimSpace(kpeng.GetString("ws_url"))
	if u == "" {
		u = "ws://127.0.0.1:2023"
	}
	return u
}

func kpDebug(format string, args ...interface{}) {
	if kpeng.GetBool("debug") {
		fmt.Printf("[kpeng:debug] "+format+"\n", args...)
	}
}

// ---------- 适配器初始化 ----------
func initKpengBot() {
	adapterOnce.Do(func() {
		adapter = &core.Factory{}
		adapter.Init("kpeng", kpeng.GetString("bot_wxid"), nil)
		// 面板/插件 → 鲲鹏发送
		adapter.Send(func(msg map[string]interface{}) string {
			userID := fmt.Sprint(msg[core.USER_ID])
			content := fmt.Sprint(msg[core.CONETNT])
			if kpeng.GetString("mode") == "ws" {
				sendViaWS(userID, content)
			} else {
				sendViaHTTP(userID, content)
			}
			return ""
		})
	})
}

// ---------- HTTP 发送 ----------
func kpHTTPCall(action string, payload map[string]interface{}) (*kpengResponse, error) {
	body, _ := json.Marshal(payload)
	req, err := http.NewRequest(http.MethodPost, kpHTTPAPI()+action, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	if key := strings.TrimSpace(kpeng.GetString("http_key")); key != "" {
		req.Header.Set("X-KP-Key", key) // 逆向证实: X-KP-Key 请求头
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, err
	}
	kpDebug("HTTP %s %s -> %s", action, payload, string(data))
	out := &kpengResponse{}
	if err := json.Unmarshal(data, out); err != nil {
		return nil, fmt.Errorf("鲲鹏响应解析失败: %w", err)
	}
	return out, nil
}

// sendViaHTTP: 文本走 /sendText（K10005 族），图片走 /sendImage（K10033 族）
func sendViaHTTP(toWxid, content string) {
	botWxid := strings.TrimSpace(kpeng.GetString("bot_wxid"))
	remaining := content
	for _, m := range kpCQImagePattern.FindAllStringSubmatch(content, -1) {
		imgURL := decodeCQValue(m[2])
		_, _ = kpHTTPCall("/sendImage", map[string]interface{}{
			"wxid":    botWxid,
			"to_wxid": toWxid,
			"url":     imgURL,
		})
		remaining = strings.Replace(remaining, m[0], "", 1)
	}
	text := strings.TrimSpace(stripCQ(remaining))
	if text == "" {
		return
	}
	_, _ = kpHTTPCall("/sendText", map[string]interface{}{
		"wxid":    botWxid,
		"to_wxid": toWxid,
		"msg":     text,
	})
}

// ---------- WS 发送 ----------
func sendViaWS(toWxid, content string) {
	wsMu.Lock()
	defer wsMu.Unlock()
	if wsConn == nil {
		kpDebug("WS 未连接，消息丢弃: %s", content)
		return
	}
	payload := map[string]interface{}{
		"action": "K10005", // 文本发送动作码（逆向证实族）
		"params": map[string]string{
			"to_wxid": toWxid,
			"msg":     stripCQ(content),
		},
	}
	if err := wsConn.WriteJSON(payload); err != nil {
		kpDebug("WS 写入失败: %v", err)
	}
}

// ---------- WS 接收循环 ----------
func runWS() {
	headURL := kpWSURL()
	for {
		if !kpeng.GetBool("enable") {
			time.Sleep(3 * time.Second)
			continue
		}
		conn, _, err := websocket.DefaultDialer.Dial(headURL, nil)
		if err != nil {
			kpDebug("WS 连接失败: %v", err)
			time.Sleep(5 * time.Second)
			continue
		}
		wsMu.Lock()
		wsConn = conn
		wsMu.Unlock()
		fmt.Println("[kpeng] WS 已连接", headURL)
		for {
			var ev kpengEvent
			if err := conn.ReadJSON(&ev); err != nil {
				kpDebug("WS 读取失败: %v", err)
				break
			}
			dispatchEvent(ev)
		}
		wsMu.Lock()
		wsConn = nil
		wsMu.Unlock()
		time.Sleep(3 * time.Second)
	}
}

// ---------- 消息分发（HTTP webhook 与 WS 共用） ----------
func dispatchEvent(ev kpengEvent) {
	initKpengBot()
	content := decodeCQValue(ev.Msg)
	text := strings.TrimSpace(stripCQ(content))
	if text == "" {
		return
	}
	// 群消息以 roomid 为会话，私聊以来源 wxid
	from := ev.FromWxid
	if ev.Roomid != "" {
		from = ev.Roomid
	}
	if from == "" {
		from = ev.FinalFromWxid
	}
	kpDebug("收到消息 from=%s room=%s text=%s", ev.FromWxid, ev.Roomid, text)
	adapter.Push(map[string]string{
		core.USER_ID: from,
		core.CONETNT: text,
	})
}

// ---------- webhook 鉴权 ----------
func checkWebhookKey(ctx *gin.Context) bool {
	want := strings.TrimSpace(kpeng.GetString("webhook_key"))
	if want == "" {
		return true
	}
	got := ctx.Query("key")
	if got == "" {
		got = ctx.GetHeader("X-Webhook-Key")
	}
	return got == want
}

// ---------- webhook 接收 ----------
func receiveWebhook(ctx *gin.Context) {
	if !checkWebhookKey(ctx) {
		ctx.JSON(http.StatusForbidden, gin.H{"error": "invalid key"})
		return
	}
	body, err := io.ReadAll(io.LimitReader(ctx.Request.Body, 1<<20))
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "read body"})
		return
	}
	kpDebug("webhook 原文: %s", string(body))
	var ev kpengEvent
	if err := json.Unmarshal(body, &ev); err != nil {
		ctx.JSON(http.StatusOK, gin.H{"code": 0})
		return
	}
	dispatchEvent(ev)
	ctx.JSON(http.StatusOK, gin.H{"code": 0, "msg": "ok"})
}

func init() {
	go func() {
		time.Sleep(time.Second)
		if kpeng.GetBool("enable") {
			initKpengBot()
			if kpeng.GetString("mode") == "ws" {
				go runWS()
			}
		}
	}()
	core.GinApi(core.GET, "/kpeng/webhook", receiveWebhook)
	core.GinApi(core.POST, "/kpeng/webhook", receiveWebhook)
}

var (
	_ = url.Values{} // 预留 query 签名模式
	_ = kpengResponse{}
)
