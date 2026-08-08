package ghost

import (
	"testing"
	"time"
)

// alwaysOnline 是「所有房间都连着」的 online 回调，用于不关心断线的用例。
func alwaysOnline(string) bool { return true }

// t0 是所有用例共用的基准时刻，避免依赖真实时钟。
var t0 = time.Date(2026, 8, 9, 12, 0, 0, 0, time.UTC)

const (
	room = "1781257934"
	self = "3546956351671045"
)

func TestSweepReportsGhostWhenEchoNeverArrives(t *testing.T) {
	d := New(10 * time.Second)
	var got []Report
	d.OnGhost = func(r Report) { got = append(got, r) }

	d.Sent(room, self, "喵", t0)
	d.Sweep(t0.Add(11*time.Second), alwaysOnline)

	if len(got) != 1 {
		t.Fatalf("发出去却没回显，应报 1 条幽灵，实际 %d 条", len(got))
	}
	if got[0].Text != "喵" {
		t.Errorf("Text = %q，期望 %q", got[0].Text, "喵")
	}
	if got[0].RoomID != room || got[0].UID != self {
		t.Errorf("房间/账号 = %q/%q，期望 %q/%q", got[0].RoomID, got[0].UID, room, self)
	}
	if got[0].Waited != 11*time.Second {
		t.Errorf("Waited = %v，期望 11s", got[0].Waited)
	}
}

func TestObserveSettlesSoNothingIsReported(t *testing.T) {
	d := New(10 * time.Second)
	var got []Report
	d.OnGhost = func(r Report) { got = append(got, r) }

	d.Sent(room, self, "喵", t0)
	d.Observe(room, self, "喵")
	d.Sweep(t0.Add(11*time.Second), alwaysOnline)

	if len(got) != 0 {
		t.Fatalf("回显已到达，不该报幽灵，实际报了 %d 条: %+v", len(got), got)
	}
}

func TestSweepDefersWhileRoomIsOffline(t *testing.T) {
	d := New(10 * time.Second)
	var got []Report
	d.OnGhost = func(r Report) { got = append(got, r) }

	d.Sent(room, self, "喵", t0)

	// 断线期间本来就收不到任何回显，这时判定就是纯误报。
	d.Sweep(t0.Add(11*time.Second), func(string) bool { return false })
	if len(got) != 0 {
		t.Fatalf("断线期间不该判定，实际报了 %d 条: %+v", len(got), got)
	}

	// 顺延不等于丢弃：连接恢复后仍然没有回显，这时才该报出来。
	d.Sweep(t0.Add(12*time.Second), alwaysOnline)
	if len(got) != 1 {
		t.Fatalf("恢复连接后仍无回显，应报 1 条，实际 %d 条", len(got))
	}
}

func TestObserveReportsMismatchWhenEchoTextWasRewritten(t *testing.T) {
	d := New(10 * time.Second)
	var ghosts []Report
	var mismatches []Mismatch
	d.OnGhost = func(r Report) { ghosts = append(ghosts, r) }
	d.OnMismatch = func(m Mismatch) { mismatches = append(mismatches, m) }

	d.Sent(room, self, "抽奖了快来", t0)
	d.Observe(room, self, "**了快来")
	d.Sweep(t0.Add(11*time.Second), alwaysOnline)

	if len(mismatches) != 1 {
		t.Fatalf("回显文本被改写，应报 1 条不一致，实际 %d 条", len(mismatches))
	}
	if mismatches[0].Sent != "抽奖了快来" || mismatches[0].Echo != "**了快来" {
		t.Errorf("不一致内容 = 发%q/回%q，期望 发%q/回%q",
			mismatches[0].Sent, mismatches[0].Echo, "抽奖了快来", "**了快来")
	}
	// 回显确实到了，只是被改写——这与「压根没广播」是两码事，不能再报幽灵。
	if len(ghosts) != 0 {
		t.Errorf("回显已到达，不该同时报幽灵，实际报了 %d 条", len(ghosts))
	}
}

func TestObserveIgnoresDanmakuWeNeverSent(t *testing.T) {
	d := New(10 * time.Second)
	var mismatches []Mismatch
	d.OnMismatch = func(m Mismatch) { mismatches = append(mismatches, m) }

	// 没有任何登记时收到本账号的弹幕：这条是别处发的（比如主播自己用
	// 手机在同一个账号下发言），与本系统无关，不该报不一致。
	d.Observe(room, self, "手机上发的")

	if len(mismatches) != 0 {
		t.Fatalf("没有待销账的登记时不该报不一致，实际报了 %d 条", len(mismatches))
	}
}

func TestEachSegmentIsJudgedIndependently(t *testing.T) {
	d := New(10 * time.Second)
	var got []Report
	d.OnGhost = func(r Report) { got = append(got, r) }

	// 超长弹幕被切成两段分别发出，回显也是分开的两条。按整条登记会永远
	// 匹配不上，所以每段必须各自登记、各自判定。
	d.Sent(room, self, "前半段", t0)
	d.Sent(room, self, "后半段", t0)
	d.Observe(room, self, "前半段")

	d.Sweep(t0.Add(11*time.Second), alwaysOnline)

	if len(got) != 1 {
		t.Fatalf("只有后半段没回显，应报 1 条，实际 %d 条: %+v", len(got), got)
	}
	if got[0].Text != "后半段" {
		t.Errorf("Text = %q，期望 %q", got[0].Text, "后半段")
	}
}

func TestObserveSettlesOldestFirstWhenTextRepeats(t *testing.T) {
	d := New(10 * time.Second)
	var got []Report
	d.OnGhost = func(r Report) { got = append(got, r) }

	// 轮播三分钟一条，同样的内容会反复出现。
	d.Sent(room, self, "喵", t0)
	d.Sent(room, self, "喵", t0.Add(3*time.Minute))
	// 只回来一条。
	d.Observe(room, self, "喵")

	d.Sweep(t0.Add(3*time.Minute+11*time.Second), alwaysOnline)

	if len(got) != 1 {
		t.Fatalf("两条同文本只回显一条，应报 1 条幽灵，实际 %d 条", len(got))
	}
	// 先发的先回显，销掉的应该是早的那条，留下来被判幽灵的是晚的那条。
	if !got[0].SentAt.Equal(t0.Add(3 * time.Minute)) {
		t.Errorf("应销早报晚，实际报的 SentAt = %v，期望 %v",
			got[0].SentAt, t0.Add(3*time.Minute))
	}
}
