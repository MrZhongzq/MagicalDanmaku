<script lang="ts">
/**
 * PkPanel 是「PK 播报」功能块（弹幕姬页下半部分之一，设计文档 §7.2）。
 *
 * ## P4-4 Task 7：两块都已接上真实数据，不再悬空
 *
 * 曾经整块悬空的前提——`event.Battle` 只有一个 `SubCommand` 字段、串门
 * 判定没有事件类型可挂——已经被 P4-4 前几个任务解决：
 *
 * - `event.PkMember` 现在带 `RoomID/UID/Username/Face/Votes/IsWinner`
 *   与 PK 接通瞬间抓取的 `Online/GuardTotal/GuardOnline` 一次性快照
 *   （`server/internal/event/payload.go`），`rules.VarsFromEvent` 把它们
 *   展开成 `pk.pkId`/`pk.opponents`/`pk.opponent.*`
 *   （`server/internal/rules/vars.go`）。
 * - `bilibili.PKPipeline`（Task 7 新增）把 `StartPK`/
 *   `FetchOpponentSnapshots` 接进了实时事件流：PK 接通时异步建立对面
 *   连接、抓一次快照，就绪后合成一条 `battle.subCommand ==
 *   "PK_OPPONENT_SNAPSHOT"` 的事件——`buildPkRule` 的 `when` 条件就锁
 *   定这一刻，不在 PK_BATTLE_* 系列几十次状态流转上都播一遍。
 * - `event.TypeVisitFromOpponent`/`TypeVisitToOpponent` 是两个独立事件
 *   类型（欢迎 vs 警示，语义相反，见各自注释），已登记进规则层的三处
 *   （`spec/convert.go`/`vars.go`/`meta_handler.go`）。串门欢迎只播「对面
 *   来访」这一个方向（`pk_visit_from_opponent`）——警示方向没有对应的
 *   播报 UI，用户从未要求过要把警示语也自动发出去，保留手动/自定义规则
 *   的空间。
 *
 * `matchTemplates`/`visitTemplates` 都会被保存进各自的内置规则
 * （`PK_RULE_NAME`/`PK_VISIT_RULE_NAME`），刷新页面或切换直播间不再丢失。
 *
 * ## P5-5 Task 7：真机反馈整改（PK 播报变量说明 + 串门筛选 + 串门动态化）
 *
 * **7a**：PK 匹配信息/串门欢迎的模板变量说明，从一整段 `NTooltip` 大白话
 * 改成「变量 / 含义 / 示例输出」的表格——跟 P5-4 5d/6e 重写礼物答谢变量
 * 说明用的是同一种形式（见 `Danmaku.vue` 的 `ENTER_TEMPLATE_VAR_ROWS`/
 * `GIFT_TEMPLATE_VAR_ROWS`），不是第三种写法。
 *
 * **7b**：串门欢迎加三个筛选子项——只欢迎对面主播佩戴粉丝牌的用户 / 只
 * 欢迎对面粉丝牌 X 级以上 / 只欢迎对面大航海——与进房欢迎的筛选同构：
 * 独立勾选、条件叠加（AND），照抄 `Danmaku.vue` 的 `EnterFilter`/
 * `buildEnterCondition` 那一套，见下面 `PkVisitFilter`/
 * `buildPkVisitCondition`。
 *
 * 三个筛选都落在 `visit.matchedBy`/`user.medal.*` 这些已经登记过的
 * 变量上，没有新增变量、事件类型或操作符，所以 `spec/convert.go` 的
 * `knownEventTypes`、`meta_handler.go` 的元数据表都不需要改动——这里
 * 特意点出「查过、不需要改」，不是漏检查。
 *
 * 「只欢迎对面主播佩戴粉丝牌的用户」为什么拼成
 * `{field:'visit.matchedBy', op:'eq', value:'fan_medal'}` 而不是直接判
 * `user.medal.roomId`：多人 PK 下这一刻的「对面」是哪个房间号只有事件
 * 本身知道（`visit.opponentRoomId`），前端在拼规则的这一刻不知道，没法
 * 写死一个字面量去跟 `user.medal.roomId` 比较，而条件引擎（`rules/
 * condition.go`）目前只支持「字段 vs 字面量」，不支持「字段 vs 字段」。
 * `visit.matchedBy == 'fan_medal'` 是等价的现成信号——后端
 * `classifyVisitFromOpponent`（`server/internal/connector/bilibili/
 * visit.go`）判据 1 命中的充要条件就是「这条事件的 user.medal.roomId
 * 是这一轮 PK 某个对手的房间号」，判据 1 命中时 matchedBy 恒为
 * `fan_medal`，判据 2/3（观众集合/高能榜窗口）命中时恒不是——不需要新
 * 操作符就能精确表达同一个判断。「粉丝牌 X 级以上」「大航海」这两条
 * 在此基础上再叠加 `user.medal.level`/`user.medal.guardLevel` 的判据，
 * 同样要求先命中 `fan_medal`（否则拿到的可能是某个跟这场 PK 毫不相干
 * 的第三方主播的勋章等级）。
 *
 * **7c（不在本文件）**：串门判定本身改成动态——对面高能榜过去 10 秒
 * 滚动窗口，纯后端改动（`server/internal/connector/bilibili/
 * opponent_link.go`/`visit.go`），前端这边只是多了一个可选的
 * `matchedBy == 'energy_rank'` 取值，不影响这里的筛选拼装逻辑。PK 匹配
 * 信息保持「只截取接通那一瞬间」不变——用户原话明确区分了这两者，见
 * 上面 `<NAlert>` 与 `buildPkRule` 的注释。
 */
