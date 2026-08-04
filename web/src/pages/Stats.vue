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
 * 汇总成 7 张总览卡片。**P7 起这些卡片改为跟随页面上的日期选择器**，
 * 不再是不设时间范围的全历史求和——原来靠一张「分桶明细」表才能看出的
 * 「切维度/切日期确实有效果」，现在切一下日期或维度、看卡片数字跟着变
 * 就是同一件事，明细表已经去掉，见下方 P7 段落。
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
 * **P4-4 Task 7：盲盒盈亏卡片已接上真实数据，不再悬空**（悬空清单第
 * 7/15 条）。`event.Gift.BlindBox` 早在 Task 1 就补上了，`store.stats.go`
 * 的 `countExprs` 新增 `blind_box_profit`：对 `detail->>'BlindBox'`
 * 非 null 的 gift 行按 `Price*Count - TotalCoin`（单位 1/100 电池）逐行
 * 求和，不按礼物名分组——同一电池消耗量在不同盲盒池可能对应不同的
 * 开出礼物，礼物名不是稳定的价值锚点，用户明确要求过必须按电池数量
 * 统计。「元」的换算只在这里（展示层）做。
 *
 * **`->>`（取文本）不是 `->`（取 jsonb）**——终审曾经在这里抓到一个真实
 * 存在的 bug：`->` 对「JSON 值是 null」返回的是 jsonb 的 null（一个非
 * SQL NULL 的值），`IS NOT NULL` 对这种情况判真，导致全部普通礼物都被
 * 误判成盲盒。这条注释本身就是当时的修复点——如果哪天又被改回 `->`，
 * 大概率是有人看着这段旧注释「顺手改回去」，所以这里特意把正确写法
 * 钉死，不要改回 `->`。
 *
 * 「最近活动预览」区块保留：它展示的是原始事件行（含用户、类型），
 * 统计卡片/明细表展示的是聚合数字，两者用途不同（“最近发生了什么”
 * vs “这段时间总共发生了多少”），不是同一份信息的两种画法，见文件
 * 末尾该区块自己的注释。
 *
 * **P6 任务 5：「今日电池到账」卡片 + 「礼物」明细列表**——用户真机反馈：
 * 「礼物数量」卡片后面要能看到今天到账了多少电池，下方要有按礼物分组
 * 的明细（礼物名/数量/电池数）。
 *
 * 电池数按"电池价值是不是 0"计算（price×count，不是 total_coin），
 * 不是"是不是金瓜子礼物"——`giftCoins`/`coins` 字段后端已经按这条判据
 * 算好（排除法：只排除银瓜子，见 `web/src/api/types.ts` 里
 * `StatsBucket.giftCoins` 的注释），前端不需要也不能再猜。「当日电池
 * 到账」含盲盒爆出的礼物（主播的真实收入不该被排除），但下方「礼物」
 * 明细列表不含盲盒（P4-4 硬性要求，盲盒不是稳定的价值锚点），两个数字
 * 因此不必相等，不是 bug。展示单位是「电池」而不是「元」，换算系数是
 * **除以 100**（原始值是 1/100 电池），不要跟盲盒盈亏卡片的 /1000
 * （换算成「元」）搞混，那是两个不同的展示单位。
 *
 * **P7：真机反馈第二轮——改用日期选择器，去掉分桶明细表**。用户看过
 * P6 之后说：既然礼物明细已经能看了，日期就该能自己选（`n-date-picker`
 * 日历控件，不是 `n-select` 下拉——下拉会随日期变多越拖越长），选中哪天
 * 就看哪天的卡片与礼物明细，默认选中今天。
 *
 * **全部 8 张卡片现在都跟随选中日期**，不只是原来 P6 单独拎出来的
 * 「今日电池到账」+「礼物」明细——这正是「分桶明细」表能被去掉的原因：
 * 以前明细表是唯一能直观看到「换一天/换一场次数字会不同」的地方（另外
 * 7 张总览卡片此前是不限时间范围的全历史求和）；现在挑单个日期直接看
 * 总览卡片就是同一件事，不必再多维护一张表格。
 *
 * **`dimension`（按日/按场次）现在的含义**：在选中的这一天范围内，是把
 * 它整体当一天聚合（`by=day`，通常聚出 0 或 1 个分桶），还是按这一天里
 * 实际发生的开播场次分别聚合（`by=session`，一天可能有 0 场、1 场或多
 * 场直播，各场结果再求和）。**「按场次」的含义是「看选中这天的各场
 * 直播」，不是「看全部历史场次」**——这是两种维度里唯一说得通的组合：
 * 「按日」选哪天看哪天很直白；「按场次」如果脱离选中日期去看全部历史
 * 场次，日期选择器对这个维度就形同虚设，用户选了日期却看不出任何影响，
 * 是一个用户看不懂的搭配。
 *
 * **时区：选中日期按浏览器本地自然天解释，不是 UTC 自然天**——这是重新
 * 权衡过 P6 遗留决定后的结果，务必别改回去。P6 的 `todayRangeUTC()`
 * 故意选 UTC，理由是要跟 `store.QueryStatsByDay` 的
 * `date_trunc('day', occurred_at)`（数据库会话时区，这套环境下是 UTC）
 * 对齐，避免前后端两个「今天」不是同一个时间窗口——但那条理由只在
 * 「界面上从不出现具体日期，只说抽象的『今天』」这个前提下成立。现在
 * 用户会在日历控件上点一个具体日期（比如「8 月 4 日」），控件画的就是
 * 浏览器本地时区的日历格子，用户脑子里的「8 月 4 日」也是本地时钟——
 * 这台服务器实际部署在东八区（`TZ=Asia/Shanghai`），直播时间表、弹幕
 * 答谢规则都按本地时间安排，如果这里悄悄按 UTC 解释，选中的「8 月 4 日」
 * 会变成本地时间 8 月 4 日早上 8 点到 8 月 5 日早上 8 点：用户选的那天
 * 前 8 小时的数据凭空消失，下一天前 8 小时的数据却混了进来。这种错不会
 * 报错、只会让数字对不上，且界面上没有任何线索能让用户往时区上想。
 *
 * 改成本地自然天不会破坏跟后端的对齐：`since`/`until` 只是传给后端的两
 * 个时间点（闭区间过滤），后端拿到之后仍然按它自己的 UTC 自然天分桶、
 * GROUP BY 求和——分桶的内部边界画在哪儿不影响"把返回的所有桶加总"这
 * 个用法，本页现在也不再展示单个分桶的标签（分桶明细表已去掉），只用
 * 总和，所以查询窗口没有必要跟后端内部的分桶边界对齐，只需要覆盖用户
 * 真正想看的那 24 小时本地时间。
 */
