package qx

// ============================================================
// qx — 千寻微信 Pro 适配器（青玄面板二开版）
// 协议规格来源: docs/ADAPTERS-REVERSE.md（羽化 v1.1.2 二进制逆向）
// 接收: 千寻框架 HTTP 回调 → 本面板 /qx/webhook?key=<qx.webhook_key>
// 发送: 千寻 HTTPAPI ({qx.http_api}/sendtext|sendimage, safe_key 鉴权)
// ============================================================

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/smallfawn/sillyGirl/core"
)

var qx = core.MakeBucket("qx")

var (
	adapter     *core.Factory
	adapterOnce sync.Once
)

// ---------- 千寻回调消息结构（对齐 qianxunEvent/qianxunMsgData） ----------
type qianxunEvent struct {
	Event     string         `json:"event"`      // 事件类型: EventMsg / EventFriendMsg 等
	Data      qianxunMsgData `json:"data"`       // 消息体
	RobotWXID string         `json:"robot_wxid"` // 机器人 wxid（部分版本在顶层）
}

type qianxunMsgData struct {
	RobotWXID string `json:"robot_wxid"` // 机器人 wxid
	FromWXID  string `json:"from_wxid"`  // 来源（私聊=对方 wxid, 群聊=群 id）
	FromName  string `json:"from_name"`  // 来源昵称
	ToWXID    string `json:"to_wxid"`    // 接收方
	MsgType   int    `json:"type"`       // 消息类型: 1=文字 3=图片 等
	Content   string `json:"msg"`        // 消息内容（含 CQ 码）
	SelfWXID  string `json:"self_wxid"`  // 自身 wxid
	IsGroup   bool   `json:"is_group"`   // 是否群聊
}

// 千寻 HTTPAPI 响应
type qianxunResponse struct {
	Code int         `json:"code"`
	Msg  string      `json:"msg"`
	Data interface{} `json:"data"`
}

// ---------- CQ 码 ----------
var cqImagePattern = regexp.MustCompile(`\[CQ:image,file=([^,\]]+),url=([^\]]+)\]`)
var cqAtPattern = regexp.MustCompile(`\[CQ:at,qq=([^\]]+)\]`)
var cqAllPattern = regexp.MustCompile(`\[CQ:[a-z]+[^\]]*\]`)

func decodeCQValue(v string) string {
	replacer := strings.NewReplacer(
		"&amp;", "&", "&#91;", "[", "&#93;", "]", "&#44;", ",",
	)
	return replacer.Replace(v)
}

// stripCQ 提取纯文本（去掉 CQ 码）
func stripCQ(content string) string {
	content = cqImagePattern.ReplaceAllString(content, "")
	content = cqAtPattern.ReplaceAllString(content, "@$1")
	return strings.TrimSpace(cqAllPattern.ReplaceAllString(content, ""))
}

// ---------- 鉴权 ----------
func checkWebhookKey(ctx *gin.Context) bool {
	want := strings.TrimSpace(qx.GetString("webhook_key"))
	if want == "" {
		return true // 未配置则不校验（与羽化行为一致）
	}
	got := ctx.Query("key")
	if got == "" {
		got = ctx.GetHeader("X-Webhook-Key")
	}
	return got == want
}

// ---------- 机器人初始化 ----------
func initQXBot() {
	adapterOnce.Do(func() {
		adapter = &core.Factory{}
		adapter.Init("qx", qx.GetString("bot_wxid"), nil)
		adapter.SetIsAdmin(func(s string) bool {
			masters := qx.GetString("admin_only")
			_ = masters // admin 判定走 core 通用 master 机制
			return false
		})
		// 面板/插件发送消息 → 千寻 HTTPAPI
		adapter.Send(func(msg map[string]interface{}) string {
			userID := fmt.Sprint(msg[core.USER_ID])
			content := fmt.Sprint(msg[core.CONETNT])
			sendToQianXun(userID, content)
			return ""
		})
	})
}

