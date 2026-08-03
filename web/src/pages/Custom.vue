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
 * 页面而言不存在。这一页反过来：**排除掉 `BUILTIN_RULE_NAMES` 里那些
 * 固定名字之后剩下的规则都是"自定义规则"**，`isCustomRule` 就是这道
 * 过滤。
 *
 * **重名/占用内置命名空间的校验，前端也做一道**（P4-4 Task 7 复审
 * 加的）：新建或改名的自定义规则如果以 `内置/` 开头，会在下次保存时
 * 被 `isCustomRule` 判成"内置规则"，从而在合并逻辑里被本页新组装的
 * `customRules` 悄悄顶掉——用户辛苦配的规则不会报任何错，就这么在下一次
 * 保存时消失了。`isReservedRuleName` 在两处拦这个前缀：名字输入框旁
 * 实时显示错误提示（v-model 驱动，不用等失焦）、`onSave` 里再兜底拦一次
 * 拒绝保存——不指望用户先踩一次坑才知道不能这么起名（后端接口统一最后
 * 处理的原则不变，后端仍然是最终防线，这里只是把最容易踩的一种误用
 * 提前到界面上挡住）。
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
 * ## 变量清单：从 GET /api/meta/variables 拉，不再手抄
 *
 * P4-2 时 `fieldOptions` 传的是 `ConditionTree.vue` 里手抄的
 * `COMMON_FIELD_OPTIONS`（20 条，抄自 `vars.go`）——P4-3 后端补上了
 * `GET /api/meta/variables`（原样转发 `rules.VariableCatalog()`：一组
 * 公共变量 `common` + 按事件类型分组的 `byEvent`），本页 `loadMeta()`
 * 里跟其它三份元数据一起拉，`buildFieldOptions` 把 `common` 与全部
 * `byEvent` 分组拍平、按路径去重后传给 `ConditionTree`。清单不全（新事件
 * 类型的字段还没登记）或接口一时没拉到时，`ConditionTree` 的 `NSelect`
 * 仍开着 `filterable tag`，用户始终能直接打字输入任意路径。
 *
 * `optional` 标记的字段（如未佩戴粉丝牌时不存在的 `medal.*`）
 * 仍然出现在候选项里、仍然可选——只是标签里加一句「可能不存在」提示，
 * 不能因为字段是可选的就不让配。
 *
 * ## 「排除通用规则」：真功能（spec.Rule.Suppress）
 *
 * 设计文档 §7.2 明确要求："一条自定义规则命中后，可以声明屏蔽掉哪些
 * 通用功能"，典型场景是"给某位舰长配了专属进房欢迎，就不该再触发通用
 * 进房欢迎"。P4-3 后端加上了 `spec.Rule.Suppress`
 * （`server/internal/rules/spec/spec.go` 第 49-56 行），多选框列出
 * `BUILTIN_RULE_NAMES` 里的全部内置规则名，`CustomRuleDraft.excludeBuiltinRules`
 * 现在原样进 `buildCustomRule` 组装出的 `rule.suppress`。
 *
 * **后端两条校验决定了前端要防两件事**：
 *
 * 1. 压制不存在的规则名——`NewEngine` 在重建引擎（也就是保存后热重载）
 *    时才会报错，前端拦不住也不用拦：既然只能从 `BUILTIN_RULE_OPTIONS`
 *    （固定的内置名单）里选，选出来的名字天然存在，除非哪天这些
 *    内置规则被整体停用/改名——那种情况留给后端在保存时报错足够。
 * 2. 定时触发（`schedule`）的规则配 `suppress` 会被 `Validate()` 直接
 *    拒绝——压制只在"同一次事件触发命中多条规则"时才有意义，定时规则
 *    一次调用只触发自己。**前端选择在 `triggerMode === 'schedule'` 时
 *    直接禁用这个多选框**（而不是等保存时才报错）：不让用户配出一个
 *    必然被拒的组合，比配完之后才告诉他"这个选择保存不了"更省心；
 *    `buildCustomRule` 同时也在组装层面做了兜底——`triggerMode` 为
 *    `schedule` 时，即便 `excludeBuiltinRules` 里还留着上次在事件触发
 *    模式下选的值（切换触发方式不会清空这份草稿状态），组装出的规则
 *    也绝不会带 `suppress` 字段。
 *
 * ## 保存：GET → 合并 → PUT → POST reload（Task 13 接上，现已实接）
 *
 * 与 Danmaku.vue 同一套约定：改动先只进内存草稿；`SaveBar` 的 `save`
 * 事件接的是 `onSave`（本文件下方 `<script setup>` 部分），走 `useDraft`
 * 提供的统一流程：GET 现有规则 → 合并（保留全部内置规则，替换本页管的
 * 自定义规则）→ PUT 写库 → POST reload。第 2 步失败时 `dirty` 不归假，界面会
 * 给一条持久的「已保存到数据库，但重载失败」提示，具体行为见下方
 * `onSave` 与 `partialFailureMessage` 处的注释。
 */
