# P3 多租户数据层 实施计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 把配置、授权与业务日志迁进 PostgreSQL，建立「用户 / B站账号 / 绑定 / 权限点」模型，使 `magicd run` 不再依赖 `config.yaml`。

**Architecture:** 新增 `internal/store`（PostgreSQL 存储层，不做接口抽象）、`internal/perm`（权限点常量）、`internal/logging`（系统日志与业务日志写入器）、`internal/rules/spec`（规则的唯一序列化表示，供 YAML / JSONB / 未来的 API 共用）。`internal/rules/config` 退化为薄薄的 YAML 加载层。

**Tech Stack:** Go 1.25、PostgreSQL 14+、`jackc/pgx/v5`、`golang.org/x/crypto/bcrypt`、`gopkg.in/natefinch/lumberjack.v2`

## Global Constraints

- 设计文档：`docs/superpowers/specs/2026-07-31-p3-data-layer-design.md`，本计划的每个决定都以它为准
- module 路径 `github.com/MrZhongzq/MagicalDanmaku/server`，全部代码在仓库 `server/` 目录下
- **纯 Go 依赖，`CGO_ENABLED=0` 必须能交叉编译六个目标**（win/darwin/linux × amd64/arm64）。新增依赖前先确认无 cgo
- 只支持 PostgreSQL，不实现 SQLite，不做数据库抽象接口
- Cookie 明文存储，不加密。`accounts` 表的 Cookie 读写各只允许有一处
- 迁移只做前向，不写回滚脚本
- 注释、日志、错误信息一律用中文；错误信息带上包名前缀，如 `store: 用户 %q 不存在`
- TDD：先写失败的测试 → 跑一遍确认它失败 → 写最小实现 → 跑一遍确认通过 → 提交
- **验证命令一律用 `; echo "退出码=$?"` 而非 `&&`**。管道会让 `&&` 判断管道末端命令的退出码，曾因此提交过失败的测试
- 每个任务结束时 `gofmt -l .` 必须无输出、`go vet ./...` 必须干净

## 任务分布

| 文件 | 任务 |
|---|---|
| 本文件 | Task 1–4：权限点、规则序列化表示、存储基座与迁移器、用户表 |
| `2026-07-31-p3-data-layer-part2.md` | Task 5–10：账号、绑定、规则、授权、KV 与禁言名单、运行配置载入 |
| `2026-07-31-p3-data-layer-part3.md` | Task 11–14：业务日志存储、异步写入器、引擎钩子、系统日志 |
| `2026-07-31-p3-data-layer-part4.md` | Task 15–19：命令行、run 接线、CI 与文档 |

## 文件结构

```
server/internal/perm/perm.go                      权限点常量与校验
server/internal/rules/spec/spec.go                线上格式类型（yaml + json 标签）
server/internal/rules/spec/duration.go            Duration 的四个 marshal 方法
server/internal/rules/spec/convert.go             spec.Rule → rules.Rule
server/internal/rules/config/config.go            退化为 YAML 薄加载层
server/internal/store/store.go                    Store 结构、连接池、Close
server/internal/store/migrate.go                  迁移器（advisory lock + embed）
server/internal/store/migrations/001_init.sql     全部建表语句
server/internal/store/testhelp_test.go            测试基座：独立 schema
server/internal/store/user.go                     用户 CRUD 与密码
server/internal/store/account.go                  账号 CRUD，Cookie 读写唯一收口
server/internal/store/binding.go                  绑定与冷却组 CRUD
server/internal/store/rule.go                     规则 CRUD，JSONB 拆装
server/internal/store/membership.go               授权 CRUD 与 Can
server/internal/store/kv.go                       脚本 storage
server/internal/store/blocklist.go                永久禁言名单
server/internal/store/activity.go                 业务日志写入与查询
server/internal/store/runconfig.go                LoadRunConfig
server/internal/logging/system.go                 slog + lumberjack
server/internal/logging/activity.go               业务日志异步批量写入器
server/cmd/magicd/{migrate,user,grant,import}.go  新增子命令
docker-compose.dev.yml                            本地开发用 PostgreSQL
```

---

## Task 1: 权限点

**Files:**
- Create: `server/internal/perm/perm.go`
- Test: `server/internal/perm/perm_test.go`

**Interfaces:**
- Consumes: 无
- Produces: `perm.Permission` 字符串类型；七个常量 `perm.RuleRead`、`perm.RuleWrite`、`perm.DanmakuSend`、`perm.UserBlock`、`perm.AccountManage`、`perm.MemberManage`、`perm.EventRead`；`perm.All() []Permission`；`perm.Parse(s string) (Permission, error)`；`perm.ParseList(s string) ([]Permission, error)`；`perm.Strings(ps []Permission) []string`

- [ ] **Step 1: 写失败的测试**

创建 `server/internal/perm/perm_test.go`：

```go
package perm_test

import (
	"strings"
	"testing"

	"github.com/MrZhongzq/MagicalDanmaku/server/internal/perm"
)

func TestAllContainsSevenPermissions(t *testing.T) {
	all := perm.All()
	if len(all) != 7 {
		t.Fatalf("权限点数量 = %d, 期望 7: %v", len(all), all)
	}
	seen := make(map[perm.Permission]bool, len(all))
	for _, p := range all {
		if seen[p] {
			t.Errorf("权限点 %q 重复", p)
		}
		seen[p] = true
	}
	for _, want := range []perm.Permission{
		perm.RuleRead, perm.RuleWrite, perm.DanmakuSend,
		perm.UserBlock, perm.AccountManage, perm.MemberManage, perm.EventRead,
	} {
		if !seen[want] {
			t.Errorf("All() 缺少 %q", want)
		}
	}
}

func TestParseKnownPermission(t *testing.T) {
	got, err := perm.Parse("rule:write")
	if err != nil {
		t.Fatalf("Parse 报错: %v", err)
	}
	if got != perm.RuleWrite {
		t.Errorf("Parse(\"rule:write\") = %q, 期望 %q", got, perm.RuleWrite)
	}
}

func TestParseUnknownPermissionListsValidOnes(t *testing.T) {
	_, err := perm.Parse("rule:delete")
	if err == nil {
		t.Fatal("未知权限点应报错")
	}
	// 报错信息要能直接告诉用户合法值有哪些，否则只能翻文档
	if !strings.Contains(err.Error(), "rule:write") {
		t.Errorf("错误信息应列出合法权限点，实际: %v", err)
	}
}

func TestParseListSplitsOnComma(t *testing.T) {
	got, err := perm.ParseList("rule:read, rule:write ,event:read")
	if err != nil {
		t.Fatalf("ParseList 报错: %v", err)
	}
	want := []perm.Permission{perm.RuleRead, perm.RuleWrite, perm.EventRead}
	if len(got) != len(want) {
		t.Fatalf("ParseList 返回 %v, 期望 %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("第 %d 项 = %q, 期望 %q", i, got[i], want[i])
		}
	}
}

func TestParseListRejectsEmpty(t *testing.T) {
	if _, err := perm.ParseList("  "); err == nil {
		t.Fatal("空权限列表应报错")
	}
}

func TestParseListDeduplicates(t *testing.T) {
	got, err := perm.ParseList("rule:read,rule:read")
	if err != nil {
		t.Fatalf("ParseList 报错: %v", err)
	}
	if len(got) != 1 {
		t.Errorf("重复项应去重，实际 %v", got)
	}
}

func TestStringsRoundTrip(t *testing.T) {
	ss := perm.Strings([]perm.Permission{perm.RuleRead, perm.UserBlock})
	if len(ss) != 2 || ss[0] != "rule:read" || ss[1] != "user:block" {
		t.Errorf("Strings = %v", ss)
	}
}
```

- [ ] **Step 2: 跑测试确认失败**

```bash
cd server; go test ./internal/perm/ 2>&1 | tail -5; echo "退出码=$?"
```

预期：编译失败，`no Go files in .../internal/perm`。

- [ ] **Step 3: 实现**

创建 `server/internal/perm/perm.go`：

```go
// Package perm 定义授权用的权限点。
//
// 不设固定角色。角色（「运营」「房管」）只是若干权限点的预设组合，
// 展开发生在界面层，存储层里永远只有权限点本身——这样新增一种职责
// 分工不需要改数据模型。
package perm

import (
	"fmt"
	"strings"
)

// Permission 是一个权限点。
type Permission string

// 全部权限点。授权单位是「账号-直播间」绑定，即持有者能对某个账号在
// 某个直播间做什么。
const (
	RuleRead      Permission = "rule:read"      // 查看规则
	RuleWrite     Permission = "rule:write"     // 增删改规则、启停规则
	DanmakuSend   Permission = "danmaku:send"   // 手动发送弹幕
	UserBlock     Permission = "user:block"     // 禁言与解禁，含维护禁言名单
	AccountManage Permission = "account:manage" // 修改 Cookie 与账号参数
	MemberManage  Permission = "member:manage"  // 授权他人、撤销授权
	EventRead     Permission = "event:read"     // 查看事件流与历史业务日志
)

// all 按声明顺序排列，All 与错误提示都依赖这个顺序。
var all = []Permission{
	RuleRead, RuleWrite, DanmakuSend,
	UserBlock, AccountManage, MemberManage, EventRead,
}

// known 用于 Parse 的查表。
var known = func() map[Permission]bool {
	m := make(map[Permission]bool, len(all))
	for _, p := range all {
		m[p] = true
	}
	return m
}()

// All 返回全部权限点的副本。
func All() []Permission {
	out := make([]Permission, len(all))
	copy(out, all)
	return out
}

// Parse 把字符串解析成权限点。
//
// 未知权限点的错误信息会列出全部合法值：用户拼错时不该被迫去翻文档。
func Parse(s string) (Permission, error) {
	p := Permission(strings.TrimSpace(s))
	if !known[p] {
		return "", fmt.Errorf("perm: 未知的权限点 %q，合法值为 %s",
			s, strings.Join(Strings(all), ", "))
	}
	return p, nil
}

// ParseList 解析逗号分隔的权限点列表，去重并保持首次出现的顺序。
func ParseList(s string) ([]Permission, error) {
	var out []Permission
	seen := make(map[Permission]bool)

	for _, part := range strings.Split(s, ",") {
		if strings.TrimSpace(part) == "" {
			continue
		}
		p, err := Parse(part)
		if err != nil {
			return nil, err
		}
		if seen[p] {
			continue
		}
		seen[p] = true
		out = append(out, p)
	}

	if len(out) == 0 {
		return nil, fmt.Errorf("perm: 权限点列表为空，至少要给一个")
	}
	return out, nil
}

// Strings 把权限点列表转成字符串切片，用于写库与打印。
func Strings(ps []Permission) []string {
	out := make([]string, len(ps))
	for i, p := range ps {
		out[i] = string(p)
	}
	return out
}
```

