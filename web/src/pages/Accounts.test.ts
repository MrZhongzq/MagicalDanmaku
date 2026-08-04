import { beforeEach, describe, expect, it, vi } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import { useBindingsStore } from '@/stores/bindings'
import type { Binding } from '@/api'

// 删账号会连带删掉它名下的全部绑定与规则，这条测试要钉住的是
// 「没确认之前绝不能发 DELETE 请求」，而不只是「弹窗被叫起来了」。
//
// 用 vi.mock 顶掉 naive-ui 的 useDialog/useMessage，而不是挂真的
// NDialogProvider 走真实 DOM 点击：真弹窗牵涉 Teleport 到 document.body
// 和进场动画的时序，会让「点确认前没发请求」这类断言变得不确定；
// mock 拿到 dialog.warning 收到的 onPositiveClick 之后手动调用它，
// 等价于「用户点了确认框里的删除」，但是确定性的。
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

const { default: Accounts } = await import('./Accounts.vue')
const { default: QRCodeLogin } = await import('@/components/QRCodeLogin.vue')

const 小号: {
  id: number
  name: string
  uid: string
  rateLimitMs: number
  maxLength: number
  ownerId: number
  isOwner: boolean
  createdAt: string
  loginState: 'valid' | 'invalid' | 'unknown'
  loginCheckedAt: string | null
} = {
  id: 1,
  name: '小号',
  uid: '10086',
  rateLimitMs: 1000,
  maxLength: 20,
  ownerId: 1,
  isOwner: true,
  createdAt: '',
  loginState: 'unknown',
  loginCheckedAt: null,
}

function ok(body: unknown) {
  return new Response(JSON.stringify(body), {
    status: 200,
    headers: { 'Content-Type': 'application/json' },
  })
}

function stubFetch(accounts: unknown[] = [小号]) {
  const f = vi.fn().mockImplementation((url: string) => {
    if (url === '/api/accounts') return Promise.resolve(ok(accounts))
    if (url === '/api/bindings') return Promise.resolve(ok([]))
    return Promise.resolve(ok({ status: 'ok' }))
  })
  vi.stubGlobal('fetch', f)
  return f
}

function deleteCallCount(f: ReturnType<typeof vi.fn>): number {
  return f.mock.calls.filter((call) => {
    const init = call[1] as RequestInit | undefined
    return init?.method === 'DELETE'
  }).length
}

describe('Accounts 删除账号的二次确认', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    vi.unstubAllGlobals()
    localStorage.clear()
    warningMock.mockClear()
    messageMock.success.mockClear()
    messageMock.error.mockClear()
  })

  it('点删除账号先弹确认，确认之前不发 DELETE 请求；确认之后才发', async () => {
    const f = stubFetch()
    const wrapper = mount(Accounts)
    await flushPromises()

    const deleteBtn = wrapper.findAll('button').find((b) => b.text() === '删除账号')
    expect(deleteBtn).toBeTruthy()

    await deleteBtn!.trigger('click')
    await flushPromises()

    // 只是弹出确认框——dialog.warning 被调用了一次，但还没有人点「确认删除」
    expect(warningMock).toHaveBeenCalledTimes(1)
    expect(deleteCallCount(f)).toBe(0)

    // 模拟用户在确认框里点「删除」：调用 dialog.warning 收到的 onPositiveClick
    const opts = warningMock.mock.calls[0][0] as { onPositiveClick: () => void }
    opts.onPositiveClick()
    await flushPromises()

    expect(deleteCallCount(f)).toBe(1)
  })
})

