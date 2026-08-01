<script lang="ts">
/**
 * PkPanel 是「PK 播报」功能块（弹幕姬页下半部分之一，设计文档 §7.2）。
 *
 * ## 为什么整块都要标「待后端支持」
 *
 * 后端 `event.Battle`（`server/internal/event/payload.go` 第 119-121 行）
 * 目前只有一个字段：
 *
 * ```go
 * type Battle struct {
 *     SubCommand string // 原始 CMD 名，如 "PK_BATTLE_START_NEW"
 * }
 * ```
 *
 * 用户要的「PK 接通那一瞬间截取对面数据——主播昵称、直播间人数、大航海
 * 总数、大航海在线数」一个都没解析；`rules.VarsFromEvent`
 * （`server/internal/rules/vars.go` 第 87-88 行）对应给模板的变量也只有
 * `battle.subCommand` 一项。PK 串门欢迎（对面观众过来时用单独欢迎语）
 * 还要知道「这个进场事件的来源是不是对面直播间」，但 `event.UserEnter`
 * 完全没有来源房间标识，甚至连挂在哪个事件类型上都不确定。
 *
 * 这与「随机 vs 轮询」那种「能跑，只是退化」不同：这里是「界面能画，
 * 后端完全没有数据」，所以本文件里的字段名（`battle.opponentName` 等）
 * 都是**占位示例**，用来说明"打算怎么做"，不是已经确定的真实报文形状。
 * 这一点必须由用户在真实 PK 场景抓包后才能定案（设计文档 §13.5），
 * 控制器凭现有信息编不出来。
 *
 * ## 「PK 匹配信息」与「PK 串门欢迎」在数据模型上不对等
 *
 * `buildPkRule` 只组装「PK 匹配信息」那部分（`on: ['battle']` +
 * 播报模板）——这部分好歹对应着一个已存在的事件类型。「PK 串门欢迎」
 * 连触发事件类型都定不下来（现有的 `user_enter` 不带来源信息，不能
 * 直接拿它当"对面观众进场"用），所以 `visitGreetingEnabled`/
 * `visitTemplates` 两个字段**不写进任何规则**，只是纯粹的草稿 UI 状态，
 * 供评审看这块设想的形状。
 */
import type { Rule } from '@/api/rule-types'

/** PK 播报规则的固定名字，与「进房欢迎」「礼物答谢」用同一套按 name 认领的机制。 */
export const PK_RULE_NAME = '内置/PK播报'

const PK_ON = 'battle'

export interface PkDraft {
  /** 整块的开关。默认关——这块目前保存了也不会生效，不该假装它已经能用。 */
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
      '对面主播是 {{.battle.opponentName}}，直播间 {{.battle.opponentRoomCount}} 人在线，' +
        '大航海 {{.battle.opponentGuardTotal}} 位，一起加油！',
    ],
    visitGreetingEnabled: false,
    visitTemplates: ['欢迎对面直播间的朋友 {{.user.username}} 来串门认识一下~'],
  }
}

/**
 * buildPkRule 组装出的规则**只覆盖「PK 匹配信息」**，且字段形状是占位的：
 * `battle.opponentName` 等在 `rules.VarsFromEvent` 里并不存在，真渲染时
 * 会被 `tmplGet` 兜底成空字符串（`server/internal/rules/template.go` 的
 * `rewriteFieldChains`/`tmplGet`），不会报错，但也不会显示真实数据。
 * 之所以仍然拼出来，是为了让评审者在预览 JSON 里看清楚"打算怎么做"。
 *
 * `announceXxx` 四个勾选目前**不参与**模板组装——后端还没有任何字段
 * 能承载"只播报其中几项"这种选择，四个勾选目前纯粹是给评审看控件形状，
 * 具体怎么落地（四个独立模板变量，还是别的）要等真实报文定下来再设计。
 */
