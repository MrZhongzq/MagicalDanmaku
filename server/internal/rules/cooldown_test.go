package rules

import (
	"testing"
	"time"
)

// fakeClock 提供可控的时间源。
type fakeClock struct{ t time.Time }

func (c *fakeClock) Now() time.Time          { return c.t }
func (c *fakeClock) Advance(d time.Duration) { c.t = c.t.Add(d) }

func newTestCooldown() (*Cooldown, *fakeClock) {
	clk := &fakeClock{t: time.Unix(1700000000, 0)}
	return NewCooldown(clk.Now), clk
}

func TestCooldownAllowsFirstFire(t *testing.T) {
	cd, _ := newTestCooldown()
	r := Rule{Name: "规则A", Cooldown: 5 * time.Second}
	if !cd.Allow(r) {
		t.Error("首次触发应当放行")
	}
}

func TestCooldownBlocksWithinInterval(t *testing.T) {
	cd, clk := newTestCooldown()
	r := Rule{Name: "规则A", Cooldown: 5 * time.Second}

	if !cd.Allow(r) {
		t.Fatal("首次应放行")
	}
	clk.Advance(3 * time.Second)
	if cd.Allow(r) {
		t.Error("冷却期内应被拦截")
	}
	clk.Advance(3 * time.Second) // 累计 6s > 5s
	if !cd.Allow(r) {
		t.Error("冷却结束后应放行")
	}
}

func TestCooldownZeroMeansNoLimit(t *testing.T) {
	cd, _ := newTestCooldown()
	r := Rule{Name: "无冷却"}
	for i := 0; i < 5; i++ {
		if !cd.Allow(r) {
			t.Fatalf("第 %d 次：冷却为 0 时不应拦截", i+1)
		}
	}
}

func TestCooldownRulesAreIndependent(t *testing.T) {
	cd, _ := newTestCooldown()
	a := Rule{Name: "规则A", Cooldown: 10 * time.Second}
	b := Rule{Name: "规则B", Cooldown: 10 * time.Second}

	if !cd.Allow(a) {
		t.Fatal("A 首次应放行")
	}
	if !cd.Allow(b) {
		t.Error("B 不应受 A 的冷却影响")
	}
}

func TestCooldownGroupSharedAcrossRules(t *testing.T) {
	cd, clk := newTestCooldown()
	cd.SetGroupInterval("greeting", 10*time.Second)

	a := Rule{Name: "欢迎舰长", CooldownGroup: "greeting"}
	b := Rule{Name: "欢迎普通用户", CooldownGroup: "greeting"}

	if !cd.Allow(a) {
		t.Fatal("A 首次应放行")
	}
	if cd.Allow(b) {
		t.Error("同组的 B 应被 A 的触发拦截")
	}

	clk.Advance(11 * time.Second)
	if !cd.Allow(b) {
		t.Error("组冷却结束后 B 应放行")
	}
}

func TestCooldownGroupWithoutIntervalIsNoLimit(t *testing.T) {
	cd, _ := newTestCooldown()
	// 未通过 SetGroupInterval 配置的组不做限制
	r := Rule{Name: "规则", CooldownGroup: "未配置的组"}
	for i := 0; i < 3; i++ {
		if !cd.Allow(r) {
			t.Fatalf("第 %d 次：未配置间隔的组不应拦截", i+1)
		}
	}
}

func TestCooldownRuleAndGroupBothApply(t *testing.T) {
	cd, clk := newTestCooldown()
	cd.SetGroupInterval("thanks", 2*time.Second)
	r := Rule{Name: "礼物答谢", CooldownGroup: "thanks", Cooldown: 10 * time.Second}

	if !cd.Allow(r) {
		t.Fatal("首次应放行")
	}
	// 组冷却已过但规则冷却未过，仍应拦截
	clk.Advance(3 * time.Second)
	if cd.Allow(r) {
		t.Error("规则冷却未结束，应被拦截")
	}
	clk.Advance(8 * time.Second) // 累计 11s
	if !cd.Allow(r) {
		t.Error("两层冷却都结束后应放行")
	}
}

func TestCooldownDoesNotRecordWhenBlocked(t *testing.T) {
	cd, clk := newTestCooldown()
	r := Rule{Name: "规则", Cooldown: 10 * time.Second}

	cd.Allow(r) // t=0 放行并记录
	clk.Advance(5 * time.Second)
	cd.Allow(r) // t=5 被拦截，不应刷新记录
	clk.Advance(6 * time.Second)
	if !cd.Allow(r) {
		t.Error("被拦截的尝试不应刷新冷却起点（t=11 距 t=0 已超 10s）")
	}
}
