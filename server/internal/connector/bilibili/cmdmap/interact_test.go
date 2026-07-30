package cmdmap

import (
	"testing"

	"github.com/MrZhongzq/MagicalDanmaku/server/internal/event"
)

func TestInteractWordEnter(t *testing.T) {
	evs, err := Map(testCtx(), loadSample(t, "INTERACT_WORD_enter"))
	if err != nil {
		t.Fatalf("Map 失败: %v", err)
	}
	if len(evs) != 1 || evs[0].Type != event.TypeUserEnter {
		t.Fatalf("结果错误: %+v", evs)
	}

	e := evs[0].Payload.(event.UserEnter)
	if e.User.UID != "20285041" {
		t.Errorf("UID = %q", e.User.UID)
	}
	if e.User.Username != "进场用户" {
		t.Errorf("Username = %q", e.User.Username)
	}
	if e.User.AvatarURL != "https://i0.hdslb.com/bfs/face/ddd.jpg" {
		t.Errorf("AvatarURL = %q，应从 uinfo.base.face 取得", e.User.AvatarURL)
	}
	if e.User.WealthLevel != 12 {
		t.Errorf("WealthLevel = %d，应从 uinfo.wealth.level 取得", e.User.WealthLevel)
	}
	if e.User.Medal != nil {
		t.Errorf("medal_level=0 应解析为 nil，实际 %+v", e.User.Medal)
	}
	if got := evs[0].Timestamp.Unix(); got != 1617974941 {
		t.Errorf("Timestamp = %d", got)
	}
}

func TestInteractWordFollow(t *testing.T) {
	evs, err := Map(testCtx(), loadSample(t, "INTERACT_WORD_follow"))
	if err != nil {
		t.Fatalf("Map 失败: %v", err)
	}
	if evs[0].Type != event.TypeUserFollow {
		t.Fatalf("Type = %s, 期望 %s", evs[0].Type, event.TypeUserFollow)
	}
	f := evs[0].Payload.(event.UserFollow)
	if f.User.Username != "新粉丝" {
		t.Errorf("Username = %q", f.User.Username)
	}
	if f.User.Medal == nil || f.User.Medal.Level != 15 {
		t.Errorf("Medal = %+v", f.User.Medal)
	}
	if f.User.GuardLevel != event.GuardCaptain {
		t.Errorf("GuardLevel = %d，应从 fans_medal.guard_level 取得", f.User.GuardLevel)
	}
}

func TestInteractWordShare(t *testing.T) {
	evs, err := Map(testCtx(), loadSample(t, "INTERACT_WORD_share"))
	if err != nil {
		t.Fatalf("Map 失败: %v", err)
	}
	if evs[0].Type != event.TypeUserShare {
		t.Fatalf("Type = %s, 期望 %s", evs[0].Type, event.TypeUserShare)
	}
	if _, ok := evs[0].Payload.(event.UserShare); !ok {
		t.Fatalf("载荷类型 = %T", evs[0].Payload)
	}
}

func TestEntryEffect(t *testing.T) {
	evs, err := Map(testCtx(), loadSample(t, "ENTRY_EFFECT_guard"))
	if err != nil {
		t.Fatalf("Map 失败: %v", err)
	}
	if evs[0].Type != event.TypeUserEnter {
		t.Fatalf("Type = %s, 期望 %s", evs[0].Type, event.TypeUserEnter)
	}

	e := evs[0].Payload.(event.UserEnter)
	if e.User.UID != "55555555" {
		t.Errorf("UID = %q", e.User.UID)
	}
	if e.User.GuardLevel != event.GuardCaptain {
		t.Errorf("GuardLevel = %d，应从 privilege_type 取得", e.User.GuardLevel)
	}
	if e.User.AvatarURL != "https://i0.hdslb.com/bfs/face/eee.jpg" {
		t.Errorf("AvatarURL = %q", e.User.AvatarURL)
	}
	// ENTRY_EFFECT 不含昵称字段，允许为空
	if e.User.Username != "" {
		t.Errorf("Username = %q, 期望空串", e.User.Username)
	}
}

func TestLikeInfoClick(t *testing.T) {
	evs, err := Map(testCtx(), loadSample(t, "LIKE_INFO_V3_CLICK_basic"))
	if err != nil {
		t.Fatalf("Map 失败: %v", err)
	}
	if evs[0].Type != event.TypeUserLike {
		t.Fatalf("Type = %s, 期望 %s", evs[0].Type, event.TypeUserLike)
	}
	l := evs[0].Payload.(event.UserLike)
	if l.User.Username != "点赞用户" {
		t.Errorf("Username = %q", l.User.Username)
	}
	if l.User.WealthLevel != 5 {
		t.Errorf("WealthLevel = %d", l.User.WealthLevel)
	}
}