- [ ] **Step 4: 跑测试确认通过**

```bash
cd server; go test ./internal/perm/ -v 2>&1 | tail -20; echo "退出码=$?"
```

预期：全部 PASS。

- [ ] **Step 5: 检查格式并提交**

```bash
cd server; gofmt -l . ; go vet ./internal/perm/; echo "退出码=$?"
git add server/internal/perm/
git commit -m "$(cat <<'EOF'
feat: 新增权限点定义

授权单位是「账号-直播间」绑定，不设固定角色。角色只是权限点的
预设组合，展开留给界面层，存储层只认权限点。

Co-Authored-By: Claude Opus 5 <noreply@anthropic.com>
EOF
)"
```

---

## Task 2: 把规则的序列化表示提取到 spec 包

P3 之后有三个通道需要同一份规则表示：YAML 导入、数据库 JSONB、P4 的 HTTP API。现在这套线上格式是 `internal/rules/config` 的私有类型，不提取出来，三处必然长出不同的字段名与默认值。

**这是一次纯搬家，不改行为。** `internal/rules/config` 的现有测试必须一行不改地继续全部通过——这是「没改行为」的唯一证明。

**Files:**
- Create: `server/internal/rules/spec/duration.go`
- Create: `server/internal/rules/spec/spec.go`
- Create: `server/internal/rules/spec/convert.go`
- Create: `server/internal/rules/spec/spec_test.go`
- Modify: `server/internal/rules/config/config.go`（删除线上格式类型与转换函数，改为委托 spec）
- 不修改：`server/internal/rules/config/config_test.go`、`example_test.go`

**Interfaces:**
- Consumes: `rules.Rule`、`rules.Condition`、`rules.Action`、`rules.AggregateSpec`（`internal/rules/rule.go`）；`event.Type`；`scheduler.ValidateSpec`
- Produces:
  - `spec.Duration`（底层 `time.Duration`），实现 `UnmarshalYAML`/`MarshalYAML`/`UnmarshalJSON`/`MarshalJSON`，四者都用 `"1.5s"` 形式
  - `spec.Config{Accounts []Account}`、`spec.Account{Name, CookieFile string; RateLimit Duration; MaxLength int; Rooms []Room}`、`spec.Room{ID string; CooldownGroups map[string]Duration; Rules []Rule}`
  - `spec.Rule`、`spec.Condition`、`spec.Aggregate`、`spec.Action`（全部带 `yaml` 与 `json` 标签）
  - `func (r Rule) ToRule() (rules.Rule, error)`
  - `func (c Condition) ToCondition() (rules.Condition, error)`

### 现状与目标

现在 `config.go` 里有两组类型：

| 组 | 类型 | 去向 |
|---|---|---|
| 领域结构 | `Config`、`Account`、`Room`、`Binding`、`Duration` | **留在 config 包**，形状一个字节都不能变 |
| 线上格式 | `configYAML`、`accountYAML`、`roomYAML`、`ruleYAML`、`conditionYAML`、`aggregateYAML`、`actionYAML` | 搬到 spec，改名去掉 YAML 后缀，加 json 标签 |
| 转换与查表 | `convertRule`、`convertCondition`、`normalizeOp`、`opAliases`、`knownEventTypes` | 搬到 spec |

`config.Duration` 改成 `type Duration = spec.Duration` 的**类型别名**（不是新类型），这样 `config_test.go` 里的 `time.Duration(c.Accounts[0].RateLimit)` 照旧编译。

结构层面的校验（账号名重复、缺 `cookieFile`、规则名在绑定内重复）**留在 `config.Parse`**——那是 YAML 树的形状问题。单条规则的校验搬进 `spec.Rule.ToRule()`。

- [ ] **Step 1: 写 spec 包的失败测试**

创建 `server/internal/rules/spec/spec_test.go`：

```go
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
```

- [ ] **Step 2: 跑测试确认失败**

```bash
cd server; go test ./internal/rules/spec/ 2>&1 | tail -5; echo "退出码=$?"
```

预期：`no Go files in .../internal/rules/spec`。

- [ ] **Step 3: 实现 Duration**

创建 `server/internal/rules/spec/duration.go`：

```go
package spec

import (
	"encoding/json"
	"fmt"
	"time"

	"gopkg.in/yaml.v3"
)

// Duration 是可读的时长，YAML 与 JSON 两侧都用 "1.5s"、"500ms"、"3m" 这种
// 字符串形式。
//
// 不接受裸数字：`window: 1500` 到底是毫秒还是纳秒，读的人猜不出来。
// JSONB 里也存字符串，是为了让人能直接看懂库里的行。
type Duration time.Duration

// UnmarshalYAML 解析形如 "1.5s" 的时长字符串。
func (d *Duration) UnmarshalYAML(n *yaml.Node) error {
	var s string
	if err := n.Decode(&s); err != nil {
		return fmt.Errorf("时长必须是字符串，如 \"1.5s\": %w", err)
	}
	return d.parse(s)
}

// MarshalYAML 输出形如 "1m30s" 的时长字符串。
func (d Duration) MarshalYAML() (any, error) {
	return time.Duration(d).String(), nil
}

// UnmarshalJSON 解析形如 "1.5s" 的时长字符串。
func (d *Duration) UnmarshalJSON(b []byte) error {
	var s string
	if err := json.Unmarshal(b, &s); err != nil {
		return fmt.Errorf("时长必须是字符串，如 \"1.5s\": %w", err)
	}
	return d.parse(s)
}

// MarshalJSON 输出形如 "1m30s" 的时长字符串。
func (d Duration) MarshalJSON() ([]byte, error) {
	return json.Marshal(time.Duration(d).String())
}

// parse 是两条反序列化路径共用的解析。
func (d *Duration) parse(s string) error {
	parsed, err := time.ParseDuration(s)
	if err != nil {
		return fmt.Errorf("非法的时长 %q: %w", s, err)
	}
	*d = Duration(parsed)
	return nil
}
```

- [ ] **Step 4: 实现线上格式类型**

创建 `server/internal/rules/spec/spec.go`：

```go
// Package spec 是规则的序列化表示，同时服务三个通道：
//
//	YAML 配置导入 ──┐
//	数据库 JSONB  ──┼──→ spec.Rule ──→ rules.Rule（领域模型）
//	HTTP API      ──┘
//
// 三处若各写一份，字段名与默认值必然漂移。本包是唯一的线上格式定义，
// 与 internal/rules 的领域模型严格分离：领域模型只管求值，本包只管
// 序列化与转换。
package spec

// Config 是完整的配置树，只在 YAML 导入路径上用。
// 数据库路径下账号、直播间、规则各自成表，用不到这一层。
type Config struct {
	Accounts []Account `yaml:"accounts" json:"accounts"`
}

// Account 是一个账号及其连接的全部直播间。
type Account struct {
	Name       string   `yaml:"name"       json:"name"`
	CookieFile string   `yaml:"cookieFile" json:"cookieFile"`
	RateLimit  Duration `yaml:"rateLimit"  json:"rateLimit"`
	MaxLength  int      `yaml:"maxLength"  json:"maxLength"`
	Rooms      []Room   `yaml:"rooms"      json:"rooms"`
}

// Room 是某账号在某直播间的配置，即一个绑定。
type Room struct {
	ID             string              `yaml:"id"             json:"id"`
	CooldownGroups map[string]Duration `yaml:"cooldownGroups" json:"cooldownGroups"`
	Rules          []Rule              `yaml:"rules"          json:"rules"`
}

// Rule 是一条规则的序列化形式。
//
// 存进数据库时 name 与 enabled 会被提到列上，JSONB 里不含这两个字段——
// 同一个值存两处必然漂移。见 store/rule.go。
type Rule struct {
	Name          string     `yaml:"name"          json:"name,omitempty"`
	Enabled       *bool      `yaml:"enabled"       json:"enabled,omitempty"` // 指针以区分「未写」与「写了 false」
	On            []string   `yaml:"on"            json:"on,omitempty"`
	Schedule      string     `yaml:"schedule"      json:"schedule,omitempty"`
	When          *Condition `yaml:"when"          json:"when,omitempty"`
	Aggregate     *Aggregate `yaml:"aggregate"     json:"aggregate,omitempty"`
	Cooldown      Duration   `yaml:"cooldown"      json:"cooldown,omitempty"`
	CooldownGroup string     `yaml:"cooldownGroup" json:"cooldownGroup,omitempty"`
	Do            []Action   `yaml:"do"            json:"do,omitempty"`
}

// Condition 是条件树的一个节点。
type Condition struct {
	Field  string      `yaml:"field"  json:"field,omitempty"`
	Op     string      `yaml:"op"     json:"op,omitempty"`
	Value  any         `yaml:"value"  json:"value,omitempty"`
	All    []Condition `yaml:"all"    json:"all,omitempty"`
	Any    []Condition `yaml:"any"    json:"any,omitempty"`
	Not    *Condition  `yaml:"not"    json:"not,omitempty"`
	Script string      `yaml:"script" json:"script,omitempty"`
}

// Aggregate 是合并窗口的序列化形式。
type Aggregate struct {
	Window   Duration `yaml:"window"   json:"window"`
	MaxWait  Duration `yaml:"maxWait"  json:"maxWait,omitempty"`
	MinCount int      `yaml:"minCount" json:"minCount,omitempty"`
	By       string   `yaml:"by"       json:"by"`
}

// Action 是一个动作的序列化形式。
type Action struct {
	Type     string   `yaml:"type"     json:"type"`
	Template []string `yaml:"template" json:"template,omitempty"`
	Hours    int      `yaml:"hours"    json:"hours,omitempty"`
	Script   string   `yaml:"script"   json:"script,omitempty"`
}
```

