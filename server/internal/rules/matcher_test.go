package rules

import (
	"testing"
	"time"

	"github.com/MrZhongzq/MagicalDanmaku/server/internal/event"
)

func danmakuTrigger(text string, guardLevel int) Trigger {
	ev := event.Event{
		Type: event.TypeDanmaku, RoomID: "1", Timestamp: time.Unix(1700000000, 0),
		Payload: event.Danmaku{
			User: event.User{UID: "123", Username: "甲", GuardLevel: guardLevel},
			Text: text,
		},
	}
	return PassthroughTrigger(ev)
}

func ruleNames(rs []Rule) []string {
	out := make([]string, 0, len(rs))
	for _, r := range rs {
		out = append(out, r.Name)
	}
	return out
}

func TestMatcherMatchesByEventType(t *testing.T) {
	rs := []Rule{
		{Name: "弹幕规则", Enabled: true, On: []event.Type{event.TypeDanmaku},
			Do: []Action{{Type: ActionLog}}},
		{Name: "礼物规则", Enabled: true, On: []event.Type{event.TypeGift},
			Do: []Action{{Type: ActionLog}}},
	}
	m := NewMatcher(rs, NewEvaluator(nil), nil)

	got := m.Match(danmakuTrigger("你好", 0))
	if len(got) != 1 || got[0].Name != "弹幕规则" {
		t.Errorf("应只命中弹幕规则，实际 %v", ruleNames(got))
	}
}

func TestMatcherSkipsDisabled(t *testing.T) {
	rs := []Rule{
		{Name: "已禁用", Enabled: false, On: []event.Type{event.TypeDanmaku},
			Do: []Action{{Type: ActionLog}}},
	}
	m := NewMatcher(rs, NewEvaluator(nil), nil)
	if got := m.Match(danmakuTrigger("你好", 0)); len(got) != 0 {
		t.Errorf("禁用的规则不应命中，实际 %v", ruleNames(got))
	}
}

func TestMatcherAppliesCondition(t *testing.T) {
	rs := []Rule{
		{Name: "仅舰长", Enabled: true, On: []event.Type{event.TypeDanmaku},
			When: &Condition{Field: "user.guardLevel", Op: "gt", Value: 0},
			Do:   []Action{{Type: ActionLog}}},
	}
	m := NewMatcher(rs, NewEvaluator(nil), nil)

	if got := m.Match(danmakuTrigger("你好", 3)); len(got) != 1 {
		t.Errorf("舰长应命中，实际 %v", ruleNames(got))
	}
	if got := m.Match(danmakuTrigger("你好", 0)); len(got) != 0 {
		t.Errorf("非舰长不应命中，实际 %v", ruleNames(got))
	}
}

func TestMatcherNilConditionAlwaysMatches(t *testing.T) {
	rs := []Rule{
		{Name: "无条件", Enabled: true, On: []event.Type{event.TypeDanmaku},
			Do: []Action{{Type: ActionLog}}},
	}
	m := NewMatcher(rs, NewEvaluator(nil), nil)
	if got := m.Match(danmakuTrigger("任意", 0)); len(got) != 1 {
		t.Error("无条件规则应总是命中")
	}
}

func TestMatcherPreservesConfigOrder(t *testing.T) {
	rs := []Rule{
		{Name: "第一条", Enabled: true, On: []event.Type{event.TypeDanmaku}, Do: []Action{{Type: ActionLog}}},
		{Name: "第二条", Enabled: true, On: []event.Type{event.TypeDanmaku}, Do: []Action{{Type: ActionLog}}},
		{Name: "第三条", Enabled: true, On: []event.Type{event.TypeDanmaku}, Do: []Action{{Type: ActionLog}}},
	}
	m := NewMatcher(rs, NewEvaluator(nil), nil)

	got := ruleNames(m.Match(danmakuTrigger("你好", 0)))
	want := []string{"第一条", "第二条", "第三条"}
	if len(got) != len(want) {
		t.Fatalf("命中数 = %d, 期望 %d", len(got), len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("顺序错误: %v", got)
		}
	}
}

func TestMatcherMultipleEventTypes(t *testing.T) {
	rs := []Rule{
		{Name: "多类型", Enabled: true,
			On: []event.Type{event.TypeDanmaku, event.TypeGift},
			Do: []Action{{Type: ActionLog}}},
	}
	m := NewMatcher(rs, NewEvaluator(nil), nil)
	if got := m.Match(danmakuTrigger("你好", 0)); len(got) != 1 {
		t.Error("弹幕应命中")
	}
}

func TestMatcherErrorIsolation(t *testing.T) {
	// 第一条规则的正则非法，不应影响第二条
	rs := []Rule{
		{Name: "坏正则", Enabled: true, On: []event.Type{event.TypeDanmaku},
			When: &Condition{Field: "text", Op: "regex", Value: "([("},
			Do:   []Action{{Type: ActionLog}}},
		{Name: "正常规则", Enabled: true, On: []event.Type{event.TypeDanmaku},
			Do: []Action{{Type: ActionLog}}},
	}
	m := NewMatcher(rs, NewEvaluator(nil), nil)

	got := ruleNames(m.Match(danmakuTrigger("你好", 0)))
	if len(got) != 1 || got[0] != "正常规则" {
		t.Errorf("出错的规则应被跳过而不影响其他规则，实际 %v", got)
	}
}

func TestMatcherScheduledRulesExcludedFromEventMatch(t *testing.T) {
	rs := []Rule{
		{Name: "定时任务", Enabled: true, Schedule: "0 */5 * * * *", Do: []Action{{Type: ActionLog}}},
		{Name: "事件规则", Enabled: true, On: []event.Type{event.TypeDanmaku}, Do: []Action{{Type: ActionLog}}},
	}
	m := NewMatcher(rs, NewEvaluator(nil), nil)

	got := ruleNames(m.Match(danmakuTrigger("你好", 0)))
	if len(got) != 1 || got[0] != "事件规则" {
		t.Errorf("定时规则不应被事件触发，实际 %v", got)
	}

	sched := ruleNames(m.ScheduledRules())
	if len(sched) != 1 || sched[0] != "定时任务" {
		t.Errorf("ScheduledRules = %v", sched)
	}
}

func TestMatcherRulesForType(t *testing.T) {
	rs := []Rule{
		{Name: "A", Enabled: true, On: []event.Type{event.TypeDanmaku}, Do: []Action{{Type: ActionLog}}},
		{Name: "B", Enabled: true, On: []event.Type{event.TypeGift}, Do: []Action{{Type: ActionLog}}},
		{Name: "C", Enabled: false, On: []event.Type{event.TypeDanmaku}, Do: []Action{{Type: ActionLog}}},
	}
	m := NewMatcher(rs, NewEvaluator(nil), nil)

	got := ruleNames(m.RulesFor(event.TypeDanmaku))
	if len(got) != 1 || got[0] != "A" {
		t.Errorf("RulesFor = %v，应只含启用的弹幕规则", got)
	}
}
