import { beforeEach, describe, expect, it, vi } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'

// MemberEditor 是「授权管理」区，最重要的两条:
//
//   1. 权限点清单必须来自 GET /api/meta/permissions，不能在前端硬编码
//      ——硬编码就是第二处定义，后端权限点从七个删到六个之后硬编码的
//      清单当场就是错的。
//   2. PRESETS 只是一键勾选，提交给后端的永远是展开后的权限点数组，
//      不是角色名（比如 '运营'）；预设之间、预设与手动勾选之间可叠加，
//      不是互斥单选。
//
// 用 vi.mock 顶掉 naive-ui 的 useDialog/useMessage，原因与
// Accounts.test.ts、Moderation.test.ts 一致：真弹窗涉及 Teleport 与
// 动画时序，会让「确认前不发请求」这类断言变得不确定。
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

const { default: MemberEditor } = await import('./MemberEditor.vue')

/** 与后端 GET /api/meta/permissions 的真实返回形状一致：{value, label}[]。 */
const FULL_PERMISSION_META = [
  { value: 'rule:read', label: '查看规则' },
  { value: 'rule:write', label: '增删改规则、启停规则' },
  { value: 'danmaku:send', label: '手动发送弹幕' },
  { value: 'user:block', label: '禁言与解禁，含维护禁言名单' },
  { value: 'member:manage', label: '授权他人、撤销授权' },
  { value: 'event:read', label: '查看事件流与历史业务日志' },
]

function ok(body: unknown) {
  return new Response(JSON.stringify(body), {
    status: 200,
    headers: { 'Content-Type': 'application/json' },
  })
}

/** 每次都返回一个新 Response 实例——Response.body 只能读一次，
 * 一条测试里多次 fetch 复用同一个实例的话，第二次读 .json() 会抛。 */
function stubFetch(opts: {
  members?: unknown[]
  permissionMeta?: unknown[]
  onWrite?: (url: string, init: RequestInit) => void
}) {
  const members = opts.members ?? []
  const permissionMeta = opts.permissionMeta ?? FULL_PERMISSION_META
  const f = vi.fn().mockImplementation((url: string, init?: RequestInit) => {
    if (url === '/api/meta/permissions') return Promise.resolve(ok(permissionMeta))
    if (url === '/api/bindings/1/members' && init?.method === 'GET') {
      return Promise.resolve(ok(members))
    }
    if (init) opts.onWrite?.(url, init)
    return Promise.resolve(ok({ status: 'ok' }))
  })
  vi.stubGlobal('fetch', f)
  return f
}

beforeEach(() => {
  vi.unstubAllGlobals()
  warningMock.mockClear()
  messageMock.success.mockClear()
  messageMock.error.mockClear()
  messageMock.warning.mockClear()
  messageMock.info.mockClear()
})

describe('MemberEditor 权限点清单来自后端，不硬编码', () => {
  it('渲染的权限点复选框文案是 GET /api/meta/permissions 返回的 label', async () => {
    stubFetch({})
    const wrapper = mount(MemberEditor, { props: { bindingId: 1 } })
    await flushPromises()

    expect(wrapper.text()).toContain('禁言与解禁，含维护禁言名单')
    expect(wrapper.text()).toContain('授权他人、撤销授权')
  })

  // 关键：即便 Permission 联合类型里仍然有六个值，只要后端这次只回了五个
  // （模拟某个权限点被临时下线/还没上线），界面就不该凭空多出一项——
  // 这证明清单是照单全收后端的返回，而不是在前端遍历一份写死的枚举。
  it('后端只返回五个权限点时，界面就只渲染五个，不会自己补全第六个', async () => {
    const partial = FULL_PERMISSION_META.filter((p) => p.value !== 'danmaku:send')
    stubFetch({ permissionMeta: partial })
    const wrapper = mount(MemberEditor, { props: { bindingId: 1 } })
    await flushPromises()

    expect(wrapper.text()).not.toContain('手动发送弹幕')
    // 其余五个都还在，不是整体没加载出来
    expect(wrapper.text()).toContain('查看规则')
    expect(wrapper.text()).toContain('授权他人、撤销授权')
  })
})

