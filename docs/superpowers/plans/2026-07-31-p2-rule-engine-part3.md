# P2 规则引擎 Implementation Plan · Part 3（定时、配置、组装、CLI）

> 续 `2026-07-31-p2-rule-engine-part2.md`。执行前请先完成 Task 1–10。
> Global Constraints 沿用 Part 1，此处不再重复。

本篇覆盖 Task 11–14，完成 P2 的最终交付：`magicd run -c config.yaml`。

---

### Task 11: 定时任务

**Files:**
- Create: `server/internal/scheduler/cron.go`
- Test: `server/internal/scheduler/cron_test.go`
- Modify: `server/go.mod`（新增 `github.com/robfig/cron/v3`）

**Interfaces:**
- Consumes: 无
- Produces:
  - `scheduler.Scheduler` 结构，`scheduler.New(log *slog.Logger) *Scheduler`
  - `(*Scheduler).Add(spec, name string, fn func()) error`
  - `(*Scheduler).Start()`、`(*Scheduler).Stop()`
  - `scheduler.ValidateSpec(spec string) error` — 供配置加载时预校验

**采用 6 字段表达式（含秒）**：标准 5 字段 cron 最细只能到分钟，
表达不了「每 30 秒」这类常见需求。

- [ ] **Step 1: 写失败测试**

创建 `server/internal/scheduler/cron_test.go`：

```go
package scheduler

import (
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestValidateSpecAcceptsSixFields(t *testing.T) {
	// 6 字段含秒：秒 分 时 日 月 周
	for _, spec := range []string{
		"*/1 * * * * *",
		"0 */5 * * * *",
		"30 0 12 * * *",
		"0 0 0 1 1 *",
	} {
		if err := ValidateSpec(spec); err != nil {
			t.Errorf("%q 应为合法表达式: %v", spec, err)
		}
	}
}

func TestValidateSpecRejectsInvalid(t *testing.T) {
	for _, spec := range []string{
		"",
		"不是表达式",
		"* * *",
		"99 * * * * *",
	} {
		if err := ValidateSpec(spec); err == nil {
			t.Errorf("%q 应被拒绝", spec)
		}
	}
}

func TestSchedulerRunsJob(t *testing.T) {
	s := New(nil)
	var n int32
	if err := s.Add("*/1 * * * * *", "每秒任务", func() {
		atomic.AddInt32(&n, 1)
	}); err != nil {
		t.Fatalf("Add 失败: %v", err)
	}

	s.Start()
	time.Sleep(2500 * time.Millisecond)
	s.Stop()

	if got := atomic.LoadInt32(&n); got < 2 {
		t.Errorf("2.5 秒内应至少触发 2 次，实际 %d", got)
	}
}

func TestSchedulerAddRejectsBadSpec(t *testing.T) {
	s := New(nil)
	if err := s.Add("坏表达式", "任务", func() {}); err == nil {
		t.Error("非法表达式应当报错")
	}
}

func TestSchedulerStopHaltsJobs(t *testing.T) {
	s := New(nil)
	var n int32
	s.Add("*/1 * * * * *", "任务", func() { atomic.AddInt32(&n, 1) })

	s.Start()
	time.Sleep(1200 * time.Millisecond)
	s.Stop()

	after := atomic.LoadInt32(&n)
	time.Sleep(1500 * time.Millisecond)
	if got := atomic.LoadInt32(&n); got != after {
		t.Errorf("Stop 后不应再触发，%d → %d", after, got)
	}
}

func TestSchedulerPanicInJobDoesNotCrash(t *testing.T) {
	s := New(nil)
	var ok int32
	// 第一个任务 panic，不得影响第二个
	s.Add("*/1 * * * * *", "会 panic 的任务", func() { panic("故意的") })
	s.Add("*/1 * * * * *", "正常任务", func() { atomic.AddInt32(&ok, 1) })

	s.Start()
	time.Sleep(2500 * time.Millisecond)
	s.Stop()

	if atomic.LoadInt32(&ok) < 2 {
		t.Errorf("panic 的任务不应拖垮其他任务，正常任务只跑了 %d 次", ok)
	}
}

func TestSchedulerConcurrentAdd(t *testing.T) {
	s := New(nil)
	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			s.Add("0 0 0 1 1 *", "任务", func() {})
		}()
	}
	wg.Wait()
}
```

- [ ] **Step 2: 运行测试确认失败**

```bash
cd server && go get github.com/robfig/cron/v3@latest && go test ./internal/scheduler/ -v
```

Expected: 编译失败，`undefined: New`。

- [ ] **Step 3: 实现**

创建 `server/internal/scheduler/cron.go`：

```go
// Package scheduler 提供基于 cron 表达式的定时任务调度。
package scheduler

import (
	"fmt"
	"log/slog"

	"github.com/robfig/cron/v3"
)

// parser 解析 6 字段表达式：秒 分 时 日 月 周。
//
// 标准 5 字段 cron 最细只到分钟，表达不了「每 30 秒」这类常见需求，
// 因此启用秒级字段。
var parser = cron.NewParser(
	cron.Second | cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow | cron.Descriptor,
)

// ValidateSpec 校验 cron 表达式，供配置加载时预检。
func ValidateSpec(spec string) error {
	if spec == "" {
		return fmt.Errorf("scheduler: cron 表达式不能为空")
	}
	if _, err := parser.Parse(spec); err != nil {
		return fmt.Errorf("scheduler: 非法的 cron 表达式 %q: %w", spec, err)
	}
	return nil
}

// Scheduler 管理一组定时任务。
type Scheduler struct {
	cron *cron.Cron
	log  *slog.Logger
}

// New 创建调度器。
func New(log *slog.Logger) *Scheduler {
	if log == nil {
		log = slog.Default()
	}
	return &Scheduler{
		cron: cron.New(cron.WithParser(parser)),
		log:  log,
	}
}

// Add 注册一个定时任务。
//
// 任务内的 panic 会被捕获并记录，不得拖垮调度器或其他任务——
// 用户脚本的错误不该让整个机器人停摆。
func (s *Scheduler) Add(spec, name string, fn func()) error {
	if err := ValidateSpec(spec); err != nil {
		return err
	}

	wrapped := func() {
		defer func() {
			if r := recover(); r != nil {
				s.log.Error("定时任务发生 panic", "task", name, "panic", r)
			}
		}()
		fn()
	}

	if _, err := s.cron.AddFunc(spec, wrapped); err != nil {
		return fmt.Errorf("scheduler: 注册任务 %q 失败: %w", name, err)
	}
	s.log.Info("已注册定时任务", "task", name, "schedule", spec)
	return nil
}

// Start 开始调度，非阻塞。
func (s *Scheduler) Start() { s.cron.Start() }

// Stop 停止调度并等待运行中的任务结束。
func (s *Scheduler) Stop() {
	ctx := s.cron.Stop()
	<-ctx.Done()
}
```

- [ ] **Step 4: 运行测试确认通过**

```bash
cd server && go mod tidy && go test ./internal/scheduler/ -v
```

Expected: 全部 PASS。

- [ ] **Step 5: 提交**

```bash
cd server && go vet ./... && gofmt -l .
git add server/
git commit -m "feat: 实现 cron 定时任务调度"
```

---

### Task 12: YAML 配置加载与校验

**Files:**
- Create: `server/internal/rules/config/config.go`
- Test: `server/internal/rules/config/config_test.go`
- Modify: `server/go.mod`（新增 `gopkg.in/yaml.v3`）

**Interfaces:**
- Consumes: Task 1 的 `rules.Rule` 等模型；Task 11 的 `scheduler.ValidateSpec`
- Produces:
  - `config.Config{Accounts []Account, Rooms []Room, RateLimit RateLimit, CooldownGroups map[string]Duration, Rules []rules.Rule}`
  - `config.Account{Name, CookieFile string}`
  - `config.Room{ID string, Accounts []string}`
  - `config.RateLimit{Interval Duration}`
  - `config.Duration`（包装 `time.Duration`，支持 `"1.5s"` 形式）
  - `config.Load(path string) (*Config, error)`
  - `config.Parse(data []byte) (*Config, error)`

**操作符别名：** 设计文档第 11 节的示例写了 `op: ">"`，而模型定义的是
`gt`。配置加载时做别名归一化，两种写法都接受——用户不该被迫记住内部枚举名。

- [ ] **Step 1: 写失败测试**

创建 `server/internal/rules/config/config_test.go`：

```go
package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/MrZhongzq/MagicalDanmaku/server/internal/event"
	"github.com/MrZhongzq/MagicalDanmaku/server/internal/rules"
)

const fullYAML = `
accounts:
  - name: main
    cookieFile: cookie.txt
  - name: sub1
    cookieFile: cookie-sub1.txt

rooms:
  - id: "1706666491"
    accounts: [main, sub1]

rateLimit:
  interval: 1.5s

cooldownGroups:
  greeting: 5s
  thanks: 2s

rules:
  - name: 舰长进场欢迎
    enabled: true
    on: [user_enter]
    when:
      field: user.guardLevel
      op: ">"
      value: 0
    aggregate:
      window: 2s
      by: type
    cooldownGroup: greeting
    cooldown: 3s
    do:
      - type: danmaku
        template:
          - "欢迎 {{join .users \"、\"}} 回家~"
          - "{{join .users \"、\"}} 来啦！"

  - name: 关键词禁言
    enabled: true
    on: [danmaku]
    when:
      any:
        - field: text
          op: regex
          value: "(广告|加群)"
        - field: text
          op: contains
          value: "违禁词"
    do:
      - type: block
        hours: 1

  - name: 定时广告
    enabled: true
    schedule: "0 */5 * * * *"
    do:
      - type: danmaku
        template: ["关注主播不迷路~"]