import { computed, ref, watch } from 'vue'
import {
  NAlert,
  NButton,
  NCard,
  NDataTable,
  NDatePicker,
  NEmpty,
  NRadioButton,
  NRadioGroup,
  NSpin,
  NStatistic,
} from 'naive-ui'
import type { DataTableColumns } from 'naive-ui'
import { ApiError, request } from '@/api'
import type { Activity, GiftBreakdownRow, StatsBucket, StatsDimension } from '@/api'
import { useBindingsStore } from '@/stores/bindings'
import BindingSelector from '@/components/BindingSelector.vue'

const bindings = useBindingsStore()

/** 唯一合法的占位符——只在 hasBuckets 为假（这段时间聚合不出任何分桶）时使用。 */
const PLACEHOLDER = '—'

/**
 * formatBlindBoxProfit 把 1/100 电池的原始整数换算成「元」展示。
 *
 * **除数是 1000，不是 100**——1 电池 = 0.1 元，原始值是 1/100 电池，
 * 所以「元 = 原始值 / 1000」。跟 `server/internal/rules/aggregate.go`
 * 的 `formatYuan`（弹幕答谢模板里 `{{.blindBox.profitYuan}}` 用的就是
 * 它）必须是同一个换算系数——这里曾经错写成 `/100`，同一笔盲盒在弹幕
 * 答谢（0.2 元）与统计页（当时错误地显示 2.00 元）之间差了 10 倍，
 * 已修正并在这条注释里把系数钉死，防止第三次漂移。
 *
 * 正数前缀 `+` 号强调「赚了」，负数 `toFixed` 自带负号不需要额外处理，
 * 0 既不加号也不减号（不赔不赚）。
 */
function formatBlindBoxProfit(profitCentiBattery: number): string {
  const yuan = profitCentiBattery / 1000
  const sign = yuan > 0 ? '+' : ''
  return `${sign}${yuan.toFixed(2)} 元`
}

