# P4-3 后端补丁批次 实施计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development.

**Goal:** 把 P4-2 悬空清单里**不需要真实样本**的 11 条补上，让 WebUI 上那些悬空的开关真正生效。

**输入：** `docs/superpowers/specs/2026-08-01-p4-2-悬空清单.md`（15 条）

**不在本批次：** 第 7、10、11、15 条——盲盒与 PK，需要用户在真实直播间触发并抓包。第 3 条已由 P4-2 Task 14 解决。

## Global Constraints

- module `github.com/MrZhongzq/MagicalDanmaku/server`，代码在 `server/`
- **纯 Go 依赖，不引入任何新依赖**，`CGO_ENABLED=0` 六平台交叉编译必须通过
- 只用标准库 `http.ServeMux`，不引入第三方路由/中间件/Web 框架
- 注释、错误信息、提交信息一律中文
- **一切改变状态的接口不得用 GET**
- **授权判定只有 `guard.go` 一处实现**（账号所有权判定收在 `isAccountOwner`）
- **B 站 Cookie 绝不出现在任何 HTTP 响应体里**
- **列表接口按调用者可见范围过滤**；「不存在」与「对调用者不可见」返回相同的 404
- `gofmt -l .` 无输出、`go vet ./...` 干净
- TDD：先写失败测试 → 确认失败 → 实现 → 确认通过 → 提交

## 测试环境

```bash
export MAGICD_TEST_DATABASE_URL='postgres://magicd:magicd@localhost:5433/magicd?sslmode=disable'
```
没有这个变量存储测试整包 skip 且退出码 0，那是假绿。**每次跑测试都要带 `-count=1`。**
**验证命令用 `; echo "退出码=$?"` 而不是 `&&`**（有管道时 `&&` 判断的是管道末端）。

`-race` 若报「requires cgo」，把这个目录加进 PATH：
```
/c/Users/ZIQI/AppData/Local/Microsoft/WinGet/Packages/BrechtSanders.WinLibs.POSIX.UCRT_Microsoft.Winget.Source_8wekyb3d8bbwe/mingw64/bin
```

## 任务分组与顺序

按**层**分组，先改引擎层（改动面最大、后面的任务要用它的产物），再改存储/API 层，最后是连接器层。

| Task | 悬空清单条目 | 层 | 风险 |
|---|---|---|---|
| 1 | 5、9 模板轮询 | 规则引擎 | 中：要给 `Action` 加字段并引入游标状态 |
| 2 | 6 单人/多人两套模板 | 规则引擎 | 中：与 Task 1 同一批字段 |
| 3 | 8 `{{gifts}}` 合并变量 | 规则引擎 | 低 |
| 4 | 13 规则排除 | 规则引擎 | **高：改的是 Engine 的分发顺序** |
| 5 | 14 统计聚合接口（含直播时长） | 存储 + API | 中 |
| 6 | 4 删除业务日志接口 | 存储 + API | 低 |
| 7 | 12 `GET /api/meta/variables` | API | 低 |
| 8 | 1 账号登录态检测 | 连接器 + 存储 + API | 中 |
| 9 | 2 拉黑/解除拉黑接口 | 连接器 + API | 中 |
| 10 | 前端接线：把上面这些接上 | 前端 | 中 |

**Task 10 一次性做完前端接线**，不要每个后端任务都回头改前端——那会让前端产物反复重建。

---

## Task 1: 模板轮询模式

**悬空清单第 5、9 条。** 两条同一个病根。

**Files:**
- Modify: `server/internal/rules/spec/spec.go`（`Action` 加 `Pick`）
- Modify: `server/internal/rules/rule.go`（`Action` 加 `Pick`，`Validate` 校验取值）
- Modify: `server/internal/rules/spec/convert.go`（转换时带上）
- Modify: `server/internal/rules/template.go`（`Render` 支持顺序取）
- Modify: `server/internal/rules/executor.go`（持有游标，按 `Pick` 分派）
- Modify: 对应的 `_test.go`