- [ ] **Step 5: 实现转换**

创建 `server/internal/rules/spec/convert.go`。内容整体搬自 `config.go` 的 `convertRule`／`convertCondition`／`normalizeOp`／`opAliases`／`knownEventTypes`，只改类型名与接收者：

```go
package spec

import (
	"fmt"
	"regexp"
	"time"

	"github.com/MrZhongzq/MagicalDanmaku/server/internal/event"
	"github.com/MrZhongzq/MagicalDanmaku/server/internal/rules"
	"github.com/MrZhongzq/MagicalDanmaku/server/internal/scheduler"
)

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

// ToRule 把序列化形式转成领域模型并校验。
//
// 校验在这里做完，非法规则不允许进入运行期——配置写错却静默忽略，
// 比直接报错更难排查。
func (r Rule) ToRule() (rules.Rule, error) {
	out := rules.Rule{
		Name:          r.Name,
		Enabled:       true, // 未写 enabled 时默认启用：写了规则却不生效最反直觉
		Schedule:      r.Schedule,
		Cooldown:      time.Duration(r.Cooldown),
		CooldownGroup: r.CooldownGroup,
	}
	if r.Enabled != nil {
		out.Enabled = *r.Enabled
	}

	for _, name := range r.On {
		t, ok := knownEventTypes[name]
		if !ok {
			return out, fmt.Errorf("未知的事件类型 %q", name)
		}
		out.On = append(out.On, t)
	}

	if r.Schedule != "" {
		if err := scheduler.ValidateSpec(r.Schedule); err != nil {
			return out, err
		}
	}

	if r.When != nil {
		c, err := r.When.ToCondition()
		if err != nil {
			return out, err
		}
		out.When = &c
	}

	if r.Aggregate != nil {
		out.Aggregate = &rules.AggregateSpec{
			Window:   time.Duration(r.Aggregate.Window),
			MaxWait:  time.Duration(r.Aggregate.MaxWait),
			MinCount: r.Aggregate.MinCount,
			By:       rules.AggregateBy(r.Aggregate.By),
		}
	}

	for _, a := range r.Do {
		out.Do = append(out.Do, rules.Action{
			Type:     rules.ActionType(a.Type),
			Template: a.Template,
			Hours:    a.Hours,
			Script:   a.Script,
		})
	}

	if err := out.Validate(); err != nil {
		return out, err
	}
	return out, nil
}

// ToCondition 递归转换条件并做正则预编译校验。
func (c Condition) ToCondition() (rules.Condition, error) {
	out := rules.Condition{
		Field:  c.Field,
		Op:     normalizeOp(c.Op),
		Value:  c.Value,
		Script: c.Script,
	}

	// 正则在转换时就编译一次，把错误从运行时提前到启动时
	if out.Op == "regex" {
		pattern, _ := c.Value.(string)
		if _, err := regexp.Compile(pattern); err != nil {
			return out, fmt.Errorf("非法的正则表达式 %q: %w", pattern, err)
		}
	}

	for _, sub := range c.All {
		s, err := sub.ToCondition()
		if err != nil {
			return out, err
		}
		out.All = append(out.All, s)
	}
	for _, sub := range c.Any {
		s, err := sub.ToCondition()
		if err != nil {
			return out, err
		}
		out.Any = append(out.Any, s)
	}
	if c.Not != nil {
		s, err := c.Not.ToCondition()
		if err != nil {
			return out, err
		}
		out.Not = &s
	}
	return out, nil
}

// normalizeOp 把符号别名归一化为内部枚举名。
func normalizeOp(op string) string {
	if alias, ok := opAliases[op]; ok {
		return alias
	}
	return op
}
```

- [ ] **Step 6: 跑 spec 测试确认通过**

```bash
cd server; go test ./internal/rules/spec/ -v 2>&1 | tail -30; echo "退出码=$?"
```

预期：全部 PASS。

- [ ] **Step 7: 让 config 包委托给 spec**

改写 `server/internal/rules/config/config.go`。删掉 `Duration` 的定义与全部 `*YAML` 类型、`convertRule`、`convertCondition`、`normalizeOp`、`opAliases`、`knownEventTypes`，替换为：

```go
// Package config 负责从 YAML 加载并校验规则配置。
//
// 配置结构与运行模型同构，三层嵌套：账号 → 直播间 → 规则。
// 每个「账号-直播间」组合是一个独立的运行单元（绑定）。
//
// 线上格式与规则转换住在 internal/rules/spec，本包只做两件事：
// 读文件，以及校验配置树的形状（账号名重复、缺 cookieFile 之类）。
// P3 之后 YAML 降级为导入入口，运行期配置从数据库读。
package config

import (
	"fmt"
	"os"
	"time"

	"gopkg.in/yaml.v3"

	"github.com/MrZhongzq/MagicalDanmaku/server/internal/rules"
	"github.com/MrZhongzq/MagicalDanmaku/server/internal/rules/spec"
)

// Duration 是 spec.Duration 的别名。别名而非新类型：两处各定义一遍
// 解析逻辑，迟早会在某个边角上不一致。
type Duration = spec.Duration

// Config 是完整配置。
type Config struct {
	Accounts []Account
}

// Account 是一个已登录账号及其连接的全部直播间。
type Account struct {
	Name       string
	CookieFile string
	RateLimit  Duration // 该账号全部直播间共享的发送间隔
	MaxLength  int      // 单条弹幕字符上限，0 表示用默认值
	Rooms      []Room
}

// Room 是某个账号在某个直播间的配置，即一个绑定。
type Room struct {
	ID             string
	CooldownGroups map[string]Duration
	Rules          []rules.Rule
}

// Binding 是摊平后的运行单元。
type Binding struct {
	AccountName    string
	CookieFile     string
	RateLimit      time.Duration
	MaxLength      int
	RoomID         string
	CooldownGroups map[string]time.Duration
	Rules          []rules.Rule
}

// Bindings 把三层结构摊平成运行单元列表，保持配置顺序。
func (c *Config) Bindings() []Binding {
	var out []Binding
	for _, a := range c.Accounts {
		for _, r := range a.Rooms {
			groups := make(map[string]time.Duration, len(r.CooldownGroups))
			for k, v := range r.CooldownGroups {
				groups[k] = time.Duration(v)
			}
			out = append(out, Binding{
				AccountName:    a.Name,
				CookieFile:     a.CookieFile,
				RateLimit:      time.Duration(a.RateLimit),
				MaxLength:      a.MaxLength,
				RoomID:         r.ID,
				CooldownGroups: groups,
				Rules:          r.Rules,
			})
		}
	}
	return out
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
//
// 单条规则的校验在 spec.Rule.ToRule 里，这里只管配置树的形状。
func Parse(data []byte) (*Config, error) {
	var raw spec.Config
	if err := yaml.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("YAML 语法错误: %w", err)
	}
	if len(raw.Accounts) == 0 {
		return nil, fmt.Errorf("配置中没有任何账号")
	}

	c := &Config{}
	seenAccount := make(map[string]bool, len(raw.Accounts))

	for i, ay := range raw.Accounts {
		if ay.Name == "" {
			return nil, fmt.Errorf("第 %d 个账号缺少 name", i+1)
		}
		if seenAccount[ay.Name] {
			return nil, fmt.Errorf("账号名 %q 重复", ay.Name)
		}
		seenAccount[ay.Name] = true

		if ay.CookieFile == "" {
			return nil, fmt.Errorf("账号 %q 缺少 cookieFile", ay.Name)
		}
		if len(ay.Rooms) == 0 {
			return nil, fmt.Errorf("账号 %q 未配置任何直播间", ay.Name)
		}

		acc := Account{
			Name:       ay.Name,
			CookieFile: ay.CookieFile,
			RateLimit:  ay.RateLimit,
			MaxLength:  ay.MaxLength,
		}

		seenRoom := make(map[string]bool, len(ay.Rooms))
		for j, ry := range ay.Rooms {
			if ry.ID == "" {
				return nil, fmt.Errorf("账号 %q 的第 %d 个直播间缺少 id", ay.Name, j+1)
			}
			if seenRoom[ry.ID] {
				return nil, fmt.Errorf("账号 %q 下的直播间 %s 重复配置", ay.Name, ry.ID)
			}
			seenRoom[ry.ID] = true

			room := Room{ID: ry.ID, CooldownGroups: ry.CooldownGroups}

			// 规则名只需在单个绑定内唯一——同一条「进场欢迎」本来就会
			// 出现在多个绑定下。冷却按规则名记录，同绑定内重名会互相干扰。
			seenRule := make(map[string]bool, len(ry.Rules))
			for k, rl := range ry.Rules {
				r, err := rl.ToRule()
				if err != nil {
					return nil, fmt.Errorf("账号 %q 的直播间 %s 第 %d 条规则(%s)非法: %w",
						ay.Name, ry.ID, k+1, rl.Name, err)
				}
				if seenRule[r.Name] {
					return nil, fmt.Errorf("账号 %q 的直播间 %s 下规则名 %q 重复",
						ay.Name, ry.ID, r.Name)
				}
				seenRule[r.Name] = true
				room.Rules = append(room.Rules, r)
			}

			acc.Rooms = append(acc.Rooms, room)
		}
		c.Accounts = append(c.Accounts, acc)
	}
	return c, nil
}
```