import type { Condition, Rule } from '@/api/rule-types'
import { PICK_MODE_OPTIONS, parsePickMode, type PickMode } from '@/api/rule-types'
import {
  GUARD_TIER_OPTIONS,
  GUARD_TIER_VALUES,
  guardTierFromValues,
  type GuardTier,
} from '@/utils/guardTier'

/** PK 播报（匹配信息）规则的固定名字，与「进房欢迎」「礼物答谢」用同一套按 name 认领的机制。 */
export const PK_RULE_NAME = '内置/PK播报'
/** PK 串门欢迎规则的固定名字。 */
export const PK_VISIT_RULE_NAME = '内置/PK串门欢迎'

const PK_ON = 'battle'
const PK_VISIT_ON = 'pk_visit_from_opponent'

/**
 * PK_SNAPSHOT_SUBCOMMAND **必须**跟后端
 * `bilibili.PKOpponentSnapshotSubCommand`（`server/internal/connector/
 * bilibili/pk_pipeline.go`）的字面量完全一致——这不是协议里真实存在的
 * B 站 CMD 名，是 PKPipeline 自己合成、专门标记「PK 接通那一瞬间的对面
 * 快照已经就绪」的一个标记值。两边任何一边改了字面量而另一边没跟上，
 * 表现是 PK 播报规则从此再也不会触发，且不报任何错。
 */
const PK_SNAPSHOT_SUBCOMMAND = 'PK_OPPONENT_SNAPSHOT'

/**
 * PkVisitFilter 是 P5-5 7b 新增的三个筛选子项，与 `Danmaku.vue` 的
 * `EnterFilter` 同构——独立勾选、条件叠加（AND）。三者都要求先命中
 * `visit.matchedBy == 'fan_medal'`（见文件头注释「为什么拼成
 * matchedBy」一节），下面 `buildPkVisitCondition` 只在需要时补一次这条
 * 前提，不会因为三个开关都勾了就重复拼三遍。
 */
export interface PkVisitFilter {
  /** 只欢迎佩戴对面主播粉丝牌的用户。 */
  opponentMedalOnly: boolean
  /**
   * 是否启用「对面粉丝牌等级下限」筛选——独立勾选，不勾即不启用，
   * 与 `EnterFilter.minMedalLevelEnabled` 同一个设计。
   */
  minOpponentMedalLevelEnabled: boolean
  /** 对面粉丝牌等级下限，仅 minOpponentMedalLevelEnabled 为真时生效。 */
  minOpponentMedalLevel: number
  /** 只欢迎对面大航海（舰长/提督/总督）用户。 */
  opponentGuardOnly: boolean
  /** 大航海档位下限，仅 opponentGuardOnly 为真时生效。 */
  opponentGuardTier: GuardTier
}