// 登录状态三态：valid/invalid/unknown 必须显示成三种不同的文案与样式，
// 「待后端支持」标签要整个撤掉。unknown 尤其要验证不能被误读成「已失效」。
describe('Accounts 登录状态三态展示', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    vi.unstubAllGlobals()
    localStorage.clear()
    messageMock.success.mockClear()
    messageMock.error.mockClear()
    messageMock.warning.mockClear()
  })

  it('不再显示「待后端支持」标签', async () => {
    stubFetch()
    const wrapper = mount(Accounts)
    await flushPromises()

    expect(wrapper.text()).not.toContain('待后端支持')
  })

  it('valid：显示「登录有效」，不带失效高亮样式', async () => {
    stubFetch([{ ...小号, loginState: 'valid', loginCheckedAt: '2026-08-01T12:00:00Z' }])
    const wrapper = mount(Accounts)
    await flushPromises()

    expect(wrapper.text()).toContain('登录有效')
    expect(wrapper.find('.account-card--invalid').exists()).toBe(false)
  })

  it('invalid：显示「登录已失效」并高亮，重新扫码按钮跟着变色', async () => {
    stubFetch([{ ...小号, loginState: 'invalid', loginCheckedAt: '2026-08-01T12:00:00Z' }])
    const wrapper = mount(Accounts)
    await flushPromises()

    expect(wrapper.text()).toContain('登录已失效')
    expect(wrapper.find('.account-card--invalid').exists()).toBe(true)
    const rescanBtn = wrapper.findAll('button').find((b) => b.text() === '重新扫码')
    expect(rescanBtn).toBeTruthy()
    expect(rescanBtn!.classes().join(' ')).toContain('error')
  })

  it('unknown 且从未检测过（loginCheckedAt 为 null）：显示「尚未检测」，不是「已失效」', async () => {
    stubFetch([{ ...小号, loginState: 'unknown', loginCheckedAt: null }])
    const wrapper = mount(Accounts)
    await flushPromises()

    expect(wrapper.text()).toContain('尚未检测')
    expect(wrapper.text()).not.toContain('登录已失效')
    expect(wrapper.find('.account-card--invalid').exists()).toBe(false)
  })

  it('unknown 且曾经检测过（loginCheckedAt 非 null，说明上次探测失败）：显示「状态未知」，且说明不代表失效', async () => {
    stubFetch([{ ...小号, loginState: 'unknown', loginCheckedAt: '2026-08-01T12:00:00Z' }])
    const wrapper = mount(Accounts)
    await flushPromises()

    expect(wrapper.text()).toContain('状态未知')
    expect(wrapper.text()).not.toContain('登录已失效')

    // 详情文案常驻显示（不塞进悬浮提示），要说明「不代表失效」
    expect(wrapper.text()).toContain('不代表账号已失效')
  })

  // 后端现在扫码成功时会同步做一次立即检测（P5-2 任务 2），不再是
  // "换了 Cookie 就必须等 10 分钟"——提示文案要如实反映这一点，不能
  // 继续宣称"下一轮检测"才能看到结果，否则会误导用户以为立即检测
  // 这件事没做。
  it('扫码成功后的提示说明状态已立即检测，而不是宣称要等下一轮', async () => {
    stubFetch()
    const wrapper = mount(Accounts)
    await flushPromises()

    // 直接调用组件内部的成功回调路径：点击「重新扫码」打开弹窗，
    // 再触发 QRCodeLogin 的 success 事件，而不是真的走一遍扫码轮询。
    const rescanBtn = wrapper.findAll('button').find((b) => b.text() === '重新扫码')
    await rescanBtn!.trigger('click')
    await flushPromises()

    const qrLogin = wrapper.findComponent(QRCodeLogin)
    expect(qrLogin.exists()).toBe(true)
    qrLogin.vm.$emit('success', '小号')
    await flushPromises()

    const call = messageMock.success.mock.calls.find((c) => String(c[0]).includes('小号'))
    expect(call?.[0]).not.toContain('下一轮检测')
    expect(call?.[0]).toContain('已立即检测')
  })
})

