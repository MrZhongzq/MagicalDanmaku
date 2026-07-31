package rules

import (
	"testing"
	"time"

	"github.com/MrZhongzq/MagicalDanmaku/server/internal/event"
)

func danmakuEvent() event.Event {
	return event.Event{
		Type:      event.TypeDanmaku,
		RoomID:    "1706666491",
		Timestamp: time.Unix(1753920000, 0),
		Payload: event.Danmaku{
			User: event.User{
				UID: "12345678", Username: "路人甲",
				GuardLevel: 3, UserLevel: 18, WealthLevel: 7, IsAdmin: true,
				Medal: &event.Medal{Name: "真yu中", Level: 24, RoomID: "999"},
			},
			Text:  "主播晚上好",
			Color: "#ffffff",
		},
	}
}

func TestVarsFromDanmaku(t *testing.T) {
	v := VarsFromEvent(danmakuEvent())

	cases := map[string]any{
		"type":             "danmaku",
		"roomId":           "1706666491",
		"text":             "主播晚上好",
		"user.uid":         "12345678",
		"user.username":    "路人甲",
		"user.guardLevel":  3,
		"user.userLevel":   18,
		"user.wealthLevel": 7,
		"user.isAdmin":     true,
		"user.medal.name":  "真yu中",
		"user.medal.level": 24,
	}
	for path, want := range cases {
		got, ok := LookupPath(v, path)
		if !ok {
			t.Errorf("路径 %q 不存在", path)
			continue
		}
		if got != want {
			t.Errorf("%s = %v (%T), 期望 %v (%T)", path, got, got, want, want)
		}
	}
}

func TestVarsMissingMedalIsAbsent(t *testing.T) {
	ev := danmakuEvent()
	d := ev.Payload.(event.Danmaku)
	d.User.Medal = nil
	ev.Payload = d

	v := VarsFromEvent(ev)
	if _, ok := LookupPath(v, "user.medal.name"); ok {
		t.Error("未佩戴勋章时 user.medal.name 不应存在")
	}
	// 但 user.username 仍应存在
	if _, ok := LookupPath(v, "user.username"); !ok {
		t.Error("user.username 应当存在")
	}
}

func TestVarsFromGift(t *testing.T) {
	ev := event.Event{
		Type: event.TypeGift, RoomID: "1", Timestamp: time.Unix(1700000000, 0),
		Payload: event.Gift{
			User:     event.User{UID: "9", Username: "土豪"},
			GiftID:   31531,
			GiftName: "小花花",
			Count:    10,
			CoinType: "gold", TotalCoin: 10000, Action: "投喂",
		},
	}
	v := VarsFromEvent(ev)
	cases := map[string]any{
		"gift.name":      "小花花",
		"gift.count":     int64(10),
		"gift.coinType":  "gold",
		"gift.totalCoin": int64(10000),
		"user.username":  "土豪",
	}
	for path, want := range cases {
		got, _ := LookupPath(v, path)
		if got != want {
			t.Errorf("%s = %v (%T), 期望 %v (%T)", path, got, got, want, want)
		}
	}
}

func TestVarsFromGuardBuy(t *testing.T) {
	ev := event.Event{
		Type: event.TypeGuardBuy, RoomID: "1", Timestamp: time.Unix(1700000000, 0),
		Payload: event.GuardBuy{
			User:       event.User{UID: "9", Username: "新舰长"},
			GuardLevel: 3, GuardName: "舰长", Count: 1, Price: 198000, IsRenew: false,
		},
	}
	v := VarsFromEvent(ev)
	if got, _ := LookupPath(v, "guard.name"); got != "舰长" {
		t.Errorf("guard.name = %v", got)
	}
	if got, _ := LookupPath(v, "guard.isRenew"); got != false {
		t.Errorf("guard.isRenew = %v", got)
	}
}

func TestVarsFromSuperChat(t *testing.T) {
	ev := event.Event{
		Type: event.TypeSuperChat, RoomID: "1", Timestamp: time.Unix(1700000000, 0),
		Payload: event.SuperChat{
			User: event.User{UID: "9", Username: "SC用户"},
			Text: "加油", Price: 30, Duration: 60,
		},
	}
	v := VarsFromEvent(ev)
	if got, _ := LookupPath(v, "text"); got != "加油" {
		t.Errorf("text = %v", got)
	}
	if got, _ := LookupPath(v, "superChat.price"); got != int64(30) {
		t.Errorf("superChat.price = %v", got)
	}
}

func TestVarsFromUserEnter(t *testing.T) {
	ev := event.Event{
		Type: event.TypeUserEnter, RoomID: "1", Timestamp: time.Unix(1700000000, 0),
		Payload: event.UserEnter{User: event.User{UID: "9", Username: "进场用户", GuardLevel: 3}},
	}
	v := VarsFromEvent(ev)
	if got, _ := LookupPath(v, "user.username"); got != "进场用户" {
		t.Errorf("user.username = %v", got)
	}
	if got, _ := LookupPath(v, "user.guardLevel"); got != 3 {
		t.Errorf("user.guardLevel = %v", got)
	}
}

func TestLookupPathMissingReturnsFalse(t *testing.T) {
	v := VarsFromEvent(danmakuEvent())
	for _, p := range []string{"不存在", "user.不存在", "text.深一层", ""} {
		if got, ok := LookupPath(v, p); ok {
			t.Errorf("路径 %q 不应存在，却返回 %v", p, got)
		}
	}
}

func TestMergeVarsKeepsNonEmpty(t *testing.T) {
	// 模拟 ENTRY_EFFECT（无昵称）与 INTERACT_WORD_V2（完整）的合并
	sparse := map[string]any{
		"type": "user_enter",
		"user": map[string]any{"uid": "123", "username": "", "guardLevel": 3},
	}
	full := map[string]any{
		"type": "user_enter",
		"user": map[string]any{"uid": "123", "username": "完整昵称", "guardLevel": 0},
	}

	MergeVars(sparse, full)

	u := sparse["user"].(map[string]any)
	if u["username"] != "完整昵称" {
		t.Errorf("空值应被非空值覆盖，实际 %v", u["username"])
	}
	if u["guardLevel"] != 3 {
		t.Errorf("非空值不应被空值覆盖，实际 %v", u["guardLevel"])
	}
}

func TestMergeVarsAddsMissingKeys(t *testing.T) {
	dst := map[string]any{"a": 1}
	MergeVars(dst, map[string]any{"b": 2})
	if dst["b"] != 2 {
		t.Errorf("缺失的键应被补上，实际 %v", dst)
	}
}
