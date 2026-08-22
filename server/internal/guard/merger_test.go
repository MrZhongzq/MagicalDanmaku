package guard

import (
	"testing"
	"time"

	"github.com/MrZhongzq/MagicalDanmaku/server/internal/event"
)

var t0 = time.Date(2026, 8, 22, 8, 22, 41, 0, time.UTC)

const (
	room  = "1775065569"
	uid   = "519338244"
	owner = int64(3)
)

// buy 造一条上舰事件。renewKnown 就是真机上区分两条广播的那个点：
// GUARD_BUY 没有 op_type，USER_TOAST_MSG 有。
func buy(renewKnown, isRenew bool, mark string) event.Event {
	return event.Event{
		RoomID: room,
		Type:   event.TypeGuardBuy,
		Raw:    []byte(mark),
		Payload: event.GuardBuy{
			User:       event.User{UID: uid},
			GuardLevel: 3,
			IsRenew:    isRenew,
			RenewKnown: renewKnown,
		},
	}
}

func mark(ev event.Event) string { return string(ev.Raw) }

// TestToastWinsOverGuardBuy 是这个类存在的理由。
//
// 真机时序（2026-08-22 铃七直播间）：GUARD_BUY 先到，0.46 秒后
// USER_TOAST_MSG 才到，而**只有后者带 op_type**。原来「先到先得」的去重
// 保留的恰恰是没有信息的那条，于是续费永远被说成开通。
func TestToastWinsOverGuardBuy(t *testing.T) {
	m := NewMerger(2*time.Second, 10*time.Second)

	if _, ok := m.Offer(owner, buy(false, false, "GUARD_BUY"), t0); ok {
		t.Fatal("GUARD_BUY 不该立刻放行——它没有 op_type，要等一下更全的那条")
	}

	got, ok := m.Offer(owner, buy(true, true, "USER_TOAST_MSG"), t0.Add(460*time.Millisecond))
	if !ok {
		t.Fatal("带 op_type 的那条应当立刻放行")
	}
	if mark(got) != "USER_TOAST_MSG" {
		t.Errorf("放行的是 %q，期望 USER_TOAST_MSG", mark(got))
	}

	// 缓冲里那条 GUARD_BUY 必须被顶掉，否则窗口一到又发一次，
	// 就退回成「答谢两遍」这个老问题。
	if left := m.Sweep(t0.Add(10 * time.Second)); len(left) != 0 {
		t.Errorf("USER_TOAST_MSG 放行后缓冲应清空，实际还剩 %d 条", len(left))
	}
}

// TestGuardBuyEmittedWhenToastNeverComes 守住兜底：不能因为等一条可能
// 永远不来的消息，就把答谢整个吞掉。
func TestGuardBuyEmittedWhenToastNeverComes(t *testing.T) {
	m := NewMerger(2*time.Second, 10*time.Second)

	if _, ok := m.Offer(owner, buy(false, false, "GUARD_BUY"), t0); ok {
		t.Fatal("GUARD_BUY 不该立刻放行")
	}
	if got := m.Sweep(t0.Add(time.Second)); len(got) != 0 {
		t.Fatalf("窗口未到就交出了 %d 条", len(got))
	}

	got := m.Sweep(t0.Add(2 * time.Second))
	if len(got) != 1 {
		t.Fatalf("窗口到了应当交出 1 条，实际 %d 条", len(got))
	}
	if got[0].Owner != owner {
		t.Errorf("Owner = %d, 期望 %d", got[0].Owner, owner)
	}
	if mark(got[0].Event) != "GUARD_BUY" {
		t.Errorf("交出的是 %q", mark(got[0].Event))
	}
}

// TestSecondBroadcastAfterEmitIsDropped 覆盖两条广播倒序到达的情况：
// 先放行了带 op_type 的那条，随后到的 GUARD_BUY 必须被丢掉。
func TestSecondBroadcastAfterEmitIsDropped(t *testing.T) {
	m := NewMerger(2*time.Second, 10*time.Second)

	if _, ok := m.Offer(owner, buy(true, true, "USER_TOAST_MSG"), t0); !ok {
		t.Fatal("带 op_type 的那条应当立刻放行")
	}
	if _, ok := m.Offer(owner, buy(false, false, "GUARD_BUY"), t0.Add(400*time.Millisecond)); ok {
		t.Error("同一次上舰的另一条广播应当被丢掉，否则会答谢两次")
	}
	if left := m.Sweep(t0.Add(10 * time.Second)); len(left) != 0 {
		t.Errorf("后到的那条不该被塞进缓冲，实际剩 %d 条", len(left))
	}
}

