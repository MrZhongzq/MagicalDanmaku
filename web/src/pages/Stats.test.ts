import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import { NDatePicker, NRadioGroup, NStatistic } from 'naive-ui'

// 统计页现在按"浏览器本地自然天"解释日期选择器（见 Stats.vue 文件头
// "时区"那段说明），把测试进程的时区钉死成生产部署时区（东八区）——不这样
// 做的话，测试在不同开发机/CI 上跑，`new Date()` 的本地日历日会因为宿主
// 时区不同而算出不同的 since/until，同一份测试代码在两台机器上一个绿一个
// 红，而且不是本页逻辑的问题。必须在下面动态 import 之前、任何 Date
// 计算发生之前设置。
process.env.TZ = 'Asia/Shanghai'

const { default: Stats } = await import('./Stats.vue')
const { useBindingsStore } = await import('@/stores/bindings')

const 绑定: {
  id: number
  accountId: number
  accountName: string
  roomId: string
  enabled: boolean
  ruleCount: number
  permissions: (
    'rule:read' | 'rule:write' | 'danmaku:send' | 'user:block' | 'member:manage' | 'event:read'
  )[]
  isOwner: boolean
  liveStatus: 'living' | 'offline' | 'unknown'
  liveCheckedAt: string | null
  anchorUid: string
  anchorName: string
} = {
  id: 1,
  accountId: 1,
  accountName: '小号',
  roomId: '123',
  enabled: true,
  ruleCount: 0,
  permissions: ['event:read'],
  isOwner: true,
  liveStatus: 'unknown',
  liveCheckedAt: null,
  anchorUid: '',
  anchorName: '',
}

function ok(body: unknown, status = 200) {
  return new Response(JSON.stringify(body), {
    status,
    headers: { 'Content-Type': 'application/json' },
  })
}

function err(status: number, message: string) {
  return new Response(JSON.stringify({ error: message }), {
    status,
    headers: { 'Content-Type': 'application/json' },
  })
}

/** 选好当前绑定的 pinia store。 */
function setupStore() {
  setActivePinia(createPinia())
  const bindings = useBindingsStore()
  bindings.list = [绑定]
  bindings.select(1)
  return { bindings }
}

/** 构造 500 条 activity 原始行——模拟后端 maxActivityLimit 撑满的情形。 */
function make500Activity() {
  return Array.from({ length: 500 }, (_, i) => ({
    id: i + 1,
    kind: 'event' as const,
    eventType: 'danmaku',
    actionType: '',
    ruleName: '',
    userUid: String(1000 + i),
    userName: `观众${i}`,
    detail: { text: `弹幕 ${i}` },
    occurredAt: '2026-08-01T12:00:00Z',
  }))
}

/** 一条 StatsBucket 的默认值，测试按需覆盖字段。 */
function statsBucket(overrides: Partial<Record<string, unknown>> = {}) {
  return {
    bucket: '2026-08-01',
    danmakuCount: 0,
    enterCount: 0,
    giftCount: 0,
    giftKinds: 0,
    guardCount: 0,
    liveSeconds: 0,
    blindBoxProfit: 0,
    giftCoins: 0,
    ...overrides,
  }
}

/** 一行 GiftBreakdownRow 的默认值，测试按需覆盖字段。 */
function giftRow(overrides: Partial<Record<string, unknown>> = {}) {
  return {
    giftName: '辣条',
    count: 1,
    coins: 0,
    ...overrides,
  }
}

/**
 * `Response.body` 只能读一次——每次调用都要返回一个新的 Response 实例，
 * 不能复用同一个对象（否则第二次 fetch 读到的是已经耗尽的流）。
 *
 * 按 URL 里的 `by=` 参数区分按日/按场次返回不同数据，默认两个维度都
 * 返回空数组。**P7 起不再单独区分"今日"请求**：主维度卡片与"当日电池
 * 到账"卡片现在都跟随选中日期，当维度是 `day` 时两者发出的是完全相同的
 * `by=day&since=...&until=...` 请求（这是设计使然，见 Stats.vue 里
 * loadStats/loadDayExtras 的注释），所以只按 `by` 区分就足够、也更贴近
 * 真实后端的行为——后端从不关心调用方是不是"当日卡片"发的请求。
 */
