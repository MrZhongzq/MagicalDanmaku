package spec_test

import (
	"encoding/json"
	"testing"
	"time"

	"gopkg.in/yaml.v3"

	"github.com/MrZhongzq/MagicalDanmaku/server/internal/event"
	"github.com/MrZhongzq/MagicalDanmaku/server/internal/rules"
	"github.com/MrZhongzq/MagicalDanmaku/server/internal/rules/spec"
)

func TestDurationFromYAML(t *testing.T) {
	var d spec.Duration
	if err := yaml.Unmarshal([]byte(`"1.5s"`), &d); err != nil {
		t.Fatalf("解析报错: %v", err)
	}
	if time.Duration(d) != 1500*time.Millisecond {
		t.Errorf("= %v, 期望 1.5s", time.Duration(d))
	}
}

func TestDurationFromJSON(t *testing.T) {
	var d spec.Duration
	if err := json.Unmarshal([]byte(`"3m"`), &d); err != nil {
		t.Fatalf("解析报错: %v", err)
	}
	if time.Duration(d) != 3*time.Minute {
		t.Errorf("= %v, 期望 3m", time.Duration(d))
	}
}

// JSONB 里存 "3m" 而非 180000000000，是为了让人能直接看懂库里的行。
func TestDurationToJSONIsHumanReadable(t *testing.T) {
	b, err := json.Marshal(spec.Duration(3 * time.Minute))
	if err != nil {
		t.Fatalf("序列化报错: %v", err)
	}
	if string(b) != `"3m0s"` {
		t.Errorf("= %s, 期望 \"3m0s\"", b)
	}
}

func TestDurationZeroToJSON(t *testing.T) {
	b, err := json.Marshal(spec.Duration(0))
	if err != nil {
		t.Fatalf("序列化报错: %v", err)
	}
	if string(b) != `"0s"` {
		t.Errorf("= %s, 期望 \"0s\"", b)
	}
}

func TestDurationRejectsNonString(t *testing.T) {
	var d spec.Duration
	if err := json.Unmarshal([]byte(`1500`), &d); err == nil {
		t.Error("裸数字应被拒绝：单位含糊，纳秒还是毫秒说不清")
	}
	if err := yaml.Unmarshal([]byte(`1500`), &d); err == nil {
		t.Error("YAML 里的裸数字同样应被拒绝")
	}
}

func TestDurationToYAML(t *testing.T) {
	b, err := yaml.Marshal(spec.Duration(90 * time.Second))
	if err != nil {
		t.Fatalf("序列化报错: %v", err)
	}
	if string(b) != "1m30s\n" {
		t.Errorf("= %q, 期望 \"1m30s\\n\"", b)
	}
}

// 规则经过 JSON 往返后必须完全等价——这是它能存进 JSONB 的前提。
func TestRuleJSONRoundTrip(t *testing.T) {
	src := `{
		"name": "舰长进场欢迎",
		"enabled": true,
		"on": ["user_enter"],
		"when": {"field": "user.guardLevel", "op": ">", "value": 0},
		"aggregate": {"window": "3m", "maxWait": "5m", "minCount": 4, "by": "type"},
		"cooldownGroup": "greeting",
		"do": [{"type": "danmaku", "template": ["欢迎 {{join .users \"、\"}} 回家~"]}]
	}`

	var r1 spec.Rule
	if err := json.Unmarshal([]byte(src), &r1); err != nil {
		t.Fatalf("首次解析报错: %v", err)
	}

	b, err := json.Marshal(r1)
	if err != nil {
		t.Fatalf("序列化报错: %v", err)
	}

	var r2 spec.Rule
	if err := json.Unmarshal(b, &r2); err != nil {
		t.Fatalf("二次解析报错: %v", err)
	}

	d1, err := r1.ToRule()
	if err != nil {
		t.Fatalf("r1.ToRule 报错: %v", err)
	}
	d2, err := r2.ToRule()
	if err != nil {
		t.Fatalf("r2.ToRule 报错: %v", err)
	}

	if d1.Name != d2.Name || d1.CooldownGroup != d2.CooldownGroup {
		t.Errorf("往返后基本字段不一致: %+v vs %+v", d1, d2)
	}
	if d1.Aggregate.Window != d2.Aggregate.Window || d1.Aggregate.MaxWait != d2.Aggregate.MaxWait {
		t.Errorf("往返后合并窗口不一致: %+v vs %+v", d1.Aggregate, d2.Aggregate)
	}
	if d1.When.Op != d2.When.Op {
		t.Errorf("往返后操作符不一致: %q vs %q", d1.When.Op, d2.When.Op)
	}
}