- [ ] **Step 8: 跑 config 的旧测试，一行都不许改**

```bash
cd server; go test ./internal/rules/config/ -v 2>&1 | tail -40; echo "退出码=$?"
```

预期：**全部 PASS**，且 `git diff --stat` 显示 `config_test.go` 与 `example_test.go` 均未改动。

若某个测试挂了，**先确认是搬家搬错了还是行为真变了**，不要改测试来迁就实现——这些测试正是「没改行为」的证明。

- [ ] **Step 9: 跑全量测试与静态检查**

```bash
cd server; go build ./... ; go vet ./... ; gofmt -l . ; go test ./... 2>&1 | tail -25; echo "退出码=$?"
git diff --stat -- internal/rules/config/config_test.go internal/rules/config/example_test.go; echo "退出码=$?"
```

预期：全部 PASS；后一条命令**无任何输出**（两个测试文件零改动）。

- [ ] **Step 10: 提交**

```bash
git add server/internal/rules/spec/ server/internal/rules/config/
git commit -m "$(cat <<'EOF'
refactor: 把规则的序列化表示提取到 spec 包

P3 之后 YAML 导入、数据库 JSONB、未来的 HTTP API 需要共用同一份
规则表示。三处各写一份，字段名与默认值必然漂移——这是 P0 里
「全项目只允许有一个字段展开处」那条教训的同类问题。

spec.Duration 同时实现 YAML 与 JSON 两侧的 marshal，两边都用
"3m" 这种可读形式；JSONB 里存字符串而非纳秒整数，是为了让人能
直接看懂库里的行。

config 包退化为薄加载层，只负责读文件与校验配置树的形状。
纯搬家不改行为，config 的现有测试一行未改且全部通过。

Co-Authored-By: Claude Opus 5 <noreply@anthropic.com>
EOF
)"
```

---

## Task 3: 存储基座与迁移器

**Files:**
- Create: `docker-compose.dev.yml`（仓库根目录）
- Create: `server/internal/store/store.go`
- Create: `server/internal/store/migrate.go`
- Create: `server/internal/store/migrations/001_init.sql`
- Create: `server/internal/store/testhelp_test.go`
- Create: `server/internal/store/migrate_test.go`
- Modify: `server/go.mod`、`server/go.sum`

**Interfaces:**
- Consumes: 无
- Produces:
  - `func store.Open(ctx context.Context, dsn string) (*store.Store, error)`
  - `func (s *Store) Close()`
  - `func (s *Store) Pool() *pgxpool.Pool`（仅供同包内与测试使用）
  - `func (s *Store) Migrate(ctx context.Context) error`
  - `func (s *Store) SchemaVersion(ctx context.Context) (int, error)`
  - `store.ErrSchemaOutdated`（哨兵错误）
  - 测试基座 `func testStore(t *testing.T) *Store`（`testhelp_test.go`，同包内可用）

- [ ] **Step 1: 加依赖并写本地开发数据库**

```bash
cd server; go get github.com/jackc/pgx/v5@latest; echo "退出码=$?"
cd server; go mod tidy; echo "退出码=$?"
```

确认 pgx 是纯 Go（它是，不依赖 libpq）：

```bash
cd server; CGO_ENABLED=0 GOOS=darwin GOARCH=arm64 go build ./... ; echo "退出码=$?"
```

创建仓库根目录的 `docker-compose.dev.yml`：

```yaml
# 本地开发与测试用的 PostgreSQL。
#
#   docker compose -f docker-compose.dev.yml up -d
#   export MAGICD_TEST_DATABASE_URL='postgres://magicd:magicd@localhost:5433/magicd?sslmode=disable'
#   cd server && go test ./internal/store/
#
# 端口用 5433 而非 5432，避开本机可能已在跑的 PostgreSQL。
services:
  postgres:
    image: postgres:16-alpine
    environment:
      POSTGRES_USER: magicd
      POSTGRES_PASSWORD: magicd
      POSTGRES_DB: magicd
    ports:
      - "5433:5432"
    healthcheck:
      test: ["CMD-SHELL", "pg_isready -U magicd"]
      interval: 2s
      timeout: 3s
      retries: 15
```

- [ ] **Step 2: 写建表 SQL**

创建 `server/internal/store/migrations/001_init.sql`，内容与设计文档 §3.2 一字不差：

```sql
CREATE TABLE users (
    id            BIGSERIAL PRIMARY KEY,
    username      TEXT        NOT NULL UNIQUE,
    password_hash TEXT        NOT NULL,
    is_admin      BOOLEAN     NOT NULL DEFAULT FALSE,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE accounts (
    id            BIGSERIAL PRIMARY KEY,
    name          TEXT        NOT NULL UNIQUE,
    uid           TEXT        NOT NULL DEFAULT '',
    cookie        TEXT        NOT NULL,
    rate_limit_ms INTEGER     NOT NULL DEFAULT 1500,
    max_length    INTEGER     NOT NULL DEFAULT 40,
    owner_id      BIGINT      NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE bindings (
    id         BIGSERIAL PRIMARY KEY,
    account_id BIGINT      NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
    room_id    TEXT        NOT NULL,
    enabled    BOOLEAN     NOT NULL DEFAULT TRUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (account_id, room_id)
);

CREATE TABLE memberships (
    id          BIGSERIAL PRIMARY KEY,
    user_id     BIGINT      NOT NULL REFERENCES users(id)    ON DELETE CASCADE,
    binding_id  BIGINT      NOT NULL REFERENCES bindings(id) ON DELETE CASCADE,
    permissions TEXT[]      NOT NULL DEFAULT '{}',
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (user_id, binding_id)
);
-- 注意：查询这个数组必须写成 permissions @> ARRAY['rule:write']。
-- 写成 'rule:write' = ANY(permissions) 语义相同，但那是逐行的数组展开，
-- PostgreSQL 不会改写成可索引形式，本索引对它完全不起作用（实测 20 万行
-- 时前者走 Bitmap Index Scan，后者 Parallel Seq Scan 扫完全表）。
CREATE INDEX memberships_permissions_idx ON memberships USING GIN (permissions);

CREATE TABLE rules (
    id         BIGSERIAL PRIMARY KEY,
    binding_id BIGINT      NOT NULL REFERENCES bindings(id) ON DELETE CASCADE,
    name       TEXT        NOT NULL,
    enabled    BOOLEAN     NOT NULL DEFAULT TRUE,
    position   INTEGER     NOT NULL DEFAULT 0,
    spec       JSONB       NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (binding_id, name)
);

CREATE TABLE cooldown_groups (
    id          BIGSERIAL PRIMARY KEY,
    binding_id  BIGINT  NOT NULL REFERENCES bindings(id) ON DELETE CASCADE,
    name        TEXT    NOT NULL,
    interval_ms INTEGER NOT NULL,
    UNIQUE (binding_id, name)
);

CREATE TABLE kv_store (
    binding_id BIGINT      NOT NULL REFERENCES bindings(id) ON DELETE CASCADE,
    key        TEXT        NOT NULL,
    value      TEXT        NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (binding_id, key)
);

CREATE TABLE block_list (
    id         BIGSERIAL PRIMARY KEY,
    binding_id BIGINT      NOT NULL REFERENCES bindings(id) ON DELETE CASCADE,
    uid        TEXT        NOT NULL,
    username   TEXT        NOT NULL DEFAULT '',
    reason     TEXT        NOT NULL DEFAULT '',
    created_by BIGINT      REFERENCES users(id) ON DELETE SET NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (binding_id, uid)
);

CREATE TABLE activity_logs (
    id          BIGSERIAL PRIMARY KEY,
    account_id  BIGINT      NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
    binding_id  BIGINT      REFERENCES bindings(id) ON DELETE SET NULL,
    room_id     TEXT        NOT NULL DEFAULT '',
    kind        TEXT        NOT NULL,
    event_type  TEXT        NOT NULL DEFAULT '',
    action_type TEXT        NOT NULL DEFAULT '',
    rule_name   TEXT        NOT NULL DEFAULT '',
    user_uid    TEXT        NOT NULL DEFAULT '',
    user_name   TEXT        NOT NULL DEFAULT '',
    detail      JSONB,
    occurred_at TIMESTAMPTZ NOT NULL
);
CREATE INDEX activity_logs_account_time_idx ON activity_logs (account_id, occurred_at DESC);
CREATE INDEX activity_logs_binding_time_idx ON activity_logs (binding_id, occurred_at DESC);
CREATE INDEX activity_logs_type_time_idx    ON activity_logs (event_type, occurred_at DESC);
```

