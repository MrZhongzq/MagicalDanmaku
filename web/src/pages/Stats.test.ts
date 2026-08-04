import { beforeEach, describe, expect, it, vi } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import { NRadioGroup, NStatistic } from 'naive-ui'

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
    ...overrides,
  }
}

/**
 * `Response.body` 只能读一次——每次调用都要返回一个新的 Response 实例，
 * 不能复用同一个对象（否则第二次 fetch 读到的是已经耗尽的流）。
 *
 * `statsFor` 按 URL 里的 `by=` 参数区分按日/按场次返回不同数据，
 * 默认两个维度都返回空数组。
 */
function stubFetch(
  opts: {
    activity?: unknown[]
    statsByDay?: unknown[]
    statsBySession?: unknown[]
  } = {},
) {
  const activity = opts.activity ?? []
  const statsByDay = opts.statsByDay ?? []
  const statsBySession = opts.statsBySession ?? []
  const f = vi.fn().mockImplementation((url: string) => {
    if (url.startsWith('/api/bindings/1/stats')) {
      const by = new URL(url, 'http://localhost').searchParams.get('by')
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
    // 7 张卡片：6 张真实数字 + 1 张一直悬空的盲盒盈亏，全部应为占位符
    expect(values).toEqual(['—', '—', '—', '—', '—', '—', '—'])
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

      const values = wrapper.findAllComponents(NStatistic).map((c) => c.props('value'))
      expect(values.every((v) => v === '—')).toBe(true)

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

  it('维度切换（按场次/按日）会重新请求聚合接口，带上对应的 by 参数，且明细表跟着变', async () => {
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

    // 卡片数字与明细表都要变成「按场次」的数据
    expect(wrapper.find('.stats-grid').text()).toContain('20')
    expect(wrapper.find('.bucket-card').text()).toContain('session-1')
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
