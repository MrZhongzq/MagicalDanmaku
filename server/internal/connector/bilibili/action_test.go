package bilibili

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/MrZhongzq/MagicalDanmaku/server/internal/connector"
	"github.com/MrZhongzq/MagicalDanmaku/server/internal/connector/bilibili/api"
	"github.com/MrZhongzq/MagicalDanmaku/server/internal/connector/bilibili/auth"
	"github.com/MrZhongzq/MagicalDanmaku/server/internal/ratelimit"
)

// newTestActions 构造指向假服务器的 Actions。
func newTestActions(t *testing.T, body string, record *[]url.Values) *Actions {
	t.Helper()
	var mu sync.Mutex

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r.ParseForm()
		if record != nil {
			mu.Lock()
			*record = append(*record, r.PostForm)
			mu.Unlock()
		}
		w.Write([]byte(body))
	}))
	t.Cleanup(srv.Close)

	sess, err := auth.ParseSession("SESSDATA=x; bili_jct=tok; DedeUserID=42")
	if err != nil {
		t.Fatalf("ParseSession 失败: %v", err)
	}
	ac := api.New(sess, api.WithHTTPClient(srv.Client()))
	for _, n := range []string{"sendMsg", "addBlock", "delBlock"} {
		ac.SetBaseURL(n, srv.URL)
	}
	return NewActions(ac, ratelimit.NewInterval(0))
}

func TestSendDanmaku(t *testing.T) {
	var forms []url.Values
	a := newTestActions(t, `{"code":0,"data":{}}`, &forms)

	err := a.SendDanmaku(context.Background(), connector.SendDanmakuRequest{
		RoomID: "21452505", Text: "你好世界",
	})
	if err != nil {
		t.Fatalf("SendDanmaku 失败: %v", err)
	}
	if len(forms) != 1 {
		t.Fatalf("请求次数 = %d, 期望 1", len(forms))
	}

	f := forms[0]
	if f.Get("msg") != "你好世界" {
		t.Errorf("msg = %q", f.Get("msg"))
	}
	if f.Get("roomid") != "21452505" {
		t.Errorf("roomid = %q", f.Get("roomid"))
	}
	if f.Get("csrf") != "tok" {
		t.Errorf("csrf = %q", f.Get("csrf"))
	}
	if f.Get("color") == "" || f.Get("fontsize") == "" || f.Get("mode") == "" {
		t.Errorf("缺少必需的样式参数: %v", f)
	}
	if f.Get("rnd") == "" {
		t.Error("缺少 rnd 参数")
	}
}

func TestSendDanmakuWithReply(t *testing.T) {
	var forms []url.Values
	a := newTestActions(t, `{"code":0,"data":{}}`, &forms)

	err := a.SendDanmaku(context.Background(), connector.SendDanmakuRequest{
		RoomID: "21452505", Text: "收到", ReplyToUID: "12345",
	})
	if err != nil {
		t.Fatalf("SendDanmaku 失败: %v", err)
	}
	if got := forms[0].Get("reply_mid"); got != "12345" {
		t.Errorf("reply_mid = %q", got)
	}
}

func TestSendDanmakuSplitsLongText(t *testing.T) {
	var forms []url.Values
	a := newTestActions(t, `{"code":0,"data":{}}`, &forms)
	a.SetMaxLength(5)

	err := a.SendDanmaku(context.Background(), connector.SendDanmakuRequest{
		RoomID: "1", Text: "一二三四五六七八九十",
	})
	if err != nil {
		t.Fatalf("SendDanmaku 失败: %v", err)
	}
	if len(forms) != 2 {
		t.Fatalf("请求次数 = %d, 期望 2", len(forms))
	}
	if forms[0].Get("msg") != "一二三四五" {
		t.Errorf("第一条 = %q", forms[0].Get("msg"))
	}
	if forms[1].Get("msg") != "六七八九十" {
		t.Errorf("第二条 = %q", forms[1].Get("msg"))
	}
}

func TestSendDanmakuRejectsEmpty(t *testing.T) {
	a := newTestActions(t, `{"code":0,"data":{}}`, nil)
	if err := a.SendDanmaku(context.Background(), connector.SendDanmakuRequest{RoomID: "1"}); err == nil {
		t.Error("空文本应当报错")
	}
}

