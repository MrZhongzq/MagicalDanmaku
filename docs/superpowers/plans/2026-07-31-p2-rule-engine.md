# P2 规则引擎 Implementation Plan · Part 1（模型与求值）

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 把 P0 的归一化事件流变成可配置的自动化行为，交付 `magicd run -c config.yaml` —— 一个真正能跑的无头弹幕机器人。

**Architecture:** 每房间一条串行 Pipeline：合并窗口（去重+聚合）→ 规则匹配（结构化条件）→ 动作执行（模板/脚本）→ 三层冷却 → P0 的 `connector.Actions`。规则是结构化数据而非代码，为 P3 存库与 P4 表单编辑留好接口。

**Tech Stack:** Go 1.24、`dop251/goja`（JS 沙箱）、`robfig/cron/v3`（定时）、`gopkg.in/yaml.v3`（配置）、stdlib `text/template`。三者均为纯 Go，已实测不破坏 `CGO_ENABLED=0` 交叉编译。

## Global Constraints

- Go module 路径：`github.com/MrZhongzq/MagicalDanmaku/server`
- 仅使用 stdlib `testing`，不引入断言库
- P2 **只通过 P0 的公开接口交互**，不修改 P0 代码；唯一例外是修复 P0 的事件映射缺陷（需同时补黄金样本）
- `Trigger.Vars` 是条件求值与模板渲染的**唯一取值来源**，字段展开逻辑只能存在于 `vars.go` 一处
- Vars 中不存在的路径求值为 `nil`；条件比较视为不匹配，模板渲染输出空串——**不得因字段缺失而报错或崩溃**
- 单条规则出错不得影响其他规则；单个房间出错不得影响其他房间
- goja 沙箱**不注入网络访问**
- 所有导出标识符带中文注释；错误信息使用中文
- 提交信息使用中文，格式 `<type>: <描述>`
- 每个任务结束前必须通过：`go test ./... -count=1`、`go vet ./...`、`gofmt -l .` 无输出

## 文件结构

```
server/internal/rules/
├── rule.go        Rule / Condition / Action / AggregateSpec 模型与校验
├── trigger.go     Trigger 定义与构造
├── vars.go        Event → Vars 展开（唯一取值来源）
├── condition.go   条件求值器（纯函数，不起 goja）
├── template.go    模板渲染与内置函数
├── script.go      goja 沙箱与 Runtime 池
├── cooldown.go    三层节流
├── aggregate.go   合并窗口与去重
├── matcher.go     规则匹配
├── executor.go    动作执行
├── engine.go      Pipeline 组装
└── config/
    └── yaml.go    YAML 加载与校验
server/internal/account/
└── pool.go        多账号轮换
server/internal/scheduler/
└── cron.go        定时任务
server/cmd/magicd/
└── run.go         run 子命令
```

**分篇说明：** Part 1 覆盖 Task 1–5（模型、Vars、条件、模板、沙箱），
Part 2 覆盖 Task 6–10，Part 3 覆盖 Task 11–14。

---

### Task 1: 规则模型与 Trigger

**Files:**
- Create: `server/internal/rules/rule.go`
- Create: `server/internal/rules/trigger.go`
- Test: `server/internal/rules/rule_test.go`

**Interfaces:**
- Consumes: `event.Type`、`event.Event`（P0 已有）
- Produces:
  - `rules.Rule{Name, Enabled, On []event.Type, Schedule string, When *Condition, Aggregate *AggregateSpec, Do []Action, Cooldown time.Duration, CooldownGroup string}`
  - `rules.Condition{Field, Op string, Value any, All, Any []Condition, Not *Condition, Script string}`
  - `rules.Action{Type ActionType, Template []string, Hours int, Script string}`
  - `rules.ActionType` 与常量 `ActionDanmaku`、`ActionBlock`、`ActionScript`、`ActionLog`
  - `rules.AggregateSpec{Window time.Duration, By AggregateBy}`
  - `rules.AggregateBy` 与常量 `AggregateByType`、`AggregateByUser`、`AggregateByGift`
  - `(Rule).Validate() error`、`(Condition).Validate() error`、`(Action).Validate() error`
  - `rules.Trigger{Type event.Type, Events []event.Event, Vars map[string]any}`
  - `rules.TypeScheduled event.Type`（定时任务专用的伪事件类型）

- [ ] **Step 1: 写失败测试**

创建 `server/internal/rules/rule_test.go`：

```go
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
```

- [ ] **Step 2: 运行测试确认失败**

```bash
cd server && go test ./internal/rules/ -v
```

Expected: 编译失败，`undefined: Rule`。

- [ ] **Step 3: 实现模型**

创建 `server/internal/rules/rule.go`：

```go
// Package rules 实现声明式规则引擎：结构化条件匹配 + 模板/脚本动作。
package rules

import (
	"fmt"
	"time"

	"github.com/MrZhongzq/MagicalDanmaku/server/internal/event"
)

// ActionType 是动作类型。
type ActionType string

// 全部动作类型。
const (
	ActionDanmaku ActionType = "danmaku" // 发弹幕
	ActionBlock   ActionType = "block"   // 禁言
	ActionScript  ActionType = "script"  // 执行脚本
	ActionLog     ActionType = "log"     // 只记日志，用于调试规则
)

// AggregateBy 是合并窗口的分组键。
type AggregateBy string

// 全部分组方式。
const (
	AggregateByType AggregateBy = "type" // 按事件类型：窗口内全部合成一条
	AggregateByUser AggregateBy = "user" // 按类型+UID：仅去重不聚合
	AggregateByGift AggregateBy = "gift" // 按类型+UID+礼物名：数量累加
)

// validOps 是条件支持的全部操作符。
var validOps = map[string]bool{
	"eq": true, "ne": true,
	"gt": true, "gte": true, "lt": true, "lte": true,
	"contains": true, "prefix": true, "suffix": true, "regex": true,
	"in": true,
}

// Rule 是一条规则。
//
// 触发方式二选一：On 由事件驱动，Schedule 由 cron 驱动。
type Rule struct {
	Name          string         // 规则名，用于日志与去重，必填
	Enabled       bool           // 是否启用
	On            []event.Type   // 事件触发：监听的事件类型
	Schedule      string         // 定时触发：6 字段 cron 表达式
	When          *Condition     // 过滤条件，nil 表示无条件
	Aggregate     *AggregateSpec // 合并窗口，nil 表示不合并
	Do            []Action       // 动作列表，按序执行
	Cooldown      time.Duration  // 本规则最小触发间隔
	CooldownGroup string         // 命名冷却组，可空
}

// Validate 校验规则自身的完整性。
func (r Rule) Validate() error {
	if r.Name == "" {
		return fmt.Errorf("rules: 规则名不能为空")
	}

	hasOn := len(r.On) > 0
	hasSchedule := r.Schedule != ""
	if hasOn && hasSchedule {
		return fmt.Errorf("rules: 规则 %q 的 on 与 schedule 互斥，不能同时指定", r.Name)
	}
	if !hasOn && !hasSchedule {
		return fmt.Errorf("rules: 规则 %q 必须指定 on 或 schedule 之一", r.Name)
	}

	if len(r.Do) == 0 {
		return fmt.Errorf("rules: 规则 %q 的动作列表不能为空", r.Name)
	}
	for i, a := range r.Do {
		if err := a.Validate(); err != nil {
			return fmt.Errorf("rules: 规则 %q 的第 %d 个动作非法: %w", r.Name, i+1, err)
		}
	}

	if r.When != nil {
		if err := r.When.Validate(); err != nil {
			return fmt.Errorf("rules: 规则 %q 的条件非法: %w", r.Name, err)
		}
	}
	if r.Aggregate != nil {
		if err := r.Aggregate.Validate(); err != nil {
			return fmt.Errorf("rules: 规则 %q 的合并规格非法: %w", r.Name, err)
		}
	}
	return nil
}

// Condition 是一个条件节点。
//
// Field（叶子）、All、Any、Not、Script 五种形态互斥，只能有一个生效。
type Condition struct {
	Field string // Vars 中的路径，如 "user.guardLevel"
	Op    string // 操作符，见 validOps
	Value any    // 比较值

	All []Condition // 全部满足
	Any []Condition // 任一满足
	Not *Condition  // 取反

	Script string // JS 表达式逃生舱，须返回 boolean
}

// Validate 递归校验条件树。
func (c Condition) Validate() error {
	// 统计有几种形态被指定
	forms := 0
	if c.Field != "" {
		forms++
	}
	if len(c.All) > 0 {
		forms++
	}
	if len(c.Any) > 0 {
		forms++
	}
	if c.Not != nil {
		forms++
	}
	if c.Script != "" {
		forms++
	}

	switch {
	case forms == 0:
		return fmt.Errorf("条件为空，须指定 field / all / any / not / script 之一")
	case forms > 1:
		return fmt.Errorf("field / all / any / not / script 只能指定一个")
	}

	if c.Field != "" && !validOps[c.Op] {
		return fmt.Errorf("未知的操作符 %q", c.Op)
	}
	for i, sub := range c.All {
		if err := sub.Validate(); err != nil {
			return fmt.Errorf("all 的第 %d 项非法: %w", i+1, err)
		}
	}
	for i, sub := range c.Any {
		if err := sub.Validate(); err != nil {
			return fmt.Errorf("any 的第 %d 项非法: %w", i+1, err)
		}
	}
	if c.Not != nil {
		if err := c.Not.Validate(); err != nil {
			return fmt.Errorf("not 的子条件非法: %w", err)
		}
	}
	return nil
}

// Action 是一个动作。
type Action struct {
	Type ActionType

	// Type == ActionDanmaku 时使用，多条则随机挑一条
	Template []string

	// Type == ActionBlock 时使用，禁言小时数
	Hours int

	// Type == ActionScript 时使用
	Script string
}

// Validate 校验动作。
func (a Action) Validate() error {
	switch a.Type {
	case ActionDanmaku:
		if len(a.Template) == 0 {
			return fmt.Errorf("danmaku 动作必须提供至少一条模板")
		}
	case ActionScript:
		if a.Script == "" {
			return fmt.Errorf("script 动作必须提供脚本内容")
		}
	case ActionBlock, ActionLog:
		// 无必填字段
	default:
		return fmt.Errorf("未知的动作类型 %q", a.Type)
	}
	return nil
}

// AggregateSpec 描述如何合并窗口内的事件。
type AggregateSpec struct {
	Window time.Duration // 缓冲时长
	By     AggregateBy   // 分组键
}

// Validate 校验合并规格。
func (s AggregateSpec) Validate() error {
	if s.Window <= 0 {
		return fmt.Errorf("合并窗口必须大于 0")
	}
	switch s.By {
	case AggregateByType, AggregateByUser, AggregateByGift:
		return nil
	default:
		return fmt.Errorf("未知的分组键 %q", s.By)
	}
}
```

