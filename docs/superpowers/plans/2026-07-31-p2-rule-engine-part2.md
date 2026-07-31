# P2 规则引擎 Implementation Plan · Part 2（冷却、合并、匹配、执行）

> 续 `2026-07-31-p2-rule-engine.md`。执行前请先完成 Task 1–5。
> Global Constraints 沿用 Part 1，此处不再重复。

本篇覆盖 Task 6–10。

---

### Task 6: 三层冷却

**Files:**
- Create: `server/internal/rules/cooldown.go`
- Test: `server/internal/rules/cooldown_test.go`

**Interfaces:**
- Consumes: Task 1 的 `Rule`；P0 的 `ratelimit.Limiter`
- Produces:
  - `rules.Cooldown` 结构，`rules.NewCooldown(global ratelimit.Limiter, now func() time.Time) *Cooldown`
  - `(*Cooldown).Allow(r Rule) bool` — 检查规则冷却与冷却组，通过则记录触发时间
  - `(*Cooldown).SetGroupInterval(group string, d time.Duration)`
  - `(*Cooldown).WaitGlobal(ctx context.Context) error` — 全局限流，动作真正发送前调用

**设计要点：** 三层**依次检查，任一不通过即跳过本次触发**——不排队、不
延迟发送。理由：弹幕的时效性强，迟到几十秒的欢迎语没有意义，堆积的队列
反而会在冷却结束后集中喷发。

`Allow` 只管规则级与组级（纯时间比较，不阻塞）；全局限流因为要真正等待，
拆成独立的 `WaitGlobal`，在动作发送前调用。

- [ ] **Step 1: 写失败测试**

创建 `server/internal/rules/cooldown_test.go`：

```go
package rules

import (
	"context"
	"testing"
	"time"

	"github.com/MrZhongzq/MagicalDanmaku/server/internal/ratelimit"
)

// fakeClock 提供可控的时间源。
type fakeClock struct{ t time.Time }

func (c *fakeClock) Now() time.Time      { return c.t }
func (c *fakeClock) Advance(d time.Duration) { c.t = c.t.Add(d) }

func newTestCooldown() (*Cooldown, *fakeClock) {
	clk := &fakeClock{t: time.Unix(1700000000, 0)}
	return NewCooldown(ratelimit.NewInterval(0), clk.Now), clk
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

	cd.Allow(r)              // t=0 放行并记录
	clk.Advance(5 * time.Second)
	cd.Allow(r)              // t=5 被拦截，不应刷新记录
	clk.Advance(6 * time.Second)
	if !cd.Allow(r) {
		t.Error("被拦截的尝试不应刷新冷却起点（t=11 距 t=0 已超 10s）")
	}
}

func TestCooldownWaitGlobal(t *testing.T) {
	cd := NewCooldown(ratelimit.NewInterval(60*time.Millisecond), time.Now)
	ctx := context.Background()

	if err := cd.WaitGlobal(ctx); err != nil {
		t.Fatalf("首次 WaitGlobal 失败: %v", err)
	}
	start := time.Now()
	if err := cd.WaitGlobal(ctx); err != nil {
		t.Fatalf("第二次 WaitGlobal 失败: %v", err)
	}
	if d := time.Since(start); d < 40*time.Millisecond {
		t.Errorf("第二次应受全局限流约束，实际间隔 %v", d)
	}
}

func TestCooldownWaitGlobalRespectsContext(t *testing.T) {
	cd := NewCooldown(ratelimit.NewInterval(5*time.Second), time.Now)
	ctx := context.Background()
	if err := cd.WaitGlobal(ctx); err != nil {
		t.Fatalf("首次失败: %v", err)
	}

	ctx2, cancel := context.WithTimeout(ctx, 50*time.Millisecond)
	defer cancel()
	if err := cd.WaitGlobal(ctx2); err == nil {
		t.Error("ctx 超时后应返回错误")
	}
}
```

- [ ] **Step 2: 运行测试确认失败**

```bash
cd server && go test ./internal/rules/ -run TestCooldown -v
```

Expected: 编译失败，`undefined: NewCooldown`。

- [ ] **Step 3: 实现**

创建 `server/internal/rules/cooldown.go`：

```go
package rules

import (
	"context"
	"sync"
	"time"

	"github.com/MrZhongzq/MagicalDanmaku/server/internal/ratelimit"
)

// Cooldown 实现三层节流。
//
//	全局限流   防账号触发风控，复用 P0 的 ratelimit.Limiter
//	冷却组     多规则共享节流，如 greeting / thanks
//	规则冷却   单规则最小触发间隔
//
// 三层依次检查，任一不通过即跳过本次触发——不排队、不延迟发送。
// 弹幕时效性强，迟到几十秒的欢迎语没有意义，堆积的队列反而会在
// 冷却结束后集中喷发。
//
// 命名冷却组取代原项目的 msgCds[100] 编号通道：运营不必再记住
// 「3 号通道是关注答谢」，且在 P4 的 WebUI 中可直接下拉选择。
type Cooldown struct {
	global ratelimit.Limiter
	now    func() time.Time

	mu             sync.Mutex
	ruleLastFired  map[string]time.Time  // 规则名 → 上次触发时间
	groupLastFired map[string]time.Time  // 组名 → 上次触发时间
	groupInterval  map[string]time.Duration
}

// NewCooldown 创建节流器。now 可为 nil，此时使用 time.Now。
func NewCooldown(global ratelimit.Limiter, now func() time.Time) *Cooldown {
	if now == nil {
		now = time.Now
	}
	if global == nil {
		global = ratelimit.NewInterval(0)
	}
	return &Cooldown{
		global:         global,
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
// **不刷新任何时间戳**——被拦截的尝试不该延长冷却期。
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

// WaitGlobal 等待全局限流放行。
//
// 与 Allow 分开是因为全局限流需要真正阻塞等待，而规则级与组级
// 只是纯时间比较。动作真正发送前调用本方法。
func (c *Cooldown) WaitGlobal(ctx context.Context) error {
	return c.global.Wait(ctx)
}
```

- [ ] **Step 4: 运行测试确认通过**

```bash
cd server && go test ./internal/rules/ -v
```

Expected: 全部 PASS。

- [ ] **Step 5: 提交**

```bash
cd server && go vet ./... && gofmt -l .
git add server/internal/rules/
git commit -m "feat: 实现三层冷却节流"
```

---

### Task 7: 合并窗口与去重

**Files:**
- Create: `server/internal/rules/aggregate.go`
- Test: `server/internal/rules/aggregate_test.go`

**Interfaces:**
- Consumes: Task 1 的 `AggregateSpec`、`Trigger`；Task 2 的 `VarsFromEvent`、`MergeVars`、`LookupPath`
- Produces:
  - `rules.Aggregator` 结构，`rules.NewAggregator(spec AggregateSpec, out func(Trigger)) *Aggregator`
  - `(*Aggregator).Add(ev event.Event)` — 投入事件
  - `(*Aggregator).Flush()` — 立即结算当前窗口
  - `(*Aggregator).Close()` — 停止内部计时器并结算
  - `rules.PassthroughTrigger(ev event.Event) Trigger` — 未配置合并时的直通构造

**核心逻辑（顺序不可颠倒）：**

