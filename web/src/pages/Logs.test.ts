import { beforeEach, describe, expect, it, vi } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import { NSwitch } from 'naive-ui'

// Logs 页最容易出问题的两处：
//   1. 后端推的是命名 SSE 事件（event: danmaku），不是默认 message——
//      这部分的正确性已经由 useEventStream.test.ts 单独钉住了；这里只
//      验证「组件把 useEventStream 的产出正确混进了列表顶部」。
//   2. 「清除」是悬空功能，点了不能假装删除了什么。
const messageMock = { success: vi.fn(), error: vi.fn(), warning: vi.fn(), info: vi.fn() }

vi.mock('naive-ui', async () => {
  const actual = await vi.importActual<typeof import('naive-ui')>('naive-ui')
  return {
    ...actual,
    useMessage: () => messageMock,
  }
})

const { default: Logs } = await import('./Logs.vue')
const { useBindingsStore } = await import('@/stores/bindings')

/** 与 useEventStream.test.ts 里同款的手动喂消息替身。 */
class FakeEventSource {
  static instances: FakeEventSource[] = []
  url: string
  onmessage: ((e: MessageEvent) => void) | null = null
  onerror: ((e: Event) => void) | null = null
  onopen: ((e: Event) => void) | null = null
  closed = false
  private listeners = new Map<string, ((e: MessageEvent) => void)[]>()

  constructor(url: string) {
    this.url = url
    FakeEventSource.instances.push(this)
  }
  addEventListener(type: string, fn: (e: MessageEvent) => void) {
    const arr = this.listeners.get(type) ?? []
    arr.push(fn)
    this.listeners.set(type, arr)
  }
  close() {
    this.closed = true
  }
  emit(type: string, data: unknown) {
    const e = new MessageEvent(type, { data: JSON.stringify(data) })
    for (const fn of this.listeners.get(type) ?? []) fn(e)
  }
}

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

const 历史记录 = [
  {
    id: 1,
    kind: 'event' as const,
    eventType: 'danmaku',
    actionType: '',
    ruleName: '',
    userUid: '999',
    userName: '老观众',
    detail: { text: '历史弹幕' },
    occurredAt: '2026-07-31T12:00:00Z',
  },
]

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

/** 默认的 fetch 桩：事件类型清单为空、历史记录固定返回一条。 */
function stubFetch(activity: unknown[] = 历史记录) {
  const f = vi.fn().mockImplementation((url: string) => {
    if (url === '/api/meta/event-types') {
      return Promise.resolve(
        ok([
          { value: 'danmaku', label: '弹幕' },
          { value: 'gift', label: '礼物' },
        ]),
      )
    }
    if (url.startsWith('/api/bindings/1/activity')) {
      return Promise.resolve(ok(activity))
    }
    return Promise.resolve(ok({ status: 'ok' }))
  })
  vi.stubGlobal('fetch', f)
  return f
}

