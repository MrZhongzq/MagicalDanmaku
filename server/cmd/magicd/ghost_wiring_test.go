package main

import (
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/MrZhongzq/MagicalDanmaku/server/internal/account"
	"github.com/MrZhongzq/MagicalDanmaku/server/internal/connector/bilibili/auth"
	"github.com/MrZhongzq/MagicalDanmaku/server/internal/event"
	"github.com/MrZhongzq/MagicalDanmaku/server/internal/ghost"
)

// testLogger 返回一个丢弃输出的 logger，避免用例互相刷屏。
func testLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

// danmakuEvent 造一条指定房间、指定发送者的弹幕事件。
func danmakuEvent(roomID, uid, text string) event.Event {
	return event.Event{
		RoomID:  roomID,
		Type:    event.TypeDanmaku,
		Payload: event.Danmaku{User: event.User{UID: uid}, Text: text},
	}
}

// newEchoTestRuntime 建一个只够 settleEcho 用的最小运行时：账号 UID 42、
// 房间 111。
func newEchoTestRuntime(t *testing.T) *roomRuntime {
	t.Helper()
	sess, err := auth.ParseSession("SESSDATA=x; bili_jct=tok; DedeUserID=42")
	if err != nil {
		t.Fatalf("ParseSession 失败: %v", err)
	}
	return &roomRuntime{
		binding: &account.Binding{
			Account: account.New("小号", sess, 0),
			RoomID:  "111",
		},
	}
}

func TestSettleEchoOnlyAcceptsOwnDanmakuFromOwnRoom(t *testing.T) {
	base := time.Date(2026, 8, 9, 12, 0, 0, 0, time.UTC)

	d := ghost.New(10 * time.Second)
	var got []ghost.Report
	d.OnGhost = func(r ghost.Report) { got = append(got, r) }

	rt := newEchoTestRuntime(t)
	rm := &runtimeManager{ghost: d}

	d.Sent("111", "42", "本人本房", base)
	d.Sent("111", "42", "对面房间", base)
	d.Sent("111", "42", "别人发的", base)

	// 只有这条该销账。
	rm.settleEcho(rt, danmakuEvent("111", "42", "本人本房"))
	// PK 期间对面房间的事件会合流进同一个通道，不能拿来销本房间的账。
	rm.settleEcho(rt, danmakuEvent("999", "42", "对面房间"))
	// 同房间但不是本账号发的，与「我发出去的有没有被广播」无关。
	rm.settleEcho(rt, danmakuEvent("111", "77", "别人发的"))

	d.Sweep(base.Add(11*time.Second), func(string) bool { return true })

	if len(got) != 2 {
		t.Fatalf("只有一条该被销账，应剩 2 条判幽灵，实际 %d 条: %+v", len(got), got)
	}
	for _, r := range got {
		if r.Text == "本人本房" {
			t.Errorf("本房间本账号的回显没有销账，仍被判成幽灵")
		}
	}
}

func TestSettleEchoIsInertWithoutDetector(t *testing.T) {
	// 检测关闭时（rm.ghost 为 nil）事件循环每条事件都会调到这里，
	// 不能 panic。
	rm := &runtimeManager{}
	rm.settleEcho(newEchoTestRuntime(t), danmakuEvent("111", "42", "喵"))
}

func TestNewGhostDetectorRejectsBadWindowAndKeepsDefault(t *testing.T) {
	t.Setenv("MAGICD_GHOST_WINDOW", "不是时长")
	d := newGhostDetector(testLogger())
	if d == nil {
		t.Fatal("窗口值非法只该回落到默认值，不该把检测整个关掉")
	}
}

func TestNewGhostDetectorCanBeDisabled(t *testing.T) {
	t.Setenv("MAGICD_GHOST_DETECT", "0")
	if d := newGhostDetector(testLogger()); d != nil {
		t.Error("MAGICD_GHOST_DETECT=0 时应当返回 nil，表示不检测")
	}
}

func TestShouldFireScheduled(t *testing.T) {
	cases := []struct {
		name         string
		onlyWhenLive bool
		liveOffline  bool
		want         bool
	}{
		{"仅直播中 + 确认未开播 → 不触发", true, true, false},
		{"仅直播中 + 未确认未开播 → 触发", true, false, true},
		// 显式关掉开关的规则，下播了照样触发——这是用户主动选择的行为。
		{"关掉开关 + 确认未开播 → 照常触发", false, true, true},
		{"关掉开关 + 在播 → 触发", false, false, true},
	}
	for _, c := range cases {
		if got := shouldFireScheduled(c.onlyWhenLive, c.liveOffline); got != c.want {
			t.Errorf("%s: shouldFireScheduled(%v, %v) = %v，期望 %v",
				c.name, c.onlyWhenLive, c.liveOffline, got, c.want)
		}
	}
}
