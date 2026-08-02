import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { useEventStream } from './useEventStream'

/** 一个能手动喂消息的 EventSource 替身。 */
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
  /** 测试用：模拟服务端推一条命名事件。 */
  emit(type: string, data: unknown) {
    const e = new MessageEvent(type, { data: JSON.stringify(data) })
    for (const fn of this.listeners.get(type) ?? []) fn(e)
  }
}

describe('useEventStream', () => {
  beforeEach(() => {
    FakeEventSource.instances = []
    vi.stubGlobal('EventSource', FakeEventSource)
  })
  afterEach(() => {
    vi.unstubAllGlobals()
  })

  it('订阅的是这个绑定的流', () => {
    useEventStream(7)
    expect(FakeEventSource.instances[0].url).toContain('/api/bindings/7/stream')
  })

  // 后端推的是命名事件（event: danmaku\ndata: {...}），不是默认的 message
  it('收到命名事件后追加进列表', () => {
    const s = useEventStream(1)
    FakeEventSource.instances[0].emit('danmaku', { id: 'a', type: 'danmaku', payload: {} })
    expect(s.events.value).toHaveLength(1)
  })

  // 一个开着一整场直播的页面会收到几万条。不设上限的话内存一直涨，
  // 浏览器最后会卡死——而用户只是把日志页开着没关。
  it('超过上限时丢掉最旧的，不是无限增长', () => {
    const s = useEventStream(1, { max: 3 })
    for (let i = 0; i < 5; i++) {
      FakeEventSource.instances[0].emit('danmaku', { id: String(i), type: 'danmaku', payload: {} })
    }
    expect(s.events.value).toHaveLength(3)
    // 日志页新的在前：喂 0..4 之后，留下的是 4,3,2，最新一条排在最前面
    expect(s.events.value[0].id).toBe('4')
  })

  it('close 之后不再接收，且底层连接真的关了', () => {
    const s = useEventStream(1)
    const es = FakeEventSource.instances[0]
    s.close()
    expect(es.closed).toBe(true)
    es.emit('danmaku', { id: 'x', type: 'danmaku', payload: {} })
    expect(s.events.value).toHaveLength(0)
  })

  // 覆盖 event/type.go 里除 danmaku 之外的全部类型，确认每一种都真的
  // 挂了 addEventListener——漏一种的表现是那类事件永远不显示，且不报错。
  it('对 event/type.go 里定义的每一种事件类型都能收到', () => {
    const s = useEventStream(1)
    const es = FakeEventSource.instances[0]
    const types = [
      'danmaku',
      'super_chat',
      'super_chat_delete',
      'gift',
      'gift_combo',
      'guard_buy',
      'user_enter',
      'user_follow',
      'user_share',
      'user_like',
      'live_start',
      'live_stop',
      'room_change',
      'user_blocked',
      'online_rank_update',
      'room_stats_update',
      'battle',
      'pk_visit_from_opponent',
      'pk_visit_to_opponent',
      'manual',
      'unknown',
    ]
    for (const t of types) {
      es.emit(t, { id: t, type: t, payload: {} })
    }
    expect(s.events.value).toHaveLength(types.length)
  })
})
