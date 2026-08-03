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
 * 序列化结果里，所以这里是必填字段，不加 `?`。`minCount`/`solo` 才是
 * omitempty，可选。
 */
export interface Aggregate {
  window: Duration
  minCount?: number
  by: string
  /**
   * Solo 对应 spec.Solo——「单人优先，多人兜底」的双轨聚合（P5-4）。
   * 不启用时整个字段不出现，不是 `{minItems: 0}`：0 会被后端 Validate
   * 拒绝（`server/internal/rules/rule.go` 的 SoloSpec.Validate），
   * 「配了但不生效」在这里没有合法状态。
   */
  solo?: AggregateSolo
}

/** AggregateSolo 对应 spec.Solo。 */
export interface AggregateSolo {
  /** 单人优先的件数阈值：窗口内某用户（不含盲盒）总件数达标就单独一条。 */
  minItems: number
}

/**
 * Action 对应 spec.Action。
 *
 * `type` 没有 `omitempty`，必填；其余字段都是 omitempty，可选，按 `type`
 * 决定用哪些。
 *
 * `template`/`templateMulti` 二选一即可（也可以两个都给）：单人触发用
 * `template`，合并触发（`count > 1`）优先用 `templateMulti`，留空则回落到
 * `template`。**后端有一条校验**：只给 `templateMulti`、不给 `template`、
 * 且规则没有 `aggregate` 时会被拒（422）——不合并的触发永远只有一个人，
 * `templateMulti` 用不上（`server/internal/rules/rule.go` 第 110-128 行）。
 *
 * `pick` 控制 `template`/`templateMulti` 有多条时怎么挑："random"（默认，
 * 空字符串等同 `"random"`，与历史配置兼容）或 `"sequential"`（轮询）。
 */
export interface Action {
  type: string
  template?: string[]
  templateMulti?: string[]
  pick?: string
  hours?: number
  script?: string
}

/**
 * PickMode 直接对应 spec.Action.Pick 的取值（`""`/`"random"`/`"sequential"`）。
 * 草稿态不使用空字符串——`parsePickMode` 把「空」也当作 `'random'` 处理
 * （与历史配置兼容），保存时统一显式写出 `'random'` 或 `'sequential'`。
 *
 * 挪到这里是共享点：Danmaku.vue 与 PkPanel.vue 都要用（P5-4 之前只有
 * Danmaku.vue 用，定义在那边；PK 播报/串门欢迎补上轮询开关后 PkPanel.vue
 * 也要用同一套，放在两者都不依赖对方的这个类型文件里，避免循环 import。
 */
export type PickMode = 'random' | 'sequential'

export const PICK_MODE_OPTIONS: { label: string; value: PickMode }[] = [
  { label: '随机抽取', value: 'random' },
  { label: '轮询（按顺序循环）', value: 'sequential' },
]

/** parsePickMode 把后端的 pick 字段（可能是空字符串或缺省）还原成草稿用的 PickMode。 */
export function parsePickMode(pick: string | undefined): PickMode {
  return pick === 'sequential' ? 'sequential' : 'random'
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
  /**
   * Suppress 列出本规则命中后要跳过的规则名。**只对同一次触发生效**，
   * **只对事件驱动（`on`）的规则生效**——定时（`schedule`）规则配了会被
   * 后端 `Validate()` 拒绝（`server/internal/rules/rule.go` 第 89-100 行）。
   */
  suppress?: string[]
}

/**
 * RuleView 对应后端 `ruleView`（`server/internal/httpapi/rule_handler.go`）：
 * Go 用匿名字段嵌入 `spec.Rule`，JSON 展开后就是 Rule 的全部字段
 * 外加一个 `position`。`GET /api/bindings/{id}/rules` 返回这个类型的数组。
 */
export interface RuleView extends Rule {
  position: number
}