`

func TestParseFullConfig(t *testing.T) {
	c, err := Parse([]byte(fullYAML))
	if err != nil {
		t.Fatalf("Parse 失败: %v", err)
	}

	if len(c.Accounts) != 2 || c.Accounts[0].Name != "main" {
		t.Errorf("Accounts = %+v", c.Accounts)
	}
	if c.Accounts[0].CookieFile != "cookie.txt" {
		t.Errorf("CookieFile = %q", c.Accounts[0].CookieFile)
	}
	if len(c.Rooms) != 1 || c.Rooms[0].ID != "1706666491" {
		t.Errorf("Rooms = %+v", c.Rooms)
	}
	if len(c.Rooms[0].Accounts) != 2 {
		t.Errorf("Room.Accounts = %v", c.Rooms[0].Accounts)
	}
	if time.Duration(c.RateLimit.Interval) != 1500*time.Millisecond {
		t.Errorf("RateLimit.Interval = %v", time.Duration(c.RateLimit.Interval))
	}
	if time.Duration(c.CooldownGroups["greeting"]) != 5*time.Second {
		t.Errorf("greeting = %v", time.Duration(c.CooldownGroups["greeting"]))
	}
	if len(c.Rules) != 3 {
		t.Fatalf("Rules 数 = %d, 期望 3", len(c.Rules))
	}
}

func TestParseRuleFields(t *testing.T) {
	c, err := Parse([]byte(fullYAML))
	if err != nil {
		t.Fatalf("Parse 失败: %v", err)
	}
	r := c.Rules[0]

	if r.Name != "舰长进场欢迎" {
		t.Errorf("Name = %q", r.Name)
	}
	if !r.Enabled {
		t.Error("Enabled 应为 true")
	}
	if len(r.On) != 1 || r.On[0] != event.TypeUserEnter {
		t.Errorf("On = %v", r.On)
	}
	if r.Cooldown != 3*time.Second {
		t.Errorf("Cooldown = %v", r.Cooldown)
	}
	if r.CooldownGroup != "greeting" {
		t.Errorf("CooldownGroup = %q", r.CooldownGroup)
	}
	if r.Aggregate == nil {
		t.Fatal("Aggregate 不应为 nil")
	}
	if r.Aggregate.Window != 2*time.Second || r.Aggregate.By != rules.AggregateByType {
		t.Errorf("Aggregate = %+v", *r.Aggregate)
	}
	if len(r.Do) != 1 || r.Do[0].Type != rules.ActionDanmaku {
		t.Errorf("Do = %+v", r.Do)
	}
	if len(r.Do[0].Template) != 2 {
		t.Errorf("Template 数 = %d, 期望 2", len(r.Do[0].Template))
	}
}

func TestParseNormalizesOperatorAliases(t *testing.T) {
	c, err := Parse([]byte(fullYAML))
	if err != nil {
		t.Fatalf("Parse 失败: %v", err)
	}
	// 配置里写的是 ">"，应被归一化为 "gt"
	if c.Rules[0].When.Op != "gt" {
		t.Errorf("Op = %q, 期望归一化为 gt", c.Rules[0].When.Op)
	}
}

func TestParseAllOperatorAliases(t *testing.T) {
	cases := map[string]string{
		">": "gt", ">=": "gte", "<": "lt", "<=": "lte",
		"==": "eq", "=": "eq", "!=": "ne", "<>": "ne",
		"gt": "gt", "contains": "contains",
	}
	for alias, want := range cases {
		y := `
rules:
  - name: 测试
    on: [danmaku]
    when: {field: text, op: "` + alias + `", value: "x"}
    do: [{type: log}]
`
		c, err := Parse([]byte(y))
		if err != nil {
			t.Errorf("别名 %q 解析失败: %v", alias, err)
			continue
		}
		if got := c.Rules[0].When.Op; got != want {
			t.Errorf("别名 %q 归一化为 %q, 期望 %q", alias, got, want)
		}
	}
}

func TestParseNestedCondition(t *testing.T) {
	c, err := Parse([]byte(fullYAML))
	if err != nil {
		t.Fatalf("Parse 失败: %v", err)
	}
	w := c.Rules[1].When
	if w == nil || len(w.Any) != 2 {
		t.Fatalf("嵌套条件解析错误: %+v", w)
	}
	if w.Any[0].Op != "regex" || w.Any[1].Op != "contains" {
		t.Errorf("子条件 = %+v", w.Any)
	}
}

func TestParseScheduledRule(t *testing.T) {
	c, err := Parse([]byte(fullYAML))
	if err != nil {
		t.Fatalf("Parse 失败: %v", err)
	}
	r := c.Rules[2]
	if r.Schedule != "0 */5 * * * *" {
		t.Errorf("Schedule = %q", r.Schedule)
	}
	if len(r.On) != 0 {
		t.Errorf("定时规则不应有 On，实际 %v", r.On)
	}
}

func TestParseEnabledDefaultsToTrue(t *testing.T) {
	y := `
rules:
  - name: 未写 enabled
    on: [danmaku]
    do: [{type: log}]
`
	c, err := Parse([]byte(y))
	if err != nil {
		t.Fatalf("Parse 失败: %v", err)
	}
	if !c.Rules[0].Enabled {
		t.Error("未写 enabled 时应默认启用——写了规则却不生效最反直觉")
	}
}

func TestParseExplicitDisable(t *testing.T) {
	y := `
rules:
  - name: 显式禁用
    enabled: false
    on: [danmaku]
    do: [{type: log}]
`
	c, err := Parse([]byte(y))
	if err != nil {
		t.Fatalf("Parse 失败: %v", err)
	}
	if c.Rules[0].Enabled {
		t.Error("显式写 false 应被尊重")
	}
}

func TestParseRejectsUnknownEventType(t *testing.T) {
	y := `
rules:
  - name: 坏事件类型
    on: [不存在的事件]
    do: [{type: log}]
`
	_, err := Parse([]byte(y))
	if err == nil {
		t.Fatal("未知事件类型应当报错")
	}
	if !strings.Contains(err.Error(), "事件类型") {
		t.Errorf("错误信息应指出问题所在，实际: %v", err)
	}
}

func TestParseRejectsInvalidRule(t *testing.T) {
	cases := map[string]string{
		"既无 on 也无 schedule": `
rules:
  - name: 无触发
    do: [{type: log}]
`,
		"on 与 schedule 并存": `
rules:
  - name: 双触发
    on: [danmaku]
    schedule: "0 * * * * *"
    do: [{type: log}]
`,
		"动作列表为空": `
rules:
  - name: 无动作
    on: [danmaku]
`,
		"未知操作符": `
rules:
  - name: 坏操作符
    on: [danmaku]
    when: {field: text, op: 不存在, value: x}
    do: [{type: log}]
`,
		"danmaku 缺模板": `
rules:
  - name: 缺模板
    on: [danmaku]
    do: [{type: danmaku}]
`,
		"非法 cron": `
rules:
  - name: 坏 cron
    schedule: 不是表达式
    do: [{type: log}]
`,
		"非法正则": `
rules:
  - name: 坏正则
    on: [danmaku]
    when: {field: text, op: regex, value: "([("}
    do: [{type: log}]
`,
	}
	for name, y := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := Parse([]byte(y)); err == nil {
				t.Error("应当报错")
			}
		})
	}
}

func TestParseRejectsRoomWithUnknownAccount(t *testing.T) {
	y := `
accounts:
  - name: main
    cookieFile: cookie.txt
rooms:
  - id: "1"
    accounts: [不存在的账号]
rules:
  - name: 规则
    on: [danmaku]
    do: [{type: log}]
`
	_, err := Parse([]byte(y))
	if err == nil {
		t.Fatal("引用不存在的账号应当报错")
	}
	if !strings.Contains(err.Error(), "账号") {
		t.Errorf("错误信息应提及账号，实际: %v", err)
	}
}

func TestParseRejectsDuplicateRuleNames(t *testing.T) {
	y := `
rules:
  - name: 重名
    on: [danmaku]
    do: [{type: log}]
  - name: 重名
    on: [gift]
    do: [{type: log}]
`
	_, err := Parse([]byte(y))
	if err == nil {
		t.Fatal("规则重名应当报错——冷却按规则名记录，重名会互相干扰")
	}
}

func TestParseRejectsBadYAML(t *testing.T) {
	if _, err := Parse([]byte("这不是: [合法的 YAML")); err == nil {
		t.Error("非法 YAML 应当报错")
	}
}

func TestLoadFromFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(path, []byte(fullYAML), 0o644); err != nil {
		t.Fatalf("写入测试文件失败: %v", err)
	}

	c, err := Load(path)
	if err != nil {
		t.Fatalf("Load 失败: %v", err)
	}
	if len(c.Rules) != 3 {
		t.Errorf("Rules 数 = %d", len(c.Rules))
	}
}

func TestLoadMissingFileReportsPath(t *testing.T) {
	_, err := Load("/不存在的路径/config.yaml")
	if err == nil {
		t.Fatal("文件不存在应当报错")
	}
	if !strings.Contains(err.Error(), "config.yaml") {
		t.Errorf("错误信息应含路径，实际: %v", err)
	}
}

func TestDurationParsing(t *testing.T) {
	y := `
