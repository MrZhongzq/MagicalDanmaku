package rules

import (
	"strings"
	"testing"
	"time"

	"github.com/MrZhongzq/MagicalDanmaku/server/internal/event"
)

func TestRuleValidateAcceptsEventRule(t *testing.T) {
	r := Rule{
		Name: "进场欢迎",
		On:   []event.Type{event.TypeUserEnter},
		Do:   []Action{{Type: ActionDanmaku, Template: []string{"欢迎"}}},
	}
	if err := r.Validate(); err != nil {
		t.Errorf("合法的事件规则不应报错: %v", err)
	}
}

func TestRuleValidateAcceptsScheduledRule(t *testing.T) {
	r := Rule{
		Name:     "定时广告",
		Schedule: "0 */5 * * * *",
		Do:       []Action{{Type: ActionDanmaku, Template: []string{"关注主播"}}},
	}
	if err := r.Validate(); err != nil {
		t.Errorf("合法的定时规则不应报错: %v", err)
	}
}

func TestRuleValidateRejectsBothTriggers(t *testing.T) {
	// On 与 Schedule 互斥
	r := Rule{
		Name:     "两种触发都写了",
		On:       []event.Type{event.TypeDanmaku},
		Schedule: "0 */5 * * * *",
		Do:       []Action{{Type: ActionDanmaku, Template: []string{"x"}}},
	}
	err := r.Validate()
	if err == nil {
		t.Fatal("On 与 Schedule 同时存在应当报错")
	}
	if !strings.Contains(err.Error(), "互斥") {
		t.Errorf("错误信息应说明互斥关系，实际: %v", err)
	}
}

func TestRuleValidateRejectsNoTrigger(t *testing.T) {
	r := Rule{Name: "没有触发条件", Do: []Action{{Type: ActionDanmaku, Template: []string{"x"}}}}
	if err := r.Validate(); err == nil {
		t.Error("既无 On 也无 Schedule 应当报错")
	}
}

func TestRuleValidateRejectsEmptyName(t *testing.T) {
	r := Rule{On: []event.Type{event.TypeDanmaku}, Do: []Action{{Type: ActionLog}}}
	if err := r.Validate(); err == nil {
		t.Error("规则名为空应当报错")
	}
}

func TestRuleValidateRejectsNoAction(t *testing.T) {
	r := Rule{Name: "无动作", On: []event.Type{event.TypeDanmaku}}
	if err := r.Validate(); err == nil {
		t.Error("动作列表为空应当报错")
	}
}

func TestConditionValidateAcceptsLeaf(t *testing.T) {
	c := Condition{Field: "user.guardLevel", Op: "gt", Value: 0}
	if err := c.Validate(); err != nil {
		t.Errorf("合法叶子条件不应报错: %v", err)
	}
}

func TestConditionValidateRejectsMultipleForms(t *testing.T) {
	// Field / All / Any / Not / Script 只能有一个生效
	c := Condition{
		Field: "text",
		Op:    "contains",
		Value: "x",
		Any:   []Condition{{Field: "text", Op: "eq", Value: "y"}},
	}
	err := c.Validate()
	if err == nil {
		t.Fatal("同时指定叶子与分支应当报错")
	}
	if !strings.Contains(err.Error(), "只能") {
		t.Errorf("错误信息应说明互斥，实际: %v", err)
	}
}

func TestConditionValidateRejectsEmpty(t *testing.T) {
	if err := (Condition{}).Validate(); err == nil {
		t.Error("空条件应当报错")
	}
}

func TestConditionValidateRejectsUnknownOp(t *testing.T) {
	c := Condition{Field: "text", Op: "不存在的操作符", Value: "x"}
	if err := c.Validate(); err == nil {
		t.Error("未知操作符应当报错")
	}
}

func TestConditionValidateRecursesIntoBranches(t *testing.T) {
	c := Condition{All: []Condition{
		{Field: "text", Op: "contains", Value: "ok"},
		{Field: "text", Op: "坏操作符", Value: "x"},
	}}
	if err := c.Validate(); err == nil {
		t.Error("分支内的非法子条件应当被发现")
	}
}

func TestActionValidateRejectsDanmakuWithoutTemplate(t *testing.T) {
	if err := (Action{Type: ActionDanmaku}).Validate(); err == nil {
		t.Error("danmaku 动作缺少模板应当报错")
	}
}

func TestActionValidateRejectsScriptWithoutCode(t *testing.T) {
	if err := (Action{Type: ActionScript}).Validate(); err == nil {
		t.Error("script 动作缺少代码应当报错")
	}
}

func TestActionValidateRejectsUnknownType(t *testing.T) {
	if err := (Action{Type: "不存在的动作"}).Validate(); err == nil {
		t.Error("未知动作类型应当报错")
	}
}

func TestActionValidateAcceptsKnownPickValues(t *testing.T) {
	for _, pick := range []string{"", PickRandom, PickSequential} {
		a := Action{Type: ActionDanmaku, Template: []string{"x"}, Pick: pick}
		if err := a.Validate(); err != nil {
			t.Errorf("pick=%q 不应报错: %v", pick, err)
		}
	}
}

func TestActionValidateRejectsUnknownPick(t *testing.T) {
	a := Action{Type: ActionDanmaku, Template: []string{"x"}, Pick: "不存在的取法"}
	err := a.Validate()
	if err == nil {
		t.Fatal("未知的 pick 取值应当报错")
	}
	if !strings.Contains(err.Error(), PickRandom) || !strings.Contains(err.Error(), PickSequential) {
		t.Errorf("错误信息应列出合法值 %q 与 %q，实际: %v", PickRandom, PickSequential, err)
	}
}

func TestAggregateSpecValidate(t *testing.T) {
	ok := AggregateSpec{Window: 2 * time.Second, By: AggregateByType}
	if err := ok.Validate(); err != nil {
		t.Errorf("合法合并规格不应报错: %v", err)
	}
	if err := (AggregateSpec{Window: 0, By: AggregateByType}).Validate(); err == nil {
		t.Error("窗口为 0 应当报错")
	}
	if err := (AggregateSpec{Window: time.Second, By: "坏分组"}).Validate(); err == nil {
		t.Error("未知分组键应当报错")
	}
}

func TestTriggerHoldsEvents(t *testing.T) {
	ev := event.Event{Type: event.TypeDanmaku, RoomID: "1"}
	tr := Trigger{Type: event.TypeDanmaku, Events: []event.Event{ev}, Vars: map[string]any{"text": "hi"}}
	if len(tr.Events) != 1 {
		t.Errorf("Events 长度 = %d", len(tr.Events))
	}
	if tr.Vars["text"] != "hi" {
		t.Errorf("Vars 取值错误: %v", tr.Vars)
	}
}
