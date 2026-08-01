<script lang="ts">
/**
 * Custom 是「自定义弹幕姬」页（Task 11，设计文档 §7.2 页面 5）——P4-2 里
 * 最难的一个组件的落地场景：给主播完全的自由度，把**触发器 + 模板**的
 * 组合暴露成可视化编辑器，而不是像 `Danmaku.vue` 那样把几个固定功能块
 * 的开关翻译成规则。
 *
 * ## 与 Danmaku.vue 的关系：谁认领谁
 *
 * `Danmaku.vue`/`PkPanel.vue` 用固定 `name`（`内置/进房欢迎` 等）从
 * `GET /api/bindings/{id}/rules` 里"认领"规则，认领不到的规则对那两个
 * 页面而言不存在。这一页反过来：**排除掉那七个固定名字之后剩下的规则
 * 都是"自定义规则"**，`isCustomRule` 就是这道过滤。用户在这页新建的
 * 规则也只需要避免撞上那七个保留名——具体重名校验留给后端保存时做
 * （用户的原则：后端接口统一最后处理）。
 *
 * ## 条件树：委托给 ConditionTree.vue
 *
 * `spec.Condition` 的递归结构（`all`/`any`/`not`/`script`/叶子）由
 * `ConditionTree.vue` 负责渲染与编辑，本页只管接线：
 * `operators`/`fieldOptions` 两份清单往下传，`v-model="draft.when"`
 * 拿到编辑结果。真正写进规则前要过一遍 `pruneCondition`——道理见
 * `ConditionTree.vue` 文件头注释："删到空"不是"合法但无意义"，是根本
 * 过不了后端 `Condition.Validate()`，必须在这一层收拢。
 *
 * ## 变量清单的坑，同样在这里体现
 *
 * `fieldOptions` 传的是 `ConditionTree.vue` 导出的 `COMMON_FIELD_OPTIONS`
 * ——因为压根没有 `GET /api/meta/variables` 这个接口。已登记进悬空清单，
 * 见该文件顶部注释。
 *
 * ## 「排除通用规则」：界面做出来，标悬空
 *
 * 设计文档 §7.2 明确要求："一条自定义规则命中后，可以声明屏蔽掉哪些
 * 通用功能"，典型场景是"给某位舰长配了专属进房欢迎，就不该再触发通用
 * 进房欢迎"。规则引擎（`server/internal/rules`）目前没有"一条规则命中后
 * 跳过另一条"这种互斥/优先级机制，`spec.Rule` 里也没有承载这份声明的
 * 字段。这里给出一个多选框，列出 Task 9/10 建立的七个内置规则名，
 * 但 `CustomRuleDraft.excludeBuiltinRules` **不参与** `buildCustomRule`
 * 的组装——写了也没有字段能装，装了引擎也不会读。界面上用"待后端支持"
 * 标签说明，已登记进悬空清单。
 *
 * ## 保存不在本任务范围内
 *
 * 与 Task 9/10 同一套约定：改动只进内存草稿，`SaveBar` 的 `save` 事件
 * 先留空，注释「Task 13 接」。
 */
import type { Action, Aggregate, Condition, Rule, RuleView } from '@/api/rule-types'
import { defaultLeafCondition, pruneCondition } from '@/components/ConditionTree.vue'
import {
  ENTER_RULE_NAME,
  GIFT_RULE_NAME,
  BROADCAST_RULE_NAME,
  FOLLOW_RULE_NAME,
  SHARE_RULE_NAME,
  GUARD_RULE_NAME,
} from './Danmaku.vue'
import { PK_RULE_NAME } from '@/components/PkPanel.vue'

/** Task 9/10 建立的七个内置规则名——判定"是不是自定义规则"、"排除通用规则"多选框都靠它。 */
export const BUILTIN_RULE_NAMES: string[] = [
  ENTER_RULE_NAME,
  GIFT_RULE_NAME,
  PK_RULE_NAME,
  BROADCAST_RULE_NAME,
  FOLLOW_RULE_NAME,
  SHARE_RULE_NAME,
  GUARD_RULE_NAME,
]

export const BUILTIN_RULE_OPTIONS: { label: string; value: string }[] = BUILTIN_RULE_NAMES.map(
  (n) => ({ label: n, value: n }),
)

