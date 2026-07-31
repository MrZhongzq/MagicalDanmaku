package rules_test

import (
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/MrZhongzq/MagicalDanmaku/server/internal/event"
	"github.com/MrZhongzq/MagicalDanmaku/server/internal/rules"
)

var errSendFailed = errors.New("发送失败")

// recordingSink 记录引擎报上来的事件与动作。
type recordingSink struct {
	mu      sync.Mutex
	events  []event.Event
	actions []recordedAction
}

type recordedAction struct {
	rule   string
	action rules.Action
	err    error
}

func (s *recordingSink) RecordEvent(ev event.Event) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.events = append(s.events, ev)
}

func (s *recordingSink) RecordAction(ruleName string, a rules.Action, _ rules.Trigger, err error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.actions = append(s.actions, recordedAction{ruleName, a, err})
}

func (s *recordingSink) snapshot() ([]event.Event, []recordedAction) {
	s.mu.Lock()
	defer s.mu.Unlock()
	evs := make([]event.Event, len(s.events))
	copy(evs, s.events)
	as := make([]recordedAction, len(s.actions))
	copy(as, s.actions)
	return evs, as
}

// fakeBot 记录发出去的弹幕。
type fakeBot struct {
	mu   sync.Mutex
	sent []string
	err  error
}

func (b *fakeBot) SendDanmaku(text string) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.err != nil {
		return b.err
	}
	b.sent = append(b.sent, text)
	return nil
}

func (b *fakeBot) Block(string, int) error { return nil }

// danmakuEvent 构造一条弹幕事件。
func danmakuEvent(uid, name, text string) event.Event {
	return event.Event{
		Type: event.TypeDanmaku,
		Payload: event.Danmaku{
			User: event.User{UID: uid, Username: name},
			Text: text,
		},
	}
}

func TestEngineRecordsEvent(t *testing.T) {
	sink := &recordingSink{}
	eng, err := rules.NewEngine(rules.EngineOptions{
		Label:    "小号@123",
		RoomID:   "123",
		Bot:      &fakeBot{},
		Activity: sink,
		Rules: []rules.Rule{{
			Name: "回复", Enabled: true, On: []event.Type{event.TypeDanmaku},
			Do: []rules.Action{{Type: rules.ActionLog}},
		}},
	})
	if err != nil {
		t.Fatalf("创建引擎报错: %v", err)
	}
	defer eng.Close()

	eng.Handle(danmakuEvent("10086", "张三", "求歌单"))

	evs, _ := sink.snapshot()
	if len(evs) != 1 {
		t.Fatalf("应记录 1 条事件，实际 %d 条", len(evs))
	}
	if evs[0].Type != event.TypeDanmaku {
		t.Errorf("事件类型 = %s", evs[0].Type)
	}
}

// 不匹配任何规则的事件也要记录：业务日志是完整的房间流水，
// 不是「触发过规则的事件」的子集
func TestEngineRecordsUnmatchedEvents(t *testing.T) {
	sink := &recordingSink{}
	eng, err := rules.NewEngine(rules.EngineOptions{
		RoomID:   "123",
		Bot:      &fakeBot{},
		Activity: sink,
		Rules: []rules.Rule{{
			Name: "只管礼物", Enabled: true, On: []event.Type{event.TypeGift},
			Do: []rules.Action{{Type: rules.ActionLog}},
		}},
	})
	if err != nil {
		t.Fatalf("创建引擎报错: %v", err)
	}
	defer eng.Close()

	eng.Handle(danmakuEvent("10086", "张三", "你好"))

	evs, actions := sink.snapshot()
	if len(evs) != 1 {
		t.Errorf("未命中规则的事件也应记录，实际 %d 条", len(evs))
	}
	if len(actions) != 0 {
		t.Errorf("未命中规则不该有动作记录，实际 %d 条", len(actions))
	}
}