1. **按 UID 逐字段合并（总是执行）** — 解决 P0 联调发现的进场重复：
   `ENTRY_EFFECT` 只有 UID 无昵称，`INTERACT_WORD_V2` 信息完整，合并后完整。
2. **按 `By` 分组产出 Trigger** — `type` 全合一条、`user` 每人一条、
   `gift` 每种「用户+礼物」一条。

- [ ] **Step 1: 写失败测试**

创建 `server/internal/rules/aggregate_test.go`：

```go
package rules

import (
	"sync"
	"testing"
	"time"

	"github.com/MrZhongzq/MagicalDanmaku/server/internal/event"
)

// collector 收集 Aggregator 产出的 Trigger。
type collector struct {
	mu  sync.Mutex
	got []Trigger
}

func (c *collector) add(tr Trigger) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.got = append(c.got, tr)
}

func (c *collector) all() []Trigger {
	c.mu.Lock()
	defer c.mu.Unlock()
	out := make([]Trigger, len(c.got))
	copy(out, c.got)
	return out
}

func enterEvent(uid, name string, guard int) event.Event {
	return event.Event{
		Type: event.TypeUserEnter, RoomID: "1", Timestamp: time.Unix(1700000000, 0),
		Payload: event.UserEnter{User: event.User{UID: uid, Username: name, GuardLevel: guard}},
	}
}

func giftEvent(uid, name, giftName string, count, coin int64) event.Event {
	return event.Event{
		Type: event.TypeGift, RoomID: "1", Timestamp: time.Unix(1700000000, 0),
		Payload: event.Gift{
			User:     event.User{UID: uid, Username: name},
			GiftName: giftName, Count: count, TotalCoin: coin, CoinType: "gold",
		},
	}
}

func TestAggregateByTypeMergesAll(t *testing.T) {
	c := &collector{}
	// 窗口设很长，用 Flush 手动结算，避免测试依赖真实时间
	agg := NewAggregator(AggregateSpec{Window: time.Hour, By: AggregateByType}, c.add)
	defer agg.Close()

	agg.Add(enterEvent("1", "甲", 0))
	agg.Add(enterEvent("2", "乙", 3))
	agg.Add(enterEvent("3", "丙", 0))
	agg.Flush()

	got := c.all()
	if len(got) != 1 {
		t.Fatalf("按类型合并应产出 1 条 Trigger，实际 %d", len(got))
	}
	tr := got[0]
	if tr.Type != event.TypeUserEnter {
		t.Errorf("Type = %s", tr.Type)
	}
	if len(tr.Events) != 3 {
		t.Errorf("Events 数 = %d, 期望 3", len(tr.Events))
	}
	if cnt, _ := LookupPath(tr.Vars, "count"); cnt != 3 {
		t.Errorf("count = %v, 期望 3", cnt)
	}
	users, ok := tr.Vars["users"].([]string)
	if !ok {
		t.Fatalf("users 类型错误: %T", tr.Vars["users"])
	}
	if len(users) != 3 || users[0] != "甲" || users[2] != "丙" {
		t.Errorf("users = %v, 期望按首次出现顺序 [甲 乙 丙]", users)
	}
}

func TestAggregateMergesSameUIDFields(t *testing.T) {
	// 模拟 ENTRY_EFFECT（无昵称）+ INTERACT_WORD_V2（完整）
	c := &collector{}
	agg := NewAggregator(AggregateSpec{Window: time.Hour, By: AggregateByType}, c.add)
	defer agg.Close()

	agg.Add(enterEvent("1018633655", "", 3))          // ENTRY_EFFECT：有 UID 有舰长，无昵称
	agg.Add(enterEvent("1018633655", "洛洛的小小小", 0)) // INTERACT_WORD_V2：有昵称，无舰长
	agg.Flush()

	got := c.all()
	if len(got) != 1 {
		t.Fatalf("同一 UID 应合并为 1 条，实际 %d", len(got))
	}
	users := got[0].Vars["users"].([]string)
	if len(users) != 1 {
		t.Fatalf("同一用户只应出现一次，实际 %v", users)
	}
	if users[0] != "洛洛的小小小" {
		t.Errorf("users[0] = %q，应取非空昵称", users[0])
	}
	if cnt, _ := LookupPath(got[0].Vars, "count"); cnt != 1 {
		t.Errorf("count = %v，同一用户应计为 1", cnt)
	}
	// 舰长等级来自第一条，不该被第二条的 0 覆盖
	if gl, _ := LookupPath(got[0].Vars, "user.guardLevel"); gl != 3 {
		t.Errorf("user.guardLevel = %v，非空值不应被零值覆盖", gl)
	}
}

func TestAggregateByUserProducesOnePerUser(t *testing.T) {
	c := &collector{}
	agg := NewAggregator(AggregateSpec{Window: time.Hour, By: AggregateByUser}, c.add)
	defer agg.Close()

	agg.Add(enterEvent("1", "甲", 0))
	agg.Add(enterEvent("1", "甲", 0)) // 重复，应被去重
	agg.Add(enterEvent("2", "乙", 0))
	agg.Flush()

	got := c.all()
	if len(got) != 2 {
		t.Fatalf("两个用户应产出 2 条 Trigger，实际 %d", len(got))
	}
}

func TestAggregateByGiftAccumulatesCount(t *testing.T) {
	c := &collector{}
	agg := NewAggregator(AggregateSpec{Window: time.Hour, By: AggregateByGift}, c.add)
	defer agg.Close()

	agg.Add(giftEvent("9", "土豪", "小花花", 1, 100))
	agg.Add(giftEvent("9", "土豪", "小花花", 1, 100))
	agg.Add(giftEvent("9", "土豪", "小花花", 3, 300))
	agg.Add(giftEvent("9", "土豪", "辣条", 5, 50)) // 不同礼物，另算一组
	agg.Flush()

	got := c.all()
	if len(got) != 2 {
		t.Fatalf("两种礼物应产出 2 条 Trigger，实际 %d", len(got))
	}

	var flower Trigger
	for _, tr := range got {
		if n, _ := LookupPath(tr.Vars, "gift.name"); n == "小花花" {
			flower = tr
		}
	}
	if cnt, _ := LookupPath(flower.Vars, "gift.count"); cnt != int64(5) {
		t.Errorf("gift.count = %v (%T), 期望累加为 5", cnt, cnt)
	}
	if coin, _ := LookupPath(flower.Vars, "gift.totalCoin"); coin != int64(500) {
		t.Errorf("gift.totalCoin = %v, 期望累加为 500", coin)
	}
}

func TestAggregateFiresAfterWindow(t *testing.T) {
	c := &collector{}
	agg := NewAggregator(AggregateSpec{Window: 60 * time.Millisecond, By: AggregateByType}, c.add)
	defer agg.Close()

	agg.Add(enterEvent("1", "甲", 0))
	if len(c.all()) != 0 {
		t.Error("窗口未到期不应产出")
	}

	time.Sleep(150 * time.Millisecond)
	if got := c.all(); len(got) != 1 {
		t.Fatalf("窗口到期应自动产出，实际 %d 条", len(got))
	}
}

func TestAggregateEmptyFlushProducesNothing(t *testing.T) {
	c := &collector{}
	agg := NewAggregator(AggregateSpec{Window: time.Hour, By: AggregateByType}, c.add)
	defer agg.Close()

	agg.Flush()
	if got := c.all(); len(got) != 0 {
		t.Errorf("空窗口不应产出，实际 %d 条", len(got))
	}
}

func TestAggregateSeparatesEventTypes(t *testing.T) {
	c := &collector{}
	agg := NewAggregator(AggregateSpec{Window: time.Hour, By: AggregateByType}, c.add)
	defer agg.Close()

	agg.Add(enterEvent("1", "甲", 0))
	agg.Add(giftEvent("2", "乙", "小花花", 1, 100))
	agg.Flush()

	got := c.all()
	if len(got) != 2 {
		t.Fatalf("不同事件类型应分开产出，实际 %d 条", len(got))
	}
}

func TestPassthroughTrigger(t *testing.T) {
	ev := enterEvent("1", "甲", 3)
	tr := PassthroughTrigger(ev)

	if tr.Type != event.TypeUserEnter {
		t.Errorf("Type = %s", tr.Type)
	}
	if len(tr.Events) != 1 {
		t.Errorf("Events 数 = %d, 期望 1", len(tr.Events))
	}
	if u, _ := LookupPath(tr.Vars, "user.username"); u != "甲" {
		t.Errorf("user.username = %v", u)
	}
	// 直通事件也应带 count，让模板可以统一写法
	if cnt, _ := LookupPath(tr.Vars, "count"); cnt != 1 {
		t.Errorf("count = %v, 期望 1", cnt)
	}
	users, _ := tr.Vars["users"].([]string)
	if len(users) != 1 || users[0] != "甲" {
		t.Errorf("users = %v", users)
	}
}

func TestAggregateCloseFlushesPending(t *testing.T) {
	c := &collector{}
	agg := NewAggregator(AggregateSpec{Window: time.Hour, By: AggregateByType}, c.add)

	agg.Add(enterEvent("1", "甲", 0))
	agg.Close()

	if got := c.all(); len(got) != 1 {
		t.Errorf("Close 应结算未决窗口，实际 %d 条", len(got))
	}
}
```

