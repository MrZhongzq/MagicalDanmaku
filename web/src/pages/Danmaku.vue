<script lang="ts">
/**
 * 弹幕姬页：进房欢迎、礼物答谢（Task 9）+ PK 播报、轮播消息、其他答谢（Task 10）。
 *
 * ## 这一页的本质
 *
 * 它是规则编辑器的「傻瓜模式」：每个功能块在后端各是一条
 * `spec.Rule`，前端把它渲染成一组开关与输入框，用户点的每个控件最终都
 * 对应规则里的一个字段。加载时靠固定的 `name` 从
 * `GET /api/bindings/{id}/rules` 里认领已保存的配置，认领不到就当作
 * 「还没配过」，用默认值起步——见 `claimRule`。
 *
 * ## Task 10 新增三块的功能虚实（重要，决定了每块该怎么读）
 *
 * - **PK 播报**：整体悬空，只画界面。`event.Battle` 只有一个
 *   `SubCommand` 字段，对面数据（主播昵称/人数/大航海）协议层完全没解析。
 *   这部分的组件与纯函数都挪到独立的 `PkPanel.vue`——见该文件顶部注释。
 * - **轮播消息**：真功能。`spec.Rule.Schedule` 是 cron 驱动，与
 *   `On` 二选一（`rules/rule.go` 第 61-68 行的 `Validate` 会拒绝二者同时
 *   出现），组好的规则只给 `schedule`，绝不给 `on`。多条模板的挑选方式
 *   （随机/轮询）见下方「悬空清单」上方新增的 P4-3 说明——这一项 P4-3
 *   已经接通，不再是「名不副实的轮播」。
 * - **关注答谢 / 分享答谢 / 上舰答谢**：真功能。`event.UserFollow`/
 *   `UserShare` 只有 `User` 一个字段，模板简单；`event.GuardBuy` 载荷
 *   齐全（`GuardLevel`/`GuardName`/`Count`/`Price`/`IsRenew`），
 *   新购与续费的区分**不需要拆成两条规则**——直接在模板里用
 *   `text/template` 自带的 `{{if .guard.isRenew}}续费{{else}}开通{{end}}`
 *   语法，一条规则、一套模板就够（`rules/template.go` 用的是标准库
 *   `text/template`，天然支持条件分支，`rewriteFieldChains` 只改写字段
 *   访问不碰 `if`/`else`/`end` 关键字）。
 *
 * ## 保存：GET → 合并 → PUT → POST reload（Task 13 接上，现已实接）
 *
 * 改动先只进内存草稿，`dirty` 变真；右上角 `SaveBar` 的 `save` 事件接的
 * 是 `onSave`（本文件下方 `<script setup>` 部分），走 `useDraft` 提供的
 * 统一流程：GET 现有规则 → 与本页管的全部内置规则（`OWNED_RULE_NAMES`，
 * 见下方定义，目前九条）合并（不误删别的页面建的自定义规则）→ PUT
 * 写库 → POST reload 让规则引擎拿到新配置。第 2 步
 * （reload）失败时 `dirty` 不会归假，界面会给一条持久的「已保存到数据库，
 * 但重载失败」提示，具体行为见下方 `onSave` 与 `partialFailureMessage`
 * 处的注释。
 *
 * ## P4-3：模板轮询与单人/多人两套模板已接通，不再悬空
 *
 * 后端加了两个字段（`server/internal/rules/spec/spec.go`）：
 *
 * - `Action.Pick`（`""`/`"random"`/`"sequential"`，空等同随机，与历史
 *   配置兼容）：进房欢迎、礼物答谢、轮播消息三处「轮询/随机」单选框现在
 *   真的接进 `do[].pick`，选「轮询」会让规则引擎按顺序循环模板
 *   （`server/internal/rules/executor.go` 的 `Renderer` 按游标状态选取），
 *   不再是「选了也不生效」。
 * - `Action.TemplateMulti`（合并触发 `count > 1` 时用的模板，留空则回落到
 *   `Template`）：进房欢迎的「多人合并欢迎语」现在真的接进
 *   `do[].templateMulti`。进房欢迎的规则**始终带 `aggregate`**
 *   （见 `buildAggregateCommon` 的调用点），所以不会撞上后端那条
 *   「只配 templateMulti 又没有 aggregate」的校验（`rule.go` 第
 *   110-128 行）——这条校验只在没有合并窗口的规则上才可能触发，进房
 *   欢迎恒有合并窗口，天然安全，不需要额外的前端防呆。礼物答谢/轮播
 *   消息本页没有多人/单人两套模板的界面，`TemplateMulti` 不适用。
 *
 * ## P4-4 Task 7：盲盒单列已接通，不再悬空
 *
 * `event.Gift.BlindBox`（P4-4 Task 1 补上）+ `AggregateByBlindBox`
 * 聚合（Task 3）+ `blindBox.*` 模板变量（同上）三块能力早就绪，只是
 * 一直没在这一页接出口子——现在 `giftDraft.blindBoxSeparate`/
 * `blindBoxProfitTracking` 真的接进了 `buildBlindBoxRule` 组装出的
 * 独立规则（`BLINDBOX_RULE_NAME`），不再是纯 UI 占位，见该函数与
 * `buildGiftRule` 上方的注释。至此设计文档 §13 列的悬空项全部清零。
 *
 * ## 一处「以为悬空、实测已通」的纠正
 *
 * 简报原话是「`user_enter` 事件 Payload 没有粉丝牌字段」，但去读
 * `server/internal/event/user.go`（`User.Medal *Medal`，含
 * `IsLighted`/`RoomID`/`Level`）、`cmdmap/interactv2.go`
 * （`mapInteractWordV2` 确实会解析 `medal_info` 填进 `Medal`）、
 * `rules/vars.go`（`userVars` 把它们展开成
 * `user.medal.isLighted`/`user.medal.roomId`/`user.medal.level`）、
 * `rules/condition.go`（`eq`/`gte`/`in` 都支持这些字段）——**数据从解析
 * 到条件求值全链路都已经打通**，与设计文档 §13.2 标题「已具备，需写成
 * 规则」一致。所以「佩戴粉丝牌」筛选在本页是**真实可用**的功能，不是
 * 悬空占位：`buildEnterCondition` 直接拼出
 * `{isLighted:true, roomId:本房间} + {level>=N} + {guardLevel in 档位}`
 * 三选一/组合的 `when` 条件，不需要后端补丁。§13.2 里唯一还欠缺的是一个
 * 「现成谓词」（如 `user.medal.wearing`）省得前端自己拼三条件，这是
 * 便利性优化，不是功能缺口，因此没有计入悬空清单。
 */
import type { Action, Aggregate, Condition, Rule, RuleView } from '@/api/rule-types'
import { PK_RULE_NAME, PK_VISIT_RULE_NAME } from '@/components/PkPanel.vue'

/** 进房欢迎规则的固定名字，前端靠它从规则列表里认领已保存的配置。 */
export const ENTER_RULE_NAME = '内置/进房欢迎'
/** 礼物答谢规则的固定名字。 */
export const GIFT_RULE_NAME = '内置/礼物答谢'

const ENTER_ON = 'user_enter'
const GIFT_ON = 'gift'