function stubFetch(
  opts: {
    activity?: unknown[]
    statsByDay?: unknown[]
    statsBySession?: unknown[]
    /** 「礼物」明细列表用的 GET .../gifts?since=...&until=... */
    giftsForDay?: unknown[]
  } = {},
) {
  const activity = opts.activity ?? []
  const statsByDay = opts.statsByDay ?? []
  const statsBySession = opts.statsBySession ?? []
  const giftsForDay = opts.giftsForDay ?? []
  const f = vi.fn().mockImplementation((url: string) => {
    if (url.startsWith('/api/bindings/1/gifts')) {
      return Promise.resolve(ok(giftsForDay))
    }
    if (url.startsWith('/api/bindings/1/stats')) {
      const parsed = new URL(url, 'http://localhost')
      const by = parsed.searchParams.get('by')
      return Promise.resolve(ok(by === 'session' ? statsBySession : statsByDay))
    }
    if (url.startsWith('/api/bindings/1/activity')) {
      return Promise.resolve(ok(activity))
    }
    return Promise.resolve(ok({ status: 'ok' }))
  })
  vi.stubGlobal('fetch', f)
  return f
}

describe('Stats 页', () => {
  beforeEach(() => {
    vi.unstubAllGlobals()
  })

  // 部分测试用 vi.useFakeTimers()/vi.setSystemTime() 固定"现在"来验证
  // 日期选择器的默认值与时区换算——不在每个用例里手动复位会让伪造的系统
  // 时间泄漏到后面的测试，useRealTimers 在没用过假时钟时也是安全的空操作。
  afterEach(() => {
    vi.useRealTimers()
  })

  it('未选择直播间时提示先选择，不请求任何数据', async () => {
    setActivePinia(createPinia())
    const f = stubFetch()
    const wrapper = mount(Stats)
    await flushPromises()

    expect(wrapper.text()).toContain('请先在顶部选择一个直播间')
    expect(f).not.toHaveBeenCalled()
  })

  it('选中直播间后自动请求 GET .../stats?by=day，并渲染全部统计卡片与两个维度选项', async () => {
    setupStore()
    const f = stubFetch({ statsByDay: [statsBucket({ danmakuCount: 10 })] })
    const wrapper = mount(Stats)
    await flushPromises()

    for (const label of [
      '弹幕数',
      '进房人数',
      '礼物种类',
      '礼物数量',
      '上舰数',
      '直播时长',
      '盲盒盈亏',
    ]) {
      expect(wrapper.text()).toContain(label)
    }
    expect(wrapper.text()).toContain('按日')
    expect(wrapper.text()).toContain('按场次')

    const statsCall = f.mock.calls.find((call) =>
      String(call[0]).startsWith('/api/bindings/1/stats'),
    )
    expect(statsCall).toBeTruthy()
    expect(String(statsCall![0])).toContain('by=day')
  })

  // P4-4 Task 7：盲盒盈亏卡片接上了真实数据（悬空清单第 7/15 条已解决），
  // 全页不应再出现"待后端支持"标签，且卡片要显示按 1/100 电池换算成
  // 「元」的真实数字，可正可负。
  it('盲盒盈亏卡片显示真实换算后的元，不再是占位符或待后端支持标签', async () => {
    setupStore()
    stubFetch({ statsByDay: [statsBucket({ blindBoxProfit: 20050 })] })
    const wrapper = mount(Stats)
    await flushPromises()

    const grid = wrapper.find('.stats-grid')
    expect(grid.text()).toContain('盲盒盈亏')
    expect(grid.text()).not.toContain('待后端支持')
    expect(wrapper.text()).not.toContain('待后端支持')
    // 换算系数是 1/1000（1 电池 = 0.1 元），不是 1/100——曾经错写成
    // /100 把这个值显示成 +200.50 元（差 10 倍），这条断言就是钉住
    // 正确系数的回归测试：20050（1/100 电池）= 20.05 元，正数带 + 号。
    expect(grid.text()).toContain('+20.05 元')
  })

  it('盲盒盈亏为负时显示负号，不做绝对值处理（真实体现"亏了"）', async () => {
    setupStore()
    stubFetch({ statsByDay: [statsBucket({ blindBoxProfit: -5600 })] })
    const wrapper = mount(Stats)
    await flushPromises()

    const grid = wrapper.find('.stats-grid')
    // -5600（1/100 电池）= -5.60 元
    expect(grid.text()).toContain('-5.60 元')
  })

  it('没有分桶数据时盲盒盈亏卡片显示占位符——不能把「算不出来」显示成「盈亏为 0」', async () => {
    setupStore()
    stubFetch({ statsByDay: [] })
    const wrapper = mount(Stats)
    await flushPromises()

    const grid = wrapper.find('.stats-grid')
    expect(grid.text()).toContain('—')
  })

  it('页面顶部说明数字来自真实聚合接口，并提示直播时长的历史缺口与礼物种类的求和限制', async () => {
    setupStore()
    stubFetch({ statsByDay: [statsBucket()] })
    const wrapper = mount(Stats)
    await flushPromises()

    expect(wrapper.text()).toContain('来自后端聚合接口')
    expect(wrapper.text()).toContain('不代表当时没开播')
    expect(wrapper.text()).toContain('重复计入')
  })

  // ---- 核心：统计数字必须来自 /stats 接口，不能是从 /activity 算出来的假数字 ----
  it('【关键证据】卡片数字直接等于 /stats 接口返回的聚合值，与 /activity 是否有数据无关', async () => {
    setupStore()
    const f = stubFetch({
      activity: make500Activity(),
      statsByDay: [
        statsBucket({
          danmakuCount: 1234,
          enterCount: 56,
          giftCount: 78,
          giftKinds: 9,
          guardCount: 3,
          liveSeconds: 3661, // 1 小时 1 分钟
        }),
      ],
    })
    const wrapper = mount(Stats)
    await flushPromises()

    const grid = wrapper.find('.stats-grid')
    expect(grid.text()).toContain('1234')
    expect(grid.text()).toContain('56')
    expect(grid.text()).toContain('78')
    // 礼物种类 9、上舰数 3 分别出现在各自卡片里
    expect(grid.text()).toContain('1 小时 1 分钟')

    // 展开「最近活动预览」，确认即便 /activity 有 500 条原始行，
    // 统计卡片区域用的仍然是 /stats 返回的聚合值，不会被 500 干扰
    const expandBtn = wrapper.findAll('button').find((b) => b.text() === '展开')
    await expandBtn!.trigger('click')
    await flushPromises()
    expect(wrapper.text()).toContain('本次采样加载了 500 条原始行')
    expect(grid.text()).not.toContain('500')
    expect(grid.text()).toContain('1234')

    // 请求确实发生了（证明卡片不是硬编码）
    const statsCall = f.mock.calls.find((call) =>
      String(call[0]).startsWith('/api/bindings/1/stats'),
    )
    expect(statsCall).toBeTruthy()
  })

  it('请求 /stats 失败时显示后端错误原文，不显示假数字', async () => {
    setupStore()
    const backendMessage = 'since 不能晚于 until（since=..., until=...）'
    vi.stubGlobal(
      'fetch',
      vi.fn().mockImplementation((url: string) => {
        if (url.startsWith('/api/bindings/1/stats'))
          return Promise.resolve(err(422, backendMessage))
        return Promise.resolve(ok([]))
      }),
    )
    const wrapper = mount(Stats)
    await flushPromises()

    expect(wrapper.text()).toContain(backendMessage)
  })

  it('最近活动预览默认折叠，且明确标注「不是统计数字」', async () => {
    setupStore()
    const f = stubFetch({ activity: make500Activity(), statsByDay: [statsBucket()] })
    const wrapper = mount(Stats)
    await flushPromises()

    // 默认折叠：还没点「展开」之前不应该发出 /activity 请求
    const hitActivity = f.mock.calls.some((call) => String(call[0]).includes('/activity'))
    expect(hitActivity).toBe(false)
    expect(wrapper.text()).toContain('这不是统计数字')
  })

  // ---- 切绑定要清空预览与重新拉取统计 ----
  it('展开预览后切换绑定，旧绑定的预览行被清空，统计卡片也为新绑定重新请求', async () => {
    setActivePinia(createPinia())
    const bindings = useBindingsStore()
    const 绑定乙 = { ...绑定, id: 2, roomId: '456' }
    bindings.list = [绑定, 绑定乙]
    bindings.select(1)

    const rowsFor甲 = [
      {
        id: 1,
        kind: 'event' as const,
        eventType: 'danmaku',
        actionType: '',
        ruleName: '',
        userUid: '1',
        userName: '甲房间观众',
        detail: { text: '你好' },
        occurredAt: '2026-08-01T12:00:00Z',
      },
    ]
    const rowsFor乙 = [
      {
        id: 2,
        kind: 'event' as const,
        eventType: 'danmaku',
        actionType: '',
        ruleName: '',
        userUid: '2',
        userName: '乙房间观众',
        detail: { text: '嗨' },
        occurredAt: '2026-08-01T12:05:00Z',
      },
    ]

    const f = vi.fn().mockImplementation((url: string) => {
      if (url.startsWith('/api/bindings/1/stats'))
        return Promise.resolve(ok([statsBucket({ danmakuCount: 1 })]))
      if (url.startsWith('/api/bindings/2/stats'))
        return Promise.resolve(ok([statsBucket({ danmakuCount: 2 })]))
      if (url.startsWith('/api/bindings/1/activity')) return Promise.resolve(ok(rowsFor甲))
      if (url.startsWith('/api/bindings/2/activity')) return Promise.resolve(ok(rowsFor乙))
      // /gifts 是 P6 任务 5 新增的礼物明细列表接口，必须显式返回数组——
      // 落到下面 { status: 'ok' } 那个兜底会把 giftBreakdown 塞进一个
      // 非数组对象，NDataTable 拿着它算内部 treemate 结构会直接抛错。
      if (url.startsWith('/api/bindings/1/gifts')) return Promise.resolve(ok([]))
      if (url.startsWith('/api/bindings/2/gifts')) return Promise.resolve(ok([]))
      return Promise.resolve(ok({ status: 'ok' }))
    })
    vi.stubGlobal('fetch', f)

    const wrapper = mount(Stats)
    await flushPromises()

    const expandBtn = wrapper.findAll('button').find((b) => b.text() === '展开')
    expect(expandBtn).toBeTruthy()
    await expandBtn!.trigger('click')
    await flushPromises()

    expect(wrapper.text()).toContain('甲房间观众')
    expect(wrapper.find('.stats-grid').text()).toContain('1')

    bindings.select(2)
    await flushPromises()

    // 旧绑定（甲）的预览行必须被清空，不能残留展示成乙房间的数据
    expect(wrapper.text()).not.toContain('甲房间观众')
    // 之前是展开状态，切绑定后应当自动为新绑定重新请求并显示新数据
    expect(wrapper.text()).toContain('乙房间观众')
    // 统计卡片也要跟着换成乙绑定的数字
    expect(wrapper.find('.stats-grid').text()).toContain('2')
  })

  // ---- 全批次终审项【4】：没有分桶数据时不能显示 0 ----
  //
  // 修复前 totals 对空数组求和永远得到 0，卡片会斩钉截铁地显示
  // "弹幕数 0"——而这不代表"这段时间真的一条弹幕都没有"，可能只是
  // "压根没有分桶数据可算"。这正是这一页此前用占位符防住的错误，
  // 只是这次的触发条件是空桶数组，不是接口报错。
  it('by=day 没有任何分桶数据时，六张真实数字卡片显示占位符「—」而不是 0', async () => {
    setupStore()
    stubFetch({ statsByDay: [] })
    const wrapper = mount(Stats)
    await flushPromises()

    const values = wrapper.findAllComponents(NStatistic).map((c) => c.props('value'))
    // 8 张卡片：6 张原有真实数字 + 1 张一直悬空的盲盒盈亏 + P6 任务 5
    // 新增的「当日电池到账」，全部应为占位符——「当日电池到账」固定用
    // by=day 请求，跟主维度请求命中的是同一份（空）statsByDay 数据。
    expect(values).toEqual(['—', '—', '—', '—', '—', '—', '—', '—'])
  })

  it(
    'by=session 没有任何分桶数据时，页面给出关于 live_start/live_stop 的明确说明——' +
      '不能只在顶部提示里说"直播时长会显示 0"，这次连弹幕数、上舰数等全部字段都受影响',
    async () => {
      setupStore()
      const f = stubFetch({
        statsByDay: [statsBucket({ danmakuCount: 10 })],
        statsBySession: [],
      })
      const wrapper = mount(Stats)
      await flushPromises()

      wrapper.findComponent(NRadioGroup).vm.$emit('update:value', 'session')
      await flushPromises()

      const lastStatsCall = [...f.mock.calls]
        .reverse()
        .find((call) => String(call[0]).startsWith('/api/bindings/1/stats'))
      expect(String(lastStatsCall![0])).toContain('by=session')

      const cards = wrapper
        .findAllComponents(NStatistic)
        .map((c) => ({ label: c.props('label'), value: c.props('value') }))

      // 7 张跟随「按场次」维度的卡片必须全部占位符——这是 by=session
      // 空分桶时一直守住的行为，日期选择器加进来之后不该变。
      const dimensionCards = cards.filter((c) => c.label !== '当日电池到账')
      expect(dimensionCards.every((c) => c.value === '—')).toBe(true)

      // 「当日电池到账」卡片固定用 by=day 请求（statsByDay 非空，
      // danmakuCount:10 那份数据），不受「按场次」维度切到空分桶影响——
      // 这正是"当日卡片与维度切换解耦"设计要验证的地方：如果哪天不小心
      // 把这张卡片也接到了 dimension 上，这条断言会先变红。
      const dayCard = cards.find((c) => c.label === '当日电池到账')
      expect(dayCard?.value).not.toBe('—')

      // 页面级说明必须点名 live_start/live_stop，而不是一句笼统的"没有数据"
      expect(wrapper.text()).toContain('live_start')
      expect(wrapper.text()).toContain('live_stop')
      expect(wrapper.text()).toContain('按场次维度无法分桶')
    },
  )

  it('by=session 有分桶数据时不显示"按场次维度暂时无法分桶"的说明', async () => {
    setupStore()
    stubFetch({ statsBySession: [statsBucket({ danmakuCount: 5 })] })
    const wrapper = mount(Stats)
    wrapper.findComponent(NRadioGroup).vm.$emit('update:value', 'session')
    await flushPromises()

    expect(wrapper.text()).not.toContain('按场次维度暂时无法分桶')
    const values = wrapper.findAllComponents(NStatistic).map((c) => c.props('value'))
    expect(values[0]).toBe('5') // 弹幕数卡片：真实数字，不是占位符
  })

  it('维度切换（按场次/按日）会重新请求聚合接口，带上对应的 by 参数，卡片数字跟着变', async () => {
    setupStore()
    const f = stubFetch({
      statsByDay: [statsBucket({ bucket: '2026-08-01', danmakuCount: 10 })],
      statsBySession: [statsBucket({ bucket: 'session-1', danmakuCount: 20 })],
    })
    const wrapper = mount(Stats)
    await flushPromises()

    expect(wrapper.find('.stats-grid').text()).toContain('10')
    const callsBefore = f.mock.calls.length

    // 用 findComponent 直接驱动 NRadioGroup 的 v-model，而不是点 DOM——
    // 与 ConditionTree.test.ts 里驱动单选框的方式一致，更确定。
    wrapper.findComponent(NRadioGroup).vm.$emit('update:value', 'session')
    await flushPromises()

    // 确实发出了新请求，而不是原地不动
    expect(f.mock.calls.length).toBeGreaterThan(callsBefore)
    const lastStatsCall = [...f.mock.calls]
      .reverse()
      .find((call) => String(call[0]).startsWith('/api/bindings/1/stats'))
    expect(String(lastStatsCall![0])).toContain('by=session')

    // 卡片数字要变成「按场次」的数据——P7 去掉了分桶明细表，维度切换是
    // 否生效现在只能靠卡片数字本身验证。
    expect(wrapper.find('.stats-grid').text()).toContain('20')
  })

  it('P7：页面不再渲染「分桶明细」表', async () => {
    setupStore()
    stubFetch({ statsByDay: [statsBucket({ bucket: '2026-08-01', danmakuCount: 10 })] })
    const wrapper = mount(Stats)
    await flushPromises()

    expect(wrapper.find('.bucket-card').exists()).toBe(false)
    expect(wrapper.text()).not.toContain('分桶明细')
  })
})

