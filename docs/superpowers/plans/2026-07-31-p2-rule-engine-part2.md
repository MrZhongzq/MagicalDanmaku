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
### Task 10: 账号与绑定模型

> **本任务已于 2026-07-31 重写。** 初版实现为「轮询账号池 + 失效自动
> 切换」，与实际需求相反，已由提交 `e5c81bf0` 撤销。账号不是可互换的
> 资源，而是各有职责的参与者，详见设计文档第 9 节。

**Files:**
- Create: `server/internal/account/account.go`
- Test: `server/internal/account/account_test.go`

**Interfaces:**
- Consumes: P0 的 `connector.Actions`、`auth.Session`、`ratelimit.Limiter`
- Produces:
  - `account.Account{Name string, Session *auth.Session, Limiter ratelimit.Limiter}`
  - `account.New(name string, sess *auth.Session, interval time.Duration) *Account`
  - `account.Binding{Account *Account, RoomID string, Actions connector.Actions}`
  - `(*Binding).Label() string` — 形如 `主播号@1706666491`，用于日志
  - `(*Binding).SendDanmaku(ctx context.Context, text string) error`
  - `(*Binding).Block(ctx context.Context, uid string, hours int) error`
  - `account.ErrNoAccount`

**设计要点：**

1. **运行单元是 Binding**，即「账号-直播间」组合。同一直播间被两个账号
   连接时是两个独立 Binding，各跑各的规则。
2. **不做账号轮换与 fallback。** 指定账号失效就返回错误并记日志——主播号
   被封不该让小号顶替房管操作，小号根本没有房管权限。
3. **限流按账号共享。** B 站风控按账号计算，所以 `Limiter` 挂在 `Account`
   上而非 `Binding` 上：账号 A 同时连甲乙两个房间时共用节奏，账号 B 完全
   不受影响。

- [ ] **Step 1: 写失败测试**

创建 `server/internal/account/account_test.go`：