**Interfaces:**
- Produces: `spec.Action.Pick string`（`""`/`"random"`/`"sequential"`）、`rules.Action.Pick`

### 设计

**`Pick` 是动作级而不是规则级**，因为一条规则可以有多个发弹幕动作，各自的模板列表独立。

```go
// Pick 决定多条模板怎么挑。
//
// 空或 "random"：随机挑一条（默认，与历史行为一致）
// "sequential"：按顺序轮流用，到末尾回到第一条
const (
	PickRandom     = "random"
	PickSequential = "sequential"
)
```

**游标放 `Executor` 里，按「规则名 + 动作下标」为键。**

```go
// pickCursor 记住每个动作的模板轮询到第几条。
//
// 键是「规则名#动作下标」——一条规则可以有多个发弹幕动作，
// 各自的模板列表独立，共用一个游标会互相打乱。
//
// **热重载会重置它**：Executor 属于 Engine，换引擎就是新的一份。
// 这是可接受的——轮询是为了「文案别老重复」，重载后从头开始不影响
// 这个目的，而把游标持久化进 kv_store 要为一个纯展示效果引入写库开销。
mu     sync.Mutex
cursor map[string]int
```

**`Execute` 要把动作下标传给 `runAction`**（现在只传了规则名）。

**并发**：`Executor` 会被多个 goroutine 调用吗？`Engine.Handle` 持锁串行化，但定时规则走 `FireScheduled` 也持同一把锁。所以实际是串行的——**但仍然给游标加锁**，理由写进注释：依赖调用方的锁是隐式契约，将来 Engine 改成并行分发时这里会静默出错。

### Step 1: 写失败的测试

`server/internal/rules/template_test.go` 追加：

```go
// 顺序模式按顺序轮流用，到末尾回到第一条。
func TestRenderSequentialCyclesThroughTemplates(t *testing.T) {
	r := NewRenderer(nil)
	tmpls := []string{"甲", "乙", "丙"}

	var got []string
	for i := 0; i < 5; i++ {
		s, err := r.RenderAt(tmpls, i, nil)
		if err != nil {
			t.Fatalf("渲染报错: %v", err)
		}
		got = append(got, s)
	}
	want := []string{"甲", "乙", "丙", "甲", "乙"}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("第 %d 次 = %q, 期望 %q（全部: %v）", i, got[i], want[i], got)
		}
	}
}

// 下标为负或超界都不该 panic
func TestRenderAtHandlesOutOfRange(t *testing.T) {
	r := NewRenderer(nil)
	for _, idx := range []int{-1, 99} {
		if _, err := r.RenderAt([]string{"甲"}, idx, nil); err != nil {
			t.Errorf("下标 %d 报错: %v", idx, err)
		}
	}
}
```

`server/internal/rules/executor_test.go` 追加（**这条是核心**）：

```go
// 同一条规则的两个发弹幕动作各有各的游标，不互相打乱。
//
// 共用一个游标的话，两个动作会交替推进它——第一个动作拿甲、
// 第二个拿乙、下一轮第一个拿丙，每个动作看到的都是跳着的。
func TestSequentialCursorIsPerAction(t *testing.T) {
	// 构造一条规则，Do 里有两个 danmaku 动作，各自三条模板、都是 sequential
	// 连续触发三次，断言动作一依次拿到 甲1/甲2/甲3，动作二依次拿到 乙1/乙2/乙3
}
```

**具体写法由实现者定**（要用现有的测试替身），但必须钉住「两个动作的游标互不干扰」。

### Step 2–5

跑测试确认失败 → 实现 → 跑测试确认通过（含 `-race`）→ 提交。

**实现要点**：

- `Renderer` 加 `RenderAt(templates []string, idx int, vars) (string, error)`，对 `idx` 取模并处理负数
- `Render` 保持原样（随机），不改签名——它还有别的调用方
- `Executor.runAction` 按 `a.Pick` 分派：`PickSequential` 走 `RenderAt` + 推进游标，其余走 `Render`
- `rules.Action.Validate`（或 `spec` 的转换处）拒绝未知的 `Pick` 取值，错误信息列出合法值

