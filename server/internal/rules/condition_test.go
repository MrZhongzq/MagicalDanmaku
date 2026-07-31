package rules

import (
	"errors"
	"testing"
)

// fakeScript 是 ScriptRunner 的测试替身。
type fakeScript struct {
	result bool
	err    error
	called []string
}

func (f *fakeScript) EvalBool(code string, vars map[string]any) (bool, error) {
	f.called = append(f.called, code)
	return f.result, f.err
}

func testVars() map[string]any {
	return map[string]any{
		"type": "danmaku",
		"text": "主播晚上好，点歌一首",
		"user": map[string]any{
			"uid": "123", "username": "路人甲",
			"guardLevel": 3, "userLevel": 18, "isAdmin": false,
		},
		"gift": map[string]any{"name": "小花花", "count": int64(10)},
	}
}

func TestEvalLeafOperators(t *testing.T) {
	cases := []struct {
		name string
		c    Condition
		want bool
	}{
		{"字符串相等", Condition{Field: "user.username", Op: "eq", Value: "路人甲"}, true},
		{"字符串不等", Condition{Field: "user.username", Op: "ne", Value: "别人"}, true},
		{"包含", Condition{Field: "text", Op: "contains", Value: "点歌"}, true},
		{"不包含", Condition{Field: "text", Op: "contains", Value: "不存在"}, false},
		{"前缀", Condition{Field: "text", Op: "prefix", Value: "主播"}, true},
		{"后缀", Condition{Field: "text", Op: "suffix", Value: "一首"}, true},
		{"正则", Condition{Field: "text", Op: "regex", Value: "点歌|唱歌"}, true},
		{"正则不匹配", Condition{Field: "text", Op: "regex", Value: "^广告"}, false},
		{"数值大于", Condition{Field: "user.guardLevel", Op: "gt", Value: 0}, true},
		{"数值大于不成立", Condition{Field: "user.guardLevel", Op: "gt", Value: 3}, false},
		{"数值大于等于", Condition{Field: "user.guardLevel", Op: "gte", Value: 3}, true},
		{"数值小于", Condition{Field: "user.userLevel", Op: "lt", Value: 20}, true},
		{"数值小于等于", Condition{Field: "user.userLevel", Op: "lte", Value: 18}, true},
		{"布尔相等", Condition{Field: "user.isAdmin", Op: "eq", Value: false}, true},
		{"属于集合", Condition{Field: "user.uid", Op: "in", Value: []any{"111", "123"}}, true},
		{"不属于集合", Condition{Field: "user.uid", Op: "in", Value: []any{"111", "222"}}, false},
		{"int64 与 int 比较", Condition{Field: "gift.count", Op: "gte", Value: 10}, true},
	}

	ev := NewEvaluator(nil)
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := ev.Eval(tc.c, testVars())
			if err != nil {
				t.Fatalf("Eval 失败: %v", err)
			}
			if got != tc.want {
				t.Errorf("= %v, 期望 %v", got, tc.want)
			}
		})
	}
}

func TestEvalMissingFieldIsFalse(t *testing.T) {
	ev := NewEvaluator(nil)
	// 字段缺失应视为不匹配，而非报错
	for _, c := range []Condition{
		{Field: "不存在的字段", Op: "eq", Value: "x"},
		{Field: "user.不存在", Op: "gt", Value: 0},
		{Field: "gift.name", Op: "contains", Value: "x"},
	} {
		got, err := ev.Eval(c, map[string]any{"type": "danmaku"})
		if err != nil {
			t.Errorf("字段缺失不应报错: %v", err)
		}
		if got {
			t.Errorf("字段缺失应视为不匹配: %+v", c)
		}
	}
}

func TestEvalAll(t *testing.T) {
	ev := NewEvaluator(nil)
	c := Condition{All: []Condition{
		{Field: "user.guardLevel", Op: "gt", Value: 0},
		{Field: "text", Op: "contains", Value: "点歌"},
	}}
	got, err := ev.Eval(c, testVars())
	if err != nil {
		t.Fatalf("Eval 失败: %v", err)
	}
	if !got {
		t.Error("全部满足时应返回 true")
	}

	c.All = append(c.All, Condition{Field: "text", Op: "contains", Value: "不存在"})
	got, _ = ev.Eval(c, testVars())
	if got {
		t.Error("任一不满足时应返回 false")
	}
}