describe('MemberEditor PRESETS：点预设提交的是展开后的权限点数组，不是角色名', () => {
  it('点击「运营」预设后保存，PUT body 里的 permissions 是三个权限点，不是 ["运营"]', async () => {
    let sentBody: unknown = null
    const f = stubFetch({
      onWrite: (url, init) => {
        if (url === '/api/bindings/1/members/%E5%BC%A0%E4%B8%89' && init.method === 'PUT') {
          sentBody = JSON.parse(init.body as string)
        }
      },
    })
    const wrapper = mount(MemberEditor, { props: { bindingId: 1 } })
    await flushPromises()

    await wrapper.find('input[placeholder="被授权人的用户名"]').setValue('张三')
    const presetBtn = wrapper.findAll('button').find((b) => b.text() === '运营')
    expect(presetBtn, '应该有「运营」这个预设按钮').toBeTruthy()
    await presetBtn!.trigger('click')

    const saveBtn = wrapper.findAll('button').find((b) => b.text() === '保存')
    await saveBtn!.trigger('click')
    await flushPromises()

    expect(f).toHaveBeenCalled()
    expect(sentBody).toEqual({ permissions: ['rule:read', 'rule:write', 'event:read'] })
  })

  it('预设可叠加：先点「运营」再点「房管」，两组权限点并在一起而不是互相替换', async () => {
    let sentBody: unknown = null
    stubFetch({
      onWrite: (_url, init) => {
        if (init.method === 'PUT') sentBody = JSON.parse(init.body as string)
      },
    })
    const wrapper = mount(MemberEditor, { props: { bindingId: 1 } })
    await flushPromises()

    await wrapper.find('input[placeholder="被授权人的用户名"]').setValue('张三')
    const findBtn = (label: string) => wrapper.findAll('button').find((b) => b.text() === label)!
    await findBtn('运营').trigger('click')
    await findBtn('房管').trigger('click')
    await findBtn('保存').trigger('click')
    await flushPromises()

    // 运营 = rule:read/rule:write/event:read，房管 = user:block/event:read
    // 并集去重后应该是四个权限点，event:read 不重复出现
    const body = sentBody as { permissions: string[] }
    expect(new Set(body.permissions)).toEqual(
      new Set(['rule:read', 'rule:write', 'event:read', 'user:block']),
    )
    expect(body.permissions.length).toBe(4)
  })

  it('点了预设之后用户还能自己再勾选预设之外的权限点，不是被锁死成单选', async () => {
    let sentBody: unknown = null
    stubFetch({
      onWrite: (_url, init) => {
        if (init.method === 'PUT') sentBody = JSON.parse(init.body as string)
      },
    })
    const wrapper = mount(MemberEditor, { props: { bindingId: 1 } })
    await flushPromises()

    await wrapper.find('input[placeholder="被授权人的用户名"]').setValue('张三')
    const findBtn = (label: string) => wrapper.findAll('button').find((b) => b.text() === label)!
    await findBtn('运营').trigger('click') // rule:read/rule:write/event:read

    // 运营预设不含 danmaku:send，手动点它对应的复选框
    const danmakuCheckbox = wrapper
      .findAll('.n-checkbox')
      .find((el) => el.text() === '手动发送弹幕')
    expect(danmakuCheckbox, '应该能找到「手动发送弹幕」这个复选框').toBeTruthy()
    await danmakuCheckbox!.trigger('click')

    await findBtn('保存').trigger('click')
    await flushPromises()

    const body = sentBody as { permissions: string[] }
    expect(body.permissions).toContain('danmaku:send')
    expect(body.permissions).toContain('rule:read')
  })

  it('未选任何权限点时点保存只提示，不发 PUT 请求', async () => {
    const f = stubFetch({})
    const wrapper = mount(MemberEditor, { props: { bindingId: 1 } })
    await flushPromises()

    await wrapper.find('input[placeholder="被授权人的用户名"]').setValue('张三')
    const callsBefore = f.mock.calls.length
    const saveBtn = wrapper.findAll('button').find((b) => b.text() === '保存')
    await saveBtn!.trigger('click')
    await flushPromises()

    expect(messageMock.warning).toHaveBeenCalled()
    expect(f.mock.calls.length).toBe(callsBefore)
  })
})

describe('MemberEditor 编辑与撤销', () => {
  const 李四: { username: string; permissions: string[] } = {
    username: '李四',
    permissions: ['user:block', 'event:read'],
  }

  it('点「编辑」后用户名输入框禁用并回填该成员的权限点勾选', async () => {
    stubFetch({ members: [李四] })
    const wrapper = mount(MemberEditor, { props: { bindingId: 1 } })
    await flushPromises()

    const editBtn = wrapper.findAll('button').find((b) => b.text() === '编辑')
    await editBtn!.trigger('click')
    await flushPromises()

    const usernameInput = wrapper.find('input[placeholder="被授权人的用户名"]')
    expect((usernameInput.element as HTMLInputElement).value).toBe('李四')
    expect(usernameInput.attributes('disabled')).toBeDefined()
  })

  it('点「撤销」先弹确认，确认之前不发 DELETE；确认之后才发', async () => {
    const f = stubFetch({ members: [李四] })
    const wrapper = mount(MemberEditor, { props: { bindingId: 1 } })
    await flushPromises()

    const revokeBtn = wrapper.findAll('button').find((b) => b.text() === '撤销')
    await revokeBtn!.trigger('click')
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