- [ ] **Step 4: 实现 Trigger**

创建 `server/internal/rules/trigger.go`：

```go
package rules

import "github.com/MrZhongzq/MagicalDanmaku/server/internal/event"

// TypeScheduled 是定时任务专用的伪事件类型。
//
// 定时触发也走与事件规则完全相同的匹配与执行路径，
// 因此需要一个不与真实事件冲突的类型标识。
const TypeScheduled event.Type = "scheduled"

// Trigger 是规则匹配的输入单元。
//
// 未经合并时 Events 长度为 1；经合并窗口聚合后为 N。
type Trigger struct {
	Type   event.Type     // 事件类型
	Events []event.Event  // 参与本次触发的原始事件
	Vars   map[string]any // 条件求值与模板渲染的唯一取值来源
}
```

- [ ] **Step 5: 运行测试确认通过**

```bash
cd server && go test ./internal/rules/ -v
```

Expected: 全部 PASS。

- [ ] **Step 6: 提交**

```bash
cd server && go vet ./... && gofmt -l .
git add server/internal/rules/
git commit -m "feat: 新增规则模型与 Trigger 定义"
```

---

### Task 2: Vars 展开

**Files:**
- Create: `server/internal/rules/vars.go`
- Test: `server/internal/rules/vars_test.go`

**Interfaces:**
- Consumes: Task 1 的 `Trigger`；P0 的全部 `event.Payload` 类型
- Produces:
  - `rules.VarsFromEvent(ev event.Event) map[string]any`
  - `rules.LookupPath(vars map[string]any, path string) (any, bool)` — 按点分路径取值
  - `rules.MergeVars(dst, src map[string]any)` — 逐字段合并，空值不覆盖非空值

**这是全项目最关键的约定之一：** 条件里写 `user.guardLevel`、模板里写
`{{.user.guardLevel}}`，两者必须指向同一份数据。展开逻辑**只能存在于本文件**。

- [ ] **Step 1: 写失败测试**

创建 `server/internal/rules/vars_test.go`：