- [ ] **Step 2: 运行测试确认失败**

```bash
cd server && go test ./internal/rules/ -run 'TestAggregate|TestPassthrough' -v
```

Expected: 编译失败，`undefined: NewAggregator`。

- [ ] **Step 3: 实现**

创建 `server/internal/rules/aggregate.go`：

```go
package rules

import (
	"sync"
	"time"

	"github.com/MrZhongzq/MagicalDanmaku/server/internal/event"
)

// Aggregator 在时间窗口内缓冲事件，到期后去重、合并并产出 Trigger。
//
// 处理分两步，顺序不可颠倒：
//
//	第一步  按 UID 逐字段合并（总是执行）
//	第二步  按 spec.By 分组产出 Trigger
//
// 第一步与分组方式无关，正是它解决了 P0 联调发现的进场重复问题：
// ENTRY_EFFECT 只有 UID 没有昵称，INTERACT_WORD_V2 信息完整，
// 两条合并后得到一条完整记录。
type Aggregator struct {
	spec AggregateSpec
	out  func(Trigger)

	mu      sync.Mutex
	buckets map[string]*bucket // 键：事件类型 + UID
	timer   *time.Timer
	closed  bool
}

// bucket 是同一 UID 同一类型的事件累积。
type bucket struct {
	typ    event.Type
	uid    string
	events []event.Event
	vars   map[string]any
	seq    int // 首次出现序号，用于保持 users 数组的顺序
}

// NewAggregator 创建合并器。out 在窗口到期时被调用，可能被调用多次
// （每个分组一次）。
func NewAggregator(spec AggregateSpec, out func(Trigger)) *Aggregator {
	return &Aggregator{
		spec:    spec,
		out:     out,
		buckets: make(map[string]*bucket),
	}
}

// Add 把事件投入当前窗口。首个事件会启动窗口计时。
func (a *Aggregator) Add(ev event.Event) {
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.closed {
		return
	}

	uid := uidOf(ev)
	key := string(ev.Type) + "\x00" + uid

	b, ok := a.buckets[key]
	if !ok {
		b = &bucket{typ: ev.Type, uid: uid, vars: VarsFromEvent(ev), seq: len(a.buckets)}
		b.events = append(b.events, ev)
		a.buckets[key] = b
	} else {
		b.events = append(b.events, ev)
		// 第一步：逐字段合并，空值不覆盖非空值
		MergeVars(b.vars, VarsFromEvent(ev))
		accumulateGift(b.vars, ev)
	}

	// 首个事件启动窗口
	if a.timer == nil {
		a.timer = time.AfterFunc(a.spec.Window, a.onTimeout)
	}
}

// onTimeout 是窗口到期回调。
func (a *Aggregator) onTimeout() {
	a.mu.Lock()
	triggers := a.drainLocked()
	a.mu.Unlock()

	for _, tr := range triggers {
		a.out(tr)
	}
}

// Flush 立即结算当前窗口。
func (a *Aggregator) Flush() {
	a.mu.Lock()
	if a.timer != nil {
		a.timer.Stop()
		a.timer = nil
	}
	triggers := a.drainLocked()
	a.mu.Unlock()

	for _, tr := range triggers {
		a.out(tr)
	}
}

// Close 停止计时器并结算未决窗口。
func (a *Aggregator) Close() {
	a.mu.Lock()
	if a.closed {
		a.mu.Unlock()
		return
	}
	a.closed = true
	if a.timer != nil {
		a.timer.Stop()
		a.timer = nil
	}
	triggers := a.drainLocked()
	a.mu.Unlock()

	for _, tr := range triggers {
		a.out(tr)
	}
}

// drainLocked 清空缓冲区并按 By 分组产出 Trigger。调用者需持有锁。
func (a *Aggregator) drainLocked() []Trigger {
	if len(a.buckets) == 0 {
		a.timer = nil
		return nil
	}

	buckets := make([]*bucket, 0, len(a.buckets))
	for _, b := range a.buckets {
		buckets = append(buckets, b)
	}
	a.buckets = make(map[string]*bucket)
	a.timer = nil

	// 按首次出现顺序排序，保证 users 数组顺序稳定可预测
	sortBucketsBySeq(buckets)

	// 第二步：按 By 分组
	groups := make(map[string][]*bucket)
	var order []string
	for _, b := range buckets {
		g := a.groupKey(b)
		if _, ok := groups[g]; !ok {
			order = append(order, g)
		}
		groups[g] = append(groups[g], b)
	}

	out := make([]Trigger, 0, len(order))
	for _, g := range order {
		out = append(out, mergeBuckets(groups[g]))
	}
	return out
}

// groupKey 按 spec.By 计算分组键。
func (a *Aggregator) groupKey(b *bucket) string {
	switch a.spec.By {
	case AggregateByUser:
		return string(b.typ) + "\x00" + b.uid
	case AggregateByGift:
		name, _ := LookupPath(b.vars, "gift.name")
		return string(b.typ) + "\x00" + b.uid + "\x00" + toString(name)
	default: // AggregateByType
		return string(b.typ)
	}
}

// mergeBuckets 把同组的多个 bucket 合成一个 Trigger。
func mergeBuckets(bs []*bucket) Trigger {
	first := bs[0]
	vars := make(map[string]any, len(first.vars)+2)
	MergeVars(vars, first.vars)

	events := make([]event.Event, 0, len(bs))
	users := make([]string, 0, len(bs))
	seenUser := make(map[string]bool, len(bs))

	for _, b := range bs {
		events = append(events, b.events...)
		if b != first {
			MergeVars(vars, b.vars)
		}
		if name, ok := LookupPath(b.vars, "user.username"); ok {
			if s := toString(name); s != "" && !seenUser[s] {
				seenUser[s] = true
				users = append(users, s)
			}
		}
	}

	vars["count"] = len(bs)
	vars["users"] = users

	return Trigger{Type: first.typ, Events: events, Vars: vars}
}

// accumulateGift 累加礼物数量与价值。
//
// MergeVars 的语义是「空值不覆盖非空值」，对计数字段不适用——
// 连击的两条礼物应当相加而非取其一。
func accumulateGift(dst map[string]any, ev event.Event) {
	g, ok := ev.Payload.(event.Gift)
	if !ok {
		return
	}
	gm, ok := dst["gift"].(map[string]any)
	if !ok {
		return
	}
	if cur, ok := gm["count"].(int64); ok {
		gm["count"] = cur + g.Count
	}
	if cur, ok := gm["totalCoin"].(int64); ok {
		gm["totalCoin"] = cur + g.TotalCoin
	}
}

// uidOf 提取事件的用户 UID，无用户的事件返回空串。
func uidOf(ev event.Event) string {
	v := VarsFromEvent(ev)
	uid, _ := LookupPath(v, "user.uid")
	return toString(uid)
}

// sortBucketsBySeq 按首次出现序号排序。
func sortBucketsBySeq(bs []*bucket) {
	for i := 1; i < len(bs); i++ {
		for j := i; j > 0 && bs[j].seq < bs[j-1].seq; j-- {
			bs[j], bs[j-1] = bs[j-1], bs[j]
		}
	}
}

// PassthroughTrigger 为未配置合并的规则构造直通 Trigger。
//
// 同样填上 count 与 users，让模板对合并与非合并事件可以统一写法。
func PassthroughTrigger(ev event.Event) Trigger {
	vars := VarsFromEvent(ev)
	vars["count"] = 1

	users := []string{}
	if name, ok := LookupPath(vars, "user.username"); ok {
		if s := toString(name); s != "" {
			users = append(users, s)
		}
	}
	vars["users"] = users

	return Trigger{Type: ev.Type, Events: []event.Event{ev}, Vars: vars}
}
```

