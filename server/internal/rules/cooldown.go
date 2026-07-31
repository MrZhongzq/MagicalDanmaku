package rules

import (
	"sync"
	"time"
)

// Cooldown 实现绑定内的业务节流。
//
//	冷却组   多规则共享节流，如 greeting / thanks
//	规则冷却  单规则最小触发间隔
//
// 两层依次检查，任一不通过即跳过本次触发——不排队、不延迟发送。
// 弹幕时效性强，迟到几十秒的欢迎语没有意义，堆积的队列反而会在
// 冷却结束后集中喷发。
//
// 防风控的发送限流**不在这里**：它是账号级的，同一账号的全部直播间
// 必须共享，因此归 account.Account 持有（见 account 包）。本类型只管
// 单个「账号-直播间」绑定内部的业务节流。
//
// 命名冷却组取代原项目的 msgCds[100] 编号通道：运营不必再记住
// 「3 号通道是关注答谢」，且在 P4 的 WebUI 中可直接下拉选择。
type Cooldown struct {
	now func() time.Time

	mu             sync.Mutex
	ruleLastFired  map[string]time.Time // 规则名 → 上次触发时间
	groupLastFired map[string]time.Time // 组名 → 上次触发时间
	groupInterval  map[string]time.Duration
}

// NewCooldown 创建节流器。now 可为 nil，此时使用 time.Now。
func NewCooldown(now func() time.Time) *Cooldown {
	if now == nil {
		now = time.Now
	}
	return &Cooldown{
		now:            now,
		ruleLastFired:  make(map[string]time.Time),
		groupLastFired: make(map[string]time.Time),
		groupInterval:  make(map[string]time.Duration),
	}
}

// SetGroupInterval 设置命名冷却组的最小间隔。
// 未设置的组不做限制。
func (c *Cooldown) SetGroupInterval(group string, d time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.groupInterval[group] = d
}

// Allow 检查规则级与组级冷却。
//
// 返回 true 表示放行，同时记录本次触发时间；返回 false 表示拦截，
// 且不刷新任何时间戳——被拦截的尝试不该延长冷却期。
func (c *Cooldown) Allow(r Rule) bool {
	c.mu.Lock()
	defer c.mu.Unlock()

	now := c.now()

	// 规则级
	if r.Cooldown > 0 {
		if last, ok := c.ruleLastFired[r.Name]; ok && now.Sub(last) < r.Cooldown {
			return false
		}
	}

	// 组级
	if r.CooldownGroup != "" {
		if interval, ok := c.groupInterval[r.CooldownGroup]; ok && interval > 0 {
			if last, ok := c.groupLastFired[r.CooldownGroup]; ok && now.Sub(last) < interval {
				return false
			}
		}
	}

	// 全部通过，记录触发时间
	c.ruleLastFired[r.Name] = now
	if r.CooldownGroup != "" {
		c.groupLastFired[r.CooldownGroup] = now
	}
	return true
}