rateLimit:
  interval: 1.5s
cooldownGroups:
  a: 500ms
  b: 2m
rules:
  - name: 规则
    on: [danmaku]
    cooldown: 1h
    do: [{type: log}]
`
	c, err := Parse([]byte(y))
	if err != nil {
		t.Fatalf("Parse 失败: %v", err)
	}
	if time.Duration(c.RateLimit.Interval) != 1500*time.Millisecond {
		t.Errorf("interval = %v", time.Duration(c.RateLimit.Interval))
	}
	if time.Duration(c.CooldownGroups["a"]) != 500*time.Millisecond {
		t.Errorf("a = %v", time.Duration(c.CooldownGroups["a"]))
	}
	if time.Duration(c.CooldownGroups["b"]) != 2*time.Minute {
		t.Errorf("b = %v", time.Duration(c.CooldownGroups["b"]))
	}
	if c.Rules[0].Cooldown != time.Hour {
		t.Errorf("cooldown = %v", c.Rules[0].Cooldown)
	}
}

func TestDurationRejectsBadFormat(t *testing.T) {
	y := `
rateLimit:
  interval: 不是时长
rules:
  - name: 规则
    on: [danmaku]
    do: [{type: log}]
`
	if _, err := Parse([]byte(y)); err == nil {
		t.Error("非法时长格式应当报错")
	}
}
```

- [ ] **Step 2: 运行测试确认失败**

```bash
cd server && go get gopkg.in/yaml.v3@latest && go test ./internal/rules/config/ -v
```

Expected: 编译失败，`undefined: Parse`。

- [ ] **Step 3: 实现**

创建 `server/internal/rules/config/config.go`：

```go
// Package config 负责从 YAML 加载并校验规则配置。
//
// 配置格式只是一种序列化。规则模型本身存储无关，P3 迁进数据库时
// 核心逻辑不动，只换加载器。
package config

import (
	"fmt"
	"os"
	"regexp"
	"time"

	"gopkg.in/yaml.v3"

	"github.com/MrZhongzq/MagicalDanmaku/server/internal/event"
	"github.com/MrZhongzq/MagicalDanmaku/server/internal/rules"
	"github.com/MrZhongzq/MagicalDanmaku/server/internal/scheduler"
)

// Duration 包装 time.Duration，支持 YAML 中的 "1.5s" 形式。
type Duration time.Duration

// UnmarshalYAML 解析形如 "1.5s"、"500ms"、"2m" 的时长字符串。
func (d *Duration) UnmarshalYAML(n *yaml.Node) error {
	var s string
	if err := n.Decode(&s); err != nil {
		return fmt.Errorf("时长必须是字符串，如 \"1.5s\": %w", err)
	}
	parsed, err := time.ParseDuration(s)
	if err != nil {
		return fmt.Errorf("非法的时长 %q: %w", s, err)
	}
	*d = Duration(parsed)
	return nil
}

// Account 是一个可发言的账号。
type Account struct {
	Name       string `yaml:"name"`
	CookieFile string `yaml:"cookieFile"`
}

// Room 是一个要连接的直播间。
type Room struct {
	ID       string   `yaml:"id"`
	Accounts []string `yaml:"accounts"`
}

// RateLimit 是全局发送限流配置。
type RateLimit struct {
	Interval Duration `yaml:"interval"`
}

// Config 是完整配置。
type Config struct {
	Accounts       []Account           `yaml:"accounts"`
	Rooms          []Room              `yaml:"rooms"`
	RateLimit      RateLimit           `yaml:"rateLimit"`
	CooldownGroups map[string]Duration `yaml:"cooldownGroups"`
	Rules          []rules.Rule        `yaml:"-"` // 由 ruleYAML 转换而来
}

// configYAML 是 YAML 的线上格式，与领域模型分离。
type configYAML struct {
	Accounts       []Account           `yaml:"accounts"`
	Rooms          []Room              `yaml:"rooms"`
	RateLimit      RateLimit           `yaml:"rateLimit"`
	CooldownGroups map[string]Duration `yaml:"cooldownGroups"`
	Rules          []ruleYAML          `yaml:"rules"`
}

type ruleYAML struct {
	Name          string          `yaml:"name"`
	Enabled       *bool           `yaml:"enabled"` // 指针以区分「未写」与「写了 false」
	On            []string        `yaml:"on"`
	Schedule      string          `yaml:"schedule"`
	When          *conditionYAML  `yaml:"when"`
	Aggregate     *aggregateYAML  `yaml:"aggregate"`
	Cooldown      Duration        `yaml:"cooldown"`
	CooldownGroup string          `yaml:"cooldownGroup"`
	Do            []actionYAML    `yaml:"do"`
}

type conditionYAML struct {
	Field  string          `yaml:"field"`
	Op     string          `yaml:"op"`
	Value  any             `yaml:"value"`
	All    []conditionYAML `yaml:"all"`
	Any    []conditionYAML `yaml:"any"`
	Not    *conditionYAML  `yaml:"not"`
	Script string          `yaml:"script"`
}

type aggregateYAML struct {
	Window Duration `yaml:"window"`
	By     string   `yaml:"by"`
}

type actionYAML struct {
	Type     string   `yaml:"type"`
	Template []string `yaml:"template"`
	Hours    int      `yaml:"hours"`
	Script   string   `yaml:"script"`
}

// opAliases 把用户友好的符号归一化为内部枚举名。
//
// 用户不该被迫记住内部枚举名——写 ">" 比写 "gt" 自然得多。
var opAliases = map[string]string{
	">": "gt", ">=": "gte", "<": "lt", "<=": "lte",
	"==": "eq", "=": "eq", "!=": "ne", "<>": "ne",
}

// knownEventTypes 是允许出现在 on 字段中的事件类型。
var knownEventTypes = map[string]event.Type{
	string(event.TypeDanmaku):          event.TypeDanmaku,
	string(event.TypeSuperChat):        event.TypeSuperChat,
	string(event.TypeSuperChatDelete):  event.TypeSuperChatDelete,
	string(event.TypeGift):             event.TypeGift,
	string(event.TypeGiftCombo):        event.TypeGiftCombo,
	string(event.TypeGuardBuy):         event.TypeGuardBuy,
	string(event.TypeUserEnter):        event.TypeUserEnter,
	string(event.TypeUserFollow):       event.TypeUserFollow,
	string(event.TypeUserShare):        event.TypeUserShare,
	string(event.TypeUserLike):         event.TypeUserLike,
	string(event.TypeLiveStart):        event.TypeLiveStart,
	string(event.TypeLiveStop):         event.TypeLiveStop,
	string(event.TypeRoomChange):       event.TypeRoomChange,
	string(event.TypeUserBlocked):      event.TypeUserBlocked,
	string(event.TypeOnlineRankUpdate): event.TypeOnlineRankUpdate,
	string(event.TypeRoomStatsUpdate):  event.TypeRoomStatsUpdate,
	string(event.TypeBattle):           event.TypeBattle,
	string(event.TypeUnknown):          event.TypeUnknown,
}

// Load 从文件加载配置。
func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("config: 读取配置文件 %s 失败: %w", path, err)
	}
	c, err := Parse(data)
	if err != nil {
		return nil, fmt.Errorf("config: 解析 %s 失败: %w", path, err)
	}
	return c, nil
}

// Parse 解析配置内容并做完整校验。
//
// 校验失败即报错退出，不允许带病运行——配置写错却静默忽略，
// 比直接报错更难排查。
func Parse(data []byte) (*Config, error) {
	var raw configYAML
	if err := yaml.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("YAML 语法错误: %w", err)
	}

	c := &Config{
		Accounts:       raw.Accounts,
		Rooms:          raw.Rooms,
		RateLimit:      raw.RateLimit,
		CooldownGroups: raw.CooldownGroups,
	}

	// 账号名索引，供房间引用校验
	accountNames := make(map[string]bool, len(raw.Accounts))
	for i, a := range raw.Accounts {
		if a.Name == "" {
			return nil, fmt.Errorf("第 %d 个账号缺少 name", i+1)
		}
		if a.CookieFile == "" {
			return nil, fmt.Errorf("账号 %q 缺少 cookieFile", a.Name)
		}
		if accountNames[a.Name] {
			return nil, fmt.Errorf("账号名 %q 重复", a.Name)
		}
		accountNames[a.Name] = true
	}

	for i, r := range raw.Rooms {
		if r.ID == "" {
			return nil, fmt.Errorf("第 %d 个房间缺少 id", i+1)
		}
		for _, name := range r.Accounts {
			if !accountNames[name] {
				return nil, fmt.Errorf("房间 %s 引用了不存在的账号 %q", r.ID, name)
			}
		}
	}

	seenRule := make(map[string]bool, len(raw.Rules))
	for i, ry := range raw.Rules {
		r, err := convertRule(ry)
		if err != nil {
			return nil, fmt.Errorf("第 %d 条规则(%s)非法: %w", i+1, ry.Name, err)
		}
		// 冷却按规则名记录，重名会互相干扰
		if seenRule[r.Name] {
			return nil, fmt.Errorf("规则名 %q 重复", r.Name)
		}
		seenRule[r.Name] = true
		c.Rules = append(c.Rules, r)
	}
	return c, nil
}

// convertRule 把线上格式转成领域模型并校验。
func convertRule(ry ruleYAML) (rules.Rule, error) {
	r := rules.Rule{
		Name:          ry.Name,
		Enabled:       true, // 未写 enabled 时默认启用：写了规则却不生效最反直觉
		Schedule:      ry.Schedule,
		Cooldown:      time.Duration(ry.Cooldown),
		CooldownGroup: ry.CooldownGroup,
	}
	if ry.Enabled != nil {
		r.Enabled = *ry.Enabled
	}

	for _, name := range ry.On {
		t, ok := knownEventTypes[name]
		if !ok {
			return r, fmt.Errorf("未知的事件类型 %q", name)
		}
		r.On = append(r.On, t)
	}

	if ry.Schedule != "" {
		if err := scheduler.ValidateSpec(ry.Schedule); err != nil {
			return r, err
		}
	}

	if ry.When != nil {
		c, err := convertCondition(*ry.When)
		if err != nil {
			return r, err
		}
		r.When = &c
	}

	if ry.Aggregate != nil {
		r.Aggregate = &rules.AggregateSpec{
			Window: time.Duration(ry.Aggregate.Window),
			By:     rules.AggregateBy(ry.Aggregate.By),
		}
	}

	for _, a := range ry.Do {
		r.Do = append(r.Do, rules.Action{
			Type:     rules.ActionType(a.Type),
			Template: a.Template,
			Hours:    a.Hours,
			Script:   a.Script,
		})
	}

	if err := r.Validate(); err != nil {
		return r, err
	}
	return r, nil
}

// convertCondition 递归转换条件并做正则预编译校验。
func convertCondition(cy conditionYAML) (rules.Condition, error) {
	c := rules.Condition{
		Field:  cy.Field,
		Op:     normalizeOp(cy.Op),
		Value:  cy.Value,
		Script: cy.Script,
	}

	// 正则在加载时就编译一次，把错误从运行时提前到启动时
	if c.Op == "regex" {
		pattern, _ := cy.Value.(string)
		if _, err := regexp.Compile(pattern); err != nil {
			return c, fmt.Errorf("非法的正则表达式 %q: %w", pattern, err)
		}
	}

	for _, sub := range cy.All {
		s, err := convertCondition(sub)
		if err != nil {
			return c, err
		}
		c.All = append(c.All, s)
	}
	for _, sub := range cy.Any {
		s, err := convertCondition(sub)
		if err != nil {
			return c, err
		}
		c.Any = append(c.Any, s)
	}
	if cy.Not != nil {
		s, err := convertCondition(*cy.Not)
		if err != nil {
			return c, err
		}
		c.Not = &s
	}
	return c, nil
}

// normalizeOp 把符号别名归一化为内部枚举名。
func normalizeOp(op string) string {
	if alias, ok := opAliases[op]; ok {
		return alias
	}
	return op
}
```