- [ ] **Step 4: 运行测试确认通过**

```bash
cd server && go test ./internal/rules/ -v
```

Expected: 全部 PASS。

- [ ] **Step 5: 竞态检测**

Aggregator 有内部计时器与外部投递并发，必须跑竞态：

```bash
cd server && CGO_ENABLED=1 go test ./internal/rules/ -race -count=5
```

Expected: PASS，无 DATA RACE。

- [ ] **Step 6: 提交**

```bash
cd server && go vet ./... && gofmt -l .
git add server/internal/rules/
git commit -m "feat: 实现合并窗口与按 UID 去重"
```

---

### Task 8: 规则匹配

**Files:**
- Create: `server/internal/rules/matcher.go`
- Test: `server/internal/rules/matcher_test.go`

**Interfaces:**
- Consumes: Task 1 的 `Rule`、`Trigger`；Task 3 的 `Evaluator`
- Produces:
  - `rules.Matcher` 结构，`rules.NewMatcher(rs []Rule, ev Evaluator, log *slog.Logger) *Matcher`
  - `(*Matcher).Match(tr Trigger) []Rule` — 返回命中的规则，按配置顺序
  - `(*Matcher).RulesFor(t event.Type) []Rule` — 返回监听某事件类型的规则
  - `(*Matcher).ScheduledRules() []Rule` — 返回全部定时规则

**错误隔离：** 单条规则的条件求值出错**只跳过该规则并记日志**，不影响
其他规则——这是设计文档第 12 节的硬要求。

- [ ] **Step 1: 写失败测试**

创建 `server/internal/rules/matcher_test.go`：

```go
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

func ruleNames(rs []Rule) []string {
	out := make([]string, 0, len(rs))
	for _, r := range rs {
		out = append(out, r.Name)
	}
	return out
}
```

- [ ] **Step 2: 运行测试确认失败**

```bash
cd server && go test ./internal/rules/ -run TestMatcher -v
```

Expected: 编译失败，`undefined: NewMatcher`。

- [ ] **Step 3: 实现**

创建 `server/internal/rules/matcher.go`：

```go
package rules

import (
	"log/slog"

	"github.com/MrZhongzq/MagicalDanmaku/server/internal/event"
)

// Matcher 按事件类型与条件筛选命中的规则。
type Matcher struct {
	rules     []Rule
	eval      Evaluator
	log       *slog.Logger
	byType    map[event.Type][]Rule
	scheduled []Rule
}

// NewMatcher 创建匹配器。规则在此按事件类型预先分组，避免每次事件
// 都遍历全部规则——高频弹幕下这个开销不可忽略。
func NewMatcher(rs []Rule, ev Evaluator, log *slog.Logger) *Matcher {
	if log == nil {
		log = slog.Default()
	}
	m := &Matcher{
		rules:  rs,
		eval:   ev,
		log:    log,
		byType: make(map[event.Type][]Rule),
	}
	for _, r := range rs {
		if !r.Enabled {
			continue
		}
		if r.Schedule != "" {
			m.scheduled = append(m.scheduled, r)
			continue
		}
		for _, t := range r.On {
			m.byType[t] = append(m.byType[t], r)
		}
	}
	return m
}

// Match 返回命中该 Trigger 的全部规则，保持配置顺序。
//
// 单条规则的条件求值出错只跳过该规则并记日志，不影响其他规则。
func (m *Matcher) Match(tr Trigger) []Rule {
	candidates := m.byType[tr.Type]
	if len(candidates) == 0 {
		return nil
	}

	out := make([]Rule, 0, len(candidates))
	for _, r := range candidates {
		if r.When == nil {
			out = append(out, r)
			continue
		}
		ok, err := m.eval.Eval(*r.When, tr.Vars)
		if err != nil {
			m.log.Warn("规则条件求值失败，已跳过该规则",
				"rule", r.Name, "err", err)
			continue
		}
		if ok {
			out = append(out, r)
		}
	}
	return out
}

// RulesFor 返回监听指定事件类型的启用规则。
func (m *Matcher) RulesFor(t event.Type) []Rule {
	return m.byType[t]
}

// ScheduledRules 返回全部启用的定时规则。
func (m *Matcher) ScheduledRules() []Rule {
	return m.scheduled
}
```

- [ ] **Step 4: 运行测试确认通过**

```bash
cd server && go test ./internal/rules/ -v
```

Expected: 全部 PASS。

- [ ] **Step 5: 提交**

```bash
cd server && go vet ./... && gofmt -l .
git add server/internal/rules/
git commit -m "feat: 实现规则匹配与错误隔离"
```

---

### Task 9: 动作执行

**Files:**
- Create: `server/internal/rules/executor.go`
- Test: `server/internal/rules/executor_test.go`

