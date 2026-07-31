# P2 规则引擎 · 设计文档

- 日期：2026-07-31
- 状态：已评审
- 所属项目：神奇弹幕重构（Go 重写版）
- 前置：P0 协议内核（已完成）、P1 分发流水线（已完成）
- 全局决策见 `2026-07-29-p0-protocol-core-design.md` 附录 A

---

## 0. 目标

把 P0 产出的归一化事件流变成**可配置的自动化行为**，交付一个真正能跑的
无头弹幕机器人：

```bash
magicd run -c config.yaml
```

## 1. 范围

### 1.1 交付功能

| 功能 | 说明 |
|---|---|
| 自动欢迎进场 | 支持合并窗口，一波人进场只发一条 |
| 礼物 / 关注 / 上舰答谢 | 礼物支持连击合并 |
| 关键词自动回复 | 结构化条件匹配弹幕正文 |
| 定时任务 | 6 字段 cron（含秒） |
| 事件动作 | 任意事件类型触发任意动作 |
| 自动禁言 | 条件命中则禁言，时长可配 |
| 多账号职责分工 | 每个「账号-直播间」组合独立配置规则，见第 9 节 |

### 1.2 明确不做

| 不做的事 | 归属 | 理由 |
|---|---|---|
| 弹幕记录落盘 | P3 | 与数据层职责重叠，P2 做等于白写一次 |
| 永久禁言名单 | P3 | 需要持久化，是 P3 的职责 |
| HTTP API、WebUI | P4 | — |
| 统计聚合 | P5 | — |
| PK 大乱斗业务逻辑 | P6 | P0 已把 `PK_BATTLE_*` 归一化为 `Battle` 事件，P2 不消费 |

---

## 2. 架构

```
event.Event（来自 P0 Connector）
      │
      ▼
┌──────────────────────────────────────────┐
│ Pipeline（每个「账号-直播间」绑定一条）        │
│                                          │
│  ① Aggregator  缓冲窗口：去重 + 合并        │
│  ② Matcher     规则匹配（结构化条件）        │
│  ③ Executor    动作执行                    │
│       ├ Template  模板渲染                 │
│       └ Script    goja 沙箱                │
│  ④ Cooldown    三层节流                    │
│       └──→ connector.Actions（P0）         │
└──────────────────────────────────────────┘
      ▲
 Scheduler（cron 定时任务，直接注入 ②）
```

**每个「账号-直播间」绑定一条独立 Pipeline**（见第 9 节），绑定之间
互不影响。Pipeline 内部单 goroutine 串行处理，避免规则之间产生竞态。

同一直播间被两个账号连接时会有两条 Pipeline 与两条 WebSocket 连接，
这是刻意的：它们代表不同账号的视角与职责。

---

## 3. Trigger：规则的输入

合并后的事件不再是单个 `event.Event`，需要一层包装。

```go
// Trigger 是规则匹配的输入单元。
type Trigger struct {
    Type   event.Type      // 事件类型
    Events []event.Event   // 未合并时长度为 1，合并后为 N
    Vars   map[string]any  // 条件求值与模板渲染的唯一取值来源
}
```

### 3.1 Vars 是唯一取值来源

条件里写 `user.guardLevel`、模板里写 `{{.user.guardLevel}}`，两者必须指向
同一份数据。把展开逻辑收敛到 `vars.go` 一处，杜绝两套字段名各自演化。

单事件的 Vars 结构（以弹幕为例）：

```
type            "danmaku"
roomId          "1706666491"
timestamp       1753920000
text            "主播晚上好"
user.uid        "12345678"
user.username   "路人甲"
user.guardLevel 3
user.userLevel  18
user.wealthLevel 7
user.isAdmin    false
user.medal.name  "真yu中"
user.medal.level 24
```

合并后的 Trigger 额外带：

```
count           3               // 合并了几个事件
users           ["甲","乙","丙"] // 去重后的用户名数组
```

### 3.2 缺失字段

`Vars` 中不存在的路径求值为 `nil`，条件比较时视为不匹配（而非报错）。
模板渲染时输出空串。理由：B 站字段时有时无，规则不该因此崩掉。

---

## 4. 去重与合并

### 4.1 一个缓冲区，两个用途

这是本设计的关键简化：**去重和合并共用同一个缓冲窗口**。

已知问题（P0 真机联调发现）：同一次进场会触发两条 `UserEnter`——
`ENTRY_EFFECT`（只有 UID，无昵称）与 `INTERACT_WORD_V2`（信息完整）。

处理方式：两条事件落进同一个窗口后，按 UID 分组并**逐字段取非空值合并**，
自然合成一条完整事件。不需要单独写去重逻辑。

### 4.2 合并规格

