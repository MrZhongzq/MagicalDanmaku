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
	"github.com/MrZhongzq/MagicalDanmaku/server/internal/guard"
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

// guardBuyEvent 造一条上舰事件。
//
// renewKnown 是真机上区分两条广播的那个点：GUARD_BUY 没有 op_type，
// USER_TOAST_MSG 有。isRenew 必须能单独指定——同一次上舰的两条广播在
// 这个字段上正好相反，把它放进判定键，两条就永远不相等，收敛完全失效。
func guardBuyEvent(roomID, uid string, level int, renewKnown, isRenew bool, raw string) event.Event {
	return event.Event{
		RoomID: roomID,
		Type:   event.TypeGuardBuy,
		Raw:    []byte(raw),
		Payload: event.GuardBuy{
			User:       event.User{UID: uid},
			GuardLevel: level,
			GuardName:  "舰长",
			IsRenew:    isRenew,
			RenewKnown: renewKnown,
		},
	}
}

// TestScreenGuardBuyPrefersToastOverGuardBuy 钉住这道闸交给引擎的是
// **带 op_type 的那条**，而不是先到的那条。
//
// 真机时序（2026-08-22 铃七直播间）：GUARD_BUY 先到，0.46 秒后
// USER_TOAST_MSG 才到，只有后者带 op_type。原来先到先得，于是续费一律
// 被说成「开通」。
func TestScreenGuardBuyPrefersToastOverGuardBuy(t *testing.T) {
	rm := &runtimeManager{
		guardMerger: guard.NewMerger(guardMergeWait, guardMergeSuppress),
		log:         testLogger(),
	}
	rt := newEchoTestRuntime(t) // 账号 uid 42、房间 111

	first := guardBuyEvent("111", "10086", 3, false, false, "GUARD_BUY")
	if _, ok := rm.screenGuardBuy(7, rt, first); ok {
		t.Fatal("没有 op_type 的那条不该立刻放行——要等一下更全的那条")
	}

	second := guardBuyEvent("111", "10086", 3, true, true, "USER_TOAST_MSG")
	got, ok := rm.screenGuardBuy(7, rt, second)
	if !ok {
		t.Fatal("带 op_type 的那条应当立刻放行")
	}
	if string(got.Raw) != "USER_TOAST_MSG" {
		t.Errorf("放行的是 %q，期望 USER_TOAST_MSG——放行了没有 op_type 的那条，"+
			"续费就会被说成开通", got.Raw)
	}
	if !got.Payload.(event.GuardBuy).IsRenew {
		t.Error("放行的那条应当带着续费结论")
	}
}

// TestScreenGuardBuyIgnoresOtherRooms PK 期间对面房间的事件会合流进同一个
// 通道，不能拿来占本房间的坑，也不该被押进缓冲。
func TestScreenGuardBuyIgnoresOtherRooms(t *testing.T) {
	rm := &runtimeManager{
		guardMerger: guard.NewMerger(guardMergeWait, guardMergeSuppress),
		log:         testLogger(),
	}
	rt := newEchoTestRuntime(t)

	if _, ok := rm.screenGuardBuy(7, rt, guardBuyEvent("999", "10086", 3, false, false, "对面")); !ok {
		t.Fatal("别的房间的事件应当原样放行，不走本房间的收敛")
	}
	if _, ok := rm.screenGuardBuy(7, rt, guardBuyEvent("111", "10086", 3, true, false, "我方")); !ok {
		t.Error("本房间的事件被对面房间的事件顶掉了")
	}
}

// TestScreenGuardBuyIsInertWithoutMerger 没有收敛器时必须原样放行，
// 不能把上舰答谢整个吞掉。
func TestScreenGuardBuyIsInertWithoutMerger(t *testing.T) {
	rm := &runtimeManager{log: testLogger()}
	ev := guardBuyEvent("111", "10086", 3, false, false, "GUARD_BUY")
	got, ok := rm.screenGuardBuy(7, newEchoTestRuntime(t), ev)
	if !ok {
		t.Fatal("没有收敛器时不该丢事件")
	}
	if string(got.Raw) != "GUARD_BUY" {
		t.Errorf("应当原样返回，实际 Raw = %q", got.Raw)
	}
}

// TestNonGuardEventPassesScreenUnchanged 非上舰事件必须原样立刻放行——
// 这道闸挡在全部事件的必经之路上，误伤面是整个事件流。
func TestNonGuardEventPassesScreenUnchanged(t *testing.T) {
	rm := &runtimeManager{
		guardMerger: guard.NewMerger(guardMergeWait, guardMergeSuppress),
		log:         testLogger(),
	}
	ev := event.Event{
		RoomID:  "111",
		Type:    event.TypeDanmaku,
		Payload: event.Danmaku{User: event.User{UID: "10086"}, Text: "你好"},
	}
	got, ok := rm.screenGuardBuy(7, newEchoTestRuntime(t), ev)
	if !ok || got.Type != event.TypeDanmaku {
		t.Errorf("非上舰事件应当原样放行，实际 ok=%v type=%s", ok, got.Type)
	}
}
