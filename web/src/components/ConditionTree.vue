<script lang="ts">
/**
 * ConditionTree 是 `spec.Condition` 递归树的可视化编辑器（Task 11，设计文档
 * §7.2 页面 5）。这是整个 P4-2 里最难的一个组件：`Condition` 的五种形态
 * （叶子 `field/op/value`、`all`、`any`、`not`、`script`）互斥，`all`/`any`/
 * `not` 可以任意深度嵌套自己。
 *
 * ## 更新策略：props 向下、emit 向上，整棵子树整体上报（不是原地改 props）
 *
 * 与 `PkPanel.vue` 的 `patch()` 同一套约定：任何一层的任何改动，都由那一层
 * 现算出「自己现在应该长什么样」的一份全新 `Condition` 对象，`emit`
 * 给父级；`all`/`any` 容器收到某个子节点的新值后，把自己持有的子节点数组
 * 对应位置替换掉，再拼出自己的新 `Condition`、继续往上报。一路到根节点
 * （`Custom.vue` 持有的 `reactive` 草稿），根节点直接把新值整体写回
 * `draft.when`。**全程不允许任何一层直接修改 `props.modelValue`**——
 * `vue/no-mutating-props` 也不允许。
 *
 * 这意味着深层叶子的一次改动，会依次触发它到根的每一层重新构造对象、
 * 重新 emit，链路长度等于嵌套深度。写在这里明确一下，省得以后有人觉得
 * "怎么改一个叶子好像整棵树的对象引用都变了"是 bug——这是设计如此。
 *
 * ## 值（value）的类型怎么表达
 *
 * 后端 `spec.Condition.Value` 是 Go 的 `any`，`eq 5`（数字）与
 * `eq "5"`（字符串）在 `rules/condition.go` 的比较语义下是不同的两件事。
 * 界面上不能只给一个文本框，否则用户永远只能拼出字符串比较。
 *
 * 这里加一个「值类型」下拉：文本 / 数字 / 布尔 / 列表。
 * - 文本 → 原样字符串
 * - 数字 → `NInputNumber`，序列化成 JS number
 * - 布尔 → `NSwitch`，序列化成 true/false
 * - 列表 → 给 `in` 操作符用（如 `user.guardLevel in [1,2,3]`）。
 *   列表本身还要再选一次"元素是文本还是数字"，因为 `in` 两边比较时
 *   数字 5 与字符串 "5" 同样不相等
 *
 * 选择操作符为 `in` 时，如果当前值类型还不是"列表"，自动切成列表——
 * 这是最常见的用法，但不强制：切完用户还能自己改回文本/数字，
 * 只是那种写法多半通不过后端语义，界面不阻止，因为「怎么用是用户的事」
 * 与「界面本身要不要给个顺手的默认值」是两件事。
 *
 * ## 删到空怎么处理
 *
 * `all: []`/`any: []` 在 JSON 层面能序列化，但 `rules.Condition.Validate()`
 * （`server/internal/rules/rule.go`）统计"有几种形态被指定"时用的是
 * `len(c.All) > 0`，空数组等于"一种都没指定"，会被判成
 * "条件为空，须指定 field/all/any/not/script 之一"直接拒收——
 * 不是"合法但无意义"，是根本过不了校验。
 *
 * 界面上不阻止用户把 all/any 删空（删的时候不用二次确认，草稿状态删空
 * 不是破坏性操作），但会在空容器旁边给出提示。真正的处理在 `pruneCondition`
 * 里：递归剪掉空的 all/any 分支；单个子节点的 all/any 收拢成子节点自己
 * （all 只剩一项等于就是那一项）；整棵树剪成空，就等于"没有条件"，
 * 与 `Danmaku.vue` 里 `buildEnterCondition` 的既有约定一致
 * （`leaves.length === 0 → return undefined`）。`not` 的子节点剪空后，
 * 同样把 `not` 一起丢掉——"对空条件取反"没有良好定义的语义，这里选择
 * "等于没有约束"而不是报错，是为了让草稿状态始终能生成一个可预览的
 * JSON，真正的非法输入交给后端在保存时报错（用户的既定原则：
 * 后端接口统一最后处理）。
 */