/**
 * claimRule 按规则名从列表里认领一条规则。
 *
 * 这是本页加载逻辑的核心：认领不到就是「还没配过」，返回 null 让调用方
 * 落回默认值，而不是报错——一个全新的绑定本来就不该有这两条规则。
 */
export function claimRule(rules: Rule[], name: string): Rule | null {
  return rules.find((r) => r.name === name) ?? null
}

// ---- 大航海档位：event.GuardGovernor=1 / GuardAdmiral=2 / GuardCaptain=3。
// 数值越小档位越高（总督最贵、数值最小），"及以上" 因此要用 in 一组数值
// 而不是简单的 gte——gte 在这种反向编号下语义会反过来。 ----

export type GuardTier = 'captain' | 'admiral' | 'governor'

/** 每个档位对应「这个档位及以上」包含的 guardLevel 数值集合。 */
const GUARD_TIER_VALUES: Record<GuardTier, number[]> = {
  captain: [1, 2, 3], // 舰长即可：三档都算
  admiral: [1, 2], // 提督及以上
  governor: [1], // 仅总督
}

export const GUARD_TIER_OPTIONS: { label: string; value: GuardTier }[] = [
  { label: '舰长即可（不限档位）', value: 'captain' },
  { label: '提督及以上', value: 'admiral' },
  { label: '仅总督', value: 'governor' },
]

function arraysEqualUnordered(a: number[], b: unknown): boolean {
  if (!Array.isArray(b)) return false
  if (a.length !== b.length) return false
  const sortedA = [...a].sort()
  const sortedB = [...b].sort()
  return sortedA.every((v, i) => v === sortedB[i])
}

// ---- 进房欢迎 ----

export interface EnterFilter {
  /** 只欢迎佩戴粉丝牌的用户（口径见 §13.2：已点亮 && 是本房间的牌子）。 */
  wearMedalOnly: boolean
  /** 粉丝牌等级下限，null 表示不限。 */
  minMedalLevel: number | null
  /** 只欢迎大航海用户。 */
  guardOnly: boolean
  /** 大航海档位下限，仅 guardOnly 为真时生效。 */
  guardTier: GuardTier
}

export function defaultEnterFilter(): EnterFilter {
  return { wearMedalOnly: false, minMedalLevel: null, guardOnly: false, guardTier: 'captain' }
}

/**
 * buildEnterCondition 把筛选草稿拼成一棵 `when` 条件树。
 *
 * roomId 用于粉丝牌的「本房间」判定——§13.2 的口径要求牌子必须是本房间
 * 主播的，否则「别家的牌子也算」，与用户原话不符。
 */
export function buildEnterCondition(f: EnterFilter, roomId: string): Condition | undefined {
  const leaves: Condition[] = []

  if (f.wearMedalOnly) {
    leaves.push({ field: 'user.medal.isLighted', op: 'eq', value: true })
    if (roomId) {
      leaves.push({ field: 'user.medal.roomId', op: 'eq', value: roomId })
    }
  }
  if (f.minMedalLevel !== null && f.minMedalLevel > 0) {
    leaves.push({ field: 'user.medal.level', op: 'gte', value: f.minMedalLevel })
  }
  if (f.guardOnly) {
    leaves.push({ field: 'user.guardLevel', op: 'in', value: GUARD_TIER_VALUES[f.guardTier] })
  }

  if (leaves.length === 0) return undefined
  if (leaves.length === 1) return leaves[0]
  return { all: leaves }
}

/** parseEnterFilter 是 buildEnterCondition 的逆过程，供加载已保存配置用。 */
export function parseEnterFilter(condition: Condition | undefined): EnterFilter {
  const filter = defaultEnterFilter()
  if (!condition) return filter

  const leaves = condition.all ?? (condition.field ? [condition] : [])
  for (const leaf of leaves) {
    if (leaf.field === 'user.medal.isLighted' && leaf.op === 'eq' && leaf.value === true) {
      filter.wearMedalOnly = true
    }
    if (leaf.field === 'user.medal.level' && leaf.op === 'gte' && typeof leaf.value === 'number') {
      filter.minMedalLevel = leaf.value
    }
    if (leaf.field === 'user.guardLevel' && leaf.op === 'in') {
      const tier = (Object.keys(GUARD_TIER_VALUES) as GuardTier[]).find((k) =>
        arraysEqualUnordered(GUARD_TIER_VALUES[k], leaf.value),
      )
      if (tier) {
        filter.guardOnly = true
        filter.guardTier = tier
      }
    }
  }
  return filter
}

// ---- 合并/去重窗口，进房与礼物共用同一套草稿形状 ----

/**
 * PickMode 直接对应 spec.Action.Pick 的取值（`""`/`"random"`/`"sequential"`）。
 * 草稿态不使用空字符串——`parseXxxDraft` 把「空」也当作 `'random'`
 * 处理（与历史配置兼容），保存时统一显式写出 `'random'` 或 `'sequential'`。
 */
export type PickMode = 'random' | 'sequential'

export const PICK_MODE_OPTIONS: { label: string; value: PickMode }[] = [
  { label: '随机抽取', value: 'random' },
  { label: '轮询（按顺序循环）', value: 'sequential' },
]

/** parsePickMode 把后端的 pick 字段（可能是空字符串或缺省）还原成草稿用的 PickMode。 */
function parsePickMode(pick: string | undefined): PickMode {
  return pick === 'sequential' ? 'sequential' : 'random'
}

/** 把秒数转成 spec.Duration 要求的字符串形式，如 "180s"。 */
function secondsToDuration(seconds: number): string {
  return `${seconds}s`
}

const DURATION_UNIT_SECONDS: Record<string, number> = {
  ns: 1e-9,
  us: 1e-6,
  µs: 1e-6,
  ms: 1e-3,
  s: 1,
  m: 60,
  h: 3600,
}

/**
 * secondsFromDuration 解析 "1m30s" 这类复合时长字符串，返回总秒数。
 * 解析不出任何单位时回落到 fallback——例如遇到未来才会出现的写法。
 */
export function secondsFromDuration(d: string | undefined, fallback: number): number {
  if (!d) return fallback
  const re = /(\d+(?:\.\d+)?)(ns|us|µs|ms|s|m|h)/g
  let total = 0
  let matched = false
  let match: RegExpExecArray | null
  while ((match = re.exec(d)) !== null) {
    matched = true
    total += Number(match[1]) * (DURATION_UNIT_SECONDS[match[2]] ?? 0)
  }
  return matched ? total : fallback
}

// ---- 进房欢迎草稿 ----

export type EnterGroupMode = 'merge' | 'dedupe'

/** merge → 按类型合并（多人合并为一条）；dedupe → 按用户去重（仅频次限制，不合并多人）。 */
const ENTER_GROUP_MODE_BY: Record<EnterGroupMode, string> = { merge: 'type', dedupe: 'user' }
const ENTER_BY_TO_GROUP_MODE: Record<string, EnterGroupMode> = { type: 'merge', user: 'dedupe' }

export const ENTER_GROUP_MODE_OPTIONS: { label: string; value: EnterGroupMode }[] = [
  { label: '窗口内多人合并为一条欢迎', value: 'merge' },
  { label: '仅按用户去重（不合并，用于频次限制）', value: 'dedupe' },
]

