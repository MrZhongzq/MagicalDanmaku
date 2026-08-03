import { beforeEach, describe, expect, it, vi } from 'vitest'
import { mount, flushPromises, DOMWrapper } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import type { Permission } from '@/api'

// Admin 页三块：改自己的密码、用户管理（仅管理员）、授权管理（委托给
// MemberEditor）。MemberEditor 自己有完整的测试（PRESETS 展开、编辑/撤销），
// 这里把它 stub 掉，只验证 Admin.vue 自己负责的部分：
//
//   一、改自己密码必须带旧密码，管理员也不例外——这是 P4-1 Task 6 真实
//      修过的一个安全缺陷（管理员改自己密码曾经跳过旧密码校验）。
//   二、管理员重置他人密码不能带旧密码字段——这是与「一」完全不同的表单。
//   三、缺 member:manage 时提示语必须是「需要 member:manage 权限，请让
//      管理员授予」，不能写成「你不是所有者」——那个判断是错的。
//   四、删用户要走 dialog.warning 二次确认。
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

const { default: Admin } = await import('./Admin.vue')
const { useAuthStore } = await import('@/stores/auth')
const { useBindingsStore } = await import('@/stores/bindings')

function ok(body: unknown) {
  return new Response(JSON.stringify(body), {
    status: 200,
    headers: { 'Content-Type': 'application/json' },
  })
}

function mountAdmin(attachTo?: Element) {
  return mount(Admin, {
    attachTo,
    global: {
      // MemberEditor 有自己独立的测试文件，这里 stub 掉避免重复发请求、
      // 重复断言，Admin.vue 自己只负责「要不要显示它」这一层判断。
      stubs: { MemberEditor: true },
    },
  })
}

beforeEach(() => {
  setActivePinia(createPinia())
  vi.unstubAllGlobals()
  warningMock.mockClear()
  messageMock.success.mockClear()
  messageMock.error.mockClear()
  messageMock.warning.mockClear()
  messageMock.info.mockClear()
})

describe('Admin 一、改自己的密码：旧密码必填，管理员也不例外', () => {
  it('普通用户不填旧密码时只提示，不发请求', async () => {
    const auth = useAuthStore()
    auth.user = { id: 1, username: '张三', isAdmin: false, createdAt: '' }
    const f = vi.fn().mockImplementation(() => Promise.resolve(ok([])))
    vi.stubGlobal('fetch', f)

    const wrapper = mountAdmin()
    await flushPromises()

    await wrapper.find('input[placeholder="至少 8 个字符"]').setValue('newpassword123')
    const btn = wrapper.findAll('button').find((b) => b.text() === '修改密码')
    await btn!.trigger('click')
    await flushPromises()

    expect(messageMock.warning).toHaveBeenCalledWith('请输入旧密码')
    const passwordCalls = f.mock.calls.filter((c) => String(c[0]).includes('/password'))
    expect(passwordCalls).toHaveLength(0)
  })

  it('管理员改自己的密码同样要求带旧密码——这正是 P4-1 Task 6 修过的那处顺序缺陷', async () => {
    const auth = useAuthStore()
    auth.user = { id: 1, username: '管理员', isAdmin: true, createdAt: '' }
    const f = vi.fn().mockImplementation(() => Promise.resolve(ok([])))
    vi.stubGlobal('fetch', f)

    const wrapper = mountAdmin()
    await flushPromises()

    // 只填新密码，不填旧密码——即便当前用户是管理员，也必须被挡下来
    await wrapper.find('input[placeholder="至少 8 个字符"]').setValue('newpassword123')
    const btn = wrapper.findAll('button').find((b) => b.text() === '修改密码')
    await btn!.trigger('click')
    await flushPromises()

    expect(messageMock.warning).toHaveBeenCalledWith('请输入旧密码')
    const passwordCalls = f.mock.calls.filter((c) => String(c[0]).includes('/password'))
    expect(passwordCalls).toHaveLength(0)
  })

  it('带齐旧密码和新密码后，POST body 里同时有 oldPassword 与 newPassword，路径是自己的用户名', async () => {
    const auth = useAuthStore()
    auth.user = { id: 1, username: '管理员', isAdmin: true, createdAt: '' }
    let sentUrl = ''
    let sentBody: unknown = null
    const f = vi.fn().mockImplementation((url: string, init?: RequestInit) => {
      if (url.endsWith('/password') && init?.method === 'POST') {
        sentUrl = url
        sentBody = JSON.parse(init.body as string)
      }
      return Promise.resolve(ok([]))
    })
    vi.stubGlobal('fetch', f)

    const wrapper = mountAdmin()
    await flushPromises()

    await wrapper.find('input[placeholder="旧密码"]').setValue('oldpassword1')
    await wrapper.find('input[placeholder="至少 8 个字符"]').setValue('newpassword123')
    const btn = wrapper.findAll('button').find((b) => b.text() === '修改密码')
    await btn!.trigger('click')
    await flushPromises()

    expect(sentUrl).toBe('/api/users/%E7%AE%A1%E7%90%86%E5%91%98/password')
    expect(sentBody).toEqual({ oldPassword: 'oldpassword1', newPassword: 'newpassword123' })
  })
})

