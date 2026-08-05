import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import { NDatePicker, NRadioGroup, NSelect, NStatistic } from 'naive-ui'

// P7b：不再把测试进程的时区钉死成某个固定值（P7 曾经写死"东八区"，那
// 正是这次要纠正的错误前提本身——本页现在的日界计算只依赖"日历选中的
// Y-M-D" + "选择器里显式选中的 IANA 时区"两个量，两者都由测试显式控制，
// 与运行测试的宿主机实际处于哪个时区无关。刻意不 pin TZ，是为了让这份
// 测试在任何宿主时区下跑结果都一样——如果哪天代码不小心又开始偷看
// `process.env.TZ`/宿主本地时区参与计算，这份测试应该会因为在不同宿主
// 上结果不一致而暴露，而不是靠 pin 住一个值把问题捂住。
//
// 需要固定"现在"的用例一律用 `new Date(y, m, d, h, mi)`（本地分量构造）
// 而不是带 `+08:00` 之类固定偏移的 ISO 字符串——本地分量构造在任何宿主
// 时区下都会被 `getFullYear`/`getMonth`/`getDate` 原样读回同一组 Y-M-D，
// 偏移字符串则不会（在非 +08:00 的宿主上读回的日历日会漂移）。

const { default: Stats } = await import('./Stats.vue')
const { useBindingsStore } = await import('@/stores/bindings')

/**
 * resolveTz 把一个 IANA 时区名（可能是别名）解析成**当前运行环境**的
 * 规范拼写。
 *
 * **不能在测试里直接写字面量 `'Asia/Kolkata'`**——同一个真实时区在不同
 * ICU 版本里的规范名可能不同（这套测试环境的 ICU 认的是老拼写
 * `Asia/Calcutta`，`Asia/Kolkata` 只是它认识的一个别名，不在
 * `Intl.supportedValuesOf('timeZone')` 返回的规范列表里）。Stats.vue 的
 * `loadStoredTimezone` 会用规范列表校验存的名字，写字面量别名会被判定
 * 成"认不出的过期名字"而被拒绝、静默回退到浏览器探测值——不是逻辑错，
 * 是这条测试自己用了一个在当前平台不算规范的拼写。用同一个 `Intl` API
 * 现场解析一遍，保证测试在任何 ICU 版本的运行环境下都用平台真正认识的
 * 那个拼写，不会因为换了 Node/浏览器版本就变红。
 */
function resolveTz(name: string): string {
  return new Intl.DateTimeFormat('en-US', { timeZone: name }).resolvedOptions().timeZone
}

/**
 * NON_DEFAULT_TZ 是一个"确定跟浏览器探测到的默认时区不同"的时区名，供
 * 需要验证"切换时区"这个动作本身的用例使用。
 *
 * **不能直接写死 `'Pacific/Auckland'` 当作"切换后的新值"**——这份测试
 * 就是在真实的新西兰机器上开发的，`Intl.DateTimeFormat().resolvedOptions()
 * .timeZone` 探测到的默认值本身就是 `Pacific/Auckland`；如果测试选中的
 * "新值"恰好和默认值撞了，Vue 的 `watch` 不会因为赋值成同一个值而触发
 * （值没变化），持久化的 `watch(timezone, ...)` 回调根本不会跑，这条
 * 测试会在这台开发机上"假红"或者靠巧合"假绿"——取决于具体断言写法，
 * 两种都不可靠。这里现场探测默认值，从候选列表里挑一个确定不同的，
 * 保证这条测试不管跑在哪台机器上，"切换"这个动作都是一次真正的值变化。
 */
const NON_DEFAULT_TZ = (() => {
  const detected = Intl.DateTimeFormat().resolvedOptions().timeZone
  const candidate = ['Pacific/Auckland', 'Asia/Kolkata', 'Etc/UTC']
    .map(resolveTz)
    .find((tz) => tz !== detected)
  if (!candidate) throw new Error('挑不出一个跟默认探测值不同的候选时区，测试环境异常')
  return candidate
})()

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
    blindBox: false,
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