- [ ] **Step 4: 运行测试确认通过**

```bash
cd server && go mod tidy && go test ./internal/rules/config/ -v
```

Expected: 全部 PASS。

- [ ] **Step 5: 提交**

```bash
cd server && go vet ./... && gofmt -l .
git add server/
git commit -m "feat: 实现 YAML 配置加载与启动时校验"
```

---

### Task 13: Pipeline 组装

**Files:**
- Create: `server/internal/rules/storage.go`
- Create: `server/internal/rules/engine.go`
- Test: `server/internal/rules/engine_test.go`

**Interfaces:**
- Consumes: Task 1–10 全部产出
- Produces:
  - `rules.MemStorage` 实现 `Storage`，`rules.NewMemStorage() *MemStorage`
  - `rules.Engine` 结构，`rules.NewEngine(opts EngineOptions) (*Engine, error)`
  - `rules.EngineOptions{RoomID string, Rules []Rule, Bot BotAPI, GlobalLimiter ratelimit.Limiter, CooldownGroups map[string]time.Duration, ScriptTimeout time.Duration, Logger *slog.Logger}`
  - `(*Engine).Handle(ev event.Event)` — 投入单个事件
  - `(*Engine).FireScheduled(name string)` — 触发定时规则
  - `(*Engine).ScheduledRules() []Rule`
  - `(*Engine).Close()`

**条件求值时机（重要约定）：** 条件在**合并之前**按单个事件求值。

理由：规则「只欢迎舰长」若先合并再判断，合并后的 Vars 只保留第一个用户
的等级，会把非舰长也欢迎进去。先过滤再合并，`count` 与 `users` 才只反映
真正命中的用户。

代价：条件中无法引用 `count`、`users` 这类聚合属性。这是有意的取舍。

- [ ] **Step 1: 写失败测试**

创建 `server/internal/rules/engine_test.go`：

