package main

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/MrZhongzq/MagicalDanmaku/server/internal/event"
)

func baseEvent(t event.Type, p event.Payload) event.Event {
	return event.Event{
		ID:         "01ABC",
		RoomID:     "21452505",
		Platform:   event.PlatformBilibili,
		Type:       t,
		Timestamp:  time.Date(2026, 7, 29, 19, 23, 1, 0, time.Local),
		ReceivedAt: time.Date(2026, 7, 29, 19, 23, 1, 0, time.Local),
		Payload:    p,
		Raw:        json.RawMessage(`{}`),
	}
}

func TestRenderDanmaku(t *testing.T) {
	got := Render(baseEvent(event.TypeDanmaku, event.Danmaku{
		User: event.User{UID: "123", Username: "路人甲", UserLevel: 18, GuardLevel: event.GuardCaptain},
		Text: "主播晚上好",
	}))
	for _, want := range []string{"19:23:01", "DANMAKU", "路人甲", "123", "UL18", "舰长", "主播晚上好"} {
		if !strings.Contains(got, want) {
			t.Errorf("渲染结果缺少 %q，实际:\n%s", want, got)
		}
	}
}

func TestRenderGift(t *testing.T) {
	got := Render(baseEvent(event.TypeGift, event.Gift{
		User:     event.User{UID: "1", Username: "土豪"},
		GiftName: "小心心", Count: 3, CoinType: "silver",
	}))
	for _, want := range []string{"GIFT", "土豪", "小心心", "x3", "免费"} {
		if !strings.Contains(got, want) {
			t.Errorf("渲染结果缺少 %q，实际:\n%s", want, got)
		}
	}
}

func TestRenderGuardBuy(t *testing.T) {
	got := Render(baseEvent(event.TypeGuardBuy, event.GuardBuy{
		User: event.User{UID: "1", Username: "新舰长"}, GuardName: "舰长", Count: 1, Price: 198000,
	}))
	for _, want := range []string{"GUARD_BUY", "新舰长", "舰长", "x1"} {
		if !strings.Contains(got, want) {
			t.Errorf("渲染结果缺少 %q，实际:\n%s", want, got)
		}
	}
}

func TestRenderUnknownShowsCommand(t *testing.T) {
	got := Render(baseEvent(event.TypeUnknown, event.Unknown{Command: "LIVE_MULTI_VIEW_CHANGE"}))
	for _, want := range []string{"UNKNOWN", "LIVE_MULTI_VIEW_CHANGE", "raw"} {
		if !strings.Contains(got, want) {
			t.Errorf("渲染结果缺少 %q，实际:\n%s", want, got)
		}
	}
}

func TestRenderNeverPanics(t *testing.T) {
	payloads := []struct {
		t event.Type
		p event.Payload
	}{
		{event.TypeDanmaku, event.Danmaku{}},
		{event.TypeSuperChat, event.SuperChat{}},
		{event.TypeSuperChatDelete, event.SuperChatDelete{}},
		{event.TypeGift, event.Gift{}},
		{event.TypeGiftCombo, event.GiftCombo{}},
		{event.TypeGuardBuy, event.GuardBuy{}},
		{event.TypeUserEnter, event.UserEnter{}},
		{event.TypeUserFollow, event.UserFollow{}},
		{event.TypeUserShare, event.UserShare{}},
		{event.TypeUserLike, event.UserLike{}},
		{event.TypeLiveStart, event.LiveStart{}},
		{event.TypeLiveStop, event.LiveStop{}},
		{event.TypeRoomChange, event.RoomChange{}},
		{event.TypeUserBlocked, event.UserBlocked{}},
		{event.TypeOnlineRankUpdate, event.OnlineRankUpdate{}},
		{event.TypeRoomStatsUpdate, event.RoomStatsUpdate{}},
		{event.TypeBattle, event.Battle{}},
		{event.TypeUnknown, event.Unknown{}},
	}
	for _, tc := range payloads {
		if got := Render(baseEvent(tc.t, tc.p)); got == "" {
			t.Errorf("类型 %s 渲染为空串", tc.t)
		}
	}
}

func TestRenderOnlineRankList(t *testing.T) {
	top := []event.RankUser{
		{User: event.User{Username: "榜一"}, Rank: 1, Score: "55"},
		{User: event.User{Username: "榜二"}, Rank: 2, Score: "34"},
		{User: event.User{Username: "榜三"}, Rank: 3, Score: "33"},
		{User: event.User{Username: "榜四"}, Rank: 4, Score: "20"},
	}
	got := Render(baseEvent(event.TypeOnlineRankUpdate, event.OnlineRankUpdate{Count: -1, Top: top}))
	for _, want := range []string{"1.榜一(55)", "2.榜二(34)", "3.榜三(33)", "共 4 名"} {
		if !strings.Contains(got, want) {
			t.Errorf("渲染结果缺少 %q，实际:\n%s", want, got)
		}
	}
	if strings.Contains(got, "榜四") {
		t.Error("只应展示前 3 名，第 4 名不该出现")
	}
}

func TestRenderOnlineRankCountTakesPrecedence(t *testing.T) {
	got := Render(baseEvent(event.TypeOnlineRankUpdate, event.OnlineRankUpdate{Count: 233}))
	if !strings.Contains(got, "高能榜人数 233") {
		t.Errorf("Count >= 0 时应显示人数，实际:\n%s", got)
	}
}
