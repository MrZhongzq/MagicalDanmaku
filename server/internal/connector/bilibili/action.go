package bilibili

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"strconv"
	"sync"
	"time"

	"github.com/MrZhongzq/MagicalDanmaku/server/internal/connector"
	"github.com/MrZhongzq/MagicalDanmaku/server/internal/connector/bilibili/api"
	"github.com/MrZhongzq/MagicalDanmaku/server/internal/ratelimit"
)

// defaultMaxDanmakuLength 是普通账号的单条弹幕字数上限。
// 高等级或舰长账号可达 30/40，由上层通过 SetMaxLength 调整。
const defaultMaxDanmakuLength = 20

// fatalCodes 是不应重试的返回码。
var fatalCodes = map[int]bool{
	-101: true, // 账号未登录
	-111: true, // csrf 校验失败
	1003: true, // 已被禁言
}

// IsFatal 判断错误是否不可重试。
// 未知错误一律视为可重试，交由上层的退避策略处理。
func IsFatal(err error) bool {
	var apiErr *api.APIError
	if !errors.As(err, &apiErr) {
		return false
	}
	return fatalCodes[apiErr.Code]
}

// Actions 实现 connector.Actions。
type Actions struct {
	api     *api.Client
	limiter ratelimit.Limiter

	mu        sync.RWMutex
	maxLength int
}

var _ connector.Actions = (*Actions)(nil)

// NewActions 创建动作执行器。limiter 为 nil 时使用 1.5 秒的保守默认值。
func NewActions(c *api.Client, limiter ratelimit.Limiter) *Actions {
	if limiter == nil {
		limiter = ratelimit.NewInterval(1500 * time.Millisecond)
	}
	return &Actions{api: c, limiter: limiter, maxLength: defaultMaxDanmakuLength}
}

// SetMaxLength 设置单条弹幕的字数上限。
func (a *Actions) SetMaxLength(n int) {
	if n <= 0 {
		return
	}
	a.mu.Lock()
	a.maxLength = n
	a.mu.Unlock()
}

// SendDanmaku 发送弹幕，超长文本自动切分为多条依次发送。
func (a *Actions) SendDanmaku(ctx context.Context, req connector.SendDanmakuRequest) error {
	if req.Text == "" {
		return errors.New("bilibili: 弹幕内容不能为空")
	}
	if req.RoomID == "" {
		return errors.New("bilibili: 未指定直播间号")
	}

	a.mu.RLock()
	maxLen := a.maxLength
	a.mu.RUnlock()

	for i, part := range SplitLongText(req.Text, maxLen) {
		if err := a.limiter.Wait(ctx); err != nil {
			return err
		}
		if err := a.sendOne(ctx, req.RoomID, part, req.ReplyToUID); err != nil {
			return fmt.Errorf("发送第 %d 段弹幕失败: %w", i+1, err)
		}
	}
	return nil
}

// sendOne 发送单条弹幕。
func (a *Actions) sendOne(ctx context.Context, roomID, text, replyMID string) error {
	form := url.Values{}
	form.Set("bubble", "0")
	form.Set("msg", text)
	form.Set("color", "16777215")
	form.Set("mode", "1")
	form.Set("fontsize", "25")
	form.Set("rnd", strconv.FormatInt(time.Now().Unix(), 10))
	form.Set("roomid", roomID)
	if replyMID != "" {
		form.Set("reply_mid", replyMID)
	}
	return a.api.PostForm(ctx, a.api.URLFor("sendMsg"), form, nil)
}

// BlockUser 禁言用户。
func (a *Actions) BlockUser(ctx context.Context, req connector.BlockRequest) error {
	if req.UID == "" || req.RoomID == "" {
		return errors.New("bilibili: 禁言请求缺少 UID 或直播间号")
	}
	hours := req.Hours
	if hours <= 0 {
		hours = 1
	}
	if err := a.limiter.Wait(ctx); err != nil {
		return err
	}

	form := url.Values{}
	form.Set("room_id", req.RoomID)
	form.Set("tuid", req.UID)
	form.Set("mobile_app", "web")
	form.Set("visit_id", "")
	form.Set("hour", strconv.Itoa(hours))
	return a.api.PostForm(ctx, a.api.URLFor("addBlock"), form, nil)
}

// UnblockUser 解除禁言。
func (a *Actions) UnblockUser(ctx context.Context, roomID, uid string) error {
	if uid == "" || roomID == "" {
		return errors.New("bilibili: 解禁请求缺少 UID 或直播间号")
	}
	if err := a.limiter.Wait(ctx); err != nil {
		return err
	}

	form := url.Values{}
	form.Set("roomid", roomID)
	form.Set("tuid", uid)
	form.Set("visit_id", "")
	return a.api.PostForm(ctx, a.api.URLFor("delBlock"), form, nil)
}

// SplitLongText 按字符数（而非字节数）把文本切成若干段。
// 空文本返回 nil。
func SplitLongText(text string, maxLen int) []string {
	if text == "" {
		return nil
	}
	if maxLen <= 0 {
		return []string{text}
	}

	runes := []rune(text)
	if len(runes) <= maxLen {
		return []string{text}
	}

	out := make([]string, 0, (len(runes)+maxLen-1)/maxLen)
	for i := 0; i < len(runes); i += maxLen {
		end := i + maxLen
		if end > len(runes) {
			end = len(runes)
		}
		out = append(out, string(runes[i:end]))
	}
	return out
}