```go
type AggregateSpec struct {
    Window time.Duration // 缓冲时长
    By     string        // 分组键：type / user / gift
}
```

窗口到期时**分两步处理，顺序不可颠倒**：

**第一步：按「同一件事」合并字段（总是执行）**

窗口内描述同一件事的多条事件先逐字段合并：值相同则保留，一方为空一方
非空则取非空，数量类字段（`gift.count`、`gift.totalCoin`）累加。

「同一件事」的判定键是 **类型 + UID + 事件自身的区分项**：

| 事件类型 | 分桶键 |
|---|---|
| 礼物（`Gift` / `GiftCombo`） | 类型 + UID + **礼物名** |
| 其余 | 类型 + UID |

礼物必须带上礼物名，否则同一用户在窗口内送出的小花花与辣条会被合进
一个桶，数量被错误地混加。

这一步与 `By` 无关，是它解决了 `ENTRY_EFFECT` 无昵称的问题——两条进场
事件合成一条完整的。

**第二步：按 `By` 分组产出 Trigger**

| By | 分组键 | 产出 |
|---|---|---|
| `type` | 事件类型 | 全部进场合成一条 Trigger，`users` 为参与者数组 |
| `user` | 类型 + UID | 每个用户一条 Trigger，仅做去重不做聚合 |
| `gift` | 类型 + UID + 礼物名 | 每种「用户+礼物」一条，数量已在第一步累加 |

`users` 数组来自第一步合并后的非空用户名，按首次出现顺序排列。

> 礼物连击的额外说明：P0 会同时投递多条 `Gift` 与一条 `GiftCombo`。
> `By: gift` 只合并 `Gift`；`GiftCombo` 与之重复计数，规则若同时监听
> 两种类型会重复答谢。推荐做法是只监听 `gift`，配置示例即如此。

### 4.3 未配置合并的规则

`Aggregate` 为 nil 时事件直接透传，不进缓冲区，零延迟。

---

## 5. 规则模型

```go
type Rule struct {
    Name          string
    Enabled       bool
    On            []event.Type   // 事件触发：监听的事件类型
    Schedule      string         // 定时触发：6 字段 cron 表达式
    When          *Condition     // nil 表示无条件
    Aggregate     *AggregateSpec // nil 表示不合并
    Do            []Action       // 按序执行
    Cooldown      time.Duration  // 本规则最小触发间隔
    CooldownGroup string         // 命名冷却组，可空
}
```

`On` 与 `Schedule` 互斥：前者由事件驱动，后者由 cron 驱动。两者同时为空
或同时非空都是配置错误，加载时校验并拒绝启动。

### 5.1 条件

```go
type Condition struct {
    // 叶子节点
    Field string // Vars 中的路径，如 "user.guardLevel"
    Op    string // 见下表
    Value any

    // 分支节点
    All []Condition // 全部满足
    Any []Condition // 任一满足
    Not *Condition  // 取反

    // 逃生舱
    Script string // JS 表达式，须返回 boolean
}
```

支持的操作符：

| Op | 含义 | 适用类型 |
|---|---|---|
| `eq` `ne` | 相等 / 不等 | 任意 |
| `gt` `gte` `lt` `lte` | 数值比较 | 数值 |
| `contains` | 包含子串 | 字符串 |
| `prefix` `suffix` | 前缀 / 后缀 | 字符串 |
| `regex` | 正则匹配 | 字符串 |
| `in` | 属于集合 | 任意 |

条件求值器是**纯函数、不起 goja**，可表驱动测试。只有写了 `Script`
的条件才进沙箱——绝大多数规则不该付这个开销。

一个 Condition 中 `Field`/`All`/`Any`/`Not`/`Script` 只能有一个生效，
配置加载时校验，冲突则报错拒绝启动。

### 5.2 动作

```go
type Action struct {
    Type string // danmaku / block / script / log

    // Type == danmaku
    Template []string // 多条时随机挑一条

    // Type == block
    Hours int

    // Type == script
    Script string
}
```

`danmaku` 与 `block` 的目标用户取自 `Vars` 中的 `user.uid`；合并事件下
`block` 作用于全部参与者。

---

## 6. 模板

采用 Go 标准库 `text/template`，数据源为 `Trigger.Vars`。

```
欢迎 {{.users}} 回家~
{{.user.username}} 送出 {{.gift.name}} x{{.gift.count}}，谢谢老板！
```

选用标准库而非自造语法的理由：已充分测试、支持 `{{if}}` 条件、无需
维护解析器——这与「弃用原项目自创 DSL」的决策一脉相承。

### 6.1 内置函数