```go
package rules

import (
	"sync"
	"testing"
	"time"

	"github.com/MrZhongzq/MagicalDanmaku/server/internal/event"
	"github.com/MrZhongzq/MagicalDanmaku/server/internal/ratelimit"
)

// recordBot 记录所有发出的动作。
type recordBot struct {
	mu       sync.Mutex
	danmakus []string
	blocks   []string
}

func (b *recordBot) SendDanmaku(text string) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.danmakus = append(b.danmakus, text)
	return nil
}

func (b *recordBot) Block(uid string, hours int) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.blocks = append(b.blocks, uid)
	return nil
}

func (b *recordBot) sent() []string {
	b.mu.Lock()
	defer b.mu.Unlock()
	out := make([]string, len(b.danmakus))
	copy(out, b.danmakus)
	return out
}

func (b *recordBot) blocked() []string {
	b.mu.Lock()
	defer b.mu.Unlock()
	out := make([]string, len(b.blocks))
	copy(out, b.blocks)
	return out
}

func newTestEngine(t *testing.T, rs []Rule, bot BotAPI) *Engine {
	t.Helper()
	e, err := NewEngine(EngineOptions{
		RoomID:        "1",
		Rules:         rs,
		Bot:           bot,
		GlobalLimiter: ratelimit.NewInterval(0),
		ScriptTimeout: 200 * time.Millisecond,
	})
	if err != nil {
		t.Fatalf("NewEngine 失败: %v", err)
	}
	t.Cleanup(e.Close)
	return e
}

func mkDanmaku(uid, name, text string, guard int) event.Event {
	return event.Event{
		Type: event.TypeDanmaku, RoomID: "1", Timestamp: time.Unix(1700000000, 0),
		Payload: event.Danmaku{
			User: event.User{UID: uid, Username: name, GuardLevel: guard},
			Text: text,
		},
	}
}

func mkEnter(uid, name string, guard int) event.Event {
	return event.Event{
		Type: event.TypeUserEnter, RoomID: "1", Timestamp: time.Unix(1700000000, 0),
		Payload: event.UserEnter{User: event.User{UID: uid, Username: name, GuardLevel: guard}},
	}
}

func TestEngineSimpleRule(t *testing.T) {
	bot := &recordBot{}
	e := newTestEngine(t, []Rule{{
		Name: "复读", Enabled: true, On: []event.Type{event.TypeDanmaku},
		Do: []Action{{Type: ActionDanmaku, Template: []string{"收到：{{.text}}"}}},
	}}, bot)

	e.Handle(mkDanmaku("1", "甲", "你好", 0))

	if got := bot.sent(); len(got) != 1 || got[0] != "收到：你好" {
		t.Errorf("= %v", got)
	}
}

func TestEngineConditionFilters(t *testing.T) {
	bot := &recordBot{}
	e := newTestEngine(t, []Rule{{
		Name: "仅舰长", Enabled: true, On: []event.Type{event.TypeDanmaku},
		When: &Condition{Field: "user.guardLevel", Op: "gt", Value: 0},
		Do:   []Action{{Type: ActionDanmaku, Template: []string{"舰长你好"}}},
	}}, bot)

	e.Handle(mkDanmaku("1", "甲", "你好", 0)) // 非舰长
	e.Handle(mkDanmaku("2", "乙", "你好", 3)) // 舰长

	if got := bot.sent(); len(got) != 1 {
		t.Errorf("只应响应舰长，实际 %v", got)
	}
}

func TestEngineAggregatesEnters(t *testing.T) {
	bot := &recordBot{}
	e := newTestEngine(t, []Rule{{
		Name: "进场欢迎", Enabled: true, On: []event.Type{event.TypeUserEnter},
		Aggregate: &AggregateSpec{Window: 60 * time.Millisecond, By: AggregateByType},
		Do:        []Action{{Type: ActionDanmaku, Template: []string{`欢迎 {{join .users "、"}}`}}},
	}}, bot)

	e.Handle(mkEnter("1", "甲", 0))
	e.Handle(mkEnter("2", "乙", 0))
	e.Handle(mkEnter("3", "丙", 0))

	if got := bot.sent(); len(got) != 0 {
		t.Errorf("窗口未到期不应发送，实际 %v", got)
	}
	time.Sleep(150 * time.Millisecond)

	got := bot.sent()
	if len(got) != 1 {
		t.Fatalf("应合并为 1 条，实际 %v", got)
	}
	if got[0] != "欢迎 甲、乙、丙" {
		t.Errorf("= %q", got[0])
	}
}

func TestEngineConditionAppliedBeforeAggregation(t *testing.T) {
	// 这是核心约定：先按单事件过滤，再合并
	bot := &recordBot{}
	e := newTestEngine(t, []Rule{{
		Name: "只欢迎舰长", Enabled: true, On: []event.Type{event.TypeUserEnter},
		When:      &Condition{Field: "user.guardLevel", Op: "gt", Value: 0},
		Aggregate: &AggregateSpec{Window: 60 * time.Millisecond, By: AggregateByType},
		Do:        []Action{{Type: ActionDanmaku, Template: []string{`欢迎 {{join .users "、"}}`}}},
	}}, bot)

	e.Handle(mkEnter("1", "普通甲", 0))
	e.Handle(mkEnter("2", "舰长乙", 3))
	e.Handle(mkEnter("3", "普通丙", 0))
	e.Handle(mkEnter("4", "舰长丁", 3))

	time.Sleep(150 * time.Millisecond)

	got := bot.sent()
	if len(got) != 1 {
		t.Fatalf("应产出 1 条，实际 %v", got)
	}
	if got[0] != "欢迎 舰长乙、舰长丁" {
		t.Errorf("= %q，非舰长不该进入合并结果", got[0])
	}
}

func TestEngineMergesDuplicateEnter(t *testing.T) {
	// P0 联调发现的真实问题：ENTRY_EFFECT 无昵称 + INTERACT_WORD_V2 完整
	bot := &recordBot{}
	e := newTestEngine(t, []Rule{{
		Name: "进场欢迎", Enabled: true, On: []event.Type{event.TypeUserEnter},
		Aggregate: &AggregateSpec{Window: 60 * time.Millisecond, By: AggregateByType},
		Do:        []Action{{Type: ActionDanmaku, Template: []string{`欢迎 {{join .users "、"}}`}}},
	}}, bot)

	e.Handle(mkEnter("1018633655", "", 3))          // ENTRY_EFFECT
	e.Handle(mkEnter("1018633655", "洛洛的小小小", 0)) // INTERACT_WORD_V2

	time.Sleep(150 * time.Millisecond)

	got := bot.sent()
	if len(got) != 1 {
		t.Fatalf("同一用户应只欢迎一次，实际 %v", got)
	}
	if got[0] != "欢迎 洛洛的小小小" {
		t.Errorf("= %q，应取非空昵称且不重复", got[0])
	}
}

func TestEngineCooldownBlocksRepeat(t *testing.T) {
	bot := &recordBot{}
	e := newTestEngine(t, []Rule{{
		Name: "有冷却", Enabled: true, On: []event.Type{event.TypeDanmaku},
		Cooldown: time.Hour,
		Do:       []Action{{Type: ActionDanmaku, Template: []string{"回复"}}},
	}}, bot)

	for i := 0; i < 5; i++ {
		e.Handle(mkDanmaku("1", "甲", "你好", 0))
	}
	if got := bot.sent(); len(got) != 1 {
		t.Errorf("冷却期内只应发一次，实际 %v", got)
	}
}

func TestEngineCooldownGroupShared(t *testing.T) {
	bot := &recordBot{}
	e, err := NewEngine(EngineOptions{
		RoomID:         "1",
		GlobalLimiter:  ratelimit.NewInterval(0),
		CooldownGroups: map[string]time.Duration{"greeting": time.Hour},
		Bot:            bot,
		Rules: []Rule{
			{Name: "规则A", Enabled: true, On: []event.Type{event.TypeDanmaku},
				CooldownGroup: "greeting",
				Do:            []Action{{Type: ActionDanmaku, Template: []string{"A"}}}},
			{Name: "规则B", Enabled: true, On: []event.Type{event.TypeDanmaku},
				CooldownGroup: "greeting",
				Do:            []Action{{Type: ActionDanmaku, Template: []string{"B"}}}},
		},
	})
	if err != nil {
		t.Fatalf("NewEngine 失败: %v", err)
	}
	defer e.Close()

	e.Handle(mkDanmaku("1", "甲", "你好", 0))

	if got := bot.sent(); len(got) != 1 {
		t.Errorf("同组规则应共享节流，实际 %v", got)
	}
}

func TestEngineMultipleRulesAllFire(t *testing.T) {
	bot := &recordBot{}
	e := newTestEngine(t, []Rule{
		{Name: "规则A", Enabled: true, On: []event.Type{event.TypeDanmaku},
			Do: []Action{{Type: ActionDanmaku, Template: []string{"A"}}}},
		{Name: "规则B", Enabled: true, On: []event.Type{event.TypeDanmaku},
			Do: []Action{{Type: ActionDanmaku, Template: []string{"B"}}}},
	}, bot)

	e.Handle(mkDanmaku("1", "甲", "你好", 0))

	got := bot.sent()
	if len(got) != 2 || got[0] != "A" || got[1] != "B" {
		t.Errorf("两条规则都应触发且保持顺序，实际 %v", got)
	}
}

func TestEngineBlockAction(t *testing.T) {
	bot := &recordBot{}
	e := newTestEngine(t, []Rule{{
		Name: "关键词禁言", Enabled: true, On: []event.Type{event.TypeDanmaku},
		When: &Condition{Field: "text", Op: "contains", Value: "广告"},
		Do:   []Action{{Type: ActionBlock, Hours: 1}},
	}}, bot)

	e.Handle(mkDanmaku("999", "坏人", "这是广告", 0))

	if got := bot.blocked(); len(got) != 1 || got[0] != "999" {
		t.Errorf("= %v", got)
	}
}

func TestEngineScriptAction(t *testing.T) {
	bot := &recordBot{}
	e := newTestEngine(t, []Rule{{
		Name: "脚本", Enabled: true, On: []event.Type{event.TypeDanmaku},
		Do: []Action{{Type: ActionScript,
			Script: `if (event.text.length > 2) { bot.sendDanmaku("长弹幕：" + event.user.username) }`}},
	}}, bot)

	e.Handle(mkDanmaku("1", "甲", "很长的一条弹幕", 0))

	if got := bot.sent(); len(got) != 1 || got[0] != "长弹幕：甲" {
		t.Errorf("= %v", got)
	}
}

func TestEngineScriptCondition(t *testing.T) {
	bot := &recordBot{}
	e := newTestEngine(t, []Rule{{
		Name: "脚本条件", Enabled: true, On: []event.Type{event.TypeDanmaku},
		When: &Condition{Script: `event.text.length > 5`},
		Do:   []Action{{Type: ActionDanmaku, Template: []string{"命中"}}},
	}}, bot)

	e.Handle(mkDanmaku("1", "甲", "短", 0))
	e.Handle(mkDanmaku("1", "甲", "这是一条很长的弹幕", 0))

	if got := bot.sent(); len(got) != 1 {
		t.Errorf("只有长弹幕应命中，实际 %v", got)
	}
}

func TestEngineStorageAcrossRuns(t *testing.T) {
	bot := &recordBot{}
	e := newTestEngine(t, []Rule{{
		Name: "计数", Enabled: true, On: []event.Type{event.TypeDanmaku},
		Do: []Action{{Type: ActionScript, Script: `
			var n = parseInt(storage.get("计数") || "0") + 1;
			storage.set("计数", String(n));
			bot.sendDanmaku("第 " + n + " 条");
		`}},
	}}, bot)

	e.Handle(mkDanmaku("1", "甲", "a", 0))
	e.Handle(mkDanmaku("1", "甲", "b", 0))
	e.Handle(mkDanmaku("1", "甲", "c", 0))

	got := bot.sent()
	if len(got) != 3 || got[0] != "第 1 条" || got[2] != "第 3 条" {
		t.Errorf("storage 应跨次保持，实际 %v", got)
	}
}

func TestEngineRuleErrorIsolation(t *testing.T) {
	bot := &recordBot{}
	e := newTestEngine(t, []Rule{
		{Name: "坏脚本", Enabled: true, On: []event.Type{event.TypeDanmaku},
			Do: []Action{{Type: ActionScript, Script: `null.foo`}}},
		{Name: "正常规则", Enabled: true, On: []event.Type{event.TypeDanmaku},
			Do: []Action{{Type: ActionDanmaku, Template: []string{"正常"}}}},
	}, bot)

	e.Handle(mkDanmaku("1", "甲", "你好", 0))

	if got := bot.sent(); len(got) != 1 || got[0] != "正常" {
		t.Errorf("单条规则出错不应影响其他规则，实际 %v", got)
	}
}

func TestEngineIgnoresUnmatchedEventType(t *testing.T) {
	bot := &recordBot{}
	e := newTestEngine(t, []Rule{{
		Name: "只管礼物", Enabled: true, On: []event.Type{event.TypeGift},
		Do: []Action{{Type: ActionDanmaku, Template: []string{"谢谢"}}},
	}}, bot)

	e.Handle(mkDanmaku("1", "甲", "你好", 0))

	if got := bot.sent(); len(got) != 0 {
		t.Errorf("不匹配的事件类型不应触发，实际 %v", got)
	}
}

func TestEngineFireScheduled(t *testing.T) {
	bot := &recordBot{}
	e := newTestEngine(t, []Rule{{
		Name: "定时广告", Enabled: true, Schedule: "0 */5 * * * *",
		Do: []Action{{Type: ActionDanmaku, Template: []string{"关注主播不迷路"}}},
	}}, bot)

	if names := e.ScheduledRules(); len(names) != 1 || names[0].Name != "定时广告" {
		t.Fatalf("ScheduledRules = %v", names)
	}

	e.FireScheduled("定时广告")

	if got := bot.sent(); len(got) != 1 || got[0] != "关注主播不迷路" {
		t.Errorf("= %v", got)
	}
}

func TestEngineFireScheduledUnknownIsNoop(t *testing.T) {
	bot := &recordBot{}
	e := newTestEngine(t, nil, bot)
	e.FireScheduled("不存在的规则") // 不得 panic
	if got := bot.sent(); len(got) != 0 {
		t.Errorf("= %v", got)
	}
}

func TestEngineCloseFlushesPendingAggregates(t *testing.T) {
	bot := &recordBot{}
	e, err := NewEngine(EngineOptions{
		RoomID: "1", Bot: bot, GlobalLimiter: ratelimit.NewInterval(0),
		Rules: []Rule{{
			Name: "进场欢迎", Enabled: true, On: []event.Type{event.TypeUserEnter},
			Aggregate: &AggregateSpec{Window: time.Hour, By: AggregateByType},
			Do:        []Action{{Type: ActionDanmaku, Template: []string{`欢迎 {{join .users "、"}}`}}},
		}},
	})
	if err != nil {
		t.Fatalf("NewEngine 失败: %v", err)
	}

	e.Handle(mkEnter("1", "甲", 0))
	e.Close()

	if got := bot.sent(); len(got) != 1 {
		t.Errorf("Close 应结算未决窗口，实际 %v", got)
	}
}

func TestEngineRejectsInvalidRule(t *testing.T) {
	_, err := NewEngine(EngineOptions{
		RoomID: "1", Bot: &recordBot{}, GlobalLimiter: ratelimit.NewInterval(0),
		Rules: []Rule{{Name: "无动作", Enabled: true, On: []event.Type{event.TypeDanmaku}}},
	})
	if err == nil {
		t.Error("非法规则应在构造时报错，不允许带病运行")
	}
}

func TestMemStorage(t *testing.T) {
	s := NewMemStorage()
	if _, ok := s.Get("空"); ok {
		t.Error("未写入的键应返回 false")
	}
	s.Set("键", "值")
	if v, ok := s.Get("键"); !ok || v != "值" {
		t.Errorf("Get = %q %v", v, ok)
	}
	s.Set("键", "新值")
	if v, _ := s.Get("键"); v != "新值" {
		t.Errorf("覆盖失败: %q", v)
	}
}

func TestMemStorageConcurrent(t *testing.T) {
	s := NewMemStorage()
	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			s.Set("键", "值")
			s.Get("键")
		}(i)
	}
	wg.Wait()
}
```

