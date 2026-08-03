import { beforeEach, describe, expect, it, vi } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import { NDatePicker, NSelect, NSwitch } from 'naive-ui'

// Logs 页最容易出问题的几处：
//   1. 后端推的是命名 SSE 事件（event: danmaku），不是默认 message——
//      这部分的正确性已经由 useEventStream.test.ts 单独钉住了；这里只
//      验证「组件把 useEventStream 的产出正确混进了列表顶部」。
//   2. 「清除」是真删库操作，必须二次确认，确认前绝不能发 DELETE。
//
// 用 vi.mock 顶掉 naive-ui 的 useDialog/useMessage，做法与 Accounts.test.ts
// 删除账号的测试一致：mock 拿到 dialog.warning 收到的 onPositiveClick 之后
// 手动调用它，等价于「用户点了确认框里的确认」，但是确定性的，不用去挂
// 真的 NDialogProvider 处理 Teleport 与动画时序。
const warningMock = vi.fn()
const messageMock = { success: vi.fn(), error: vi.fn(), warning: vi.fn(), info: vi.fn() }

vi.mock('naive-ui', async () => {
  const actual = await vi.importActual<typeof import('naive-ui')>('naive-ui')
  return {
    ...actual,
    useDialog: () => ({ warning: warningMock }),
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
  liveStatus: 'unknown',
  liveCheckedAt: null,
  anchorUid: '',
  anchorName: '',
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
    warningMock.mockClear()
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

  function deleteCall(f: ReturnType<typeof vi.fn>) {
    return f.mock.calls.find((call) => (call[1] as RequestInit | undefined)?.method === 'DELETE')
  }

  it('点击清除先弹二次确认，确认之前不发 DELETE 请求', async () => {
    setupStore()
    const f = stubFetch()
    const wrapper = mount(Logs)
    await flushPromises()

    const clearBtn = wrapper.findAll('button').find((b) => b.text() === '清除')
    expect(clearBtn).toBeTruthy()
    await clearBtn!.trigger('click')
    await flushPromises()

    expect(warningMock).toHaveBeenCalledTimes(1)
    expect(deleteCall(f)).toBeUndefined()
  })

  it('未设置时间范围时确认清除：请求带 all=1，成功后把 deleted 数字显示出来', async () => {
    setupStore()
    const f = vi.fn().mockImplementation((url: string, init?: RequestInit) => {
      if (url === '/api/meta/event-types') return Promise.resolve(ok([]))
      if (url.startsWith('/api/bindings/1/activity') && init?.method === 'DELETE') {
        return Promise.resolve(ok({ deleted: 42 }))
      }
      if (url.startsWith('/api/bindings/1/activity')) return Promise.resolve(ok(历史记录))
      return Promise.resolve(ok({ status: 'ok' }))
    })
    vi.stubGlobal('fetch', f)

    const wrapper = mount(Logs)
    await flushPromises()

    const clearBtn = wrapper.findAll('button').find((b) => b.text() === '清除')
    await clearBtn!.trigger('click')
    await flushPromises()

    // 模拟用户在确认框里点击「清除」
    const opts = warningMock.mock.calls[0][0] as {
      onPositiveClick: () => void
      content: () => unknown
    }
    // 未设置时间范围，确认框文案要说明将清除全部历史
    expect(JSON.stringify(opts.content())).toContain('全部历史')

    opts.onPositiveClick()
    await flushPromises()

    const call = deleteCall(f)
    expect(call).toBeTruthy()
    expect(String(call![0])).toContain('all=1')
    expect(messageMock.success).toHaveBeenCalledWith('已清除 42 条业务日志')
    expect(wrapper.text()).toContain('上次清除了 42 条')
  })

  it('设置了时间范围时确认清除：请求带 since/until，不带 all', async () => {
    setupStore()
    const f = vi.fn().mockImplementation((url: string, init?: RequestInit) => {
      if (url === '/api/meta/event-types') return Promise.resolve(ok([]))
      if (url.startsWith('/api/bindings/1/activity') && init?.method === 'DELETE') {
        return Promise.resolve(ok({ deleted: 5 }))
      }
      if (url.startsWith('/api/bindings/1/activity')) return Promise.resolve(ok(历史记录))
      return Promise.resolve(ok({ status: 'ok' }))
    })
    vi.stubGlobal('fetch', f)

    const wrapper = mount(Logs)
    await flushPromises()

    const since = new Date('2026-07-30T00:00:00Z').getTime()
    const until = new Date('2026-07-31T00:00:00Z').getTime()
    const picker = wrapper.findComponent(NDatePicker)
    picker.vm.$emit('update:value', [since, until])
    await flushPromises()

    const clearBtn = wrapper.findAll('button').find((b) => b.text() === '清除')
    await clearBtn!.trigger('click')
    await flushPromises()

    const opts = warningMock.mock.calls[0][0] as {
      onPositiveClick: () => void
      content: () => unknown
    }
    // 有时间范围时不该再说「全部历史」
    expect(JSON.stringify(opts.content())).not.toContain('全部历史')

    opts.onPositiveClick()
    await flushPromises()

    const call = deleteCall(f)
    expect(call).toBeTruthy()
    const url = String(call![0])
    expect(url).toContain('since=')
    expect(url).toContain('until=')
    expect(url).not.toContain('all=1')
    expect(messageMock.success).toHaveBeenCalledWith('已清除 5 条业务日志')
  })

  it('筛选了事件类型但没设置时间范围时，确认框会说明清除不支持按类型筛选', async () => {
    setupStore()
    const f = vi.fn().mockImplementation((url: string) => {
      if (url === '/api/meta/event-types') {
        return Promise.resolve(ok([{ value: 'danmaku', label: '弹幕' }]))
      }
      if (url.startsWith('/api/bindings/1/activity')) return Promise.resolve(ok(历史记录))
      return Promise.resolve(ok({ status: 'ok' }))
    })
    vi.stubGlobal('fetch', f)

    const wrapper = mount(Logs)
    await flushPromises()

    const eventTypeSelect = wrapper.findAllComponents(NSelect)[1]
    eventTypeSelect.vm.$emit('update:value', 'danmaku')
    await flushPromises()

    const clearBtn = wrapper.findAll('button').find((b) => b.text() === '清除')
    await clearBtn!.trigger('click')
    await flushPromises()

    const opts = warningMock.mock.calls[0][0] as { content: () => unknown }
    expect(JSON.stringify(opts.content())).toContain('不支持按类型')
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

// P5-3：与其余页面同一条要求——日志页此前也看不出正在看哪个绑定的日志。
// 日志页只读，没有专门的权限点，选择器不带 requiredPerm。
describe('Logs 页：正文里也有账号+直播间选择器', () => {
  it('页面渲染 BindingSelector，不要求特定权限', async () => {
    setupStore()
    stubFetch([])
    const wrapper = mount(Logs)
    await flushPromises()

    const { default: BindingSelector } = await import('@/components/BindingSelector.vue')
    const selector = wrapper.findComponent(BindingSelector)
    expect(selector.exists()).toBe(true)
    expect(selector.props('requiredPerm')).toBeUndefined()
  })
})