| 函数 | 用途 |
|---|---|
| `join` | 数组拼接，如 `{{join .users "、"}}` |
| `simplifyName` | 昵称简化，去除常见前后缀与装饰字符 |
| `pick` | 从参数中随机取一个，用于问候语变化 |
| `truncate` | 按字符截断，防止超长 |

### 6.2 多模板随机

`Template` 是字符串数组，每次触发随机挑一条，对应原项目「多行随机」
的行为。数组长度为 1 时即固定文案。

---

## 7. goja 沙箱

### 7.1 默认无能力

goja 本身不提供 `require`、文件系统、`process`、网络。这是**天然白名单**：
不注入就没有。多用户场景下，别人写的脚本读不到服务器文件、起不了进程。

### 7.2 注入面

| 全局对象 | 能力 |
|---|---|
| `event` | 当前 Trigger 的 Vars，只读 |
| `bot.sendDanmaku(text)` | 发弹幕，仍受三层冷却约束 |
| `bot.block(uid, hours)` | 禁言 |
| `storage.get(k)` / `storage.set(k, v)` | 房间级键值存储（P2 用内存 + 文件，P3 迁数据库） |
| `console.log(...)` | 写入日志 |

**不注入网络访问**。原项目的 JS 引擎提供了 `fetch`/`get`/`post`，但在多用户
场景下等同把服务器变成任意请求代理，风险不可控。确有需要时应在 P3 之后
以受控白名单形式提供。

### 7.3 超时中断

每次执行设硬超时（默认 200ms），到期调用 `goja.Interrupt()` 强制打断。
死循环脚本不得拖垮整个房间的 Pipeline。

### 7.4 并发

`goja.Runtime` 不是并发安全的。每个执行槽持有独立 Runtime，用 `sync.Pool`
管理复用，避免每次执行都重建（重建开销显著）。

---

## 8. 三层冷却

| 层 | 作用 | 实现 |
|---|---|---|
| 全局限流 | 防账号触发风控 | 复用 P0 的 `ratelimit.Limiter` |
| 冷却组 | 多规则共享节流 | `map[string]ratelimit.Limiter` |
| 规则冷却 | 单规则最小间隔 | 每规则记录 `lastFired` |

三层**依次检查，任一不通过即跳过本次触发**（不排队、不延迟发送）。

### 8.1 为什么用命名而非编号

原项目用 `msgCds[100]` 加手动分配的通道号，运营需要自己记住「3 号通道是
关注答谢」。命名冷却组（`greeting`、`thanks`）消除这层记忆负担，且在
P4 的 WebUI 中可直接下拉选择。

---

## 9. 账号与绑定

### 9.1 运行单元是「账号-直播间」

账号不是可互换的资源，而是各有职责的参与者。配置结构为三层嵌套：

```
账号A ──┬── 直播间甲  →  A 在甲的规则
        └── 直播间乙  →  A 在乙的规则
账号B ──┬── 直播间乙  →  B 在乙的规则
        └── 直播间丙  →  B 在丙的规则
```

**每个「账号-直播间」组合（下称 Binding）是一个独立的运行单元**：自己的
WebSocket 连接、自己的规则集、自己的引擎实例。同一直播间被两个账号连接
时会有两条连接，它们互不知道对方存在。

这个模型同时覆盖两类场景：

| 场景 | 配置 |
|---|---|
| 多账号单直播间（职责分工） | 主播号-甲 只做统计与房管不发言；小号-甲 负责欢迎答谢 |
| 多账号多直播间 | 账号 C-张三房间、账号 D-李四房间，互不干扰 |

### 9.2 不做账号轮换

指定账号被封禁或 Cookie 失效时，**记录错误日志即可，不切换到其他账号**。

理由：账号职责不同。主播号被封不该让小号顶替房管操作——小号根本没有
房管权限；反过来让主播号去发欢迎语也违背「主播号不发言」的设定。
自动 fallback 在这个模型下只会制造意料之外的行为。

### 9.3 限流按账号而非按进程

B 站的风控是按账号计算的，因此同一账号的全部直播间共享一个限流器，
不同账号之间彼此独立。

```go
type Account struct {
    Name    string
    Session *auth.Session
    Limiter ratelimit.Limiter // 该账号全部直播间共享
}

type Binding struct {
    Account *Account
    RoomID  string
    Rules   []Rule
}
```

账号 A 同时连甲和乙时，它在两个房间的发送共用节奏；账号 B 完全不受
账号 A 的发送频率影响。

---

## 10. 定时任务

采用 `github.com/robfig/cron/v3`，启用秒级字段（6 字段表达式）。

```yaml
  - name: 定时广告
    schedule: "0 */5 * * * *"   # 每 5 分钟
    do:
      - type: danmaku
        template: ["关注主播不迷路~"]
```