export interface EnterDraft {
  enabled: boolean
  filter: EnterFilter
  groupMode: EnterGroupMode
  windowSeconds: number
  minCount: number
  pickMode: PickMode
  singleTemplates: string[]
  multiTemplates: string[]
}

export function defaultEnterDraft(): EnterDraft {
  return {
    enabled: true,
    filter: defaultEnterFilter(),
    groupMode: 'merge',
    windowSeconds: 180,
    minCount: 2,
    pickMode: 'random',
    singleTemplates: ['欢迎 {{.user.username}} 来到直播间~'],
    multiTemplates: ['欢迎 {{join .users "、"}} 等 {{.count}} 位朋友来到直播间~'],
  }
}

function buildAggregateCommon(a: {
  by: string
  windowSeconds: number
  minCount: number
  applyMinCount: boolean
}): Aggregate {
  const agg: Aggregate = { window: secondsToDuration(a.windowSeconds), by: a.by }
  if (a.applyMinCount && a.minCount > 1) {
    agg.minCount = a.minCount
  }
  return agg
}

/**
 * buildEnterRule 把草稿组装成 spec.Rule。
 *
 * `pick`/`templateMulti` 都直接写进 action：进房欢迎的规则始终带
 * `aggregate`（见上面的 buildAggregateCommon 调用），不会撞上后端
 * 「只配 templateMulti 又没有 aggregate」的校验，见文件头 P4-3 说明。
 */
export function buildEnterRule(draft: EnterDraft, roomId: string): Rule {
  const rule: Rule = {
    name: ENTER_RULE_NAME,
    enabled: draft.enabled,
    on: [ENTER_ON],
    aggregate: buildAggregateCommon({
      by: ENTER_GROUP_MODE_BY[draft.groupMode],
      windowSeconds: draft.windowSeconds,
      minCount: draft.minCount,
      applyMinCount: draft.groupMode === 'merge',
    }),
    do: [
      {
        type: 'danmaku',
        template: draft.singleTemplates.filter((t) => t.trim() !== ''),
        templateMulti: draft.multiTemplates.filter((t) => t.trim() !== ''),
        pick: draft.pickMode,
      },
    ],
  }
  const when = buildEnterCondition(draft.filter, roomId)
  if (when) rule.when = when
  return rule
}

/** parseEnterDraft 是 buildEnterRule 的逆过程，供「认领」到已保存规则时用。 */
export function parseEnterDraft(rule: Rule | null): EnterDraft {
  const draft = defaultEnterDraft()
  if (!rule) return draft

  draft.enabled = rule.enabled ?? true
  draft.filter = parseEnterFilter(rule.when)

  const agg = rule.aggregate
  if (agg) {
    draft.groupMode = ENTER_BY_TO_GROUP_MODE[agg.by] ?? 'merge'
    draft.windowSeconds = secondsFromDuration(agg.window, draft.windowSeconds)
    if (agg.minCount !== undefined) draft.minCount = agg.minCount
  }

  const action = findDanmakuAction(rule.do)
  if (action?.template && action.template.length > 0) {
    draft.singleTemplates = action.template
  }
  if (action?.templateMulti && action.templateMulti.length > 0) {
    draft.multiTemplates = action.templateMulti
  }
  draft.pickMode = parsePickMode(action?.pick)
  return draft
}

function findDanmakuAction(actions: Action[] | undefined): Action | undefined {
  return actions?.find((a) => a.type === 'danmaku')
}

// ---- 礼物答谢草稿 ----

export type GiftGroupMode = 'merge' | 'dedupeGift'

/** merge → 按类型合并（多人多礼物合并为一条）；dedupeGift → 同用户同礼物计数累加。 */
const GIFT_GROUP_MODE_BY: Record<GiftGroupMode, string> = { merge: 'type', dedupeGift: 'gift' }
const GIFT_BY_TO_GROUP_MODE: Record<string, GiftGroupMode> = { type: 'merge', gift: 'dedupeGift' }

export const GIFT_GROUP_MODE_OPTIONS: { label: string; value: GiftGroupMode }[] = [
  { label: '窗口内全部合并为一条答谢（可多人多礼物）', value: 'merge' },
  { label: '同一用户同一礼物计数累加（不跨礼物合并）', value: 'dedupeGift' },
]

export interface GiftDraft {
  enabled: boolean
  groupMode: GiftGroupMode
  windowSeconds: number
  minCount: number
  pickMode: PickMode
  templates: string[]
  /**
   * 盲盒礼物单列一类，不并入常规答谢——真实生效：勾选后通用答谢规则
   * （GIFT_RULE_NAME）会加上 `when: gift.isBlindBox == false` 排除盲盒，
   * 同时启用独立的 `BLINDBOX_RULE_NAME` 规则专门答谢盲盒，见
   * `buildBlindBoxRule`。取消勾选时盲盒仍会混进通用答谢。
   *
   * **默认值是 `true`，不是可选偏好**——计划文件的硬性要求、用户原话
   * 「盲盒类单独计算」说的是正确行为该是什么样，不是"某些人可能想要
   * 分开"。开关留着是给想混着谢的人一个退出的自由，但新用户第一次
   * 打开这页看到的必须是正确行为。
   */
  blindBoxSeparate: boolean
  /**
   * 盲盒盈亏统计——决定盲盒答谢模板是否带上盈亏信息（`blindBox.profit`/
   * `profitYuan`）。只在 blindBoxSeparate 为 true 时才有实际效果
   * （盲盒规则本身没启用，这项开不开都不会触发）。
   */
  blindBoxProfitTracking: boolean
}

export function defaultGiftDraft(): GiftDraft {
  return {
    enabled: true,
    groupMode: 'merge',
    windowSeconds: 20,
    minCount: 2,
    pickMode: 'random',
    templates: ['感谢 {{join .users "、"}} 的 {{join .gifts "、"}}，您的支持就是对主播最大的鼓励'],
    blindBoxSeparate: true,
    blindBoxProfitTracking: false,
  }
}

/**
 * buildGiftRule 组装通用礼物答谢规则。
 *
 * blindBoxSeparate 勾选时加一条 `when: gift.isBlindBox == false`——
 * 不排除的话盲盒会被这条通用规则按「爆出来的那个礼物」答谢一遍，
 * 再被 `buildBlindBoxRule` 按盲盒身份答谢第二遍，同一次投喂谢两次，
 * 而且第一次说的还是错的（用户送的是盲盒，不是里面爆出来的东西）——
 * 与 `config.example.yaml` 里「通用礼物答谢」示范规则的排除条件同一个
 * 写法。
 */
export function buildGiftRule(draft: GiftDraft): Rule {
  const rule: Rule = {
    name: GIFT_RULE_NAME,
    enabled: draft.enabled,
    on: [GIFT_ON],
    aggregate: buildAggregateCommon({
      by: GIFT_GROUP_MODE_BY[draft.groupMode],
      windowSeconds: draft.windowSeconds,
      minCount: draft.minCount,
      applyMinCount: draft.groupMode === 'merge',
    }),
    do: [
      {
        type: 'danmaku',
        template: draft.templates.filter((t) => t.trim() !== ''),
        pick: draft.pickMode,
      },
    ],
  }
  if (draft.blindBoxSeparate) {
    rule.when = { field: 'gift.isBlindBox', op: 'eq', value: false }
  }
  return rule
}