- [ ] **Step 2: 运行测试确认失败**

```bash
cd server && go test ./internal/rules/ -run 'TestEngine|TestMemStorage' -v
```

Expected: 编译失败，`undefined: NewEngine`。

- [ ] **Step 3: 实现内存存储**

创建 `server/internal/rules/storage.go`：

```go
package rules

import "sync"

// MemStorage 是房间级键值存储的内存实现。
//
// P2 阶段够用；P3 会换成数据库实现，届时只需换掉本类型，
// 脚本侧的 storage.get/set 接口不变。
type MemStorage struct {
	mu sync.RWMutex
	m  map[string]string
}

var _ Storage = (*MemStorage)(nil)

// NewMemStorage 创建内存存储。
func NewMemStorage() *MemStorage {
	return &MemStorage{m: make(map[string]string)}
}

// Get 取值，不存在时返回 ("", false)。
func (s *MemStorage) Get(key string) (string, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	v, ok := s.m[key]
	return v, ok
}

// Set 写入或覆盖。
func (s *MemStorage) Set(key, value string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.m[key] = value
}
```

- [ ] **Step 4: 实现 Engine**

创建 `server/internal/rules/engine.go`：

```go
package rules

import (
	"context"
	"log/slog"
	"math/rand"
	"sync"
	"time"

	"github.com/MrZhongzq/MagicalDanmaku/server/internal/event"
	"github.com/MrZhongzq/MagicalDanmaku/server/internal/ratelimit"
)

// EngineOptions 配置规则引擎。
type EngineOptions struct {
	RoomID         string
	Rules          []Rule
	Bot            BotAPI
	Storage        Storage // 可为 nil，此时使用内存存储
	GlobalLimiter  ratelimit.Limiter
	CooldownGroups map[string]time.Duration
	ScriptTimeout  time.Duration
	Logger         *slog.Logger
}

// Engine 是单个房间的规则处理流水线。
//
//	事件 → 按单事件求值条件 → 命中的规则各自合并（或直通）
//	     → 冷却检查 → 动作执行
//
// 条件在合并之前按单个事件求值。若先合并再判断，「只欢迎舰长」这类
// 规则会因为合并后的 Vars 只保留首个用户的等级而误伤非舰长。代价是
// 条件中无法引用 count/users 这类聚合属性——这是有意的取舍。
type Engine struct {
	roomID   string
	matcher  *Matcher
	executor *Executor
	cooldown *Cooldown
	log      *slog.Logger

	// 每条配置了合并的规则各有一个 Aggregator
	aggregators map[string]*Aggregator
	byName      map[string]Rule

	// 串行化处理，避免规则之间产生竞态
	mu     sync.Mutex
	closed bool
}

// NewEngine 创建引擎。规则在此完成校验，非法配置直接报错而非带病运行。
func NewEngine(opts EngineOptions) (*Engine, error) {
	if opts.Logger == nil {
		opts.Logger = slog.Default()
	}
	if opts.Storage == nil {
		opts.Storage = NewMemStorage()
	}
	if opts.GlobalLimiter == nil {
		opts.GlobalLimiter = ratelimit.NewInterval(0)
	}

	for _, r := range opts.Rules {
		if err := r.Validate(); err != nil {
			return nil, err
		}
	}

	sandbox := NewSandbox(SandboxOptions{
		Timeout: opts.ScriptTimeout,
		Bot:     opts.Bot,
		Storage: opts.Storage,
		Logger:  opts.Logger,
	})

	cd := NewCooldown(opts.GlobalLimiter, time.Now)
	for g, d := range opts.CooldownGroups {
		cd.SetGroupInterval(g, d)
	}

	e := &Engine{
		roomID:      opts.RoomID,
		cooldown:    cd,
		log:         opts.Logger,
		aggregators: make(map[string]*Aggregator),
		byName:      make(map[string]Rule, len(opts.Rules)),
	}

	e.matcher = NewMatcher(opts.Rules, NewEvaluator(sandbox), opts.Logger)
	e.executor = NewExecutor(ExecutorOptions{
		Bot:      opts.Bot,
		Renderer: NewRenderer(rand.New(rand.NewSource(time.Now().UnixNano()))),
		Script:   sandbox,
		Cooldown: cd,
		Logger:   opts.Logger,
	})

	// 为配置了合并的规则各建一个 Aggregator。
	// 窗口到期后走冷却检查与动作执行。
	for _, r := range opts.Rules {
		e.byName[r.Name] = r
		if r.Aggregate == nil {
			continue
		}
		rule := r // 捕获副本
		e.aggregators[r.Name] = NewAggregator(*r.Aggregate, func(tr Trigger) {
			e.fire(rule, tr)
		})
	}
	return e, nil
}

// Handle 处理一个事件。
//
// 同一房间内串行处理：规则之间共享冷却状态与存储，串行可避免竞态，
// 也让规则的触发顺序可预测。
func (e *Engine) Handle(ev event.Event) {
	e.mu.Lock()
	defer e.mu.Unlock()

	if e.closed {
		return
	}

	// 条件按单个事件求值
	tr := PassthroughTrigger(ev)
	matched := e.matcher.Match(tr)

	for _, r := range matched {
		if agg, ok := e.aggregators[r.Name]; ok {
			agg.Add(ev) // 进入窗口，到期后再触发
			continue
		}
		e.fireLocked(r, tr)
	}
}

// FireScheduled 触发一条定时规则。
func (e *Engine) FireScheduled(name string) {
	e.mu.Lock()
	defer e.mu.Unlock()

	if e.closed {
		return
	}
	r, ok := e.byName[name]
	if !ok || !r.Enabled || r.Schedule == "" {
		return
	}

	tr := Trigger{
		Type:   TypeScheduled,
		Events: nil,
		Vars: map[string]any{
			"type":      string(TypeScheduled),
			"roomId":    e.roomID,
			"timestamp": time.Now().Unix(),
			"count":     1,
			"users":     []string{},
		},
	}
	e.fireLocked(r, tr)
}

// ScheduledRules 返回全部启用的定时规则。
func (e *Engine) ScheduledRules() []Rule {
	return e.matcher.ScheduledRules()
}

// fire 是 Aggregator 窗口到期时的回调入口。
//
// 这里刻意**不检查 closed**：Aggregator 只会在自己存活期间或自己的
// Close() 过程中回调，而 Engine.Close() 正是要让这些未决事件结算完毕。
// 若在此拦截，Close 时缓冲区里的欢迎语就会被静默丢弃。
func (e *Engine) fire(r Rule, tr Trigger) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.fireLocked(r, tr)
}

// fireLocked 执行冷却检查与动作。调用者需持有锁。
func (e *Engine) fireLocked(r Rule, tr Trigger) {
	if !e.cooldown.Allow(r) {
		return
	}
	if err := e.executor.Execute(context.Background(), r, tr); err != nil {
		// 单条规则出错不影响其他规则
		e.log.Warn("规则执行出错", "room", e.roomID, "rule", r.Name, "err", err)
	}
}

// Close 停止接收新事件，并结算全部未决的合并窗口。
//
// closed 只拦截新事件入口（Handle / FireScheduled），不拦截 Aggregator
// 的回调——后者正是 Close 要触发的收尾动作。Aggregator.Close() 返回后
// 不会再有回调，因此无需额外的关闭标志。
func (e *Engine) Close() {
	e.mu.Lock()
	if e.closed {
		e.mu.Unlock()
		return
	}
	e.closed = true // 不再接收新事件
	aggs := make([]*Aggregator, 0, len(e.aggregators))
	for _, a := range e.aggregators {
		aggs = append(aggs, a)
	}
	e.mu.Unlock()

	// 必须在释放锁之后调用：Aggregator.Close() 会同步触发 fire()，
	// 而 fire() 需要获取同一把锁。
	for _, a := range aggs {
		a.Close()
	}
}
```