**Interfaces:**
- Consumes: Task 1 的 `Rule`、`Action`、`Trigger`；Task 4 的 `Renderer`；
  Task 5 的 `Sandbox`、`BotAPI`；Task 6 的 `Cooldown`
- Produces:
  - `rules.Executor` 结构
  - `rules.NewExecutor(opts ExecutorOptions) *Executor`
  - `rules.ExecutorOptions{Bot BotAPI, Renderer *Renderer, Script *Sandbox, Cooldown *Cooldown, DefaultBlockHours int, Logger *slog.Logger}`
  - `(*Executor).Execute(ctx context.Context, r Rule, tr Trigger) error`

**错误隔离：** 单个动作失败**记录日志并继续执行后续动作**，不中断整条
规则——设计文档第 12 节的硬要求。

- [ ] **Step 1: 写失败测试**

创建 `server/internal/rules/executor_test.go`：

```go
package rules

import (
	"context"
	"errors"
	"math/rand"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/MrZhongzq/MagicalDanmaku/server/internal/event"
	"github.com/MrZhongzq/MagicalDanmaku/server/internal/ratelimit"
)

// failingBot 让指定次数的发送失败，用于测试错误隔离。
type failingBot struct {
	mu       sync.Mutex
	danmakus []string
	blocks   []blockCall
	failNext bool
}

type blockCall struct {
	uid   string
	hours int
}

func (f *failingBot) SendDanmaku(text string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.failNext {
		f.failNext = false
		return errors.New("模拟发送失败")
	}
	f.danmakus = append(f.danmakus, text)
	return nil
}

func (f *failingBot) Block(uid string, hours int) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.blocks = append(f.blocks, blockCall{uid, hours})
	return nil
}

func (f *failingBot) sent() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := make([]string, len(f.danmakus))
	copy(out, f.danmakus)
	return out
}

func newTestExecutor(bot BotAPI) *Executor {
	return NewExecutor(ExecutorOptions{
		Bot:               bot,
		Renderer:          NewRenderer(rand.New(rand.NewSource(1))),
		Script:            NewSandbox(SandboxOptions{Timeout: 200 * time.Millisecond, Bot: bot}),
		Cooldown:          NewCooldown(ratelimit.NewInterval(0), time.Now),
		DefaultBlockHours: 1,
	})
}

func enterTrigger(uid, name string) Trigger {
	return PassthroughTrigger(event.Event{
		Type: event.TypeUserEnter, RoomID: "1", Timestamp: time.Unix(1700000000, 0),
		Payload: event.UserEnter{User: event.User{UID: uid, Username: name}},
	})
}

func TestExecuteDanmakuAction(t *testing.T) {
	bot := &failingBot{}
	ex := newTestExecutor(bot)

	r := Rule{Name: "欢迎", Do: []Action{
		{Type: ActionDanmaku, Template: []string{"欢迎 {{.user.username}}"}},
	}}
	if err := ex.Execute(context.Background(), r, enterTrigger("1", "甲")); err != nil {
		t.Fatalf("Execute 失败: %v", err)
	}

	sent := bot.sent()
	if len(sent) != 1 || sent[0] != "欢迎 甲" {
		t.Errorf("发送内容 = %v", sent)
	}
}

func TestExecuteDanmakuUsesMergedUsers(t *testing.T) {
	bot := &failingBot{}
	ex := newTestExecutor(bot)

	tr := enterTrigger("1", "甲")
	tr.Vars["users"] = []string{"甲", "乙", "丙"}
	tr.Vars["count"] = 3

	r := Rule{Name: "欢迎", Do: []Action{
		{Type: ActionDanmaku, Template: []string{`欢迎 {{join .users "、"}} 回家`}},
	}}
	if err := ex.Execute(context.Background(), r, tr); err != nil {
		t.Fatalf("Execute 失败: %v", err)
	}
	if got := bot.sent(); len(got) != 1 || got[0] != "欢迎 甲、乙、丙 回家" {
		t.Errorf("= %v", got)
	}
}

func TestExecuteBlockAction(t *testing.T) {
	bot := &failingBot{}
	ex := newTestExecutor(bot)

	r := Rule{Name: "禁言", Do: []Action{{Type: ActionBlock, Hours: 12}}}
	if err := ex.Execute(context.Background(), r, enterTrigger("999", "坏人")); err != nil {
		t.Fatalf("Execute 失败: %v", err)
	}

	if len(bot.blocks) != 1 {
		t.Fatalf("禁言次数 = %d", len(bot.blocks))
	}
	if bot.blocks[0].uid != "999" || bot.blocks[0].hours != 12 {
		t.Errorf("blocks[0] = %+v", bot.blocks[0])
	}
}

func TestExecuteBlockUsesDefaultHours(t *testing.T) {
	bot := &failingBot{}
	ex := newTestExecutor(bot)

	r := Rule{Name: "禁言", Do: []Action{{Type: ActionBlock}}} // Hours 未指定
	if err := ex.Execute(context.Background(), r, enterTrigger("999", "坏人")); err != nil {
		t.Fatalf("Execute 失败: %v", err)
	}
	if bot.blocks[0].hours != 1 {
		t.Errorf("hours = %d, 期望使用默认值 1", bot.blocks[0].hours)
	}
}

func TestExecuteBlockAppliesToAllMergedUsers(t *testing.T) {
	bot := &failingBot{}
	ex := newTestExecutor(bot)

	// 合并后的 Trigger 含多个事件，禁言应作用于全部参与者
	tr := Trigger{
		Type: event.TypeDanmaku,
		Events: []event.Event{
			{Type: event.TypeDanmaku, Payload: event.Danmaku{User: event.User{UID: "1"}}},
			{Type: event.TypeDanmaku, Payload: event.Danmaku{User: event.User{UID: "2"}}},
		},
		Vars: map[string]any{"user": map[string]any{"uid": "1"}},
	}
	r := Rule{Name: "禁言", Do: []Action{{Type: ActionBlock, Hours: 1}}}
	if err := ex.Execute(context.Background(), r, tr); err != nil {
		t.Fatalf("Execute 失败: %v", err)
	}
	if len(bot.blocks) != 2 {
		t.Errorf("应禁言 2 个用户，实际 %d", len(bot.blocks))
	}
}

func TestExecuteScriptAction(t *testing.T) {
	bot := &failingBot{}
	ex := newTestExecutor(bot)

	r := Rule{Name: "脚本", Do: []Action{
		{Type: ActionScript, Script: `bot.sendDanmaku("来自脚本: " + event.user.username)`},
	}}
	if err := ex.Execute(context.Background(), r, enterTrigger("1", "甲")); err != nil {
		t.Fatalf("Execute 失败: %v", err)
	}
	if got := bot.sent(); len(got) != 1 || got[0] != "来自脚本: 甲" {
		t.Errorf("= %v", got)
	}
}

func TestExecuteLogAction(t *testing.T) {
	bot := &failingBot{}
	ex := newTestExecutor(bot)

	r := Rule{Name: "只记日志", Do: []Action{{Type: ActionLog}}}
	if err := ex.Execute(context.Background(), r, enterTrigger("1", "甲")); err != nil {
		t.Fatalf("Execute 失败: %v", err)
	}
	if len(bot.sent()) != 0 {
		t.Error("log 动作不应发送弹幕")
	}
}

func TestExecuteRunsAllActionsInOrder(t *testing.T) {
	bot := &failingBot{}
	ex := newTestExecutor(bot)

	r := Rule{Name: "多动作", Do: []Action{
		{Type: ActionDanmaku, Template: []string{"第一条"}},
		{Type: ActionDanmaku, Template: []string{"第二条"}},
	}}
	if err := ex.Execute(context.Background(), r, enterTrigger("1", "甲")); err != nil {
		t.Fatalf("Execute 失败: %v", err)
	}
	got := bot.sent()
	if len(got) != 2 || got[0] != "第一条" || got[1] != "第二条" {
		t.Errorf("= %v", got)
	}
}

func TestExecuteContinuesAfterActionFailure(t *testing.T) {
	bot := &failingBot{failNext: true}
	ex := newTestExecutor(bot)

	r := Rule{Name: "多动作", Do: []Action{
		{Type: ActionDanmaku, Template: []string{"会失败的"}},
		{Type: ActionDanmaku, Template: []string{"应当仍被执行"}},
	}}
	err := ex.Execute(context.Background(), r, enterTrigger("1", "甲"))
	if err == nil {
		t.Error("有动作失败时应返回错误")
	}

	got := bot.sent()
	if len(got) != 1 || got[0] != "应当仍被执行" {
		t.Errorf("单个动作失败不应中断后续动作，实际 %v", got)
	}
}

func TestExecuteBadTemplateDoesNotCrash(t *testing.T) {
	bot := &failingBot{}
	ex := newTestExecutor(bot)

	r := Rule{Name: "坏模板", Do: []Action{
		{Type: ActionDanmaku, Template: []string{"{{.未闭合"}},
		{Type: ActionDanmaku, Template: []string{"后续动作"}},
	}}
	err := ex.Execute(context.Background(), r, enterTrigger("1", "甲"))
	if err == nil {
		t.Error("模板错误应被上报")
	}
	if got := bot.sent(); len(got) != 1 || got[0] != "后续动作" {
		t.Errorf("模板错误不应中断后续动作，实际 %v", got)
	}
}

func TestExecuteEmptyRenderIsSkipped(t *testing.T) {
	bot := &failingBot{}
	ex := newTestExecutor(bot)

	// 模板渲染结果为空时不该发空弹幕
	r := Rule{Name: "空模板", Do: []Action{
		{Type: ActionDanmaku, Template: []string{"{{.不存在的字段}}"}},
	}}
	if err := ex.Execute(context.Background(), r, enterTrigger("1", "甲")); err != nil {
		t.Fatalf("Execute 失败: %v", err)
	}
	if len(bot.sent()) != 0 {
		t.Errorf("渲染为空时不应发送，实际 %v", bot.sent())
	}
}

func TestExecuteScriptTimeoutIsReported(t *testing.T) {
	bot := &failingBot{}
	ex := NewExecutor(ExecutorOptions{
		Bot:      bot,
		Renderer: NewRenderer(rand.New(rand.NewSource(1))),
		Script:   NewSandbox(SandboxOptions{Timeout: 50 * time.Millisecond, Bot: bot}),
		Cooldown: NewCooldown(ratelimit.NewInterval(0), time.Now),
	})

	r := Rule{Name: "死循环", Do: []Action{{Type: ActionScript, Script: `while(true){}`}}}
	err := ex.Execute(context.Background(), r, enterTrigger("1", "甲"))
	if err == nil {
		t.Fatal("脚本超时应被上报")
	}
	if !strings.Contains(err.Error(), "超时") {
		t.Errorf("错误信息应提及超时，实际 %v", err)
	}
}
```

