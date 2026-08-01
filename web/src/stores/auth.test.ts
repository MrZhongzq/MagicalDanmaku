import { beforeEach, describe, expect, it, vi } from 'vitest'
import { createPinia, setActivePinia } from 'pinia'
import { useAuthStore } from './auth'

function ok(body: unknown) {
  return new Response(JSON.stringify(body), {
    status: 200,
    headers: { 'Content-Type': 'application/json' },
  })
}

describe('useAuthStore', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    vi.unstubAllGlobals()
  })

  it('登录成功后记住当前用户', async () => {
    vi.stubGlobal(
      'fetch',
      vi.fn().mockResolvedValue(ok({ id: 1, username: '张三', isAdmin: false, createdAt: '' })),
    )
    const auth = useAuthStore()
    await auth.login('张三', 'pw')
    expect(auth.user?.username).toBe('张三')
    expect(auth.isLoggedIn).toBe(true)
  })

  it('登录用 POST——会话认证靠 SameSite=Lax 替代 CSRF token，改状态的接口不能用 GET', async () => {
    const f = vi
      .fn()
      .mockResolvedValue(ok({ id: 1, username: '张三', isAdmin: false, createdAt: '' }))
    vi.stubGlobal('fetch', f)
    await useAuthStore().login('张三', 'pw')
    expect(f.mock.calls[0][1]).toMatchObject({ method: 'POST' })
  })

  it('登出后清空用户', async () => {
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue(new Response(null, { status: 204 })))
    const auth = useAuthStore()
    auth.user = { id: 1, username: '张三', isAdmin: false, createdAt: '' }
    await auth.logout()
    expect(auth.user).toBeNull()
  })

  // fetchMe 是刷新页面后恢复会话的唯一手段：Cookie 是 HttpOnly，
  // 前端读不到，只能问后端「我是谁」。
  // fetchMe 的响应是 {user, memberships}，不是裸的 User
  it('fetchMe 成功时从 user 字段里取，不是把整个响应当成用户', async () => {
    vi.stubGlobal(
      'fetch',
      vi.fn().mockResolvedValue(
        ok({
          user: { id: 1, username: '张三', isAdmin: true, createdAt: '' },
          memberships: [
            { bindingId: 1, accountName: '小号', roomId: '111', permissions: ['rule:read'] },
          ],
        }),
      ),
    )
    const auth = useAuthStore()
    await auth.fetchMe()
    expect(auth.user?.username).toBe('张三')
    // isAdmin 拿错的话管理员会丢掉管理入口，而且不报任何错
    expect(auth.user?.isAdmin).toBe(true)
    expect(auth.memberships).toHaveLength(1)
  })

  it('fetchMe 拿到 401 时把用户置空而不是抛出去', async () => {
    vi.stubGlobal(
      'fetch',
      vi.fn().mockResolvedValue(
        new Response(JSON.stringify({ error: '未登录' }), {
          status: 401,
          headers: { 'Content-Type': 'application/json' },
        }),
      ),
    )
    const auth = useAuthStore()
    await expect(auth.fetchMe()).resolves.toBeUndefined()
    expect(auth.user).toBeNull()
  })

  it('管理员对任何绑定都有全部权限', () => {
    const auth = useAuthStore()
    auth.user = { id: 1, username: '管理员', isAdmin: true, createdAt: '' }
    expect(auth.hasPerm({ permissions: [] } as never, 'rule:write')).toBe(true)
  })

  it('普通用户按绑定自带的 permissions 判定', () => {
    const auth = useAuthStore()
    auth.user = { id: 2, username: '李四', isAdmin: false, createdAt: '' }
    const b = { permissions: ['rule:read'] } as never
    expect(auth.hasPerm(b, 'rule:read')).toBe(true)
    expect(auth.hasPerm(b, 'rule:write')).toBe(false)
  })
})