```go
package rules

import (
	"testing"
	"time"

	"github.com/MrZhongzq/MagicalDanmaku/server/internal/event"
)

func danmakuEvent() event.Event {
	return event.Event{
		Type:      event.TypeDanmaku,
		RoomID:    "1706666491",
		Timestamp: time.Unix(1753920000, 0),
		Payload: event.Danmaku{
			User: event.User{
				UID: "12345678", Username: "路人甲",
				GuardLevel: 3, UserLevel: 18, WealthLevel: 7, IsAdmin: true,
				Medal: &event.Medal{Name: "真yu中", Level: 24, RoomID: "999"},
			},
			Text:  "主播晚上好",
			Color: "#ffffff",
		},
	}
}

func TestVarsFromDanmaku(t *testing.T) {
	v := VarsFromEvent(danmakuEvent())

	cases := map[string]any{
		"type":             "danmaku",
		"roomId":           "1706666491",
		"text":             "主播晚上好",
		"user.uid":         "12345678",
		"user.username":    "路人甲",
		"user.guardLevel":  3,
		"user.userLevel":   18,
		"user.wealthLevel": 7,
		"user.isAdmin":     true,
		"user.medal.name":  "真yu中",
		"user.medal.level": 24,
	}
	for path, want := range cases {
		got, ok := LookupPath(v, path)
		if !ok {
			t.Errorf("路径 %q 不存在", path)
			continue
		}
		if got != want {
			t.Errorf("%s = %v (%T), 期望 %v (%T)", path, got, got, want, want)
		}
	}
}

func TestVarsMissingMedalIsAbsent(t *testing.T) {
	ev := danmakuEvent()
	d := ev.Payload.(event.Danmaku)
	d.User.Medal = nil
	ev.Payload = d

	v := VarsFromEvent(ev)
	if _, ok := LookupPath(v, "user.medal.name"); ok {
		t.Error("未佩戴勋章时 user.medal.name 不应存在")
	}
	// 但 user.username 仍应存在
	if _, ok := LookupPath(v, "user.username"); !ok {
		t.Error("user.username 应当存在")
	}
}

func TestVarsFromGift(t *testing.T) {
	ev := event.Event{
		Type: event.TypeGift, RoomID: "1", Timestamp: time.Unix(1700000000, 0),
		Payload: event.Gift{
			User:     event.User{UID: "9", Username: "土豪"},
			GiftID:   31531,
			GiftName: "小花花",
			Count:    10,
			CoinType: "gold", TotalCoin: 10000, Action: "投喂",
		},
	}
	v := VarsFromEvent(ev)
	cases := map[string]any{
		"gift.name":      "小花花",
		"gift.count":     int64(10),
		"gift.coinType":  "gold",
		"gift.totalCoin": int64(10000),
		"user.username":  "土豪",
	}
	for path, want := range cases {
		got, _ := LookupPath(v, path)
		if got != want {
			t.Errorf("%s = %v (%T), 期望 %v (%T)", path, got, got, want, want)
		}
	}
}

func TestVarsFromGuardBuy(t *testing.T) {
	ev := event.Event{
		Type: event.TypeGuardBuy, RoomID: "1", Timestamp: time.Unix(1700000000, 0),
		Payload: event.GuardBuy{
			User:       event.User{UID: "9", Username: "新舰长"},
			GuardLevel: 3, GuardName: "舰长", Count: 1, Price: 198000, IsRenew: false,
		},
	}
	v := VarsFromEvent(ev)
	if got, _ := LookupPath(v, "guard.name"); got != "舰长" {
		t.Errorf("guard.name = %v", got)
	}
	if got, _ := LookupPath(v, "guard.isRenew"); got != false {
		t.Errorf("guard.isRenew = %v", got)
	}
}

func TestVarsFromSuperChat(t *testing.T) {
	ev := event.Event{
		Type: event.TypeSuperChat, RoomID: "1", Timestamp: time.Unix(1700000000, 0),
		Payload: event.SuperChat{
			User: event.User{UID: "9", Username: "SC用户"},
			Text: "加油", Price: 30, Duration: 60,
		},
	}
	v := VarsFromEvent(ev)
	if got, _ := LookupPath(v, "text"); got != "加油" {
		t.Errorf("text = %v", got)
	}
	if got, _ := LookupPath(v, "superChat.price"); got != int64(30) {
		t.Errorf("superChat.price = %v", got)
	}
}

func TestVarsFromUserEnter(t *testing.T) {
	ev := event.Event{
		Type: event.TypeUserEnter, RoomID: "1", Timestamp: time.Unix(1700000000, 0),
		Payload: event.UserEnter{User: event.User{UID: "9", Username: "进场用户", GuardLevel: 3}},
	}
	v := VarsFromEvent(ev)
	if got, _ := LookupPath(v, "user.username"); got != "进场用户" {
		t.Errorf("user.username = %v", got)
	}
	if got, _ := LookupPath(v, "user.guardLevel"); got != 3 {
		t.Errorf("user.guardLevel = %v", got)
	}
}

func TestLookupPathMissingReturnsFalse(t *testing.T) {
	v := VarsFromEvent(danmakuEvent())
	for _, p := range []string{"不存在", "user.不存在", "text.深一层", ""} {
		if got, ok := LookupPath(v, p); ok {
			t.Errorf("路径 %q 不应存在，却返回 %v", p, got)
		}
	}
}

func TestMergeVarsKeepsNonEmpty(t *testing.T) {
	// 模拟 ENTRY_EFFECT（无昵称）与 INTERACT_WORD_V2（完整）的合并
	sparse := map[string]any{
		"type": "user_enter",
		"user": map[string]any{"uid": "123", "username": "", "guardLevel": 3},
	}
	full := map[string]any{
		"type": "user_enter",
		"user": map[string]any{"uid": "123", "username": "完整昵称", "guardLevel": 0},
	}

	MergeVars(sparse, full)

	u := sparse["user"].(map[string]any)
	if u["username"] != "完整昵称" {
		t.Errorf("空值应被非空值覆盖，实际 %v", u["username"])
	}
	if u["guardLevel"] != 3 {
		t.Errorf("非空值不应被空值覆盖，实际 %v", u["guardLevel"])
	}
}

func TestMergeVarsAddsMissingKeys(t *testing.T) {
	dst := map[string]any{"a": 1}
	MergeVars(dst, map[string]any{"b": 2})
	if dst["b"] != 2 {
		t.Errorf("缺失的键应被补上，实际 %v", dst)
	}
}
```

- [ ] **Step 2: 运行测试确认失败**

```bash
cd server && go test ./internal/rules/ -run 'TestVars|TestLookup|TestMerge' -v
```

Expected: 编译失败，`undefined: VarsFromEvent`。

- [ ] **Step 3: 实现**

创建 `server/internal/rules/vars.go`：

```go
package rules

import (
	"strings"

	"github.com/MrZhongzq/MagicalDanmaku/server/internal/event"
)

// VarsFromEvent 把事件展开为条件求值与模板渲染共用的取值表。
//
// 这是全项目唯一的字段展开处。条件里写 "user.guardLevel"、模板里写
// "{{.user.guardLevel}}"，两者指向同一份数据，杜绝两套字段名各自演化。
//
// 约定：值为零值的可选字段（如未佩戴勋章）不写入表中，
// 使 LookupPath 能区分「字段不存在」与「字段值为零」。
func VarsFromEvent(ev event.Event) map[string]any {
	v := map[string]any{
		"type":      string(ev.Type),
		"roomId":    ev.RoomID,
		"timestamp": ev.Timestamp.Unix(),
	}

	switch p := ev.Payload.(type) {
	case event.Danmaku:
		v["user"] = userVars(p.User)
		v["text"] = p.Text
		v["danmaku"] = map[string]any{
			"color":      p.Color,
			"isEmoticon": p.IsEmoticon,
			"replyToUid": p.ReplyToUID,
		}
	case event.SuperChat:
		v["user"] = userVars(p.User)
		v["text"] = p.Text
		v["superChat"] = map[string]any{
			"id": p.ID, "price": p.Price, "duration": int64(p.Duration),
		}
	case event.Gift:
		v["user"] = userVars(p.User)
		v["gift"] = map[string]any{
			"id": p.GiftID, "name": p.GiftName, "count": p.Count,
			"coinType": p.CoinType, "totalCoin": p.TotalCoin, "action": p.Action,
		}
	case event.GiftCombo:
		v["user"] = userVars(p.User)
		v["gift"] = map[string]any{
			"id": p.GiftID, "name": p.GiftName, "count": p.Count,
			"totalCoin": p.TotalCoin, "comboId": p.ComboID,
		}
	case event.GuardBuy:
		v["user"] = userVars(p.User)
		v["guard"] = map[string]any{
			"level": int64(p.GuardLevel), "name": p.GuardName,
			"count": int64(p.Count), "price": p.Price, "isRenew": p.IsRenew,
		}
	case event.UserEnter:
		v["user"] = userVars(p.User)
	case event.UserFollow:
		v["user"] = userVars(p.User)
	case event.UserShare:
		v["user"] = userVars(p.User)
	case event.UserLike:
		v["user"] = userVars(p.User)
	case event.UserBlocked:
		v["user"] = userVars(p.User)
	case event.RoomChange:
		v["room"] = map[string]any{
			"title": p.Title, "areaName": p.AreaName, "parentAreaName": p.ParentAreaName,
		}
	case event.RoomStatsUpdate:
		stats := map[string]any{}
		if p.Fans != nil {
			stats["fans"] = *p.Fans
		}
		if p.FansClub != nil {
			stats["fansClub"] = *p.FansClub
		}
		if p.Watched != nil {
			stats["watched"] = *p.Watched
		}
		if p.LikeCount != nil {
			stats["likeCount"] = *p.LikeCount
		}
		v["stats"] = stats
	case event.OnlineRankUpdate:
		v["rank"] = map[string]any{"count": int64(p.Count)}
	case event.Battle:
		v["battle"] = map[string]any{"subCommand": p.SubCommand}
	case event.Unknown:
		v["unknown"] = map[string]any{"command": p.Command}
	}
	return v
}

// userVars 展开用户信息。零值的可选字段不写入。
func userVars(u event.User) map[string]any {
	m := map[string]any{
		"uid":         u.UID,
		"username":    u.Username,
		"guardLevel":  u.GuardLevel,
		"userLevel":   u.UserLevel,
		"wealthLevel": u.WealthLevel,
		"isAdmin":     u.IsAdmin,
	}
	if u.AvatarURL != "" {
		m["avatarUrl"] = u.AvatarURL
	}
	if u.Medal != nil {
		m["medal"] = map[string]any{
			"name":       u.Medal.Name,
			"level":      u.Medal.Level,
			"roomId":     u.Medal.RoomID,
			"anchorUid":  u.Medal.AnchorUID,
			"guardLevel": u.Medal.GuardLevel,
			"isLighted":  u.Medal.IsLighted,
		}
	}
	return m
}

// LookupPath 按点分路径取值，如 "user.medal.level"。
// 路径不存在时返回 (nil, false)。
func LookupPath(vars map[string]any, path string) (any, bool) {
	if path == "" {
		return nil, false
	}
	parts := strings.Split(path, ".")

	var cur any = vars
	for _, p := range parts {
		m, ok := cur.(map[string]any)
		if !ok {
			return nil, false
		}
		cur, ok = m[p]
		if !ok {
			return nil, false
		}
	}
	return cur, true
}

// MergeVars 把 src 逐字段合并进 dst。
//
// 合并规则：dst 中缺失的键直接补上；已存在的键，只有当 dst 的值为
// 零值而 src 非零值时才覆盖。嵌套的 map 递归合并。
//
// 这条规则解决了 P0 联调发现的进场重复问题：ENTRY_EFFECT 只有 UID
// 没有昵称，INTERACT_WORD_V2 信息完整，两者合并后得到完整记录。
func MergeVars(dst, src map[string]any) {
	for k, sv := range src {
		dv, exists := dst[k]
		if !exists {
			dst[k] = sv
			continue
		}
		// 嵌套 map 递归合并
		if dm, ok := dv.(map[string]any); ok {
			if sm, ok := sv.(map[string]any); ok {
				MergeVars(dm, sm)
				continue
			}
		}
		if isZeroValue(dv) && !isZeroValue(sv) {
			dst[k] = sv
		}
	}
}

// isZeroValue 判断是否为「空」值：空串、0、false、nil。
func isZeroValue(v any) bool {
	switch t := v.(type) {
	case nil:
		return true
	case string:
		return t == ""
	case bool:
		return !t
	case int:
		return t == 0
	case int64:
		return t == 0
	case float64:
		return t == 0
	default:
		return false
	}
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
git commit -m "feat: 实现事件到 Vars 的字段展开"
```