// ---------- 发送侧（千寻 HTTPAPI） ----------
func qxAPIBase() string {
	base := strings.TrimSpace(qx.GetString("http_api"))
	if base == "" {
		base = "http://127.0.0.1:7777/qianxun/httpapi"
	}
	return strings.TrimRight(base, "/")
}

func qxCall(action string, params url.Values) (*qianxunResponse, error) {
	full := qxAPIBase() + action
	if safeKey := strings.TrimSpace(qx.GetString("safe_key")); safeKey != "" {
		params.Set("safe_key", safeKey)
	}
	resp, err := http.PostForm(full, params)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, err
	}
	out := &qianxunResponse{}
	if err := json.Unmarshal(body, out); err != nil {
		return nil, fmt.Errorf("千寻响应解析失败: %w", err)
	}
	return out, nil
}

// sendToQianXun 发送内容到指定会话（私聊 wxid 或群 id），支持多条 CQ:image 拆分
func sendToQianXun(toWXID, content string) {
	botWXID := strings.TrimSpace(qx.GetString("bot_wxid"))
	// 拆出图片 CQ 码单独走 sendimage
	remaining := content
	for _, m := range cqImagePattern.FindAllStringSubmatch(content, -1) {
		imageURL := decodeCQValue(m[2])
		params := url.Values{}
		params.Set("wxid", botWXID)
		params.Set("to_wxid", toWXID)
		params.Set("imageurl", imageURL)
		if _, err := qxCall("/sendimage", params); err != nil {
			fmt.Printf("[qx] sendimage 失败: %v\n", err)
		}
		remaining = strings.Replace(remaining, m[0], "", 1)
	}
	text := strings.TrimSpace(stripCQ(remaining))
	if text == "" {
		return
	}
	params := url.Values{}
	params.Set("wxid", botWXID)
	params.Set("to_wxid", toWXID)
	params.Set("msg", text)
	if _, err := qxCall("/sendtext", params); err != nil {
		fmt.Printf("[qx] sendtext 失败: %v\n", err)
	}
}

// ---------- 接收侧（千寻回调 webhook） ----------
func receiveWebhook(ctx *gin.Context) {
	initQXBot()
	if !checkWebhookKey(ctx) {
		ctx.JSON(http.StatusForbidden, gin.H{"error": "invalid key"})
		return
	}
	body, err := io.ReadAll(io.LimitReader(ctx.Request.Body, 1<<20))
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "read body"})
		return
	}
	var ev qianxunEvent
	if err := json.Unmarshal(body, &ev); err != nil {
		// 兼容千寻直接 POST 表单/扁平 JSON 的情况
		var flat qianxunMsgData
		if err2 := json.Unmarshal(body, &flat); err2 != nil {
			ctx.JSON(http.StatusOK, gin.H{"code": 0})
			return
		}
		ev.Data = flat
	}
	d := ev.Data
	if d.RobotWXID == "" {
		d.RobotWXID = ev.RobotWXID
	}
	// 仅处理文本类消息（图片/事件可后续扩展）
	content := decodeCQValue(d.Content)
	if strings.TrimSpace(stripCQ(content)) == "" && d.MsgType != 1 {
		ctx.JSON(http.StatusOK, gin.H{"code": 0})
		return
	}
	// 注入适配器消息流（user_id = 来源 wxid）
	adapter.Push(map[string]string{
		core.USER_ID: d.FromWXID,
		core.CONETNT: stripCQ(content),
	})
	ctx.JSON(http.StatusOK, gin.H{"code": 0, "msg": "ok"})
}

// ---------- 环境巡检（可选，供外部探活） ----------
func qxHealth() map[string]interface{} {
	return map[string]interface{}{
		"enabled":  qx.GetBool("enable"),
		"http_api": qxAPIBase(),
		"bot_wxid": qx.GetString("bot_wxid"),
	}
}

func init() {
	go func() {
		time.Sleep(time.Second)
		if qx.GetBool("enable") {
			initQXBot()
		}
	}()
	core.GinApi(core.GET, "/qx/webhook", receiveWebhook)
	core.GinApi(core.POST, "/qx/webhook", receiveWebhook)
}