- [ ] **Step 3: 写测试基座与迁移器的失败测试**

创建 `server/internal/store/testhelp_test.go`：

```go
package store

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
)

// testStore 为每个测试建一个独立 schema，测完 drop。
//
// 独立 schema 而非独立数据库：建库要连 postgres 库、慢且需要额外权限，
// 而 schema 是同库内的命名空间，建删都是毫秒级，且天然支持并行。
func testStore(t *testing.T) *Store {
	t.Helper()

	dsn := os.Getenv("MAGICD_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("未设置 MAGICD_TEST_DATABASE_URL，跳过存储层测试。\n" +
			"本地起库：docker compose -f docker-compose.dev.yml up -d\n" +
			"然后：export MAGICD_TEST_DATABASE_URL='postgres://magicd:magicd@localhost:5433/magicd?sslmode=disable'")
	}

	schema := schemaNameFor(t.Name())
	ctx := context.Background()

	// 先用默认 search_path 建 schema
	admin, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatalf("连接数据库失败: %v", err)
	}
	if _, err := admin.Exec(ctx, `DROP SCHEMA IF EXISTS `+schema+` CASCADE`); err != nil {
		admin.Close()
		t.Fatalf("清理旧 schema 失败: %v", err)
	}
	if _, err := admin.Exec(ctx, `CREATE SCHEMA `+schema); err != nil {
		admin.Close()
		t.Fatalf("创建 schema 失败: %v", err)
	}

	s, err := openWithSchema(ctx, dsn, schema)
	if err != nil {
		admin.Close()
		t.Fatalf("打开存储失败: %v", err)
	}
	if err := s.Migrate(ctx); err != nil {
		s.Close()
		admin.Close()
		t.Fatalf("迁移失败: %v", err)
	}

	t.Cleanup(func() {
		s.Close()
		if _, err := admin.Exec(context.Background(),
			`DROP SCHEMA IF EXISTS `+schema+` CASCADE`); err != nil {
			t.Logf("清理 schema %s 失败: %v", schema, err)
		}
		admin.Close()
	})
	return s
}

// schemaNameFor 把测试名转成合法的 PostgreSQL 标识符。
//
// t.Name() 含斜杠与中文，且 PG 标识符上限 63 字节，都要处理。
func schemaNameFor(name string) string {
	var b strings.Builder
	b.WriteString("t_")
	for _, r := range name {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			b.WriteRune(r)
		case r >= 'A' && r <= 'Z':
			b.WriteRune(r + 32)
		default:
			b.WriteByte('_')
		}
	}
	s := b.String()
	if len(s) > 50 {
		// 截断会撞名，补一段长度做区分
		s = fmt.Sprintf("%s_%d", s[:44], len(name))
	}
	return s
}
```

创建 `server/internal/store/migrate_test.go`：

```go
package store

import (
	"context"
	"testing"
)

func TestMigrateCreatesAllTables(t *testing.T) {
	s := testStore(t) // testStore 内部已跑过 Migrate
	ctx := context.Background()

	want := []string{
		"users", "accounts", "bindings", "memberships", "rules",
		"cooldown_groups", "kv_store", "block_list", "activity_logs",
		"schema_migrations",
	}
	for _, table := range want {
		var exists bool
		err := s.pool.QueryRow(ctx, `
			SELECT EXISTS (
				SELECT 1 FROM information_schema.tables
				WHERE table_schema = current_schema() AND table_name = $1
			)`, table).Scan(&exists)
		if err != nil {
			t.Fatalf("查询表 %s 是否存在时报错: %v", table, err)
		}
		if !exists {
			t.Errorf("表 %s 未被创建", table)
		}
	}
}

func TestMigrateIsIdempotent(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()

	// 再跑一次不应报错，也不应重复执行
	if err := s.Migrate(ctx); err != nil {
		t.Fatalf("重复迁移报错: %v", err)
	}

	var n int
	if err := s.pool.QueryRow(ctx, `SELECT count(*) FROM schema_migrations`).Scan(&n); err != nil {
		t.Fatalf("查询迁移记录报错: %v", err)
	}
	if n != 1 {
		t.Errorf("迁移记录数 = %d, 期望 1", n)
	}
}

func TestSchemaVersionReportsLatest(t *testing.T) {
	s := testStore(t)
	v, err := s.SchemaVersion(context.Background())
	if err != nil {
		t.Fatalf("SchemaVersion 报错: %v", err)
	}
	if v != 1 {
		t.Errorf("版本 = %d, 期望 1", v)
	}
}

func TestSchemaVersionOnEmptyDatabaseIsZero(t *testing.T) {
	// 未迁移过的库里没有 schema_migrations 表，应返回 0 而非报错
	s := testStore(t)
	ctx := context.Background()

	if _, err := s.pool.Exec(ctx, `DROP TABLE schema_migrations`); err != nil {
		t.Fatalf("删表报错: %v", err)
	}
	v, err := s.SchemaVersion(ctx)
	if err != nil {
		t.Fatalf("SchemaVersion 报错: %v", err)
	}
	if v != 0 {
		t.Errorf("版本 = %d, 期望 0", v)
	}
}

func TestForeignKeyOwnerIsRestrictNotCascade(t *testing.T) {
	// 删掉一个还拥有账号的用户应当报错，而不是留下无主的 Cookie
	s := testStore(t)
	ctx := context.Background()

	var userID int64
	err := s.pool.QueryRow(ctx, `
		INSERT INTO users (username, password_hash) VALUES ('张三', 'x') RETURNING id`).Scan(&userID)
	if err != nil {
		t.Fatalf("建用户报错: %v", err)
	}
	_, err = s.pool.Exec(ctx, `
		INSERT INTO accounts (name, cookie, owner_id) VALUES ('主播号', 'c', $1)`, userID)
	if err != nil {
		t.Fatalf("建账号报错: %v", err)
	}

	if _, err := s.pool.Exec(ctx, `DELETE FROM users WHERE id = $1`, userID); err == nil {
		t.Error("删除仍拥有账号的用户应被外键拒绝")
	}
}

func TestForeignKeyBindingChildrenCascade(t *testing.T) {
	// 删绑定应带走它的规则、冷却组、KV 与禁言名单
	s := testStore(t)
	ctx := context.Background()

	var userID, accountID, bindingID int64
	if err := s.pool.QueryRow(ctx, `
		INSERT INTO users (username, password_hash) VALUES ('张三', 'x') RETURNING id`).Scan(&userID); err != nil {
		t.Fatalf("建用户报错: %v", err)
	}
	if err := s.pool.QueryRow(ctx, `
		INSERT INTO accounts (name, cookie, owner_id) VALUES ('主播号', 'c', $1) RETURNING id`,
		userID).Scan(&accountID); err != nil {
		t.Fatalf("建账号报错: %v", err)
	}
	if err := s.pool.QueryRow(ctx, `
		INSERT INTO bindings (account_id, room_id) VALUES ($1, '123') RETURNING id`,
		accountID).Scan(&bindingID); err != nil {
		t.Fatalf("建绑定报错: %v", err)
	}
	if _, err := s.pool.Exec(ctx, `
		INSERT INTO rules (binding_id, name, spec) VALUES ($1, 'r', '{}')`, bindingID); err != nil {
		t.Fatalf("建规则报错: %v", err)
	}

	if _, err := s.pool.Exec(ctx, `DELETE FROM bindings WHERE id = $1`, bindingID); err != nil {
		t.Fatalf("删绑定报错: %v", err)
	}

	var n int
	if err := s.pool.QueryRow(ctx, `SELECT count(*) FROM rules`).Scan(&n); err != nil {
		t.Fatalf("查规则数报错: %v", err)
	}
	if n != 0 {
		t.Errorf("绑定删除后规则应被级联删除，实际剩 %d 条", n)
	}
}
```

- [ ] **Step 4: 跑测试确认失败**

```bash
cd server; go test ./internal/store/ 2>&1 | tail -5; echo "退出码=$?"
```

预期：编译失败（`Store` 未定义）。

- [ ] **Step 5: 实现 Store**

创建 `server/internal/store/store.go`：

```go
// Package store 是 PostgreSQL 存储层。
//
// 只有这一个实现，因此不做接口抽象——为不存在的第二实现提前抽接口，
// 只会换来一层无人受益的间接。需要替身的上层测试请在自己的测试文件里
// 定义所需的最小接口，Go 的隐式接口正是为此而生。
package store

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Store 持有连接池。并发安全，全进程共用一个即可。
type Store struct {
	pool *pgxpool.Pool
}

// Open 连接数据库。dsn 形如
// postgres://user:pass@host:5432/dbname?sslmode=disable
//
// 注意 Cookie 以明文存储：若数据库不在本机且未启用 TLS，
// Cookie 每次读取都会明文过网络。
func Open(ctx context.Context, dsn string) (*Store, error) {
	return openWithSchema(ctx, dsn, "")
}

// openWithSchema 连接数据库并可指定 schema。schema 为空则用连接默认值。
//
// 指定 schema 的能力只服务于测试隔离：每个测试用例在自己的 schema 里
// 建表，测完整个 drop 掉，因此可以并行且互不干扰。
func openWithSchema(ctx context.Context, dsn, schema string) (*Store, error) {
	cfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		return nil, fmt.Errorf("store: 数据库连接串非法: %w", err)
	}
	if schema != "" {
		cfg.ConnConfig.RuntimeParams["search_path"] = schema
	}

	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("store: 连接数据库失败: %w", err)
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("store: 数据库不可达: %w", err)
	}
	return &Store{pool: pool}, nil
}

// Close 关闭连接池。
func (s *Store) Close() {
	s.pool.Close()
}
```