import type { Condition } from '@/api/rule-types'

/** 「值 + 中文说明」，与 `GET /api/meta/*` 的返回形状一致。 */
export interface MetaItem {
  value: string
  label: string
}

export type ConditionKind = 'leaf' | 'all' | 'any' | 'not' | 'script' | 'empty'

/**
 * kindOf 判断一个 Condition 属于五种形态中的哪一种。
 *
 * 'empty' 是第六种返回值，专指"五个字段全部缺省"——通常只出现在
 * 刚创建、还没编辑过的节点上。调用方一般把它当 'leaf' 处理（给用户一个
 * 空叶子表单去填），但 kindOf 本身如实报告，不做这层归并，
 * 归并逻辑留给调用方决定。
 */
export function kindOf(c: Condition): ConditionKind {
  if (c.all !== undefined) return 'all'
  if (c.any !== undefined) return 'any'
  if (c.not !== undefined) return 'not'
  if (c.script !== undefined) return 'script'
  if (c.field !== undefined) return 'leaf'
  return 'empty'
}

/** defaultLeafCondition 是新建叶子节点的初始形状。 */
export function defaultLeafCondition(): Condition {
  return { field: '', op: 'eq', value: '' }
}

/** defaultConditionForKind 是切换节点形态时用来重建 Condition 的工厂。 */
export function defaultConditionForKind(kind: ConditionKind): Condition {
  switch (kind) {
    case 'leaf':
    case 'empty':
      return defaultLeafCondition()
    case 'all':
      return { all: [defaultLeafCondition()] }
    case 'any':
      return { any: [defaultLeafCondition()] }
    case 'not':
      return { not: defaultLeafCondition() }
    case 'script':
      return { script: '' }
  }
}

/**
 * pruneCondition 把编辑态的 Condition（可能含空 all/any、空 not）
 * 收拢成能过后端 Validate() 的形状，或者判定整棵树等价于"没有条件"。
 *
 * 规则（见文件头注释）：
 * - 叶子：field 为空视为"还没填完"，剪掉；否则原样保留
 * - all/any：递归剪子节点，剪空的丢弃；剩 0 个 → 整个节点消失；
 *   剩 1 个 → 收拢成那个子节点本身；剩 ≥2 个 → 保留容器
 * - not：子节点剪空 → 整个 not 消失；否则保留
 * - script：空字符串（含只有空白）视为"还没写"，剪掉
 * - empty：直接消失
 */
export function pruneCondition(c: Condition): Condition | undefined {
  switch (kindOf(c)) {
    case 'leaf':
      return c.field ? { field: c.field, op: c.op, value: c.value } : undefined
    case 'all': {
      const pruned = (c.all ?? [])
        .map(pruneCondition)
        .filter((x): x is Condition => x !== undefined)
      if (pruned.length === 0) return undefined
      if (pruned.length === 1) return pruned[0]
      return { all: pruned }
    }
    case 'any': {
      const pruned = (c.any ?? [])
        .map(pruneCondition)
        .filter((x): x is Condition => x !== undefined)
      if (pruned.length === 0) return undefined
      if (pruned.length === 1) return pruned[0]
      return { any: pruned }
    }
    case 'not': {
      const child = c.not ? pruneCondition(c.not) : undefined
      return child ? { not: child } : undefined
    }
    case 'script':
      return c.script && c.script.trim() !== '' ? { script: c.script } : undefined
    case 'empty':
      return undefined
  }
}

// ---- 叶子节点「值」的类型表达 ----

export type ValueKind = 'string' | 'number' | 'boolean' | 'list'

export const VALUE_KIND_OPTIONS: { label: string; value: ValueKind }[] = [
  { label: '文本', value: 'string' },
  { label: '数字', value: 'number' },
  { label: '布尔', value: 'boolean' },
  { label: '列表（给 in 用）', value: 'list' },
]

