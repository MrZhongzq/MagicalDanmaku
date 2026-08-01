// 规则的序列化表示，逐字段对应后端 server/internal/rules/spec/spec.go。
//
// **不要凭印象改这个文件。** 上一次任务凭印象给的事件类型清单漏了 6 种、
// 错了 7 个名字，实现者去核对 Go 代码才发现。这里的每个字段、每个
// omitempty/非 omitempty 都是照着 spec.go 的 json tag 抄的，改动前先去
// 读 Go 源码，不要凭感觉加字段或改可选性。
//
// GET /api/bindings/{id}/rules 返回的 ruleView 是
// `struct { spec.Rule; Position int }`，Go 的匿名字段嵌入在 JSON 里会被
// 展开，所以响应体就是 Rule 的字段们外加一个 position，这里额外定义
// RuleView 承接它。

/**
 * Duration 是后端 spec.Duration 的 TS 对应形式：可读的时长字符串，
 * 如 "1.5s"、"500ms"、"3m"、"1m30s"（Go `time.ParseDuration` 能解析的格式）。
 *
 * **不是纳秒整数，也不是毫秒数字。** spec.Duration 的 MarshalJSON 输出的
 * 就是 `time.Duration.String()`，一律是字符串。
 */
export type Duration = string

/**
 * Condition 对应 spec.Condition。
 *
 * Field/Op/Value 是叶子形态；All/Any/Not 是组合形态；Script 是脚本逃生舱。
 * 五种形态在后端是互斥的（`Condition.Validate()` 只允许恰好一种非空），
 * 但 TS 类型层面不表达这个互斥关系——校验交给后端，前端只负责别同时
 * 塞两种。
 *
 * 全部字段在 Go 侧都是 `omitempty`，对应到 TS 里全部可选。
 */
export interface Condition {
  field?: string
  op?: string
  /** 比较值，Go 侧是 `any`。TS 用 unknown 强制调用方在使用前收窄类型。 */
  value?: unknown
  all?: Condition[]
  any?: Condition[]
  not?: Condition
  script?: string
}

/**
 * Aggregate 对应 spec.Aggregate。
 *
 * 注意 `window` 与 `by` 在 Go 侧**没有** `omitempty`——它们始终会出现在
 * 序列化结果里，所以这里是必填字段，不加 `?`。`maxWait`/`minCount` 才是
 * omitempty，可选。
 */
export interface Aggregate {
  window: Duration
  maxWait?: Duration
  minCount?: number
  by: string
}

/**
 * Action 对应 spec.Action。
 *
 * `type` 没有 `omitempty`，必填；其余三个按用途二选一/三选一使用，
 * 都是 omitempty，可选。
 */
export interface Action {
  type: string
  template?: string[]
  hours?: number
  script?: string
}

/**
 * Rule 对应 spec.Rule，是本项目里规则的唯一序列化表示。
 *
 * `enabled` 是 `boolean | undefined`，**不是** `boolean`——Go 侧是 `*bool`
 * 指针，专门用来区分「请求体里没写 enabled」与「写了 enabled: false」。
 * 存进数据库时 name 与 enabled 会被提到列上，JSONB 主体里不含这两个
 * 字段，但 API 响应（ruleView）会把它们拼回来，所以前端拿到的 Rule
 * 总是带着这两个字段——只是类型层面仍然按 Go 的 json tag 标成可选。
 */
export interface Rule {
  name?: string
  enabled?: boolean
  on?: string[]
  schedule?: string
  when?: Condition
  aggregate?: Aggregate
  cooldown?: Duration
  cooldownGroup?: string
  do?: Action[]
}

/**
 * RuleView 对应后端 `ruleView`（`server/internal/httpapi/rule_handler.go`）：
 * Go 用匿名字段嵌入 `spec.Rule`，JSON 展开后就是 Rule 的全部字段
 * 外加一个 `position`。`GET /api/bindings/{id}/rules` 返回这个类型的数组。
 */
export interface RuleView extends Rule {
  position: number
}