- [ ] **Step 5: 运行测试确认通过**

```bash
cd server && go test ./internal/rules/ -v
```

Expected: 全部 PASS。

> `TestEngineCloseFlushesPendingAggregates` 若失败或死锁，检查
> `Aggregator.Close()` 是否在**释放 `e.mu` 之后**调用——它会同步触发
> `fire()`，而后者要获取同一把锁。

- [ ] **Step 6: 竞态检测**

```bash
cd server && CGO_ENABLED=1 go test ./internal/rules/ -race -count=3
```

Expected: PASS，无 DATA RACE。

- [ ] **Step 7: 提交**

```bash
cd server && go vet ./... && gofmt -l .
git add server/internal/rules/
git commit -m "feat: 组装规则引擎流水线"
```

---

### Task 14: magicd run 子命令

**Files:**
- Create: `server/cmd/magicd/run.go`
- Create: `config.example.yaml`
- Modify: `server/cmd/magicd/main.go`
- Test: `server/cmd/magicd/run_test.go`

**Interfaces:**
- Consumes: Task 12 的 `config.Load`；Task 13 的 `rules.NewEngine`；
  Task 10 的 `account.New`；Task 11 的 `scheduler.New`；
  P0 的 `bilibili.NewClient`、`bilibili.NewActions`、`api.New`、`auth.ParseSession`
- Produces:
  - `runRun(args []string) error`
  - `roomBot` — 把 `account.Pool` 适配成 `rules.BotAPI`

- [ ] **Step 1: 写失败测试**

创建 `server/cmd/magicd/run_test.go`：

```go
package main

import (
	"context"
	"errors"
	"sync"
	"testing"
)

// poolStub 记录 roomBot 转发给账号池的调用。
// 它实现 run.go 中的 danmakuSender 接口，因此无需引入 connector 包。
type poolStub struct {
	mu     sync.Mutex
	sent   []string
	blocks []string
	err    error
}

func (p *poolStub) SendDanmaku(ctx context.Context, roomID, text string) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.err != nil {
		return p.err
	}
	p.sent = append(p.sent, roomID+"|"+text)
	return nil
}

func (p *poolStub) Block(ctx context.Context, roomID, uid string, hours int) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.err != nil {
		return p.err
	}
	p.blocks = append(p.blocks, roomID+"|"+uid)
	return nil
}

func TestRoomBotForwardsRoomID(t *testing.T) {
	ps := &poolStub{}
	b := &roomBot{pool: ps, roomID: "1706666491", ctx: context.Background()}

	if err := b.SendDanmaku("你好"); err != nil {
		t.Fatalf("SendDanmaku 失败: %v", err)
	}
	if len(ps.sent) != 1 || ps.sent[0] != "1706666491|你好" {
		t.Errorf("= %v", ps.sent)
	}

	if err := b.Block("999", 2); err != nil {
		t.Fatalf("Block 失败: %v", err)
	}
	if len(ps.blocks) != 1 || ps.blocks[0] != "1706666491|999" {
		t.Errorf("= %v", ps.blocks)
	}
}

func TestRoomBotPropagatesError(t *testing.T) {
	ps := &poolStub{err: errors.New("发送失败")}
	b := &roomBot{pool: ps, roomID: "1", ctx: context.Background()}

	if err := b.SendDanmaku("x"); err == nil {
		t.Error("底层错误应当上报")
	}
}

func TestRunRejectsMissingConfig(t *testing.T) {
	err := runRun([]string{"-c", "/不存在的路径/config.yaml"})
	if err == nil {
		t.Error("配置文件不存在应当报错")
	}
}

func TestRunRejectsEmptyConfigFlag(t *testing.T) {
	if err := runRun([]string{}); err == nil {
		t.Error("未指定 -c 应当报错")
	}
}
```

- [ ] **Step 2: 运行测试确认失败**

```bash
cd server && go test ./cmd/magicd/ -run 'TestRoomBot|TestRun' -v
```

Expected: 编译失败，`undefined: roomBot`。

- [ ] **Step 3: 实现**

创建 `server/cmd/magicd/run.go`：

```go
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"strings"
	"sync"
	"time"

	"github.com/MrZhongzq/MagicalDanmaku/server/internal/account"
	"github.com/MrZhongzq/MagicalDanmaku/server/internal/connector/bilibili"
	"github.com/MrZhongzq/MagicalDanmaku/server/internal/connector/bilibili/api"
	"github.com/MrZhongzq/MagicalDanmaku/server/internal/connector/bilibili/auth"
	"github.com/MrZhongzq/MagicalDanmaku/server/internal/ratelimit"
	"github.com/MrZhongzq/MagicalDanmaku/server/internal/rules"
	"github.com/MrZhongzq/MagicalDanmaku/server/internal/rules/config"
	"github.com/MrZhongzq/MagicalDanmaku/server/internal/scheduler"
)

// danmakuSender 是 roomBot 依赖的账号池能力，抽成接口便于测试。
type danmakuSender interface {
	SendDanmaku(ctx context.Context, roomID, text string) error
	Block(ctx context.Context, roomID, uid string, hours int) error
}

// roomBot 把账号池适配成 rules.BotAPI。
//
// BotAPI 的方法不带 ctx —— 它要被 goja 从 JS 里同步调用，签名必须简单。
// 因此把 ctx 存进结构体。这在 Go 里通常是反模式，但此处 roomBot 的
// 生命周期严格等于一次 run，不会泄漏。
type roomBot struct {
	pool   danmakuSender
	roomID string
	ctx    context.Context
}

var _ rules.BotAPI = (*roomBot)(nil)

func (b *roomBot) SendDanmaku(text string) error {
	return b.pool.SendDanmaku(b.ctx, b.roomID, text)
}

func (b *roomBot) Block(uid string, hours int) error {
	return b.pool.Block(b.ctx, b.roomID, uid, hours)
}

// runRun 加载配置并启动机器人。
func runRun(args []string) error {
	fs := flag.NewFlagSet("run", flag.ExitOnError)
	cfgPath := fs.String("c", "", "配置文件路径（必填）")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *cfgPath == "" {
		return errors.New("必须通过 -c 指定配置文件")
	}

	cfg, err := config.Load(*cfgPath)
	if err != nil {
		return err
	}
	if len(cfg.Rooms) == 0 {
		return errors.New("配置中没有任何直播间")
	}

	log := slog.Default()
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	// 载入全部账号
	sessions := make(map[string]*auth.Session, len(cfg.Accounts))
	for _, a := range cfg.Accounts {
		data, err := os.ReadFile(a.CookieFile)
		if err != nil {
			return fmt.Errorf("读取账号 %q 的 Cookie 文件失败: %w", a.Name, err)
		}
		sess, err := auth.ParseSession(strings.TrimSpace(string(data)))
		if err != nil {
			return fmt.Errorf("账号 %q 的 Cookie 无效: %w", a.Name, err)
		}
		sessions[a.Name] = sess
		log.Info("已载入账号", "name", a.Name, "uid", sess.UID)
	}

	// 全局限流由全部房间共享：风控是按账号算的，不是按房间。
	interval := time.Duration(cfg.RateLimit.Interval)
	if interval <= 0 {
		interval = 1500 * time.Millisecond
	}
	globalLimiter := ratelimit.NewInterval(interval)

	groups := make(map[string]time.Duration, len(cfg.CooldownGroups))
	for k, v := range cfg.CooldownGroups {
		groups[k] = time.Duration(v)
	}

	sched := scheduler.New(log)
	var wg sync.WaitGroup
	var closers []func()

	for _, room := range cfg.Rooms {
		// 每个房间一个账号池
		var accs []account.Account
		for _, name := range room.Accounts {
			sess := sessions[name]
			ac := api.New(sess)
			accs = append(accs, account.Account{
				Name:    name,
				Actions: bilibili.NewActions(ac, globalLimiter),
			})
		}
		if len(accs) == 0 {
			return fmt.Errorf("房间 %s 未配置任何账号", room.ID)
		}
		pool := account.New(accs, log)

		bot := &roomBot{pool: pool, roomID: room.ID, ctx: ctx}
		engine, err := rules.NewEngine(rules.EngineOptions{
			RoomID:         room.ID,
			Rules:          cfg.Rules,
			Bot:            bot,
			GlobalLimiter:  globalLimiter,
			CooldownGroups: groups,
			Logger:         log,
		})
		if err != nil {
			return fmt.Errorf("房间 %s 的规则非法: %w", room.ID, err)
		}
		closers = append(closers, engine.Close)

		// 注册该房间的定时规则
		for _, r := range engine.ScheduledRules() {
			name := r.Name
			eng := engine
			if err := sched.Add(r.Schedule, room.ID+"/"+name, func() {
				eng.FireScheduled(name)
			}); err != nil {
				return err
			}
		}

		// 用第一个账号的会话建立事件流连接
		apiClient := api.New(sessions[room.Accounts[0]])
		if err := apiClient.RefreshNav(ctx); err != nil {
			return fmt.Errorf("房间 %s 初始化签名失败: %w", room.ID, err)
		}
		info, err := apiClient.RoomInfo(ctx, room.ID)
		if err != nil {
			return fmt.Errorf("房间 %s 信息获取失败: %w", room.ID, err)
		}
		log.Info("已配置直播间", "room", info.RoomID, "title", info.Title,
			"living", info.IsLiving(), "accounts", len(accs), "rules", len(cfg.Rules))

		client := bilibili.NewClient(info.RoomID, apiClient, bilibili.WithLogger(log))

		wg.Add(2)
		go func() {
			defer wg.Done()
			for ev := range client.Events() {
				engine.Handle(ev)
			}
		}()
		go func(roomID string) {
			defer wg.Done()
			if err := client.Run(ctx); err != nil && !errors.Is(err, context.Canceled) {
				log.Error("房间连接退出", "room", roomID, "err", err)
			}
		}(info.RoomID)
	}

	sched.Start()
	log.Info("机器人已启动，按 Ctrl+C 退出")

	<-ctx.Done()
	log.Info("正在退出...")

	sched.Stop()
	wg.Wait()
	for _, c := range closers {
		c()
	}
	log.Info("已退出")
	return nil
}
```