// 文件级 hook（不挂在任何一个 describe 下）：每条用例开始前清空
// localStorage。P7b 新增的 `magicd.statsTimezone` 是持久化状态，一旦某
// 条用例写入就会一直留在同一个 jsdom 环境里（同一测试文件内的用例共享
// 一个 localStorage，不会像 pinia store 那样每条用例自动重建）——不清的
// 话，跑在某条用例之后的其他用例会读到不属于自己的时区偏好，"默认取浏览
// 器探测值"之类的用例会因为运行顺序不同而忽绿忽红。挂在文件级而不是某个
// describe 的 beforeEach 里，是因为本文件有好几个平级的 describe 块，
// 只挂在其中一个保护不到另外几个。
beforeEach(() => {
  localStorage.clear()
})

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

    const giftsCall = f.mock.calls.find((call) =>
      String(call[0]).startsWith('/api/bindings/1/gifts'),
    )
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

  // P8 真机反馈：明细此前完全排除盲盒，而「当日电池到账」含盲盒——423
  // 电池只能逐行加出 103，差额没有出处。现在盲盒进明细并单独标「来源」。
  it('「礼物」明细列表列出盲盒爆出的礼物并标注来源，同名礼物的两种来源各占一行', async () => {
    setupStore()
    stubFetch({
      giftsForDay: [
        giftRow({ giftName: '爱心抱枕', blindBox: true, count: 2, coins: 32000 }),
        giftRow({ giftName: '爱心抱枕', blindBox: false, count: 1, coins: 16000 }),
      ],
    })
    const wrapper = mount(Stats)
    await flushPromises()

    const list = wrapper.find('.gift-breakdown-card')
    expect(list.text()).toContain('来源')
    expect(list.text()).toContain('盲盒')
    expect(list.text()).toContain('常规')
    expect(list.text()).toContain('320.00 电池') // 32000 / 100
    expect(list.text()).toContain('160.00 电池') // 16000 / 100

    // 同名礼物的两种来源必须各占一行，不能被合并显示。
    //
    // **这条断言守住的是「两行都被列出来」，不是「行键唯一」**——实测过：
    // 把 row-key 改回只用 giftName（重复键）后本测试仍然通过，naive-ui
    // 在这里既不告警也不吞行。行键带上来源仍然是对的（Vue 要求 key 唯一，
    // 重复键会在数据更新时错配行内容），但那条约束在这一层测不出来，
    // 不要因为这条测试是绿的就以为它把行键也钉住了。
    const bodyRows = list.findAll('tbody tr')
    expect(bodyRows.length, '同一个礼物名的盲盒行与常规行必须各占一行').toBe(2)
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

  // ---- 核心：默认选中"今天"，换算成 since/until 时用的是**显式选中的
  // 统计时区**，不是宿主机/浏览器本地时区 ----
  //
  // P7 的等价测试把"现在"钉死在北京时间、断言换算用的是本地时区——那条
  // 断言建立在"用户在东八区"这个已经被证伪的前提上。P7b 改成参数化：
  // 时区通过 localStorage 显式指定成 Pacific/Auckland（不是宿主机时区，
  // 也不是任何写死在生产环境里的值），"现在"用本地分量构造
  // （`new Date(y, m, d, h, mi)`），这样不管测试实际跑在哪个宿主时区上，
  // 读回的日历 Y-M-D 都是同一组值——因此这条测试在任何宿主机/CI 上跑
  // 结果都一样，不依赖"测试机恰好是哪个时区"。
  //
  // 8 月处于南半球冬季，Pacific/Auckland 当时是 NZST（+12，不实行夏令
  // 时的季节），since/until 之间是标准 24 小时——夏令时切换日的场景
  // 单独在下面的"P7b：统计时区选择器"里用专门的用例覆盖。
  it('默认选中今天，请求的 since/until 按选中的统计时区换算（不是宿主机本地时区）', async () => {
    localStorage.setItem('magicd.statsTimezone', 'Pacific/Auckland')
    setupStore()
    vi.useFakeTimers()
    // 本地分量构造："现在"是本地日历的 2026-08-04 10:00，与宿主机实际
    // 处于哪个时区无关——Stats.vue 内部同样用本地 getter 读回这组分量，
    // 两边用的是同一套（与时区无关的）Y-M-D，不会因为宿主时区不同而漂移。
    vi.setSystemTime(new Date(2026, 7, 4, 10, 0, 0))
    const f = stubFetch({ statsByDay: [statsBucket({ danmakuCount: 1 })] })
    mount(Stats)
    await flushPromises()

    const statsCall = f.mock.calls.find((call) =>
      String(call[0]).startsWith('/api/bindings/1/stats'),
    )
    expect(statsCall, '应该发出 stats 请求').toBeTruthy()
    const url = new URL(String(statsCall![0]), 'http://localhost')
    // Pacific/Auckland 8 月 4 日 00:00（NZST，+12）= UTC 8 月 3 日 12:00；
    // 8 月 4 日 23:59:59.999 = UTC 8 月 4 日 11:59:59.999。
    expect(url.searchParams.get('since')).toBe('2026-08-03T12:00:00.000Z')
    expect(url.searchParams.get('until')).toBe('2026-08-04T11:59:59.999Z')
  })

  it('换一个日期后，主统计与「礼物」明细都用新日期重新请求（不是继续显示"今天"）', async () => {
    localStorage.setItem('magicd.statsTimezone', resolveTz('Asia/Kolkata'))
    setupStore()
    vi.useFakeTimers()
    vi.setSystemTime(new Date(2026, 7, 4, 10, 0, 0))
    const f = stubFetch({ statsByDay: [statsBucket({ danmakuCount: 1 })] })
    const wrapper = mount(Stats)
    await flushPromises()
    const callsBefore = f.mock.calls.length

    // 选一个过去日期：2026-07-20（月份从 0 开始，6 = 7 月），同样用本地
    // 分量构造，与宿主机时区无关。
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
    // Asia/Kolkata（+5:30，非整点偏移）7 月 20 日 00:00 = UTC 7 月 19 日
    // 18:30——不是默认今天（8 月 4 日）的区间，证明请求真的跟着选中日期
    // 换了，不是仍然停在"今天"上（自检变异 (a)：卡片不跟随选中日期，
    // 会在这里变红）。分钟位是 30 而不是 00，顺带验证了非整点偏移
    // （用户提到的 +5:45 尼泊尔场景的姊妹案例）没有被四舍五入成整点。
    expect(statsUrl.searchParams.get('since')).toBe('2026-07-19T18:30:00.000Z')
    expect(statsUrl.searchParams.get('until')).toBe('2026-07-20T18:29:59.999Z')
    expect(giftsUrl.searchParams.get('since')).toBe('2026-07-19T18:30:00.000Z')
    expect(giftsUrl.searchParams.get('until')).toBe('2026-07-20T18:29:59.999Z')
  })
})

