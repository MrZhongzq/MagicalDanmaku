package cmdmap

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/MrZhongzq/MagicalDanmaku/server/internal/event"
)

func testCtx() Context {
	return Context{RoomID: "21452505", ReceivedAt: time.Unix(1700000000, 0)}
}

func TestCommandOfPlain(t *testing.T) {
	if got := CommandOf(json.RawMessage(`{"cmd":"SEND_GIFT"}`)); got != "SEND_GIFT" {
		t.Errorf("CommandOf = %q, 期望 SEND_GIFT", got)
	}
}

func TestCommandOfStripsSuffix(t *testing.T) {
	// B 站的弹幕 CMD 会带后缀，如 DANMU_MSG:4:0:2:2:2:0
	if got := CommandOf(json.RawMessage(`{"cmd":"DANMU_MSG:4:0:2:2:2:0"}`)); got != "DANMU_MSG" {
		t.Errorf("CommandOf = %q, 期望 DANMU_MSG", got)
	}
}

func TestMapFallsBackToUnknown(t *testing.T) {
	raw := json.RawMessage(`{"cmd":"TOTALLY_NEW_THING","data":{"a":1}}`)
	evs, err := Map(testCtx(), raw)
	if err != nil {
		t.Fatalf("Map 失败: %v", err)
	}
	if len(evs) != 1 {
		t.Fatalf("事件数 = %d, 期望 1", len(evs))
	}
	if evs[0].Type != event.TypeUnknown {
		t.Errorf("Type = %s, 期望 %s", evs[0].Type, event.TypeUnknown)
	}
	u, ok := evs[0].Payload.(event.Unknown)
	if !ok {
		t.Fatalf("载荷类型 = %T, 期望 event.Unknown", evs[0].Payload)
	}
	if u.Command != "TOTALLY_NEW_THING" {
		t.Errorf("Command = %q", u.Command)
	}
	if string(evs[0].Raw) != string(raw) {
		t.Error("Raw 必须原样保留")
	}
}

func TestMapUsesRegisteredMapper(t *testing.T) {
	Register("TEST_ONLY_CMD", func(ctx Context, raw json.RawMessage) ([]event.Event, error) {
		return []event.Event{NewEvent(ctx, event.TypeLiveStart, ctx.ReceivedAt, event.LiveStart{}, raw)}, nil
	})
	evs, err := Map(testCtx(), json.RawMessage(`{"cmd":"TEST_ONLY_CMD"}`))
	if err != nil {
		t.Fatalf("Map 失败: %v", err)
	}
	if len(evs) != 1 || evs[0].Type != event.TypeLiveStart {
		t.Fatalf("结果错误: %+v", evs)
	}
}

func TestNewEventAlwaysFillsRequiredFields(t *testing.T) {
	raw := json.RawMessage(`{"cmd":"X"}`)
	e := NewEvent(testCtx(), event.TypeLiveStop, time.Time{}, event.LiveStop{}, raw)

	if e.ID == "" {
		t.Error("ID 不得为空")
	}
	if e.RoomID != "21452505" {
		t.Errorf("RoomID = %q", e.RoomID)
	}
	if e.Platform != event.PlatformBilibili {
		t.Errorf("Platform = %q", e.Platform)
	}
	if e.Raw == nil {
		t.Error("Raw 不得为 nil")
	}
	// 传入零值时间时，Timestamp 应回落到 ReceivedAt
	if !e.Timestamp.Equal(e.ReceivedAt) {
		t.Errorf("零值 Timestamp 应回落到 ReceivedAt，实际 %v vs %v", e.Timestamp, e.ReceivedAt)
	}
}