/** 盲盒答谢规则的固定名字——与进房欢迎/礼物答谢等其余内置规则同一套按 name 认领的机制。 */
export const BLINDBOX_RULE_NAME = '内置/盲盒答谢'
const BLINDBOX_ON = 'gift'

/**
 * 两条固定模板，分别对应「带盈亏」/「不带盈亏」——这里不用 TemplateList
 * 给用户自由editable，是因为 `parseBlindBoxDraft` 要靠精确匹配这两个
 * 字符串反推 blindBoxProfitTracking 这个布尔值（规则本身没有单独一个
 * 字段记录这个开关，只能从模板内容反推）。真给用户开放自由编辑的话，
 * 保存后重新加载这个开关会因为文本对不上而丢失状态。
 */
const BLIND_BOX_TEMPLATE_WITH_PROFIT =
  '感谢 {{simplifyName .user.username}} 的 {{.blindBox.name}} x{{.blindBox.count}}，' +
  '{{if gt .blindBox.profit 0}}赚了{{else}}亏了{{end}} {{.blindBox.profitYuan}} 元！'
const BLIND_BOX_TEMPLATE_WITHOUT_PROFIT =
  '感谢 {{simplifyName .user.username}} 开出了 {{.blindBox.count}} 次 {{.blindBox.name}}！'

/**
 * buildBlindBoxRule 组装盲盒答谢规则，语义照抄 `config.example.yaml`
 * 里的示范规则：`when: gift.isBlindBox == true` + `aggregate: {by:
 * blindBox}`（键=类型+uid+盲盒名称，交叉送不同盲盒必须分开统计，见
 * `server/internal/rules/aggregate.go` AggregateByBlindBox 的注释）。
 * 复用礼物答谢的合并窗口秒数，不单开一个输入框——没有必要为一个联动
 * 特性单独占一块界面。
 */
export function buildBlindBoxRule(draft: GiftDraft): Rule {
  return {
    name: BLINDBOX_RULE_NAME,
    enabled: draft.blindBoxSeparate,
    on: [BLINDBOX_ON],
    when: { field: 'gift.isBlindBox', op: 'eq', value: true },
    aggregate: buildAggregateCommon({
      by: 'blindBox',
      windowSeconds: draft.windowSeconds,
      minCount: 1,
      applyMinCount: false,
    }),
    do: [
      {
        type: 'danmaku',
        template: [
          draft.blindBoxProfitTracking ? BLIND_BOX_TEMPLATE_WITH_PROFIT : BLIND_BOX_TEMPLATE_WITHOUT_PROFIT,
        ],
      },
    ],
  }
}

/** parseBlindBoxDraft 从已保存的盲盒规则里还原两个开关，写回 draft（就地修改）。 */
function parseBlindBoxDraft(draft: GiftDraft, rule: Rule | null) {
  if (!rule) return
  draft.blindBoxSeparate = rule.enabled ?? false
  const action = findDanmakuAction(rule.do)
  draft.blindBoxProfitTracking = action?.template?.[0] === BLIND_BOX_TEMPLATE_WITH_PROFIT
}

/** parseGiftDraft 是 buildGiftRule/buildBlindBoxRule 的逆过程，供「认领」到已保存规则时用。 */
export function parseGiftDraft(rule: Rule | null, blindBoxRule: Rule | null): GiftDraft {
  const draft = defaultGiftDraft()
  if (rule) {
    draft.enabled = rule.enabled ?? true

    const agg = rule.aggregate
    if (agg) {
      draft.groupMode = GIFT_BY_TO_GROUP_MODE[agg.by] ?? 'merge'
      draft.windowSeconds = secondsFromDuration(agg.window, draft.windowSeconds)
      if (agg.minCount !== undefined) draft.minCount = agg.minCount
    }

    const action = findDanmakuAction(rule.do)
    if (action?.template && action.template.length > 0) {
      draft.templates = action.template
    }
    draft.pickMode = parsePickMode(action?.pick)
  }
  parseBlindBoxDraft(draft, blindBoxRule)
  return draft
}

// ---- 轮播消息草稿（真功能：spec.Rule.Schedule 由 cron 驱动） ----

export type ScheduleMode = 'interval' | 'cron'

export const SCHEDULE_MODE_OPTIONS: { label: string; value: ScheduleMode }[] = [
  { label: '按固定间隔（分钟）', value: 'interval' },
  { label: '自定义 cron 表达式', value: 'cron' },
]

// 按分钟间隔生成 6 段 cron（秒 分 时 日 月 周），如每 10 分钟一次写成
// "0 " + "*/10" + " * * * *"。注意：这条注释故意不用 /** */ 块注释、
// 不把 "*/N" 直接连着写出来——星号加斜杠会被 TS 解析成块注释的结束符，
// 现网就因为这个把后面一大段代码解析成了字符串字面量。
function intervalMinutesToCron(minutes: number): string {
  const m = Math.max(1, Math.round(minutes))
  return `0 */${m} * * * *`
}

/** 识别 intervalMinutesToCron 生成的形状，用于加载时反推 intervalMinutes。 */
const INTERVAL_CRON_RE = /^0 \*\/(\d+) \* \* \* \*$/

export const BROADCAST_RULE_NAME = '内置/轮播消息'

export interface BroadcastDraft {
  enabled: boolean
  scheduleMode: ScheduleMode
  intervalMinutes: number
  cronExpr: string
  pickMode: PickMode
  templates: string[]
}

export function defaultBroadcastDraft(): BroadcastDraft {
  return {
    enabled: true,
    scheduleMode: 'interval',
    intervalMinutes: 10,
    cronExpr: '0 */10 * * * *',
    pickMode: 'random',
    templates: ['感谢大家的观看，记得点关注不迷路~', '直播间有什么问题欢迎在弹幕里提出~'],
  }
}

/** buildBroadcastSchedule 把两种模式统一折成一个 cron 字符串。 */
export function buildBroadcastSchedule(draft: BroadcastDraft): string {
  return draft.scheduleMode === 'interval'
    ? intervalMinutesToCron(draft.intervalMinutes)
    : draft.cronExpr.trim()
}

/**
 * buildBroadcastRule 组装轮播规则。
 *
 * **只给 `schedule`，绝不给 `on`。** `rules.Rule.Validate()`
 * （`server/internal/rules/rule.go` 第 61-68 行）把 on 与 schedule
 * 定成互斥：两个都给会被后端直接拒收（422）。这里连 `on` 字段都不写
 * 进返回的对象——不是留空字符串，是整个属性不存在——避免哪天不小心
 * 在别处又给它塞了值。
 */
export function buildBroadcastRule(draft: BroadcastDraft): Rule {
  return {
    name: BROADCAST_RULE_NAME,
    enabled: draft.enabled,
    schedule: buildBroadcastSchedule(draft),
    do: [
      {
        type: 'danmaku',
        template: draft.templates.filter((t) => t.trim() !== ''),
        pick: draft.pickMode,
      },
    ],
  }
}