describe('Admin 二、用户管理：仅管理员可见，重置他人密码不带旧密码字段', () => {
  it('非管理员看不到用户管理区', async () => {
    const auth = useAuthStore()
    auth.user = { id: 2, username: '李四', isAdmin: false, createdAt: '' }
    vi.stubGlobal(
      'fetch',
      vi.fn().mockImplementation(() => Promise.resolve(ok([]))),
    )

    const wrapper = mountAdmin()
    await flushPromises()

    expect(wrapper.text()).not.toContain('用户管理')
  })

  it('管理员能看到用户列表并新建用户', async () => {
    const auth = useAuthStore()
    auth.user = { id: 1, username: '管理员', isAdmin: true, createdAt: '' }
    let createBody: unknown = null
    const f = vi.fn().mockImplementation((url: string, init?: RequestInit) => {
      if (url === '/api/users' && init?.method === 'GET') {
        return Promise.resolve(
          ok([{ id: 2, username: '李四', isAdmin: false, createdAt: '2026-08-01T00:00:00Z' }]),
        )
      }
      if (url === '/api/users' && init?.method === 'POST') {
        createBody = JSON.parse(init.body as string)
        return Promise.resolve(ok({ id: 3, username: '王五', isAdmin: false, createdAt: '' }))
      }
      return Promise.resolve(ok([]))
    })
    vi.stubGlobal('fetch', f)

    const wrapper = mountAdmin()
    await flushPromises()

    expect(wrapper.text()).toContain('用户管理')
    expect(wrapper.text()).toContain('李四')

    await wrapper.find('input[placeholder="用户名"]').setValue('王五')
    await wrapper.find('input[placeholder="初始密码（至少 8 个字符）"]').setValue('somepassword1')
    const btn = wrapper.findAll('button').find((b) => b.text() === '创建用户')
    await btn!.trigger('click')
    await flushPromises()

    expect(createBody).toEqual({ username: '王五', password: 'somepassword1', isAdmin: false })
  })

  it('管理员重置他人密码：POST body 只有 newPassword，没有 oldPassword 这个字段', async () => {
    const auth = useAuthStore()
    auth.user = { id: 1, username: '管理员', isAdmin: true, createdAt: '' }
    let sentBody: Record<string, unknown> | null = null
    const f = vi.fn().mockImplementation((url: string, init?: RequestInit) => {
      if (url === '/api/users' && init?.method === 'GET') {
        return Promise.resolve(
          ok([{ id: 2, username: '李四', isAdmin: false, createdAt: '2026-08-01T00:00:00Z' }]),
        )
      }
      if (url.endsWith('/password') && init?.method === 'POST') {
        sentBody = JSON.parse(init.body as string)
      }
      return Promise.resolve(ok({ status: 'ok' }))
    })
    vi.stubGlobal('fetch', f)

    // NModal 默认把内容 Teleport 到 document.body，不在 wrapper.element 的
    // 子树里——wrapper.find 找不到它，必须真的挂到 document 上，再用
    // DOMWrapper 包一层 document.body 去找被 Teleport 出去的节点。
    const container = document.createElement('div')
    document.body.appendChild(container)
    const wrapper = mountAdmin(container)
    await flushPromises()

    const resetBtn = wrapper.findAll('button').find((b) => b.text() === '重置密码')
    await resetBtn!.trigger('click')
    await flushPromises()

    const body = new DOMWrapper(document.body)
    // 弹窗里只填新密码——压根没有旧密码输入框可填
    const modalPasswordInput = body.find('input[placeholder="新密码（至少 8 个字符）"]')
    expect(modalPasswordInput.exists()).toBe(true)
    await modalPasswordInput.setValue('resetpassword1')
    const confirmBtn = body.findAll('button').find((b) => b.text() === '确认重置')
    await confirmBtn!.trigger('click')
    await flushPromises()

    expect(sentBody).toEqual({ newPassword: 'resetpassword1' })
    expect(sentBody).not.toHaveProperty('oldPassword')

    wrapper.unmount()
    container.remove()
  })

  it('删除用户先弹确认，确认之前不发 DELETE；确认之后才发', async () => {
    const auth = useAuthStore()
    auth.user = { id: 1, username: '管理员', isAdmin: true, createdAt: '' }
    const f = vi.fn().mockImplementation((url: string, init?: RequestInit) => {
      if (url === '/api/users' && init?.method === 'GET') {
        return Promise.resolve(
          ok([{ id: 2, username: '李四', isAdmin: false, createdAt: '2026-08-01T00:00:00Z' }]),
        )
      }
      return Promise.resolve(ok({ status: 'ok' }))
    })
    vi.stubGlobal('fetch', f)

    const wrapper = mountAdmin()
    await flushPromises()

    const deleteBtn = wrapper.findAll('button').find((b) => b.text() === '删除')
    await deleteBtn!.trigger('click')
    await flushPromises()

    expect(warningMock).toHaveBeenCalledTimes(1)
    const deleteCallsBefore = f.mock.calls.filter(
      (c) => (c[1] as RequestInit | undefined)?.method === 'DELETE',
    ).length
    expect(deleteCallsBefore).toBe(0)

    const opts = warningMock.mock.calls[0][0] as { onPositiveClick: () => void }
    opts.onPositiveClick()
    await flushPromises()

    const deleteCallsAfter = f.mock.calls.filter(
      (c) => (c[1] as RequestInit | undefined)?.method === 'DELETE',
    ).length
    expect(deleteCallsAfter).toBe(1)
  })
})

