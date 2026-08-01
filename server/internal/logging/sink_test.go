package logging_test

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/MrZhongzq/MagicalDanmaku/server/internal/event"
	"github.com/MrZhongzq/MagicalDanmaku/server/internal/logging"
	"github.com/MrZhongzq/MagicalDanmaku/server/internal/rules"
	"github.com/MrZhongzq/MagicalDanmaku/server/internal/store"
)

// newTestWriter 建一个同步冲刷的写入器，方便断言。
func newTestWriter(c *collector) *logging.ActivityWriter {
	return logging.NewActivityWriter(logging.ActivityWriterOptions{
		Flush:     c.flush,
		BatchSize: 1000,
		Interval:  time.Hour,
	})
}

func TestSinkImplementsRulesActivitySink(t *testing.T) {
	var _ rules.ActivitySink = (*logging.Sink)(nil)
}

func TestSinkRecordsDanmakuEvent(t *testing.T) {
	c := newCollector()
	w := newTestWriter(c)
	bid := int64(7)
	s := w.Sink(3, bid, "1706666491")

	s.RecordEvent(event.Event{
		Type: event.TypeDanmaku,
		Payload: event.Danmaku{
			User: event.User{UID: "10086", Username: "张三"},
			Text: "求歌单",
		},
	})
	w.Close()

	rows, _ := c.snapshot()
	if len(rows) != 1 {
		t.Fatalf("应写出 1 条，实际 %d 条", len(rows))
	}
	r := rows[0]
	if r.AccountID != 3 || r.BindingID == nil || *r.BindingID != bid {
		t.Errorf("归属 ID 不对: account=%d binding=%v", r.AccountID, r.BindingID)
	}
	if r.RoomID != "1706666491" {
		t.Errorf("RoomID = %q", r.RoomID)
	}
	if r.Kind != store.ActivityEvent || r.EventType != "danmaku" {
		t.Errorf("kind/type = %s / %s", r.Kind, r.EventType)
	}
	if r.UserUID != "10086" || r.UserName != "张三" {
		t.Errorf("用户信息未提取: uid=%q name=%q", r.UserUID, r.UserName)
	}
	if r.OccurredAt.IsZero() {
		t.Error("时间戳不能为零值")
	}

	var detail map[string]any
	if err := json.Unmarshal(r.Detail, &detail); err != nil {
		t.Fatalf("detail 应是合法 JSON: %v (原始 %s)", err, r.Detail)
	}
}

// live_start/live_stop 是统计接口划分开播场次、计算直播时长的唯一依据，
// 必须记——这是 P4-3 任务 5 订正的根因：此前它们没在白名单里，
// 直播时长从没进过 activity_logs。
func TestSinkRecordsLiveStartAndStop(t *testing.T) {
	c := newCollector()
	w := newTestWriter(c)
	s := w.Sink(3, 7, "123")

	s.RecordEvent(event.Event{Type: event.TypeLiveStart, Payload: event.LiveStart{}})
	s.RecordEvent(event.Event{Type: event.TypeLiveStop, Payload: event.LiveStop{}})
	w.Close()

	rows, _ := c.snapshot()
	if len(rows) != 2 {
		t.Fatalf("应写出 2 条，实际 %d 条", len(rows))
	}
	if rows[0].EventType != "live_start" || rows[1].EventType != "live_stop" {
		t.Errorf("事件类型 = %s / %s", rows[0].EventType, rows[1].EventType)
	}
}

// 排行榜每 8 秒一条且没有分析价值，不记
func TestSinkSkipsNoiseEvents(t *testing.T) {
	c := newCollector()
	w := newTestWriter(c)
	s := w.Sink(1, 1, "123")

	s.RecordEvent(event.Event{Type: event.TypeOnlineRankUpdate})
	s.RecordEvent(event.Event{Type: event.TypeRoomStatsUpdate})
	s.RecordEvent(event.Event{Type: event.TypeUnknown})
	w.Close()

	rows, _ := c.snapshot()
	if len(rows) != 0 {
		t.Errorf("噪声事件不该入库，实际写出 %d 条: %+v", len(rows), rows)
	}
}