- [ ] **Step 2: 运行测试确认失败**

```bash
cd server && go test ./internal/rules/ -run TestExecute -v
```

Expected: 编译失败，`undefined: NewExecutor`。

- [ ] **Step 3: 实现**

创建 `server/internal/rules/executor.go`：

```go
package rules

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
)

// defaultBlockHours 是未指定时长时的默认禁言小时数。
const defaultBlockHours = 1

// ExecutorOptions 配置动作执行器。
type ExecutorOptions struct {
	Bot               BotAPI
	Renderer          *Renderer
	Script            *Sandbox
	Cooldown          *Cooldown
	DefaultBlockHours int
	Logger            *slog.Logger
}

// Executor 执行规则的动作列表。
type Executor struct {
	bot        BotAPI
	renderer   *Renderer
	script     *Sandbox
	cooldown   *Cooldown
	blockHours int
	log        *slog.Logger
}

// NewExecutor 创建动作执行器。
func NewExecutor(opts ExecutorOptions) *Executor {
	if opts.DefaultBlockHours <= 0 {
		opts.DefaultBlockHours = defaultBlockHours
	}
	if opts.Logger == nil {
		opts.Logger = slog.Default()
	}
	return &Executor{
		bot:        opts.Bot,
		renderer:   opts.Renderer,
		script:     opts.Script,
		cooldown:   opts.Cooldown,
		blockHours: opts.DefaultBlockHours,
		log:        opts.Logger,
	}
}

// Execute 按序执行规则的全部动作。
//
// 单个动作失败只记录日志并继续执行后续动作，最后返回聚合错误——
// 一条规则里前面的动作失败，不该让后面的动作也做不成。
func (e *Executor) Execute(ctx context.Context, r Rule, tr Trigger) error {
	var errs []error

	for i, a := range r.Do {
		if err := e.runAction(ctx, a, tr); err != nil {
			e.log.Warn("动作执行失败",
				"rule", r.Name, "action", i+1, "type", a.Type, "err", err)
			errs = append(errs, fmt.Errorf("第 %d 个动作(%s): %w", i+1, a.Type, err))
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("规则 %q 有 %d 个动作失败: %w", r.Name, len(errs), errors.Join(errs...))
	}
	return nil
}

// runAction 执行单个动作。
func (e *Executor) runAction(ctx context.Context, a Action, tr Trigger) error {
	switch a.Type {
	case ActionDanmaku:
		return e.sendDanmaku(ctx, a, tr)
	case ActionBlock:
		return e.blockUsers(ctx, a, tr)
	case ActionScript:
		if e.script == nil {
			return errors.New("rules: 未配置脚本沙箱")
		}
		return e.script.RunAction(a.Script, tr.Vars)
	case ActionLog:
		e.log.Info("规则触发", "type", tr.Type, "count", tr.Vars["count"], "vars", tr.Vars)
		return nil
	default:
		return fmt.Errorf("rules: 未知的动作类型 %q", a.Type)
	}
}

// sendDanmaku 渲染模板并发送弹幕。
func (e *Executor) sendDanmaku(ctx context.Context, a Action, tr Trigger) error {
	if e.bot == nil {
		return errors.New("rules: 未配置机器人接口")
	}

	text, err := e.renderer.Render(a.Template, tr.Vars)
	if err != nil {
		return err
	}
	// 渲染结果为空时静默跳过：空弹幕发不出去，报错反而制造噪声。
	if strings.TrimSpace(text) == "" {
		return nil
	}

	// 全局限流在真正发送前等待
	if e.cooldown != nil {
		if err := e.cooldown.WaitGlobal(ctx); err != nil {
			return err
		}
	}
	return e.bot.SendDanmaku(text)
}

// blockUsers 禁言 Trigger 涉及的全部用户。
//
// 合并后的 Trigger 可能包含多个用户，逐个禁言。
func (e *Executor) blockUsers(ctx context.Context, a Action, tr Trigger) error {
	if e.bot == nil {
		return errors.New("rules: 未配置机器人接口")
	}

	hours := a.Hours
	if hours <= 0 {
		hours = e.blockHours
	}

	uids := uidsOf(tr)
	if len(uids) == 0 {
		return errors.New("rules: 事件中没有可禁言的用户")
	}

	var errs []error
	for _, uid := range uids {
		if e.cooldown != nil {
			if err := e.cooldown.WaitGlobal(ctx); err != nil {
				return err
			}
		}
		if err := e.bot.Block(uid, hours); err != nil {
			errs = append(errs, fmt.Errorf("禁言 %s 失败: %w", uid, err))
		}
	}
	return errors.Join(errs...)
}

// uidsOf 提取 Trigger 涉及的全部用户 UID，去重且保持顺序。
func uidsOf(tr Trigger) []string {
	seen := make(map[string]bool, len(tr.Events))
	out := make([]string, 0, len(tr.Events))

	for _, ev := range tr.Events {
		if uid := uidOf(ev); uid != "" && !seen[uid] {
			seen[uid] = true
			out = append(out, uid)
		}
	}
	// 事件列表为空时回退到 Vars（定时任务等场景）
	if len(out) == 0 {
		if uid, ok := LookupPath(tr.Vars, "user.uid"); ok {
			if s := toString(uid); s != "" {
				out = append(out, s)
			}
		}
	}
	return out
}
```