- [ ] **Step 4: 接进 main.go**

修改 `server/cmd/magicd/main.go`，在用法说明中加入 run，并注册子命令：

```go
  magicd run -c config.yaml
        按配置文件运行弹幕机器人
```

在 switch 中新增：

```go
	case "run":
		err = runRun(os.Args[2:])
```

- [ ] **Step 5: 写示例配置**

创建仓库根的 `config.example.yaml`：

```yaml
# 神奇弹幕配置示例
#
# 用法：
#   magicd login -o cookie.txt        # 先扫码登录
#   cp config.example.yaml config.yaml
#   magicd run -c config.yaml

accounts:
  - name: main
    cookieFile: cookie.txt

rooms:
  - id: "1706666491"
    accounts: [main]

# 全局发送限流，防止账号触发风控。所有房间共享。
rateLimit:
  interval: 1.5s

# 命名冷却组：多条规则共享节流。
cooldownGroups:
  greeting: 5s
  thanks: 2s

rules:
  - name: 舰长进场欢迎
    on: [user_enter]
    when:
      field: user.guardLevel
      op: ">"
      value: 0
    # 合并窗口：一波人同时进场只发一条，顺带解决同一次进场的重复事件
    aggregate:
      window: 3s
      by: type
    cooldownGroup: greeting
    do:
      - type: danmaku
        template:
          - '欢迎 {{join .users "、"}} 回家~'
          - '{{join .users "、"}} 来啦！'

  - name: 礼物答谢
    on: [gift]
    # 按「用户+礼物」合并，连击会累加数量
    aggregate:
      window: 3s
      by: gift
    cooldownGroup: thanks
    do:
      - type: danmaku
        template:
          - '感谢 {{simplifyName .user.username}} 的 {{.gift.name}} x{{.gift.count}}！'

  - name: 上舰答谢
    on: [guard_buy]
    do:
      - type: danmaku
        template:
          - '感谢 {{simplifyName .user.username}} 开通{{.guard.name}}！'

  - name: 关键词回复
    on: [danmaku]
    when:
      field: text
      op: contains
      value: 求歌单
    cooldown: 30s
    do:
      - type: danmaku
        template: ['歌单在主播的动态里哦~']

  - name: 广告禁言
    enabled: false     # 默认关闭，确认规则无误后再启用
    on: [danmaku]
    when:
      any:
        - field: text
          op: regex
          value: '(加群|私聊|代打)'
    do:
      - type: block
        hours: 1

  - name: 定时提醒
    schedule: '0 */10 * * * *'   # 6 字段含秒：每 10 分钟
    do:
      - type: danmaku
        template: ['喜欢主播的话点个关注吧~']
```

- [ ] **Step 6: 运行测试确认通过**

```bash
cd server && go test ./... -count=1 && go vet ./... && gofmt -l .
```

Expected: 全部 PASS，vet 与 gofmt 无输出。

- [ ] **Step 7: 校验示例配置可被加载**

```bash
cd server && cat > /tmp/cfgcheck_test.go <<'EOF'
package config

import "testing"

func TestExampleConfigLoads(t *testing.T) {
	c, err := Load("../../../../config.example.yaml")
	if err != nil {
		t.Fatalf("示例配置应当可以加载: %v", err)
	}
	if len(c.Rules) == 0 {
		t.Error("示例配置应含规则")
	}
}
EOF
cp /tmp/cfgcheck_test.go internal/rules/config/example_test.go
go test ./internal/rules/config/ -run TestExampleConfig -v
```

Expected: PASS。这条测试留在仓库中，保证示例配置不会随模型演化而失效。

- [ ] **Step 8: 交叉编译复检**

```bash
cd server
for t in linux/amd64 linux/arm64 darwin/arm64 darwin/amd64 windows/amd64 windows/arm64; do
  GOOS=${t%/*} GOARCH=${t#*/} CGO_ENABLED=0 go build -o /dev/null ./cmd/magicd && echo "OK $t"
done
```

Expected: 六个目标全部 OK。

- [ ] **Step 9: 提交**

```bash
git add server/ config.example.yaml
git commit -m "feat: 实现 run 子命令与示例配置"
```

---

## P2 验收

- [ ] **验收 1: 静态检查与测试**

```bash
cd server && go vet ./... && gofmt -l . && go test ./... -count=1
```

Expected: 无 vet 输出、无未格式化文件、全部包 PASS。

- [ ] **验收 2: 竞态检测**

```bash
cd server && CGO_ENABLED=1 go test ./... -race -count=3
```

Expected: PASS，无 DATA RACE。

> Windows 需先把 MinGW 加进 PATH：
> `C:\Users\ZIQI\AppData\Local\Microsoft\WinGet\Packages\BrechtSanders.WinLibs.POSIX.UCRT_Microsoft.Winget.Source_8wekyb3d8bbwe\mingw64\bin`

- [ ] **验收 3: 全平台构建**

```bash
scripts/build.sh
```

Expected: 六个目标全部 OK，产物落在 `dist/`。

- [ ] **验收 4: 真实直播间联调**

```bash
./magicd.exe login -o cookie.txt          # 若 Cookie 已过期
cp config.example.yaml config.yaml
# 编辑 config.yaml，把 rooms[0].id 改成一个正在直播的房间号
./magicd.exe run -c config.yaml
```

逐项确认：

1. 启动日志打印出账号 UID、房间标题、规则数
2. 有人进场时，**若干秒后合并发出一条欢迎**，而非每人一条
3. 同一用户不会被欢迎两次（`ENTRY_EFFECT` 与 `INTERACT_WORD_V2` 已合并）
4. 收到礼物时按「用户+礼物」合并答谢，连击数量正确累加
5. 定时规则按 cron 表达式触发
6. Ctrl+C 能干净退出，未决的合并窗口被结算

> **联调前务必先用 `enabled: false` 关掉禁言规则**，确认匹配无误后再启用。
> 误禁言真实观众无法自动撤销。

- [ ] **验收 5: 配置错误能被拦截**

故意写错配置，确认启动即报错而非带病运行：

```bash
printf 'rules:\n  - name: 坏规则\n    do: [{type: log}]\n' > /tmp/bad.yaml
./magicd.exe run -c /tmp/bad.yaml
```

Expected: 报错退出，信息指出「必须指定 on 或 schedule 之一」。

---

## 计划自查记录

**规格覆盖**：设计文档各节与任务的对应关系——

| 设计文档章节 | 覆盖任务 |
|---|---|
| 2. 架构 Pipeline | Task 13 |
| 3. Trigger 与 Vars | Task 1、Task 2 |
| 3.2 缺失字段不报错 | Task 2（LookupPath）、Task 4（missingkey=zero） |
| 4. 去重与合并 | Task 7 |
| 4.1 共用缓冲区解决进场重复 | Task 7（MergeVars）、Task 13（端到端验证） |
| 5.1 条件模型与操作符 | Task 1（模型）、Task 3（求值） |
| 5.2 动作模型 | Task 1、Task 9 |
| 6. 模板与内置函数 | Task 4 |
| 7. goja 沙箱 | Task 5 |
| 7.2 不注入网络访问 | Task 5（测试断言无 fetch/XMLHttpRequest） |
| 7.3 超时中断 | Task 5 |
| 7.4 Runtime 池 | Task 5 |
| 8. 三层冷却 | Task 6 |
| 9. 多账号轮换 | Task 10 |
| 10. 定时任务 | Task 11、Task 13（FireScheduled）、Task 14（注册） |
| 11. 配置 | Task 12、Task 14（示例） |
| 12. 错误处理 | Task 8（匹配隔离）、Task 9（动作隔离）、Task 11（panic 捕获）、Task 13（规则隔离） |
| 13. 测试策略 | 各任务的测试步骤 |
| 14. 仓库布局 | 各任务的 Files 段 |
| 15. 只通过 P0 公开接口 | Task 10、Task 14 |

**已知偏差**（有意为之，均在对应任务中说明）：

1. **条件在合并之前按单事件求值**（Task 13）。设计文档未明确时机；
   若先合并再判断，「只欢迎舰长」会误伤非舰长。代价是条件中无法引用
   `count`、`users`。
2. **操作符支持符号别名**（Task 12）。设计文档第 11 节示例写 `op: ">"`，
   而模型定义为 `gt`。加载时归一化，两种写法都接受。
3. **Storage 只做内存实现**（Task 13）。设计文档提到「内存 + 文件」，
   但文件持久化与 P3 的数据层职责重叠，P2 做等于白写。脚本接口不变，
   P3 换实现即可。