// 直播间开播状态：与账号登录状态是同一套三态设计（living/offline/
// unknown），unknown 同样绝不能被误读成"未开播"。
describe('Accounts 直播间开播状态展示', () => {
  const 绑定fixture: Binding = {
    id: 100,
    accountId: 1,
    accountName: '小号',
    roomId: '1706666491',
    enabled: true,
    ruleCount: 0,
    permissions: [],
    isOwner: true,
    liveStatus: 'unknown',
    liveCheckedAt: null,
    anchorUid: '',
    anchorName: '',
  }

  beforeEach(() => {
    setActivePinia(createPinia())
    vi.unstubAllGlobals()
    localStorage.clear()
    messageMock.success.mockClear()
    messageMock.error.mockClear()
    messageMock.warning.mockClear()
  })

  function mountWithBinding(binding: Partial<Binding>) {
    stubFetch()
    const wrapper = mount(Accounts)
    useBindingsStore().list = [{ ...绑定fixture, ...binding }]
    return wrapper
  }

  // 断言全部限定在 .binding-row 内部，而不是整页文本搜索——账号登录态
  // 那一档本来就会渲染"尚未检测"/"状态未知"这类字样，整页搜索测不出
  // "绑定行到底有没有渲染直播间状态"，只会跟账号那一档的文案撞出假绿。
  function bindingRowText(wrapper: ReturnType<typeof mount>): string {
    const row = wrapper.find('.binding-row')
    expect(row.exists()).toBe(true)
    return row.text()
  }

  // 状态标签单独取（而不是从整行文本里找"未开播"）：详情说明文字里
  // 合法地会出现"不代表未开播"这样的解释性负述，与"标签本身写的是不是
  // 未开播"是两件不同的事，混在一起断言会自相矛盾。
  function bindingLiveTagText(wrapper: ReturnType<typeof mount>): string {
    const tag = wrapper.find('.binding-live-tag')
    expect(tag.exists()).toBe(true)
    return tag.text()
  }

  it('living：显示"直播中"', async () => {
    const wrapper = mountWithBinding({ liveStatus: 'living', liveCheckedAt: '2026-08-01T12:00:00Z' })
    await flushPromises()
    expect(bindingLiveTagText(wrapper)).toContain('直播中')
  })

  it('offline：显示"未开播"', async () => {
    const wrapper = mountWithBinding({ liveStatus: 'offline', liveCheckedAt: '2026-08-01T12:00:00Z' })
    await flushPromises()
    expect(bindingLiveTagText(wrapper)).toContain('未开播')
  })

  // 自检项 (a)：让开播状态接口失败（liveStatus 停在 unknown），确认
  // 界面显示"未知"/"尚未检测"而不是"未开播"——绝不能因为接口失败就
  // 显示成"未开播"，那是在用一个看起来正常的错误答案骗用户。
  it('unknown 且从未检测过：显示"尚未检测"，不是"未开播"', async () => {
    const wrapper = mountWithBinding({ liveStatus: 'unknown', liveCheckedAt: null })
    await flushPromises()
    expect(bindingLiveTagText(wrapper)).toContain('尚未检测')
    expect(bindingLiveTagText(wrapper)).not.toContain('未开播')
  })

  it('unknown 且曾经探测失败（liveCheckedAt 非 null）：显示"状态未知"，且说明不代表未开播', async () => {
    const wrapper = mountWithBinding({ liveStatus: 'unknown', liveCheckedAt: '2026-08-01T12:00:00Z' })
    await flushPromises()
    expect(bindingLiveTagText(wrapper)).toContain('状态未知')
    expect(bindingLiveTagText(wrapper)).not.toContain('未开播')
    expect(bindingRowText(wrapper)).toContain('不代表未开播')
  })

  it('显示主播 UID 与昵称，而不只是房间号', async () => {
    const wrapper = mountWithBinding({ anchorUid: '20285041', anchorName: '舞月雅白' })
    await flushPromises()
    const text = bindingRowText(wrapper)
    expect(text).toContain('20285041')
    expect(text).toContain('舞月雅白')
  })

  it('主播身份尚未探测到时，UID 显示为"未知"而不是空白', async () => {
    const wrapper = mountWithBinding({ anchorUid: '', anchorName: '' })
    await flushPromises()
    expect(bindingRowText(wrapper)).toContain('未知')
  })
})