export function parseBroadcastDraft(rule: Rule | null): BroadcastDraft {
  const draft = defaultBroadcastDraft()
  if (!rule) return draft

  draft.enabled = rule.enabled ?? true
  if (rule.schedule) {
    const m = INTERVAL_CRON_RE.exec(rule.schedule)
    if (m) {
      draft.scheduleMode = 'interval'
      draft.intervalMinutes = Number(m[1])
    } else {
      draft.scheduleMode = 'cron'
      draft.cronExpr = rule.schedule
    }
  }

  const action = findDanmakuAction(rule.do)
  if (action?.template && action.template.length > 0) {
    draft.templates = action.template
  }
  draft.pickMode = parsePickMode(action?.pick)
  return draft
}

// ---- 关注答谢 / 分享答谢 / 上舰答谢：三者形状相同（开关 + 模板），共用一套草稿 ----

export interface SimpleThanksDraft {
  enabled: boolean
  templates: string[]
}

function defaultSimpleThanksDraft(templates: string[]): SimpleThanksDraft {
  return { enabled: true, templates }
}

function buildSimpleThanksRule(name: string, on: string, draft: SimpleThanksDraft): Rule {
  return {
    name,
    enabled: draft.enabled,
    on: [on],
    do: [{ type: 'danmaku', template: draft.templates.filter((t) => t.trim() !== '') }],
  }
}

function parseSimpleThanksDraft(rule: Rule | null, defaultTemplates: string[]): SimpleThanksDraft {
  const draft = defaultSimpleThanksDraft(defaultTemplates)
  if (!rule) return draft
  draft.enabled = rule.enabled ?? true
  const action = findDanmakuAction(rule.do)
  if (action?.template && action.template.length > 0) {
    draft.templates = action.template
  }
  return draft
}

export const FOLLOW_RULE_NAME = '内置/关注答谢'
const FOLLOW_ON = 'user_follow'

/** event.UserFollow 只有一个 User 字段（server/internal/event/payload.go 第 66-67 行），模板只需要 user.username。 */
export function defaultFollowDraft(): SimpleThanksDraft {
  return defaultSimpleThanksDraft(['感谢 {{.user.username}} 的关注，欢迎常来玩~'])
}
export function buildFollowRule(draft: SimpleThanksDraft): Rule {
  return buildSimpleThanksRule(FOLLOW_RULE_NAME, FOLLOW_ON, draft)
}
export function parseFollowDraft(rule: Rule | null): SimpleThanksDraft {
  return parseSimpleThanksDraft(rule, defaultFollowDraft().templates)
}

export const SHARE_RULE_NAME = '内置/分享答谢'
const SHARE_ON = 'user_share'

/** event.UserShare 同样只有一个 User 字段。 */
export function defaultShareDraft(): SimpleThanksDraft {
  return defaultSimpleThanksDraft(['感谢 {{.user.username}} 分享了直播间，谢谢支持~'])
}
export function buildShareRule(draft: SimpleThanksDraft): Rule {
  return buildSimpleThanksRule(SHARE_RULE_NAME, SHARE_ON, draft)
}
export function parseShareDraft(rule: Rule | null): SimpleThanksDraft {
  return parseSimpleThanksDraft(rule, defaultShareDraft().templates)
}

export const GUARD_RULE_NAME = '内置/上舰答谢'
const GUARD_ON = 'guard_buy'

/**
 * event.GuardBuy 载荷齐全：GuardLevel/GuardName/Count/Price/IsRenew
 * （server/internal/event/payload.go 第 40-47 行），rules/vars.go 第
 * 50-55 行把它们展开成 guard.level/name/count/price/isRenew。
 * 新购与续费的区分直接写进模板的 {{if}}，不需要拆成两条规则。
 */
export function defaultGuardDraft(): SimpleThanksDraft {
  return defaultSimpleThanksDraft([
    '感谢 {{.user.username}} {{if .guard.isRenew}}续费{{else}}开通{{end}} ' +
      '{{.guard.count}} 个月{{.guard.name}}，感谢老板的支持！',
  ])
}
export function buildGuardRule(draft: SimpleThanksDraft): Rule {
  return buildSimpleThanksRule(GUARD_RULE_NAME, GUARD_ON, draft)
}
export function parseGuardDraft(rule: Rule | null): SimpleThanksDraft {
  return parseSimpleThanksDraft(rule, defaultGuardDraft().templates)
}

/**
 * 本页管的九条内置规则名——合并保存时用来从「现有全部规则」里挑出该被
 * 本页替换的那些。**必须与 `Custom.vue` 的 `BUILTIN_RULE_NAMES` 保持
 * 恰好相等**（钉在 `Custom.test.ts` 里）：不相等的话，新增的内置规则若
 * 只改了这一侧，`Custom.vue` 会把它当自定义规则加载进草稿、并在保存时
 * 用草稿版本覆盖它——静默的数据错误，不报任何错。
 *
 * P4-4 Task 7 新增两条：`BLINDBOX_RULE_NAME`（盲盒单独答谢，
 * `giftDraft.blindBoxSeparate` 接上真实规则）与 `PK_VISIT_RULE_NAME`
 * （PK 串门欢迎，`PkPanel.vue` 的 `visitGreetingEnabled` 接上真实规则）。
 */
export const OWNED_RULE_NAMES = [
  ENTER_RULE_NAME,
  GIFT_RULE_NAME,
  BLINDBOX_RULE_NAME,
  PK_RULE_NAME,
  PK_VISIT_RULE_NAME,
  BROADCAST_RULE_NAME,
  FOLLOW_RULE_NAME,
  SHARE_RULE_NAME,
  GUARD_RULE_NAME,
]

export type { RuleView }
</script>

<script setup lang="ts">
import { computed, reactive, ref, watch } from 'vue'
import {
  NAlert,
  NCard,
  NCheckbox,
  NCollapse,
  NCollapseItem,
  NEmpty,
  NInput,
  NInputNumber,
  NRadio,
  NRadioGroup,
  NSelect,
  NSpin,
  NSwitch,
  NTag,
  NTooltip,
  useMessage,
} from 'naive-ui'
import { ApiError, request } from '@/api'
import { useAuthStore } from '@/stores/auth'
import { useBindingsStore } from '@/stores/bindings'
import { useDraft } from '@/composables/useDraft'
import SaveBar from '@/components/SaveBar.vue'
import TemplateList from '@/components/TemplateList.vue'
import PermissionWarning from '@/components/PermissionWarning.vue'
import PkPanel, {
  buildPkRule,
  buildPkVisitRule,
  defaultPkDraft,
  parsePkDraft,
  type PkDraft,
} from '@/components/PkPanel.vue'

const auth = useAuthStore()
const bindings = useBindingsStore()
const message = useMessage()

/**
 * 缺 rule:write 时的警告条。只有 rule:read 的成员能把整页规则配完，
 * 直到点「保存并生效」才被后端 403 打回——白干一整轮配置。与
 * Moderation.vue 缺 user:block 同一套约定：只警告，不锁面板。
 */
const missingWritePerm = computed(() => {
  const b = bindings.current
  return b !== null && !auth.hasPerm(b, 'rule:write')
})