> 注意 import 块中**不要**引入 `event` 包：`uidsOf` 通过类型推导使用
> `tr.Events`，签名中并未直接命名该包，引入会导致 `imported and not used`。

- [ ] **Step 4: 运行测试确认通过**

```bash
cd server && go test ./internal/rules/ -v
```

Expected: 全部 PASS。

> 若 `TestExecuteBlockAppliesToAllMergedUsers` 失败，检查 `uidsOf` 是否
> 正确遍历了 `tr.Events` 而非只看 `tr.Vars`——合并事件的多个用户信息
> 只存在于 Events 中。

- [ ] **Step 5: 提交**

```bash
cd server && go vet ./... && gofmt -l .
git add server/internal/rules/
git commit -m "feat: 实现动作执行与错误隔离"
```

---

### Task 10: 多账号轮换

**Files:**
- Create: `server/internal/account/pool.go`
- Test: `server/internal/account/pool_test.go`

**Interfaces:**
- Consumes: P0 的 `connector.Actions`、`connector.SendDanmakuRequest`、
  `connector.BlockRequest`、`bilibili.IsFatal`
- Produces:
  - `account.Account{Name string, Actions connector.Actions}`
  - `account.Pool` 结构，`account.New(accounts []Account, log *slog.Logger) *Pool`
  - `(*Pool).SendDanmaku(ctx context.Context, roomID, text string) error`
  - `(*Pool).Block(ctx context.Context, roomID, uid string, hours int) error`
  - `(*Pool).Healthy() int` — 返回当前可用账号数
  - `account.ErrNoHealthyAccount`

**设计要点：** 账号被 P0 的 `IsFatal` 判定为致命错误（`-101` 未登录、
`-111` csrf 失效、`1003` 已被禁言）后移出轮换并记日志。全部账号失效时
返回错误而**不静默丢弃**。

- [ ] **Step 1: 写失败测试**

创建 `server/internal/account/pool_test.go`：

```go
package account

import (
	"context"
	"errors"
	"sync"
	"testing"

	"github.com/MrZhongzq/MagicalDanmaku/server/internal/connector"
	"github.com/MrZhongzq/MagicalDanmaku/server/internal/connector/bilibili/api"
)

// stubActions 是 connector.Actions 的测试替身。
type stubActions struct {
	mu    sync.Mutex
	name  string
	sent  []string
	err   error // 每次调用都返回该错误
	calls int
}

func (s *stubActions) SendDanmaku(ctx context.Context, req connector.SendDanmakuRequest) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.calls++
	if s.err != nil {
		return s.err
	}
	s.sent = append(s.sent, req.Text)
	return nil
}

func (s *stubActions) BlockUser(ctx context.Context, req connector.BlockRequest) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.calls++
	return s.err
}

func (s *stubActions) UnblockUser(ctx context.Context, roomID, uid string) error {
	return s.err
}

func (s *stubActions) count() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.calls
}

func TestPoolRoundRobin(t *testing.T) {
	a := &stubActions{name: "A"}
	b := &stubActions{name: "B"}
	p := New([]Account{{Name: "A", Actions: a}, {Name: "B", Actions: b}}, nil)

	ctx := context.Background()
	for i := 0; i < 4; i++ {
		if err := p.SendDanmaku(ctx, "1", "消息"); err != nil {
			t.Fatalf("第 %d 次发送失败: %v", i+1, err)
		}
	}

	if a.count() != 2 || b.count() != 2 {
		t.Errorf("应均匀轮询，A=%d B=%d", a.count(), b.count())
	}
}

func TestPoolSingleAccount(t *testing.T) {
	a := &stubActions{}
	p := New([]Account{{Name: "A", Actions: a}}, nil)

	for i := 0; i < 3; i++ {
		if err := p.SendDanmaku(context.Background(), "1", "消息"); err != nil {
			t.Fatalf("发送失败: %v", err)
		}
	}
	if a.count() != 3 {
		t.Errorf("单账号应全部承担，实际 %d", a.count())
	}
}

func TestPoolRemovesFatallyFailedAccount(t *testing.T) {
	bad := &stubActions{err: &api.APIError{Code: -101, Message: "账号未登录"}}
	good := &stubActions{}
	p := New([]Account{{Name: "坏账号", Actions: bad}, {Name: "好账号", Actions: good}}, nil)

	ctx := context.Background()
	// 第一次会打到坏账号，失败后应自动切到好账号并完成发送
	if err := p.SendDanmaku(ctx, "1", "消息"); err != nil {
		t.Fatalf("应自动切换到可用账号，实际失败: %v", err)
	}
	if len(good.sent) != 1 {
		t.Errorf("好账号应完成发送，实际 %v", good.sent)
	}
	if p.Healthy() != 1 {
		t.Errorf("坏账号应被移出轮换，剩余健康账号 = %d", p.Healthy())
	}

	// 后续发送不应再尝试坏账号
	before := bad.count()
	for i := 0; i < 3; i++ {
		p.SendDanmaku(ctx, "1", "消息")
	}
	if bad.count() != before {
		t.Errorf("已失效账号不应再被调用，调用数从 %d 变为 %d", before, bad.count())
	}
}

func TestPoolKeepsAccountOnRetryableError(t *testing.T) {
	// 10030 发送过快是可重试错误，不该移出轮换
	a := &stubActions{err: &api.APIError{Code: 10030, Message: "发送过快"}}
	p := New([]Account{{Name: "A", Actions: a}}, nil)

	if err := p.SendDanmaku(context.Background(), "1", "消息"); err == nil {
		t.Error("发送应当失败")
	}
	if p.Healthy() != 1 {
		t.Errorf("可重试错误不应移出账号，健康账号 = %d", p.Healthy())
	}
}

func TestPoolAllAccountsFailed(t *testing.T) {
	a := &stubActions{err: &api.APIError{Code: -101}}
	b := &stubActions{err: &api.APIError{Code: -111}}
	p := New([]Account{{Name: "A", Actions: a}, {Name: "B", Actions: b}}, nil)

	err := p.SendDanmaku(context.Background(), "1", "消息")
	if err == nil {
		t.Fatal("全部账号失效时应返回错误，不得静默丢弃")
	}
	if p.Healthy() != 0 {
		t.Errorf("健康账号数 = %d, 期望 0", p.Healthy())
	}

	// 再次发送应立刻返回 ErrNoHealthyAccount
	err = p.SendDanmaku(context.Background(), "1", "消息")
	if !errors.Is(err, ErrNoHealthyAccount) {
		t.Errorf("err = %v, 期望 ErrNoHealthyAccount", err)
	}
}

func TestPoolEmptyIsError(t *testing.T) {
	p := New(nil, nil)
	if err := p.SendDanmaku(context.Background(), "1", "消息"); !errors.Is(err, ErrNoHealthyAccount) {
		t.Errorf("空账号池应返回 ErrNoHealthyAccount，实际 %v", err)
	}
}

func TestPoolBlock(t *testing.T) {
	a := &stubActions{}
	p := New([]Account{{Name: "A", Actions: a}}, nil)

	if err := p.Block(context.Background(), "1", "999", 12); err != nil {
		t.Fatalf("Block 失败: %v", err)
	}
	if a.count() != 1 {
		t.Errorf("调用数 = %d", a.count())
	}
}

func TestPoolConcurrentSend(t *testing.T) {
	a := &stubActions{}
	b := &stubActions{}
	p := New([]Account{{Name: "A", Actions: a}, {Name: "B", Actions: b}}, nil)

	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			p.SendDanmaku(context.Background(), "1", "并发消息")
		}()
	}
	wg.Wait()

	if total := a.count() + b.count(); total != 50 {
		t.Errorf("总调用数 = %d, 期望 50", total)
	}
}
```