// YAML 与 JSON 必须解析出同一条规则，否则「一份表示」就是空话。
func TestYAMLAndJSONAgree(t *testing.T) {
	yamlSrc := `
name: 礼物答谢
on: [gift]
aggregate:
  window: 3s
  by: gift
do:
  - type: danmaku
    template: ['感谢 {{.user.username}}']
`
	jsonSrc := `{
		"name": "礼物答谢",
		"on": ["gift"],
		"aggregate": {"window": "3s", "by": "gift"},
		"do": [{"type": "danmaku", "template": ["感谢 {{.user.username}}"]}]
	}`

	var fromYAML, fromJSON spec.Rule
	if err := yaml.Unmarshal([]byte(yamlSrc), &fromYAML); err != nil {
		t.Fatalf("YAML 解析报错: %v", err)
	}
	if err := json.Unmarshal([]byte(jsonSrc), &fromJSON); err != nil {
		t.Fatalf("JSON 解析报错: %v", err)
	}

	ry, err := fromYAML.ToRule()
	if err != nil {
		t.Fatalf("YAML 转换报错: %v", err)
	}
	rj, err := fromJSON.ToRule()
	if err != nil {
		t.Fatalf("JSON 转换报错: %v", err)
	}

	if ry.Name != rj.Name {
		t.Errorf("Name: %q vs %q", ry.Name, rj.Name)
	}
	if ry.Aggregate.Window != rj.Aggregate.Window || ry.Aggregate.By != rj.Aggregate.By {
		t.Errorf("Aggregate: %+v vs %+v", ry.Aggregate, rj.Aggregate)
	}
	if len(ry.Do) != 1 || len(rj.Do) != 1 || ry.Do[0].Template[0] != rj.Do[0].Template[0] {
		t.Errorf("Do: %+v vs %+v", ry.Do, rj.Do)
	}
}

func TestToRuleDefaultsEnabledToTrue(t *testing.T) {
	// 写了规则却不生效最反直觉，所以未写 enabled 时默认启用
	r, err := spec.Rule{
		Name: "x",
		On:   []string{"danmaku"},
		Do:   []spec.Action{{Type: "log"}},
	}.ToRule()
	if err != nil {
		t.Fatalf("ToRule 报错: %v", err)
	}
	if !r.Enabled {
		t.Error("未写 enabled 时应默认启用")
	}
}

func TestToRuleRespectsExplicitFalse(t *testing.T) {
	no := false
	r, err := spec.Rule{
		Name:    "x",
		Enabled: &no,
		On:      []string{"danmaku"},
		Do:      []spec.Action{{Type: "log"}},
	}.ToRule()
	if err != nil {
		t.Fatalf("ToRule 报错: %v", err)
	}
	if r.Enabled {
		t.Error("显式写 false 时不应启用")
	}
}

func TestToRuleNormalizesOperatorAliases(t *testing.T) {
	cases := map[string]string{
		">": "gt", ">=": "gte", "<": "lt", "<=": "lte",
		"==": "eq", "=": "eq", "!=": "ne", "<>": "ne",
	}
	for alias, want := range cases {
		r, err := spec.Rule{
			Name: "x",
			On:   []string{"danmaku"},
			When: &spec.Condition{Field: "user.guardLevel", Op: alias, Value: 0},
			Do:   []spec.Action{{Type: "log"}},
		}.ToRule()
		if err != nil {
			t.Fatalf("别名 %q 转换报错: %v", alias, err)
		}
		if r.When.Op != want {
			t.Errorf("别名 %q 归一化为 %q, 期望 %q", alias, r.When.Op, want)
		}
	}
}

func TestToRuleRejectsUnknownEventType(t *testing.T) {
	_, err := spec.Rule{
		Name: "x",
		On:   []string{"没有这种事件"},
		Do:   []spec.Action{{Type: "log"}},
	}.ToRule()
	if err == nil {
		t.Fatal("未知事件类型应报错")
	}
}

func TestToRuleRejectsBadRegexAtLoadTime(t *testing.T) {
	// 正则在加载时就编译一次，把错误从运行时提前到启动时
	_, err := spec.Rule{
		Name: "x",
		On:   []string{"danmaku"},
		When: &spec.Condition{Field: "text", Op: "regex", Value: "([unclosed"},
		Do:   []spec.Action{{Type: "log"}},
	}.ToRule()
	if err == nil {
		t.Fatal("非法正则应在转换时就报错")
	}
}

func TestToRuleRejectsBadCronSpec(t *testing.T) {
	_, err := spec.Rule{
		Name:     "x",
		Schedule: "不是 cron",
		Do:       []spec.Action{{Type: "log"}},
	}.ToRule()
	if err == nil {
		t.Fatal("非法 cron 表达式应报错")
	}
}