定时任务产生一个 `Type` 为 `scheduled` 的 Trigger，Vars 只含 `roomId`
与 `timestamp`，之后走与事件规则完全相同的匹配与执行路径。

---

## 11. 配置

配置结构与 9.1 的三层模型同构：账号 → 直播间 → 规则。

```yaml
accounts:
  - name: 主播号
    cookieFile: cookie-main.txt
    # 该账号全部直播间共享的发送限流
    rateLimit: 1.5s

    rooms:
      - id: "1706666491"
        cooldownGroups:
          moderation: 5s
        rules:
          # 主播号只做房管，不发言
          - name: 广告禁言
            on: [danmaku]
            when:
              any:
                - {field: text, op: regex, value: "(广告|加群)"}
                - {field: text, op: contains, value: "违禁词"}
            cooldownGroup: moderation
            do:
              - type: block
                hours: 1

  - name: 小号
    cookieFile: cookie-sub.txt
    rateLimit: 1.5s

    rooms:
      - id: "1706666491"
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

          - name: 礼物答谢
            on: [gift]
            aggregate:
              window: 3s
              by: gift
            cooldownGroup: thanks
            do:
              - type: danmaku
                template:
                  - "感谢 {{simplifyName .user.username}} 的 {{.gift.name}} x{{.gift.count}}！"
```

### 11.1 规则内联，不做引用

规则完整写在所属的「账号-直播间」下，不提供跨绑定的引用机制。

多个绑定需要相同规则时确实要重复书写。这是有意的取舍：未来会引入
**模板功能**来解决复用，现在引入一套引用机制会与之冲突。

### 11.2 规则名的唯一性范围

规则名在**单个绑定内**唯一即可，不要求全局唯一——同一条「进场欢迎」
本来就会出现在多个绑定下。冷却状态也按绑定隔离，互不干扰。

配置格式只是**一种序列化**。规则模型本身存储无关，P3 迁进数据库时核心
逻辑不动，只换加载器。

---

## 12. 错误处理

| 情况 | 处理 |
|---|---|
| 配置校验失败 | 启动时报错退出，不带病运行 |
| 条件求值出错（如正则非法） | 加载时校验；运行时出错记录并视为不匹配 |
| 模板渲染失败 | 记录日志，跳过该动作，不影响后续动作 |
| 脚本超时 | 强制中断，记录日志，继续处理下一事件 |
| 发弹幕失败（可重试码） | 按 P0 的 `IsFatal` 判定，可重试则退避重试一次 |
| 发弹幕失败（致命码） | 标记账号失效，切换下一账号 |

**原则**：单条规则出错不得影响其他规则，单个房间出错不得影响其他房间。

---

## 13. 测试策略

| 对象 | 方式 |
|---|---|
| 条件求值器 | 表驱动单测，覆盖全部操作符与类型组合 |
| Vars 展开 | 对每种事件类型断言字段路径 |
| 模板渲染 | 表驱动，含内置函数与缺失字段 |
| 合并窗口 | 注入时序事件，断言合并结果与去重效果 |
| 三层冷却 | 断言各层独立生效与叠加行为 |
| goja 沙箱 | 专测超时中断、无文件系统访问、注入 API 可用 |
| Pipeline | 假事件流灌入，断言产出的动作序列 |
| 账号轮换 | 断言轮询顺序与失效剔除 |

**不做真实发弹幕的集成测试**——沿用 P0 的原则，动作执行层用假的
`connector.Actions` 替身。

---

## 14. 仓库布局

```
server/internal/rules/
├── rule.go        Rule / Condition / Action 模型
├── trigger.go     Trigger 定义
├── condition.go   条件求值器
├── vars.go        Event → Vars 展开
├── template.go    模板渲染与内置函数
├── script.go      goja 沙箱
├── cooldown.go    三层节流
├── aggregate.go   合并窗口与去重
├── matcher.go     规则匹配
├── executor.go    动作执行
├── engine.go      Pipeline 组装
└── config/
    └── yaml.go    YAML 加载与校验
server/internal/account/
└── account.go     账号与「账号-直播间」绑定
server/internal/scheduler/
└── cron.go        定时任务
```

新增依赖：`github.com/dop251/goja`、`github.com/robfig/cron/v3`、
`gopkg.in/yaml.v3`。三者均为纯 Go，不破坏交叉编译。

---

## 15. 与 P0 的接口

P2 **只通过 P0 的公开接口交互**，不修改 P0 代码：

- 消费 `connector.Connector.Events()`
- 调用 `connector.Actions.SendDanmaku` / `BlockUser`
- 复用 `ratelimit.Limiter`

唯一例外：若联调中发现 P0 的事件映射有误，修 P0 并补黄金样本——这属于
缺陷修复，不属于 P2 的功能开发。