export function defaultPkVisitFilter(): PkVisitFilter {
  return {
    opponentMedalOnly: false,
    minOpponentMedalLevelEnabled: false,
    minOpponentMedalLevel: 1,
    opponentGuardOnly: false,
    opponentGuardTier: 'captain',
  }
}

/**
 * MATCHED_BY_FAN_MEDAL 是后端 `event.VisitMatchedByFanMedal` 的字面量
 * 常量，两边必须完全一致——不一致的表现是这三个筛选悄悄失效（条件
 * 恒不匹配），不报任何错，跟 `PK_SNAPSHOT_SUBCOMMAND` 是同一类风险。
 */
const MATCHED_BY_FAN_MEDAL = 'fan_medal'

/**
 * buildPkVisitCondition 把 7b 的三个筛选拼成一棵 `when` 条件树，直接
 * 照抄 `Danmaku.vue` 的 `buildEnterCondition`：AND 叠加，`leaves` 超过
 * 一条才拼 `all` 树。
 */
export function buildPkVisitCondition(f: PkVisitFilter): Condition | undefined {
  const needsFanMedalGate =
    f.opponentMedalOnly || f.minOpponentMedalLevelEnabled || f.opponentGuardOnly
  const leaves: Condition[] = []

  if (needsFanMedalGate) {
    leaves.push({ field: 'visit.matchedBy', op: 'eq', value: MATCHED_BY_FAN_MEDAL })
  }
  if (f.minOpponentMedalLevelEnabled && f.minOpponentMedalLevel > 0) {
    leaves.push({ field: 'user.medal.level', op: 'gte', value: f.minOpponentMedalLevel })
  }
  if (f.opponentGuardOnly) {
    leaves.push({
      field: 'user.medal.guardLevel',
      op: 'in',
      value: GUARD_TIER_VALUES[f.opponentGuardTier],
    })
  }

  if (leaves.length === 0) return undefined
  if (leaves.length === 1) return leaves[0]
  return { all: leaves }
}

/**
 * parsePkVisitFilter 是 buildPkVisitCondition 的逆过程。
 *
 * 「只欢迎对面主播佩戴粉丝牌的用户」这一项的还原有意保守：只有当
 * `visit.matchedBy` 这条叶子是**唯一**的叶子时才认定 opponentMedalOnly
 * 为真——它同时也是等级/大航海两项筛选隐含要求的前提（`needsFanMedalGate`
 * 恒会补上这条叶子），如果不加"唯一"这个限制，勾了"粉丝牌 X 级以上"
 * 时也会被误还原成同时勾了"只欢迎戴牌用户"，虽然两者语义上确实同时
 * 成立（等级筛选本来就蕴含戴牌），但会让还原结果多出一个用户没有主动
 * 勾选过的开关，不是"往返一致"的正确定义。这跟 EnterFilter 的三个筛选
 * 互相独立、可以精确往返不同——这里损失的只是"是否额外勾了这个冗余
 * 开关"这一个比特位的还原精度，不影响生成的规则效果（无论
 * opponentMedalOnly 是否为真，只要等级筛选打开，条件树完全相同）。
 */
