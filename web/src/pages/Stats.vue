<script setup lang="ts">
/**
 * Stats 是「统计」页（设计文档 §7.2 页面 6）：分账号、分直播间展示弹幕数、
 * 进房人数、礼物种类与数量、上舰数、直播时长、盲盒盈亏，按场次与按日两个
 * 维度。
 *
 * **这一页没有后端聚合接口，这是本页最重要的约束，不是次要细节。**
 *
 * 后端目前只有 `GET /api/bindings/{id}/activity`，单次最多返回 500 条
 * 原始行（`maxActivityLimit`，见 `server/internal/httpapi/activity_handler.go`）。
 * 一个活跃房间一天的行数远超 500。如果在前端把这 500 条当全量数字加总
 * 展示——「今天弹幕 500 条」——那是一个**错误的数字，而且不会告诉任何人
 * 它是错的**：用户会以为这就是真实总数。这比空着糟得多，空着至少诚实。
 *
 * 所以这一版的处理方式：
 *
 *   1. 图表与卡片的**结构**照设计文档全部渲染出来，让用户能靠界面评审
 *      「功能划分对不对」——这正是当前阶段（用户裁决：后端接口统一最后处理）
 *      要验证的东西。
 *   2. 卡片里的**数字**一律是占位符 `PLACEHOLDER`（`—`），不读取任何接口
 *      返回值去计算它。
 *   3. 页面顶部有醒目的说明条，指向悬空清单里登记的聚合接口缺口。
 *   4. 额外做了一个**可选**的「最近活动预览」区块，复用 `/activity`
 *      展示最近的原始事件行——但它被极其明确地标注为「采样，不是统计」，
 *      默认折叠、需要手动点开才加载，且物理上和上面的统计卡片区分开，
 *      不会被误认成统计数字。
 *
 * 盲盒盈亏那张卡片是**双重悬空**：既缺这里的聚合接口，也缺 `event.Gift`
 * 的盲盒字段（悬空清单第 7 条）。即使补上聚合接口，没有盲盒字段也算不出
 * 盈亏——两层缺口分别登记，见悬空清单第 14、15 条。
 */
import { computed, ref } from 'vue'
import {
  NAlert,
  NButton,
  NCard,
  NDataTable,
  NEmpty,
  NRadioButton,
  NRadioGroup,
  NStatistic,
  NTag,
} from 'naive-ui'
import type { DataTableColumns } from 'naive-ui'
import { ApiError, request } from '@/api'
import type { Activity } from '@/api'
import { useBindingsStore } from '@/stores/bindings'

const bindings = useBindingsStore()

/**
 * 数字位置的唯一合法取值。
 *
 * **绝不要用 `activityPreview.value.length` 或任何从 `/activity` 算出来的
 * 值替换它**——那正是这个文件存在的意义要防的事。真实数字必须来自后端
 * 聚合接口（悬空清单第 14 条），接口补上之前，这里只能是占位符。
 */
const PLACEHOLDER = '—'

// ---- 维度切换：按场次 / 按日 ----
//
// 目前没有聚合接口可供请求，切换维度不会改变卡片里的任何数字（因为数字
// 本来就是占位符）。控件仍然照常渲染并可以点——评审的是「维度这个概念
// 有没有在界面上出现」，不是「切换后数字会不会变」。

const DIMENSION_OPTIONS = [
  { label: '按日', value: 'day' as const },
  { label: '按场次', value: 'session' as const },
]
const dimension = ref<'day' | 'session'>('day')

// ---- 统计卡片：结构渲染，数字占位 ----

interface StatCardDef {
  key: string
  label: string
  hint: string
  /** 双重悬空的卡片（目前只有盲盒盈亏）额外挂一个标签，提醒缺口不止一层。 */
  doublyHanging?: boolean
}

const STAT_CARDS: StatCardDef[] = [
  { key: 'danmakuCount', label: '弹幕数', hint: '本维度内的弹幕总条数' },
  { key: 'enterCount', label: '进房人数', hint: '本维度内的进房人次' },
  { key: 'giftKinds', label: '礼物种类', hint: '本维度内出现过的不同礼物种类数' },
  { key: 'giftCount', label: '礼物数量', hint: '本维度内送出的礼物总件数' },
  { key: 'guardCount', label: '上舰数', hint: '本维度内新增/续费的大航海数量' },
  { key: 'liveDuration', label: '直播时长', hint: '本维度内的开播时长' },
  {
    key: 'blindBoxProfit',
    label: '盲盒盈亏',
    hint: '双重悬空：既缺聚合接口，也缺 Gift 事件的盲盒字段，见悬空清单第 14、15 条',
    doublyHanging: true,
  },
]

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
      <NAlert type="warning" title="统计需要后端聚合接口" class="hanging-alert">
        下面卡片里的数字目前都是占位符 <NTag size="small">{{ PLACEHOLDER }}</NTag
        >， 不是算出来的 0 或任何真实数字。后端只有 <code>GET /activity</code>，单次最多 返回 500
        条原始行，一个活跃房间一天的行数远超于此——在前端把这 500 条当全量
        加总展示会给出一个错误的数字而不告诉任何人，所以宁可空着。 补丁方向见悬空清单第 14、15
        条：需要一个按 <code>GROUP BY</code> 聚合的
        <code>GET /api/bindings/{id}/stats?by=day|session</code> 接口。
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
          维度切换目前不会改变任何数字——数字是占位符，与维度无关；这里渲染出来是为了让评审确认「按场次/按日」两个入口都在
        </span>
      </div>

      <div class="stats-grid">
        <NCard v-for="card in STAT_CARDS" :key="card.key" class="stat-card" size="small">
          <NStatistic :label="card.label" :value="PLACEHOLDER" />
          <NTag v-if="card.doublyHanging" type="warning" size="small" class="doubly-hanging-tag">
            双重悬空
          </NTag>
          <div class="stat-hint">{{ card.hint }}</div>
        </NCard>
      </div>

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
.doubly-hanging-tag {
  margin-top: 8px;
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