export type ListElementKind = 'string' | 'number'

/** detectValueKind 从已有的 value 反推它属于哪种值类型，供加载已保存条件时用。 */
export function detectValueKind(v: unknown): ValueKind {
  if (typeof v === 'number') return 'number'
  if (typeof v === 'boolean') return 'boolean'
  if (Array.isArray(v)) return 'list'
  return 'string'
}

/** detectListElementKind 从已有列表反推元素类型：看第一个元素。空列表默认文本。 */
export function detectListElementKind(v: unknown): ListElementKind {
  if (Array.isArray(v) && v.length > 0 && typeof v[0] === 'number') return 'number'
  return 'string'
}

/**
 * buildLeafValue 把值编辑器的本地状态序列化成最终写进 Condition.value 的值。
 *
 * 列表元素类型为数字时，非数字文本（用户输入了空字符串或非法数字）
 * 会被过滤掉而不是塞进结果——总比序列化出 NaN 到 JSON 里（变成 null）
 * 更容易发现问题。
 */
export function buildLeafValue(state: {
  valueKind: ValueKind
  stringValue: string
  numberValue: number
  boolValue: boolean
  listValues: string[]
  listElementKind: ListElementKind
}): unknown {
  switch (state.valueKind) {
    case 'string':
      return state.stringValue
    case 'number':
      return state.numberValue
    case 'boolean':
      return state.boolValue
    case 'list':
      return state.listElementKind === 'number'
        ? state.listValues.map(Number).filter((n) => !Number.isNaN(n))
        : state.listValues
  }
}

/** 常用变量路径清单，兜底给字段下拉用——见下方 <script setup> 顶部的详细说明。 */
export const COMMON_FIELD_OPTIONS: { label: string; value: string }[] = [
  { label: 'user.uid（用户 UID）', value: 'user.uid' },
  { label: 'user.username（用户昵称）', value: 'user.username' },
  { label: 'user.guardLevel（大航海档位，1总督/2提督/3舰长/0无）', value: 'user.guardLevel' },
  { label: 'user.userLevel（用户等级）', value: 'user.userLevel' },
  { label: 'user.wealthLevel（财富等级）', value: 'user.wealthLevel' },
  { label: 'user.isAdmin（是否房管）', value: 'user.isAdmin' },
  { label: 'user.medal.name（粉丝牌名字）', value: 'user.medal.name' },
  { label: 'user.medal.level（粉丝牌等级）', value: 'user.medal.level' },
  { label: 'user.medal.roomId（粉丝牌所属房间）', value: 'user.medal.roomId' },
  { label: 'user.medal.isLighted（粉丝牌是否点亮）', value: 'user.medal.isLighted' },
  { label: 'text（弹幕/醒目留言正文）', value: 'text' },
  { label: 'gift.name（礼物名）', value: 'gift.name' },
  { label: 'gift.count（礼物数量）', value: 'gift.count' },
  { label: 'gift.totalCoin（礼物总价，瓜子）', value: 'gift.totalCoin' },
  { label: 'guard.level（本次上舰档位）', value: 'guard.level' },
  { label: 'guard.name（本次上舰档位名）', value: 'guard.name' },
  { label: 'guard.count（本次购买月数）', value: 'guard.count' },
  { label: 'guard.isRenew（是否续费）', value: 'guard.isRenew' },
  { label: 'superChat.price（醒目留言价格）', value: 'superChat.price' },
  { label: 'roomId（本直播间房间号）', value: 'roomId' },
]
</script>

