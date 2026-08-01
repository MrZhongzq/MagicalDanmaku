<script setup lang="ts">
/**
 * Stats 是「统计」页（设计文档 §7.2 页面 6）：分账号、分直播间展示弹幕数、
 * 进房人数、礼物种类与数量、上舰数、直播时长、盲盒盈亏，按场次与按日两个
 * 维度。
 *
 * **P4-3 接上了真实聚合接口**：`GET /api/bindings/{id}/stats?by=day|session`
 * 由 SQL 侧 `GROUP BY` 聚合，不再是从 `/activity` 的 500 条截断样本里
 * 算出来的假数字（那正是这一页此前一直显示占位符的原因）。接口一次返回
 * 一组分桶（`by=day` 每天一桶，`by=session` 每场直播一桶），本页把它们
 * 汇总成 7 张总览卡片，并在下面附一张分桶明细表——明细表是「维度切换有
 * 真实效果」最直接的证据：按日/按场次切出来的行数、边界都不一样。
 *
 * **两个仍然要在界面上说清楚的限制，不能被真实数字的出现盖过去**：
 *
 *   1. **`liveSeconds` 有历史数据缺口**：`live_start`/`live_stop` 是这次
 *      才加进 `logging/sink.go` 的入库白名单的，在此之前的数据里没有这
 *      两类事件，更早的日子/场次时长永远算不出来、会显示 0——**不代表
 *      那天没有开播**，是数据缺口，不是「没直播」。页面上有醒目提示。
 *   2. **`giftKinds`（礼物种类）汇总卡片是「各分桶种类数之和」，不是
 *      全局去重后的真实种类数**：每个分桶内部是去重的，但跨分桶求和时，
 *      同一件礼物如果在不同的日子/场次都出现过，会被重复计入。这是求和
 *      算法本身的局限，不是接口返回错了——已经在卡片提示里写明，不能
 *      让用户误以为这是精确的全局种类数。
 *
 * 盲盒盈亏那张卡片仍然悬空：聚合接口本身已经补上了（悬空清单第 14 条
 * 已解决），但 `event.Gift` 依然没有盲盒字段（悬空清单第 7、15 条），
 * 没有原始数据可聚合，算不出盈亏，卡片继续显示占位符。
 *
 * 「最近活动预览」区块保留：它展示的是原始事件行（含用户、类型），
 * 统计卡片/明细表展示的是聚合数字，两者用途不同（“最近发生了什么”
 * vs “这段时间总共发生了多少”），不是同一份信息的两种画法，见文件
 * 末尾该区块自己的注释。
 */
import { computed, ref, watch } from 'vue'
import {
  NAlert,
  NButton,
  NCard,
  NDataTable,
  NEmpty,
  NRadioButton,
  NRadioGroup,
  NSpin,
  NStatistic,
  NTag,
} from 'naive-ui'
import type { DataTableColumns } from 'naive-ui'
import { ApiError, request } from '@/api'
import type { Activity, StatsBucket, StatsDimension } from '@/api'
import { useBindingsStore } from '@/stores/bindings'

const bindings = useBindingsStore()

/** 唯一合法的占位符——目前只有盲盒盈亏还在用它，其余卡片都是真实数字了。 */
const PLACEHOLDER = '—'

// ---- 维度切换：按场次 / 按日，现在真的会重新请求聚合接口 ----

const DIMENSION_OPTIONS = [
  { label: '按日', value: 'day' as const },
  { label: '按场次', value: 'session' as const },
]
const dimension = ref<StatsDimension>('day')
const dimensionLabel = computed(() => (dimension.value === 'day' ? '每日' : '每场'))

// ---- 聚合统计：GET /api/bindings/{id}/stats?by=day|session ----

const statsBuckets = ref<StatsBucket[]>([])
const loadingStats = ref(false)
const statsError = ref<string | null>(null)

async function loadStats() {
  const b = bindings.current
  if (!b) {
    statsBuckets.value = []
    return
  }
  loadingStats.value = true
  statsError.value = null
  try {
    statsBuckets.value = await request<StatsBucket[]>(
      'GET',
      `/api/bindings/${b.id}/stats?by=${dimension.value}`,
    )
  } catch (e) {
    statsError.value = e instanceof ApiError ? e.message : '加载统计数据失败'
    statsBuckets.value = []
  } finally {
    loadingStats.value = false
  }
}

// 绑定或维度任一变化都要重新拉取——这正是「维度切换现在有真实效果」的
// 落地位置：切换按日/按场次会发出带不同 by 参数的新请求。
watch(
  () => [bindings.currentId, dimension.value] as const,
  () => void loadStats(),
  { immediate: true },
)