import type { Action, Aggregate, Condition, Rule, RuleView } from '@/api/rule-types'
import { defaultLeafCondition, pruneCondition } from '@/components/ConditionTree.vue'
import {
  ENTER_RULE_NAME,
  GIFT_RULE_NAME,
  BLINDBOX_RULE_NAME,
  BROADCAST_RULE_NAME,
  FOLLOW_RULE_NAME,
  SHARE_RULE_NAME,
  GUARD_RULE_NAME,
} from './Danmaku.vue'
import { PK_RULE_NAME, PK_VISIT_RULE_NAME } from '@/components/PkPanel.vue'

/**
 * Task 9/10 建立的内置规则名——判定"是不是自定义规则"、"排除通用规则"
 * 多选框都靠它。P4-4 Task 7 从七条加到九条：`BLINDBOX_RULE_NAME`（盲盒
 * 单独答谢）与 `PK_VISIT_RULE_NAME`（PK 串门欢迎）两个悬空项这次接上了
 * 真实规则。
 */
export const BUILTIN_RULE_NAMES: string[] = [
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

export const BUILTIN_RULE_OPTIONS: { label: string; value: string }[] = BUILTIN_RULE_NAMES.map(
  (n) => ({ label: n, value: n }),
)

// ---- 变量清单：GET /api/meta/variables 的响应形状 ----

/** VariableItem 对应后端 rules.Variable 序列化后的一项。 */
export interface VariableItem {
  path: string
  label: string
  /** 可能不存在（如未佩戴粉丝牌时没有 medal.*），配条件时仍可选用。 */
  optional: boolean
}

/** VariablesResponse 对应 GET /api/meta/variables 的响应体。 */
export interface VariablesResponse {
  common: VariableItem[]
  byEvent: Record<string, VariableItem[]>
}

/**
 * buildFieldOptions 把 VariablesResponse 拍平成 ConditionTree 要的
 * `{label, value}[]` 候选项列表。
 *
 * 拍平顺序：先 common，再按事件类型名字母序遍历 byEvent 的各分组——顺序
 * 本身不重要（`NSelect` 是 filterable 的，用户主要靠搜索），重要的是
 * 确定性：同一份后端响应每次拍平出的顺序都一样，不依赖 Object.keys 的
 * 遍历顺序在不同环境下是否稳定。
 *
 * **按 path 去重，先出现的赢**：同一个字段名在不同事件类型下可能有不同
 * 说明（如 "text" 在弹幕分组是"弹幕正文"、在醒目留言分组是"醒目留言
 * 正文"），但字段下拉是不区分事件类型的单一列表，不可能让同一个 value
 * 出现两次（NSelect 的候选项要求 value 唯一）。这是有意的简化：label
 * 只是给用户的提示文案，`field` 真正写进条件里的还是那个 path 本身，
 * 说明文字选了哪个不影响条件的实际语义。
 */
export function buildFieldOptions(resp: VariablesResponse): { label: string; value: string }[] {
  const seen = new Set<string>()
  const out: { label: string; value: string }[] = []

  function addAll(vars: VariableItem[]) {
    for (const v of vars) {
      if (seen.has(v.path)) continue
      seen.add(v.path)
      const suffix = v.optional ? '，可能不存在' : ''
      out.push({ label: `${v.path}（${v.label}${suffix}）`, value: v.path })
    }
  }

  addAll(resp.common ?? [])
  for (const eventType of Object.keys(resp.byEvent ?? {}).sort()) {
    addAll(resp.byEvent[eventType])
  }
  return out
}

/** isCustomRule 判断一条从后端拉回来的规则是不是"自定义规则"——不在内置名单（BUILTIN_RULE_NAMES）里就是。 */
export function isCustomRule(rule: RuleView): boolean {
  return !BUILTIN_RULE_NAMES.includes(rule.name ?? '')
}

/**
 * RESERVED_RULE_NAME_PREFIX 是全部内置规则名共用的命名空间。
 *
 * `isCustomRule` 只按"是不是恰好等于 BUILTIN_RULE_NAMES 里的某一个"来
 * 判断，不检查前缀——所以理论上用户可以新建一条叫 `内置/盲盒答谢` 的
 * 自定义规则，这条规则本身合法（后端不校验名字前缀），但只要它的名字
 * 恰好等于任意一个内置名，下次从 `Danmaku.vue`/本页各自加载时，
 * `isCustomRule`/`isOwnedByCustomPage` 就会把它误判成"属于对方"，
 * 合并保存时被静默顶掉——不报错，用户配的规则就这么消失了。
 *
 * 更常见的触发方式不需要恰好撞上九个名字之一：只要用户新建/改名成任意
 * 一个以 `内置/` 开头的名字，就已经进了这个高风险地带（哪天再新增一条
 * 内置规则，撞名的概率就上升一次）。`isReservedRuleName` 拦的是整个
 * 前缀，不是九个具体名字的精确匹配，把这类误用提前到界面上挡住。
 */
export const RESERVED_RULE_NAME_PREFIX = '内置/'

/** isReservedRuleName 判断一个（可能还没保存的）规则名是否落进了内置命名空间。 */
export function isReservedRuleName(name: string): boolean {
  return name.trim().startsWith(RESERVED_RULE_NAME_PREFIX)
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
  minCount: number
  cooldownEnabled: boolean
  cooldownSeconds: number
  cooldownGroup: string
  actions: CustomActionDraft[]
  /**
   * 排除通用规则——对应 spec.Rule.Suppress，只列出 `BUILTIN_RULE_NAMES`
   * 里的内置规则名供选。**只在 triggerMode === 'on' 时才会被
   * `buildCustomRule` 写进 `rule.suppress`**，见文件头注释第 2 点：
   * 定时触发的规则配 suppress 会被后端拒绝，切换触发方式不清空这份草稿，
   * 靠组装时的判断兜底。
   */
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
    if (draft.minCount > 1) agg.minCount = draft.minCount
    rule.aggregate = agg
  }

  if (draft.cooldownEnabled) {
    rule.cooldown = secondsToDuration(draft.cooldownSeconds)
  }
  if (draft.cooldownGroup.trim() !== '') {
    rule.cooldownGroup = draft.cooldownGroup.trim()
  }

  // 只在事件触发模式下带 suppress——定时触发的规则配了会被后端 Validate()
  // 拒绝（见文件头注释第 2 点）。triggerMode 切到 schedule 时草稿里的
  // excludeBuiltinRules 不会被清空（用户随时可能切回来），这里在组装层面
  // 兜底，保证永远不会拼出一个必然被拒的规则。
  if (draft.triggerMode === 'on' && draft.excludeBuiltinRules.length > 0) {
    rule.suppress = [...draft.excludeBuiltinRules]
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

  if (rule.suppress && rule.suppress.length > 0) {
    draft.excludeBuiltinRules = [...rule.suppress]
  }

  return draft
}

export type { RuleView }
</script>

<script setup lang="ts">
import { computed, onMounted, reactive, ref, watch } from 'vue'
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
import { useAuthStore } from '@/stores/auth'
import { useBindingsStore } from '@/stores/bindings'
import { useDraft } from '@/composables/useDraft'
import SaveBar from '@/components/SaveBar.vue'
import TemplateList from '@/components/TemplateList.vue'
import ConditionTree from '@/components/ConditionTree.vue'
import type { MetaItem } from '@/components/ConditionTree.vue'
import PermissionWarning from '@/components/PermissionWarning.vue'

const auth = useAuthStore()
const bindings = useBindingsStore()
const message = useMessage()
const dialog = useDialog()

/**
 * 缺 rule:write 时的警告条。只有 rule:read 的成员能把整页自定义规则配完，
 * 直到点「保存并生效」才被后端 403 打回——白干一整轮配置。与
 * Moderation.vue 缺 user:block 同一套约定：只警告，不锁面板。
 */
const missingWritePerm = computed(() => {
  const b = bindings.current
  return b !== null && !auth.hasPerm(b, 'rule:write')
})

const loading = ref(false)

// ---- 元数据：全部从 GET /api/meta/* 拉，不硬编码 ----
//
// 与 MemberEditor.vue 拉权限点清单同一套模式：这里唯一的真相来源是这
// 四个 ref，界面渲染多少项完全取决于后端这次回了多少项。

const eventTypeOptions = ref<MetaItem[]>([])
const actionTypeOptions = ref<MetaItem[]>([])
const operatorOptions = ref<MetaItem[]>([])
const aggregateByOptions = ref<MetaItem[]>([])
/** 条件树的字段候选项——从 GET /api/meta/variables 拉，见文件头「变量清单」一节。 */
const fieldOptions = ref<{ label: string; value: string }[]>([])

async function loadMeta() {
  try {
    const [events, actions, operators, aggregateBy, variables] = await Promise.all([
      request<MetaItem[]>('GET', '/api/meta/event-types'),
      request<MetaItem[]>('GET', '/api/meta/action-types'),
      request<MetaItem[]>('GET', '/api/meta/operators'),
      request<MetaItem[]>('GET', '/api/meta/aggregate-by'),
      request<VariablesResponse>('GET', '/api/meta/variables'),
    ])
    eventTypeOptions.value = events
    actionTypeOptions.value = actions
    operatorOptions.value = operators
    aggregateByOptions.value = aggregateBy
    fieldOptions.value = buildFieldOptions(variables)
  } catch (e) {
    message.error(e instanceof ApiError ? e.message : '加载元数据失败')
  }
}

// ---- 自定义规则列表：草稿态，加载时从"排除全部内置规则之后剩下的规则"里还原 ----

const customRules = reactive<CustomRuleDraft[]>([])

function currentDraftsSnapshot(): string {
  return JSON.stringify(customRules)
}

/**
 * 本页管的是「不在 BUILTIN_RULE_NAMES 名单里」的一切规则——与
 * Danmaku.vue 互补。合并保存时，凡是不属于内置规则的现有规则，都被
 * 本页新组装的 customRules 整批替换；内置规则原样保留，不受本页保存
 * 影响。
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
 * GET 现有规则 → 合并（保留全部内置规则，替换本页管的自定义规则）→ PUT → POST reload。
 *
 * 保存前先拦一道 `RESERVED_RULE_NAME_PREFIX` 校验——见该常量注释：
 * 撞进内置命名空间的规则会在下一次加载时被静默顶掉，不报任何错，比
 * 后端拒绝这次保存更难发现。在这里挡住比等用户哪天发现规则"消失了"
 * 再回头排查省心得多。
 */
async function onSave() {
  const reserved = customRules.filter((d) => isReservedRuleName(d.name))
  if (reserved.length > 0) {
    message.error(
      `规则名不能以「${RESERVED_RULE_NAME_PREFIX}」开头（${reserved
        .map((d) => d.name || '（未命名）')
        .join('、')}）——这是内置规则专用的命名空间，撞名会导致这条规则在下次加载时被静默替换掉`,
    )
    return
  }
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
      <PermissionWarning
        v-if="missingWritePerm"
        text="你在这个直播间没有 rule:write 权限，保存会被拒绝"
      />

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
            <div class="rule-name-field">
              <NInput
                v-model:value="draft.name"
                placeholder="规则名（如：舰长专属欢迎）"
                style="width: 280px"
                :status="isReservedRuleName(draft.name) ? 'error' : undefined"
              />
              <span v-if="isReservedRuleName(draft.name)" class="reserved-name-hint">
                不能以「{{ RESERVED_RULE_NAME_PREFIX }}」开头——这是内置规则的命名空间，撞名会导致这条规则被静默替换掉
              </span>
            </div>
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
            :field-options="fieldOptions"
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

          <!-- ==================== 排除通用规则 ==================== -->
          <h4>
            排除通用规则
            <NTooltip v-if="draft.triggerMode === 'schedule'">
              <template #trigger>
                <NTag type="info" size="small">定时触发不可用</NTag>
              </template>
              压制只在"同一次事件触发命中多条规则"时才有意义，定时触发的规则一次调用只触发自己，
              配了 suppress 会被后端拒绝保存。切回"事件触发"后即可继续使用；之前选过的内容
              仍保留在草稿里，不会因为切换触发方式而丢失。
            </NTooltip>
          </h4>
          <p class="hint">
            典型场景：给某位舰长配了专属进房欢迎，就不该再触发通用进房欢迎——否则那位舰长进房会被欢迎两次。
          </p>
          <NSelect
            v-model:value="draft.excludeBuiltinRules"
            multiple
            :disabled="draft.triggerMode === 'schedule'"
            :options="BUILTIN_RULE_OPTIONS"
            placeholder="选择命中后要屏蔽的通用规则"
            style="min-width: 320px"
          />

          <NCollapse class="preview-collapse">
            <NCollapseItem title="预览将要生成的规则 JSON（本地草稿，尚未保存）" name="preview">
              <pre class="json-preview">{{ JSON.stringify(buildCustomRule(draft), null, 2) }}</pre>
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
.rule-name-field {
  display: flex;
  flex-direction: column;
  gap: 2px;
}
.reserved-name-hint {
  font-size: 12px;
  color: var(--n-error-color, #d03050);
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