func TestEvalAny(t *testing.T) {
	ev := NewEvaluator(nil)
	c := Condition{Any: []Condition{
		{Field: "text", Op: "contains", Value: "不存在"},
		{Field: "text", Op: "contains", Value: "点歌"},
	}}
	got, err := ev.Eval(c, testVars())
	if err != nil {
		t.Fatalf("Eval 失败: %v", err)
	}
	if !got {
		t.Error("任一满足时应返回 true")
	}

	c.Any = []Condition{{Field: "text", Op: "contains", Value: "都不满足"}}
	got, _ = ev.Eval(c, testVars())
	if got {
		t.Error("全部不满足时应返回 false")
	}
}

func TestEvalNot(t *testing.T) {
	ev := NewEvaluator(nil)
	c := Condition{Not: &Condition{Field: "user.guardLevel", Op: "eq", Value: 0}}
	got, err := ev.Eval(c, testVars())
	if err != nil {
		t.Fatalf("Eval 失败: %v", err)
	}
	if !got {
		t.Error("取反后应为 true")
	}
}

func TestEvalNested(t *testing.T) {
	ev := NewEvaluator(nil)
	// (舰长 或 房管) 且 包含点歌
	c := Condition{All: []Condition{
		{Any: []Condition{
			{Field: "user.guardLevel", Op: "gt", Value: 0},
			{Field: "user.isAdmin", Op: "eq", Value: true},
		}},
		{Field: "text", Op: "contains", Value: "点歌"},
	}}
	got, err := ev.Eval(c, testVars())
	if err != nil {
		t.Fatalf("Eval 失败: %v", err)
	}
	if !got {
		t.Error("嵌套条件应为 true")
	}
}

func TestEvalScriptDelegatesToRunner(t *testing.T) {
	fs := &fakeScript{result: true}
	ev := NewEvaluator(fs)

	got, err := ev.Eval(Condition{Script: "event.text.length > 5"}, testVars())
	if err != nil {
		t.Fatalf("Eval 失败: %v", err)
	}
	if !got {
		t.Error("应返回脚本的结果")
	}
	if len(fs.called) != 1 || fs.called[0] != "event.text.length > 5" {
		t.Errorf("脚本未被正确调用: %v", fs.called)
	}
}

func TestEvalScriptWithoutRunnerFails(t *testing.T) {
	ev := NewEvaluator(nil)
	_, err := ev.Eval(Condition{Script: "true"}, testVars())
	if !errors.Is(err, ErrNoScriptRunner) {
		t.Errorf("err = %v, 期望 ErrNoScriptRunner", err)
	}
}

func TestEvalLeafDoesNotInvokeScript(t *testing.T) {
	fs := &fakeScript{result: true}
	ev := NewEvaluator(fs)

	if _, err := ev.Eval(Condition{Field: "text", Op: "contains", Value: "点歌"}, testVars()); err != nil {
		t.Fatalf("Eval 失败: %v", err)
	}
	if len(fs.called) != 0 {
		t.Errorf("结构化条件不该起 goja，实际调用了 %v", fs.called)
	}
}

func TestEvalBadRegexReturnsError(t *testing.T) {
	ev := NewEvaluator(nil)
	_, err := ev.Eval(Condition{Field: "text", Op: "regex", Value: "([("}, testVars())
	if err == nil {
		t.Error("非法正则应当报错")
	}
}

func TestEvalNilConditionIsTrue(t *testing.T) {
	ev := NewEvaluator(nil)
	// 空条件（零值 Condition）视为无条件匹配，供 Rule.When == nil 时使用
	got, err := ev.Eval(Condition{}, testVars())
	if err != nil {
		t.Fatalf("Eval 失败: %v", err)
	}
	if !got {
		t.Error("零值条件应视为无条件通过")
	}
}