---

## Task 2: 单人/多人两套模板

**悬空清单第 6 条。**

**Files:**
- Modify: `server/internal/rules/spec/spec.go`、`rule.go`、`convert.go`、`executor.go` 与测试

### 设计

`spec.Action` 加：

```go
	// TemplateMulti 是合并触发（count > 1）时用的模板。
	//
	// 为什么要两套：「欢迎 张三 回家」与「欢迎 张三、李四、王五 回家」
	// 句式本就不同，共用一套必然有一边别扭。
	//
	// 留空则不论单人多人都用 Template——保持与历史配置兼容。
	TemplateMulti []string `yaml:"templateMulti" json:"templateMulti,omitempty"`
```

**判据是 `tr.Vars["count"]`**，`Aggregator` 合并时会填它（`aggregate.go` 的 `mergeBuckets`）。非合并触发时 `count` 是 1。

**要处理的边界**：`count` 可能不存在（某些触发路径没填）或不是数字。**取不到就当 1**（走单人模板），不要 panic。

### 测试要钉住的

1. `count == 1` 用 `Template`
2. `count > 1` 且 `TemplateMulti` 非空 → 用 `TemplateMulti`
3. `count > 1` 但 `TemplateMulti` 为空 → **回落到 `Template`**（兼容旧配置，不是报错）
4. `count` 缺失或类型不对 → 当 1
5. `TemplateMulti` 与 `Pick` 组合：轮询游标对两套模板是**分开**的还是共用的？——**分开**，键要带上是哪一套。写测试钉住

---

## Task 3: `{{gifts}}` 多礼物名合并变量

**悬空清单第 8 条。**

**Files:**
- Modify: `server/internal/rules/aggregate.go`（`mergeBuckets`）与测试

### 设计

`mergeBuckets` 现在收集了去重的 `users`，但从不构建 `gifts`。设计文档 §7.2 给的示例模板是：

```
感谢 {{users}} 的 {{gifts}} 等，您的支持就是对主播最大的鼓励
```

照 `users` 的做法加 `gifts`：合并窗口内出现过的礼物名，**去重、保持首次出现的顺序**。

**要想清楚的**：`users` 是怎么去重与排序的？照它来，保持一致。**去重键用礼物名**（`gift.name`）。

**测试**：合并窗口里有「小花花 ×3、人气票 ×1、小花花 ×2」→ `gifts` 应当是 `["小花花", "人气票"]`，不是三条也不是乱序。

---

## Task 4: 规则排除（引擎互斥机制）

**悬空清单第 13 条。这是本批次风险最高的一个——改的是 Engine 的分发顺序。**

**Files:**
- Modify: `server/internal/rules/spec/spec.go`、`rule.go`（`Rule` 加 `Suppress`）
- Modify: `server/internal/rules/engine.go`（分发时按顺序并跳过被压制的）
- Modify: 测试

### 设计

用户的要求：

> 一条自定义规则命中后，可以声明屏蔽掉哪些通用功能。典型场景是「给某位舰长配了专属进房欢迎，就不该再触发通用进房欢迎」——否则那位舰长进房会被欢迎两次。

`spec.Rule` 加：

```go
	// Suppress 列出本规则命中后要跳过的规则名。
	//
	// 典型场景：给某位舰长配了专属进房欢迎，就不该再触发通用进房欢迎，
	// 否则他进房会被欢迎两次。
	//
	// **只对同一次触发生效**，不是全局开关。
	Suppress []string `yaml:"suppress" json:"suppress,omitempty"`
```

### 分发顺序必须确定

**这是这个任务的关键。** 压制只有在「先执行的规则能压制后执行的」时才有意义，所以分发必须按确定的顺序。

- `store.ListRules` 是 `ORDER BY position, id`
- `NewEngine` 收到的 `[]Rule` 保持这个顺序吗？**去核实 `Matcher` 有没有打乱它**（比如用 map 存）

