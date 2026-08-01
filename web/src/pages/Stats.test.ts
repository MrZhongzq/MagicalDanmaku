import { beforeEach, describe, expect, it, vi } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'

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
} = {
  id: 1,
  accountId: 1,
  accountName: '小号',
  roomId: '123',
  enabled: true,
  ruleCount: 0,
  permissions: ['event:read'],
}

function ok(body: unknown, status = 200) {
  return new Response(JSON.stringify(body), {
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

/**
 * `Response.body` 只能读一次——每次调用都要返回一个新的 Response 实例，
 * 不能复用同一个对象（否则第二次 fetch 读到的是已经耗尽的流）。
 */
function stubFetch(activity: unknown[]) {
  const f = vi.fn().mockImplementation((url: string) => {
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
    const f = stubFetch([])
    const wrapper = mount(Stats)
    await flushPromises()

    expect(wrapper.text()).toContain('请先在顶部选择一个直播间')
    expect(f).not.toHaveBeenCalled()
  })

  it('选中直播间后渲染全部统计卡片与两个维度选项，卡片数字是占位符', async () => {
    setupStore()
    stubFetch([])
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

    const grid = wrapper.find('.stats-grid')
    expect(grid.exists()).toBe(true)
    // 7 张卡片，每张一个占位符
    expect(grid.text().match(/—/g)?.length).toBe(7)
  })

  it('盲盒盈亏卡片标注双重悬空', async () => {
    setupStore()
    stubFetch([])
    const wrapper = mount(Stats)
    await flushPromises()

    expect(wrapper.text()).toContain('双重悬空')
  })

  it('页面顶部说明统计需要后端聚合接口', async () => {
    setupStore()
    stubFetch([])
    const wrapper = mount(Stats)
    await flushPromises()

    expect(wrapper.text()).toContain('统计需要后端聚合接口')
  })

  // ---- 最重要的一条：绝不能把 500 条原始行算成一个假的统计数字 ----
  it('后端 /activity 返回 500 条原始行时，统计卡片仍然只显示占位符，不是 500 或任何由这 500 条算出来的数', async () => {
    setupStore()
    stubFetch(make500Activity())
    const wrapper = mount(Stats)
    await flushPromises()

    // 展开「最近活动预览」，让组件真的把这 500 条拉回来、渲染出来——
    // 确认即便数据已经在页面里，统计卡片区域依然纹丝不动
    const expandBtn = wrapper.findAll('button').find((b) => b.text() === '展开')
    expect(expandBtn).toBeTruthy()
    await expandBtn!.trigger('click')
    await flushPromises()

    // 预览区确实拉到了 500 条（证明 mock 生效、组件真的请求并渲染了数据）
    expect(wrapper.text()).toContain('本次采样加载了 500 条原始行')

    // 但统计卡片区域必须只有占位符，不能出现 500、也不能出现任何数字
    const grid = wrapper.find('.stats-grid')
    expect(grid.text()).not.toContain('500')
    expect(grid.text().match(/—/g)?.length).toBe(7)
    // 卡片区域不应该出现活动条数量级的数字（500 本身，或任何三位数）——
    // 悬空清单编号（如「第 14、15 条」）里出现一两位数没关系，那不是统计数字
    expect(/\b\d{3,}\b/.test(grid.text())).toBe(false)
  })

  it('最近活动预览默认折叠，且明确标注「不是统计数字」', async () => {
    setupStore()
    const f = stubFetch(make500Activity())
    const wrapper = mount(Stats)
    await flushPromises()

    // 默认折叠：还没点「展开」之前不应该发出 /activity 请求
    const hitActivity = f.mock.calls.some((call) => String(call[0]).includes('/activity'))
    expect(hitActivity).toBe(false)
    expect(wrapper.text()).toContain('这不是统计数字')
  })

  it('维度切换（按场次/按日）可以点击，且不触发任何网络请求', async () => {
    setupStore()
    const f = stubFetch([])
    const wrapper = mount(Stats)
    await flushPromises()
    const callsBefore = f.mock.calls.length

    const sessionBtn = wrapper
      .findAll('.n-radio-button, label')
      .find((el) => el.text().includes('按场次'))
    expect(sessionBtn).toBeTruthy()
    await sessionBtn!.trigger('click')
    await flushPromises()

    expect(f.mock.calls.length).toBe(callsBefore)
  })
})