export function parsePkVisitFilter(condition: Condition | undefined): PkVisitFilter {
  const filter = defaultPkVisitFilter()
  if (!condition) return filter

  const leaves = condition.all ?? (condition.field ? [condition] : [])
  let hasFanMedalGate = false
  for (const leaf of leaves) {
    if (
      leaf.field === 'visit.matchedBy' &&
      leaf.op === 'eq' &&
      leaf.value === MATCHED_BY_FAN_MEDAL
    ) {
      hasFanMedalGate = true
    }
    if (leaf.field === 'user.medal.level' && leaf.op === 'gte' && typeof leaf.value === 'number') {
      filter.minOpponentMedalLevelEnabled = true
      filter.minOpponentMedalLevel = leaf.value
    }
    if (leaf.field === 'user.medal.guardLevel' && leaf.op === 'in') {
      const tier = guardTierFromValues(leaf.value)
      if (tier) {
        filter.opponentGuardOnly = true
        filter.opponentGuardTier = tier
      }
    }
  }
  if (hasFanMedalGate && leaves.length === 1) {
    filter.opponentMedalOnly = true
  }
  return filter
}

export interface PkDraft {
  /** 「PK 匹配信息」整块的开关。 */
  enabled: boolean

  /** 播报对面主播昵称。 */
  announceOpponentName: boolean
  /** 播报对面直播间人数。 */
  announceRoomCount: boolean
  /** 播报对面大航海总数。 */
  announceGuardTotal: boolean
  /** 播报对面大航海在线数。 */
  announceGuardOnline: boolean
  /** PK 匹配播报模板有多条时怎么挑（P5-4 8：每一条答谢/播报都要有这个开关）。 */
  matchPickMode: PickMode
  /** PK 匹配播报模板。 */
  matchTemplates: string[]

  /** PK 串门欢迎开关。 */
  visitGreetingEnabled: boolean
  /** 串门欢迎的三个筛选子项（P5-5 7b）。 */
  visitFilter: PkVisitFilter
  /** PK 串门欢迎模板有多条时怎么挑。 */
  visitPickMode: PickMode
  /** PK 串门欢迎模板。 */
  visitTemplates: string[]
}

export function defaultPkDraft(): PkDraft {
  return {
    enabled: false,
    announceOpponentName: true,
    announceRoomCount: true,
    announceGuardTotal: true,
    announceGuardOnline: false,
    matchPickMode: 'random',
    matchTemplates: [
      '对面主播是{{.pk.opponent.uname}}，直播间{{.pk.opponent.online}}人在线，' +
        '大航海{{.pk.opponent.guardTotal}}位，一起加油！',
    ],
    visitGreetingEnabled: false,
    visitFilter: defaultPkVisitFilter(),
    visitPickMode: 'random',
    visitTemplates: ['欢迎对面直播间的朋友{{.user.username}}来串门认识一下~'],
  }
}

/**
 * buildPkRule 组装「PK 匹配信息」规则。
 *
 * `when` 锁定 PKPipeline 合成的快照就绪事件——用户原话「只截取 PK 接通
 * 的那一瞬间的数据」，不该在一场 PK 的几十次状态流转（PK_INFO/
 * PK_BATTLE_PRE/PK_BATTLE_PROCESS/...）上都播一遍。
 *
 * `announceXxx` 四个勾选**不参与组装**——它们只决定 `defaultPkDraft()`
 * 默认模板里出现哪些字段（默认模板包含前三项、不含
 * `announceGuardOnline` 默认关闭对应的那个字段），模板本身是自由文本
 * （`TemplateList`），保存的是用户最终编辑的结果，不会被这四个勾选
 * 事后重新拼接覆盖——这与进房欢迎「佩戴粉丝牌」那种直接拼 `when` 条件
 * 的自动化程度不同，是因为后端从未有过「选择性隐藏某几个字段」这种
 * 机制，模板引擎只按用户写的文本渲染。
 */
export function buildPkRule(draft: PkDraft): Rule {
  return {
    name: PK_RULE_NAME,
    enabled: draft.enabled,
    on: [PK_ON],
    when: { field: 'battle.subCommand', op: 'eq', value: PK_SNAPSHOT_SUBCOMMAND },
    do: [
      {
        type: 'danmaku',
        template: draft.matchTemplates.filter((t) => t.trim() !== ''),
        pick: draft.matchPickMode,
      },
    ],
  }
}