```go
package account

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/MrZhongzq/MagicalDanmaku/server/internal/connector"
	"github.com/MrZhongzq/MagicalDanmaku/server/internal/connector/bilibili/api"
	"github.com/MrZhongzq/MagicalDanmaku/server/internal/connector/bilibili/auth"
)

// stubActions 是 connector.Actions 的测试替身。
type stubActions struct {
	mu     sync.Mutex
	sent   []string
	blocks []string
	rooms  []string
	err    error
}

func (s *stubActions) SendDanmaku(ctx context.Context, req connector.SendDanmakuRequest) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.err != nil {
		return s.err
	}
	s.sent = append(s.sent, req.Text)
	s.rooms = append(s.rooms, req.RoomID)
	return nil
}

func (s *stubActions) BlockUser(ctx context.Context, req connector.BlockRequest) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.err != nil {
		return s.err
	}
	s.blocks = append(s.blocks, req.UID)
	s.rooms = append(s.rooms, req.RoomID)
	return nil
}

func (s *stubActions) UnblockUser(ctx context.Context, roomID, uid string) error {
	return s.err
}

func testSession(t *testing.T) *auth.Session {
	t.Helper()
	sess, err := auth.ParseSession("SESSDATA=x; bili_jct=tok; DedeUserID=42")
	if err != nil {
		t.Fatalf("ParseSession 失败: %v", err)
	}
	return sess
}

func TestAccountSharesLimiterAcrossRooms(t *testing.T) {
	// 风控按账号算：同一账号的不同房间必须共用限流器
	acc := New("主播号", testSession(t), 60*time.Millisecond)

	a := &Binding{Account: acc, RoomID: "甲", Actions: &stubActions{}}
	b := &Binding{Account: acc, RoomID: "乙", Actions: &stubActions{}}

	ctx := context.Background()
	if err := a.SendDanmaku(ctx, "第一条"); err != nil {
		t.Fatalf("发送失败: %v", err)
	}
	start := time.Now()
	if err := b.SendDanmaku(ctx, "第二条"); err != nil {
		t.Fatalf("发送失败: %v", err)
	}
	if d := time.Since(start); d < 40*time.Millisecond {
		t.Errorf("同账号的另一个房间应受同一限流器约束，实际间隔 %v", d)
	}
}

func TestDifferentAccountsDoNotShareLimiter(t *testing.T) {
	a := New("账号A", testSession(t), 5*time.Second)
	b := New("账号B", testSession(t), 5*time.Second)

	ba := &Binding{Account: a, RoomID: "甲", Actions: &stubActions{}}
	bb := &Binding{Account: b, RoomID: "甲", Actions: &stubActions{}}

	ctx := context.Background()
	if err := ba.SendDanmaku(ctx, "A 发的"); err != nil {
		t.Fatalf("发送失败: %v", err)
	}
	start := time.Now()
	if err := bb.SendDanmaku(ctx, "B 发的"); err != nil {
		t.Fatalf("发送失败: %v", err)
	}
	if d := time.Since(start); d > 500*time.Millisecond {
		t.Errorf("不同账号不应互相拖慢，实际等待 %v", d)
	}
}

func TestBindingPassesRoomID(t *testing.T) {
	st := &stubActions{}
	b := &Binding{
		Account: New("账号", testSession(t), 0),
		RoomID:  "1706666491",
		Actions: st,
	}

	ctx := context.Background()
	if err := b.SendDanmaku(ctx, "你好"); err != nil {
		t.Fatalf("SendDanmaku 失败: %v", err)
	}
	if err := b.Block(ctx, "999", 12); err != nil {
		t.Fatalf("Block 失败: %v", err)
	}

	if len(st.rooms) != 2 {
		t.Fatalf("调用数 = %d", len(st.rooms))
	}
	for i, r := range st.rooms {
		if r != "1706666491" {
			t.Errorf("第 %d 次调用的 roomID = %q", i+1, r)
		}
	}
	if len(st.sent) != 1 || st.sent[0] != "你好" {
		t.Errorf("sent = %v", st.sent)
	}
	if len(st.blocks) != 1 || st.blocks[0] != "999" {
		t.Errorf("blocks = %v", st.blocks)
	}
}

func TestBindingReportsErrorWithoutFallback(t *testing.T) {
	// 账号失效就报错，不切换到其他账号
	st := &stubActions{err: &api.APIError{Code: -101, Message: "账号未登录"}}
	b := &Binding{
		Account: New("失效账号", testSession(t), 0),
		RoomID:  "甲",
		Actions: st,
	}

	err := b.SendDanmaku(context.Background(), "消息")
	if err == nil {
		t.Fatal("账号失效应当报错")
	}
	// 错误信息要能定位到是哪个账号在哪个房间出的问题
	if !strings.Contains(err.Error(), "失效账号") {
		t.Errorf("错误信息应含账号名，实际 %v", err)
	}
	if !strings.Contains(err.Error(), "甲") {
		t.Errorf("错误信息应含房间号，实际 %v", err)
	}
}

func TestBindingLabel(t *testing.T) {
	b := &Binding{
		Account: New("主播号", testSession(t), 0),
		RoomID:  "1706666491",
	}
	if got := b.Label(); got != "主播号@1706666491" {
		t.Errorf("Label = %q", got)
	}
}

func TestBindingWithoutAccountFails(t *testing.T) {
	b := &Binding{RoomID: "甲", Actions: &stubActions{}}
	if err := b.SendDanmaku(context.Background(), "x"); !errors.Is(err, ErrNoAccount) {
		t.Errorf("err = %v, 期望 ErrNoAccount", err)
	}
}

func TestBindingRespectsContext(t *testing.T) {
	acc := New("账号", testSession(t), 5*time.Second)
	b := &Binding{Account: acc, RoomID: "甲", Actions: &stubActions{}}

	ctx := context.Background()
	if err := b.SendDanmaku(ctx, "第一条"); err != nil {
		t.Fatalf("发送失败: %v", err)
	}

	ctx2, cancel := context.WithTimeout(ctx, 50*time.Millisecond)
	defer cancel()
	if err := b.SendDanmaku(ctx2, "第二条"); err == nil {
		t.Error("ctx 超时后应返回错误")
	}
}

func TestBindingConcurrentSend(t *testing.T) {
	st := &stubActions{}
	b := &Binding{Account: New("账号", testSession(t), 0), RoomID: "甲", Actions: st}

	var wg sync.WaitGroup
	for i := 0; i < 30; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			b.SendDanmaku(context.Background(), "并发")
		}()
	}
	wg.Wait()

	st.mu.Lock()
	defer st.mu.Unlock()
	if len(st.sent) != 30 {
		t.Errorf("发送数 = %d, 期望 30", len(st.sent))
	}
}
```