/**
 * totals 把分桶数组汇总成总览卡片用的数字。
 *
 * danmakuCount/enterCount/giftCount/guardCount/liveSeconds 五项可以放心
 * 相加——它们是各分桶（互不重叠的时间窗口）内的计数，求和就是这段返回
 * 范围内的真实总数。**giftKinds 不在此列**：那是「种类数」而不是
 * 「件数」，各分桶内部去重，但跨分桶求和会把同一件礼物在不同分桶重复
 * 计入，不是真正的全局去重种类数——卡片提示里必须说明这一点，不能让
 * 用户当成精确值使用。
 */
const totals = computed(() => {
  const buckets = statsBuckets.value
  const sum = (pick: (b: StatsBucket) => number) => buckets.reduce((acc, b) => acc + pick(b), 0)
  return {
    danmakuCount: sum((b) => b.danmakuCount),
    enterCount: sum((b) => b.enterCount),
    giftCount: sum((b) => b.giftCount),
    giftKinds: sum((b) => b.giftKinds),
    guardCount: sum((b) => b.guardCount),
    liveSeconds: sum((b) => b.liveSeconds),
  }
})

/** 把秒数格式化成「N 小时 M 分钟」，0 秒单独处理成「0 分钟」而不是空字符串。 */
function formatDuration(totalSeconds: number): string {
  if (totalSeconds <= 0) return '0 分钟'
  const hours = Math.floor(totalSeconds / 3600)
  const minutes = Math.floor((totalSeconds % 3600) / 60)
  return hours > 0 ? `${hours} 小时 ${minutes} 分钟` : `${minutes} 分钟`
}

// ---- 统计卡片：真实数字，随 totals 变化 ----

interface StatCardDef {
  key: string
  label: string
  value: string
  hint: string
  /** 目前只有盲盒盈亏还悬空——聚合接口本身已经补上了（原来的「双重」只剩一重）。 */
  hanging?: boolean
}

/**
 * hasBuckets 为假时（接口返回空数组），六张"真实数字"卡片一律不显示
 * `totals` 算出来的 0，改走 PLACEHOLDER。
 *
 * **这正是这一页的设计初衷要防的东西，只是换了个触发条件**：`by=session`
 * 时，每一个现存用户在升级后、第一次开播被记录到之前，切到「按场次」
 * 都会拿到空数组——`live_start`/`live_stop` 是这次才进日志白名单的。
 * 空数组求和，六项全是 0，卡片会斩钉截铁地显示「弹幕数 0」「上舰数 0」，
 * 而这不是「这段时间真的一条弹幕都没有」，是「压根没能分出场次」，两者
 * 混在一起显示就是把「算不出来」读成了「确实是零」——与这一页此前用
 * 占位符防住的是同一类错误。
 */
const hasBuckets = computed(() => statsBuckets.value.length > 0)

/**
 * noBucketsHint 是没有分桶数据时卡片统一显示的说明，取代逐字段各自的
 * hint——这时候连"有没有数据"都不确定，继续显示字段各自的固有说明
 * （比如"本维度内的弹幕总条数"）容易让人以为占位符只是碰巧显示、
 * 数字本该是真实算出来的 0。
 *
 * `by=session` 单独给一句更具体的说明：这是目前唯一已知、会稳定触发
 * 空分桶的场景（原因见 hasBuckets 的注释），点名 live_start/live_stop
 * 比一句通用的"没有数据"更能让用户判断这是不是自己直播间的问题。
 */
const noBucketsHint = computed(() =>
  dimension.value === 'session'
    ? '这段时间没有记录到开播/下播事件，按场次维度无法分桶——' +
      'live_start/live_stop 是本次升级才开始记录的，现存的历史数据里没有这两类事件'
    : '这段时间没有可用的统计数据',
)