---

### Task 3: 条件求值器

**Files:**
- Create: `server/internal/rules/condition.go`
- Test: `server/internal/rules/condition_test.go`

**Interfaces:**
- Consumes: Task 1 的 `Condition`；Task 2 的 `LookupPath`
- Produces:
  - `rules.Evaluator` 接口：`Eval(c Condition, vars map[string]any) (bool, error)`
  - `rules.NewEvaluator(script ScriptRunner) Evaluator`
  - `rules.ScriptRunner` 接口：`EvalBool(code string, vars map[string]any) (bool, error)`
  - `rules.ErrNoScriptRunner`

**关键约束：** 条件求值器是纯函数，**不起 goja**。只有写了 `Script` 的
条件才委托给 `ScriptRunner`。绝大多数规则不该为沙箱付开销。

- [ ] **Step 1: 写失败测试**

创建 `server/internal/rules/condition_test.go`：

```go
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
```

- [ ] **Step 2: 运行测试确认失败**

```bash
cd server && go test ./internal/rules/ -run TestEval -v
```

Expected: 编译失败，`undefined: NewEvaluator`。

- [ ] **Step 3: 实现**

创建 `server/internal/rules/condition.go`：

```go
package rules

import (
	"errors"
	"fmt"
	"regexp"
	"strings"
	"sync"
)

// ErrNoScriptRunner 表示条件使用了脚本，但未配置脚本执行器。
var ErrNoScriptRunner = errors.New("rules: 条件使用了脚本，但未配置脚本执行器")

// ScriptRunner 执行 JS 表达式并返回布尔结果。
type ScriptRunner interface {
	// EvalBool 求值一段 JS 表达式，vars 会作为全局 event 注入。
	EvalBool(code string, vars map[string]any) (bool, error)
}

// Evaluator 求值条件树。
type Evaluator interface {
	Eval(c Condition, vars map[string]any) (bool, error)
}

// evaluator 是默认实现。
type evaluator struct {
	script ScriptRunner

	// 正则编译结果缓存：同一条规则会被反复求值，
	// 每次重新编译在高频弹幕下开销显著。
	mu    sync.RWMutex
	reCache map[string]*regexp.Regexp
}

// NewEvaluator 创建条件求值器。script 可为 nil，此时不支持脚本条件。
func NewEvaluator(script ScriptRunner) Evaluator {
	return &evaluator{script: script, reCache: make(map[string]*regexp.Regexp)}
}

// Eval 递归求值条件树。
//
// 零值条件视为无条件通过，便于 Rule.When == nil 时统一处理。
func (e *evaluator) Eval(c Condition, vars map[string]any) (bool, error) {
	switch {
	case len(c.All) > 0:
		for _, sub := range c.All {
			ok, err := e.Eval(sub, vars)
			if err != nil {
				return false, err
			}
			if !ok {
				return false, nil
			}
		}
		return true, nil

	case len(c.Any) > 0:
		for _, sub := range c.Any {
			ok, err := e.Eval(sub, vars)
			if err != nil {
				return false, err
			}
			if ok {
				return true, nil
			}
		}
		return false, nil

	case c.Not != nil:
		ok, err := e.Eval(*c.Not, vars)
		if err != nil {
			return false, err
		}
		return !ok, nil

	case c.Script != "":
		if e.script == nil {
			return false, ErrNoScriptRunner
		}
		return e.script.EvalBool(c.Script, vars)

	case c.Field != "":
		return e.evalLeaf(c, vars)

	default:
		// 零值条件：无条件通过
		return true, nil
	}
}

// evalLeaf 求值单个字段比较。
//
// 字段缺失一律视为不匹配而非报错——B 站的字段时有时无，
// 规则不该因此崩掉。
func (e *evaluator) evalLeaf(c Condition, vars map[string]any) (bool, error) {
	actual, ok := LookupPath(vars, c.Field)
	if !ok {
		return false, nil
	}

	switch c.Op {
	case "eq":
		return looseEqual(actual, c.Value), nil
	case "ne":
		return !looseEqual(actual, c.Value), nil

	case "gt", "gte", "lt", "lte":
		a, ok1 := toFloat(actual)
		b, ok2 := toFloat(c.Value)
		if !ok1 || !ok2 {
			return false, nil // 非数值，视为不匹配
		}
		switch c.Op {
		case "gt":
			return a > b, nil
		case "gte":
			return a >= b, nil
		case "lt":
			return a < b, nil
		default:
			return a <= b, nil
		}

	case "contains":
		return strings.Contains(toString(actual), toString(c.Value)), nil
	case "prefix":
		return strings.HasPrefix(toString(actual), toString(c.Value)), nil
	case "suffix":
		return strings.HasSuffix(toString(actual), toString(c.Value)), nil

	case "regex":
		re, err := e.compile(toString(c.Value))
		if err != nil {
			return false, err
		}
		return re.MatchString(toString(actual)), nil

	case "in":
		list, ok := c.Value.([]any)
		if !ok {
			return false, nil
		}
		for _, item := range list {
			if looseEqual(actual, item) {
				return true, nil
			}
		}
		return false, nil

	default:
		return false, fmt.Errorf("rules: 未知的操作符 %q", c.Op)
	}
}

// compile 编译正则并缓存。
func (e *evaluator) compile(pattern string) (*regexp.Regexp, error) {
	e.mu.RLock()
	re, ok := e.reCache[pattern]
	e.mu.RUnlock()
	if ok {
		return re, nil
	}

	re, err := regexp.Compile(pattern)
	if err != nil {
		return nil, fmt.Errorf("rules: 非法的正则表达式 %q: %w", pattern, err)
	}

	e.mu.Lock()
	e.reCache[pattern] = re
	e.mu.Unlock()
	return re, nil
}

// looseEqual 做宽松相等比较：数值跨类型可比，其余按字符串比。
//
// 必要性：YAML 解析出的整数是 int，而事件里的计数是 int64，
// 严格比较会让 {field: gift.count, op: eq, value: 10} 意外失败。
func looseEqual(a, b any) bool {
	if af, ok1 := toFloat(a); ok1 {
		if bf, ok2 := toFloat(b); ok2 {
			return af == bf
		}
	}
	if ab, ok1 := a.(bool); ok1 {
		if bb, ok2 := b.(bool); ok2 {
			return ab == bb
		}
	}
	return toString(a) == toString(b)
}

// toFloat 把任意数值类型转成 float64。
func toFloat(v any) (float64, bool) {
	switch t := v.(type) {
	case int:
		return float64(t), true
	case int32:
		return float64(t), true
	case int64:
		return float64(t), true
	case float32:
		return float64(t), true
	case float64:
		return t, true
	default:
		return 0, false
	}
}

// toString 把任意值转成字符串用于文本比较。
func toString(v any) string {
	switch t := v.(type) {
	case string:
		return t
	case nil:
		return ""
	default:
		return fmt.Sprintf("%v", t)
	}
}
```

