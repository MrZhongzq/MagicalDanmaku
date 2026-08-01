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

function err(status: number, message: string) {
  return new Response(JSON.stringify({ error: message }), {
    status,
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

  // 回退发生后要把新选中的那个写回 localStorage，不然下次刷新页面
  // （这次 999 已经不在了）会读到旧的 999 又走一遍回退分支
  it('回退到第一个之后，把这个新选中写回 localStorage', async () => {
    localStorage.setItem('magicd.currentBinding', '999')
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue(ok([甲])))
    const s = useBindingsStore()
    await s.refresh()
    expect(localStorage.getItem('magicd.currentBinding')).toBe('1')
  })

  it('列表为空时 current 是 null，不该抛异常', async () => {
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue(ok([])))
    const s = useBindingsStore()
    await s.refresh()
    expect(s.current).toBeNull()
  })

  // ---- 全分支终审第 2 条：refresh() 失败不能静默 ----
  //
  // 原来只有 finally 没有 catch，GET /api/bindings 非 401 失败时，顶部
  // 选择器渲染 placeholder="没有可用的直播间"，与「这个账号确实没绑过
  // 直播间」在界面上完全无法区分，一条错误提示都不出现。
  it('刷新失败时记录后端原文到 loadError，不能静默吞掉', async () => {
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue(err(500, '数据库连不上')))
    const s = useBindingsStore()
    await s.refresh()
    expect(s.loadError).toBe('数据库连不上')
    expect(s.list).toEqual([])
  })

  it('刷新成功后 loadError 归 null，不会残留上一次的错误', async () => {
    vi.stubGlobal(
      'fetch',
      vi.fn().mockImplementation(() => Promise.resolve(err(500, '第一次失败'))),
    )
    const s = useBindingsStore()
    await s.refresh()
    expect(s.loadError).toBe('第一次失败')

    vi.stubGlobal('fetch', vi.fn().mockResolvedValue(ok([甲])))
    await s.refresh()
    expect(s.loadError).toBeNull()
  })
})
