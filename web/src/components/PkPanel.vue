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
 */
import type { Rule } from '@/api/rule-types'

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
  /** PK 匹配播报模板。 */
  matchTemplates: string[]

  /** PK 串门欢迎开关。 */
  visitGreetingEnabled: boolean
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
    matchTemplates: [
      '对面主播是 {{.pk.opponent.uname}}，直播间 {{.pk.opponent.online}} 人在线，' +
        '大航海 {{.pk.opponent.guardTotal}} 位，一起加油！',
    ],
    visitGreetingEnabled: false,
    visitTemplates: ['欢迎对面直播间的朋友 {{.user.username}} 来串门认识一下~'],
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
    do: [{ type: 'danmaku', template: draft.matchTemplates.filter((t) => t.trim() !== '') }],
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
  return {
    name: PK_VISIT_RULE_NAME,
    enabled: draft.visitGreetingEnabled,
    on: [PK_VISIT_ON],
    aggregate: { window: `${PK_VISIT_AGGREGATE_WINDOW_SECONDS}s`, by: 'type' },
    do: [{ type: 'danmaku', template: draft.visitTemplates.filter((t) => t.trim() !== '') }],
  }
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
  }

  if (visitRule) {
    draft.visitGreetingEnabled = visitRule.enabled ?? false
    const action = visitRule.do?.find((a) => a.type === 'danmaku')
    if (action?.template && action.template.length > 0) {
      draft.visitTemplates = action.template
    }
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

const builtMatchRule = computed(() => buildPkRule(props.modelValue))
const builtVisitRule = computed(() => buildPkVisitRule(props.modelValue))
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
        右上角总开关与下面的播报模板都会被保存。模板变量：<code>pk.opponent.uname</code>
        （对面主播昵称）、<code>pk.opponent.online</code>（对面直播间人数）、
        <code>pk.opponent.guardTotal</code>（对面大航海总数）、
        <code>pk.opponent.guardOnline</code>（对面大航海在线数）、<code>pk.pkId</code>
        （这场 PK 的 ID）。多人 PK 下 <code>pk.opponents</code> 是全部对手的列表，
        <code>pk.opponent</code> 只是取第一个的便利写法。下面四个勾选只决定「恢复默认值」时
        模板长什么样，不会在你编辑过模板之后反过来改写它——模板是自由文本，想加减字段直接改
        文本即可。
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

    <TemplateList
      :model-value="modelValue.matchTemplates"
      placeholder="PK播报语模板"
      @update:model-value="(v: string[]) => patch({ matchTemplates: v })"
    />

    <h4>
      PK 串门欢迎
      <NTooltip>
        <template #trigger>
          <NTag type="success" size="small">已接入真实数据</NTag>
        </template>
        对面直播间的人（观众或主播本人）跑来我方时触发，事件类型
        <code>pk_visit_from_opponent</code>，模板可用 <code>user.username</code> 等常规用户变量。
        只覆盖「欢迎」这一个方向——「我方观众跑去对面」是语气相反的警示信号
        （<code>pk_visit_to_opponent</code>），没有对应的自动播报开关，避免两种语气在这里被混为
        一谈；需要监控警示方向可以去「自定义弹幕姬」页单独配一条规则。
      </NTooltip>
    </h4>
    <div class="row">
      <span class="label">对面观众串门时用单独欢迎语（与常规进房欢迎区分）</span>
      <NSwitch
        :value="modelValue.visitGreetingEnabled"
        @update:value="(v: boolean) => patch({ visitGreetingEnabled: v })"
      />
    </div>
    <TemplateList
      :model-value="modelValue.visitTemplates"
      placeholder="串门欢迎语模板"
      @update:model-value="(v: string[]) => patch({ visitTemplates: v })"
    />

    <NCollapse class="preview-collapse">
      <NCollapseItem title="预览将要生成的规则 JSON（PK 匹配信息 + PK 串门欢迎，各一条规则）" name="preview">
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
.hint {
  font-size: 12px;
  opacity: 0.7;
  margin: 4px 0 8px;
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