**如果 `Matcher` 用 map 遍历匹配，顺序就是随机的**——那样压制的行为每次都不一样，是比不实现更糟的结果。必须先确认，不确定就改成显式按 position 排序。

### 实现要点

`Engine.Handle` 里匹配出规则之后：

```go
	// 压制只对本次触发生效。按 position 顺序处理，前面的规则
	// 才可能压制后面的——顺序不确定的话压制行为每次都不一样，
	// 比不实现更糟。
	suppressed := map[string]bool{}
	for _, r := range matched {
		if suppressed[r.Name] {
			continue
		}
		e.fireLocked(r, tr)
		for _, name := range r.Suppress {
			suppressed[name] = true
		}
	}
```

**要处理的边界**：

1. **压制不存在的规则名**：静默忽略还是启动时校验？——**启动时校验**（`Validate` 里查不到就报错），因为写错规则名而静默不生效很难查
2. **自我压制**（`Suppress` 含自己）：`Validate` 拒绝
3. **循环压制**（甲压乙、乙压甲）：按顺序执行天然不会死循环（乙已经被跳过就不会执行它的压制），但要有测试钉住这个行为
4. **合并窗口**：被压制的规则如果配了 `aggregate`，它的窗口该不该收到这个事件？——**不该**，跳过就是完全跳过。但要确认 `Aggregator` 的喂入点在 `fireLocked` 之前还是之后

第 4 点**必须先看代码确认**，写进报告。

---

## Task 5: 统计聚合接口

**悬空清单第 14 条。**

**Files:**
- Modify: `server/internal/logging/sink.go`（`loggedEventTypes` 加 `live_start`/`live_stop`）
- Create: `server/internal/store/stats.go`、`stats_test.go`
- Create: `server/internal/httpapi/stats_handler.go`、`stats_test.go`
- Modify: `server/internal/httpapi/server.go`（注册路由）

### 先解决直播时长的根因

**`live_start`/`live_stop` 在 `event/type.go` 里本来就存在**，只是没进 `logging/sink.go` 的 `loggedEventTypes` 白名单，所以从没写进 `activity_logs`。

**加两行 map 项**即可，不需要新建场次表。这是 P4-2 终审订正过的根因。

**注意**：加了之后**历史数据里仍然没有**这两类事件——直播时长只能从加上之后的数据算起。这一点要在接口文档与前端提示里说清。

### 接口

```
GET /api/bindings/{id}/stats?by=day|session&since=...&until=...    event:read
```

响应形如：

```json
[
  {
    "bucket": "2026-08-01",
    "danmakuCount": 1234,
    "enterCount": 567,
    "giftCount": 89,
    "giftKinds": 7,
    "guardCount": 3,
    "liveSeconds": 10800
  }
]
```

**`by=session` 的分组靠 `live_start`/`live_stop` 配对**。要处理的边界：
- 只有 `live_start` 没有 `live_stop`（还在直播中，或漏了下播事件）
- 只有 `live_stop` 没有 `live_start`（查询区间从中间切开）

**这两种都要有测试**，且行为要写进注释。

### 权限与过滤

走 `s.requirePerm(perm.EventRead, ...)`，与 `/activity` 一致。

### SQL 侧 `GROUP BY`，不要拉全量到 Go 里聚合

这正是这条悬空存在的理由——前端在 500 条上聚合是错的，后端拉几万行到内存里聚合同样不对。

---

## Task 6: 删除业务日志接口

**悬空清单第 4 条。**

**Files:**
- Modify: `server/internal/store/activity.go`（加 `DeleteActivity`）
- Modify: `server/internal/httpapi/activity_handler.go`（加处理器）、`server.go`（路由）

```
DELETE /api/bindings/{id}/activity?since=...&until=...    event:read
```

**「清除」是真的删库。** 三件事：

1. **不得用 GET**（本来也不会）
2. **权限**：用 `event:read` 还是更强的？——**用 `event:read`**，与读日志一致。理由：能看到全部日志的人删掉它们不构成额外的信息泄漏，而这是个自托管单人工具。**但要在响应里返回删了多少行**，让操作者能核对
3. **不带任何时间参数时删全部**——这是危险操作。**要求必须显式传 `all=1`**，否则返回 422。理由：一个手滑的 `DELETE .../activity` 不该清空整个房间的历史