- [ ] **Step 2: 运行测试确认失败**

```bash
cd server && go test ./internal/account/ -v
```

Expected: 编译失败，`undefined: New`。

- [ ] **Step 3: 实现**

创建 `server/internal/account/pool.go`：

```go
// Package account 管理多个 B 站账号的轮换发言。
package account

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync"

	"github.com/MrZhongzq/MagicalDanmaku/server/internal/connector"
	"github.com/MrZhongzq/MagicalDanmaku/server/internal/connector/bilibili"
)

// ErrNoHealthyAccount 表示没有可用账号。
var ErrNoHealthyAccount = errors.New("account: 没有可用的账号")

// Account 是一个可发言的账号。
type Account struct {
	Name    string
	Actions connector.Actions
}

// Pool 轮询多个账号发送动作，绕开单账号的频率限制。
//
// 账号被 P0 的 IsFatal 判定为致命错误（-101 未登录、-111 csrf 失效、
// 1003 已被禁言）后移出轮换。全部账号失效时返回错误而非静默丢弃——
// 静默失败会让使用者以为机器人在正常工作。
type Pool struct {
	log *slog.Logger

	mu      sync.Mutex
	entries []*entry
	next    int
}

// entry 是池中的一个账号及其健康状态。
type entry struct {
	acc     Account
	healthy bool
}

// New 创建账号池。
func New(accounts []Account, log *slog.Logger) *Pool {
	if log == nil {
		log = slog.Default()
	}
	p := &Pool{log: log}
	for _, a := range accounts {
		p.entries = append(p.entries, &entry{acc: a, healthy: true})
	}
	return p
}

// Healthy 返回当前可用账号数。
func (p *Pool) Healthy() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	n := 0
	for _, e := range p.entries {
		if e.healthy {
			n++
		}
	}
	return n
}

// SendDanmaku 轮询选账号发送弹幕，遇致命错误自动切换下一个。
func (p *Pool) SendDanmaku(ctx context.Context, roomID, text string) error {
	return p.do(ctx, func(a connector.Actions) error {
		return a.SendDanmaku(ctx, connector.SendDanmakuRequest{RoomID: roomID, Text: text})
	})
}

// Block 轮询选账号执行禁言。
func (p *Pool) Block(ctx context.Context, roomID, uid string, hours int) error {
	return p.do(ctx, func(a connector.Actions) error {
		return a.BlockUser(ctx, connector.BlockRequest{RoomID: roomID, UID: uid, Hours: hours})
	})
}

// do 在健康账号上执行操作，遇致命错误剔除该账号并重试下一个。
//
// 最多尝试与账号数相同的次数，避免全部失效时无限循环。
func (p *Pool) do(ctx context.Context, fn func(connector.Actions) error) error {
	total := len(p.entries)
	if total == 0 {
		return ErrNoHealthyAccount
	}

	var lastErr error
	for i := 0; i < total; i++ {
		e := p.pick()
		if e == nil {
			if lastErr != nil {
				return fmt.Errorf("account: 全部账号均已失效: %w", lastErr)
			}
			return ErrNoHealthyAccount
		}

		err := fn(e.acc.Actions)
		if err == nil {
			return nil
		}
		lastErr = err

		if bilibili.IsFatal(err) {
			p.markUnhealthy(e)
			p.log.Error("账号已失效，移出轮换",
				"account", e.acc.Name, "err", err, "healthy", p.Healthy())
			continue // 换下一个账号重试
		}
		// 可重试错误（如 10030 发送过快）：保留账号，直接上报
		return err
	}

	if lastErr != nil {
		return fmt.Errorf("account: 全部账号均已失效: %w", lastErr)
	}
	return ErrNoHealthyAccount
}

// pick 轮询取下一个健康账号，全部失效时返回 nil。
func (p *Pool) pick() *entry {
	p.mu.Lock()
	defer p.mu.Unlock()

	n := len(p.entries)
	for i := 0; i < n; i++ {
		e := p.entries[p.next%n]
		p.next = (p.next + 1) % n
		if e.healthy {
			return e
		}
	}
	return nil
}

// markUnhealthy 把账号标记为失效。
func (p *Pool) markUnhealthy(e *entry) {
	p.mu.Lock()
	defer p.mu.Unlock()
	e.healthy = false
}
```

- [ ] **Step 4: 运行测试确认通过**

```bash
cd server && go test ./internal/account/ -v
```

Expected: 全部 PASS。

- [ ] **Step 5: 竞态检测**

```bash
cd server && CGO_ENABLED=1 go test ./internal/account/ -race -count=5
```

Expected: PASS，无 DATA RACE。

- [ ] **Step 6: 提交**

```bash
cd server && go vet ./... && gofmt -l .
git add server/internal/account/
git commit -m "feat: 实现多账号轮换与失效剔除"
```

---

**下一步：** 继续阅读 `2026-07-31-p2-rule-engine-part3.md`，实现定时任务、
YAML 配置、Pipeline 组装与 `magicd run` 子命令。
