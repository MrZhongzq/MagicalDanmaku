package cmdmap

import (
	"testing"

	"github.com/MrZhongzq/MagicalDanmaku/server/internal/event"
)

func TestGuardBuy(t *testing.T) {
	evs, err := Map(testCtx(), loadSample(t, "GUARD_BUY_basic"))
	if err != nil {
		t.Fatalf("Map 失败: %v", err)
	}
	if len(evs) != 1 || evs[0].Type != event.TypeGuardBuy {
		t.Fatalf("结果错误: %+v", evs)
	}

	g := evs[0].Payload.(event.GuardBuy)
	if g.User.UID != "67756641" {
		t.Errorf("UID = %q", g.User.UID)
	}
	if g.User.Username != "新舰长" {
		t.Errorf("Username = %q", g.User.Username)
	}
	if g.GuardLevel != event.GuardCaptain {
		t.Errorf("GuardLevel = %d, 期望 3", g.GuardLevel)
	}
	if g.GuardName != "舰长" {
		t.Errorf("GuardName = %q", g.GuardName)
	}
	if g.Count != 1 {
		t.Errorf("Count = %d", g.Count)
	}
	if g.Price != 198000 {
		t.Errorf("Price = %d", g.Price)
	}
	if g.IsRenew {
		t.Error("GUARD_BUY 是新购，IsRenew 应为 false")
	}
	// User.GuardLevel 也应被填上，方便下游统一取用
	if g.User.GuardLevel != event.GuardCaptain {
		t.Errorf("User.GuardLevel = %d, 期望 3", g.User.GuardLevel)
	}
	if got := evs[0].Timestamp.Unix(); got != 1611343771 {
		t.Errorf("Timestamp = %d", got)
	}
}

func TestUserToastMsgRenew(t *testing.T) {
	evs, err := Map(testCtx(), loadSample(t, "USER_TOAST_MSG_renew"))
	if err != nil {
		t.Fatalf("Map 失败: %v", err)
	}
	if evs[0].Type != event.TypeGuardBuy {
		t.Fatalf("Type = %s, 期望 %s", evs[0].Type, event.TypeGuardBuy)
	}

	g := evs[0].Payload.(event.GuardBuy)
	if !g.IsRenew {
		t.Error("op_type=2 是续费，IsRenew 应为 true")
	}
	if !g.RenewKnown {
		t.Error("USER_TOAST_MSG 带 op_type，RenewKnown 应为 true")
	}
	// 续费的实付价与开通不同（真机上 168000 对 198000），而 GUARD_BUY
	// 在两种情况下都报 198000——这也是要以本条为准的原因之一。
	if g.Price != 168000 {
		t.Errorf("Price = %d, 期望 168000", g.Price)
	}
}

// TestUserToastMsgNew 是上一条的对照：同一个人、同一个房间、同一等级，
// 只有 op_type 与实付价不同。两条都是真机抓包（2026-08-22 铃七直播间），
// 不是手写的构造样本。
//
// 在此之前 mapUserToastMsg 把 IsRenew 写死成 true，前提是「USER_TOAST_MSG
// 就是续费消息」——真机数据证明这个前提是错的：B 站对**同一次购买**会同时
// 下发 GUARD_BUY 与 USER_TOAST_MSG，开通也不例外。
func TestUserToastMsgNew(t *testing.T) {
	evs, err := Map(testCtx(), loadSample(t, "USER_TOAST_MSG_new"))
	if err != nil {
		t.Fatalf("Map 失败: %v", err)
	}

	g := evs[0].Payload.(event.GuardBuy)
	if g.IsRenew {
		t.Error("op_type=1 是开通，IsRenew 应为 false")
	}
	if !g.RenewKnown {
		t.Error("USER_TOAST_MSG 带 op_type，RenewKnown 应为 true")
	}
	if g.Price != 198000 {
		t.Errorf("Price = %d, 期望 198000", g.Price)
	}
}

// TestGuardBuyCarriesNoRenewSignal 钉住「GUARD_BUY 没有任何续费线索」这个
// 事实本身。真机抓包里，同一个人的开通与续费两次，GUARD_BUY 的全部字段
// （含 price=198000）完全一样——所以它只能报「不知道」，不能假装知道。
func TestGuardBuyCarriesNoRenewSignal(t *testing.T) {
	evs, err := Map(testCtx(), loadSample(t, "GUARD_BUY_basic"))
	if err != nil {
		t.Fatalf("Map 失败: %v", err)
	}

	g := evs[0].Payload.(event.GuardBuy)
	if g.RenewKnown {
		t.Error("GUARD_BUY 里没有 op_type，RenewKnown 必须为 false——" +
			"报成 true 会让上舰答谢用一个纯属猜测的结论覆盖真正带线索的那条")
	}
}