- [ ] **Step 4: 运行测试确认通过**

```bash
cd server && go test ./internal/rules/ -v
```

Expected: 全部 PASS，含 17 个操作符子测试。

- [ ] **Step 5: 提交**

```bash
cd server && go vet ./... && gofmt -l .
git add server/internal/rules/
git commit -m "feat: 实现结构化条件求值器"
```

---

### Task 4: 模板渲染

**Files:**
- Create: `server/internal/rules/template.go`
- Test: `server/internal/rules/template_test.go`
- Modify: `server/go.mod`（无新增依赖，使用 stdlib `text/template`）

**Interfaces:**
- Consumes: Task 1 的 `Trigger`
- Produces:
  - `rules.Renderer` 结构，`rules.NewRenderer(rand *rand.Rand) *Renderer`
  - `(*Renderer).Render(templates []string, vars map[string]any) (string, error)` — 多条时随机挑一条
  - `(*Renderer).RenderOne(tmpl string, vars map[string]any) (string, error)`
  - 内置函数 `join`、`simplifyName`、`pick`、`truncate`

- [ ] **Step 1: 写失败测试**

创建 `server/internal/rules/template_test.go`：

```go
package rules

import (
	"math/rand"
	"strings"
	"testing"
)

func newTestRenderer() *Renderer {
	// 固定种子保证随机选择可复现
	return NewRenderer(rand.New(rand.NewSource(1)))
}

func TestRenderSimpleField(t *testing.T) {
	r := newTestRenderer()
	vars := map[string]any{"user": map[string]any{"username": "路人甲"}}

	got, err := r.RenderOne("欢迎 {{.user.username}} 进入直播间", vars)
	if err != nil {
		t.Fatalf("RenderOne 失败: %v", err)
	}
	if got != "欢迎 路人甲 进入直播间" {
		t.Errorf("= %q", got)
	}
}

func TestRenderMissingFieldIsEmpty(t *testing.T) {
	r := newTestRenderer()
	got, err := r.RenderOne("值是[{{.不存在}}]", map[string]any{})
	if err != nil {
		t.Fatalf("缺失字段不应报错: %v", err)
	}
	if got != "值是[]" {
		t.Errorf("缺失字段应渲染为空串，实际 %q", got)
	}
}

func TestRenderMissingNestedFieldIsEmpty(t *testing.T) {
	r := newTestRenderer()
	vars := map[string]any{"user": map[string]any{"username": "甲"}}
	got, err := r.RenderOne("[{{.user.medal.name}}]", vars)
	if err != nil {
		t.Fatalf("缺失嵌套字段不应报错: %v", err)
	}
	if got != "[]" {
		t.Errorf("= %q", got)
	}
}

func TestRenderJoin(t *testing.T) {
	r := newTestRenderer()
	vars := map[string]any{"users": []string{"甲", "乙", "丙"}}
	got, err := r.RenderOne(`欢迎 {{join .users "、"}} 回家`, vars)
	if err != nil {
		t.Fatalf("RenderOne 失败: %v", err)
	}
	if got != "欢迎 甲、乙、丙 回家" {
		t.Errorf("= %q", got)
	}
}

func TestRenderJoinHandlesAnySlice(t *testing.T) {
	r := newTestRenderer()
	// Vars 里的数组可能是 []any
	vars := map[string]any{"users": []any{"甲", "乙"}}
	got, err := r.RenderOne(`{{join .users ","}}`, vars)
	if err != nil {
		t.Fatalf("RenderOne 失败: %v", err)
	}
	if got != "甲,乙" {
		t.Errorf("= %q", got)
	}
}

func TestRenderSimplifyName(t *testing.T) {
	cases := map[string]string{
		"路人甲":            "路人甲",
		"【官方】某某某":        "某某某",
		"某某某_official":   "某某某",
		"·-·某某·-·":       "某某",
		"某某某-许许的蓷":       "某某某-许许的蓷",
	}
	r := newTestRenderer()
	for in, want := range cases {
		vars := map[string]any{"n": in}
		got, err := r.RenderOne("{{simplifyName .n}}", vars)
		if err != nil {
			t.Fatalf("RenderOne 失败: %v", err)
		}
		if got != want {
			t.Errorf("simplifyName(%q) = %q, 期望 %q", in, got, want)
		}
	}
}

func TestRenderTruncate(t *testing.T) {
	r := newTestRenderer()
	vars := map[string]any{"s": "一二三四五六七八九十"}
	got, err := r.RenderOne("{{truncate .s 5}}", vars)
	if err != nil {
		t.Fatalf("RenderOne 失败: %v", err)
	}
	if got != "一二三四五" {
		t.Errorf("= %q（应按字符而非字节截断）", got)
	}
}

func TestRenderPick(t *testing.T) {
	r := newTestRenderer()
	got, err := r.RenderOne(`{{pick "早上好" "中午好" "晚上好"}}`, map[string]any{})
	if err != nil {
		t.Fatalf("RenderOne 失败: %v", err)
	}
	if got != "早上好" && got != "中午好" && got != "晚上好" {
		t.Errorf("pick 应返回其中之一，实际 %q", got)
	}
}

func TestRenderConditional(t *testing.T) {
	r := newTestRenderer()
	vars := map[string]any{"user": map[string]any{"guardLevel": 3, "username": "甲"}}
	tmpl := `{{if gt (int .user.guardLevel) 0}}舰长{{end}}{{.user.username}}`
	got, err := r.RenderOne(tmpl, vars)
	if err != nil {
		t.Fatalf("RenderOne 失败: %v", err)
	}
	if got != "舰长甲" {
		t.Errorf("= %q", got)
	}
}

func TestRenderPicksFromMultipleTemplates(t *testing.T) {
	r := newTestRenderer()
	tmpls := []string{"A", "B", "C"}
	seen := map[string]bool{}
	for i := 0; i < 60; i++ {
		got, err := r.Render(tmpls, map[string]any{})
		if err != nil {
			t.Fatalf("Render 失败: %v", err)
		}
		seen[got] = true
	}
	if len(seen) != 3 {
		t.Errorf("60 次应覆盖全部 3 条模板，实际只出现 %v", seen)
	}
}

func TestRenderSingleTemplate(t *testing.T) {
	r := newTestRenderer()
	got, err := r.Render([]string{"只有一条"}, map[string]any{})
	if err != nil {
		t.Fatalf("Render 失败: %v", err)
	}
	if got != "只有一条" {
		t.Errorf("= %q", got)
	}
}

func TestRenderEmptyListFails(t *testing.T) {
	r := newTestRenderer()
	if _, err := r.Render(nil, map[string]any{}); err == nil {
		t.Error("空模板列表应当报错")
	}
}

func TestRenderBadSyntaxFails(t *testing.T) {
	r := newTestRenderer()
	if _, err := r.RenderOne("{{.未闭合", map[string]any{}); err == nil {
		t.Error("语法错误应当报错")
	}
	// 错误信息应能定位问题
	_, err := r.RenderOne("{{未知函数 .x}}", map[string]any{})
	if err == nil || !strings.Contains(err.Error(), "模板") {
		t.Errorf("错误信息应提及模板，实际 %v", err)
	}
}
```