// TestDuplicateGuardBuyDoesNotEmitTwice 覆盖两条广播都不带 op_type 的
// 情况（B 站将来若改了字段，也不该退化成答谢两次）。
func TestDuplicateGuardBuyDoesNotEmitTwice(t *testing.T) {
	m := NewMerger(2*time.Second, 10*time.Second)

	m.Offer(owner, buy(false, false, "第一条"), t0)
	m.Offer(owner, buy(false, false, "第二条"), t0.Add(400*time.Millisecond))

	got := m.Sweep(t0.Add(2 * time.Second))
	if len(got) != 1 {
		t.Fatalf("同一次上舰只该交出 1 条，实际 %d 条", len(got))
	}
	if mark(got[0].Event) != "第一条" {
		t.Errorf("应当保留先到的那条，实际是 %q", mark(got[0].Event))
	}
}

// TestLaterPurchaseIsNotSuppressed 窗口之外是另一笔真实购买，不能吞。
func TestLaterPurchaseIsNotSuppressed(t *testing.T) {
	m := NewMerger(2*time.Second, 10*time.Second)

	if _, ok := m.Offer(owner, buy(true, false, "开通"), t0); !ok {
		t.Fatal("第一次应当放行")
	}
	// 真机上这两次相隔 12 分钟：先开通，后续费。
	got, ok := m.Offer(owner, buy(true, true, "续费"), t0.Add(12*time.Minute))
	if !ok {
		t.Fatal("窗口之外的再次上舰是另一件事，应当放行")
	}
	if !got.Payload.(event.GuardBuy).IsRenew {
		t.Error("放行的应当是续费那条")
	}
}

// TestUpgradeToHigherLevelIsNotSuppressed 同一个人紧接着从舰长升提督是
// 另一笔真实购买，等级进入判定键就是为了不把它吞掉。
func TestUpgradeToHigherLevelIsNotSuppressed(t *testing.T) {
	m := NewMerger(2*time.Second, 10*time.Second)

	m.Offer(owner, buy(true, false, "舰长"), t0)

	up := buy(true, false, "提督")
	g := up.Payload.(event.GuardBuy)
	g.GuardLevel = 2
	up.Payload = g

	if _, ok := m.Offer(owner, up, t0.Add(time.Second)); !ok {
		t.Error("不同大航海等级是不同的购买，不该被吞掉")
	}
}

// TestOtherRoomDoesNotInterfere PK 期间对面房间的事件会合流进同一个
// 通道，同一个人在两个直播间各上一次舰是两件事。
func TestOtherRoomDoesNotInterfere(t *testing.T) {
	m := NewMerger(2*time.Second, 10*time.Second)

	m.Offer(owner, buy(true, false, "我方"), t0)

	other := buy(true, false, "对面")
	other.RoomID = "1751769622"

	if _, ok := m.Offer(owner, other, t0.Add(time.Second)); !ok {
		t.Error("不同直播间是不同的购买")
	}
}

// TestNonGuardEventPassesThrough 非上舰事件原样立刻放行，不进缓冲。
func TestNonGuardEventPassesThrough(t *testing.T) {
	m := NewMerger(2*time.Second, 10*time.Second)

	ev := event.Event{RoomID: room, Type: event.TypeDanmaku, Payload: event.Danmaku{}}
	got, ok := m.Offer(owner, ev, t0)
	if !ok {
		t.Fatal("非上舰事件必须立刻放行")
	}
	if got.Type != event.TypeDanmaku {
		t.Errorf("Type = %s", got.Type)
	}
	if left := m.Sweep(t0.Add(10 * time.Second)); len(left) != 0 {
		t.Errorf("非上舰事件不该进缓冲，实际剩 %d 条", len(left))
	}
}
