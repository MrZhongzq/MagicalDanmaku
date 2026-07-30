package cmdmap

import (
	"testing"

	"github.com/MrZhongzq/MagicalDanmaku/server/internal/event"
)

func TestSendGiftSilver(t *testing.T) {
	evs, err := Map(testCtx(), loadSample(t, "SEND_GIFT_silver"))
	if err != nil {
		t.Fatalf("Map 失败: %v", err)
	}
	if len(evs) != 1 || evs[0].Type != event.TypeGift {
		t.Fatalf("结果错误: %+v", evs)
	}

	g := evs[0].Payload.(event.Gift)
	if g.User.UID != "20285041" {
		t.Errorf("UID = %q", g.User.UID)
	}
	if g.User.Username != "路人甲" {
		t.Errorf("Username = %q", g.User.Username)
	}
	if g.User.AvatarURL != "http://i1.hdslb.com/bfs/face/aaa.jpg" {
		t.Errorf("AvatarURL = %q", g.User.AvatarURL)
	}
	if g.User.Medal != nil {
		t.Errorf("空勋章应解析为 nil，实际 %+v", g.User.Medal)
	}
	if g.GiftID != 30607 {
		t.Errorf("GiftID = %d", g.GiftID)
	}
	if g.GiftName != "小心心" {
		t.Errorf("GiftName = %q", g.GiftName)
	}
	if g.Count != 3 {
		t.Errorf("Count = %d, 期望 3", g.Count)
	}
	if g.CoinType != "silver" {
		t.Errorf("CoinType = %q", g.CoinType)
	}
	if g.TotalCoin != 0 {
		t.Errorf("TotalCoin = %d", g.TotalCoin)
	}
	if g.Action != "投喂" {
		t.Errorf("Action = %q", g.Action)
	}
	if got := evs[0].Timestamp.Unix(); got != 1614439816 {
		t.Errorf("Timestamp = %d", got)
	}
}

func TestSendGiftGoldWithMedal(t *testing.T) {
	evs, err := Map(testCtx(), loadSample(t, "SEND_GIFT_gold_medal"))
	if err != nil {
		t.Fatalf("Map 失败: %v", err)
	}
	g := evs[0].Payload.(event.Gift)

	if g.CoinType != "gold" || g.TotalCoin != 2000 {
		t.Errorf("CoinType=%q TotalCoin=%d", g.CoinType, g.TotalCoin)
	}
	if g.User.GuardLevel != event.GuardCaptain {
		t.Errorf("GuardLevel = %d", g.User.GuardLevel)
	}
	if g.User.Medal == nil {
		t.Fatal("Medal 不应为 nil")
	}
	if g.User.Medal.Level != 25 || g.User.Medal.Name != "KKZ" {
		t.Errorf("Medal = %+v", g.User.Medal)
	}
	if g.User.Medal.AnchorName != "某某主播" {
		t.Errorf("Medal.AnchorName = %q", g.User.Medal.AnchorName)
	}
	if g.User.Medal.RoomID != "1010" {
		t.Errorf("Medal.RoomID = %q", g.User.Medal.RoomID)
	}
	if g.User.Medal.AnchorUID != "389088" {
		t.Errorf("Medal.AnchorUID = %q", g.User.Medal.AnchorUID)
	}
	if !g.User.Medal.IsLighted {
		t.Error("IsLighted 应为 true")
	}
}

func TestComboSend(t *testing.T) {
	evs, err := Map(testCtx(), loadSample(t, "COMBO_SEND_basic"))
	if err != nil {
		t.Fatalf("Map 失败: %v", err)
	}
	if len(evs) != 1 || evs[0].Type != event.TypeGiftCombo {
		t.Fatalf("结果错误: %+v", evs)
	}

	c := evs[0].Payload.(event.GiftCombo)
	if c.User.UID != "87654321" {
		t.Errorf("UID = %q", c.User.UID)
	}
	if c.GiftName != "小花花" {
		t.Errorf("GiftName = %q", c.GiftName)
	}
	if c.Count != 5 {
		t.Errorf("Count = %d, 期望 5", c.Count)
	}
	if c.TotalCoin != 5000 {
		t.Errorf("TotalCoin = %d", c.TotalCoin)
	}
	if c.ComboID == "" {
		t.Error("ComboID 不应为空")
	}
}