- [ ] **Step 2: 运行测试确认失败**

```bash
cd server && go test ./internal/account/ -v
```

Expected: 编译失败，`undefined: New`。

- [ ] **Step 3: 实现**

创建 `server/internal/account/account.go`：

```go
// Package account 定义账号与「账号-直播间」绑定。
//
// 账号不是可互换的资源，而是各有职责的参与者：主播号可能只做统计与
// 房管而不发言，小号负责欢迎答谢。因此本包不提供轮换或 fallback——
// 指定账号失效就报错，由使用者处理。
package account

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/MrZhongzq/MagicalDanmaku/server/internal/connector"
	"github.com/MrZhongzq/MagicalDanmaku/server/internal/connector/bilibili/auth"
	"github.com/MrZhongzq/MagicalDanmaku/server/internal/ratelimit"
)

// ErrNoAccount 表示绑定缺少账号。
var ErrNoAccount = errors.New("account: 绑定未指定账号")

// Account 是一个已登录账号，可连接多个直播间。
//
// Limiter 挂在账号上而非绑定上：B 站的风控按账号计算，同一账号的全部
// 直播间必须共享发送节奏，不同账号之间则完全独立。
type Account struct {
	Name    string
	Session *auth.Session
	Limiter ratelimit.Limiter
}

// New 创建账号。interval 为该账号全部直播间共享的最小发送间隔。
func New(name string, sess *auth.Session, interval time.Duration) *Account {
	return &Account{
		Name:    name,
		Session: sess,
		Limiter: ratelimit.NewInterval(interval),
	}
}

// Binding 是「账号-直播间」组合，P2 的运行单元。
//
// 同一直播间被两个账号连接时是两个独立 Binding，各自有独立的连接、
// 规则集与冷却状态，互不知道对方存在。
type Binding struct {
	Account *Account
	RoomID  string
	Actions connector.Actions
}

// Label 返回用于日志的标识，形如 "主播号@1706666491"。
func (b *Binding) Label() string {
	name := "(未指定账号)"
	if b.Account != nil {
		name = b.Account.Name
	}
	return name + "@" + b.RoomID
}

// SendDanmaku 以本绑定的账号身份，向本绑定的直播间发送弹幕。
func (b *Binding) SendDanmaku(ctx context.Context, text string) error {
	if b.Account == nil {
		return ErrNoAccount
	}
	if err := b.Account.Limiter.Wait(ctx); err != nil {
		return err
	}
	err := b.Actions.SendDanmaku(ctx, connector.SendDanmakuRequest{
		RoomID: b.RoomID,
		Text:   text,
	})
	if err != nil {
		return fmt.Errorf("%s 发送弹幕失败: %w", b.Label(), err)
	}
	return nil
}

// Block 以本绑定的账号身份，在本绑定的直播间禁言用户。
func (b *Binding) Block(ctx context.Context, uid string, hours int) error {
	if b.Account == nil {
		return ErrNoAccount
	}
	if err := b.Account.Limiter.Wait(ctx); err != nil {
		return err
	}
	err := b.Actions.BlockUser(ctx, connector.BlockRequest{
		RoomID: b.RoomID,
		UID:    uid,
		Hours:  hours,
	})
	if err != nil {
		return fmt.Errorf("%s 禁言 %s 失败: %w", b.Label(), uid, err)
	}
	return nil
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
git commit -m "feat: 实现账号与直播间绑定模型"
```

---

**下一步：** 继续阅读 `2026-07-31-p2-rule-engine-part3.md`，实现定时任务、
YAML 配置、Pipeline 组装与 `magicd run` 子命令。