<script setup lang="ts">
/**
 * 变量清单的坑：没有 `GET /api/meta/variables` 这个接口。
 *
 * `Condition.field` 是点分路径（`user.uid`、`user.medal.isLighted` 这些），
 * 理论上应该跟 operators/event-types 一样从后端拉，但后端压根没开这个口。
 * 去读了 `server/internal/rules/vars.go`（`VarsFromEvent`/`userVars`）后
 * 把它实际展开的路径整理成了上面的 `COMMON_FIELD_OPTIONS`，作为下拉的
 * 候选项；但 `NSelect` 开了 `filterable tag`，用户可以直接打字输入
 * 任意路径（比如 `battle.subCommand` 这种没进清单的），不受下拉限制。
 *
 * 这是**第二处定义**：后端 `vars.go` 改了字段，这份清单不会跟着变。
 * 已登记进 `docs/superpowers/specs/2026-08-01-p4-2-悬空清单.md`。
 */
import { computed, ref, watch } from 'vue'
import {
  NAlert,
  NButton,
  NDynamicTags,
  NInput,
  NInputNumber,
  NRadioButton,
  NRadioGroup,
  NSelect,
  NSwitch,
  NTag,
  NTooltip,
} from 'naive-ui'

defineOptions({ name: 'ConditionTree' })

const props = defineProps<{
  modelValue: Condition
  operators: MetaItem[]
  fieldOptions: { label: string; value: string }[]
  /** 根节点没有"删除自己"这个动作（删除整棵树是外层"移除 when"的事），
   * all/any/not 的子节点才需要显示"删除此条件"按钮。 */
  removable?: boolean
}>()

const emit = defineEmits<{
  'update:modelValue': [Condition]
  /** 请求父级把「我」从它的子节点列表里摘掉。仅 removable 为真时可能触发。 */
  remove: []
}>()

// ---- 形态（kind）：本地状态是编辑态的唯一真相来源，只在挂载时从 props 取一次初值 ----
//
// 不在这里 watch(props.modelValue) 做双向同步：本组件自己发出的每次 emit
// 都会产出一个新对象引用，若同时监听 props 变化再回写本地状态，
// 编辑输入框时光标位置、正在输入到一半的数字都会被"回声"打断。
// 父级除了把子组件 emit 上来的值原样存好、下一帧再传下来之外，
// 不会有其它来源替换这棵子树，所以只需初始化一次。

function normalizeInitialKind(k: ConditionKind): Exclude<ConditionKind, 'empty'> {
  return k === 'empty' ? 'leaf' : k
}

const kind = ref<Exclude<ConditionKind, 'empty'>>(normalizeInitialKind(kindOf(props.modelValue)))

// ---- 叶子节点状态 ----

const leafField = ref(props.modelValue.field ?? '')
const leafOp = ref(props.modelValue.op || 'eq')
const initialValueKind = detectValueKind(props.modelValue.value)
const valueKind = ref<ValueKind>(initialValueKind)
const stringValue = ref(initialValueKind === 'string' ? String(props.modelValue.value ?? '') : '')
const numberValue = ref(initialValueKind === 'number' ? Number(props.modelValue.value) : 0)
const boolValue = ref(initialValueKind === 'boolean' ? Boolean(props.modelValue.value) : true)
const listElementKind = ref<ListElementKind>(detectListElementKind(props.modelValue.value))
const listValues = ref<string[]>(
  initialValueKind === 'list' ? (props.modelValue.value as unknown[]).map(String) : [],
)

function emitLeaf() {
  emit('update:modelValue', {
    field: leafField.value,
    op: leafOp.value,
    value: buildLeafValue({
      valueKind: valueKind.value,
      stringValue: stringValue.value,
      numberValue: numberValue.value,
      boolValue: boolValue.value,
      listValues: listValues.value,
      listElementKind: listElementKind.value,
    }),
  })
}

function onOpChange(op: string) {
  leafOp.value = op
  // in 最常见的用法是配合列表，值类型还不是列表时顺手切过去，
  // 但不是强制的——用户随时能自己改回去，见文件头注释。
  if (op === 'in' && valueKind.value !== 'list') {
    valueKind.value = 'list'
  }
  emitLeaf()
}

watch(
  [leafField, valueKind, stringValue, numberValue, boolValue, listValues, listElementKind],
  emitLeaf,
)

