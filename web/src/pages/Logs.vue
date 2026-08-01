<script setup lang="ts">
/**
 * Logs 是「日志」页（设计文档 §7.2 页面 5）。
 *
 * **这一页有两种数据源，不能混为一谈**：
 *
 *   - 历史：`GET /activity`，一次拉取、可按 kind/eventType/时间过滤，
 *     是规则引擎记下来的业务日志（哪条规则命中了、执行了什么动作）。
 *   - 实时：SSE `/stream`，持续推送每一条归一化后的原始事件——不管有没有
 *     规则命中都会推。两者字段形状也不一样（activityView 有 ruleName/
 *     actionType，StreamEvent 只有 type/payload），所以「规则名」这一列
 *     对实时行天然是空的，不是 bug。
 *
 * 「实时」开关打开时，SSE 推来的事件混进列表顶部；关掉时只看历史查询结果，
 * 同时断开 SSE 连接——开着日志页却不看实时数据不该白占一个长连接。
 *
 * 「清除」按钮：设计文档要求真的删库，但 P4-1 的后端没有删除业务日志的
 * 接口。所以这里跟 Moderation.vue「主播区」同一处理方式：控件照常渲染、
 * **不 disabled**（会让人以为是权限问题），标一个「待后端支持」的
 * NTag + NTooltip，点击如实提示。
 *
 * 关键词过滤在前端做（后端 `/activity` 没有搜索参数），所以界面上必须
 * 说清楚「只在已加载的记录里搜」——不说明的话，用户会以为搜的是全部历史，
 * 搜不到就以为这段时间真的没有记录。
 */
import { computed, h, onMounted, onUnmounted, ref, shallowRef, watch } from 'vue'
import {
  NButton,
  NCode,
  NDataTable,
  NDatePicker,
  NEmpty,
  NInput,
  NSelect,
  NSpin,
  NSwitch,
  NTag,
  NTooltip,
  useMessage,
} from 'naive-ui'
import type { DataTableColumns } from 'naive-ui'
import { ApiError, request } from '@/api'
import type { Activity } from '@/api'
import { useBindingsStore } from '@/stores/bindings'
import { useEventStream } from '@/composables/useEventStream'
import type { StreamEvent } from '@/composables/useEventStream'

const bindings = useBindingsStore()
const message = useMessage()

/** 「值 + 中文说明」，与后端 GET /api/meta/event-types 的返回形状一致。 */
interface MetaItem {
  value: string
  label: string
}

// ---- 过滤器 ----

const KIND_OPTIONS = [
  { label: '全部', value: 'all' },
  { label: '事件', value: 'event' },
  { label: '动作', value: 'action' },
]

const kindFilter = ref<'all' | 'event' | 'action'>('all')
const eventTypeFilter = ref<string | null>(null)
const eventTypeOptions = ref<MetaItem[]>([])
/** [since, until]（毫秒时间戳），双选任一没填就是 null。 */
const range = ref<[number, number] | null>(null)
/** 前端本地过滤，不发给后端——后端 /activity 没有搜索参数。 */
const keyword = ref('')
/** 实时开关，默认开。关掉时断开 SSE，避免白占一个长连接。 */
const realtime = ref(true)

async function loadEventTypeOptions() {
  try {
    eventTypeOptions.value = await request<MetaItem[]>('GET', '/api/meta/event-types')
  } catch (e) {
    message.error(e instanceof ApiError ? e.message : '加载事件类型清单失败')
  }
}

// ---- 历史查询 ----

const history = ref<Activity[]>([])
const loadingHistory = ref(false)

async function loadHistory() {
  const b = bindings.current
  if (!b) {
    history.value = []
    return
  }
  loadingHistory.value = true
  try {
    const params = new URLSearchParams()
    if (kindFilter.value !== 'all') params.set('kind', kindFilter.value)
    if (eventTypeFilter.value) params.set('eventType', eventTypeFilter.value)
    if (range.value) {
      params.set('since', new Date(range.value[0]).toISOString())
      params.set('until', new Date(range.value[1]).toISOString())
    }
    const qs = params.toString()
    history.value = await request<Activity[]>(
      'GET',
      `/api/bindings/${b.id}/activity${qs ? `?${qs}` : ''}`,
    )
  } catch (e) {
    // 时间填反（since 晚于 until）后端返回 422，原文本来就是写给操作者
    // 看的，例如「since 不能晚于 until（since=..., until=...）」，
    // 不能包装成「查询失败」——包装掉的话用户不知道要改哪个输入框
    message.error(e instanceof ApiError ? e.message : '加载历史日志失败')
  } finally {
    loadingHistory.value = false
  }
}

