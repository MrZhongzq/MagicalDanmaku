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
 * 「清除」按钮：接的是 `DELETE /api/bindings/{id}/activity?since=...&until=...&all=1`，
 * 返回 `{"deleted": N}`。设计文档明写「『清除』是真的删库，需要二次确认」，
 * 所以点击先弹 `dialog.warning`，文案要把删除范围与「不可恢复」都说清楚，
 * 确认之后才真的发 DELETE。
 *
 * **两个容易踩的坑**：
 *
 *   1. 后端 `handleDeleteActivity` 不带 since/until 时要求显式 `all=1`，
 *      否则 422——一次手滑的清除不该清空整个房间的历史。这里的策略是：
 *      有时间范围就按范围删，没有就在确认框里把「将清除全部历史」说明白，
 *      并在请求里带上 `all=1`。
 *   2. 后端删除只认 since/until，**不认 kind/eventType**——这两个过滤器
 *      只用来筛「查询」，删除接口没有这两个参数。如果用户开着类型筛选
 *      去清除，清掉的仍是这段时间范围内的全部日志，不只是筛选出来的那些。
 *      不说明这一点，用户会以为清除只删了他筛出来的类型，其实全删了。
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
  useDialog,
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
/**
 * useDialog() 要求外层有 NDialogProvider——App.vue 里已经套好了。
 *
 * **不要为迁就缺 provider 的测试加 `if (!dialog)` 之类的降级分支。**
 * 清除是真的删库，缺 provider 属于配置错误，应当响亮地抛异常而不是悄悄
 * 跳过二次确认；测试缺 provider 时应该给测试补 provider（或按 Accounts.vue
 * 的既定做法直接 mock `useDialog`），这是 Accounts.vue 已经踩过、写进
 * 注释里的教训。
 */
const dialog = useDialog()

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

// ---- 清除：真的删库，必须二次确认 ----

const clearingHistory = ref(false)
/** 最近一次清除的结果，常驻显示在按钮旁边——message 提示会自己消失，
 * 用户过一会儿回头看的话，仍然要能确认「删成功了、删了多少条」，
 * 而不是只能靠回忆刚才那条 toast 说了什么。 */
const lastClearResult = ref<number | null>(null)

/** 把当前时间范围翻成人话，用在确认框标题下面。 */
function clearScopeText(): string {
  if (!range.value) return '全部历史（未设置时间范围）'
  return `${new Date(range.value[0]).toLocaleString()} 至 ${new Date(range.value[1]).toLocaleString()}`
}

function confirmClearHistory() {
  const b = bindings.current
  if (!b) return

  const scope = clearScopeText()
  // 删除接口只认 since/until，不认 kind/eventType——用户开着类型筛选
  // 去清除的话，删掉的仍是这段时间范围内的全部日志，不只是筛选出来的
  // 那部分。不说清楚这一点，用户会以为清除只删了他看到的这些。
  const filterActive = kindFilter.value !== 'all' || !!eventTypeFilter.value
  const contentLines = [
    `确定要清除「${scope}」范围内的业务日志吗？这是真的从数据库删除，不可恢复。`,
  ]
  if (filterActive) {
    contentLines.push(
      '注意：清除只按时间范围执行，不支持按类型/关键词筛选——即使你已经用类型或关键词筛出了' +
        '一部分记录，清除的仍是这个时间范围内的全部日志，不局限于当前筛选结果。',
    )
  }

  dialog.warning({
    title: '清除业务日志',
    content: () =>
      h(
        'div',
        contentLines.map((line) => h('p', { style: 'margin: 4px 0' }, line)),
      ),
    positiveText: '清除',
    negativeText: '取消',
    onPositiveClick: () => void doClearHistory(b.id),
  })
}

async function doClearHistory(bindingId: number) {
  clearingHistory.value = true
  try {
    const params = new URLSearchParams()
    if (range.value) {
      params.set('since', new Date(range.value[0]).toISOString())
      params.set('until', new Date(range.value[1]).toISOString())
    } else {
      // 后端不带 since/until 时要求显式 all=1，否则 422——这是它的
      // 防手滑机制，前端配合带上，而不是绕过去。
      params.set('all', '1')
    }
    const res = await request<{ deleted: number }>(
      'DELETE',
      `/api/bindings/${bindingId}/activity?${params.toString()}`,
    )
    lastClearResult.value = res.deleted
    message.success(`已清除 ${res.deleted} 条业务日志`)
    await loadHistory()
  } catch (e) {
    message.error(e instanceof ApiError ? e.message : '清除失败')
  } finally {
    clearingHistory.value = false
  }
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
          <span v-if="lastClearResult !== null" class="clear-result">
            上次清除了 {{ lastClearResult }} 条
          </span>
          <NButton type="error" :loading="clearingHistory" @click="confirmClearHistory">
            清除
          </NButton>
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
.clear-result {
  font-size: 12px;
  opacity: 0.7;
}
</style>
