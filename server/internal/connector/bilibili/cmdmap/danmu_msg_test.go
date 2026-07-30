package cmdmap

import (
	"os"
	"testing"

	"github.com/MrZhongzq/MagicalDanmaku/server/internal/event"
)

func loadSample(t *testing.T, name string) []byte {
	t.Helper()
	b, err := os.ReadFile("../../../../testdata/cmds/" + name + ".json")
	if err != nil {
		t.Fatalf("读取样本 %s 失败: %v", name, err)
	}
	return b
}

func TestDanmuMsgBasic(t *testing.T) {
	evs, err := Map(testCtx(), loadSample(t, "DANMU_MSG_basic"))
	if err != nil {
		t.Fatalf("Map 失败: %v", err)
	}
	if len(evs) != 1 {
		t.Fatalf("事件数 = %d, 期望 1", len(evs))
	}
	if evs[0].Type != event.TypeDanmaku {
		t.Fatalf("Type = %s, 期望 %s", evs[0].Type, event.TypeDanmaku)
	}

	d := evs[0].Payload.(event.Danmaku)
	if d.Text != "主播晚上好" {
		t.Errorf("Text = %q", d.Text)
	}
	if d.User.UID != "12345678" {
		t.Errorf("UID = %q", d.User.UID)
	}
	if d.User.Username != "路人甲" {
		t.Errorf("Username = %q", d.User.Username)
	}
	if d.User.UserLevel != 18 {
		t.Errorf("UserLevel = %d, 期望 18", d.User.UserLevel)
	}
	if d.User.WealthLevel != 7 {
		t.Errorf("WealthLevel = %d, 期望 7", d.User.WealthLevel)
	}
	if d.User.GuardLevel != event.GuardNone {
		t.Errorf("GuardLevel = %d, 期望 0", d.User.GuardLevel)
	}
	if d.User.IsAdmin {
		t.Error("IsAdmin 应为 false")
	}
	if d.User.Medal != nil {
		t.Errorf("未佩戴勋章时 Medal 应为 nil，实际 %+v", d.User.Medal)
	}
	if d.User.AvatarURL != "https://i0.hdslb.com/bfs/face/aaa.jpg" {
		t.Errorf("AvatarURL = %q", d.User.AvatarURL)
	}
	if d.Color != "#ffffff" {
		t.Errorf("Color = %q, 期望 #ffffff", d.Color)
	}
	if d.IsEmoticon {
		t.Error("IsEmoticon 应为 false")
	}
	if d.ReplyToUID != "" {
		t.Errorf("ReplyToUID = %q, 期望空", d.ReplyToUID)
	}
	if got := evs[0].Timestamp.UnixMilli(); got != 1700000000000 {
		t.Errorf("Timestamp = %d, 期望 1700000000000", got)
	}
}

func TestDanmuMsgGuardAndMedal(t *testing.T) {
	evs, err := Map(testCtx(), loadSample(t, "DANMU_MSG_guard_medal"))
	if err != nil {
		t.Fatalf("Map 失败: %v", err)
	}
	d := evs[0].Payload.(event.Danmaku)

	if d.User.GuardLevel != event.GuardCaptain {
		t.Errorf("GuardLevel = %d, 期望 3", d.User.GuardLevel)
	}
	if !d.User.IsAdmin {
		t.Error("IsAdmin 应为 true")
	}
	if d.User.Medal == nil {
		t.Fatal("Medal 不应为 nil")
	}
	if d.User.Medal.Level != 21 {
		t.Errorf("Medal.Level = %d, 期望 21", d.User.Medal.Level)
	}
	if d.User.Medal.Name != "小心心" {
		t.Errorf("Medal.Name = %q", d.User.Medal.Name)
	}
	if d.User.Medal.AnchorName != "某某主播" {
		t.Errorf("Medal.AnchorName = %q", d.User.Medal.AnchorName)
	}
	if d.User.Medal.RoomID != "21452505" {
		t.Errorf("Medal.RoomID = %q", d.User.Medal.RoomID)
	}
	if !d.IsEmoticon {
		t.Error("IsEmoticon 应为 true")
	}
	if d.ReplyToUID != "20285041" {
		t.Errorf("ReplyToUID = %q, 期望 20285041", d.ReplyToUID)
	}
	if d.ReplyToName != "某某主播" {
		t.Errorf("ReplyToName = %q", d.ReplyToName)
	}
	// 样本中 textColor 为 5566168，即 0x54eed8
	if d.Color != "#54eed8" {
		t.Errorf("Color = %q, 期望 #54eed8", d.Color)
	}
}
