package store

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/MrZhongzq/MagicalDanmaku/server/internal/rules"
	"github.com/MrZhongzq/MagicalDanmaku/server/internal/rules/spec"
)

// mustBinding 建一个账号加绑定，返回绑定 ID。
func mustBinding(t *testing.T, s *Store, accountName, roomID string) int64 {
	t.Helper()
	accID := mustAccount(t, s, accountName)
	b, err := s.UpsertBinding(context.Background(), accID, roomID)
	if err != nil {
		t.Fatalf("创建绑定报错: %v", err)
	}
	return b.ID
}

// sampleRule 是测试用的一条完整规则。
func sampleRule() spec.Rule {
	return spec.Rule{
		Name: "舰长进场欢迎",
		On:   []string{"user_enter"},
		When: &spec.Condition{Field: "user.guardLevel", Op: ">", Value: 0},
		Aggregate: &spec.Aggregate{
			Window:   spec.Duration(3 * time.Minute),
			MinCount: 4, By: "type",
		},
		CooldownGroup: "greeting",
		Do:            []spec.Action{{Type: "danmaku", Template: []string{"欢迎回家~"}}},
	}
}

func TestSaveAndGetRule(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	bid := mustBinding(t, s, "小号", "123")

	saved, err := s.SaveRule(ctx, bid, 0, sampleRule())
	if err != nil {
		t.Fatalf("保存规则报错: %v", err)
	}
	if saved.ID == 0 || saved.Name != "舰长进场欢迎" || !saved.Enabled {
		t.Errorf("保存结果 = %+v", saved)
	}

	got, err := s.GetRule(ctx, bid, "舰长进场欢迎")
	if err != nil {
		t.Fatalf("查询规则报错: %v", err)
	}
	if got.Spec.CooldownGroup != "greeting" {
		t.Errorf("CooldownGroup = %q", got.Spec.CooldownGroup)
	}
	if got.Spec.Aggregate == nil || time.Duration(got.Spec.Aggregate.Window) != 3*time.Minute {
		t.Errorf("Aggregate = %+v", got.Spec.Aggregate)
	}
}

// name 与 enabled 是列，JSONB 里必须没有——同一个值存两处必然漂移
func TestRuleJSONBExcludesNameAndEnabled(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	bid := mustBinding(t, s, "小号", "123")

	if _, err := s.SaveRule(ctx, bid, 0, sampleRule()); err != nil {
		t.Fatalf("保存规则报错: %v", err)
	}

	var raw []byte
	if err := s.pool.QueryRow(ctx,
		`SELECT spec FROM rules WHERE binding_id = $1`, bid).Scan(&raw); err != nil {
		t.Fatalf("读取 JSONB 报错: %v", err)
	}

	var m map[string]any
	if err := json.Unmarshal(raw, &m); err != nil {
		t.Fatalf("解析 JSONB 报错: %v", err)
	}
	if _, ok := m["name"]; ok {
		t.Error("JSONB 里不该有 name，它是列")
	}
	if _, ok := m["enabled"]; ok {
		t.Error("JSONB 里不该有 enabled，它是列")
	}
	if _, ok := m["on"]; !ok {
		t.Error("JSONB 里应有 on")
	}
}

// 从列读回来的 name/enabled 必须填进 Spec，调用方拿到的是完整规则
func TestGetRuleAssemblesNameAndEnabledFromColumns(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	bid := mustBinding(t, s, "小号", "123")

	if _, err := s.SaveRule(ctx, bid, 0, sampleRule()); err != nil {
		t.Fatalf("保存规则报错: %v", err)
	}
	if err := s.SetRuleEnabled(ctx, bid, "舰长进场欢迎", false); err != nil {
		t.Fatalf("停用规则报错: %v", err)
	}

	got, err := s.GetRule(ctx, bid, "舰长进场欢迎")
	if err != nil {
		t.Fatalf("查询规则报错: %v", err)
	}
	if got.Spec.Name != "舰长进场欢迎" {
		t.Errorf("Spec.Name = %q, 应从列填回", got.Spec.Name)
	}
	if got.Spec.Enabled == nil || *got.Spec.Enabled {
		t.Error("Spec.Enabled 应从列填回 false")
	}

	d, err := got.Domain()
	if err != nil {
		t.Fatalf("转领域模型报错: %v", err)
	}
	if d.Enabled {
		t.Error("领域模型的 Enabled 应为 false")
	}
}