func TestEngineRecordsAction(t *testing.T) {
	sink := &recordingSink{}
	bot := &fakeBot{}
	eng, err := rules.NewEngine(rules.EngineOptions{
		RoomID:   "123",
		Bot:      bot,
		Activity: sink,
		Rules: []rules.Rule{{
			Name: "关键词回复", Enabled: true, On: []event.Type{event.TypeDanmaku},
			Do: []rules.Action{{Type: rules.ActionDanmaku, Template: []string{"歌单在动态里"}}},
		}},
	})
	if err != nil {
		t.Fatalf("创建引擎报错: %v", err)
	}
	defer eng.Close()

	eng.Handle(danmakuEvent("10086", "张三", "求歌单"))

	_, actions := sink.snapshot()
	if len(actions) != 1 {
		t.Fatalf("应记录 1 条动作，实际 %d 条", len(actions))
	}
	if actions[0].rule != "关键词回复" {
		t.Errorf("规则名 = %q", actions[0].rule)
	}
	if actions[0].action.Type != rules.ActionDanmaku {
		t.Errorf("动作类型 = %s", actions[0].action.Type)
	}
	if actions[0].err != nil {
		t.Errorf("成功的动作 err 应为 nil，实际 %v", actions[0].err)
	}
}

// 失败的动作也要记录，且带上错误——「为什么没发出去」正是要查的
func TestEngineRecordsFailedAction(t *testing.T) {
	sink := &recordingSink{}
	bot := &fakeBot{err: errSendFailed}
	eng, err := rules.NewEngine(rules.EngineOptions{
		RoomID:   "123",
		Bot:      bot,
		Activity: sink,
		Rules: []rules.Rule{{
			Name: "回复", Enabled: true, On: []event.Type{event.TypeDanmaku},
			Do: []rules.Action{{Type: rules.ActionDanmaku, Template: []string{"你好"}}},
		}},
	})
	if err != nil {
		t.Fatalf("创建引擎报错: %v", err)
	}
	defer eng.Close()

	eng.Handle(danmakuEvent("10086", "张三", "hi"))

	_, actions := sink.snapshot()
	if len(actions) != 1 {
		t.Fatalf("失败的动作也应记录，实际 %d 条", len(actions))
	}
	if actions[0].err == nil {
		t.Error("失败的动作应带上错误")
	}
}

func TestEngineWithoutSinkDoesNotPanic(t *testing.T) {
	// Activity 为 nil 是常见配置（比如单机跑不接数据库）
	eng, err := rules.NewEngine(rules.EngineOptions{
		RoomID: "123",
		Bot:    &fakeBot{},
		Rules: []rules.Rule{{
			Name: "回复", Enabled: true, On: []event.Type{event.TypeDanmaku},
			Do: []rules.Action{{Type: rules.ActionLog}},
		}},
	})
	if err != nil {
		t.Fatalf("创建引擎报错: %v", err)
	}
	defer eng.Close()

	eng.Handle(danmakuEvent("10086", "张三", "hi")) // 不应 panic
}

// 合并窗口的规则：每个原始事件各记一条，动作只在窗口结算时记一条
func TestEngineRecordsEachEventButOneAggregatedAction(t *testing.T) {
	sink := &recordingSink{}
	eng, err := rules.NewEngine(rules.EngineOptions{
		RoomID:   "123",
		Bot:      &fakeBot{},
		Activity: sink,
		Rules: []rules.Rule{{
			Name: "进场欢迎", Enabled: true, On: []event.Type{event.TypeUserEnter},
			Aggregate: &rules.AggregateSpec{Window: 30 * time.Millisecond, By: rules.AggregateByType},
			Do:        []rules.Action{{Type: rules.ActionDanmaku, Template: []string{"欢迎"}}},
		}},
	})
	if err != nil {
		t.Fatalf("创建引擎报错: %v", err)
	}

	for _, uid := range []string{"1", "2", "3"} {
		eng.Handle(event.Event{
			Type:    event.TypeUserEnter,
			Payload: event.UserEnter{User: event.User{UID: uid, Username: "观众" + uid}},
		})
	}
	eng.Close() // Close 会结算未决窗口

	evs, actions := sink.snapshot()
	if len(evs) != 3 {
		t.Errorf("每个原始事件各记一条，实际 %d 条", len(evs))
	}
	if len(actions) != 1 {
		t.Errorf("合并后只该有 1 条动作记录，实际 %d 条", len(actions))
	}
}
