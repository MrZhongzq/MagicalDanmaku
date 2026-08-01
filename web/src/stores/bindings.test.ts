import { beforeEach, describe, expect, it, vi } from 'vitest'
import { createPinia, setActivePinia } from 'pinia'
import { useBindingsStore } from './bindings'

const 甲 = {
  id: 1,
  accountId: 1,
  accountName: '小号',
  roomId: '111',
  enabled: true,
  ruleCount: 2,
  permissions: ['rule:read'],
}
const 乙 = {
  id: 2,
  accountId: 1,
  accountName: '小号',
  roomId: '222',
  enabled: false,
  ruleCount: 0,
  permissions: [],
}

function ok(body: unknown) {
  return new Response(JSON.stringify(body), {
    status: 200,
    headers: { 'Content-Type': 'application/json' },
  })
}

describe('useBindingsStore', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    vi.unstubAllGlobals()
    localStorage.clear()
  })

  it('刷新后自动选中第一个绑定', async () => {
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue(ok([甲, 乙])))
    const s = useBindingsStore()
    await s.refresh()
    expect(s.current?.id).toBe(1)
  })

  // 选中的绑定要记住：开播时会在几个页面之间来回切，
  // 每次回来都重置成第一个非常烦人
  it('记住上次选中的绑定，刷新页面后仍然选中它', async () => {
    // 这里刷新会调两次 fetch（s 一次、s2 一次）。Response 的 body 只能读一次，
    // mockResolvedValue 返回的是同一个实例，第二次 .json() 会报「body 已读过」；
    // 用 mockImplementation 让每次调用都拿到一个新的 Response。
    vi.stubGlobal(
      'fetch',
      vi.fn().mockImplementation(() => Promise.resolve(ok([甲, 乙]))),
    )
    const s = useBindingsStore()
    await s.refresh()
    s.select(2)

    setActivePinia(createPinia())
    const s2 = useBindingsStore()
    await s2.refresh()
    expect(s2.current?.id).toBe(2)
  })

  // 记住的那个绑定可能已经被删了，或者授权被撤了变成不可见
  it('记住的绑定已经不在列表里时回退到第一个，不是留一个空选中', async () => {
    localStorage.setItem('magicd.currentBinding', '999')
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue(ok([甲])))
    const s = useBindingsStore()
    await s.refresh()
    expect(s.current?.id).toBe(1)
  })

  it('列表为空时 current 是 null，不该抛异常', async () => {
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue(ok([])))
    const s = useBindingsStore()
    await s.refresh()
    expect(s.current).toBeNull()
  })
})