/**
 * 礼物模板可用变量的提示文案，字面量含 `{{ }}`。
 *
 * **不能直接把它们当字符串字面量写进 `<template>` 的插值里**——如
 * `{{ '{{join .users "、"}}' }}`——Vue 的插值解析对 `{{`/`}}` 是非贪婪
 * 匹配，会在字符串字面量内部的 `}}` 处提前收口，导致模板编译失败。
 * 挪到这里作为普通变量，模板里只留一层 `{{ TEMPLATE_VAR_HINT.xxx }}`，
 * 源码里就不会出现裸的 `{{`/`}}` 字符对。
 */
const TEMPLATE_VAR_HINT = {
  users: '{{join .users "、"}}',
  giftName: '{{.gift.name}}',
  count: '{{.count}}',
  gifts: '{{join .gifts "、"}}',
}

/** 上舰答谢模板可用变量提示，同样要挪成变量，理由见上方 TEMPLATE_VAR_HINT 的注释。 */
const GUARD_TEMPLATE_VAR_HINT = {
  username: '{{.user.username}}',
  guardName: '{{.guard.name}}',
  guardCount: '{{.guard.count}}',
  renewCondition: '{{if .guard.isRenew}}续费{{else}}开通{{end}}',
}

const loading = ref(false)

const enterDraft = reactive<EnterDraft>(defaultEnterDraft())
const giftDraft = reactive<GiftDraft>(defaultGiftDraft())
const pkDraft = reactive<PkDraft>(defaultPkDraft())
const broadcastDraft = reactive<BroadcastDraft>(defaultBroadcastDraft())
const followDraft = reactive<SimpleThanksDraft>(defaultFollowDraft())
const shareDraft = reactive<SimpleThanksDraft>(defaultShareDraft())
const guardDraft = reactive<SimpleThanksDraft>(defaultGuardDraft())

/**
 * 上一次加载完成时的草稿快照，用来算 dirty——比对内容而不是加监听器逐字段设标记。
 *
 * **初值必须是当前默认草稿的序列化结果，不能是空字符串。** 页面刚挂载、
 * 还没选中直播间（或 loadRules 还没跑完）时，各草稿已经是默认值，
 * 若 snapshot 初值是 ''，一比对就不相等，dirty 会在用户什么都没做的
 * 情况下先亮一次「有未保存的改动」。
 */
function currentDraftsSnapshot() {
  return JSON.stringify({
    enter: enterDraft,
    gift: giftDraft,
    pk: pkDraft,
    broadcast: broadcastDraft,
    follow: followDraft,
    share: shareDraft,
    guard: guardDraft,
  })
}

const builtEnterRule = computed(() => buildEnterRule(enterDraft, bindings.current?.roomId ?? ''))
const builtGiftRule = computed(() => buildGiftRule(giftDraft))
const builtBlindBoxRule = computed(() => buildBlindBoxRule(giftDraft))
const builtBroadcastRule = computed(() => buildBroadcastRule(broadcastDraft))
const builtFollowRule = computed(() => buildFollowRule(followDraft))
const builtShareRule = computed(() => buildShareRule(shareDraft))
const builtGuardRule = computed(() => buildGuardRule(guardDraft))

/** onPkDraftUpdate 接住 PkPanel emit 出的整份新草稿，写回本页持有的 reactive 对象。 */
function onPkDraftUpdate(next: PkDraft) {
  Object.assign(pkDraft, next)
}

// OWNED_RULE_NAMES 现在定义在上面的 <script lang="ts"> 块里并导出（供
// Custom.test.ts 钉住与 BUILTIN_RULE_NAMES 的相等性），<script setup>
// 与它共享模块作用域，直接用即可，不需要再 import 或重新声明。

/** buildAllRules 组装本页管的全部九条规则，保存时整批送去跟其余页面的规则合并。 */
function buildAllRules(): Rule[] {
  return [
    buildEnterRule(enterDraft, bindings.current?.roomId ?? ''),
    buildGiftRule(giftDraft),
    buildBlindBoxRule(giftDraft),
    buildPkRule(pkDraft),
    buildPkVisitRule(pkDraft),
    buildBroadcastRule(broadcastDraft),
    buildFollowRule(followDraft),
    buildShareRule(shareDraft),
    buildGuardRule(guardDraft),
  ]
}

const { dirty, saving, partialFailureMessage, markSaved, save } = useDraft({
  bindingId: () => bindings.current?.id ?? null,
  snapshot: currentDraftsSnapshot,
  isOwned: (name) => OWNED_RULE_NAMES.includes(name),
  buildRules: buildAllRules,
})

async function loadRules() {
  const b = bindings.current
  if (!b) return
  loading.value = true
  try {
    const rules = await request<RuleView[]>('GET', `/api/bindings/${b.id}/rules`)
    Object.assign(enterDraft, parseEnterDraft(claimRule(rules, ENTER_RULE_NAME)))
    Object.assign(
      giftDraft,
      parseGiftDraft(claimRule(rules, GIFT_RULE_NAME), claimRule(rules, BLINDBOX_RULE_NAME)),
    )
    Object.assign(
      pkDraft,
      parsePkDraft(claimRule(rules, PK_RULE_NAME), claimRule(rules, PK_VISIT_RULE_NAME)),
    )
    Object.assign(broadcastDraft, parseBroadcastDraft(claimRule(rules, BROADCAST_RULE_NAME)))
    Object.assign(followDraft, parseFollowDraft(claimRule(rules, FOLLOW_RULE_NAME)))
    Object.assign(shareDraft, parseShareDraft(claimRule(rules, SHARE_RULE_NAME)))
    Object.assign(guardDraft, parseGuardDraft(claimRule(rules, GUARD_RULE_NAME)))
    markSaved()
    // 换了绑定就把上一个绑定的「保存了一半」提示清掉。
    //
    // 不清的话：在甲房间保存失败 -> 切到乙房间 -> 乙房间顶上挂着甲房间的
    // 警告。更糟的是操作者在乙房间把它关掉，甲房间那个**未解决**的重载
    // 失败信号也跟着没了——而甲房间的引擎确实还在跑旧规则。
    partialFailureMessage.value = null
  } catch (e) {
    message.error(e instanceof ApiError ? e.message : '加载规则失败')
  } finally {
    loading.value = false
  }
}

// 切换直播间要重新认领——上一个直播间的草稿不该带到下一个直播间去。
watch(
  () => bindings.currentId,
  () => void loadRules(),
  { immediate: true },
)

/**
 * onSave 接 useDraft 的保存流程：GET 现有规则 → 合并 → PUT → POST reload。
 *
 * 两步都成功才提示「已保存并生效」。第 2 步（reload）失败时 useDraft 内部
 * 已经把 partialFailureMessage 设好、dirty 也没有归假（见 useDraft.ts
 * 文件头说明），这里的 catch 只负责把后端原文以 toast 形式再提醒一遍——
 * 「仍在用上一份配置运行」这句安抚必须原样带到用户面前。
 */
async function onSave() {
  try {
    await save()
    message.success('已保存并生效')
  } catch (e) {
    message.error(e instanceof ApiError ? e.message : '保存失败')
  }
}

/** 手动关掉「已保存到数据库，但重载失败」的持久提示——只是收起提示条，不影响 dirty。 */
function dismissPartialFailure() {
  partialFailureMessage.value = null
}
</script>