/**
 * PK_VISIT_AGGREGATE_WINDOW_SECONDS 是串门欢迎的合并窗口——终审复审
 * 指出的 C-1 残留一半：`welcomedFromOpponent`（后端）只堵死了「同一个人
 * 刷屏」，堵不住「PK 接通瞬间对面 N 个不同的人一起涌入」，这条规则此前
 * 完全没有 `aggregate`，N 个人 = N 条弹幕，被账号级限流拉成一串串行
 * 发送、把礼物答谢等其它规则挤到后面。跟 `buildEnterRule`（`Danmaku.vue`）
 * 同款做法：加一个「窗口内全部合并」的 `aggregate`（`by: 'type'`，不设
 * `minCount`——0/1 都表示「总是合并」，见 `rules.AggregateSpec.MinCount`
 * 的注释），把这个窗口内的全部串门事件收成一条 Trigger，只触发一次
 * danmaku 动作，而不是每个人各触发一次。
 *
 * 窗口内只有一个人时行为不变（模板仍然读到这唯一一个人的
 * `user.username`，见 `rules/aggregate.go` `mergeBuckets`：单元素分组的
 * `vars` 就是那一个事件自己的 vars，`MergeVars` 不会覆盖非空字段）；
 * 窗口内多人涌入时只播出分组里第一个人的名字，不是完整地一一欢迎——
 * 但这仍然是从「N 条弹幕刷屏」到「1 条弹幕」的严格改善，不是回归。真要
 * 做「欢迎 A、B、C 等 N 位朋友」这种多人文案，需要仿照
 * `buildEnterRule`/`EnterDraft` 那套 `templateMulti` + `groupMode` UI，
 * 范围明显更大，这次不做。
 */
const PK_VISIT_AGGREGATE_WINDOW_SECONDS = 5

/**
 * buildPkVisitRule 组装「PK 串门欢迎」规则——只覆盖欢迎方向
 * （`pk_visit_from_opponent`），警示方向（`pk_visit_to_opponent`，我方
 * 观众跑去对面）没有对应的自动播报 UI，避免把两个语气相反的方向在
 * 界面上混为一谈；需要监控警示方向的用户可以在「自定义弹幕姬」页自己
 * 配一条 `on: pk_visit_to_opponent` 的规则。
 */
export function buildPkVisitRule(draft: PkDraft): Rule {
  const rule: Rule = {
    name: PK_VISIT_RULE_NAME,
    enabled: draft.visitGreetingEnabled,
    on: [PK_VISIT_ON],
    aggregate: { window: `${PK_VISIT_AGGREGATE_WINDOW_SECONDS}s`, by: 'type' },
    do: [
      {
        type: 'danmaku',
        template: draft.visitTemplates.filter((t) => t.trim() !== ''),
        pick: draft.visitPickMode,
      },
    ],
  }
  const when = buildPkVisitCondition(draft.visitFilter)
  if (when) rule.when = when
  return rule
}

/**
 * parsePkDraft 从两条已保存的规则里还原一份 PkDraft，供「认领」到已保存
 * 配置时用。matchRule 对应 `PK_RULE_NAME`，visitRule 对应
 * `PK_VISIT_RULE_NAME`——两条规则触发的事件类型不同（`battle` vs
 * `pk_visit_from_opponent`），无法合并成一条规则各自配一半模板。
 */
export function parsePkDraft(matchRule: Rule | null, visitRule: Rule | null): PkDraft {
  const draft = defaultPkDraft()

  if (matchRule) {
    draft.enabled = matchRule.enabled ?? true
    const action = matchRule.do?.find((a) => a.type === 'danmaku')
    if (action?.template && action.template.length > 0) {
      draft.matchTemplates = action.template
    }
    draft.matchPickMode = parsePickMode(action?.pick)
  }

  if (visitRule) {
    draft.visitGreetingEnabled = visitRule.enabled ?? false
    draft.visitFilter = parsePkVisitFilter(visitRule.when)
    const action = visitRule.do?.find((a) => a.type === 'danmaku')
    if (action?.template && action.template.length > 0) {
      draft.visitTemplates = action.template
    }
    draft.visitPickMode = parsePickMode(action?.pick)
  }

  return draft
}
</script>