- [ ] **Step 2: 运行测试确认失败**

```bash
cd server && go test ./internal/rules/ -run TestRender -v
```

Expected: 编译失败，`undefined: NewRenderer`。

- [ ] **Step 3: 实现**

创建 `server/internal/rules/template.go`：

```go
package rules

import (
	"fmt"
	"math/rand"
	"strings"
	"sync"
	"text/template"
)

// Renderer 渲染动作模板。
//
// 采用标准库 text/template 而非自造语法：已充分测试、支持条件与循环、
// 无需维护解析器。这与「弃用原项目自创 DSL」的决策一脉相承。
type Renderer struct {
	rnd *rand.Rand

	// 模板编译结果缓存：同一条规则会被反复触发，
	// 每次重新解析在高频事件下开销显著。
	mu    sync.Mutex
	cache map[string]*template.Template
}

// NewRenderer 创建渲染器。rnd 为 nil 时使用全局随机源。
func NewRenderer(rnd *rand.Rand) *Renderer {
	return &Renderer{rnd: rnd, cache: make(map[string]*template.Template)}
}

// Render 从多条模板中随机挑一条渲染，实现文案变化。
func (r *Renderer) Render(templates []string, vars map[string]any) (string, error) {
	if len(templates) == 0 {
		return "", fmt.Errorf("rules: 模板列表为空")
	}
	idx := 0
	if len(templates) > 1 {
		idx = r.intn(len(templates))
	}
	return r.RenderOne(templates[idx], vars)
}

// RenderOne 渲染单条模板。
func (r *Renderer) RenderOne(tmpl string, vars map[string]any) (string, error) {
	t, err := r.compile(tmpl)
	if err != nil {
		return "", err
	}
	var sb strings.Builder
	if err := t.Execute(&sb, vars); err != nil {
		return "", fmt.Errorf("rules: 模板渲染失败 %q: %w", truncateForError(tmpl), err)
	}
	return sb.String(), nil
}

// compile 解析模板并缓存。
func (r *Renderer) compile(tmpl string) (*template.Template, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if t, ok := r.cache[tmpl]; ok {
		return t, nil
	}
	// Option "missingkey=zero"：缺失的键渲染为零值而非报错。
	// B 站字段时有时无，模板不该因此失败。
	t, err := template.New("action").
		Option("missingkey=zero").
		Funcs(r.funcMap()).
		Parse(tmpl)
	if err != nil {
		return nil, fmt.Errorf("rules: 模板解析失败 %q: %w", truncateForError(tmpl), err)
	}
	r.cache[tmpl] = t
	return t, nil
}

// intn 返回 [0,n) 的随机数，线程安全。
func (r *Renderer) intn(n int) int {
	if r.rnd == nil {
		return rand.Intn(n)
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.rnd.Intn(n)
}

// funcMap 返回模板可用的内置函数。
func (r *Renderer) funcMap() template.FuncMap {
	return template.FuncMap{
		"join":         tmplJoin,
		"simplifyName": SimplifyName,
		"truncate":     tmplTruncate,
		"pick":         r.tmplPick,
		"int":          tmplInt,
	}
}

// tmplJoin 拼接数组，兼容 []string 与 []any。
func tmplJoin(v any, sep string) string {
	switch t := v.(type) {
	case []string:
		return strings.Join(t, sep)
	case []any:
		parts := make([]string, 0, len(t))
		for _, item := range t {
			parts = append(parts, toString(item))
		}
		return strings.Join(parts, sep)
	case nil:
		return ""
	default:
		return toString(v)
	}
}

// tmplTruncate 按字符（而非字节）截断，避免把中文切坏。
func tmplTruncate(v any, n int) string {
	s := toString(v)
	runes := []rune(s)
	if n < 0 || len(runes) <= n {
		return s
	}
	return string(runes[:n])
}

// tmplInt 把值转成 int，供模板里的数值比较使用。
// text/template 的 gt/lt 要求类型一致，而 Vars 中的数值类型不统一。
func tmplInt(v any) int {
	f, _ := toFloat(v)
	return int(f)
}

// tmplPick 从参数中随机取一个。
func (r *Renderer) tmplPick(items ...string) string {
	if len(items) == 0 {
		return ""
	}
	return items[r.intn(len(items))]
}

// nameDecorations 是昵称中常见的装饰性前后缀。
var nameDecorations = []string{
	"_official", "-official", "官方", "【官方】",
	"·-·", "-·-", "、", "丶",
}

// SimplifyName 去除昵称中常见的装饰性前后缀，让答谢弹幕更自然。
//
// 只做保守的前后缀剥离，不动昵称中间的内容——把「某某某-许许的蓷」
// 截成「某某某」会认错人。
func SimplifyName(v any) string {
	s := strings.TrimSpace(toString(v))
	if s == "" {
		return ""
	}

	changed := true
	for changed {
		changed = false
		for _, d := range nameDecorations {
			if len(s) > len(d) && strings.HasPrefix(s, d) {
				s = strings.TrimPrefix(s, d)
				changed = true
			}
			if len(s) > len(d) && strings.HasSuffix(s, d) {
				s = strings.TrimSuffix(s, d)
				changed = true
			}
		}
		s = strings.TrimSpace(s)
	}
	return s
}

// truncateForError 截断过长模板，避免错误信息刷屏。
func truncateForError(s string) string {
	r := []rune(s)
	if len(r) <= 40 {
		return s
	}
	return string(r[:40]) + "..."
}
```

- [ ] **Step 4: 运行测试确认通过**

```bash
cd server && go test ./internal/rules/ -v
```

Expected: 全部 PASS。

> 若 `TestRenderSimplifyName` 的某个用例不符预期，**以测试中列出的期望
> 为准调整 `nameDecorations`**，不要反过来改测试放宽要求——「不动昵称
> 中间内容」是这个函数的核心约束。

- [ ] **Step 5: 提交**

```bash
cd server && go vet ./... && gofmt -l .
git add server/internal/rules/
git commit -m "feat: 实现动作模板渲染与内置函数"
```

---

### Task 5: goja 沙箱

**Files:**
- Create: `server/internal/rules/script.go`
- Test: `server/internal/rules/script_test.go`
- Modify: `server/go.mod`（新增 `github.com/dop251/goja`）

**Interfaces:**
- Consumes: Task 3 的 `ScriptRunner` 接口
- Produces:
  - `rules.Sandbox` 实现 `ScriptRunner`
  - `rules.NewSandbox(opts SandboxOptions) *Sandbox`
  - `rules.SandboxOptions{Timeout time.Duration, Bot BotAPI, Storage Storage, Logger *slog.Logger}`
  - `rules.BotAPI` 接口：`SendDanmaku(text string) error`、`Block(uid string, hours int) error`
  - `rules.Storage` 接口：`Get(key string) (string, bool)`、`Set(key, value string)`
  - `(*Sandbox).EvalBool(code string, vars map[string]any) (bool, error)`
  - `(*Sandbox).RunAction(code string, vars map[string]any) error`
  - `rules.ErrScriptTimeout`