// ---- all / any 容器：子节点数组，用稳定 id 而不是数组下标做 :key ----
//
// 下标做 key 在"删除中间一个子节点"时会导致后面的子节点全部错位复用，
// 残留的本地 state（比如某个子节点正在编辑到一半的文本框）会串到别的
// 子节点上。这里给每个子节点分配一个挂载时生成的稳定 id。

let idSeq = 0
function nextId(): string {
  idSeq += 1
  return `c${idSeq}`
}

interface ChildSlot {
  id: string
  value: Condition
}

function initialChildren(): ChildSlot[] {
  const arr = kind.value === 'all' ? props.modelValue.all : props.modelValue.any
  return (arr ?? []).map((v) => ({ id: nextId(), value: v }))
}

const children = ref<ChildSlot[]>(
  kind.value === 'all' || kind.value === 'any' ? initialChildren() : [],
)

function emitContainer() {
  const arr = children.value.map((c) => c.value)
  if (kind.value === 'all') emit('update:modelValue', { all: arr })
  else if (kind.value === 'any') emit('update:modelValue', { any: arr })
}

function onChildUpdate(id: string, value: Condition) {
  const idx = children.value.findIndex((c) => c.id === id)
  if (idx === -1) return
  children.value[idx] = { id, value }
  emitContainer()
}

function addChild() {
  children.value.push({ id: nextId(), value: defaultLeafCondition() })
  emitContainer()
}

function removeChild(id: string) {
  children.value = children.value.filter((c) => c.id !== id)
  emitContainer()
}

// ---- not 容器：恰好一个子节点 ----

const notChild = ref<Condition>(props.modelValue.not ?? defaultLeafCondition())

function onNotChildUpdate(value: Condition) {
  notChild.value = value
  emit('update:modelValue', { not: value })
}

// ---- script 节点 ----

const scriptText = ref(props.modelValue.script ?? '')

function onScriptInput(v: string) {
  scriptText.value = v
  emit('update:modelValue', { script: v })
}

// ---- 切换节点形态：重建整个节点，本地各分支的状态互不保留 ----

const KIND_OPTIONS: { label: string; value: Exclude<ConditionKind, 'empty'> }[] = [
  { label: '叶子条件', value: 'leaf' },
  { label: '全部满足（AND）', value: 'all' },
  { label: '任一满足（OR）', value: 'any' },
  { label: '取反（NOT）', value: 'not' },
  { label: '脚本', value: 'script' },
]

function onKindChange(next: Exclude<ConditionKind, 'empty'>) {
  kind.value = next
  if (next === 'all' || next === 'any') {
    children.value = [{ id: nextId(), value: defaultLeafCondition() }]
  } else if (next === 'not') {
    notChild.value = defaultLeafCondition()
  } else if (next === 'leaf') {
    leafField.value = ''
    leafOp.value = 'eq'
    valueKind.value = 'string'
    stringValue.value = ''
  } else if (next === 'script') {
    scriptText.value = ''
  }
  emit('update:modelValue', defaultConditionForKind(next))
}

/** 空容器（all/any 删到 0 个子节点）的提示：不算错误，保存时会被自动忽略。 */
const isEmptyContainer = computed(
  () => (kind.value === 'all' || kind.value === 'any') && children.value.length === 0,
)
</script>