func TestToRuleConvertsNestedConditions(t *testing.T) {
	r, err := spec.Rule{
		Name: "x",
		On:   []string{"danmaku"},
		When: &spec.Condition{
			Any: []spec.Condition{
				{Field: "text", Op: "contains", Value: "加群"},
				{Not: &spec.Condition{Field: "user.uid", Op: "eq", Value: "1"}},
			},
		},
		Do: []spec.Action{{Type: "log"}},
	}.ToRule()
	if err != nil {
		t.Fatalf("ToRule 报错: %v", err)
	}
	if len(r.When.Any) != 2 {
		t.Fatalf("any 应有 2 项，实际 %d", len(r.When.Any))
	}
	if r.When.Any[1].Not == nil || r.When.Any[1].Not.Field != "user.uid" {
		t.Errorf("嵌套 not 未正确转换: %+v", r.When.Any[1])
	}
}

func TestToRuleMapsAllKnownEventTypes(t *testing.T) {
	// 事件类型表漏一个，用户就会莫名其妙地配不出某类规则
	for _, tp := range []event.Type{
		event.TypeDanmaku, event.TypeSuperChat, event.TypeSuperChatDelete,
		event.TypeGift, event.TypeGiftCombo, event.TypeGuardBuy,
		event.TypeUserEnter, event.TypeUserFollow, event.TypeUserShare,
		event.TypeUserLike, event.TypeLiveStart, event.TypeLiveStop,
		event.TypeRoomChange, event.TypeUserBlocked, event.TypeOnlineRankUpdate,
		event.TypeRoomStatsUpdate, event.TypeBattle, event.TypeUnknown,
	} {
		r, err := spec.Rule{
			Name: "x",
			On:   []string{string(tp)},
			Do:   []spec.Action{{Type: "log"}},
		}.ToRule()
		if err != nil {
			t.Errorf("事件类型 %q 转换报错: %v", tp, err)
			continue
		}
		if len(r.On) != 1 || r.On[0] != tp {
			t.Errorf("事件类型 %q 转换成了 %v", tp, r.On)
		}
	}
}

func TestToRuleRunsDomainValidation(t *testing.T) {
	// 动作列表为空由 rules.Rule.Validate 拦下，spec 不重复实现校验
	_, err := spec.Rule{Name: "x", On: []string{"danmaku"}}.ToRule()
	if err == nil {
		t.Fatal("空动作列表应报错")
	}
}

// Pick 字段要原样带到领域模型，否则 WebUI 的轮询开关无法生效。
func TestToRuleCarriesActionPick(t *testing.T) {
	r, err := spec.Rule{
		Name: "x",
		On:   []string{"user_enter"},
		Do: []spec.Action{
			{Type: "danmaku", Template: []string{"甲", "乙"}, Pick: "sequential"},
		},
	}.ToRule()
	if err != nil {
		t.Fatalf("ToRule 报错: %v", err)
	}
	if len(r.Do) != 1 || r.Do[0].Pick != rules.PickSequential {
		t.Errorf("Pick 未正确转换: %+v", r.Do)
	}
}

// TemplateMulti 要原样带到领域模型，否则单人/多人两套模板在数据库
// 路径下形同虚设。
func TestToRuleCarriesTemplateMulti(t *testing.T) {
	r, err := spec.Rule{
		Name: "x",
		On:   []string{"user_enter"},
		Do: []spec.Action{
			{Type: "danmaku", Template: []string{"欢迎 甲"}, TemplateMulti: []string{"欢迎 甲、乙 回家"}},
		},
	}.ToRule()
	if err != nil {
		t.Fatalf("ToRule 报错: %v", err)
	}
	if len(r.Do) != 1 || len(r.Do[0].TemplateMulti) != 1 || r.Do[0].TemplateMulti[0] != "欢迎 甲、乙 回家" {
		t.Errorf("TemplateMulti 未正确转换: %+v", r.Do)
	}
}

func TestToRuleRejectsUnknownPick(t *testing.T) {
	_, err := spec.Rule{
		Name: "x",
		On:   []string{"user_enter"},
		Do: []spec.Action{
			{Type: "danmaku", Template: []string{"甲"}, Pick: "不存在的取法"},
		},
	}.ToRule()
	if err == nil {
		t.Fatal("未知的 pick 取值应在转换时报错")
	}
}

func TestToRuleCarriesAggregateFields(t *testing.T) {
	r, err := spec.Rule{
		Name: "x",
		On:   []string{"user_enter"},
		Aggregate: &spec.Aggregate{
			Window:   spec.Duration(3 * time.Minute),
			MaxWait:  spec.Duration(5 * time.Minute),
			MinCount: 4,
			By:       "type",
		},
		Do: []spec.Action{{Type: "log"}},
	}.ToRule()
	if err != nil {
		t.Fatalf("ToRule 报错: %v", err)
	}
	want := rules.AggregateSpec{
		Window:   3 * time.Minute,
		MaxWait:  5 * time.Minute,
		MinCount: 4,
		By:       rules.AggregateByType,
	}
	if *r.Aggregate != want {
		t.Errorf("Aggregate = %+v, 期望 %+v", *r.Aggregate, want)
	}
}
