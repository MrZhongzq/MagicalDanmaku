package event

import (
	"encoding/json"
	"testing"
	"time"
)

func TestNewIDUnique(t *testing.T) {
	seen := make(map[string]bool, 1000)
	for i := 0; i < 1000; i++ {
		id := NewID()
		if id == "" {
			t.Fatal("NewID 返回空字符串")
		}
		if seen[id] {
			t.Fatalf("NewID 产生重复 ID: %s", id)
		}
		seen[id] = true
	}
}

func TestEventHoldsTypedPayload(t *testing.T) {
	now := time.Now()
	e := Event{
		ID:         NewID(),
		RoomID:     "21452505",
		Platform:   PlatformBilibili,
		Type:       TypeDanmaku,
		Timestamp:  now,
		ReceivedAt: now,
		Payload:    Danmaku{User: User{UID: "1", Username: "甲"}, Text: "你好"},
		Raw:        json.RawMessage(`{"cmd":"DANMU_MSG"}`),
	}

	d, ok := e.Payload.(Danmaku)
	if !ok {
		t.Fatalf("载荷类型断言失败，实际为 %T", e.Payload)
	}
	if d.Text != "你好" {
		t.Errorf("Text = %q, 期望 %q", d.Text, "你好")
	}
	if e.Raw == nil {
		t.Error("Raw 不得为 nil")
	}
}

func TestAllPayloadsImplementInterface(t *testing.T) {
	payloads := []Payload{
		Danmaku{}, Gift{}, GiftCombo{}, GuardBuy{}, SuperChat{}, SuperChatDelete{},
		UserEnter{}, UserFollow{}, UserShare{}, UserLike{},
		LiveStart{}, LiveStop{}, RoomChange{}, UserBlocked{},
		OnlineRankUpdate{}, RoomStatsUpdate{}, Battle{}, Unknown{},
	}
	if len(payloads) != 18 {
		t.Fatalf("载荷类型数量 = %d, 期望 18", len(payloads))
	}
}