**安全约束（必须由测试守住）：** goja 默认不提供 `require`、文件系统、
`process`、网络——已实测确认。能力全靠注入，**不得注入网络访问**。

- [ ] **Step 1: 写失败测试**

创建 `server/internal/rules/script_test.go`：

```go
package rules

import (
	"errors"
	"strings"
	"sync"
	"testing"
	"time"
)

// fakeBot 是 BotAPI 的测试替身。
type fakeBot struct {
	mu       sync.Mutex
	danmakus []string
	blocks   []string
}

func (f *fakeBot) SendDanmaku(text string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.danmakus = append(f.danmakus, text)
	return nil
}

func (f *fakeBot) Block(uid string, hours int) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.blocks = append(f.blocks, uid)
	return nil
}

// memStorage 是 Storage 的内存实现，测试用。
type memStorage struct {
	mu sync.Mutex
	m  map[string]string
}

func newMemStorage() *memStorage { return &memStorage{m: map[string]string{}} }

func (s *memStorage) Get(k string) (string, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	v, ok := s.m[k]
	return v, ok
}

func (s *memStorage) Set(k, v string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.m[k] = v
}

func newTestSandbox(bot BotAPI, st Storage) *Sandbox {
	return NewSandbox(SandboxOptions{Timeout: 200 * time.Millisecond, Bot: bot, Storage: st})
}

func TestSandboxEvalBool(t *testing.T) {
	sb := newTestSandbox(nil, nil)
	vars := map[string]any{
		"text": "点歌 晴天",
		"user": map[string]any{"guardLevel": 3, "username": "甲"},
	}

	cases := []struct {
		code string
		want bool
	}{
		{`event.user.guardLevel > 0`, true},
		{`event.user.guardLevel > 5`, false},
		{`event.text.indexOf("点歌") === 0`, true},
		{`event.text.length > 100`, false},
		{`event.user.username === "甲"`, true},
	}
	for _, tc := range cases {
		got, err := sb.EvalBool(tc.code, vars)
		if err != nil {
			t.Errorf("%s: 求值失败 %v", tc.code, err)
			continue
		}
		if got != tc.want {
			t.Errorf("%s = %v, 期望 %v", tc.code, got, tc.want)
		}
	}
}

func TestSandboxEvalBoolNonBooleanIsTruthy(t *testing.T) {
	sb := newTestSandbox(nil, nil)
	// JS 的真值语义：非空字符串为真，0 为假
	got, err := sb.EvalBool(`"非空"`, map[string]any{})
	if err != nil {
		t.Fatalf("求值失败: %v", err)
	}
	if !got {
		t.Error("非空字符串应为真")
	}
	got, _ = sb.EvalBool(`0`, map[string]any{})
	if got {
		t.Error("0 应为假")
	}
}

func TestSandboxTimeoutInterruptsInfiniteLoop(t *testing.T) {
	sb := NewSandbox(SandboxOptions{Timeout: 50 * time.Millisecond})

	start := time.Now()
	_, err := sb.EvalBool(`while(true){}`, map[string]any{})
	elapsed := time.Since(start)

	if !errors.Is(err, ErrScriptTimeout) {
		t.Errorf("err = %v, 期望 ErrScriptTimeout", err)
	}
	if elapsed > 2*time.Second {
		t.Errorf("死循环应在超时后被打断，实际耗时 %v", elapsed)
	}
}

func TestSandboxHasNoFileSystemAccess(t *testing.T) {
	sb := newTestSandbox(nil, nil)
	// 这些全局对象一律不得存在——沙箱安全的核心保证
	for _, name := range []string{"require", "process", "fs", "child_process", "fetch", "XMLHttpRequest", "eval_file"} {
		code := `typeof ` + name + ` === "undefined"`
		got, err := sb.EvalBool(code, map[string]any{})
		if err != nil {
			t.Errorf("%s: 求值失败 %v", name, err)
			continue
		}
		if !got {
			t.Errorf("全局对象 %q 不该存在于沙箱中", name)
		}
	}
}

func TestSandboxBotAPI(t *testing.T) {
	bot := &fakeBot{}
	sb := newTestSandbox(bot, nil)

	err := sb.RunAction(`bot.sendDanmaku("你好"); bot.block("123", 2)`, map[string]any{})
	if err != nil {
		t.Fatalf("RunAction 失败: %v", err)
	}
	if len(bot.danmakus) != 1 || bot.danmakus[0] != "你好" {
		t.Errorf("danmakus = %v", bot.danmakus)
	}
	if len(bot.blocks) != 1 || bot.blocks[0] != "123" {
		t.Errorf("blocks = %v", bot.blocks)
	}
}

func TestSandboxStorage(t *testing.T) {
	st := newMemStorage()
	sb := newTestSandbox(nil, st)

	if err := sb.RunAction(`storage.set("计数", "1")`, map[string]any{}); err != nil {
		t.Fatalf("RunAction 失败: %v", err)
	}
	if v, ok := st.Get("计数"); !ok || v != "1" {
		t.Errorf("storage 未写入: %v %v", v, ok)
	}

	got, err := sb.EvalBool(`storage.get("计数") === "1"`, map[string]any{})
	if err != nil {
		t.Fatalf("求值失败: %v", err)
	}
	if !got {
		t.Error("storage.get 未取到写入的值")
	}
}

func TestSandboxStorageMissingKeyReturnsEmpty(t *testing.T) {
	sb := newTestSandbox(nil, newMemStorage())
	got, err := sb.EvalBool(`storage.get("从未写过") === ""`, map[string]any{})
	if err != nil {
		t.Fatalf("求值失败: %v", err)
	}
	if !got {
		t.Error("未写过的键应返回空串而非抛异常")
	}
}

func TestSandboxSyntaxErrorReported(t *testing.T) {
	sb := newTestSandbox(nil, nil)
	_, err := sb.EvalBool(`这不是合法的 JS ((( `, map[string]any{})
	if err == nil {
		t.Fatal("语法错误应当报错")
	}
	if !strings.Contains(err.Error(), "脚本") {
		t.Errorf("错误信息应提及脚本，实际 %v", err)
	}
}

func TestSandboxRuntimeErrorReported(t *testing.T) {
	sb := newTestSandbox(nil, nil)
	_, err := sb.EvalBool(`null.foo`, map[string]any{})
	if err == nil {
		t.Error("运行时异常应当报错")
	}
}

func TestSandboxConcurrentUse(t *testing.T) {
	// goja.Runtime 非并发安全，Sandbox 必须自行隔离
	sb := newTestSandbox(nil, nil)
	var wg sync.WaitGroup
	errs := make(chan error, 50)

	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			vars := map[string]any{"n": n}
			ok, err := sb.EvalBool(`event.n >= 0`, vars)
			if err != nil {
				errs <- err
				return
			}
			if !ok {
				errs <- errors.New("结果错误")
			}
		}(i)
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		t.Errorf("并发执行出错: %v", err)
	}
}

func TestSandboxVarsIsolatedBetweenRuns(t *testing.T) {
	sb := newTestSandbox(nil, nil)
	// 前一次执行污染的全局变量不得泄漏到下一次
	if err := sb.RunAction(`globalThis.污染 = "脏数据"`, map[string]any{}); err != nil {
		t.Fatalf("RunAction 失败: %v", err)
	}
	got, err := sb.EvalBool(`typeof 污染 === "undefined"`, map[string]any{})
	if err != nil {
		t.Fatalf("求值失败: %v", err)
	}
	if !got {
		t.Error("上一次执行的全局变量泄漏到了下一次")
	}
}
```

- [ ] **Step 2: 运行测试确认失败**

```bash
cd server && go get github.com/dop251/goja@latest && go test ./internal/rules/ -run TestSandbox -v
```

Expected: 编译失败，`undefined: NewSandbox`。

- [ ] **Step 3: 实现**

创建 `server/internal/rules/script.go`：

```go
package rules

import (
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/dop251/goja"
)

// ErrScriptTimeout 表示脚本执行超时被强制中断。
var ErrScriptTimeout = errors.New("rules: 脚本执行超时")

// defaultScriptTimeout 是脚本执行的默认硬超时。
const defaultScriptTimeout = 200 * time.Millisecond

// BotAPI 是注入脚本的机器人能力。
type BotAPI interface {
	SendDanmaku(text string) error
	Block(uid string, hours int) error
}

// Storage 是注入脚本的房间级键值存储。
type Storage interface {
	Get(key string) (string, bool)
	Set(key, value string)
}

// SandboxOptions 配置沙箱。
type SandboxOptions struct {
	Timeout time.Duration // 单次执行硬超时，0 表示用默认值
	Bot     BotAPI        // 可为 nil，此时脚本调用 bot.* 会抛异常
	Storage Storage       // 可为 nil，此时脚本调用 storage.* 会抛异常
	Logger  *slog.Logger
}

// Sandbox 是 goja 脚本沙箱，实现 ScriptRunner。
//
// 安全模型：goja 默认不提供 require、文件系统、process、网络，
// 这是天然白名单——不注入就没有。本沙箱刻意不注入网络访问：
// 在多用户场景下，那等同把服务器变成任意请求代理。
//
// goja.Runtime 非并发安全，故用池管理，每次执行独占一个实例。
type Sandbox struct {
	timeout time.Duration
	bot     BotAPI
	storage Storage
	log     *slog.Logger

	pool sync.Pool
}

var _ ScriptRunner = (*Sandbox)(nil)

// NewSandbox 创建沙箱。
func NewSandbox(opts SandboxOptions) *Sandbox {
	if opts.Timeout <= 0 {
		opts.Timeout = defaultScriptTimeout
	}
	if opts.Logger == nil {
		opts.Logger = slog.Default()
	}

	s := &Sandbox{
		timeout: opts.Timeout,
		bot:     opts.Bot,
		storage: opts.Storage,
		log:     opts.Logger,
	}
	s.pool.New = func() any { return s.newRuntime() }
	return s
}

// newRuntime 创建一个注入好 API 的 Runtime。
func (s *Sandbox) newRuntime() *goja.Runtime {
	vm := goja.New()
	// 让 JS 的驼峰命名映射到 Go 的导出方法
	vm.SetFieldNameMapper(goja.UncapFieldNameMapper())

	bot := vm.NewObject()
	bot.Set("sendDanmaku", func(text string) {
		if s.bot == nil {
			panic(vm.ToValue("bot 能力未启用"))
		}
		if err := s.bot.SendDanmaku(text); err != nil {
			panic(vm.ToValue("发送弹幕失败: " + err.Error()))
		}
	})
	bot.Set("block", func(uid string, hours int) {
		if s.bot == nil {
			panic(vm.ToValue("bot 能力未启用"))
		}
		if err := s.bot.Block(uid, hours); err != nil {
			panic(vm.ToValue("禁言失败: " + err.Error()))
		}
	})
	vm.Set("bot", bot)

	storage := vm.NewObject()
	storage.Set("get", func(k string) string {
		if s.storage == nil {
			panic(vm.ToValue("storage 能力未启用"))
		}
		v, _ := s.storage.Get(k) // 缺失返回空串，不抛异常
		return v
	})
	storage.Set("set", func(k, v string) {
		if s.storage == nil {
			panic(vm.ToValue("storage 能力未启用"))
		}
		s.storage.Set(k, v)
	})
	vm.Set("storage", storage)

	console := vm.NewObject()
	logFn := func(args ...any) { s.log.Info("脚本日志", "args", args) }
	console.Set("log", logFn)
	console.Set("info", logFn)
	console.Set("warn", func(args ...any) { s.log.Warn("脚本日志", "args", args) })
	console.Set("error", func(args ...any) { s.log.Error("脚本日志", "args", args) })
	vm.Set("console", console)

	return vm
}

// EvalBool 求值一段 JS 表达式，按 JS 真值语义返回布尔结果。
func (s *Sandbox) EvalBool(code string, vars map[string]any) (bool, error) {
	v, err := s.run(code, vars)
	if err != nil {
		return false, err
	}
	return v.ToBoolean(), nil
}

// RunAction 执行一段 JS 语句，忽略返回值。
func (s *Sandbox) RunAction(code string, vars map[string]any) error {
	_, err := s.run(code, vars)
	return err
}

// run 在受控环境中执行脚本。
func (s *Sandbox) run(code string, vars map[string]any) (goja.Value, error) {
	vm := s.pool.Get().(*goja.Runtime)
	defer func() {
		s.cleanup(vm)
		s.pool.Put(vm)
	}()

	if vars == nil {
		vars = map[string]any{}
	}
	vm.Set("event", vars)

	// 超时守卫：到期强制中断，防死循环拖垮房间。
	timer := time.AfterFunc(s.timeout, func() {
		vm.Interrupt(ErrScriptTimeout)
	})
	defer timer.Stop()

	v, err := vm.RunString(code)
	// 无论成功与否都要清除中断标志，否则该 Runtime 会永久不可用。
	vm.ClearInterrupt()

	if err != nil {
		var ie *goja.InterruptedError
		if errors.As(err, &ie) {
			return nil, ErrScriptTimeout
		}
		return nil, fmt.Errorf("rules: 脚本执行失败: %w", err)
	}
	return v, nil
}

// cleanup 清除本次执行注入的变量与脚本产生的全局污染。
//
// Runtime 会被复用，必须保证上一次执行的全局变量不泄漏到下一次。
func (s *Sandbox) cleanup(vm *goja.Runtime) {
	vm.Set("event", goja.Undefined())

	// 删除脚本自行创建的全局变量，保留注入的 API。
	protected := map[string]bool{
		"bot": true, "storage": true, "console": true, "event": true,
		"globalThis": true, "undefined": true, "NaN": true, "Infinity": true,
	}
	global := vm.GlobalObject()
	for _, k := range global.Keys() {
		if !protected[k] {
			// 内置构造器（Object、Array 等）不可删除，Delete 会静默失败，无害。
			global.Delete(k)
		}
	}
}
```

- [ ] **Step 4: 运行测试确认通过**

```bash
cd server && go mod tidy && go test ./internal/rules/ -v
```

Expected: 全部 PASS，含并发与超时用例。

- [ ] **Step 5: 竞态检测**

沙箱涉及 `sync.Pool` 与并发执行，必须跑竞态：

```bash
cd server && CGO_ENABLED=1 go test ./internal/rules/ -race -count=3
```

Expected: PASS，无 DATA RACE。

> Windows 上需先把 MinGW 加进 PATH：
> `C:\Users\ZIQI\AppData\Local\Microsoft\WinGet\Packages\BrechtSanders.WinLibs.POSIX.UCRT_Microsoft.Winget.Source_8wekyb3d8bbwe\mingw64\bin`

- [ ] **Step 6: 交叉编译复检**

新增依赖不得破坏交叉编译：

```bash
cd server
for t in linux/amd64 linux/arm64 darwin/arm64 windows/amd64; do
  GOOS=${t%/*} GOARCH=${t#*/} CGO_ENABLED=0 go build -o /dev/null ./cmd/magicd && echo "OK $t"
done
```

Expected: 四个目标全部 OK。

- [ ] **Step 7: 提交**

```bash
cd server && go vet ./... && gofmt -l .
git add server/
git commit -m "feat: 实现 goja 脚本沙箱"
```

---

**下一步：** 继续阅读 `2026-07-31-p2-rule-engine-part2.md`，实现冷却、
合并窗口、规则匹配与动作执行。