/**
 * formatBattery 把 1/100 电池的原始整数换算成「电池」展示。
 *
 * **除数是 100，不是 1000**——这里展示的单位是「电池」，不是
 * formatBlindBoxProfit 那样的「元」（1 电池 = 0.1 元，元的换算要多除
 * 一次 10）。两个函数换算系数不同是刻意的，不是疏漏，别把它们对齐。
 */
function formatBattery(centiBattery: number): string {
  return `${(centiBattery / 100).toFixed(2)} 电池`
}

// ---- 维度切换：按场次 / 按日，现在真的会重新请求聚合接口 ----

const DIMENSION_OPTIONS = [
  { label: '按日', value: 'day' as const },
  { label: '按场次', value: 'session' as const },
]
const dimension = ref<StatsDimension>('day')
const dimensionLabel = computed(() => (dimension.value === 'day' ? '每日' : '每场'))

// ---- 日期选择器：统计维度那一行新增，替代原来的固定"今天" ----

/**
 * startOfLocalDay 把任意时刻对齐到"当地自然天"的零点。
 *
 * 用 `getFullYear`/`getMonth`/`getDate`（本地时区取值）而不是
 * `getUTCFullYear` 等 UTC 版本——这是本次改动最容易出错的地方，见文件
 * 头部"时区"那段说明：日期选择器画的是浏览器本地时区的日历格子，这里
 * 必须用同一套时区口径，否则选中的日期文本与实际请求的时间窗口会对不上
 * （典型症状：数字看起来"差了大半天"，但界面上完全看不出哪里错了）。
 */
function startOfLocalDay(d: Date): number {
  return new Date(d.getFullYear(), d.getMonth(), d.getDate()).getTime()
}

/**
 * dayRangeFor 把"选中日期的本地零点"换算成传给后端的 `[since, until]`
 * 闭区间（后端 since/until 的比较都用 `>=`/`<=`，见
 * `parseActivityTimeRange`，两端都含）。
 *
 * **终点用"次日零点减 1 毫秒"，不是"当前时刻 `now`"**——P6 的
 * `todayRangeUTC` 只处理"今天"，用 `now` 当终点也够用（反正数据库里
 * 不会有超过 `now` 的行）。但日期选择器允许挑任意历史日期，如果选了
 * 昨天还用 `now` 当终点，查询窗口会一路开到现在，把今天的数据也算进
 * "昨天"里——必须显式算出所选那一天的最后一刻，不能偷懒复用 `now`。
 *
 * 用本地日历"加一天再减 1 毫秒"而不是直接加 `24*60*60*1000` 毫秒，是
 * 为了在有夏令时的时区也不出错（一天可能是 23 或 25 小时）——中国不
 * 实行夏令时，这里差别不大，但这样写不依赖这个前提，换了部署时区也
 * 不用重新审视这段逻辑。
 */
function dayRangeFor(dayStartMs: number): { since: string; until: string } {
  const start = new Date(dayStartMs)
  const nextDayStart = new Date(start.getFullYear(), start.getMonth(), start.getDate() + 1)
  const end = new Date(nextDayStart.getTime() - 1)
  return { since: start.toISOString(), until: end.toISOString() }
}

/** 统计维度那一行的日期选择器，默认选中今天（本地自然天）。 */
const selectedDate = ref<number>(startOfLocalDay(new Date()))

/** 选中日期的 `YYYY-MM-DD` 文本，供卡片提示里说明"现在具体在看哪一天"。 */
const selectedDateLabel = computed(() => {
  const d = new Date(selectedDate.value)
  const y = d.getFullYear()
  const m = String(d.getMonth() + 1).padStart(2, '0')
  const day = String(d.getDate()).padStart(2, '0')
  return `${y}-${m}-${day}`
})

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
    const { since, until } = dayRangeFor(selectedDate.value)
    const qs = `since=${encodeURIComponent(since)}&until=${encodeURIComponent(until)}`
    statsBuckets.value = await request<StatsBucket[]>(
      'GET',
      `/api/bindings/${b.id}/stats?by=${dimension.value}&${qs}`,
    )
  } catch (e) {
    statsError.value = e instanceof ApiError ? e.message : '加载统计数据失败'
    statsBuckets.value = []
  } finally {
    loadingStats.value = false
  }
}

// 绑定、维度、选中日期任一变化都要重新拉取——这正是「切维度/切日期都有
// 真实效果」的落地位置：三者任一变化都会发出带新 by/since/until 的请求。
watch(
  () => [bindings.currentId, dimension.value, selectedDate.value] as const,
  () => void loadStats(),
  { immediate: true },
)