// ---- 实时流：随「当前绑定」与「实时开关」联动开关 ----

const streamHandle = shallowRef<ReturnType<typeof useEventStream> | null>(null)

function stopStream() {
  streamHandle.value?.close()
  streamHandle.value = null
}

function startStream(bindingId: number) {
  stopStream()
  streamHandle.value = useEventStream(bindingId)
}

watch(
  () => [bindings.currentId, realtime.value] as const,
  ([id, on]) => {
    if (on && id !== null) startStream(id)
    else stopStream()
  },
  { immediate: true },
)

watch(
  () => bindings.currentId,
  () => void loadHistory(),
  { immediate: true },
)

onMounted(() => void loadEventTypeOptions())
onUnmounted(stopStream)

// ---- 历史记录 + 实时事件 → 统一的表格行 ----

interface LogRow {
  key: string
  time: string
  type: string
  ruleName: string
  user: string
  detail: unknown
  realtime: boolean
}

function historyToRow(a: Activity): LogRow {
  return {
    key: `h-${a.id}`,
    time: a.occurredAt,
    type: a.kind === 'action' ? a.actionType : a.eventType,
    ruleName: a.ruleName || '-',
    user: a.userName || a.userUid || '-',
    detail: a.detail,
    realtime: false,
  }
}

/**
 * 从 StreamEvent.payload 里挑用户信息展示。
 *
 * payload 的具体形状因 event.Type 而异（Danmaku/Gift/... 各不相同），
 * 前端拿到手只是 unknown——这里只做「有就取，没有就显示 -」的防御式读取，
 * 不对某一种事件类型的字段结构做强假设。
 */
function extractStreamUser(payload: unknown): string {
  if (payload && typeof payload === 'object' && 'User' in (payload as Record<string, unknown>)) {
    const u = (payload as Record<string, unknown>).User as
      { UID?: string; Username?: string } | undefined
    if (u?.Username) return u.UID ? `${u.Username}(${u.UID})` : u.Username
  }
  return '-'
}

function streamToRow(e: StreamEvent): LogRow {
  return {
    key: `s-${e.id}`,
    time: e.timestamp,
    type: e.type,
    // 实时事件是规则引擎之前的原始事件，谈不上「命中了哪条规则」
    ruleName: '-',
    user: extractStreamUser(e.payload),
    detail: e.payload,
    realtime: true,
  }
}

/**
 * 实时行要再过一遍 kind/eventType 过滤——历史行已经在后端按这两项筛过，
 * 这里不用重复筛；但 SSE 推的是未经筛选的全量事件，不在前端挡一道的话，
 * 用户选了「只看 danmaku」还是会看到礼物事件混进来。
 *
 * kind==='action' 时实时行全部隐藏：SSE 推的是原始事件，永远不是
 * 「规则命中后执行的动作」，混进去也对不上号。
 */
const streamRows = computed<LogRow[]>(() => {
  if (!realtime.value) return []
  const events = streamHandle.value?.events.value ?? []
  return events
    .filter(() => kindFilter.value !== 'action')
    .filter((e) => !eventTypeFilter.value || e.type === eventTypeFilter.value)
    .map(streamToRow)
})

const historyRows = computed<LogRow[]>(() => history.value.map(historyToRow))

/** 实时事件混进列表顶部，历史记录跟在后面。 */
const mergedRows = computed<LogRow[]>(() => [...streamRows.value, ...historyRows.value])

const displayRows = computed<LogRow[]>(() => {
  const kw = keyword.value.trim().toLowerCase()
  if (!kw) return mergedRows.value
  return mergedRows.value.filter((r) => {
    const hay = `${r.type} ${r.ruleName} ${r.user} ${JSON.stringify(r.detail)}`.toLowerCase()
    return hay.includes(kw)
  })
})

