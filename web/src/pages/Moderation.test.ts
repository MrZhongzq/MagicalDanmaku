import { beforeEach, describe, expect, it, vi } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import { createRouter, createMemoryHistory } from 'vue-router'

// Moderation 页最重要的一条是「警告但绝不锁面板」：缺 user:block 权限时
// 只弹一条提示，控件不能 disabled；拉黑区同理，缺账号所有权也只警告。
// 用 vi.mock 顶掉 naive-ui 的 useMessage/useDialog，这样可以直接断言
// message.error 收到的是不是后端原文、dialog.warning 收到的是不是正确的
// 确认文案，而不必真的挂 NDialogProvider/NMessageProvider 去读 DOM 里
// 转瞬即逝的提示条——与 Custom.test.ts 删规则草稿用的同一套模式。
//
// P5-6 之前这里没有 useDialog：Moderation.vue 为「拉黑」加了确认对话框
// 之后，测试没跟着挂 provider，导致全部用例在 setup() 里就抛
// 「No outer <n-dialog-provider /> founded」——这是测试侧的问题（生产
// 代码在真实 App.vue 里本来就有全局的 NDialogProvider），不是生产代码
// 的 bug，所以这里用 mock 补上，不改 Moderation.vue。
const messageMock = { success: vi.fn(), error: vi.fn(), warning: vi.fn(), info: vi.fn() }
const dialogWarningMock = vi.fn()