func TestSaveRuleRejectsInvalidRule(t *testing.T) {
	// 非法规则不许进库：写进去了，run 每次启动都会炸
	s := testStore(t)
	ctx := context.Background()
	bid := mustBinding(t, s, "小号", "123")

	_, err := s.SaveRule(ctx, bid, 0, spec.Rule{
		Name: "坏规则",
		On:   []string{"没有这种事件"},
		Do:   []spec.Action{{Type: "log"}},
	})
	if err == nil {
		t.Error("未知事件类型的规则应被拒绝")
	}

	_, err = s.SaveRule(ctx, bid, 0, spec.Rule{Name: "空动作", On: []string{"danmaku"}})
	if err == nil {
		t.Error("空动作列表的规则应被拒绝")
	}
}

func TestSaveRuleUpdatesExistingByName(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	bid := mustBinding(t, s, "小号", "123")

	first, err := s.SaveRule(ctx, bid, 0, sampleRule())
	if err != nil {
		t.Fatalf("首次保存报错: %v", err)
	}

	r := sampleRule()
	r.CooldownGroup = "changed"
	second, err := s.SaveRule(ctx, bid, 0, r)
	if err != nil {
		t.Fatalf("二次保存报错: %v", err)
	}

	if first.ID != second.ID {
		t.Errorf("同名规则应更新同一行，ID 从 %d 变成 %d", first.ID, second.ID)
	}
	if second.Spec.CooldownGroup != "changed" {
		t.Errorf("CooldownGroup = %q, 期望 changed", second.Spec.CooldownGroup)
	}
}

// 规则名只需在单个绑定内唯一——同一条「进场欢迎」本来就会出现在多个绑定下
func TestSameRuleNameInDifferentBindings(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	b1 := mustBinding(t, s, "小号", "111")
	b2 := mustBinding(t, s, "大号", "222")

	if _, err := s.SaveRule(ctx, b1, 0, sampleRule()); err != nil {
		t.Fatalf("绑定一保存报错: %v", err)
	}
	if _, err := s.SaveRule(ctx, b2, 0, sampleRule()); err != nil {
		t.Fatalf("绑定二保存报错: %v", err)
	}

	r1, err := s.ListRules(ctx, b1)
	if err != nil {
		t.Fatalf("列出绑定一的规则报错: %v", err)
	}
	r2, err := s.ListRules(ctx, b2)
	if err != nil {
		t.Fatalf("列出绑定二的规则报错: %v", err)
	}
	if len(r1) != 1 || len(r2) != 1 {
		t.Errorf("两个绑定各应有 1 条规则，实际 %d 与 %d", len(r1), len(r2))
	}
}

func TestListRulesOrderedByPosition(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	bid := mustBinding(t, s, "小号", "123")

	for i, name := range []string{"第三条", "第一条", "第二条"} {
		pos := map[string]int{"第一条": 0, "第二条": 1, "第三条": 2}[name]
		r := spec.Rule{Name: name, On: []string{"danmaku"}, Do: []spec.Action{{Type: "log"}}}
		if _, err := s.SaveRule(ctx, bid, pos, r); err != nil {
			t.Fatalf("保存第 %d 条报错: %v", i, err)
		}
	}

	rs, err := s.ListRules(ctx, bid)
	if err != nil {
		t.Fatalf("列出规则报错: %v", err)
	}
	if len(rs) != 3 {
		t.Fatalf("规则数 = %d, 期望 3", len(rs))
	}
	want := []string{"第一条", "第二条", "第三条"}
	for i, w := range want {
		if rs[i].Name != w {
			t.Errorf("第 %d 条 = %q, 期望 %q", i, rs[i].Name, w)
		}
	}
}

func TestGetRuleNotFound(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	bid := mustBinding(t, s, "小号", "123")

	if _, err := s.GetRule(ctx, bid, "没这条规则"); !errors.Is(err, ErrNotFound) {
		t.Errorf("应返回 ErrNotFound，实际: %v", err)
	}
}