---

## Task 7: `GET /api/meta/variables`

**悬空清单第 12 条。**

**Files:**
- Modify: `server/internal/httpapi/meta_handler.go`、`server.go`

前端现在内置了一份从 `vars.go` 手抄的常用路径清单（P4-2 Task 11 登记的第二处定义）。

**难点**：`vars.go` 的变量是**按事件类型不同而不同**的（弹幕有 `text`，礼物有 `gift.*`）。所以这个接口应当返回「按事件类型分组的变量路径」。

去读 `vars.go` 的 `VarsFromEvent`，把它实际会产出的键**穷举**出来。**不要手写一份清单放在 `meta_handler.go` 里**——那又是第二处定义。想办法让它从 `vars` 那边导出，比如在 `rules` 包里加一个 `VariableCatalog()` 返回结构化的清单，`vars.go` 与它放在一起，改一处就都改了。

**这一条的价值就在于消灭第二处定义，实现方式如果又造了一处，就白做了。**

---

## Task 8: 账号登录态检测

**悬空清单第 1 条。**

**Files:**
- Modify: `server/internal/store/migrations/`（新增迁移：`accounts` 加登录态列）
- Modify: `server/internal/store/account.go`
- Modify: `server/cmd/magicd/run.go`（起定期检测）
- Modify: `server/internal/httpapi/account_handler.go`（`accountView` 带上状态）

### 设计

**B 站的登录态失效得很快**，这是这个功能存在的首要理由——不能等到发弹幕失败才发现账号掉线。

- `accounts` 加两列：`login_valid BOOLEAN`、`login_checked_at TIMESTAMPTZ`
- `run` 起一个定时循环（比如每 10 分钟）对每个账号调一次轻量的 B 站接口（`nav` 之类）判断登录态
- 结果写库，`accountView` 带上

**注意 Cookie 绝不出现在响应体里**——`accountView` 已经有这条约束，加字段时别破坏它。

**检测失败与登录态失效要分开**：网络不通不等于账号掉线。`login_valid` 应当是三态（有效/已失效/检测失败）还是两态 + `login_checked_at`？——**自己定，在报告里说明**。

---

## Task 9: 拉黑与解除拉黑

**悬空清单第 2 条。**

**Files:**
- Modify: `server/internal/connector/bilibili/action.go`（加拉黑动作）
- Modify: `server/internal/account/account.go`（`Binding` 加方法）
- Modify: `server/internal/httpapi/action_handler.go`（`BindingRuntime` 加方法 + 处理器）、`server.go`
- Modify: `server/cmd/magicd/run.go`（`roomRuntime` 实现新方法）

**主播才能拉黑，房管只能禁言。** 但按既定原则：**不预判、不锁死**——让操作者试，B 站拒绝时把原因原样回传。

走 `perm.UserBlock` 守卫（与禁言一致）。

**手动动作要记进业务日志**（`roomRuntime.recordManual` 已有这个机制，照 `Block`/`Unblock` 的写法来）。

**B 站的拉黑接口路径与参数要去查**——如果查不到可靠的接口定义，**不要猜**，把这条退回悬空清单并说明。

---

## Task 10: 前端接线

把上面九个任务的产物接到 WebUI 上：

- 悬空标记逐条撤掉（改成真实控件）
- `rule-types.ts` 补 `Pick`、`TemplateMulti`、`Suppress` 字段
- 变量下拉改成从 `GET /api/meta/variables` 拉
- 统计页接真实聚合接口
- 日志页「清除」接上
- 账号登录状态显示真实值
- 房管页主播区接上拉黑

**逐条更新悬空清单**：做掉的划掉或标注「已由 P4-3 Task N 解决」，剩下的（盲盒、PK）保留。

**最后重新构建前端产物并提交**（CI 有守卫会比对）。