describe('Admin 三、授权管理：门禁判断委托给 auth.hasPerm，文案不能写成「你不是所有者」', () => {
  function bindingWith(permissions: Permission[]) {
    return {
      id: 1,
      accountId: 1,
      accountName: '小号',
      roomId: '123',
      enabled: true,
      ruleCount: 0,
      permissions,
      liveStatus: 'unknown' as const,
      liveCheckedAt: null,
      anchorUid: '',
      anchorName: '',
    }
  }

  it('没有选中直播间时提示先选择，而不是权限问题', async () => {
    const auth = useAuthStore()
    auth.user = { id: 1, username: '张三', isAdmin: false, createdAt: '' }
    vi.stubGlobal(
      'fetch',
      vi.fn().mockImplementation(() => Promise.resolve(ok([]))),
    )

    const wrapper = mountAdmin()
    await flushPromises()

    expect(wrapper.text()).toContain('请先在顶部选择一个直播间')
  })

  it('账号所有者但绑定上没有 member:manage 时，提示是「需要 member:manage 权限，请让管理员授予」，不是「你不是所有者」', async () => {
    const auth = useAuthStore()
    auth.user = { id: 1, username: '张三', isAdmin: false, createdAt: '' }
    const bindings = useBindingsStore()
    // 所有者拿不到 member:manage 是刻意设计（perm.OwnerBypass 排除了它），
    // 这里模拟的正是这种情况：绑定上只有其他权限点，没有 member:manage
    bindings.list = [bindingWith(['rule:read', 'rule:write', 'user:block', 'event:read'])]
    bindings.select(1)
    vi.stubGlobal(
      'fetch',
      vi.fn().mockImplementation(() => Promise.resolve(ok([]))),
    )

    const wrapper = mountAdmin()
    await flushPromises()

    expect(wrapper.text()).toContain('授权管理需要 member:manage 权限，请让管理员授予')
    expect(wrapper.text()).not.toContain('你不是所有者')
  })

  it('有 member:manage 权限时渲染 MemberEditor，不显示警告', async () => {
    const auth = useAuthStore()
    auth.user = { id: 1, username: '张三', isAdmin: false, createdAt: '' }
    const bindings = useBindingsStore()
    bindings.list = [bindingWith(['member:manage'])]
    bindings.select(1)
    vi.stubGlobal(
      'fetch',
      vi.fn().mockImplementation(() => Promise.resolve(ok([]))),
    )

    const wrapper = mountAdmin()
    await flushPromises()

    expect(wrapper.text()).not.toContain('授权管理需要 member:manage 权限')
    expect(wrapper.findComponent({ name: 'MemberEditor' }).exists()).toBe(true)
  })

  it('管理员在任何绑定上都能打开授权管理（hasPerm 对管理员一律放行）', async () => {
    const auth = useAuthStore()
    auth.user = { id: 1, username: '管理员', isAdmin: true, createdAt: '' }
    const bindings = useBindingsStore()
    bindings.list = [bindingWith([])] // 权限点列表是空的，全靠 isAdmin 放行
    bindings.select(1)
    vi.stubGlobal(
      'fetch',
      vi.fn().mockImplementation(() => Promise.resolve(ok([]))),
    )

    const wrapper = mountAdmin()
    await flushPromises()

    expect(wrapper.text()).not.toContain('授权管理需要 member:manage 权限')
  })
})
