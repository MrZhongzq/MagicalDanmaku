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

// Schedule 触发的规则配 Suppress 是无声死配置：cron 按规则名逐条注册
// 任务，一次调用只触发一条规则（FireScheduled 直接 fireLocked，不经过
// matcher.Match 与 Handle 里的 suppressed 循环），根本不存在「本次触发
// 命中的其他规则」这个集合可供压制。不拦的话它能通过全部校验、运行时
// 被彻底忽略、不报错不记日志——与「压制不存在的规则名」是同一类问题：
// 静默不生效非常难查。
func TestRuleValidateRejectsSuppressOnScheduledRule(t *testing.T) {
	r := Rule{
		Name:     "定时压制",
		Schedule: "0 */5 * * * *",
		Suppress: []string{"某规则"},
		Do:       []Action{{Type: ActionDanmaku, Template: []string{"x"}}},
	}
	err := r.Validate()
	if err == nil {
		t.Fatal("定时规则配 suppress 应当报错")
	}
	if !strings.Contains(err.Error(), "定时") && !strings.Contains(err.Error(), "suppress") {
		t.Errorf("错误信息应说明是定时规则配 suppress 不生效，实际: %v", err)
	}
}

// On 触发的规则配 Suppress 是正常场景，不该被上一条误伤。
func TestRuleValidateAcceptsSuppressOnEventRule(t *testing.T) {
	r := Rule{
		Name:     "事件压制",
		On:       []event.Type{event.TypeUserEnter},
		Suppress: []string{"某规则"},
		Do:       []Action{{Type: ActionDanmaku, Template: []string{"x"}}},
	}
	if err := r.Validate(); err != nil {
		t.Errorf("事件驱动规则配 suppress 不应报错: %v", err)
	}
}

// Schedule 触发但没配 Suppress 的规则应正常放行。
func TestRuleValidateAcceptsScheduledRuleWithoutSuppress(t *testing.T) {
	r := Rule{
		Name:     "普通定时",
		Schedule: "0 */5 * * * *",
		Do:       []Action{{Type: ActionDanmaku, Template: []string{"x"}}},
	}
	if err := r.Validate(); err != nil {
		t.Errorf("定时规则未配 suppress 不应报错: %v", err)
	}
}

// Suppress 含自己会导致规则命中后把自己标记为压制——虽然按当前实现的
// 执行顺序不会真的自我拦截（先执行再标记），但这明显不是用户的意图，
// 应当在校验阶段就拒绝，而不是留一个费解的死配置。
func TestRuleValidateRejectsSelfSuppress(t *testing.T) {
	r := Rule{
		Name:     "自压制",
		On:       []event.Type{event.TypeUserEnter},
		Suppress: []string{"自压制"},
		Do:       []Action{{Type: ActionDanmaku, Template: []string{"x"}}},
	}
	err := r.Validate()
	if err == nil {
		t.Fatal("Suppress 包含自身规则名应当报错")
	}
	if !strings.Contains(err.Error(), "自压制") {
		t.Errorf("错误信息应提及规则名，实际: %v", err)
	}
}

// 只配 TemplateMulti、规则又没有 Aggregate 的组合必然失败：没有
// Aggregate 时走 PassthroughTrigger，count 恒为 1，永远选不中
// TemplateMulti，而 Template 是空的——每次触发都会报「模板列表为空」。
// 这种配置错误应当在 Validate 阶段就拦下，而不是留到运行期每次触发
// 都报一条查不出原因的错误。
func TestRuleValidateRejectsTemplateMultiOnlyWithoutAggregate(t *testing.T) {
	r := Rule{
		Name: "只配多人模板",
		On:   []event.Type{event.TypeUserEnter},
		Do:   []Action{{Type: ActionDanmaku, TemplateMulti: []string{"欢迎 {{join .users \"、\"}} 回家"}}},
	}
	err := r.Validate()
	if err == nil {
		t.Fatal("只配 TemplateMulti 且无 Aggregate 应当报错")
	}
	if !strings.Contains(err.Error(), "aggregate") {
		t.Errorf("错误信息应提及 aggregate，实际: %v", err)
	}
}

// 同样只配 TemplateMulti，但规则配了 Aggregate 时是合法的——用户就是
// 只要多人合并欢迎，单人不发言。Aggregate 存在时可能配了
// MinCount > 1，届时根本不会有单人触发，不该被拦。
func TestRuleValidateAcceptsTemplateMultiOnlyWithAggregate(t *testing.T) {
	r := Rule{
		Name:      "只配多人模板带合并",
		On:        []event.Type{event.TypeUserEnter},
		Aggregate: &AggregateSpec{Window: 3 * time.Second, By: AggregateByType},
		Do:        []Action{{Type: ActionDanmaku, TemplateMulti: []string{"欢迎 {{join .users \"、\"}} 回家"}}},
	}
	if err := r.Validate(); err != nil {
		t.Errorf("只配 TemplateMulti 但有 Aggregate 不应报错: %v", err)
	}
}

// 最常见的配置：只配 Template、没有 Aggregate，必须继续放行。
func TestRuleValidateAcceptsTemplateOnlyWithoutAggregate(t *testing.T) {
	r := Rule{
		Name: "只配单人模板",
		On:   []event.Type{event.TypeUserEnter},
		Do:   []Action{{Type: ActionDanmaku, Template: []string{"欢迎 {{.user.username}}"}}},
	}
	if err := r.Validate(); err != nil {
		t.Errorf("只配 Template 且无 Aggregate 不应报错: %v", err)
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

// Template 与 TemplateMulti 二选一提供即可，只配 TemplateMulti 也合法——
// 比如一条只处理合并欢迎、单人不发言的规则。
func TestActionValidateAcceptsTemplateMultiOnly(t *testing.T) {
	a := Action{Type: ActionDanmaku, TemplateMulti: []string{"欢迎 {{join .users \"、\"}} 回家"}}
	if err := a.Validate(); err != nil {
		t.Errorf("只有 TemplateMulti 不应报错: %v", err)
	}
}

// Template 与 TemplateMulti 都为空才该被拒绝。
func TestActionValidateRejectsWhenBothTemplatesEmpty(t *testing.T) {
	a := Action{Type: ActionDanmaku, Template: nil, TemplateMulti: nil}
	if err := a.Validate(); err == nil {
		t.Error("Template 与 TemplateMulti 都为空应当报错")
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