func TestDeleteRule(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	bid := mustBinding(t, s, "小号", "123")

	if _, err := s.SaveRule(ctx, bid, 0, sampleRule()); err != nil {
		t.Fatalf("保存规则报错: %v", err)
	}
	if err := s.DeleteRule(ctx, bid, "舰长进场欢迎"); err != nil {
		t.Fatalf("删除规则报错: %v", err)
	}
	if _, err := s.GetRule(ctx, bid, "舰长进场欢迎"); !errors.Is(err, ErrNotFound) {
		t.Errorf("删除后应查不到，实际: %v", err)
	}
}

func TestReplaceRulesDropsMissingOnes(t *testing.T) {
	// import 用：YAML 里删掉的规则，重新导入后库里也该没有
	s := testStore(t)
	ctx := context.Background()
	bid := mustBinding(t, s, "小号", "123")

	a := spec.Rule{Name: "甲", On: []string{"danmaku"}, Do: []spec.Action{{Type: "log"}}}
	b := spec.Rule{Name: "乙", On: []string{"gift"}, Do: []spec.Action{{Type: "log"}}}
	if err := s.ReplaceRules(ctx, bid, []spec.Rule{a, b}); err != nil {
		t.Fatalf("首次替换报错: %v", err)
	}
	if err := s.ReplaceRules(ctx, bid, []spec.Rule{a}); err != nil {
		t.Fatalf("二次替换报错: %v", err)
	}

	rs, err := s.ListRules(ctx, bid)
	if err != nil {
		t.Fatalf("列出规则报错: %v", err)
	}
	if len(rs) != 1 || rs[0].Name != "甲" {
		t.Errorf("替换后 = %+v, 期望只剩「甲」", rs)
	}
}

func TestReplaceRulesRejectsDuplicateNames(t *testing.T) {
	// 冷却按规则名记录，同绑定内重名会互相干扰
	s := testStore(t)
	ctx := context.Background()
	bid := mustBinding(t, s, "小号", "123")

	r := spec.Rule{Name: "甲", On: []string{"danmaku"}, Do: []spec.Action{{Type: "log"}}}
	if err := s.ReplaceRules(ctx, bid, []spec.Rule{r, r}); err == nil {
		t.Error("同绑定内重名应被拒绝")
	}
}

func TestReplaceRulesIsAtomic(t *testing.T) {
	// 中途有一条非法，整批都不该落库，否则会留下半套规则
	s := testStore(t)
	ctx := context.Background()
	bid := mustBinding(t, s, "小号", "123")

	good := spec.Rule{Name: "好的", On: []string{"danmaku"}, Do: []spec.Action{{Type: "log"}}}
	bad := spec.Rule{Name: "坏的", On: []string{"没有这种事件"}, Do: []spec.Action{{Type: "log"}}}

	if err := s.ReplaceRules(ctx, bid, []spec.Rule{good, bad}); err == nil {
		t.Fatal("含非法规则的批次应整体失败")
	}
	rs, err := s.ListRules(ctx, bid)
	if err != nil {
		t.Fatalf("列出规则报错: %v", err)
	}
	if len(rs) != 0 {
		t.Errorf("失败的批次不该留下任何规则，实际 %+v", rs)
	}
}

func TestRuleRecordDomainConversion(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	bid := mustBinding(t, s, "小号", "123")

	if _, err := s.SaveRule(ctx, bid, 0, sampleRule()); err != nil {
		t.Fatalf("保存规则报错: %v", err)
	}
	rec, err := s.GetRule(ctx, bid, "舰长进场欢迎")
	if err != nil {
		t.Fatalf("查询规则报错: %v", err)
	}

	d, err := rec.Domain()
	if err != nil {
		t.Fatalf("转领域模型报错: %v", err)
	}
	if d.When == nil || d.When.Op != "gt" {
		t.Errorf("操作符别名应已归一化，实际 %+v", d.When)
	}
	if d.Aggregate == nil || d.Aggregate.By != rules.AggregateByType {
		t.Errorf("Aggregate = %+v", d.Aggregate)
	}
	if d.Aggregate.MinCount != 4 {
		t.Errorf("MinCount = %d, 期望 4", d.Aggregate.MinCount)
	}
}