<script setup lang="ts">
import { computed } from 'vue'
import {
  NAlert,
  NCard,
  NCheckbox,
  NCollapse,
  NCollapseItem,
  NInputNumber,
  NRadio,
  NRadioGroup,
  NSwitch,
  NTag,
  NTooltip,
} from 'naive-ui'
import TemplateList from './TemplateList.vue'

const props = defineProps<{ modelValue: PkDraft }>()
const emit = defineEmits<{ 'update:modelValue': [PkDraft] }>()

/**
 * patch 是这个组件更新草稿的唯一出口：不直接改 props（`vue/no-mutating-props`
 * 也不允许），而是拼出一份新对象整体 emit 出去，由父组件（Danmaku.vue）
 * 用 `Object.assign` 写回它自己持有的 reactive 草稿。
 */
function patch(partial: Partial<PkDraft>) {
  emit('update:modelValue', { ...props.modelValue, ...partial })
}

/** patchVisitFilter 是 patch 的一个便利封装，只更新 visitFilter 里的某几项。 */
function patchVisitFilter(partial: Partial<PkVisitFilter>) {
  patch({ visitFilter: { ...props.modelValue.visitFilter, ...partial } })
}

const builtMatchRule = computed(() => buildPkRule(props.modelValue))
const builtVisitRule = computed(() => buildPkVisitRule(props.modelValue))

/**
 * P5-5 7a：变量说明表格，形式照抄 `Danmaku.vue` 的
 * `ENTER_TEMPLATE_VAR_ROWS`/`GIFT_TEMPLATE_VAR_ROWS`——「变量」列放能
 * 直接抄进模板的字面量，「含义」列才是中文说明，不再把两者揉进一整段
 * 夹杂顿号的 `NTooltip` 大白话里。
 */
interface VarHintRow {
  variable: string
  meaning: string
  example: string
}

const PK_MATCH_TEMPLATE_VAR_ROWS: VarHintRow[] = [
  { variable: '{{.pk.opponent.uname}}', meaning: '对面主播昵称', example: '花花' },
  {
    variable: '{{.pk.opponent.online}}',
    meaning: '对面直播间人数（PK 接通瞬间的一次性快照，接口失败或还没就绪时不存在，不等于 0）',
    example: '1234',
  },
  {
    variable: '{{.pk.opponent.guardTotal}}',
    meaning: '对面大航海总数（同上，一次性快照）',
    example: '56',
  },
  {
    variable: '{{.pk.opponent.guardOnline}}',
    meaning: '对面大航海在线数（同上，一次性快照）',
    example: '12',
  },
  { variable: '{{.pk.pkId}}', meaning: '这场 PK 的 ID', example: 'pk_20260804_001' },
  {
    variable: '{{.pk.opponents}}',
    meaning: '多人 PK 下全部对手的列表；pk.opponent 只是取第一个的便利写法',
    example: '（列表，需配合 range 使用）',
  },
]

const PK_VISIT_TEMPLATE_VAR_ROWS: VarHintRow[] = [
  { variable: '{{.user.username}}', meaning: '来访者（对面观众/主播本人）的昵称', example: '张三' },
  {
    variable: '{{.visit.opponentRoomId}}',
    meaning: '来访者所属的对手直播间号（多人 PK 下用来分清是哪一个对手）',
    example: '88888',
  },
  {
    variable: '{{.visit.matchedBy}}',
    meaning:
      '判定依据：fan_medal 佩戴对面粉丝牌 / audience PK 期间累计的观众集合 / ' +
      'energy_rank 对面高能榜过去 10 秒滚动窗口',
    example: 'fan_medal',
  },
]
</script>