- [ ] **Step 6: 实现迁移器**

创建 `server/internal/store/migrate.go`：

```go
package store

import (
	"context"
	"embed"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/jackc/pgx/v5"
)

//go:embed migrations/*.sql
var migrationFS embed.FS

// migrateLockKey 是迁移用的 advisory lock 键，取 "magi" 的 ASCII。
// 值本身不重要，只要全项目一致。
const migrateLockKey int64 = 0x6d616769

// ErrSchemaOutdated 表示数据库 schema 版本落后于二进制。
//
// run 遇到它应拒绝启动而非自动迁移：多实例部署下，让每个实例
// 各自决定何时改表是危险的。
var ErrSchemaOutdated = errors.New("store: 数据库 schema 版本落后，请先运行 magicd migrate")

// migration 是一个待执行的迁移文件。
type migration struct {
	version int
	name    string
	sql     string
}

// Migrate 把 schema 升到最新版本。
//
// 只做前向迁移，不实现回滚：回滚脚本在实践中几乎从不被执行，
// 却要一直维护；真出问题时恢复备份比跑回滚脚本可靠。
func (s *Store) Migrate(ctx context.Context) error {
	ms, err := loadMigrations()
	if err != nil {
		return err
	}

	// 迁移期间独占一条连接：advisory lock 是会话级的，
	// 用连接池会把 lock 和 unlock 发到两条不同的连接上。
	conn, err := s.pool.Acquire(ctx)
	if err != nil {
		return fmt.Errorf("store: 获取连接失败: %w", err)
	}
	defer conn.Release()

	// 防止多个实例同时启动时并发建表
	if _, err := conn.Exec(ctx, `SELECT pg_advisory_lock($1)`, migrateLockKey); err != nil {
		return fmt.Errorf("store: 获取迁移锁失败: %w", err)
	}
	defer func() {
		_, _ = conn.Exec(context.WithoutCancel(ctx),
			`SELECT pg_advisory_unlock($1)`, migrateLockKey)
	}()

	if _, err := conn.Exec(ctx, `
		CREATE TABLE IF NOT EXISTS schema_migrations (
			version    INTEGER     PRIMARY KEY,
			name       TEXT        NOT NULL,
			applied_at TIMESTAMPTZ NOT NULL DEFAULT now()
		)`); err != nil {
		return fmt.Errorf("store: 创建迁移记录表失败: %w", err)
	}

	applied := make(map[int]bool)
	rows, err := conn.Query(ctx, `SELECT version FROM schema_migrations`)
	if err != nil {
		return fmt.Errorf("store: 读取迁移记录失败: %w", err)
	}
	for rows.Next() {
		var v int
		if err := rows.Scan(&v); err != nil {
			rows.Close()
			return fmt.Errorf("store: 读取迁移记录失败: %w", err)
		}
		applied[v] = true
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return fmt.Errorf("store: 读取迁移记录失败: %w", err)
	}

	for _, m := range ms {
		if applied[m.version] {
			continue
		}
		// 每个迁移单独一个事务：失败即整体回滚，不留半截 schema
		err := pgx.BeginFunc(ctx, conn, func(tx pgx.Tx) error {
			if _, err := tx.Exec(ctx, m.sql); err != nil {
				return err
			}
			_, err := tx.Exec(ctx,
				`INSERT INTO schema_migrations (version, name) VALUES ($1, $2)`,
				m.version, m.name)
			return err
		})
		if err != nil {
			return fmt.Errorf("store: 执行迁移 %03d_%s 失败: %w", m.version, m.name, err)
		}
	}
	return nil
}

// SchemaVersion 返回已应用的最高迁移版本。未迁移过的库返回 0。
//
// 先单独查一次表是否存在：PostgreSQL 在计划阶段就会因为表不存在而报错，
// 把存在性判断塞进同一条语句的 WHERE 里救不回来。两次往返换一个不靠
// 匹配错误信息文本的判断，值得。
func (s *Store) SchemaVersion(ctx context.Context) (int, error) {
	var exists bool
	if err := s.pool.QueryRow(ctx,
		`SELECT to_regclass(current_schema() || '.schema_migrations') IS NOT NULL`,
	).Scan(&exists); err != nil {
		return 0, fmt.Errorf("store: 检查迁移记录表失败: %w", err)
	}
	if !exists {
		return 0, nil
	}

	var v *int // 空表时 max() 返回 NULL
	if err := s.pool.QueryRow(ctx,
		`SELECT max(version) FROM schema_migrations`).Scan(&v); err != nil {
		return 0, fmt.Errorf("store: 查询 schema 版本失败: %w", err)
	}
	if v == nil {
		return 0, nil
	}
	return *v, nil
}

// LatestSchemaVersion 返回二进制里内置的最高迁移版本。
func LatestSchemaVersion() (int, error) {
	ms, err := loadMigrations()
	if err != nil {
		return 0, err
	}
	if len(ms) == 0 {
		return 0, nil
	}
	return ms[len(ms)-1].version, nil
}

// loadMigrations 读出内嵌的迁移文件并按版本号排序。
func loadMigrations() ([]migration, error) {
	entries, err := migrationFS.ReadDir("migrations")
	if err != nil {
		return nil, fmt.Errorf("store: 读取内嵌迁移失败: %w", err)
	}

	out := make([]migration, 0, len(entries))
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".sql") {
			continue
		}
		// 文件名形如 001_init.sql
		base := strings.TrimSuffix(e.Name(), ".sql")
		num, name, ok := strings.Cut(base, "_")
		if !ok {
			return nil, fmt.Errorf("store: 迁移文件名 %q 不合规，应形如 001_init.sql", e.Name())
		}
		v, err := strconv.Atoi(num)
		if err != nil {
			return nil, fmt.Errorf("store: 迁移文件名 %q 的版本号非法: %w", e.Name(), err)
		}
		data, err := migrationFS.ReadFile("migrations/" + e.Name())
		if err != nil {
			return nil, fmt.Errorf("store: 读取迁移 %s 失败: %w", e.Name(), err)
		}
		out = append(out, migration{version: v, name: name, sql: string(data)})
	}

	sort.Slice(out, func(i, j int) bool { return out[i].version < out[j].version })
	for i := range out {
		if out[i].version != i+1 {
			return nil, fmt.Errorf("store: 迁移版本号不连续，第 %d 个是 %d", i+1, out[i].version)
		}
	}
	return out, nil
}
```

- [ ] **Step 7: 起数据库并跑测试**

```bash
docker compose -f docker-compose.dev.yml up -d; echo "退出码=$?"
```

等健康检查通过后：

```bash
export MAGICD_TEST_DATABASE_URL='postgres://magicd:magicd@localhost:5433/magicd?sslmode=disable'
cd server; go test ./internal/store/ -v 2>&1 | tail -30; echo "退出码=$?"
```

预期：全部 PASS。

- [ ] **Step 8: 确认没有数据库时会跳过而非失败**

```bash
cd server; env -u MAGICD_TEST_DATABASE_URL go test ./internal/store/ -v 2>&1 | tail -20; echo "退出码=$?"
```

预期：全部 SKIP，退出码 0，且跳过信息里写明了怎么起本地库。

- [ ] **Step 9: 确认六平台交叉编译仍然通过**

```bash
cd server; for os in windows darwin linux; do for arch in amd64 arm64; do
  CGO_ENABLED=0 GOOS=$os GOARCH=$arch go build -o /dev/null ./... || echo "失败: $os/$arch"
done; done; echo "退出码=$?"
```

预期：无「失败」输出。

- [ ] **Step 10: 提交**

```bash
cd server; gofmt -l . ; go vet ./... ; echo "退出码=$?"
git add docker-compose.dev.yml server/internal/store/ server/go.mod server/go.sum
git commit -m "$(cat <<'EOF'
feat: 新增 PostgreSQL 存储基座与迁移器

迁移器自写而非引入 goose：需求很简单，一张 schema_migrations 表
加按序执行内嵌 SQL 即可。执行前取 advisory lock，防止多实例同时
启动时并发建表；每个迁移单独一个事务，失败即整体回滚。

只做前向迁移，不写回滚脚本——回滚脚本几乎从不被执行却要一直维护，
真出问题时恢复备份更可靠。

测试基座为每个用例建独立 schema，测完 drop，因此可并行。未设置
MAGICD_TEST_DATABASE_URL 时整包跳过，并提示如何起本地库。

Co-Authored-By: Claude Opus 5 <noreply@anthropic.com>
EOF
)"
```

---

## Task 4: 用户表与密码

**Files:**
- Create: `server/internal/store/user.go`
- Create: `server/internal/store/user_test.go`
- Modify: `server/internal/store/migrate.go`（`Migrate` 末尾调用 `ensureAdmin`）
- Modify: `server/go.mod`、`server/go.sum`（加 `golang.org/x/crypto`）