// ---- 当日卡片 + 礼物明细列表：固定用 by=day，不随维度切换变化 ----
//
// 这两块跟主统计卡片一样跟随"选中日期"变化，但请求方式仍然保留 P6 定下
// 的另一条边界：固定用 by=day，不随 dimension 切到 by=session——"这一天
// 总共到账多少电池""这一天送了哪些礼物"是两个自然按天理解的概念，不该
// 因为用户切到「按场次」维度就变成"这一天里每一场分别算一次"，那样反而
// 会让人分不清这张卡片是不是也在跟着维度切换变化（它不该变）。

const dayGiftCoins = ref<number | null>(null) // null 表示"算不出来"（未选绑定/加载失败），不能显示成 0
const giftBreakdown = ref<GiftBreakdownRow[]>([])
const loadingDayExtras = ref(false)
const dayExtrasError = ref<string | null>(null)

async function loadDayExtras() {
  const b = bindings.current
  if (!b) {
    dayGiftCoins.value = null
    giftBreakdown.value = []
    return
  }
  loadingDayExtras.value = true
  dayExtrasError.value = null
  try {
    const { since, until } = dayRangeFor(selectedDate.value)
    const qs = `since=${encodeURIComponent(since)}&until=${encodeURIComponent(until)}`
    // 两个接口各自独立失败也不该互相拖累——用 allSettled 而不是 all，
    // 电池到账查不到不该连带把已经查到的礼物明细也清空，反之亦然。
    const [statsResult, giftsResult] = await Promise.allSettled([
      request<StatsBucket[]>('GET', `/api/bindings/${b.id}/stats?by=day&${qs}`),
      request<GiftBreakdownRow[]>('GET', `/api/bindings/${b.id}/gifts?${qs}`),
    ])

    if (statsResult.status === 'fulfilled') {
      // 空数组不能求和成 0——那会把"选中日期压根没有可用的分桶数据"显示
      // 成"这一天电池到账确实是 0"，与本页其余卡片的 hasBuckets/
      // PLACEHOLDER 处理原则一致（见 noBucketsHint 的注释）。
      //
      // by=day&since=选中日期零点&until=次日零点前一毫秒 至多只会返回
      // 一个分桶，但仍然用 reduce 求和而不是直接取第一个——万一后端跨天
      // 分桶的简化规则（见 QueryStatsByDay 的说明）在某个边界返回了不止
      // 一桶，求和永远是安全的，取第一个则可能悄悄漏掉一部分。
      dayGiftCoins.value =
        statsResult.value.length > 0
          ? statsResult.value.reduce((sum, bkt) => sum + bkt.giftCoins, 0)
          : null
    } else {
      dayGiftCoins.value = null
      dayExtrasError.value =
        statsResult.reason instanceof ApiError ? statsResult.reason.message : '加载当日电池到账失败'
    }

    if (giftsResult.status === 'fulfilled') {
      giftBreakdown.value = giftsResult.value
    } else {
      giftBreakdown.value = []
      dayExtrasError.value =
        giftsResult.reason instanceof ApiError ? giftsResult.reason.message : '加载礼物明细失败'
    }
  } finally {
    loadingDayExtras.value = false
  }
}