const STAT_CARDS = computed<StatCardDef[]>(() => [
  {
    key: 'danmakuCount',
    label: '弹幕数',
    value: hasBuckets.value ? String(totals.value.danmakuCount) : PLACEHOLDER,
    hint: hasBuckets.value ? '本维度内的弹幕总条数' : noBucketsHint.value,
  },
  {
    key: 'enterCount',
    label: '进房人数',
    value: hasBuckets.value ? String(totals.value.enterCount) : PLACEHOLDER,
    hint: hasBuckets.value ? '本维度内的进房人次' : noBucketsHint.value,
  },
  {
    key: 'giftKinds',
    label: '礼物种类',
    value: hasBuckets.value ? String(totals.value.giftKinds) : PLACEHOLDER,
    hint: hasBuckets.value
      ? `各${dimensionLabel.value}种类数之和：同一件礼物如果在多个${dimensionLabel.value}都出现过，会被重复计入，不是这段时间内去重后的真实种类数`
      : noBucketsHint.value,
  },
  {
    key: 'giftCount',
    label: '礼物数量',
    value: hasBuckets.value ? String(totals.value.giftCount) : PLACEHOLDER,
    hint: hasBuckets.value ? '本维度内送出的礼物总件数' : noBucketsHint.value,
  },
  {
    key: 'guardCount',
    label: '上舰数',
    value: hasBuckets.value ? String(totals.value.guardCount) : PLACEHOLDER,
    hint: hasBuckets.value ? '本维度内新增/续费的大航海数量' : noBucketsHint.value,
  },
  {
    key: 'liveDuration',
    label: '直播时长',
    value: hasBuckets.value ? formatDuration(totals.value.liveSeconds) : PLACEHOLDER,
    hint: hasBuckets.value
      ? 'live_start/live_stop 是这次才加入日志记录范围的，更早的历史数据没有这两类事件，那些日子/场次会显示 0，不代表当时没有开播'
      : noBucketsHint.value,
  },
  {
    key: 'blindBoxProfit',
    label: '盲盒盈亏',
    value: PLACEHOLDER,
    hint: '聚合接口已经补上了，但 event.Gift 依然没有盲盒字段，算不出盈亏，见悬空清单第 7、15 条',
    hanging: true,
  },
])

// ---- 分桶明细表：维度切换真实效果的直接证据 ----
//
// 用 computed 而不是常量：第一列表头要随维度在「日期」/「场次」之间切换，
// 写成常量的话只会在组件初始化那一刻求值一次，切维度后表头不会跟着变。
const bucketColumns = computed<DataTableColumns<StatsBucket>>(() => [
  { title: dimension.value === 'day' ? '日期' : '场次', key: 'bucket' },
  { title: '弹幕数', key: 'danmakuCount' },
  { title: '进房人数', key: 'enterCount' },
  { title: '礼物种类', key: 'giftKinds' },
  { title: '礼物数量', key: 'giftCount' },
  { title: '上舰数', key: 'guardCount' },
  {
    title: '直播时长',
    key: 'liveSeconds',
    render: (row) => formatDuration(row.liveSeconds),
  },
])

// ---- 可选的「最近活动预览」：明确标注为采样，不是统计 ----
//
// 默认折叠、手动点开才请求，避免用户还没意识到这是采样就先看到一堆数字。

const previewVisible = ref(false)
const previewLoaded = ref(false)
const previewLoading = ref(false)
const previewError = ref<string | null>(null)
const previewRows = ref<Activity[]>([])

async function loadPreview() {
  const b = bindings.current
  if (!b) return
  previewLoading.value = true
  previewError.value = null
  try {
    previewRows.value = await request<Activity[]>('GET', `/api/bindings/${b.id}/activity`)
    previewLoaded.value = true
  } catch (e) {
    previewError.value = e instanceof ApiError ? e.message : '加载最近活动预览失败'
  } finally {
    previewLoading.value = false
  }
}

async function togglePreview() {
  previewVisible.value = !previewVisible.value
  if (previewVisible.value && !previewLoaded.value) {
    await loadPreview()
  }
}

// 换房间要把预览清空并允许重新加载。
//
// 不清的话：在甲房间展开预览后切到乙房间，previewRows 还是甲房间的行，
// 而 previewLoaded 为真会让 togglePreview 永远不再请求——收起再展开也
// 不刷新。表头写着「最近活动预览」，用户没有任何线索知道这是别的房间的数据。
watch(
  () => bindings.currentId,
  () => {
    previewLoaded.value = false
    previewRows.value = []
    if (previewVisible.value) void loadPreview()
  },
)

interface PreviewRow {
  key: string
  time: string
  type: string
  user: string
}

function toPreviewRow(a: Activity): PreviewRow {
  return {
    key: String(a.id),
    time: a.occurredAt,
    type: a.kind === 'action' ? a.actionType : a.eventType,
    user: a.userName || a.userUid || '-',
  }
}

const previewTableRows = computed<PreviewRow[]>(() => previewRows.value.map(toPreviewRow))

const previewColumns: DataTableColumns<PreviewRow> = [
  { title: '时间', key: 'time', width: 190 },
  { title: '类型', key: 'type', width: 160 },
  { title: '用户', key: 'user', width: 160 },
]
</script>