**Interfaces:**
- Consumes: `store.Store`、`store.Migrate`（Task 3）
- Produces:
  - `type store.User struct { ID int64; Username string; IsAdmin bool; CreatedAt, UpdatedAt time.Time }`
  - `func (s *Store) CreateUser(ctx context.Context, username, password string, isAdmin bool) (*User, error)`
  - `func (s *Store) GetUserByName(ctx context.Context, username string) (*User, error)`
  - `func (s *Store) ListUsers(ctx context.Context) ([]User, error)`
  - `func (s *Store) SetPassword(ctx context.Context, username, password string) error`
  - `func (s *Store) VerifyPassword(ctx context.Context, username, password string) (*User, error)`
  - `func (s *Store) CountUsers(ctx context.Context) (int, error)`
  - `func (s *Store) EnsureAdmin(ctx context.Context) (username, password string, created bool, err error)`
  - `store.ErrNotFound`、`store.ErrDuplicate`、`store.ErrBadCredentials`（哨兵错误）

- [ ] **Step 1: 加 bcrypt 依赖**

```bash
cd server; go get golang.org/x/crypto@latest; go mod tidy; echo "退出码=$?"
```

- [ ] **Step 2: 写失败的测试**

创建 `server/internal/store/user_test.go`：

```go
package store

import (
	"context"
	"errors"
	"strings"
	"testing"
)

func TestCreateAndGetUser(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()

	created, err := s.CreateUser(ctx, "张三", "hunter2", false)
	if err != nil {
		t.Fatalf("创建用户报错: %v", err)
	}
	if created.ID == 0 {
		t.Error("新用户应有非零 ID")
	}
	if created.IsAdmin {
		t.Error("isAdmin 传 false 时不应是管理员")
	}

	got, err := s.GetUserByName(ctx, "张三")
	if err != nil {
		t.Fatalf("查询用户报错: %v", err)
	}
	if got.ID != created.ID || got.Username != "张三" {
		t.Errorf("查到的用户 = %+v", got)
	}
}

func TestGetUserNotFound(t *testing.T) {
	s := testStore(t)
	_, err := s.GetUserByName(context.Background(), "不存在的人")
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("应返回 ErrNotFound，实际: %v", err)
	}
}

func TestCreateUserRejectsDuplicate(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()

	if _, err := s.CreateUser(ctx, "张三", "a", false); err != nil {
		t.Fatalf("首次创建报错: %v", err)
	}
	_, err := s.CreateUser(ctx, "张三", "b", false)
	if !errors.Is(err, ErrDuplicate) {
		t.Errorf("重名应返回 ErrDuplicate，实际: %v", err)
	}
}

func TestPasswordIsHashedNotStored(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()

	if _, err := s.CreateUser(ctx, "张三", "hunter2", false); err != nil {
		t.Fatalf("创建用户报错: %v", err)
	}

	var hash string
	if err := s.pool.QueryRow(ctx,
		`SELECT password_hash FROM users WHERE username = '张三'`).Scan(&hash); err != nil {
		t.Fatalf("读取哈希报错: %v", err)
	}
	if strings.Contains(hash, "hunter2") {
		t.Error("密码不得以任何形式明文出现在库里")
	}
	if !strings.HasPrefix(hash, "$2") {
		t.Errorf("应是 bcrypt 哈希，实际前缀: %.4s", hash)
	}
}

func TestVerifyPasswordAcceptsCorrect(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()

	if _, err := s.CreateUser(ctx, "张三", "hunter2", false); err != nil {
		t.Fatalf("创建用户报错: %v", err)
	}
	u, err := s.VerifyPassword(ctx, "张三", "hunter2")
	if err != nil {
		t.Fatalf("正确密码应通过: %v", err)
	}
	if u.Username != "张三" {
		t.Errorf("返回的用户 = %+v", u)
	}
}

func TestVerifyPasswordRejectsWrong(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()

	if _, err := s.CreateUser(ctx, "张三", "hunter2", false); err != nil {
		t.Fatalf("创建用户报错: %v", err)
	}
	_, err := s.VerifyPassword(ctx, "张三", "wrong")
	if !errors.Is(err, ErrBadCredentials) {
		t.Errorf("错误密码应返回 ErrBadCredentials，实际: %v", err)
	}
}

func TestVerifyPasswordHidesWhetherUserExists(t *testing.T) {
	// 用户名不存在与密码错误返回同一个错误，否则接口就成了用户名枚举器
	s := testStore(t)
	ctx := context.Background()

	if _, err := s.CreateUser(ctx, "张三", "hunter2", false); err != nil {
		t.Fatalf("创建用户报错: %v", err)
	}
	_, errNoUser := s.VerifyPassword(ctx, "不存在的人", "whatever")
	_, errBadPass := s.VerifyPassword(ctx, "张三", "wrong")

	if !errors.Is(errNoUser, ErrBadCredentials) {
		t.Errorf("用户不存在时也应返回 ErrBadCredentials，实际: %v", errNoUser)
	}
	if errNoUser.Error() != errBadPass.Error() {
		t.Errorf("两种失败的错误信息应完全一致:\n  无此人: %v\n  密码错: %v", errNoUser, errBadPass)
	}
}

func TestSetPasswordChangesIt(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()

	if _, err := s.CreateUser(ctx, "张三", "old", false); err != nil {
		t.Fatalf("创建用户报错: %v", err)
	}
	if err := s.SetPassword(ctx, "张三", "new"); err != nil {
		t.Fatalf("改密码报错: %v", err)
	}
	if _, err := s.VerifyPassword(ctx, "张三", "new"); err != nil {
		t.Errorf("新密码应通过: %v", err)
	}
	if _, err := s.VerifyPassword(ctx, "张三", "old"); !errors.Is(err, ErrBadCredentials) {
		t.Error("旧密码应失效")
	}
}

func TestSetPasswordOnMissingUser(t *testing.T) {
	s := testStore(t)
	err := s.SetPassword(context.Background(), "不存在的人", "x")
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("应返回 ErrNotFound，实际: %v", err)
	}
}

func TestCreateUserRejectsEmptyPassword(t *testing.T) {
	s := testStore(t)
	if _, err := s.CreateUser(context.Background(), "张三", "", false); err == nil {
		t.Error("空密码应被拒绝")
	}
}

func TestCreateUserRejectsEmptyUsername(t *testing.T) {
	s := testStore(t)
	if _, err := s.CreateUser(context.Background(), "  ", "x", false); err == nil {
		t.Error("空用户名应被拒绝")
	}
}

func TestListUsersOrderedByID(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()

	for _, n := range []string{"张三", "李四", "王五"} {
		if _, err := s.CreateUser(ctx, n, "x", false); err != nil {
			t.Fatalf("创建 %s 报错: %v", n, err)
		}
	}
	us, err := s.ListUsers(ctx)
	if err != nil {
		t.Fatalf("列出用户报错: %v", err)
	}
	if len(us) != 3 {
		t.Fatalf("用户数 = %d, 期望 3", len(us))
	}
	if us[0].Username != "张三" || us[2].Username != "王五" {
		t.Errorf("顺序不对: %v", []string{us[0].Username, us[1].Username, us[2].Username})
	}
}

func TestEnsureAdminCreatesOnEmptyDatabase(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()

	name, pass, created, err := s.EnsureAdmin(ctx)
	if err != nil {
		t.Fatalf("EnsureAdmin 报错: %v", err)
	}
	if !created {
		t.Fatal("空库上应创建管理员")
	}
	if name != "admin" {
		t.Errorf("管理员用户名 = %q, 期望 admin", name)
	}
	if len(pass) < 16 {
		t.Errorf("随机密码太短: %d 个字符", len(pass))
	}

	u, err := s.VerifyPassword(ctx, name, pass)
	if err != nil {
		t.Fatalf("打印出来的密码应当能登录: %v", err)
	}
	if !u.IsAdmin {
		t.Error("自动创建的应是管理员")
	}
}

func TestEnsureAdminSkipsWhenUsersExist(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()

	if _, err := s.CreateUser(ctx, "张三", "x", false); err != nil {
		t.Fatalf("创建用户报错: %v", err)
	}
	_, _, created, err := s.EnsureAdmin(ctx)
	if err != nil {
		t.Fatalf("EnsureAdmin 报错: %v", err)
	}
	if created {
		t.Error("已有用户时不应再创建管理员")
	}
}

func TestEnsureAdminPasswordsDiffer(t *testing.T) {
	// 随机密码必须真随机，写死的常量等于没有密码
	s1, s2 := testStore(t), testStore(t)
	ctx := context.Background()

	_, p1, _, err := s1.EnsureAdmin(ctx)
	if err != nil {
		t.Fatalf("第一次 EnsureAdmin 报错: %v", err)
	}
	_, p2, _, err := s2.EnsureAdmin(ctx)
	if err != nil {
		t.Fatalf("第二次 EnsureAdmin 报错: %v", err)
	}
	if p1 == p2 {
		t.Error("两次生成的随机密码相同")
	}
}
```

注意 `TestEnsureAdminPasswordsDiffer` 调了两次 `testStore(t)`——两次用的是同一个 `t.Name()`，schema 会撞。改成用子测试拿到不同的名字：

```go
func TestEnsureAdminPasswordsDiffer(t *testing.T) {
	var p1, p2 string
	t.Run("第一次", func(t *testing.T) {
		s := testStore(t)
		_, p, _, err := s.EnsureAdmin(context.Background())
		if err != nil {
			t.Fatalf("EnsureAdmin 报错: %v", err)
		}
		p1 = p
	})
	t.Run("第二次", func(t *testing.T) {
		s := testStore(t)
		_, p, _, err := s.EnsureAdmin(context.Background())
		if err != nil {
			t.Fatalf("EnsureAdmin 报错: %v", err)
		}
		p2 = p
	})
	if p1 == "" || p2 == "" {
		t.Skip("子测试被跳过（无数据库）")
	}
	if p1 == p2 {
		t.Error("两次生成的随机密码相同")
	}
}
```

