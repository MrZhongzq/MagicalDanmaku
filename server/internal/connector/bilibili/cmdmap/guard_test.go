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
	if g.GuardLevel != event.GuardAdmiral {
		t.Errorf("GuardLevel = %d, 期望 2", g.GuardLevel)
	}
	if g.GuardName != "提督" {
		t.Errorf("GuardName = %q", g.GuardName)
	}
	if g.Count != 3 {
		t.Errorf("Count = %d, 期望 3", g.Count)
	}
	if !g.IsRenew {
		t.Error("is_auto_renew=1 时 IsRenew 应为 true")
	}
}