vi.mock('naive-ui', async () => {
  const actual = await vi.importActual<typeof import('naive-ui')>('naive-ui')
  return {
    ...actual,
    useMessage: () => messageMock,
    useDialog: () => ({ warning: dialogWarningMock }),
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
  isOwner: boolean
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
  permissions: ['user:block'],
  isOwner: true,
  liveStatus: 'unknown',
  liveCheckedAt: null,
  anchorUid: '',
  anchorName: '',
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

/**
 * 设好 pinia + 当前绑定，返回 mount 用的 global 配置。
 *
 * permissions 控制 user:block 权限点（房管区），isOwner 控制账号所有权
 * （拉黑区）——两条独立的轴，默认都给全，测某一条缺失时单独覆盖。
 */
function setupStores(permissions: string[], isOwner = true) {
  setActivePinia(createPinia())
  const bindings = useBindingsStore()
  bindings.list = [
    { ...绑定, permissions: permissions as typeof 绑定.permissions, isOwner },
  ]
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

/** 默认把 /blocklist 与 /blacklist-status 都喂空数据，测试只关心自己那一条请求时用。 */
function baselineFetch(overrides: (url: string, init?: RequestInit) => Response | undefined) {
  return vi.fn().mockImplementation((url: string, init?: RequestInit) => {
    const custom = overrides(url, init)
    if (custom) return Promise.resolve(custom)
    if (url.endsWith('/blocklist')) return Promise.resolve(ok([]))
    return Promise.resolve(ok({ status: 'ok' }))
  })
}

function clearMocks() {
  vi.unstubAllGlobals()
  messageMock.success.mockClear()
  messageMock.error.mockClear()
  messageMock.warning.mockClear()
  messageMock.info.mockClear()
  dialogWarningMock.mockClear()
}

describe('Moderation 权限警告：提示但不锁面板', () => {
  beforeEach(clearMocks)

  it('缺 user:block 时顶部出现警告，但禁言/解禁/加入名单/拉黑/解除拉黑按钮都不是 disabled', async () => {
    vi.stubGlobal(
      'fetch',
      vi.fn().mockImplementation(() => Promise.resolve(ok([]))),
    )
    const { router } = setupStores([]) // 没有 user:block，仍是所有者
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

  it('有 user:block 权限、是账号所有者时不显示任何警告条', async () => {
    vi.stubGlobal(
      'fetch',
      vi.fn().mockImplementation(() => Promise.resolve(ok([]))),
    )
    const { router } = setupStores(['user:block'], true)
    const wrapper = await mountModeration(router)

    expect(wrapper.text()).not.toContain('没有 user:block 权限')
    expect(wrapper.text()).not.toContain('不是这个账号的所有者')
  })

  // 拉黑区走的是账号所有权，不是 user:block——这是本次任务在权限模型上
  // 的核心决定。持有 user:block（普通房管）但不是所有者时，应当看到
  // 拉黑区专属的警告，且房管区不受影响（房管区仍然没有警告）。
  it('持有 user:block 但不是账号所有者：只有拉黑区出现警告，拉黑/解除拉黑仍不 disabled', async () => {
    vi.stubGlobal(
      'fetch',
      vi.fn().mockImplementation(() => Promise.resolve(ok([]))),
    )
    const { router } = setupStores(['user:block'], false)
    const wrapper = await mountModeration(router)

    expect(wrapper.text()).not.toContain('没有 user:block 权限')
    expect(wrapper.text()).toContain('你不是这个账号的所有者')

    for (const label of ['拉黑', '解除拉黑']) {
      const btn = wrapper.findAll('button').find((b) => b.text() === label)
      expect(btn!.attributes('disabled')).toBeUndefined()
    }
  })
})

// P5-3：真机反馈原话——房管页看不到也换不了当前是哪个绑定，用户在这页做的
// 操作其实打在某个绑定上，但不知道是哪个。删掉页面里的 BindingSelector
// 这条测试就会变红。
describe('Moderation 页：正文里也有账号+直播间选择器', () => {
  it('页面渲染 BindingSelector，且要求 user:block 权限', async () => {
    vi.stubGlobal(
      'fetch',
      vi.fn().mockImplementation(() => Promise.resolve(ok([]))),
    )
    const { router } = setupStores(['user:block'])
    const wrapper = await mountModeration(router)

    const { default: BindingSelector } = await import('@/components/BindingSelector.vue')
    const selector = wrapper.findComponent(BindingSelector)
    expect(selector.exists()).toBe(true)
    expect(selector.props('requiredPerm')).toBe('user:block')
  })

  // 这条原来钉在 Shell.test.ts 上——Shell 头部曾经有一份全局 BindingSelector，
  // 「绑定列表加载失败」的回显与重试就是靠它验证的。协调者裁决去掉那份全局
  // 选择器（只保留页面正文里这份）之后，Shell 不再展示这个状态，这条断言
  // 挪到这里，改成验证页面正文里的 BindingSelector 一样能原样回显后端错误、
  // 点「重试」一样能重新拉取绑定列表——不是删掉覆盖，是换了个真实成立的落点。
  it('绑定列表加载失败时，页面正文里的选择器原样显示后端错误，点「重试」会重新请求', async () => {
    // Moderation.vue 不像 Shell.vue 那样在 onMounted 里自己调
    // bindings.refresh()——那是 Shell 的职责，这里只挂 Moderation 本身。
    // 所以「加载失败」这个前提用手动置 loadError 模拟（等价于
    // bindings.refresh() 内部失败后的状态），点「重试」触发的才是这条
    // 用例里第一次、也是唯一一次真实的 GET /api/bindings。
    const f = vi.fn().mockImplementation((url: string) => {
      if (url === '/api/bindings') return Promise.resolve(ok([绑定]))
      if (url.endsWith('/blocklist')) return Promise.resolve(ok([]))
      return Promise.resolve(ok({ status: 'ok' }))
    })
    vi.stubGlobal('fetch', f)

    const { bindings, router } = setupStores(['user:block'])
    bindings.loadError = '数据库连不上'
    const wrapper = await mountModeration(router)

    expect(wrapper.text()).toContain('数据库连不上')

    const retryBtn = wrapper.findAll('button').find((b) => b.text() === '重试')
    expect(retryBtn).toBeTruthy()
    await retryBtn!.trigger('click')
    await flushPromises()

    expect(wrapper.text()).not.toContain('数据库连不上')
  })
})

describe('Moderation 房管区：禁言/解禁失败回显后端原文，不包装', () => {
  beforeEach(clearMocks)

  // 这条测试要验证的正是简报里最容易被"优化掉"的一句话：502 的原文
  // 「禁言失败: 你不是本房间的房管」必须原样透出，而不是被换成
  // 「操作失败，请重试」这种听起来更友好、实际上把原因删掉的文案。
  it('禁言失败（502）时 message.error 收到的是后端原文，不是笼统提示', async () => {
    const backendMessage = '禁言失败: 你不是本房间的房管'
    vi.stubGlobal(
      'fetch',
      baselineFetch((url, init) => {
        if (url.endsWith('/block') && init?.method === 'POST') return err(502, backendMessage)
        return undefined
      }),
    )
    const { router } = setupStores(['user:block'])
    const wrapper = await mountModeration(router)

    // 房管区第一个 UID 输入框是「快捷禁言」
    const uidInput = wrapper.findAll('input[placeholder="UID"]')[0]
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
      baselineFetch((url, init) => {
        if (url.endsWith('/unblock') && init?.method === 'POST') return err(502, backendMessage)
        return undefined
      }),
    )
    const { router } = setupStores(['user:block'])
    const wrapper = await mountModeration(router)

    // 第二个 UID 输入框是「快捷解禁」那一栏
    const inputs = wrapper.findAll('input[placeholder="UID"]')
    await inputs[1].setValue('10086')
    const unblockBtn = wrapper.findAll('button').find((b) => b.text() === '解禁')
    await unblockBtn!.trigger('click')
    await flushPromises()

    expect(messageMock.error).toHaveBeenCalledWith(backendMessage)
  })
})

describe('Moderation 禁言名单：加入/删除之后刷新列表，且不再要求手填昵称/理由', () => {
  beforeEach(clearMocks)

  // P5-6 硬性要求：去掉禁言理由和昵称输入框，昵称由后端按 UID 自动查询。
  // 这条测试证明「加入名单」那一行只有一个 UID 输入框——如果有人把
  // 昵称/理由输入框加回来，这条会失败。
  it('「加入名单」只有一个 UID 输入框，没有昵称/原因输入框', async () => {
    vi.stubGlobal(
      'fetch',
      vi.fn().mockImplementation(() => Promise.resolve(ok([]))),
    )
    const { router } = setupStores(['user:block'])
    const wrapper = await mountModeration(router)

    expect(wrapper.find('input[placeholder="昵称"]').exists()).toBe(false)
    expect(wrapper.find('input[placeholder="原因"]').exists()).toBe(false)
    expect(wrapper.find('input[placeholder="理由"]').exists()).toBe(false)
  })

  it('加入名单成功后重新拉取列表，界面显示新数据（昵称是后端自动回填的）', async () => {
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
                    reason: '',
                    createdBy: 1,
                    createdAt: '2026-08-01 00:00:00',
                  },
                ]
              : [],
          ),
        )
      }
      if (url.endsWith('/blocklist') && init?.method === 'POST') {
        const body = init?.body ? JSON.parse(String(init.body)) : {}
        // 前端不该发 username/reason 字段——昵称交给后端按 UID 查
        expect(body).toEqual({ uid: '10086' })
        added = true
        return Promise.resolve(ok({ uid: '10086' }, 201))
      }
      return Promise.resolve(ok({ status: 'ok' }))
    })
    vi.stubGlobal('fetch', f)

    const { router } = setupStores(['user:block'])
    const wrapper = await mountModeration(router)
    expect(wrapper.text()).not.toContain('广告号')

    // 房管区有三处 UID 输入框（快捷禁言/快捷解禁/加入名单），
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
                    reason: '',
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

// ============================================================
// P5-6 核心：拉黑区——账号级操作，与禁言彻底分开
// ============================================================
describe('Moderation 拉黑区：账号级拉黑/解除拉黑，走独立接口与独立权限判定', () => {
  beforeEach(clearMocks)

  function blacklistUidInput(wrapper: Awaited<ReturnType<typeof mountModeration>>) {
    // 房管区三个 UID 框（快捷禁言/快捷解禁/加入名单）之后，第四个是拉黑区
    // 共用的 UID 输入框（查询状态/拉黑/解除拉黑都用同一个框）。
    return wrapper.findAll('input[placeholder="UID"]')[3]
  }

  it('点「拉黑」先弹确认对话框，不立即发请求', async () => {
    const f = vi.fn().mockImplementation((url: string) => {
      if (url.endsWith('/blocklist')) return Promise.resolve(ok([]))
      return Promise.resolve(ok({ status: 'ok' }))
    })
    vi.stubGlobal('fetch', f)

    const { router } = setupStores(['user:block'])
    const wrapper = await mountModeration(router)

    await blacklistUidInput(wrapper).setValue('10086')
    const blacklistBtn = wrapper.findAll('button').find((b) => b.text() === '拉黑')
    await blacklistBtn!.trigger('click')
    await flushPromises()

    expect(dialogWarningMock).toHaveBeenCalledTimes(1)
    const opts = dialogWarningMock.mock.calls[0][0] as { content: string; title: string }
    expect(opts.title).toContain('拉黑')
    expect(opts.content).toContain('10086')
    // 确认之前，POST .../blacklist 不该被发出去
    expect(
      f.mock.calls.some((c) => (c[0] as string).endsWith('/blacklist') && (c[1] as RequestInit | undefined)?.method === 'POST'),
    ).toBe(false)
  })

  it('确认拉黑后发 POST .../blacklist（不是 .../block），成功后自动重新查询状态', async () => {
    let blacklistBody: unknown = null
    let statusCalls = 0
    const f = vi.fn().mockImplementation((url: string, init?: RequestInit) => {
      if (url.endsWith('/blocklist')) return Promise.resolve(ok([]))
      if (url.endsWith('/blacklist') && init?.method === 'POST') {
        blacklistBody = init?.body ? JSON.parse(String(init.body)) : null
        return Promise.resolve(ok({ uid: '10086' }))
      }
      if (url.includes('/blacklist-status')) {
        statusCalls += 1
        return Promise.resolve(ok({ uid: '10086', blacklisted: true, nickname: '坏人甲' }))
      }
      return Promise.resolve(ok({ status: 'ok' }))
    })
    vi.stubGlobal('fetch', f)

    const { router } = setupStores(['user:block'])
    const wrapper = await mountModeration(router)

    await blacklistUidInput(wrapper).setValue('10086')
    const blacklistBtn = wrapper.findAll('button').find((b) => b.text() === '拉黑')
    await blacklistBtn!.trigger('click')
    await flushPromises()

    // 触发确认框里的 onPositiveClick，模拟用户点了「确认拉黑」
    const opts = dialogWarningMock.mock.calls[0][0] as { onPositiveClick: () => void }
    opts.onPositiveClick()
    await flushPromises()

    expect(blacklistBody).toEqual({ uid: '10086' })
    expect(messageMock.success).toHaveBeenCalledWith('已拉黑 UID 10086')
    // 绝不能打到禁言接口——这是这次任务要修的核心 bug
    expect(
      f.mock.calls.some((c) => (c[0] as string).endsWith('/block') && (c[1] as RequestInit | undefined)?.method === 'POST'),
    ).toBe(false)
    expect(statusCalls).toBeGreaterThanOrEqual(1)
    expect(wrapper.text()).toContain('已拉黑')
    expect(wrapper.text()).toContain('坏人甲')
  })

  it('点「解除拉黑」不弹确认框，直接发 POST .../unblacklist（不是 .../unblock）', async () => {
    let unblacklistBody: unknown = null
    const f = vi.fn().mockImplementation((url: string, init?: RequestInit) => {
      if (url.endsWith('/blocklist')) return Promise.resolve(ok([]))
      if (url.endsWith('/unblacklist') && init?.method === 'POST') {
        unblacklistBody = init?.body ? JSON.parse(String(init.body)) : null
        return Promise.resolve(ok({ uid: '10086' }))
      }
      if (url.includes('/blacklist-status')) {
        return Promise.resolve(ok({ uid: '10086', blacklisted: false, nickname: '' }))
      }
      return Promise.resolve(ok({ status: 'ok' }))
    })
    vi.stubGlobal('fetch', f)

    const { router } = setupStores(['user:block'])
    const wrapper = await mountModeration(router)

    await blacklistUidInput(wrapper).setValue('10086')
    const unblacklistBtn = wrapper.findAll('button').find((b) => b.text() === '解除拉黑')
    await unblacklistBtn!.trigger('click')
    await flushPromises()

    expect(dialogWarningMock).not.toHaveBeenCalled()
    expect(unblacklistBody).toEqual({ uid: '10086' })
    expect(messageMock.success).toHaveBeenCalledWith('已解除 UID 10086 的拉黑')
  })

  it('拉黑失败时把后端原文原样显示，不包装成笼统提示', async () => {
    const backendMessage = '拉黑失败: 你不是这个账号的所有者'
    const f = vi.fn().mockImplementation((url: string, init?: RequestInit) => {
      if (url.endsWith('/blocklist')) return Promise.resolve(ok([]))
      if (url.endsWith('/blacklist') && init?.method === 'POST') return Promise.resolve(err(502, backendMessage))
      return Promise.resolve(ok({ status: 'ok' }))
    })
    vi.stubGlobal('fetch', f)

    const { router } = setupStores(['user:block'])
    const wrapper = await mountModeration(router)

    await blacklistUidInput(wrapper).setValue('10086')
    const blacklistBtn = wrapper.findAll('button').find((b) => b.text() === '拉黑')
    await blacklistBtn!.trigger('click')
    await flushPromises()

    const opts = dialogWarningMock.mock.calls[0][0] as { onPositiveClick: () => void }
    opts.onPositiveClick()
    await flushPromises()

    expect(messageMock.error).toHaveBeenCalledWith(backendMessage)
  })

  it('点「查询状态」发 GET .../blacklist-status?uid=，展示 attribute==128 换算出的已拉黑状态与自动回填的昵称', async () => {
    const f = vi.fn().mockImplementation((url: string, init?: RequestInit) => {
      if (url.endsWith('/blocklist')) return Promise.resolve(ok([]))
      if (url.includes('/blacklist-status') && (!init || init.method === undefined || init.method === 'GET')) {
        expect(url).toContain('uid=10086')
        return Promise.resolve(ok({ uid: '10086', blacklisted: true, nickname: '坏人甲' }))
      }
      return Promise.resolve(ok({ status: 'ok' }))
    })
    vi.stubGlobal('fetch', f)

    const { router } = setupStores(['user:block'])
    const wrapper = await mountModeration(router)

    await blacklistUidInput(wrapper).setValue('10086')
    const queryBtn = wrapper.findAll('button').find((b) => b.text() === '查询状态')
    await queryBtn!.trigger('click')
    await flushPromises()

    expect(wrapper.text()).toContain('坏人甲')
    expect(wrapper.text()).toContain('已拉黑')
  })

  it('切换 UID 之后，上一次查到的状态清空，不会串到新 UID 上', async () => {
    const f = vi.fn().mockImplementation((url: string) => {
      if (url.endsWith('/blocklist')) return Promise.resolve(ok([]))
      if (url.includes('/blacklist-status')) {
        return Promise.resolve(ok({ uid: '10086', blacklisted: true, nickname: '坏人甲' }))
      }
      return Promise.resolve(ok({ status: 'ok' }))
    })
    vi.stubGlobal('fetch', f)

    const { router } = setupStores(['user:block'])
    const wrapper = await mountModeration(router)

    const uidInput = blacklistUidInput(wrapper)
    await uidInput.setValue('10086')
    const queryBtn = wrapper.findAll('button').find((b) => b.text() === '查询状态')
    await queryBtn!.trigger('click')
    await flushPromises()
    expect(wrapper.text()).toContain('坏人甲')

    await uidInput.setValue('20000')
    await flushPromises()
    expect(wrapper.text()).not.toContain('坏人甲')
  })
})

describe('Moderation 自动禁言/自动拉黑规则占位：跳转到还没注册的路由不抛异常', () => {
  beforeEach(clearMocks)

  it('『自定义弹幕姬』还没注册路由时点「去配置」（自动禁言）只提示，不抛异常', async () => {
    vi.stubGlobal(
      'fetch',
      vi.fn().mockImplementation(() => Promise.resolve(ok([]))),
    )
    const { router } = setupStores(['user:block'])
    const wrapper = await mountModeration(router)

    const goBtns = wrapper.findAll('button').filter((b) => b.text() === '去配置')
    expect(goBtns.length).toBe(2) // 自动禁言规则 + 自动拉黑规则各一个
    await expect(goBtns[0].trigger('click')).resolves.not.toThrow()
    await flushPromises()

    expect(messageMock.info).toHaveBeenCalled()
  })
})

describe('Moderation 自动禁言/自动拉黑规则：路由存在时点「去配置」带上预填 query', () => {
  beforeEach(clearMocks)

  // Task 14 收尾：custom 路由已经注册了，「去配置」真能跳过去，但只跳过去
  // 不够——不带 query 的话 Custom.vue 无从得知要预填。这条测试钉住跳转
  // 一定带着 preset=automute，防止将来重构时把这个 query 弄丢。
  it('「自动禁言规则」卡片的「去配置」带 query preset=automute', async () => {
    vi.stubGlobal(
      'fetch',
      vi.fn().mockImplementation(() => Promise.resolve(ok([]))),
    )
    const { router } = setupStores(['user:block'])
    router.addRoute({ name: 'custom', path: '/custom', component: { template: '<div/>' } })
    const wrapper = await mountModeration(router)

    const goBtns = wrapper.findAll('button').filter((b) => b.text() === '去配置')
    await goBtns[0].trigger('click')
    await flushPromises()

    expect(router.currentRoute.value.name).toBe('custom')
    expect(router.currentRoute.value.query.preset).toBe('automute')
    expect(messageMock.info).not.toHaveBeenCalled()
  })

  // P5-6 新增：「自动拉黑规则」卡片是独立的一张卡片，带独立的 preset，
  // 不与自动禁言共用同一条草稿骨架——这条测试如果两张卡片的按钮被误
  // 合并成一个，或者第二个按钮仍然带 preset=automute，会失败。
  it('「自动拉黑规则」卡片的「去配置」带 query preset=autoblacklist，不是 automute', async () => {
    vi.stubGlobal(
      'fetch',
      vi.fn().mockImplementation(() => Promise.resolve(ok([]))),
    )
    const { router } = setupStores(['user:block'])
    router.addRoute({ name: 'custom', path: '/custom', component: { template: '<div/>' } })
    const wrapper = await mountModeration(router)

    const goBtns = wrapper.findAll('button').filter((b) => b.text() === '去配置')
    expect(goBtns.length).toBe(2)
    await goBtns[1].trigger('click')
    await flushPromises()

    expect(router.currentRoute.value.name).toBe('custom')
    expect(router.currentRoute.value.query.preset).toBe('autoblacklist')
  })
})