/**
 * timezoneSelect 从挂载的 Stats 组件里精确定位"统计时区"那一个 NSelect。
 *
 * **不能直接 `wrapper.findComponent(NSelect)`**——页头的 `BindingSelector`
 * 子组件自己也用 NSelect 选账号/直播间，且在 DOM 树里排在时区选择器
 * 前面，`findComponent` 找的是第一个匹配，会精确地拿到*错误*的那一个
 * （症状很隐蔽：拿到的 `options`/`value` 是绑定列表的，不是时区的，
 * 断言会以一种看似合理但完全对不上号的方式失败）。用"选项数量上百"这个
 * 只有时区选择器才有的特征来筛，比新增一个只服务于测试的 DOM class 更
 * 直接复用已经验证过的语义（IANA 规范时区名有 400+ 个，测试里出现的
 * 绑定列表最多几条，量级差两个数量级，不会混淆）。
 */
function timezoneSelect(wrapper: ReturnType<typeof mount>) {
  const found = wrapper
    .findAllComponents(NSelect)
    .find(
      (s) => Array.isArray(s.props('options')) && (s.props('options') as unknown[]).length > 100,
    )
  if (!found) throw new Error('没有找到时区选择器（选项数超过 100 的 NSelect）')
  return found
}

// ---- P7b：统计时区必须显式可选（纠正 P7"按浏览器本地自然天"的错误前提） ----
//
// 用户 2026-08-04 指出：P7 把"猜服务器时区"换成了"猜浏览器时区"，仍然是
// 猜——部署机、看统计的人、主播本人可能落在三个不同时区（他本人在 +12
// 区），任何隐式推断都会在某些组合下把一整天的数据算错。这里的用例覆盖
// 四件事：①选择器存在且默认值可见、可改；②选中值持久化到 localStorage；
// ③换时区真的改变请求的 since/until（不是摆设，也不是偷偷退回宿主本地
// 时区）；④夏令时切换日用 IANA 时区能算对、固定偏移必错——专门验证这一
// 点是为了在"改用固定偏移代替 IANA"这类回退变异下让测试变红。
describe('Stats 页：统计时区选择器', () => {
  it('存在一个可见、可编辑的时区选择器，默认值等于浏览器探测到的时区', async () => {
    setupStore()
    stubFetch({ statsByDay: [statsBucket()] })
    const wrapper = mount(Stats)
    await flushPromises()

    const select = timezoneSelect(wrapper)
    expect(select.exists(), '统计维度那一行应该有时区选择器（NSelect）').toBe(true)
    expect(select.props('clearable')).toBe(false)
    // 默认值必须等于浏览器探测结果——这里不写死具体是哪个时区名（写死
    // 就是重犯 P7 的错，把测试也绑死在一个特定时区上），而是现场用同一
    // 个标准 API 算一遍期望值，两边算法一致即代表"确实用了探测结果"。
    expect(select.props('value')).toBe(Intl.DateTimeFormat().resolvedOptions().timeZone)
    // 界面上真的显示出这个时区名文本，不是只存在 props 里用户看不见——
    // 这是"默认值要可见"这条硬性要求的直接落地。
    expect(wrapper.text()).toContain(Intl.DateTimeFormat().resolvedOptions().timeZone)
  })

  it('选项来自 Intl.supportedValuesOf("timeZone")，不是手写的固定偏移/整点范围表', async () => {
    setupStore()
    stubFetch({ statsByDay: [statsBucket()] })
    const wrapper = mount(Stats)
    await flushPromises()

    const options = timezoneSelect(wrapper).props('options') as Array<{
      label: string
      value: string
    }>
    const expected = Intl.supportedValuesOf('timeZone')
    // 数量、内容都跟平台给出的规范时区名列表完全一致——如果实现改成了
    // 手写一份"-13 到 +12"或者只含整点偏移的列表，这里的数量/内容会对
    // 不上，测试会红。
    expect(options.map((o) => o.value)).toEqual(expected)
    // 顺带确认覆盖了非整点偏移与南半球夏令时区——不是巧合，是
    // Intl.supportedValuesOf 天然给出的完整列表的一部分。
    expect(options.some((o) => o.value === 'Pacific/Auckland')).toBe(true)
  })

  it('切换时区会持久化到 localStorage，刷新（重新挂载）后记住选择', async () => {
    setupStore()
    stubFetch({ statsByDay: [statsBucket()] })
    const wrapper = mount(Stats)
    await flushPromises()

    timezoneSelect(wrapper).vm.$emit('update:value', NON_DEFAULT_TZ)
    await flushPromises()
    expect(localStorage.getItem('magicd.statsTimezone')).toBe(NON_DEFAULT_TZ)

    // 重新挂载模拟"刷新页面"——不能每次进页面都要重选一次。
    const wrapper2 = mount(Stats)
    await flushPromises()
    expect(timezoneSelect(wrapper2).props('value')).toBe(NON_DEFAULT_TZ)
  })

  it('localStorage 里存的是当前 Intl 不认识的过期时区名时，回退到浏览器探测值而不是直接用坏值', async () => {
    localStorage.setItem('magicd.statsTimezone', 'Not/A_Real_Timezone')
    setupStore()
    stubFetch({ statsByDay: [statsBucket()] })
    const wrapper = mount(Stats)
    await flushPromises()

    expect(timezoneSelect(wrapper).props('value')).toBe(
      Intl.DateTimeFormat().resolvedOptions().timeZone,
    )
  })

  // ---- 自检变异 (a)：忽略选中时区、退回浏览器/宿主本地时区 ----
  //
  // 同一个日历日、同一台"宿主机"，只换选择器里的时区值，请求的 since
  // 必须跟着变。如果实现偷偷改回读宿主本地时区（不管选择器选了什么都
  // 用同一个值），这两次请求会得到相同的 since，下面的 not.toBe 断言
  // 会变红。
  it('同一天，切换时区后请求的 since/until 跟着变——不会忽略选择器退回宿主本地时区', async () => {
    setupStore()
    vi.useFakeTimers()
    vi.setSystemTime(new Date(2026, 7, 4, 10, 0, 0))
    const f = stubFetch({ statsByDay: [statsBucket({ danmakuCount: 1 })] })
    const wrapper = mount(Stats)
    await flushPromises()

    timezoneSelect(wrapper).vm.$emit('update:value', 'Pacific/Auckland')
    await flushPromises()
    const afterAuckland = [...f.mock.calls]
      .reverse()
      .find((call) => String(call[0]).startsWith('/api/bindings/1/stats'))
    const sinceAuckland = new URL(String(afterAuckland![0]), 'http://localhost').searchParams.get(
      'since',
    )

    timezoneSelect(wrapper).vm.$emit('update:value', 'Asia/Kolkata')
    await flushPromises()
    const afterKolkata = [...f.mock.calls]
      .reverse()
      .find((call) => String(call[0]).startsWith('/api/bindings/1/stats'))
    const sinceKolkata = new URL(String(afterKolkata![0]), 'http://localhost').searchParams.get(
      'since',
    )

    // 日历日全程没变（选择器只换了时区，没换日期），Pacific/Auckland
    // （+12，8 月无夏令时）与 Asia/Kolkata（+5:30）对同一个日历日
    // （2026-08-04）算出的 since 因为偏移不同而必然不同——如果实现忽略
    // 了选中时区、退回宿主本地时区，两次切换后的 since 会变成同一个值
    // （宿主机自己的时区换算结果），下面两条断言至少有一条会先变红。
    expect(sinceAuckland).toBe('2026-08-03T12:00:00.000Z')
    expect(sinceKolkata).toBe('2026-08-03T18:30:00.000Z')
    expect(sinceAuckland).not.toBe(sinceKolkata)
  })

  // ---- 核心：夏令时切换日必须用 IANA 时区规则重新算"次日零点"，不能假设
  // 固定偏移或固定 24 小时 ----
  //
  // Pacific/Auckland 2026 年的春季夏令时切换（NZST +12 → NZDT +13）发生
  // 在 9 月 27 日凌晨——用 Intl 实测确认过这个日期（见开发记录），不是
  // 猜的。选中这一天，本地 00:00 到次日 00:00 只有 23 小时，因为切换
  // 当天凌晨 2 点直接跳到 3 点。如果实现改成"固定 +12 偏移"或者"加
  // 24*60*60*1000 毫秒"，算出来的 until 会晚 1 小时——这条断言就是钉死
  // 正确值，变异测试时把 zonedTimeToUtc 换成固定偏移版本，这里必须变红。
  it('夏令时春季切换日（跳过一小时，全天只有 23 小时）：until 比"固定偏移"算法早 1 小时', async () => {
    localStorage.setItem('magicd.statsTimezone', 'Pacific/Auckland')
    setupStore()
    const f = stubFetch({ statsByDay: [statsBucket({ danmakuCount: 1 })] })
    const wrapper = mount(Stats)
    await flushPromises()

    // 2026-09-27（月份从 0 开始，8 = 9 月）。
    const picked = new Date(2026, 8, 27).getTime()
    wrapper.findComponent(NDatePicker).vm.$emit('update:value', picked)
    await flushPromises()

    const statsCall = [...f.mock.calls]
      .reverse()
      .find((call) => String(call[0]).startsWith('/api/bindings/1/stats'))
    const url = new URL(String(statsCall![0]), 'http://localhost')
    expect(url.searchParams.get('since')).toBe('2026-09-26T12:00:00.000Z')
    // 固定 +12 偏移会算出 until = 2026-09-27T11:59:59.999Z（晚 1 小时）；
    // 正确值提前了整整一小时，因为切换当天当地时钟少走了一小时。
    expect(url.searchParams.get('until')).toBe('2026-09-27T10:59:59.999Z')
  })

  // ---- 秋季切换日（重复一小时，全天有 25 小时）：与春季对称的另一半 ----
  //
  // 只测春季不够——固定偏移方案在两天里一个偏早一个偏晚，只挑一天测有
  // 可能恰好蒙对（比如实现用了切换后的新偏移，春季碰巧算对、秋季必错，
  // 反之亦然），两天都测才能排除"蒙对一半"的可能。
  it('夏令时秋季切换日（重复一小时，全天有 25 小时）：until 比"固定偏移"算法晚 1 小时', async () => {
    localStorage.setItem('magicd.statsTimezone', 'Pacific/Auckland')
    setupStore()
    const f = stubFetch({ statsByDay: [statsBucket({ danmakuCount: 1 })] })
    const wrapper = mount(Stats)
    await flushPromises()

    // 2026-04-05（月份从 0 开始，3 = 4 月）。
    const picked = new Date(2026, 3, 5).getTime()
    wrapper.findComponent(NDatePicker).vm.$emit('update:value', picked)
    await flushPromises()

    const statsCall = [...f.mock.calls]
      .reverse()
      .find((call) => String(call[0]).startsWith('/api/bindings/1/stats'))
    const url = new URL(String(statsCall![0]), 'http://localhost')
    expect(url.searchParams.get('since')).toBe('2026-04-04T11:00:00.000Z')
    // 固定 +13（切换前用的偏移）会算出 until = 2026-04-05T10:59:59.999Z
    // （早 1 小时）；正确值因为当地时钟多走了一小时而晚了整整一小时。
    expect(url.searchParams.get('until')).toBe('2026-04-05T11:59:59.999Z')
  })

  it('「当日电池到账」卡片提示里会写出当前选中的时区名，不是只在选择器上显示', async () => {
    localStorage.setItem('magicd.statsTimezone', 'Pacific/Auckland')
    setupStore()
    stubFetch({ statsByDay: [statsBucket({ giftCoins: 100 })] })
    const wrapper = mount(Stats)
    await flushPromises()

    const card = wrapper.findAll('.stat-card').find((c) => c.text().includes('当日电池到账'))
    expect(card, '应该有「当日电池到账」这张卡片').toBeTruthy()
    expect(card!.text()).toContain('Pacific/Auckland')
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