describe('Logs 页', () => {
  beforeEach(() => {
    vi.unstubAllGlobals()
    FakeEventSource.instances = []
    vi.stubGlobal('EventSource', FakeEventSource)
    messageMock.success.mockClear()
    messageMock.error.mockClear()
    messageMock.warning.mockClear()
    messageMock.info.mockClear()
  })

  it('未选择直播间时提示先选择，不查询历史日志', async () => {
    setActivePinia(createPinia())
    const f = stubFetch()
    const wrapper = mount(Logs)
    await flushPromises()

    expect(wrapper.text()).toContain('请先在顶部选择一个直播间')
    // 事件类型清单是全局元数据，跟选没选直播间无关，加载它没问题；
    // 但没有绑定就不该去查询业务日志
    const hitActivity = f.mock.calls.some((call) => String(call[0]).includes('/activity'))
    expect(hitActivity).toBe(false)
  })

  it('选中直播间后加载历史日志并展示在表格里', async () => {
    setupStore()
    stubFetch()
    const wrapper = mount(Logs)
    await flushPromises()

    expect(wrapper.text()).toContain('老观众')
    expect(wrapper.text()).toContain('danmaku')
  })

  it('实时开关默认打开：SSE 命名事件混进列表顶部（排在历史记录前面）', async () => {
    setupStore()
    stubFetch()
    const wrapper = mount(Logs)
    await flushPromises()

    expect(FakeEventSource.instances).toHaveLength(1)
    expect(FakeEventSource.instances[0].url).toContain('/api/bindings/1/stream')

    FakeEventSource.instances[0].emit('gift', {
      id: 'evt-1',
      type: 'gift',
      roomId: '123',
      timestamp: '2026-07-31T12:30:00Z',
      payload: { User: { UID: '1', Username: '土豪' }, GiftName: '飞机' },
    })
    await flushPromises()

    const rows = wrapper.findAll('tr').map((r) => r.text())
    const giftRowIndex = rows.findIndex((t) => t.includes('土豪'))
    const historyRowIndex = rows.findIndex((t) => t.includes('老观众'))
    expect(giftRowIndex).toBeGreaterThan(-1)
    expect(historyRowIndex).toBeGreaterThan(-1)
    // 实时行必须排在历史行前面
    expect(giftRowIndex).toBeLessThan(historyRowIndex)
  })

  it('关掉实时开关后断开 SSE 连接，且新事件不再显示', async () => {
    setupStore()
    stubFetch()
    const wrapper = mount(Logs)
    await flushPromises()

    const es = FakeEventSource.instances[0]
    const sw = wrapper.findComponent(NSwitch)
    sw.vm.$emit('update:value', false)
    await flushPromises()

    expect(es.closed).toBe(true)

    es.emit('gift', {
      id: 'evt-2',
      type: 'gift',
      roomId: '123',
      timestamp: '2026-07-31T12:31:00Z',
      payload: { User: { UID: '2', Username: '关不掉也不该显示' } },
    })
    await flushPromises()

    expect(wrapper.text()).not.toContain('关不掉也不该显示')
  })

  it('点击清除只提示「待后端支持」，不发送任何删除请求', async () => {
    setupStore()
    const f = stubFetch()
    const wrapper = mount(Logs)
    await flushPromises()
    const callsBefore = f.mock.calls.length

    const clearBtn = wrapper.findAll('button').find((b) => b.text() === '清除')
    expect(clearBtn).toBeTruthy()
    await clearBtn!.trigger('click')
    await flushPromises()

    expect(messageMock.warning).toHaveBeenCalledWith('后端尚未提供删除业务日志的接口')
    // 没有多发出任何请求（尤其不能是 DELETE）
    expect(f.mock.calls.length).toBe(callsBefore)
    const hasDelete = f.mock.calls.some(
      (call) => (call[1] as RequestInit | undefined)?.method === 'DELETE',
    )
    expect(hasDelete).toBe(false)
  })

  it('关键词只在已加载的记录里过滤，且界面说明了这一点', async () => {
    setupStore()
    stubFetch([
      { ...历史记录[0], id: 1, userName: '甲', detail: { text: 'hello' } },
      { ...历史记录[0], id: 2, userName: '乙', detail: { text: 'world' } },
    ])
    const wrapper = mount(Logs)
    await flushPromises()

    expect(wrapper.text()).toContain('只在已加载的记录里搜')
    expect(wrapper.text()).toContain('甲')
    expect(wrapper.text()).toContain('乙')

    const input = wrapper.find('input[placeholder="按关键词搜索"]')
    await input.setValue('甲')
    await flushPromises()

    expect(wrapper.text()).toContain('甲')
    expect(wrapper.text()).not.toContain('乙')
  })

  it('时间范围填反时，后端 422 的原文原样显示，不包装成笼统提示', async () => {
    setupStore()
    const backendMessage =
      'since 不能晚于 until（since=2026-07-31T12:00:00Z, until=2026-07-30T12:00:00Z）'
    const f = vi.fn().mockImplementation((url: string) => {
      if (url === '/api/meta/event-types') return Promise.resolve(ok([]))
      if (url.startsWith('/api/bindings/1/activity')) {
        // 第一次加载（无过滤条件）成功，第二次点「查询」时模拟时间反了
        return Promise.resolve(err(422, backendMessage))
      }
      return Promise.resolve(ok({ status: 'ok' }))
    })
    vi.stubGlobal('fetch', f)

    const wrapper = mount(Logs)
    await flushPromises()

    const queryBtn = wrapper.findAll('button').find((b) => b.text() === '查询')
    await queryBtn!.trigger('click')
    await flushPromises()

    expect(messageMock.error).toHaveBeenCalledWith(backendMessage)
  })
})
