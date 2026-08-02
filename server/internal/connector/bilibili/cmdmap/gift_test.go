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

// TestSendGiftBlindBoxNone 验证普通礼物（blind_gift: null）解析后 BlindBox 为 nil。
func TestSendGiftBlindBoxNone(t *testing.T) {
	evs, err := Map(testCtx(), loadSample(t, "SEND_GIFT_blindbox_none"))
	if err != nil {
		t.Fatalf("Map 失败: %v", err)
	}
	g := evs[0].Payload.(event.Gift)
	if g.BlindBox != nil {
		t.Errorf("普通礼物的 BlindBox 应为 nil，实际 %+v", g.BlindBox)
	}
}

// TestSendGiftBlindBoxLucky 验证「幸运盲盒」样本的四个盲盒字段，
// 并钉住 total_coin == BlindBox.Price * num 与 Gift.Price == BlindBox.TipPrice 两条断言。
func TestSendGiftBlindBoxLucky(t *testing.T) {
	evs, err := Map(testCtx(), loadSample(t, "SEND_GIFT_blindbox_lucky"))
	if err != nil {
		t.Fatalf("Map 失败: %v", err)
	}
	g := evs[0].Payload.(event.Gift)
	if g.BlindBox == nil {
		t.Fatal("盲盒礼物的 BlindBox 不应为 nil")
	}
	if g.BlindBox.Name != "幸运盲盒" {
		t.Errorf("BlindBox.Name = %q", g.BlindBox.Name)
	}
	if g.BlindBox.GiftID != 35206 {
		t.Errorf("BlindBox.GiftID = %d", g.BlindBox.GiftID)
	}
	if g.BlindBox.Price != 5000 {
		t.Errorf("BlindBox.Price = %d", g.BlindBox.Price)
	}
	if g.BlindBox.TipPrice != 5200 {
		t.Errorf("BlindBox.TipPrice = %d", g.BlindBox.TipPrice)
	}
	if g.TotalCoin != g.BlindBox.Price*g.Count {
		t.Errorf("total_coin(%d) != BlindBox.Price(%d) * num(%d)", g.TotalCoin, g.BlindBox.Price, g.Count)
	}
	if g.Price != g.BlindBox.TipPrice {
		t.Errorf("Gift.Price(%d) != BlindBox.TipPrice(%d)", g.Price, g.BlindBox.TipPrice)
	}
}

// TestSendGiftBlindBoxHeartbeat 验证「心动盲盒」样本，同样钉住两条金额断言。
func TestSendGiftBlindBoxHeartbeat(t *testing.T) {
	evs, err := Map(testCtx(), loadSample(t, "SEND_GIFT_blindbox_heartbeat"))
	if err != nil {
		t.Fatalf("Map 失败: %v", err)
	}
	g := evs[0].Payload.(event.Gift)
	if g.BlindBox == nil {
		t.Fatal("盲盒礼物的 BlindBox 不应为 nil")
	}
	if g.BlindBox.Name != "心动盲盒" {
		t.Errorf("BlindBox.Name = %q", g.BlindBox.Name)
	}
	if g.BlindBox.GiftID != 32251 {
		t.Errorf("BlindBox.GiftID = %d", g.BlindBox.GiftID)
	}
	if g.BlindBox.Price != 15000 {
		t.Errorf("BlindBox.Price = %d", g.BlindBox.Price)
	}
	if g.BlindBox.TipPrice != 16000 {
		t.Errorf("BlindBox.TipPrice = %d", g.BlindBox.TipPrice)
	}
	if g.TotalCoin != g.BlindBox.Price*g.Count {
		t.Errorf("total_coin(%d) != BlindBox.Price(%d) * num(%d)", g.TotalCoin, g.BlindBox.Price, g.Count)
	}
	if g.Price != g.BlindBox.TipPrice {
		t.Errorf("Gift.Price(%d) != BlindBox.TipPrice(%d)", g.Price, g.BlindBox.TipPrice)
	}
}

// TestSendGiftBlindBoxBearworm 验证「小熊虫盲盒」样本，同样钉住两条金额断言。
func TestSendGiftBlindBoxBearworm(t *testing.T) {
	evs, err := Map(testCtx(), loadSample(t, "SEND_GIFT_blindbox_bearworm"))
	if err != nil {
		t.Fatalf("Map 失败: %v", err)
	}
	g := evs[0].Payload.(event.Gift)
	if g.BlindBox == nil {
		t.Fatal("盲盒礼物的 BlindBox 不应为 nil")
	}
	if g.BlindBox.Name != "小熊虫盲盒" {
		t.Errorf("BlindBox.Name = %q", g.BlindBox.Name)
	}
	if g.BlindBox.GiftID != 35800 {
		t.Errorf("BlindBox.GiftID = %d", g.BlindBox.GiftID)
	}
	if g.BlindBox.Price != 9000 {
		t.Errorf("BlindBox.Price = %d", g.BlindBox.Price)
	}
	if g.BlindBox.TipPrice != 9000 {
		t.Errorf("BlindBox.TipPrice = %d", g.BlindBox.TipPrice)
	}
	if g.TotalCoin != g.BlindBox.Price*g.Count {
		t.Errorf("total_coin(%d) != BlindBox.Price(%d) * num(%d)", g.TotalCoin, g.BlindBox.Price, g.Count)
	}
	if g.Price != g.BlindBox.TipPrice {
		t.Errorf("Gift.Price(%d) != BlindBox.TipPrice(%d)", g.Price, g.BlindBox.TipPrice)
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