export function buildPkRule(draft: PkDraft): Rule {
  return {
    name: PK_RULE_NAME,
    enabled: draft.enabled,
    on: [PK_ON],
    do: [{ type: 'danmaku', template: draft.matchTemplates.filter((t) => t.trim() !== '') }],
  }
}

/** parsePkDraft 是 buildPkRule 的逆过程，供「认领」到已保存规则时用。 */
export function parsePkDraft(rule: Rule | null): PkDraft {
  const draft = defaultPkDraft()
  if (!rule) return draft

  draft.enabled = rule.enabled ?? true
  const action = rule.do?.find((a) => a.type === 'danmaku')
  if (action?.template && action.template.length > 0) {
    draft.matchTemplates = action.template
  }
  // announceXxx 四项与 visitGreetingEnabled/visitTemplates 在后端没有
  // 对应字段，加载已保存规则时无从恢复，维持默认值。
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

const builtRule = computed(() => buildPkRule(props.modelValue))
</script>

<template>
  <NCard title="PK 播报" class="section-card">
    <template #header-extra>
      <NSwitch :value="modelValue.enabled" @update:value="(v: boolean) => patch({ enabled: v })" />
    </template>

    <NAlert type="warning" :bordered="false" class="pk-alert">
      核心数据后端完全没解析：<code>event.Battle</code> 只有一个 <code>SubCommand</code> 字段， PK
      接通瞬间的对面数据（主播昵称、人数、大航海）协议层拿不到，串门欢迎也没有
      "来源房间"这个字段。<strong>但这不等于整块都不生效</strong>：右上角的总开关与下面的
      播报模板会被正常保存，直播间真的发生 PK 时这条规则确实会触发并发出弹幕， 只是弹幕里
      <code>battle.opponentXxx</code> 这类变量会渲染成空字符串（占位示例， 真实字段名待真实 PK
      场景抓包确认后可能完全不同，设计文档 §13.5）。「PK 匹配信息」 四个勾选与「PK
      串门欢迎」整块则是另一种更彻底的悬空——不会被保存，刷新页面或
      切换直播间都会复位，具体见下面各自的提示。
    </NAlert>

    <h4>
      PK 匹配信息
      <NTooltip>
        <template #trigger>
          <NTag type="warning" size="small">待后端支持</NTag>
        </template>
        需要真实样本：是。控制器无法凭现有信息确定 PK 报文里这些字段叫什么、在第几层，
        必须先在真实直播间触发一次 PK 并抓包。上面四个勾选不会被保存——点了保存也不会写进
        后端，刷新页面或切换直播间后会复位成默认勾选状态。<strong>这四个勾选还有第二层 悬空</strong
        >：即便以后补齐了对面数据字段，"选择性播报其中几项"这个机制本身也还 没设计（后端目前
        没有任何字段能承载"只播报被勾中的那几项"这种选择），抓包定下字段形状之后还要再设计
        一版才能接上这四个勾选。下面的播报模板不同：模板文本本身会被保存（认领进
        「内置/PK播报」这条规则，随整页保存写进数据库），但模板里
        <code>battle.opponentXxx</code>
        这类变量在后端不存在，真实触发时会渲染成空字符串——不是不发，是照常发一条内容 残缺的弹幕。
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
          <NTag type="warning" size="small">待后端支持</NTag>
        </template>
        需要真实样本：是。且比上面那块缺得更彻底——连"这个进场事件来自对面直播间"
        这件事本身该怎么判断都还没确认，可能需要新的事件类型，不只是补字段。这个开关与
        模板同样完全不会被保存：点了保存也不会写进后端，刷新页面或切换直播间后开关会
        复位成关闭、模板会复位成默认文案。
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
      <NCollapseItem
        title="预览将要生成的规则 JSON（仅覆盖 PK 匹配信息，串门欢迎不产出任何规则字段）"
        name="preview"
      >
        <pre class="json-preview">{{ JSON.stringify(builtRule, null, 2) }}</pre>
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