// ---- P6 任务 5：当日电池到账 + 礼物明细列表（P7 起跟随日期选择器） ----
describe('Stats 页：当日电池到账 + 礼物明细列表', () => {
  it('选中直播间后，会发出两个 /stats 请求（主维度 + 当日卡片）与一个 /gifts 请求，均带 since/until', async () => {
    setupStore()
    const f = stubFetch({ statsByDay: [statsBucket({ danmakuCount: 1 })] })
    mount(Stats)
    await flushPromises()

    const statsCalls = f.mock.calls.filter((call) =>
      String(call[0]).startsWith('/api/bindings/1/stats'),
    )
    // 主维度请求（dimension=day 时是 by=day）与「当日卡片」请求（固定
    // by=day）是两个独立发出的调用，即便 dimension 恰好也是 day、参数
    // 因此完全相同——两者用途本来就是解耦的，不能因为参数撞了就合并成
    // 一次请求去验证，那样测不出"当日卡片确实有自己独立的请求路径"。
    expect(statsCalls.length).toBeGreaterThanOrEqual(2)
    expect(statsCalls.every((call) => String(call[0]).includes('since='))).toBe(true)

    const giftsCall = f.mock.calls.find((call) => String(call[0]).startsWith('/api/bindings/1/gifts'))
    expect(giftsCall, '应该发出 /gifts 请求').toBeTruthy()
    expect(String(giftsCall![0])).toContain('since=')
  })

  it('「当日电池到账」卡片显示 giftCoins 换算后的电池数（除以 100，不换算成元）', async () => {
    setupStore()
    // 50000（1/100 电池）= 500 电池——注意这是"电池"不是"元"，不要
    // 跟盲盒盈亏卡片的 /1000（元）换算搞混。
    stubFetch({ statsByDay: [statsBucket({ giftCoins: 50000 })] })
    const wrapper = mount(Stats)
    await flushPromises()

    expect(wrapper.find('.stats-grid').text()).toContain('500.00 电池')
  })

  it('选中日期没有任何数据时「当日电池到账」显示占位符，不是 0', async () => {
    setupStore()
    stubFetch({ statsByDay: [] })
    const wrapper = mount(Stats)
    await flushPromises()

    const card = wrapper.findAll('.stat-card').find((c) => c.text().includes('当日电池到账'))
    expect(card, '应该有「当日电池到账」这张卡片').toBeTruthy()
    expect(card!.text()).toContain('—')
  })

  it('「礼物」明细列表按礼物名分组显示数量与电池数（免费礼物电池数为 0）', async () => {
    setupStore()
    stubFetch({
      giftsForDay: [
        giftRow({ giftName: '辣条', count: 3, coins: 150000 }),
        giftRow({ giftName: '小心心', count: 5, coins: 0 }),
      ],
    })
    const wrapper = mount(Stats)
    await flushPromises()

    const list = wrapper.find('.gift-breakdown-card')
    expect(list.exists(), '应该有标题为「礼物」的明细列表卡片').toBe(true)
    expect(list.text()).toContain('辣条')
    expect(list.text()).toContain('1500.00 电池') // 150000 / 100
    expect(list.text()).toContain('小心心')
    // 免费礼物数量照常显示，但电池数是 0——不能因为是免费礼物就连数量
    // 也不显示。
    expect(list.text()).toContain('5')
    expect(list.text()).toContain('0.00 电池')
  })

  it('「礼物」明细列表为空时显示空状态提示，不是一张空表格', async () => {
    setupStore()
    stubFetch({ giftsForDay: [] })
    const wrapper = mount(Stats)
    await flushPromises()

    const list = wrapper.find('.gift-breakdown-card')
    expect(list.exists()).toBe(true)
    expect(list.text()).toContain('没有数据')
  })

  it('切换绑定后重新请求当日电池到账与礼物明细', async () => {
    setActivePinia(createPinia())
    const bindings = useBindingsStore()
    const 绑定乙 = { ...绑定, id: 2, roomId: '456' }
    bindings.list = [绑定, 绑定乙]
    bindings.select(1)

    const f = vi.fn().mockImplementation((url: string) => {
      if (url.startsWith('/api/bindings/1/gifts')) {
        return Promise.resolve(ok([giftRow({ giftName: '甲房间礼物' })]))
      }
      if (url.startsWith('/api/bindings/2/gifts')) {
        return Promise.resolve(ok([giftRow({ giftName: '乙房间礼物' })]))
      }
      return Promise.resolve(ok([]))
    })
    vi.stubGlobal('fetch', f)

    const wrapper = mount(Stats)
    await flushPromises()
    expect(wrapper.find('.gift-breakdown-card').text()).toContain('甲房间礼物')

    bindings.select(2)
    await flushPromises()
    expect(wrapper.find('.gift-breakdown-card').text()).not.toContain('甲房间礼物')
    expect(wrapper.find('.gift-breakdown-card').text()).toContain('乙房间礼物')
  })
})