写测试文件时用上面这个版本，不要用前一个。

- [ ] **Step 3: 跑测试确认失败**

```bash
export MAGICD_TEST_DATABASE_URL='postgres://magicd:magicd@localhost:5433/magicd?sslmode=disable'
cd server; go test ./internal/store/ -run TestCreateAndGetUser 2>&1 | tail -5; echo "退出码=$?"
```

预期：编译失败（`CreateUser` 未定义）。

- [ ] **Step 4: 实现**

创建 `server/internal/store/user.go`：

```go
package store

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"golang.org/x/crypto/bcrypt"
)

// 存储层的哨兵错误。
var (
	ErrNotFound       = errors.New("store: 记录不存在")
	ErrDuplicate      = errors.New("store: 记录已存在")
	ErrBadCredentials = errors.New("store: 用户名或密码错误")
)

// defaultAdminName 是首次迁移时自动创建的管理员用户名。
const defaultAdminName = "admin"

// User 是系统用户，即使用本软件的人。
//
// 与 Account（机器人操作的 B 站账号）是两回事：用户用密码登录管理
// 界面，账号用 Cookie 操作直播间。
type User struct {
	ID        int64
	Username  string
	IsAdmin   bool
	CreatedAt time.Time
	UpdatedAt time.Time
}

// CreateUser 创建用户。密码以 bcrypt 哈希存储。
func (s *Store) CreateUser(ctx context.Context, username, password string, isAdmin bool) (*User, error) {
	username = strings.TrimSpace(username)
	if username == "" {
		return nil, fmt.Errorf("store: 用户名不能为空")
	}
	if password == "" {
		return nil, fmt.Errorf("store: 密码不能为空")
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return nil, fmt.Errorf("store: 生成密码哈希失败: %w", err)
	}

	var u User
	err = s.pool.QueryRow(ctx, `
		INSERT INTO users (username, password_hash, is_admin)
		VALUES ($1, $2, $3)
		RETURNING id, username, is_admin, created_at, updated_at`,
		username, string(hash), isAdmin,
	).Scan(&u.ID, &u.Username, &u.IsAdmin, &u.CreatedAt, &u.UpdatedAt)
	if err != nil {
		if isUniqueViolation(err) {
			return nil, fmt.Errorf("store: 用户名 %q 已被占用: %w", username, ErrDuplicate)
		}
		return nil, fmt.Errorf("store: 创建用户失败: %w", err)
	}
	return &u, nil
}

// GetUserByName 按用户名查用户。
func (s *Store) GetUserByName(ctx context.Context, username string) (*User, error) {
	var u User
	err := s.pool.QueryRow(ctx, `
		SELECT id, username, is_admin, created_at, updated_at
		FROM users WHERE username = $1`, username,
	).Scan(&u.ID, &u.Username, &u.IsAdmin, &u.CreatedAt, &u.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, fmt.Errorf("store: 用户 %q 不存在: %w", username, ErrNotFound)
	}
	if err != nil {
		return nil, fmt.Errorf("store: 查询用户失败: %w", err)
	}
	return &u, nil
}

// ListUsers 按创建顺序列出全部用户。
func (s *Store) ListUsers(ctx context.Context) ([]User, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id, username, is_admin, created_at, updated_at
		FROM users ORDER BY id`)
	if err != nil {
		return nil, fmt.Errorf("store: 列出用户失败: %w", err)
	}
	defer rows.Close()

	var out []User
	for rows.Next() {
		var u User
		if err := rows.Scan(&u.ID, &u.Username, &u.IsAdmin, &u.CreatedAt, &u.UpdatedAt); err != nil {
			return nil, fmt.Errorf("store: 读取用户失败: %w", err)
		}
		out = append(out, u)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("store: 列出用户失败: %w", err)
	}
	return out, nil
}

// SetPassword 修改密码。
func (s *Store) SetPassword(ctx context.Context, username, password string) error {
	if password == "" {
		return fmt.Errorf("store: 密码不能为空")
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("store: 生成密码哈希失败: %w", err)
	}

	tag, err := s.pool.Exec(ctx, `
		UPDATE users SET password_hash = $1, updated_at = now() WHERE username = $2`,
		string(hash), username)
	if err != nil {
		return fmt.Errorf("store: 修改密码失败: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("store: 用户 %q 不存在: %w", username, ErrNotFound)
	}
	return nil
}

// VerifyPassword 校验密码，成功则返回用户。
//
// 用户名不存在与密码错误返回完全相同的错误：区分开来，这个接口就成了
// 用户名枚举器。同理，用户不存在时也走一遍 bcrypt 比对，避免用响应
// 时间的差异泄露用户是否存在。
func (s *Store) VerifyPassword(ctx context.Context, username, password string) (*User, error) {
	var u User
	var hash string
	err := s.pool.QueryRow(ctx, `
		SELECT id, username, is_admin, created_at, updated_at, password_hash
		FROM users WHERE username = $1`, username,
	).Scan(&u.ID, &u.Username, &u.IsAdmin, &u.CreatedAt, &u.UpdatedAt, &hash)

	if errors.Is(err, pgx.ErrNoRows) {
		// 拿一个固定的合法 bcrypt 哈希做无用功，让耗时与真实路径相当
		_ = bcrypt.CompareHashAndPassword([]byte(dummyHash), []byte(password))
		return nil, ErrBadCredentials
	}
	if err != nil {
		return nil, fmt.Errorf("store: 查询用户失败: %w", err)
	}

	if err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)); err != nil {
		return nil, ErrBadCredentials
	}
	return &u, nil
}

// dummyHash 是一个固定的合法 bcrypt 哈希，其明文无关紧要——它只用来
// 在「用户不存在」的路径上消耗与真实比对相当的时间。
const dummyHash = "$2a$10$N9qo8uLOickgx2ZMRZoMyeIjZAgcfl7p92ldGxad68LJZdL17lhWy"

// CountUsers 返回用户总数。
func (s *Store) CountUsers(ctx context.Context) (int, error) {
	var n int
	if err := s.pool.QueryRow(ctx, `SELECT count(*) FROM users`).Scan(&n); err != nil {
		return 0, fmt.Errorf("store: 统计用户数失败: %w", err)
	}
	return n, nil
}

// EnsureAdmin 在库里一个用户都没有时创建管理员，并返回随机密码。
//
// 密码只在这一次返回，之后无法找回——库里只有哈希。调用方必须把它
// 打印出来。
func (s *Store) EnsureAdmin(ctx context.Context) (username, password string, created bool, err error) {
	n, err := s.CountUsers(ctx)
	if err != nil {
		return "", "", false, err
	}
	if n > 0 {
		return "", "", false, nil
	}

	pass, err := randomPassword()
	if err != nil {
		return "", "", false, err
	}
	if _, err := s.CreateUser(ctx, defaultAdminName, pass, true); err != nil {
		return "", "", false, err
	}
	return defaultAdminName, pass, true, nil
}

// randomPassword 生成 24 个字符的随机密码。
func randomPassword() (string, error) {
	b := make([]byte, 18) // base64 后正好 24 个字符
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("store: 生成随机密码失败: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

// isUniqueViolation 判断错误是否为唯一约束冲突。
func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}
```

- [ ] **Step 5: 让 Migrate 自动创建管理员**

在 `server/internal/store/migrate.go` 的 `Migrate` 方法 `return nil` 之前插入说明，并**不要**在这里调用 `EnsureAdmin`——`Migrate` 只管 schema。改由命令行的 `migrate` 子命令依次调用 `Migrate` 与 `EnsureAdmin`（Task 15）。在 `Migrate` 的文档注释末尾补一句：

```go
// Migrate 只负责 schema。首个管理员由命令行的 migrate 子命令在迁移后
// 调用 EnsureAdmin 创建——把「改表」和「造数据」分开，前者可以反复跑，
// 后者只在空库上发生一次。
```

- [ ] **Step 6: 跑测试确认通过**

```bash
export MAGICD_TEST_DATABASE_URL='postgres://magicd:magicd@localhost:5433/magicd?sslmode=disable'
cd server; go test ./internal/store/ -v 2>&1 | tail -40; echo "退出码=$?"
```

预期：全部 PASS。

- [ ] **Step 7: 跑全量与交叉编译**

```bash
cd server; go vet ./... ; gofmt -l . ; go test ./... 2>&1 | tail -20; echo "退出码=$?"
cd server; CGO_ENABLED=0 GOOS=windows GOARCH=arm64 go build -o /dev/null ./... ; echo "退出码=$?"
```

- [ ] **Step 8: 提交**

```bash
git add server/internal/store/ server/go.mod server/go.sum
git commit -m "$(cat <<'EOF'
feat: 新增用户表与 bcrypt 密码

用户名不存在与密码错误返回完全相同的错误，且用户不存在时也走一遍
bcrypt 比对——否则这个接口既是用户名枚举器，又能靠响应时间的差异
判断某人在不在。

EnsureAdmin 只在库里一个用户都没有时创建管理员并返回随机密码，
密码仅此一次可见。它不放在 Migrate 里：改表可以反复跑，造数据
只该在空库上发生一次。

Co-Authored-By: Claude Opus 5 <noreply@anthropic.com>
EOF
)"
```

---

**Part 1 到此结束。** 继续 Task 5–10，见 `2026-07-31-p3-data-layer-part2.md`。