// 随绑定或选中日期变化重新拉取，不随 dimension——理由见上面这一节的
// 说明（固定按天理解的概念，不该因为切维度就变成别的意思或者消失）。
watch(
  () => [bindings.currentId, selectedDate.value] as const,
  () => void loadDayExtras(),
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
    // blindBoxProfit 同样可以放心相加：跟 danmakuCount 等五项一样，是
    // 各分桶（互不重叠的时间窗口）内已经算好的盈亏之和，不存在 giftKinds
    // 那种「去重后跨分桶重复计入」的问题——盈亏是金额加总，不是去重计数。
    blindBoxProfit: sum((b) => b.blindBoxProfit),
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
      ? `各${dimensionLabel.value}种类数之和：同一件礼物如果在多个${dimensionLabel.value}都出现过，会被重复计入，不是这段时间内去重后的真实种类数；不含盲盒（盲盒爆出的礼物名不计入种类），盲盒单独看下面「盲盒盈亏」卡片`
      : noBucketsHint.value,
  },
  {
    key: 'giftCount',
    label: '礼物数量',
    value: hasBuckets.value ? String(totals.value.giftCount) : PLACEHOLDER,
    hint: hasBuckets.value
      ? '本维度内送出的礼物总件数，不含盲盒——盲盒单独看下面「盲盒盈亏」卡片'
      : noBucketsHint.value,
  },
  {
    key: 'dayGiftCoins',
    label: '当日电池到账',
    // 跟随选中日期（本地自然天），固定用 by=day 请求，不随上面的维度
    // 切换变化——见文件头 P7 段落的说明。null 表示还没选绑定或加载
    // 失败，不能显示成 0：那会被误读成"这一天真的一分电池都没到账"。
    value: dayGiftCoins.value !== null ? formatBattery(dayGiftCoins.value) : PLACEHOLDER,
    hint:
      dayGiftCoins.value !== null
        ? `${selectedDateLabel.value}（本地时间）收到的电池总量，含盲盒爆出的礼物——只统计真正产生了电池的礼物，不产生电池的免费礼物（如小花花、人气票）不计入这里`
        : (dayExtrasError.value ?? `${selectedDateLabel.value} 暂无可用数据`),
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
    value: hasBuckets.value ? formatBlindBoxProfit(totals.value.blindBoxProfit) : PLACEHOLDER,
    hint: hasBuckets.value
      ? '本维度内全部盲盒送礼的盈亏之和（爆出礼物价值 − 实际花费），按每次消耗的电池数量原始值累加，不按礼物名分组'
      : noBucketsHint.value,
  },
])

// ---- 礼物明细列表：按礼物名分组，跟随选中日期，固定用 by=day ----
const giftBreakdownColumns: DataTableColumns<GiftBreakdownRow> = [
  { title: '礼物名', key: 'giftName' },
  { title: '数量', key: 'count' },
  {
    title: '电池数',
    key: 'coins',
    render: (row) => formatBattery(row.coins),
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
    <div class="page-header">
      <h2>统计</h2>
      <!-- 统计页不写规则也不动禁言名单，本页没有专门的权限点校验，
           选择器不带 requiredPerm——所有能看到这个绑定的成员理应都能看统计。 -->
      <BindingSelector />
    </div>

    <NEmpty v-if="!bindings.current" description="请先在顶部选择一个直播间" />

    <template v-else>
      <NAlert type="info" title="两点需要注意" class="hanging-alert">
        下面的数字来自后端聚合接口（<code>GET /api/bindings/{id}/stats</code>），是真实统计值，
        不再是占位符——包括「盲盒盈亏」也是真实数字了。另外两点务必留意：①「直播时长」在这批
        改动之前的历史数据里没有开播/下播事件，更早的日子会显示 0，<strong>不代表当时没开播</strong
        >；②「礼物种类」是各分桶种类数之和，同一件礼物如果在选中日期内的多场直播都出现过（「按
        场次」维度），会被重复计入，不是全局去重后的精确值。下面每张卡片自己也带着一行小字说明，
        不必悬停就能看到。
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
        <!-- 日期选择器：真机反馈明确要求用日历控件而不是下拉菜单——日期
             一多下拉会拖得很长。不可清空：选中日期是本页全部卡片/明细
             共同依赖的查询条件，允许清空会让"清空之后看什么"变成一个
             没有答案的状态，不如干脆不允许出现。 -->
        <NDatePicker v-model:value="selectedDate" type="date" :clearable="false" />
        <span class="dimension-hint">
          切换维度或换一个日期都会重新向后端请求聚合数据（<code>by={{ dimension }}</code
          >），上面的卡片会跟着变
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
            <div class="stat-hint">{{ card.hint }}</div>
          </NCard>
        </div>
      </NSpin>

      <!-- 礼物明细列表：跟随选中日期，固定用 by=day 请求（不随维度
           切换），见脚本头部 P7 段落的说明。 -->
      <NSpin :show="loadingDayExtras">
        <p v-if="dayExtrasError" class="stats-error">{{ dayExtrasError }}</p>
        <NCard title="礼物" class="gift-breakdown-card" size="small">
          <NDataTable
            :columns="giftBreakdownColumns"
            :data="giftBreakdown"
            :row-key="(row: GiftBreakdownRow) => row.giftName"
            :bordered="false"
            size="small"
          />
          <NEmpty v-if="giftBreakdown.length === 0" description="没有数据" size="small" />
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
.page-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  flex-wrap: wrap;
  gap: 12px;
  margin-bottom: 16px;
}
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
.stats-error {
  color: var(--n-error-color, #d03050);
  margin-bottom: 12px;
}
.gift-breakdown-card {
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