func TestBlockUser(t *testing.T) {
	var forms []url.Values
	a := newTestActions(t, `{"code":0,"data":{}}`, &forms)

	err := a.BlockUser(context.Background(), connector.BlockRequest{
		RoomID: "21452505", UID: "999", Hours: 12,
	})
	if err != nil {
		t.Fatalf("BlockUser 失败: %v", err)
	}
	f := forms[0]
	if f.Get("tuid") != "999" {
		t.Errorf("tuid = %q", f.Get("tuid"))
	}
	if f.Get("hour") != "12" {
		t.Errorf("hour = %q", f.Get("hour"))
	}
	if f.Get("room_id") != "21452505" {
		t.Errorf("room_id = %q", f.Get("room_id"))
	}
}

func TestUnblockUser(t *testing.T) {
	var forms []url.Values
	a := newTestActions(t, `{"code":0,"data":{}}`, &forms)

	if err := a.UnblockUser(context.Background(), "21452505", "888"); err != nil {
		t.Fatalf("UnblockUser 失败: %v", err)
	}
	if got := forms[0].Get("tuid"); got != "888" {
		t.Errorf("tuid = %q", got)
	}
}

func TestSendDanmakuSurfacesAPIError(t *testing.T) {
	a := newTestActions(t, `{"code":1003,"message":"您已被禁言"}`, nil)
	err := a.SendDanmaku(context.Background(), connector.SendDanmakuRequest{RoomID: "1", Text: "x"})
	if err == nil {
		t.Fatal("应当返回错误")
	}
	if !strings.Contains(err.Error(), "禁言") {
		t.Errorf("错误应含服务端消息，实际 %v", err)
	}
	if !IsFatal(err) {
		t.Error("1003 应判定为不可重试")
	}
}

func TestIsFatal(t *testing.T) {
	cases := []struct {
		code  int
		fatal bool
	}{
		{-101, true},   // 未登录
		{-111, true},   // csrf 失效
		{1003, true},   // 已被禁言
		{10030, false}, // 发送过快，可重试
		{-500, false},  // 未知错误，允许重试
	}
	for _, tc := range cases {
		if got := IsFatal(&api.APIError{Code: tc.code}); got != tc.fatal {
			t.Errorf("IsFatal(code=%d) = %v, 期望 %v", tc.code, got, tc.fatal)
		}
	}
}

func TestSplitLongText(t *testing.T) {
	cases := []struct {
		in     string
		maxLen int
		want   []string
	}{
		{"", 5, nil},
		{"短", 5, []string{"短"}},
		{"一二三四五", 5, []string{"一二三四五"}},
		{"一二三四五六", 5, []string{"一二三四五", "六"}},
		{"abcdefghij", 4, []string{"abcd", "efgh", "ij"}},
	}
	for _, tc := range cases {
		got := SplitLongText(tc.in, tc.maxLen)
		if len(got) != len(tc.want) {
			t.Errorf("SplitLongText(%q, %d) 段数 = %d, 期望 %d", tc.in, tc.maxLen, len(got), len(tc.want))
			continue
		}
		for i := range got {
			if got[i] != tc.want[i] {
				t.Errorf("SplitLongText(%q, %d)[%d] = %q, 期望 %q", tc.in, tc.maxLen, i, got[i], tc.want[i])
			}
		}
	}
}

func TestActionsRespectRateLimiter(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"code":0,"data":{}}`))
	}))
	t.Cleanup(srv.Close)

	sess, err := auth.ParseSession("SESSDATA=x; bili_jct=tok; DedeUserID=42")
	if err != nil {
		t.Fatalf("ParseSession 失败: %v", err)
	}
	ac := api.New(sess, api.WithHTTPClient(srv.Client()))
	ac.SetBaseURL("sendMsg", srv.URL)
	a := NewActions(ac, ratelimit.NewInterval(60*time.Millisecond))

	ctx := context.Background()
	req := connector.SendDanmakuRequest{RoomID: "1", Text: "x"}
	if err := a.SendDanmaku(ctx, req); err != nil {
		t.Fatalf("SendDanmaku 失败: %v", err)
	}
	start := time.Now()
	if err := a.SendDanmaku(ctx, req); err != nil {
		t.Fatalf("SendDanmaku 失败: %v", err)
	}
	if d := time.Since(start); d < 40*time.Millisecond {
		t.Errorf("第二次发送应受限流约束，实际间隔 %v", d)
	}
}