func TestSinkRecordsAllBusinessEventTypes(t *testing.T) {
	want := []event.Type{
		event.TypeDanmaku, event.TypeSuperChat, event.TypeGift, event.TypeGiftCombo,
		event.TypeGuardBuy, event.TypeUserEnter, event.TypeUserFollow,
		event.TypeUserShare, event.TypeUserLike, event.TypeUserBlocked,
		event.TypeLiveStart, event.TypeLiveStop,
	}
	logged := logging.DefaultLoggedEventTypes()
	for _, tp := range want {
		if !logged[tp] {
			t.Errorf("业务事件 %q 应被记录", tp)
		}
	}
	if len(logged) != len(want) {
		t.Errorf("默认记录的类型数 = %d, 期望 %d: %v", len(logged), len(want), logged)
	}
}

func TestSinkRecordsAction(t *testing.T) {
	c := newCollector()
	w := newTestWriter(c)
	s := w.Sink(3, 7, "123")

	s.RecordAction("关键词回复",
		rules.Action{Type: rules.ActionDanmaku, Template: []string{"歌单在动态里"}},
		rules.Trigger{
			Type: event.TypeDanmaku,
			Vars: map[string]any{
				"user":  map[string]any{"uid": "10086", "username": "张三"},
				"count": 1,
			},
		}, nil)
	w.Close()

	rows, _ := c.snapshot()
	if len(rows) != 1 {
		t.Fatalf("应写出 1 条，实际 %d 条", len(rows))
	}
	r := rows[0]
	if r.Kind != store.ActivityAction {
		t.Errorf("kind = %s, 期望 action", r.Kind)
	}
	if r.ActionType != "danmaku" || r.RuleName != "关键词回复" {
		t.Errorf("动作信息 = %s / %s", r.ActionType, r.RuleName)
	}
	if r.UserUID != "10086" || r.UserName != "张三" {
		t.Errorf("用户信息应从 Vars 里取: uid=%q name=%q", r.UserUID, r.UserName)
	}
}

func TestSinkRecordsFailedActionWithError(t *testing.T) {
	c := newCollector()
	w := newTestWriter(c)
	s := w.Sink(3, 7, "123")

	s.RecordAction("回复",
		rules.Action{Type: rules.ActionDanmaku},
		rules.Trigger{Type: event.TypeDanmaku, Vars: map[string]any{}},
		context.DeadlineExceeded)
	w.Close()

	rows, _ := c.snapshot()
	if len(rows) != 1 {
		t.Fatalf("失败的动作也应写出，实际 %d 条", len(rows))
	}

	var detail map[string]any
	if err := json.Unmarshal(rows[0].Detail, &detail); err != nil {
		t.Fatalf("解析 detail 报错: %v", err)
	}
	if _, ok := detail["error"]; !ok {
		t.Errorf("失败的动作应把错误写进 detail，实际 %v", detail)
	}
}

// 动作全都要记：机器人干了什么是这份日志的核心，不做类型筛选
func TestSinkRecordsEveryActionType(t *testing.T) {
	c := newCollector()
	w := newTestWriter(c)
	s := w.Sink(3, 7, "123")

	for _, at := range []rules.ActionType{
		rules.ActionDanmaku, rules.ActionBlock, rules.ActionScript, rules.ActionLog,
	} {
		s.RecordAction("规则", rules.Action{Type: at},
			rules.Trigger{Type: event.TypeDanmaku, Vars: map[string]any{}}, nil)
	}
	w.Close()

	rows, _ := c.snapshot()
	if len(rows) != 4 {
		t.Errorf("四种动作都该记，实际 %d 条", len(rows))
	}
}

// 同一个写入器分出的多个 Sink 各自带自己的归属 ID
func TestSinkPerBindingAttribution(t *testing.T) {
	c := newCollector()
	w := newTestWriter(c)

	w.Sink(1, 10, "甲").RecordEvent(event.Event{
		Type: event.TypeDanmaku, Payload: event.Danmaku{Text: "a"},
	})
	w.Sink(2, 20, "乙").RecordEvent(event.Event{
		Type: event.TypeDanmaku, Payload: event.Danmaku{Text: "b"},
	})
	w.Close()

	rows, _ := c.snapshot()
	if len(rows) != 2 {
		t.Fatalf("应写出 2 条，实际 %d 条", len(rows))
	}
	if rows[0].AccountID != 1 || rows[0].RoomID != "甲" {
		t.Errorf("第一条归属不对: %+v", rows[0])
	}
	if rows[1].AccountID != 2 || rows[1].RoomID != "乙" {
		t.Errorf("第二条归属不对: %+v", rows[1])
	}
}