<template>
  <div class="stats-page">
    <h2>统计</h2>

    <NEmpty v-if="!bindings.current" description="请先在顶部选择一个直播间" />

    <template v-else>
      <NAlert type="info" title="两点需要注意" class="hanging-alert">
        下面的数字来自后端聚合接口（<code>GET /api/bindings/{id}/stats</code>），是真实统计值，
        不再是占位符——除了「盲盒盈亏」，那张卡片仍标着<NTag type="warning" size="small"
          >待后端支持</NTag
        >。另外两点务必留意：①「直播时长」在这批改动之前的历史数据里没有开播/下播事件，
        更早的日子会显示 0，<strong>不代表当时没开播</strong>；②「礼物种类」是各分桶种类数
        之和，同一件礼物跨多个日子/场次出现会被重复计入，不是全局去重后的精确值。下面每张卡片
        自己也带着一行小字说明，不必悬停就能看到。
      </NAlert>

      <div class="dimension-row">
        <span class="dimension-label">统计维度</span>
        <NRadioGroup v-model:value="dimension">
          <NRadioButton
            v-for="opt in DIMENSION_OPTIONS"
            :key="opt.value"
            :value="opt.value"
            :label="opt.label"
          />
        </NRadioGroup>
        <span class="dimension-hint">
          切换维度会重新向后端请求聚合数据（<code>by={{ dimension }}</code
          >）， 上面的卡片与下面的明细表都会跟着变
        </span>
      </div>

      <NAlert
        v-if="!loadingStats && !statsError && dimension === 'session' && !hasBuckets"
        type="warning"
        title="按场次维度暂时无法分桶"
        class="session-empty-alert"
      >
        这段时间没有记录到开播/下播事件，按场次维度无法分桶——<code>live_start</code>/<code
          >live_stop</code
        >
        是本次升级才开始记录的，现存的历史数据里没有这两类事件。下面的卡片会显示为「{{
          PLACEHOLDER
        }}」而不是 0：0 会被误读成「这段时间确实没有任何弹幕/礼物/上舰」，但实际情况是
        「压根分不出场次」，切回「按日」或等下一次开播后再看会更准确。
      </NAlert>

      <NSpin :show="loadingStats">
        <p v-if="statsError" class="stats-error">{{ statsError }}</p>

        <div class="stats-grid">
          <NCard v-for="card in STAT_CARDS" :key="card.key" class="stat-card" size="small">
            <NStatistic :label="card.label" :value="card.value" />
            <NTag v-if="card.hanging" type="warning" size="small" class="hanging-tag">
              待后端支持
            </NTag>
            <div class="stat-hint">{{ card.hint }}</div>
          </NCard>
        </div>

        <NCard title="分桶明细" class="bucket-card" size="small">
          <NDataTable
            :columns="bucketColumns"
            :data="statsBuckets"
            :row-key="(row: StatsBucket) => row.bucket"
            :bordered="false"
            size="small"
          />
          <NEmpty v-if="statsBuckets.length === 0" description="没有数据" size="small" />
        </NCard>
      </NSpin>

      <NCard title="最近活动预览（可选，辅助功能）" class="preview-card" size="small">
        <template #header-extra>
          <NButton size="small" @click="togglePreview">
            {{ previewVisible ? '收起' : '展开' }}
          </NButton>
        </template>

        <NAlert type="warning" title="这不是统计数字" class="preview-alert">
          以下内容取自最近 <strong>≤500 条原始事件记录内的采样</strong>，仅用于快速核对
          「最近发生了什么」，<strong>不代表任何完整时间范围（当日/当场）的真实统计数字</strong>。
          它和上面的统计卡片是两回事，请勿混淆。
        </NAlert>

        <template v-if="previewVisible">
          <p v-if="previewError" class="preview-error">{{ previewError }}</p>
          <NDataTable
            v-else
            :columns="previewColumns"
            :data="previewTableRows"
            :loading="previewLoading"
            :row-key="(row: PreviewRow) => row.key"
            :bordered="false"
            size="small"
          />
          <p class="preview-count">
            本次采样加载了 {{ previewTableRows.length }} 条原始行（受 500 条上限约束，
            不是「当前只有这么多」的意思）
          </p>
        </template>
      </NCard>
    </template>
  </div>
</template>

<style scoped>
.hanging-alert {
  margin-bottom: 16px;
}
.session-empty-alert {
  margin-bottom: 16px;
}
.dimension-row {
  display: flex;
  align-items: center;
  gap: 12px;
  margin-bottom: 16px;
  flex-wrap: wrap;
}
.dimension-label {
  font-weight: 600;
}
.dimension-hint {
  font-size: 12px;
  opacity: 0.7;
}
.stats-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(220px, 1fr));
  gap: 12px;
  margin-bottom: 20px;
}
.stat-hint {
  margin-top: 8px;
  font-size: 12px;
  opacity: 0.7;
}
.hanging-tag {
  margin-top: 8px;
}
.stats-error {
  color: var(--n-error-color, #d03050);
  margin-bottom: 12px;
}
.bucket-card {
  margin-bottom: 16px;
}
.preview-card {
  margin-bottom: 16px;
}
.preview-alert {
  margin-bottom: 12px;
}
.preview-count {
  margin-top: 8px;
  font-size: 12px;
  opacity: 0.7;
}
.preview-error {
  color: var(--n-error-color, #d03050);
}
</style>