<template>
  <div class="condition-tree">
    <div class="node-header">
      <NRadioGroup :value="kind" size="small" @update:value="onKindChange">
        <NRadioButton v-for="opt in KIND_OPTIONS" :key="opt.value" :value="opt.value">
          {{ opt.label }}
        </NRadioButton>
      </NRadioGroup>
      <NButton v-if="props.removable" size="small" quaternary type="error" @click="emit('remove')">
        删除此条件
      </NButton>
    </div>

    <!-- ==================== 叶子：field op value ==================== -->
    <div v-if="kind === 'leaf'" class="leaf-row">
      <NSelect
        v-model:value="leafField"
        filterable
        tag
        placeholder="变量路径，如 user.uid（可直接输入，不限于下拉项）"
        :options="props.fieldOptions"
        style="min-width: 260px"
      />
      <NSelect
        :value="leafOp"
        :options="props.operators.map((o) => ({ label: o.label, value: o.value }))"
        style="width: 160px"
        @update:value="onOpChange"
      />

      <NSelect v-model:value="valueKind" :options="VALUE_KIND_OPTIONS" style="width: 140px" />
      <NInput
        v-if="valueKind === 'string'"
        v-model:value="stringValue"
        placeholder="比较值（文本）"
        style="width: 180px"
      />
      <NInputNumber
        v-else-if="valueKind === 'number'"
        v-model:value="numberValue"
        placeholder="比较值（数字）"
        style="width: 140px"
      />
      <NSwitch v-else-if="valueKind === 'boolean'" v-model:value="boolValue" />
      <template v-else-if="valueKind === 'list'">
        <NRadioGroup v-model:value="listElementKind" size="small">
          <NRadioButton value="string">文本项</NRadioButton>
          <NRadioButton value="number">数字项</NRadioButton>
        </NRadioGroup>
        <NDynamicTags v-model:value="listValues" />
      </template>
    </div>

    <!-- ==================== all / any：子节点递归 ==================== -->
    <div v-else-if="kind === 'all' || kind === 'any'" class="container-node">
      <NAlert v-if="isEmptyContainer" type="warning" :bordered="false" class="empty-hint">
        当前没有任何子条件，保存时这个分支会被自动忽略（等价于删掉它）。
      </NAlert>
      <div v-for="child in children" :key="child.id" class="child-row">
        <ConditionTree
          :model-value="child.value"
          :operators="props.operators"
          :field-options="props.fieldOptions"
          removable
          @update:model-value="(v: Condition) => onChildUpdate(child.id, v)"
          @remove="removeChild(child.id)"
        />
      </div>
      <NButton size="small" dashed @click="addChild">
        + 添加{{ kind === 'all' ? '一条 AND 子条件' : '一条 OR 子条件' }}
      </NButton>
    </div>

    <!-- ==================== not：单个子节点递归 ==================== -->
    <div v-else-if="kind === 'not'" class="container-node">
      <ConditionTree
        :model-value="notChild"
        :operators="props.operators"
        :field-options="props.fieldOptions"
        @update:model-value="onNotChildUpdate"
      />
    </div>

    <!-- ==================== script：JS 表达式逃生舱 ==================== -->
    <div v-else-if="kind === 'script'" class="script-node">
      <NTooltip>
        <template #trigger>
          <NTag type="info" size="small">goja 执行</NTag>
        </template>
        脚本由后端 goja 引擎求值，须返回布尔值。变量与叶子条件的 field 共用同一张取值表
        （server/internal/rules/vars.go 的 VarsFromEvent）。
      </NTooltip>
      <NInput
        :value="scriptText"
        type="textarea"
        placeholder="例如：user.guardLevel > 0 && gift.totalCoin >= 1000"
        :autosize="{ minRows: 2, maxRows: 6 }"
        @update:value="onScriptInput"
      />
    </div>
  </div>
</template>

<style scoped>
.condition-tree {
  border: 1px solid rgba(128, 128, 128, 0.25);
  border-radius: 6px;
  padding: 8px;
}
.node-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
  margin-bottom: 8px;
  flex-wrap: wrap;
}
.leaf-row {
  display: flex;
  align-items: center;
  gap: 8px;
  flex-wrap: wrap;
}
.container-node {
  display: flex;
  flex-direction: column;
  gap: 8px;
  padding-left: 12px;
  border-left: 2px solid rgba(128, 128, 128, 0.2);
}
.child-row {
  display: flex;
}
.child-row > * {
  flex: 1;
}
.empty-hint {
  font-size: 12px;
}
.script-node {
  display: flex;
  flex-direction: column;
  gap: 6px;
}
</style>
