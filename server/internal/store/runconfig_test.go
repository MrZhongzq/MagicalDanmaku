package store

import (
	"context"
	"testing"
	"time"

	"github.com/MrZhongzq/MagicalDanmaku/server/internal/rules/spec"
)

func TestLoadRunConfigAssemblesEverything(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()

	owner := mustUser(t, s, "张三")
	acc, err := s.CreateAccount(ctx, AccountInput{
		Name: "小号", UID: "42", Cookie: "SESSDATA=abc",
		RateLimit: 2 * time.Second, MaxLength: 30, OwnerID: owner,
	})
	if err != nil {
		t.Fatalf("创建账号报错: %v", err)
	}
	b, err := s.UpsertBinding(ctx, acc.ID, "1706666491")
	if err != nil {
		t.Fatalf("创建绑定报错: %v", err)
	}
	if err := s.SetCooldownGroups(ctx, b.ID, map[string]time.Duration{
		"greeting": 5 * time.Second,
	}); err != nil {
		t.Fatalf("写入冷却组报错: %v", err)
	}
	if _, err := s.SaveRule(ctx, b.ID, 0, spec.Rule{
		Name: "礼物答谢", On: []string{"gift"},
		Do: []spec.Action{{Type: "danmaku", Template: []string{"谢谢"}}},
	}); err != nil {
		t.Fatalf("保存规则报错: %v", err)
	}

	cfgs, err := s.LoadRunConfig(ctx)
	if err != nil {
		t.Fatalf("载入运行配置报错: %v", err)
	}
	if len(cfgs) != 1 {
		t.Fatalf("运行单元数 = %d, 期望 1", len(cfgs))
	}

	c := cfgs[0]
	if c.AccountName != "小号" || c.RoomID != "1706666491" {
		t.Errorf("绑定 = %s", c.Label())
	}
	if c.Cookie != "SESSDATA=abc" {
		t.Errorf("Cookie = %q", c.Cookie)
	}
	if c.RateLimit != 2*time.Second || c.MaxLength != 30 {
		t.Errorf("账号参数 = %v / %d", c.RateLimit, c.MaxLength)
	}
	if c.CooldownGroups["greeting"] != 5*time.Second {
		t.Errorf("冷却组 = %v", c.CooldownGroups)
	}
	if len(c.Rules) != 1 || c.Rules[0].Name != "礼物答谢" {
		t.Errorf("规则 = %+v", c.Rules)
	}
	if c.BindingID != b.ID || c.AccountID != acc.ID {
		t.Errorf("ID 未带出: binding=%d account=%d", c.BindingID, c.AccountID)
	}
}

func TestRunConfigLabel(t *testing.T) {
	c := RunConfig{AccountName: "小号", RoomID: "123"}
	if c.Label() != "小号@123" {
		t.Errorf("Label() = %q", c.Label())
	}
}

func TestLoadRunConfigSkipsDisabledBindings(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	accID := mustAccount(t, s, "小号")

	if _, err := s.UpsertBinding(ctx, accID, "111"); err != nil {
		t.Fatalf("创建绑定报错: %v", err)
	}
	if _, err := s.UpsertBinding(ctx, accID, "222"); err != nil {
		t.Fatalf("创建绑定报错: %v", err)
	}
	if err := s.SetBindingEnabled(ctx, "小号", "222", false); err != nil {
		t.Fatalf("停用绑定报错: %v", err)
	}

	cfgs, err := s.LoadRunConfig(ctx)
	if err != nil {
		t.Fatalf("载入运行配置报错: %v", err)
	}
	if len(cfgs) != 1 || cfgs[0].RoomID != "111" {
		t.Errorf("停用的绑定不该出现，实际 %+v", cfgs)
	}
}

// 停用的规则要带出来，由引擎自己跳过——引擎需要知道它存在才能在日志里报告
func TestLoadRunConfigIncludesDisabledRules(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	bid := mustBinding(t, s, "小号", "123")

	if _, err := s.SaveRule(ctx, bid, 0, spec.Rule{
		Name: "关着的", On: []string{"danmaku"}, Do: []spec.Action{{Type: "log"}},
	}); err != nil {
		t.Fatalf("保存规则报错: %v", err)
	}
	if err := s.SetRuleEnabled(ctx, bid, "关着的", false); err != nil {
		t.Fatalf("停用规则报错: %v", err)
	}

	cfgs, err := s.LoadRunConfig(ctx)
	if err != nil {
		t.Fatalf("载入运行配置报错: %v", err)
	}
	if len(cfgs) != 1 || len(cfgs[0].Rules) != 1 {
		t.Fatalf("规则应被带出: %+v", cfgs)
	}
	if cfgs[0].Rules[0].Enabled {
		t.Error("停用的规则 Enabled 应为 false")
	}
}

func TestLoadRunConfigPreservesRulePosition(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	bid := mustBinding(t, s, "小号", "123")

	if err := s.ReplaceRules(ctx, bid, []spec.Rule{
		{Name: "甲", On: []string{"danmaku"}, Do: []spec.Action{{Type: "log"}}},
		{Name: "乙", On: []string{"gift"}, Do: []spec.Action{{Type: "log"}}},
		{Name: "丙", On: []string{"guard_buy"}, Do: []spec.Action{{Type: "log"}}},
	}); err != nil {
		t.Fatalf("替换规则报错: %v", err)
	}

	cfgs, err := s.LoadRunConfig(ctx)
	if err != nil {
		t.Fatalf("载入运行配置报错: %v", err)
	}
	got := []string{cfgs[0].Rules[0].Name, cfgs[0].Rules[1].Name, cfgs[0].Rules[2].Name}
	want := []string{"甲", "乙", "丙"}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("规则顺序 = %v, 期望 %v", got, want)
		}
	}
}

func TestLoadRunConfigEmptyDatabase(t *testing.T) {
	s := testStore(t)
	cfgs, err := s.LoadRunConfig(context.Background())
	if err != nil {
		t.Fatalf("空库应正常返回而非报错: %v", err)
	}
	if len(cfgs) != 0 {
		t.Errorf("空库应返回空列表，实际 %+v", cfgs)
	}
}

// 同一直播间被两个账号连接时是两条独立运行单元
func TestLoadRunConfigTwoAccountsSameRoom(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	main := mustAccount(t, s, "主播号")
	sub := mustAccount(t, s, "小号")

	if _, err := s.UpsertBinding(ctx, main, "1706666491"); err != nil {
		t.Fatalf("主播号绑定报错: %v", err)
	}
	if _, err := s.UpsertBinding(ctx, sub, "1706666491"); err != nil {
		t.Fatalf("小号绑定报错: %v", err)
	}

	cfgs, err := s.LoadRunConfig(ctx)
	if err != nil {
		t.Fatalf("载入运行配置报错: %v", err)
	}
	if len(cfgs) != 2 {
		t.Fatalf("运行单元数 = %d, 期望 2", len(cfgs))
	}
	if cfgs[0].Label() == cfgs[1].Label() {
		t.Error("两条运行单元的标签不该相同")
	}
}