// ---- P7：统计页改用日期选择器（真机反馈二次返工） ----
describe('Stats 页：日期选择器', () => {
  it('用日历控件（n-date-picker）而不是下拉菜单，且不可清空', async () => {
    setupStore()
    stubFetch({ statsByDay: [statsBucket()] })
    const wrapper = mount(Stats)
    await flushPromises()

    const picker = wrapper.findComponent(NDatePicker)
    expect(picker.exists(), '统计维度那一行应该有 NDatePicker').toBe(true)
    expect(picker.props('type')).toBe('date')
    // 不可清空——见 Stats.vue 里 NDatePicker 旁边注释：选中日期是全部
    // 卡片/明细共同依赖的查询条件，不该允许被清成"没有选中任何一天"。
    expect(picker.props('clearable')).toBe(false)
  })

  // ---- 核心：默认选中"今天"，且换算成 since/until 时用的是本地自然天，
  // 不是 UTC 自然天 ----
  //
  // 用 vi.setSystemTime 把"现在"钉死在北京时间 2026-08-04 10:00——这个
  // 时刻换算成 UTC 是 2026-08-04 02:00，如果代码不小心按 UTC 自然天来
  // 解释"今天"（比如把 getFullYear/getMonth/getDate 误写成对应的 UTC
  // 版本），since 会变成 2026-08-04T00:00:00.000Z 而不是期望的
  // 2026-08-03T16:00:00.000Z——两者相差整整 8 小时，是本页最容易也最不
  // 该出现的一类错误，这条测试就是钉死正确方向的回归测试。
  it('默认选中今天，请求的 since/until 是本地自然天换算成的 UTC 边界（不是 UTC 自然天）', async () => {
    setupStore()
    vi.useFakeTimers()
    vi.setSystemTime(new Date('2026-08-04T10:00:00+08:00'))
    const f = stubFetch({ statsByDay: [statsBucket({ danmakuCount: 1 })] })
    mount(Stats)
    await flushPromises()

    const statsCall = f.mock.calls.find((call) =>
      String(call[0]).startsWith('/api/bindings/1/stats'),
    )
    expect(statsCall, '应该发出 stats 请求').toBeTruthy()
    const url = new URL(String(statsCall![0]), 'http://localhost')
    // 北京时间 8 月 4 日 00:00 = UTC 8 月 3 日 16:00；
    // 北京时间 8 月 4 日 23:59:59.999 = UTC 8 月 4 日 15:59:59.999。
    expect(url.searchParams.get('since')).toBe('2026-08-03T16:00:00.000Z')
    expect(url.searchParams.get('until')).toBe('2026-08-04T15:59:59.999Z')
  })

  it('换一个日期后，主统计与「礼物」明细都用新日期重新请求（不是继续显示"今天"）', async () => {
    setupStore()
    vi.useFakeTimers()
    vi.setSystemTime(new Date('2026-08-04T10:00:00+08:00'))
    const f = stubFetch({ statsByDay: [statsBucket({ danmakuCount: 1 })] })
    const wrapper = mount(Stats)
    await flushPromises()
    const callsBefore = f.mock.calls.length

    // 选一个本地时区的过去日期：2026-07-20（月份从 0 开始，6 = 7 月）。
    const picked = new Date(2026, 6, 20).getTime()
    wrapper.findComponent(NDatePicker).vm.$emit('update:value', picked)
    await flushPromises()

    expect(f.mock.calls.length).toBeGreaterThan(callsBefore)

    const newCalls = f.mock.calls.slice(callsBefore)
    const statsCall = newCalls.find((call) => String(call[0]).startsWith('/api/bindings/1/stats'))
    const giftsCall = newCalls.find((call) => String(call[0]).startsWith('/api/bindings/1/gifts'))
    expect(statsCall, '换日期后应该重新请求 stats').toBeTruthy()
    expect(giftsCall, '换日期后应该重新请求 gifts').toBeTruthy()

    const statsUrl = new URL(String(statsCall![0]), 'http://localhost')
    const giftsUrl = new URL(String(giftsCall![0]), 'http://localhost')
    // 北京时间 7 月 20 日 00:00 = UTC 7 月 19 日 16:00——不是默认今天
    // （8 月 4 日）的区间，证明请求真的跟着选中日期换了，不是仍然停在
    // "今天"上（也就是自检变异 (a)：卡片不跟随选中日期，会在这里变红）。
    expect(statsUrl.searchParams.get('since')).toBe('2026-07-19T16:00:00.000Z')
    expect(statsUrl.searchParams.get('until')).toBe('2026-07-20T15:59:59.999Z')
    expect(giftsUrl.searchParams.get('since')).toBe('2026-07-19T16:00:00.000Z')
    expect(giftsUrl.searchParams.get('until')).toBe('2026-07-20T15:59:59.999Z')
  })
})

// P5-3：与其余页面同一条要求——统计页此前完全看不出正在看哪个绑定的数据。
// 统计页没有专门的权限点校验，选择器不带 requiredPerm。
describe('Stats 页：正文里也有账号+直播间选择器', () => {
  it('页面渲染 BindingSelector，不要求特定权限', async () => {
    setupStore()
    stubFetch({ statsByDay: [] })
    const wrapper = mount(Stats)
    await flushPromises()

    const { default: BindingSelector } = await import('@/components/BindingSelector.vue')
    const selector = wrapper.findComponent(BindingSelector)
    expect(selector.exists()).toBe(true)
    expect(selector.props('requiredPerm')).toBeUndefined()
  })
})