/** isCustomRule 判断一条从后端拉回来的规则是不是"自定义规则"——不在七个内置名单里就是。 */
export function isCustomRule(rule: RuleView): boolean {
  return !BUILTIN_RULE_NAMES.includes(rule.name ?? '')
}

// ---- 时长 <-> 秒数：与 Danmaku.vue 同样的转换规则，各页自成一份，见该文件同名函数的注释 ----

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

function secondsFromDuration(d: string | undefined, fallback: number): number {
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

// ---- 触发方式：事件驱动 / 定时驱动，二选一，与 spec.Rule.Validate() 的互斥约束一致 ----

export type TriggerMode = 'on' | 'schedule'
export type ScheduleMode = 'interval' | 'cron'

export const TRIGGER_MODE_OPTIONS: { label: string; value: TriggerMode }[] = [
  { label: '事件触发', value: 'on' },
  { label: '定时触发', value: 'schedule' },
]

export const SCHEDULE_MODE_OPTIONS: { label: string; value: ScheduleMode }[] = [
  { label: '按固定间隔（分钟）', value: 'interval' },
  { label: '自定义 cron 表达式', value: 'cron' },
]

// 六段 cron："秒 分 时 日 月 周"。写法与 Danmaku.vue 的 intervalMinutesToCron
// 一致（同样刻意不用 /** */ 块注释包住含 "*/N" 的说明，理由见那边）。
function buildCronFromInterval(minutes: number): string {
  const m = Math.max(1, Math.round(minutes))
  return `0 */${m} * * * *`
}

const INTERVAL_CRON_RE = /^0 \*\/(\d+) \* \* \* \*$/

export function buildCustomSchedule(draft: CustomRuleDraft): string {
  return draft.scheduleMode === 'interval'
    ? buildCronFromInterval(draft.intervalMinutes)
    : draft.cronExpr.trim()
}

// ---- 动作：do[] 数组，每项按 type 决定用哪些字段 ----

export interface CustomActionDraft {
  type: string
  /** type === 'danmaku' 时使用。 */
  templates: string[]
  /** type === 'block' 时使用：禁言小时数。 */
  hours: number
  /** type === 'script' 时使用。 */
  script: string
}

export function defaultActionDraft(type = 'danmaku'): CustomActionDraft {
  return {
    type,
    templates: ['{{.user.username}} 触发了这条自定义规则'],
    hours: 1,
    script: '',
  }
}

/** buildCustomAction 按 type 只取相关字段——比如 type=block 时不会把 templates 也塞进去。 */
export function buildCustomAction(a: CustomActionDraft): Action {
  switch (a.type) {
    case 'danmaku':
      return { type: 'danmaku', template: a.templates.filter((t) => t.trim() !== '') }
    case 'block':
      return { type: 'block', hours: a.hours }
    case 'script':
      return { type: 'script', script: a.script }
    default:
      // 'log' 或未来新增的、这里还不认识的动作类型：无额外字段。
      return { type: a.type }
  }
}

function parseCustomAction(a: Action): CustomActionDraft {
  const fallback = defaultActionDraft(a.type)
  return {
    type: a.type,
    templates: a.template && a.template.length > 0 ? a.template : fallback.templates,
    hours: a.hours ?? fallback.hours,
    script: a.script || fallback.script,
  }
}

// ---- 一条自定义规则的完整草稿 ----

let draftIdSeq = 0
/** nextDraftId 只是前端列表 :key 用的本地标识，不进 spec.Rule，不需要跨页面加载稳定。 */
export function nextDraftId(): string {
  draftIdSeq += 1
  return `custom-${draftIdSeq}`
}

export interface CustomRuleDraft {
  id: string
  name: string
  enabled: boolean
  triggerMode: TriggerMode
  on: string[]
  scheduleMode: ScheduleMode
  intervalMinutes: number
  cronExpr: string
  /** when 是可选的——关掉就是"无条件，事件一到就触发"。 */
  whenEnabled: boolean
  when: Condition
  aggregateEnabled: boolean
  aggregateBy: string
  windowSeconds: number
  maxWaitSeconds: number | null
  minCount: number
  cooldownEnabled: boolean
  cooldownSeconds: number
  cooldownGroup: string
  actions: CustomActionDraft[]
  /** 排除通用规则——悬空，见文件头注释，不参与 buildCustomRule 组装。 */
  excludeBuiltinRules: string[]
}

export function defaultCustomRuleDraft(): CustomRuleDraft {
  return {
    id: nextDraftId(),
    name: '',
    enabled: true,
    triggerMode: 'on',
    on: [],
    scheduleMode: 'interval',
    intervalMinutes: 10,
    cronExpr: '0 */10 * * * *',
    whenEnabled: false,
    when: defaultLeafCondition(),
    aggregateEnabled: false,
    aggregateBy: 'type',
    windowSeconds: 60,
    maxWaitSeconds: null,
    minCount: 1,
    cooldownEnabled: false,
    cooldownSeconds: 60,
    cooldownGroup: '',
    actions: [defaultActionDraft()],
    excludeBuiltinRules: [],
  }
}

/**
 * buildCustomRule 把草稿组装成 spec.Rule。
 *
 * `when` 在写入前先过 `pruneCondition`：草稿态允许出现空的 all/any 分支
 * （用户正在编辑），但那种形状送到后端会被 `Condition.Validate()`
 * 直接拒收，必须在这里收拢成"没有这个分支"或者干脆整个 `when` 都不写。
 */
export function buildCustomRule(draft: CustomRuleDraft): Rule {
  const rule: Rule = {
    name: draft.name,
    enabled: draft.enabled,
    do: draft.actions.map(buildCustomAction),
  }

  if (draft.triggerMode === 'on') {
    rule.on = draft.on
  } else {
    rule.schedule = buildCustomSchedule(draft)
  }

  if (draft.whenEnabled) {
    const pruned = pruneCondition(draft.when)
    if (pruned) rule.when = pruned
  }

  if (draft.aggregateEnabled) {
    const agg: Aggregate = { window: secondsToDuration(draft.windowSeconds), by: draft.aggregateBy }
    if (draft.maxWaitSeconds !== null && draft.maxWaitSeconds > 0) {
      agg.maxWait = secondsToDuration(draft.maxWaitSeconds)
    }
    if (draft.minCount > 1) agg.minCount = draft.minCount
    rule.aggregate = agg
  }

  if (draft.cooldownEnabled) {
    rule.cooldown = secondsToDuration(draft.cooldownSeconds)
  }
  if (draft.cooldownGroup.trim() !== '') {
    rule.cooldownGroup = draft.cooldownGroup.trim()
  }

  return rule
}

/** parseCustomRuleDraft 是 buildCustomRule 的逆过程，供加载已有自定义规则用。 */
export function parseCustomRuleDraft(rule: RuleView): CustomRuleDraft {
  const draft = defaultCustomRuleDraft()
  draft.name = rule.name ?? ''
  draft.enabled = rule.enabled ?? true

  if (rule.schedule) {
    draft.triggerMode = 'schedule'
    const m = INTERVAL_CRON_RE.exec(rule.schedule)
    if (m) {
      draft.scheduleMode = 'interval'
      draft.intervalMinutes = Number(m[1])
    } else {
      draft.scheduleMode = 'cron'
      draft.cronExpr = rule.schedule
    }
  } else {
    draft.triggerMode = 'on'
    draft.on = rule.on ?? []
  }

  if (rule.when) {
    draft.whenEnabled = true
    draft.when = rule.when
  }

  if (rule.aggregate) {
    draft.aggregateEnabled = true
    draft.aggregateBy = rule.aggregate.by
    draft.windowSeconds = secondsFromDuration(rule.aggregate.window, draft.windowSeconds)
    draft.maxWaitSeconds = rule.aggregate.maxWait
      ? secondsFromDuration(rule.aggregate.maxWait, 0)
      : null
    if (rule.aggregate.minCount !== undefined) draft.minCount = rule.aggregate.minCount
  }

  if (rule.cooldown) {
    draft.cooldownEnabled = true
    draft.cooldownSeconds = secondsFromDuration(rule.cooldown, draft.cooldownSeconds)
  }
  draft.cooldownGroup = rule.cooldownGroup ?? ''

  if (rule.do && rule.do.length > 0) {
    draft.actions = rule.do.map(parseCustomAction)
  }

  return draft
}

export type { RuleView }
</script>

<script setup lang="ts">
import { onMounted, reactive, ref, watch } from 'vue'
import {
  NAlert,
  NButton,
  NCard,
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
  useDialog,
  useMessage,
} from 'naive-ui'
import { ApiError, request } from '@/api'
import { useBindingsStore } from '@/stores/bindings'
import { useDraft } from '@/composables/useDraft'
import SaveBar from '@/components/SaveBar.vue'
import TemplateList from '@/components/TemplateList.vue'
import ConditionTree, { COMMON_FIELD_OPTIONS } from '@/components/ConditionTree.vue'
import type { MetaItem } from '@/components/ConditionTree.vue'

const bindings = useBindingsStore()
const message = useMessage()
const dialog = useDialog()

const loading = ref(false)

// ---- 元数据：全部从 GET /api/meta/* 拉，不硬编码 ----
//
// 与 MemberEditor.vue 拉权限点清单同一套模式：这里唯一的真相来源是这
// 四个 ref，界面渲染多少项完全取决于后端这次回了多少项。

const eventTypeOptions = ref<MetaItem[]>([])
const actionTypeOptions = ref<MetaItem[]>([])
const operatorOptions = ref<MetaItem[]>([])
const aggregateByOptions = ref<MetaItem[]>([])

async function loadMeta() {
  try {
    const [events, actions, operators, aggregateBy] = await Promise.all([
      request<MetaItem[]>('GET', '/api/meta/event-types'),
      request<MetaItem[]>('GET', '/api/meta/action-types'),
      request<MetaItem[]>('GET', '/api/meta/operators'),
      request<MetaItem[]>('GET', '/api/meta/aggregate-by'),
    ])
    eventTypeOptions.value = events
    actionTypeOptions.value = actions
    operatorOptions.value = operators
    aggregateByOptions.value = aggregateBy
  } catch (e) {
    message.error(e instanceof ApiError ? e.message : '加载元数据失败')
  }
}

// ---- 自定义规则列表：草稿态，加载时从"排除内置七条之后剩下的规则"里还原 ----

const customRules = reactive<CustomRuleDraft[]>([])

function currentDraftsSnapshot(): string {
  return JSON.stringify(customRules)
}

/**
 * 本页管的是「不在内置七条名单里」的一切规则——与 Danmaku.vue 互补。
 * 合并保存时，凡是不属于内置七条的现有规则，都被本页新组装的
 * customRules 整批替换；内置七条原样保留，不受本页保存影响。
 */
function isOwnedByCustomPage(name: string): boolean {
  return !BUILTIN_RULE_NAMES.includes(name)
}

const { dirty, saving, partialFailureMessage, markSaved, save } = useDraft({
  bindingId: () => bindings.current?.id ?? null,
  snapshot: currentDraftsSnapshot,
  isOwned: isOwnedByCustomPage,
  buildRules: () => customRules.map(buildCustomRule),
})

async function loadRules() {
  const b = bindings.current
  if (!b) return
  loading.value = true
  try {
    const rules = await request<RuleView[]>('GET', `/api/bindings/${b.id}/rules`)
    const drafts = rules.filter(isCustomRule).map(parseCustomRuleDraft)
    customRules.splice(0, customRules.length, ...drafts)
    markSaved()
    // 换了绑定就把上一个绑定的「保存了一半」提示清掉——理由见
    // Danmaku.vue 同一处注释：不清的话，操作者在新绑定关掉这条提示，会把
    // 旧绑定那个未解决的重载失败信号一并清没，而旧绑定的引擎其实还在
    // 跑旧配置。
    partialFailureMessage.value = null
  } catch (e) {
    message.error(e instanceof ApiError ? e.message : '加载规则失败')
  } finally {
    loading.value = false
  }
}

// ---- 自动禁言预填：从「房管页 -> 去配置」跳转过来时预填一条草稿 ----
//
// Moderation.vue 的「自动禁言规则」卡片跳过来时带 query preset=automute。
// 不用 useRoute()——本页很多测试直接 mount(Custom.vue) 不挂 vue-router，
// useRoute() 在没有 router 插件时拿到 undefined，访问 .query 会直接抛错。
// 改用 URLSearchParams 读 window.location.search：vue-router 的 history
// 模式本来就会把当前路由同步进浏览器地址栏，两者读到的是同一份 query，
// 不依赖是否挂了 router 插件。
const presetParam = new URLSearchParams(window.location.search).get('preset')
let presetApplied = false

/**
 * maybeApplyPreset 在每次 loadRules() 完成后调用。只应用一次——即使切换
 * 直播间导致 loadRules 重新触发，也不会重复插入第二条预填草稿；也不会在
 * 没有 preset 参数的正常访问路径上做任何事。
 */
function maybeApplyPreset() {
  if (presetApplied || presetParam !== 'automute') return
  presetApplied = true
  customRules.push(buildAutoMutePresetDraft())
  message.info('已为你预填一条「关键词禁言」草稿，填好关键词后记得点右上角保存')
}

/**
 * buildAutoMutePresetDraft 是「去配置」按钮承诺的那一半：给出一条已经能跑
 * 的骨架（弹幕命中关键词 -> 禁言 1 小时），而不是让用户从空白页开始搭。
 * 关键词留空由用户自己填——禁言关键词因人而异，编不出默认值；硬塞一个
 * 示例词还可能被当成"已经配置好"直接点了保存。
 */
function buildAutoMutePresetDraft(): CustomRuleDraft {
  const draft = defaultCustomRuleDraft()
  draft.name = '自动禁言（关键词，待填写）'
  draft.on = ['danmaku']
  draft.whenEnabled = true
  draft.when = { field: 'text', op: 'contains', value: '' }
  draft.actions = [defaultActionDraft('block')]
  return draft
}

async function loadRulesThenMaybeApplyPreset() {
  await loadRules()
  maybeApplyPreset()
}

onMounted(() => void loadMeta())

// 切换直播间要重新加载——上一个直播间的自定义规则不该带到下一个直播间去。
watch(
  () => bindings.currentId,
  () => void loadRulesThenMaybeApplyPreset(),
  { immediate: true },
)

function addRule() {
  customRules.push(defaultCustomRuleDraft())
}

function confirmRemoveRule(draft: CustomRuleDraft) {
  dialog.warning({
    title: '删除自定义规则',
    content: `确定要删除「${draft.name || '（未命名）'}」这条草稿吗？尚未保存到后端，但当前编辑的内容会丢失。`,
    positiveText: '删除',
    negativeText: '取消',
    onPositiveClick: () => {
      const idx = customRules.findIndex((r) => r.id === draft.id)
      if (idx !== -1) customRules.splice(idx, 1)
    },
  })
}

function addAction(draft: CustomRuleDraft) {
  draft.actions.push(defaultActionDraft())
}

function removeAction(draft: CustomRuleDraft, index: number) {
  // 与 spec.Rule.Validate() 的要求一致（do 不能为空数组），至少留一条。
  if (draft.actions.length <= 1) return
  draft.actions.splice(index, 1)
}

/**
 * onSave 接 useDraft 的保存流程，与 Danmaku.vue 同一套约定：
 * GET 现有规则 → 合并（保留内置七条，替换本页管的自定义规则）→ PUT → POST reload。
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
  <div class="custom-page">
    <div class="page-header">
      <h2>自定义弹幕姬</h2>
      <SaveBar :dirty="dirty" :saving="saving" @save="onSave" />
    </div>

    <!-- 第三态：PUT 写库成功、但 POST reload 失败——见 useDraft.ts 文件头说明，
         dirty 这时候不归假，单靠 SaveBar 的「有未保存的改动」看不出「已经保存了一半」，
         所以单独给一条持久提示。 -->
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
      <NAlert type="info" :bordered="false" class="intro-alert">
        给主播完全的自由度：<strong>触发器 + 模板</strong>的组合。比如「指定 UID 进房 且
        大航海状态有效」→
        发送「欢迎我最心爱的舰长回家」。变量清单、操作符、事件/动作类型均从后端下发。
      </NAlert>

      <NSpin :show="loading">
        <NEmpty
          v-if="customRules.length === 0"
          description="还没有自定义规则，点下面按钮新建一条"
          class="empty-rules"
        />

        <NCard
          v-for="draft in customRules"
          :key="draft.id"
          class="rule-card"
          content-style="padding-top: 12px"
        >
          <template #header>
            <NInput
              v-model:value="draft.name"
              placeholder="规则名（如：舰长专属欢迎）"
              style="width: 280px"
            />
          </template>
          <template #header-extra>
            <div class="header-extra">
              <NSwitch v-model:value="draft.enabled" />
              <NButton size="small" quaternary type="error" @click="confirmRemoveRule(draft)">
                删除规则
              </NButton>
            </div>
          </template>

          <!-- ==================== 触发方式 ==================== -->
          <h4>触发方式</h4>
          <NRadioGroup v-model:value="draft.triggerMode">
            <NRadio
              v-for="opt in TRIGGER_MODE_OPTIONS"
              :key="opt.value"
              :value="opt.value"
              class="radio-item"
            >
              {{ opt.label }}
            </NRadio>
          </NRadioGroup>

          <div v-if="draft.triggerMode === 'on'" class="row">
            <span class="label">监听的事件类型</span>
            <NSelect
              v-model:value="draft.on"
              multiple
              filterable
              :options="eventTypeOptions.map((o) => ({ label: o.label, value: o.value }))"
              placeholder="选择一个或多个事件类型"
              style="min-width: 320px"
            />
          </div>
          <template v-else>
            <NRadioGroup v-model:value="draft.scheduleMode" class="row">
              <NRadio
                v-for="opt in SCHEDULE_MODE_OPTIONS"
                :key="opt.value"
                :value="opt.value"
                class="radio-item"
              >
                {{ opt.label }}
              </NRadio>
            </NRadioGroup>
            <div v-if="draft.scheduleMode === 'interval'" class="row">
              <span class="label">每隔（分钟）</span>
              <NInputNumber v-model:value="draft.intervalMinutes" :min="1" style="width: 140px" />
            </div>
            <div v-else class="row">
              <span class="label">cron 表达式（6 段：秒 分 时 日 月 周）</span>
              <NInput
                v-model:value="draft.cronExpr"
                placeholder="0 */10 * * * *"
                style="width: 220px"
              />
            </div>
          </template>

          <!-- ==================== 条件 ==================== -->
          <h4>
            <span>触发条件</span>
            <NSwitch v-model:value="draft.whenEnabled" size="small" class="inline-switch" />
            <span class="hint-inline">{{
              draft.whenEnabled ? '满足条件才触发' : '无条件，事件一到就触发'
            }}</span>
          </h4>
          <ConditionTree
            v-if="draft.whenEnabled"
            v-model="draft.when"
            :operators="operatorOptions"
            :field-options="COMMON_FIELD_OPTIONS"
          />

          <!-- ==================== 合并窗口 ==================== -->
          <h4>
            <span>合并窗口</span>
            <NSwitch v-model:value="draft.aggregateEnabled" size="small" class="inline-switch" />
          </h4>
          <div v-if="draft.aggregateEnabled" class="row">
            <span class="label">分组方式</span>
            <NSelect
              v-model:value="draft.aggregateBy"
              :options="aggregateByOptions.map((o) => ({ label: o.label, value: o.value }))"
              style="width: 260px"
            />
            <span class="label">合并窗口（秒）</span>
            <NInputNumber v-model:value="draft.windowSeconds" :min="1" style="width: 120px" />
            <span class="label">最长等待（秒，可选）</span>
            <NInputNumber
              v-model:value="draft.maxWaitSeconds"
              :min="0"
              clearable
              placeholder="不设上限"
              style="width: 120px"
            />
            <span class="label">最少合并数量</span>
            <NInputNumber v-model:value="draft.minCount" :min="1" style="width: 100px" />
          </div>

          <!-- ==================== 冷却 ==================== -->
          <h4>
            <span>冷却</span>
            <NSwitch v-model:value="draft.cooldownEnabled" size="small" class="inline-switch" />
          </h4>
          <div v-if="draft.cooldownEnabled" class="row">
            <span class="label">最小触发间隔（秒）</span>
            <NInputNumber v-model:value="draft.cooldownSeconds" :min="1" style="width: 120px" />
          </div>
          <div class="row">
            <span class="label">命名冷却组（可选，与其它规则共享冷却时留空则不共享）</span>
            <NInput
              v-model:value="draft.cooldownGroup"
              placeholder="留空表示不加入任何冷却组"
              style="width: 200px"
            />
          </div>

          <!-- ==================== 动作 ==================== -->
          <h4>触发后执行的动作</h4>
          <div v-for="(action, idx) in draft.actions" :key="idx" class="action-row">
            <NSelect
              v-model:value="action.type"
              :options="actionTypeOptions.map((o) => ({ label: o.label, value: o.value }))"
              style="width: 200px"
            />
            <NButton
              size="small"
              quaternary
              type="error"
              :disabled="draft.actions.length <= 1"
              @click="removeAction(draft, idx)"
            >
              删除此动作
            </NButton>
            <div class="action-fields">
              <TemplateList
                v-if="action.type === 'danmaku'"
                v-model="action.templates"
                placeholder="弹幕模板"
              />
              <div v-else-if="action.type === 'block'" class="row">
                <span class="label">禁言小时数</span>
                <NInputNumber v-model:value="action.hours" :min="1" style="width: 140px" />
              </div>
              <NInput
                v-else-if="action.type === 'script'"
                v-model:value="action.script"
                type="textarea"
                placeholder="脚本内容（goja 执行）"
                :autosize="{ minRows: 2, maxRows: 6 }"
              />
              <p v-else class="hint">这个动作类型没有额外参数。</p>
            </div>
          </div>
          <NButton size="small" dashed @click="addAction(draft)">+ 添加动作</NButton>

          <!-- ==================== 排除通用规则：界面做出来，标悬空 ==================== -->
          <h4>
            排除通用规则
            <NTooltip>
              <template #trigger>
                <NTag type="warning" size="small">待后端支持</NTag>
              </template>
              规则引擎目前没有"一条规则命中后跳过指定规则"这种互斥/优先级机制（设计文档
              §13.6），spec.Rule 也没有字段能装这份声明。下面的多选框能选、能预览，
              但连状态都不会被保存——点了保存也不会写进后端的规则里，刷新页面、切换
              直播间、或者离开这页再回来，这里的选择都会复位成空。不是"存了但引擎不认"，
              是压根没地方存，需要引擎侧新增"命中后跳过指定规则"的能力。
            </NTooltip>
          </h4>
          <p class="hint">
            典型场景：给某位舰长配了专属进房欢迎，就不该再触发通用进房欢迎——否则那位舰长进房会被欢迎两次。
          </p>
          <NSelect
            v-model:value="draft.excludeBuiltinRules"
            multiple
            :options="BUILTIN_RULE_OPTIONS"
            placeholder="选择命中后要屏蔽的通用规则（当前不生效）"
            style="min-width: 320px"
          />

          <NCollapse class="preview-collapse">
            <NCollapseItem title="预览将要生成的规则 JSON（本地草稿，尚未保存）" name="preview">
              <pre class="json-preview">{{ JSON.stringify(buildCustomRule(draft), null, 2) }}</pre>
              <p class="hint">
                「排除通用规则」的选择不会出现在上面的 JSON 里——见上方"排除通用规则"小节的悬空说明。
              </p>
            </NCollapseItem>
          </NCollapse>
        </NCard>

        <NButton dashed block class="add-rule-btn" @click="addRule">+ 新增自定义规则</NButton>
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
.intro-alert {
  margin-bottom: 16px;
}
.partial-failure-alert {
  margin-bottom: 16px;
}
.empty-rules {
  margin: 24px 0;
}
.rule-card {
  margin-bottom: 16px;
}
.header-extra {
  display: flex;
  align-items: center;
  gap: 8px;
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
h4 {
  display: flex;
  align-items: center;
  gap: 8px;
}
.inline-switch {
  margin-left: 4px;
}
.hint-inline {
  font-size: 12px;
  opacity: 0.7;
  font-weight: normal;
}
.action-row {
  display: flex;
  flex-direction: column;
  gap: 8px;
  padding: 8px;
  border: 1px dashed rgba(128, 128, 128, 0.3);
  border-radius: 6px;
  margin-bottom: 8px;
}
.action-fields {
  width: 100%;
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
.add-rule-btn {
  margin-top: 4px;
}
</style>