// ---- 详情列：默认折叠，点开才渲染 NCode，避免几百行 JSON 一次性铺满屏幕 ----

const expandedKeys = ref<Set<string>>(new Set())

function toggleExpand(key: string) {
  const next = new Set(expandedKeys.value)
  if (next.has(key)) next.delete(key)
  else next.add(key)
  expandedKeys.value = next
}

const columns: DataTableColumns<LogRow> = [
  { title: '时间', key: 'time', width: 190 },
  {
    title: '类型',
    key: 'type',
    width: 170,
    render: (row) =>
      row.realtime
        ? h('span', [
            h(NTag, { size: 'small', type: 'success' }, { default: () => '实时' }),
            ' ',
            row.type,
          ])
        : row.type,
  },
  { title: '规则名', key: 'ruleName', width: 140 },
  { title: '用户', key: 'user', width: 160 },
  {
    title: '详情',
    key: 'detail',
    render: (row) => {
      if (!expandedKeys.value.has(row.key)) {
        return h(
          NButton,
          { size: 'tiny', text: true, onClick: () => toggleExpand(row.key) },
          { default: () => '查看详情' },
        )
      }
      return h('div', [
        h(
          NButton,
          { size: 'tiny', text: true, onClick: () => toggleExpand(row.key) },
          { default: () => '收起' },
        ),
        h(NCode, {
          code: JSON.stringify(row.detail, null, 2),
          language: 'json',
          style: 'max-width: 640px; white-space: pre-wrap;',
        }),
      ])
    },
  },
]

// ---- 清除：后端没有删除业务日志的接口 ----

/** 如实告知，而不是假装删了什么。不 disabled——那会让人以为是权限问题。 */
function clearNotSupported() {
  message.warning('后端尚未提供删除业务日志的接口')
}
</script>

<template>
  <div class="logs-page">
    <h2>日志</h2>

    <NEmpty v-if="!bindings.current" description="请先在顶部选择一个直播间" />

    <template v-else>
      <div class="filters">
        <NSelect v-model:value="kindFilter" :options="KIND_OPTIONS" style="width: 100px" />
        <NSelect
          v-model:value="eventTypeFilter"
          :options="eventTypeOptions"
          clearable
          placeholder="全部类型"
          style="width: 180px"
        />
        <NDatePicker v-model:value="range" type="datetimerange" clearable style="width: 380px" />
        <NButton type="primary" @click="loadHistory">查询</NButton>
        <div class="realtime-toggle">
          <NSwitch v-model:value="realtime" />
          <span>实时</span>
        </div>
      </div>

      <div class="search-row">
        <NInput v-model:value="keyword" placeholder="按关键词搜索" style="width: 260px" />
        <span class="hint">只在已加载的记录里搜，不检索全部历史</span>
        <div class="clear-group">
          <NTooltip>
            <template #trigger>
              <NTag type="warning" size="small">待后端支持</NTag>
            </template>
            后端尚未提供删除业务日志的接口，点击「清除」只会提示，不会真的删除任何数据
          </NTooltip>
          <NButton @click="clearNotSupported">清除</NButton>
        </div>
      </div>

      <NSpin :show="loadingHistory">
        <NDataTable
          :columns="columns"
          :data="displayRows"
          :row-key="(row: LogRow) => row.key"
          :bordered="false"
          size="small"
        />
        <NEmpty v-if="displayRows.length === 0" description="没有符合条件的记录" size="small" />
      </NSpin>
    </template>
  </div>
</template>

<style scoped>
.filters {
  display: flex;
  align-items: center;
  gap: 8px;
  margin-bottom: 12px;
  flex-wrap: wrap;
}
.realtime-toggle {
  display: flex;
  align-items: center;
  gap: 6px;
  font-size: 13px;
}
.search-row {
  display: flex;
  align-items: center;
  gap: 8px;
  margin-bottom: 12px;
  flex-wrap: wrap;
}
.hint {
  font-size: 12px;
  opacity: 0.7;
}
.clear-group {
  display: flex;
  align-items: center;
  gap: 6px;
  margin-left: auto;
}
</style>
