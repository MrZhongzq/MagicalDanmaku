import { beforeEach, describe, expect, it, vi } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import { createRouter, createMemoryHistory } from 'vue-router'

// Moderation 页最重要的一条是「警告但绝不锁面板」：缺 user:block 权限时
// 只弹一条提示，控件不能 disabled。用 vi.mock 顶掉 naive-ui 的
// useMessage，这样可以直接断言 message.error 收到的是不是后端原文，
// 而不必真的挂 NMessageProvider 去读 DOM 里转瞬即逝的提示条。
const messageMock = { success: vi.fn(), error: vi.fn(), warning: vi.fn(), info: vi.fn() }

vi.mock('naive-ui', async () => {
  const actual = await vi.importActual<typeof import('naive-ui')>('naive-ui')
  return {
    ...actual,
    useMessage: () => messageMock,
  }
})

const { default: Moderation } = await import('./Moderation.vue')
const { useBindingsStore } = await import('@/stores/bindings')
const { useAuthStore } = await import('@/stores/auth')

/** 只注册 accounts，不注册 moderation/custom——测「未实现路由」分支要用到。 */
function testRouter() {
  return createRouter({
    history: createMemoryHistory(),
    routes: [{ path: '/accounts', name: 'accounts', component: { template: '<div/>' } }],
  })
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
  permissions: ['user:block'],
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

/** 设好 pinia + 当前绑定，返回 mount 用的 global 配置。 */
function setupStores(permissions: string[]) {
  setActivePinia(createPinia())
  const bindings = useBindingsStore()
  bindings.list = [{ ...绑定, permissions: permissions as typeof 绑定.permissions }]
  bindings.select(1)

  const auth = useAuthStore()
  auth.user = { id: 1, username: '张三', isAdmin: false, createdAt: '' }

  const router = testRouter()
  return { bindings, auth, router }
}

async function mountModeration(router: ReturnType<typeof testRouter>) {
  // 先让路由完成一次真正的初始导航，再挂载组件。不这样做的话，
  // vue-router 装好插件时会自己发起一次对当前地址（memory history
  // 默认是空字符串）的导航，这个 router 又没注册空路径，导航一直
  // 挂起到测试结束都没 resolve；等下一个测试文件把 jsdom 环境重置掉，
  // 这个迟到的 promise 才 resolve，读 window.history 时环境已经没了，
  // 报「history is not defined」。这不是断言失败，是一次真实的
  // 未处理拒绝，会在 --sequence.shuffle 之类改变文件执行顺序时冒出来。
  await router.push('/accounts')
  await router.isReady()
  const wrapper = mount(Moderation, { global: { plugins: [router] } })
  await flushPromises()
  return wrapper
}

describe('Moderation 权限警告：提示但不锁面板', () => {
  beforeEach(() => {
    vi.unstubAllGlobals()
    messageMock.success.mockClear()
    messageMock.error.mockClear()
    messageMock.warning.mockClear()
    messageMock.info.mockClear()
  })

  it('缺 user:block 时顶部出现警告，但禁言/解禁/加入名单按钮都不是 disabled', async () => {
    vi.stubGlobal(
      'fetch',
      vi.fn().mockImplementation(() => Promise.resolve(ok([]))),
    )
    const { router } = setupStores([]) // 没有 user:block
    const wrapper = await mountModeration(router)

    expect(wrapper.text()).toContain('你在这个直播间没有 user:block 权限')

    const buttonTexts = ['禁言', '解禁', '加入名单', '拉黑', '解除拉黑']
    for (const label of buttonTexts) {
      const btn = wrapper.findAll('button').find((b) => b.text() === label)
      expect(btn, `按钮「${label}」应该存在`).toBeTruthy()
      // naive-ui 的 NButton 在 disabled 时会带 .n-button--disabled 类，
      // 原生 disabled 属性也会跟着加上——两个都不该出现。
      expect(btn!.attributes('disabled'), `按钮「${label}」不该 disabled`).toBeUndefined()
      expect(btn!.classes().join(' ')).not.toContain('disabled')
    }
  })

  it('有 user:block 权限时不显示警告条', async () => {
    vi.stubGlobal(
      'fetch',
      vi.fn().mockImplementation(() => Promise.resolve(ok([]))),
    )
    const { router } = setupStores(['user:block'])
    const wrapper = await mountModeration(router)

    expect(wrapper.text()).not.toContain('没有 user:block 权限')
  })
})

describe('Moderation 失败回显：后端原文，不包装', () => {
  beforeEach(() => {
    vi.unstubAllGlobals()
    messageMock.success.mockClear()
    messageMock.error.mockClear()
    messageMock.warning.mockClear()
    messageMock.info.mockClear()
  })

  // 这条测试要验证的正是简报里最容易被"优化掉"的一句话：502 的原文
  // 「禁言失败: 你不是本房间的房管」必须原样透出，而不是被换成
  // 「操作失败，请重试」这种听起来更友好、实际上把原因删掉的文案。
  it('禁言失败（502）时 message.error 收到的是后端原文，不是笼统提示', async () => {
    const backendMessage = '禁言失败: 你不是本房间的房管'
    vi.stubGlobal(
      'fetch',
      vi.fn().mockImplementation((url: string, init?: RequestInit) => {
        if (url.endsWith('/blocklist')) return Promise.resolve(ok([]))
        if (url.endsWith('/block') && init?.method === 'POST') {
          return Promise.resolve(err(502, backendMessage))
        }
        return Promise.resolve(ok({ status: 'ok' }))
      }),
    )
    const { router } = setupStores(['user:block'])
    const wrapper = await mountModeration(router)

    const uidInput = wrapper.find('input[placeholder="UID"]')
    await uidInput.setValue('10086')
    const blockBtn = wrapper.findAll('button').find((b) => b.text() === '禁言')
    await blockBtn!.trigger('click')
    await flushPromises()

    expect(messageMock.error).toHaveBeenCalledWith(backendMessage)
  })

  it('解禁失败时同样原样回显', async () => {
    const backendMessage = '解除禁言失败: 房间不存在或已关闭'
    vi.stubGlobal(
      'fetch',
      vi.fn().mockImplementation((url: string, init?: RequestInit) => {
        if (url.endsWith('/blocklist')) return Promise.resolve(ok([]))
        if (url.endsWith('/unblock') && init?.method === 'POST') {
          return Promise.resolve(err(502, backendMessage))
        }
        return Promise.resolve(ok({ status: 'ok' }))
      }),
    )
    const { router } = setupStores(['user:block'])
    const wrapper = await mountModeration(router)

    const inputs = wrapper.findAll('input[placeholder="UID"]')
    // 第二个 UID 输入框是「快捷解禁」那一栏
    await inputs[1].setValue('10086')
    const unblockBtn = wrapper.findAll('button').find((b) => b.text() === '解禁')
    await unblockBtn!.trigger('click')
    await flushPromises()

    expect(messageMock.error).toHaveBeenCalledWith(backendMessage)
  })
})

describe('Moderation 禁言名单：加入/删除之后刷新列表', () => {
  beforeEach(() => {
    vi.unstubAllGlobals()
    messageMock.success.mockClear()
    messageMock.error.mockClear()
    messageMock.warning.mockClear()
    messageMock.info.mockClear()
  })

  it('加入名单成功后重新拉取列表，界面显示新数据', async () => {
    let added = false
    const f = vi.fn().mockImplementation((url: string, init?: RequestInit) => {
      if (url.endsWith('/blocklist') && init?.method === 'GET') {
        return Promise.resolve(
          ok(
            added
              ? [
                  {
                    id: 1,
                    uid: '10086',
                    username: '广告号',
                    reason: '刷屏',
                    createdBy: 1,
                    createdAt: '2026-08-01 00:00:00',
                  },
                ]
              : [],
          ),
        )
      }
      if (url.endsWith('/blocklist') && init?.method === 'POST') {
        added = true
        return Promise.resolve(ok({ uid: '10086' }, 201))
      }
      return Promise.resolve(ok({ status: 'ok' }))
    })
    vi.stubGlobal('fetch', f)

    const { router } = setupStores(['user:block'])
    const wrapper = await mountModeration(router)
    expect(wrapper.text()).not.toContain('广告号')

    // 页面里有五处 UID 输入框（快捷禁言/快捷解禁/加入名单/拉黑/解除拉黑），
    // 按模板里的书写顺序，「加入名单」表单的 UID 输入框是第三个。
    const uidInput = wrapper.findAll('input[placeholder="UID"]')[2]
    await uidInput.setValue('10086')
    const addBtn = wrapper.findAll('button').find((b) => b.text() === '加入名单')
    await addBtn!.trigger('click')
    await flushPromises()

    expect(wrapper.text()).toContain('广告号')
  })

  it('从名单删除一条之后重新拉取列表', async () => {
    let removed = false
    const f = vi.fn().mockImplementation((url: string, init?: RequestInit) => {
      if (url.endsWith('/blocklist')) {
        return Promise.resolve(
          ok(
            removed
              ? []
              : [
                  {
                    id: 1,
                    uid: '10086',
                    username: '广告号',
                    reason: '刷屏',
                    createdBy: 1,
                    createdAt: '2026-08-01 00:00:00',
                  },
                ],
          ),
        )
      }
      if (url.includes('/blocklist/10086') && init?.method === 'DELETE') {
        removed = true
        return Promise.resolve(ok({ status: 'ok' }))
      }
      return Promise.resolve(ok({ status: 'ok' }))
    })
    vi.stubGlobal('fetch', f)

    const { router } = setupStores(['user:block'])
    const wrapper = await mountModeration(router)
    await flushPromises()
    expect(wrapper.text()).toContain('广告号')

    const delBtn = wrapper.findAll('button').find((b) => b.text() === '删除')
    await delBtn!.trigger('click')
    await flushPromises()

    expect(wrapper.text()).not.toContain('广告号')
  })
})

describe('Moderation 主播区：拉黑接到禁言接口（hours 固定 720），不是独立功能', () => {
  beforeEach(() => {
    vi.unstubAllGlobals()
    messageMock.success.mockClear()
    messageMock.error.mockClear()
    messageMock.warning.mockClear()
    messageMock.info.mockClear()
  })

  it('不再显示「待后端支持」标签', async () => {
    vi.stubGlobal(
      'fetch',
      vi.fn().mockImplementation(() => Promise.resolve(ok([]))),
    )
    const { router } = setupStores(['user:block'])
    const wrapper = await mountModeration(router)

    expect(wrapper.text()).not.toContain('待后端支持')
  })

  it('点「拉黑」发送 POST .../block，hours 固定 720', async () => {
    let blockBody: unknown = null
    const f = vi.fn().mockImplementation((url: string, init?: RequestInit) => {
      if (url.endsWith('/blocklist')) return Promise.resolve(ok([]))
      if (url.endsWith('/block') && init?.method === 'POST') {
        blockBody = init?.body ? JSON.parse(String(init.body)) : null
        return Promise.resolve(ok({ uid: '10086', hours: 720 }))
      }
      return Promise.resolve(ok({ status: 'ok' }))
    })
    vi.stubGlobal('fetch', f)

    const { router } = setupStores(['user:block'])
    const wrapper = await mountModeration(router)

    // 「拉黑」的 UID 输入框是第 4 个（快捷禁言/快捷解禁/加入名单/拉黑/解除拉黑）
    const uidInput = wrapper.findAll('input[placeholder="UID"]')[3]
    await uidInput.setValue('10086')
    const blackoutBtn = wrapper.findAll('button').find((b) => b.text() === '拉黑')
    await blackoutBtn!.trigger('click')
    await flushPromises()

    expect(blockBody).toEqual({ uid: '10086', hours: 720 })
    expect(messageMock.success).toHaveBeenCalledWith('已拉黑 UID 10086（禁言 720 小时）')
  })

  it('点「解除拉黑」发送 POST .../unblock，与解禁走同一个接口', async () => {
    let unblockBody: unknown = null
    const f = vi.fn().mockImplementation((url: string, init?: RequestInit) => {
      if (url.endsWith('/blocklist')) return Promise.resolve(ok([]))
      if (url.endsWith('/unblock') && init?.method === 'POST') {
        unblockBody = init?.body ? JSON.parse(String(init.body)) : null
        return Promise.resolve(ok({ uid: '10086' }))
      }
      return Promise.resolve(ok({ status: 'ok' }))
    })
    vi.stubGlobal('fetch', f)

    const { router } = setupStores(['user:block'])
    const wrapper = await mountModeration(router)

    const uidInput = wrapper.findAll('input[placeholder="UID"]')[4]
    await uidInput.setValue('10086')
    const unblockBtn = wrapper.findAll('button').find((b) => b.text() === '解除拉黑')
    await unblockBtn!.trigger('click')
    await flushPromises()

    expect(unblockBody).toEqual({ uid: '10086' })
    expect(messageMock.success).toHaveBeenCalledWith('已解除 UID 10086 的拉黑')
  })

  it('拉黑失败时把后端原文原样显示，不包装成笼统提示，且非主播账号仍能点（不锁死）', async () => {
    const backendMessage = '禁言失败: 你不是本直播间的主播'
    const f = vi.fn().mockImplementation((url: string, init?: RequestInit) => {
      if (url.endsWith('/blocklist')) return Promise.resolve(ok([]))
      if (url.endsWith('/block') && init?.method === 'POST') {
        return Promise.resolve(err(502, backendMessage))
      }
      return Promise.resolve(ok({ status: 'ok' }))
    })
    vi.stubGlobal('fetch', f)

    const { router } = setupStores(['user:block'])
    const wrapper = await mountModeration(router)

    const blackoutBtn = wrapper.findAll('button').find((b) => b.text() === '拉黑')
    // 不 disabled——「保持不锁死」原则同样适用于主播区
    expect(blackoutBtn!.attributes('disabled')).toBeUndefined()

    const uidInput = wrapper.findAll('input[placeholder="UID"]')[3]
    await uidInput.setValue('10086')
    await blackoutBtn!.trigger('click')
    await flushPromises()

    expect(messageMock.error).toHaveBeenCalledWith(backendMessage)
  })
})

describe('Moderation 自动禁言规则占位：跳转到还没注册的路由不抛异常', () => {
  beforeEach(() => {
    vi.unstubAllGlobals()
    messageMock.success.mockClear()
    messageMock.error.mockClear()
    messageMock.warning.mockClear()
    messageMock.info.mockClear()
  })

  it('『自定义弹幕姬』还没注册路由时点「去配置」只提示，不抛异常', async () => {
    vi.stubGlobal(
      'fetch',
      vi.fn().mockImplementation(() => Promise.resolve(ok([]))),
    )
    const { router } = setupStores(['user:block'])
    const wrapper = await mountModeration(router)

    const goBtn = wrapper.findAll('button').find((b) => b.text() === '去配置')
    await expect(goBtn!.trigger('click')).resolves.not.toThrow()
    await flushPromises()

    expect(messageMock.info).toHaveBeenCalled()
  })
})

describe('Moderation 自动禁言规则：路由存在时点「去配置」带上预填 query', () => {
  beforeEach(() => {
    vi.unstubAllGlobals()
    messageMock.success.mockClear()
    messageMock.error.mockClear()
    messageMock.warning.mockClear()
    messageMock.info.mockClear()
  })

  // Task 14 收尾：custom 路由已经注册了，「去配置」真能跳过去，但只跳过去
  // 不够——不带 query 的话 Custom.vue 无从得知要预填。这条测试钉住跳转
  // 一定带着 preset=automute，防止将来重构时把这个 query 弄丢。
  it('跳转带 query preset=automute，不再只提示', async () => {
    vi.stubGlobal(
      'fetch',
      vi.fn().mockImplementation(() => Promise.resolve(ok([]))),
    )
    const { router } = setupStores(['user:block'])
    router.addRoute({ name: 'custom', path: '/custom', component: { template: '<div/>' } })
    const wrapper = await mountModeration(router)

    const goBtn = wrapper.findAll('button').find((b) => b.text() === '去配置')
    await goBtn!.trigger('click')
    await flushPromises()

    expect(router.currentRoute.value.name).toBe('custom')
    expect(router.currentRoute.value.query.preset).toBe('automute')
    // 路由已注册，不该再走「还没做」的提示分支
    expect(messageMock.info).not.toHaveBeenCalled()
  })
})
