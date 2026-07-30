package cmdmap

import (
	"testing"

	"github.com/MrZhongzq/MagicalDanmaku/server/internal/event"
)

func TestSuperChatMessage(t *testing.T) {
	evs, err := Map(testCtx(), loadSample(t, "SUPER_CHAT_MESSAGE_basic"))
	if err != nil {
		t.Fatalf("Map 失败: %v", err)
	}
	if len(evs) != 1 || evs[0].Type != event.TypeSuperChat {
		t.Fatalf("结果错误: %+v", evs)
	}

	sc := evs[0].Payload.(event.SuperChat)
	if sc.ID != 1278390 {
		t.Errorf("ID = %d", sc.ID)
	}
	if sc.Text != "最右边可以爬上去" {
		t.Errorf("Text = %q", sc.Text)
	}
	if sc.Price != 30 {
		t.Errorf("Price = %d", sc.Price)
	}
	if sc.Duration != 60 {
		t.Errorf("Duration = %d", sc.Duration)
	}
	if sc.User.UID != "389088" {
		t.Errorf("UID = %q", sc.User.UID)
	}
	if sc.User.Username != "SC用户" {
		t.Errorf("Username = %q", sc.User.Username)
	}
	if sc.User.AvatarURL != "https://i0.hdslb.com/bfs/face/ccc.jpg" {
		t.Errorf("AvatarURL = %q", sc.User.AvatarURL)
	}
	if sc.User.UserLevel != 20 {
		t.Errorf("UserLevel = %d", sc.User.UserLevel)
	}
	if sc.User.GuardLevel != event.GuardCaptain {
		t.Errorf("GuardLevel = %d", sc.User.GuardLevel)
	}
	if sc.User.Medal == nil || sc.User.Medal.Name != "KKZ" {
		t.Errorf("Medal = %+v", sc.User.Medal)
	}
	if got := evs[0].Timestamp.Unix(); got != 1613125845 {
		t.Errorf("Timestamp = %d", got)
	}
}

func TestSuperChatDelete(t *testing.T) {
	evs, err := Map(testCtx(), loadSample(t, "SUPER_CHAT_MESSAGE_DELETE_basic"))
	if err != nil {
		t.Fatalf("Map 失败: %v", err)
	}
	if len(evs) != 1 || evs[0].Type != event.TypeSuperChatDelete {
		t.Fatalf("结果错误: %+v", evs)
	}

	d := evs[0].Payload.(event.SuperChatDelete)
	if len(d.IDs) != 2 || d.IDs[0] != 1278390 || d.IDs[1] != 1278391 {
		t.Errorf("IDs = %v", d.IDs)
	}
}