<template>
  <div class="danmaku-page">
    <div class="page-header">
      <h2>弹幕姬</h2>
      <SaveBar :dirty="dirty" :saving="saving" @save="onSave" />
    </div>

    <!--
      第三态：PUT 写库成功、但 POST reload 失败——库已经改了，引擎还在跑
      旧配置。dirty 这时候不会归假（见 useDraft.ts），单靠 SaveBar 的
      「有未保存的改动」不足以说明这一半保存了、一半没生效，所以单独给
      一条持久提示，不只是转瞬即逝的 toast。
    -->
    <NAlert
      v-if="partialFailureMessage"
      type="warning"
      title="已保存到数据库，但重载失败"
      closable
      class="partial-failure-alert"
      @close="dismissPartialFailure"
    >
      {{ partialFailureMessage }}
    </NAlert>

    <NEmpty v-if="!bindings.current" description="请先在顶部选择一个直播间" />

    <template v-else>
      <PermissionWarning
        v-if="missingWritePerm"
        text="你在这个直播间没有 rule:write 权限，保存会被拒绝"
      />

      <NSpin :show="loading">
        <!-- ==================== 进房欢迎 ==================== -->
        <NCard title="进房欢迎" class="section-card">
          <template #header-extra>
            <NSwitch v-model:value="enterDraft.enabled" />
          </template>

          <h4>欢迎筛选</h4>
          <div class="row">
            <NCheckbox v-model:checked="enterDraft.filter.wearMedalOnly">
              只欢迎佩戴粉丝牌的用户
            </NCheckbox>
            <NTooltip>
              <template #trigger>
                <NTag type="info" size="small">自动拼条件</NTag>
              </template>
              口径（设计文档 §13.2）：粉丝牌已点亮 且 是本房间主播的牌子——
              两条都满足才算「佩戴」，保存时会自动拼成 user.medal.isLighted=true 且
              user.medal.roomId=本房间号， 不需要你自己拼条件，也不需要后端补丁。
            </NTooltip>
          </div>

          <div class="row">
            <span class="label">粉丝牌等级下限</span>
            <NInputNumber
              v-model:value="enterDraft.filter.minMedalLevel"
              :min="0"
              clearable
              placeholder="不限"
              style="width: 140px"
            />
          </div>

          <div class="row">
            <NCheckbox v-model:checked="enterDraft.filter.guardOnly">只欢迎大航海用户</NCheckbox>
            <NSelect
              v-model:value="enterDraft.filter.guardTier"
              :options="GUARD_TIER_OPTIONS"
              :disabled="!enterDraft.filter.guardOnly"
              style="width: 200px"
            />
          </div>

          <h4>频次 / 合并</h4>
          <NRadioGroup v-model:value="enterDraft.groupMode">
            <NRadio
              v-for="opt in ENTER_GROUP_MODE_OPTIONS"
              :key="opt.value"
              :value="opt.value"
              class="radio-item"
            >
              {{ opt.label }}
            </NRadio>
          </NRadioGroup>
          <div class="row">
            <span class="label">
              {{ enterDraft.groupMode === 'merge' ? '合并窗口（秒）' : '去重窗口（秒）' }}
            </span>
            <NInputNumber v-model:value="enterDraft.windowSeconds" :min="1" style="width: 140px" />
            <span class="label">最少合并人数</span>
            <NInputNumber
              v-model:value="enterDraft.minCount"
              :min="1"
              :disabled="enterDraft.groupMode !== 'merge'"
              style="width: 100px"
            />
          </div>

          <h4>欢迎语模板</h4>
          <div class="row">
            <NRadioGroup v-model:value="enterDraft.pickMode">
              <NRadio
                v-for="opt in PICK_MODE_OPTIONS"
                :key="opt.value"
                :value="opt.value"
                class="radio-item"
              >
                {{ opt.label }}
              </NRadio>
            </NRadioGroup>
          </div>

          <div class="template-block">
            <span class="label">单人欢迎语</span>
            <TemplateList v-model="enterDraft.singleTemplates" placeholder="单人欢迎语模板" />
          </div>
          <div class="template-block">
            <span class="label">多人合并欢迎语</span>
            <NTooltip>
              <template #trigger>
                <NTag type="info" size="small">仅合并触发时使用</NTag>
              </template>
              窗口内只有一人时仍然用上面的「单人欢迎语」；合并出多人（count >
              1）时才会改用这里的模板。 留空则不论单人多人都用「单人欢迎语」——与旧配置兼容。
            </NTooltip>
            <TemplateList v-model="enterDraft.multiTemplates" placeholder="多人合并欢迎语模板" />
          </div>

          <NCollapse class="preview-collapse">
            <NCollapseItem title="预览将要生成的规则 JSON（本地草稿，尚未保存）" name="preview">
              <pre class="json-preview">{{ JSON.stringify(builtEnterRule, null, 2) }}</pre>
            </NCollapseItem>
          </NCollapse>
        </NCard>

        <!-- ==================== 礼物答谢 ==================== -->
        <NCard title="礼物答谢" class="section-card">
          <template #header-extra>
            <NSwitch v-model:value="giftDraft.enabled" />
          </template>

          <h4>归类阈值</h4>
          <NRadioGroup v-model:value="giftDraft.groupMode">
            <NRadio
              v-for="opt in GIFT_GROUP_MODE_OPTIONS"
              :key="opt.value"
              :value="opt.value"
              class="radio-item"
            >
              {{ opt.label }}
            </NRadio>
          </NRadioGroup>
          <div class="row">
            <span class="label">
              {{ giftDraft.groupMode === 'merge' ? '合并窗口（秒）' : '累加窗口（秒）' }}
            </span>
            <NInputNumber v-model:value="giftDraft.windowSeconds" :min="1" style="width: 140px" />
            <span class="label">最少合并人数</span>
            <NInputNumber
              v-model:value="giftDraft.minCount"
              :min="1"
              :disabled="giftDraft.groupMode !== 'merge'"
              style="width: 100px"
            />
          </div>

          <h4>答谢语模板</h4>
          <div class="row">
            <NRadioGroup v-model:value="giftDraft.pickMode">
              <NRadio
                v-for="opt in PICK_MODE_OPTIONS"
                :key="opt.value"
                :value="opt.value"
                class="radio-item"
              >
                {{ opt.label }}
              </NRadio>
            </NRadioGroup>
          </div>
          <TemplateList v-model="giftDraft.templates" placeholder="答谢语模板" />
          <p class="hint">
            可用变量：<code>{{ TEMPLATE_VAR_HINT.users }}</code>
            （本轮参与的用户，需要用 join 拼接）、
            <code>{{ TEMPLATE_VAR_HINT.gifts }}</code>
            （本轮合并涉及的礼物名，去重后的列表，同样需要用 join 拼接）、
            <code>{{ TEMPLATE_VAR_HINT.giftName }}</code>
            （只取第一件礼物的名字，合并多种礼物时更推荐用上面的
            <code>gifts</code>）、
            <code>{{ TEMPLATE_VAR_HINT.count }}</code>
            等。
          </p>

          <h4>盲盒</h4>
          <div class="row">
            <NCheckbox v-model:checked="giftDraft.blindBoxSeparate">
              盲盒礼物单独一类，不并入常规答谢
            </NCheckbox>
            <NTooltip>
              <template #trigger>
                <NTag type="info" size="small">独立规则「内置/盲盒答谢」</NTag>
              </template>
              勾选后通用礼物答谢会加一条 <code>gift.isBlindBox == false</code> 排除盲盒，
              盲盒改由独立规则答谢（<code>when: gift.isBlindBox == true</code>，按
              「送礼人 + 盲盒名称」分开聚合——交叉送不同盲盒会分别结算，不会混在一起算出
              错误的盈亏）。不勾选则盲盒仍混在通用答谢里，跟以前一样。
            </NTooltip>
          </div>
          <div class="row">
            <NCheckbox v-model:checked="giftDraft.blindBoxProfitTracking">盲盒盈亏统计</NCheckbox>
            <NTooltip>
              <template #trigger>
                <NTag type="info" size="small">决定盲盒答谢模板内容</NTag>
              </template>
              勾选后盲盒答谢模板会带上本轮盈亏（1/100 电池换算成元，可能为负）；
              不勾选则只播报开出的次数，不透露盈亏。只在上面「单独一类」也勾选时才会
              真正触发——盲盒规则本身没启用的话，这项开不开都不会发弹幕。
            </NTooltip>
          </div>

          <NCollapse class="preview-collapse">
            <NCollapseItem title="预览将要生成的规则 JSON（本地草稿，尚未保存）" name="preview">
              <pre class="json-preview">{{ JSON.stringify(builtGiftRule, null, 2) }}</pre>
              <pre class="json-preview">{{ JSON.stringify(builtBlindBoxRule, null, 2) }}</pre>
            </NCollapseItem>
          </NCollapse>
        </NCard>

        <!-- ==================== PK 播报（整体悬空，见 PkPanel.vue 顶部注释） ==================== -->
        <PkPanel :model-value="pkDraft" @update:model-value="onPkDraftUpdate" />

        <!-- ==================== 轮播消息 ==================== -->
        <NCard title="轮播消息" class="section-card">
          <template #header-extra>
            <NSwitch v-model:value="broadcastDraft.enabled" />
          </template>

          <p class="hint">
            定时向直播间发送消息，与「进房欢迎」「礼物答谢」这类事件驱动不同——它由 cron
            表达式周期性触发（对应 spec.Rule 的 <code>schedule</code> 字段），
            不需要任何人进房或送礼。<code>on</code> 与 <code>schedule</code> 二选一，
            两个同时给后端会直接拒收，下面组装出的规则只会带 schedule。
          </p>

          <h4>发送频率</h4>
          <NRadioGroup v-model:value="broadcastDraft.scheduleMode">
            <NRadio
              v-for="opt in SCHEDULE_MODE_OPTIONS"
              :key="opt.value"
              :value="opt.value"
              class="radio-item"
            >
              {{ opt.label }}
            </NRadio>
          </NRadioGroup>
          <div v-if="broadcastDraft.scheduleMode === 'interval'" class="row">
            <span class="label">每隔（分钟）</span>
            <NInputNumber
              v-model:value="broadcastDraft.intervalMinutes"
              :min="1"
              style="width: 140px"
            />
          </div>
          <div v-else class="row">
            <span class="label">cron 表达式（6 段：秒 分 时 日 月 周）</span>
            <NInput
              v-model:value="broadcastDraft.cronExpr"
              placeholder="0 */10 * * * *"
              style="width: 220px"
            />
          </div>

          <h4>轮播内容</h4>
          <div class="row">
            <NRadioGroup v-model:value="broadcastDraft.pickMode">
              <NRadio
                v-for="opt in PICK_MODE_OPTIONS"
                :key="opt.value"
                :value="opt.value"
                class="radio-item"
              >
                {{ opt.label }}
              </NRadio>
            </NRadioGroup>
          </div>
          <p class="hint">
            默认「随机抽取」：每次触发从多条模板里随机挑一条播出。选「轮询」则按顺序循环，
            播完最后一条回到第一条。
          </p>
          <TemplateList v-model="broadcastDraft.templates" placeholder="轮播消息模板" />

          <NCollapse class="preview-collapse">
            <NCollapseItem title="预览将要生成的规则 JSON（本地草稿，尚未保存）" name="preview">
              <pre class="json-preview">{{ JSON.stringify(builtBroadcastRule, null, 2) }}</pre>
            </NCollapseItem>
          </NCollapse>
        </NCard>

        <!-- ==================== 其他答谢：关注 / 分享 / 上舰 ==================== -->
        <NCard title="其他答谢" class="section-card">
          <div class="row">
            <h4 class="inline-title">关注答谢</h4>
            <NSwitch v-model:value="followDraft.enabled" />
          </div>
          <TemplateList v-model="followDraft.templates" placeholder="关注答谢模板" />

          <div class="row">
            <h4 class="inline-title">分享答谢</h4>
            <NSwitch v-model:value="shareDraft.enabled" />
          </div>
          <TemplateList v-model="shareDraft.templates" placeholder="分享答谢模板" />

          <div class="row">
            <h4 class="inline-title">上舰答谢</h4>
            <NSwitch v-model:value="guardDraft.enabled" />
          </div>
          <p class="hint">
            可用变量：<code>{{ GUARD_TEMPLATE_VAR_HINT.username }}</code
            >、<code>{{ GUARD_TEMPLATE_VAR_HINT.guardName }}</code> （舰长/提督/总督）、<code>{{
              GUARD_TEMPLATE_VAR_HINT.guardCount
            }}</code>
            （购买月数）。新购与续费的区分直接写进模板：
            <code>{{ GUARD_TEMPLATE_VAR_HINT.renewCondition }}</code>
            （text/template 自带的条件语法，不需要拆成两条规则）。
          </p>
          <TemplateList v-model="guardDraft.templates" placeholder="上舰答谢模板" />

          <NCollapse class="preview-collapse">
            <NCollapseItem title="预览将要生成的三条规则 JSON（本地草稿，尚未保存）" name="preview">
              <pre class="json-preview">{{
                JSON.stringify([builtFollowRule, builtShareRule, builtGuardRule], null, 2)
              }}</pre>
            </NCollapseItem>
          </NCollapse>
        </NCard>
      </NSpin>
    </template>
  </div>
</template>

<style scoped>
.page-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-bottom: 16px;
}
.partial-failure-alert {
  margin-bottom: 16px;
}
.section-card {
  margin-bottom: 16px;
}
.row {
  display: flex;
  align-items: center;
  gap: 8px;
  margin-bottom: 8px;
  flex-wrap: wrap;
}
.label {
  font-size: 13px;
  opacity: 0.8;
}
.inline-title {
  margin: 0;
}
.radio-item {
  margin-right: 16px;
}
.template-block {
  margin-bottom: 12px;
}
.template-block .label {
  display: block;
  margin-bottom: 4px;
}
.hint {
  font-size: 12px;
  opacity: 0.7;
  margin: 8px 0;
}
.hint code {
  background: rgba(128, 128, 128, 0.15);
  padding: 0 4px;
  border-radius: 3px;
}
.preview-collapse {
  margin-top: 8px;
}
.json-preview {
  font-size: 12px;
  white-space: pre-wrap;
  word-break: break-all;
}
</style>