<template>
  <NCard title="PK 播报" class="section-card">
    <template #header-extra>
      <NSwitch :value="modelValue.enabled" @update:value="(v: boolean) => patch({ enabled: v })" />
    </template>

    <NAlert type="info" :bordered="false" class="pk-alert">
      PK 接通瞬间会一次性截取对面数据（主播昵称、直播间人数、大航海总数/在线数），之后对面数据的
      变化不再播报——「只截取 PK 接通的那一瞬间」。外部接口一时拿不到某个数字时该字段会渲染成空，
      不会显示成「0 人在线」这种看起来正常实则错误的数字；PK 播报本身照常发出，不会因为一个字段
      拿不到就整条不播。
    </NAlert>

    <h4>
      PK 匹配信息
      <NTooltip>
        <template #trigger>
          <NTag type="success" size="small">已接入真实数据</NTag>
        </template>
        右上角总开关与下面的播报模板都会被保存。下面四个勾选只决定「恢复默认值」时模板长什么样，
        不会在你编辑过模板之后反过来改写它——模板是自由文本，想加减字段直接改文本即可，可用变量见
        下方表格。
      </NTooltip>
    </h4>
    <p class="hint">只截取 PK 接通的那一瞬间，之后对面数据的变化不再播报。</p>
    <div class="row">
      <NCheckbox
        :checked="modelValue.announceOpponentName"
        @update:checked="(v: boolean) => patch({ announceOpponentName: v })"
      >
        对面主播昵称
      </NCheckbox>
      <NCheckbox
        :checked="modelValue.announceRoomCount"
        @update:checked="(v: boolean) => patch({ announceRoomCount: v })"
      >
        对面直播间人数
      </NCheckbox>
      <NCheckbox
        :checked="modelValue.announceGuardTotal"
        @update:checked="(v: boolean) => patch({ announceGuardTotal: v })"
      >
        对面大航海总数
      </NCheckbox>
      <NCheckbox
        :checked="modelValue.announceGuardOnline"
        @update:checked="(v: boolean) => patch({ announceGuardOnline: v })"
      >
        对面大航海在线数
      </NCheckbox>
    </div>

    <div class="row">
      <NRadioGroup
        :value="modelValue.matchPickMode"
        @update:value="(v: 'random' | 'sequential') => patch({ matchPickMode: v })"
      >
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
    <TemplateList
      :model-value="modelValue.matchTemplates"
      placeholder="PK播报语模板"
      @update:model-value="(v: string[]) => patch({ matchTemplates: v })"
    />

    <div class="var-hint-block">
      <span class="label">可用变量</span>
      <table class="var-hint-table">
        <thead>
          <tr>
            <th>变量</th>
            <th>含义</th>
            <th>示例输出</th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="row in PK_MATCH_TEMPLATE_VAR_ROWS" :key="row.variable">
            <td>
              <code>{{ row.variable }}</code>
            </td>
            <td>{{ row.meaning }}</td>
            <td>{{ row.example }}</td>
          </tr>
        </tbody>
      </table>
    </div>

    <h4>
      PK 串门欢迎
      <NTooltip>
        <template #trigger>
          <NTag type="success" size="small">已接入真实数据</NTag>
        </template>
        对面直播间的人（观众或主播本人）跑来我方时触发，事件类型
        <code>pk_visit_from_opponent</code>。只覆盖「欢迎」这一个方向——「我方观众跑去对面」是语气
        相反的警示信号（<code>pk_visit_to_opponent</code>），没有对应的自动播报开关，避免两种语气在
        这里被混为一谈；需要监控警示方向可以去「自定义弹幕姬」页单独配一条规则。
      </NTooltip>
    </h4>
    <p class="hint">
      串门判定本身是动态的：除了戴对面粉丝牌，还会参考对面高能榜过去 10
      秒的滚动窗口——刚从对面串完门回来的人也认得出来，不要求跟本房间事件严格同一瞬间。
    </p>
    <div class="row">
      <span class="label">对面观众串门时用单独欢迎语（与常规进房欢迎区分）</span>
      <NSwitch
        :value="modelValue.visitGreetingEnabled"
        @update:value="(v: boolean) => patch({ visitGreetingEnabled: v })"
      />
    </div>

    <h5>筛选条件（P5-5 7b：独立勾选，条件叠加）</h5>
    <div class="row">
      <NCheckbox
        :checked="modelValue.visitFilter.opponentMedalOnly"
        @update:checked="(v: boolean) => patchVisitFilter({ opponentMedalOnly: v })"
      >
        只欢迎佩戴对面主播粉丝牌的用户
      </NCheckbox>
    </div>
    <div class="row">
      <NCheckbox
        :checked="modelValue.visitFilter.minOpponentMedalLevelEnabled"
        @update:checked="(v: boolean) => patchVisitFilter({ minOpponentMedalLevelEnabled: v })"
      >
        只欢迎对面粉丝牌等级达到
      </NCheckbox>
      <NInputNumber
        :value="modelValue.visitFilter.minOpponentMedalLevel"
        :min="1"
        :disabled="!modelValue.visitFilter.minOpponentMedalLevelEnabled"
        style="width: 100px"
        @update:value="(v: number | null) => patchVisitFilter({ minOpponentMedalLevel: v ?? 1 })"
      />
      <span class="label">级以上的用户</span>
    </div>
    <div class="row">
      <NCheckbox
        :checked="modelValue.visitFilter.opponentGuardOnly"
        @update:checked="(v: boolean) => patchVisitFilter({ opponentGuardOnly: v })"
      >
        只欢迎对面大航海（舰长/提督/总督）用户
      </NCheckbox>
      <NRadioGroup
        :value="modelValue.visitFilter.opponentGuardTier"
        :disabled="!modelValue.visitFilter.opponentGuardOnly"
        @update:value="(v: GuardTier) => patchVisitFilter({ opponentGuardTier: v })"
      >
        <NRadio
          v-for="opt in GUARD_TIER_OPTIONS"
          :key="opt.value"
          :value="opt.value"
          class="radio-item"
        >
          {{ opt.label }}
        </NRadio>
      </NRadioGroup>
    </div>

    <div class="row">
      <NRadioGroup
        :value="modelValue.visitPickMode"
        @update:value="(v: 'random' | 'sequential') => patch({ visitPickMode: v })"
      >
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
    <TemplateList
      :model-value="modelValue.visitTemplates"
      placeholder="串门欢迎语模板"
      @update:model-value="(v: string[]) => patch({ visitTemplates: v })"
    />

    <div class="var-hint-block">
      <span class="label">可用变量</span>
      <table class="var-hint-table">
        <thead>
          <tr>
            <th>变量</th>
            <th>含义</th>
            <th>示例输出</th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="row in PK_VISIT_TEMPLATE_VAR_ROWS" :key="row.variable">
            <td>
              <code>{{ row.variable }}</code>
            </td>
            <td>{{ row.meaning }}</td>
            <td>{{ row.example }}</td>
          </tr>
        </tbody>
      </table>
    </div>

    <NCollapse class="preview-collapse">
      <NCollapseItem
        title="预览将要生成的规则 JSON（PK 匹配信息 + PK 串门欢迎，各一条规则）"
        name="preview"
      >
        <pre class="json-preview">{{ JSON.stringify(builtMatchRule, null, 2) }}</pre>
        <pre class="json-preview">{{ JSON.stringify(builtVisitRule, null, 2) }}</pre>
      </NCollapseItem>
    </NCollapse>
  </NCard>
</template>

<style scoped>
.section-card {
  margin-bottom: 16px;
}
.pk-alert {
  margin-bottom: 12px;
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
.radio-item {
  margin-right: 16px;
}
.hint {
  font-size: 12px;
  opacity: 0.7;
  margin: 4px 0 8px;
}
.var-hint-block {
  margin: 8px 0 12px;
}
.var-hint-block .label {
  display: block;
  margin-bottom: 4px;
}
.var-hint-table {
  border-collapse: collapse;
  font-size: 12px;
  width: 100%;
}
.var-hint-table th,
.var-hint-table td {
  border: 1px solid rgba(128, 128, 128, 0.25);
  padding: 4px 8px;
  text-align: left;
}
.var-hint-table code {
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
