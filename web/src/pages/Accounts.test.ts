import { beforeEach, describe, expect, it, vi } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'

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

const 小号: {
  id: number
  name: string
  uid: string
  rateLimitMs: number
  maxLength: number
  ownerId: number
  isOwner: boolean
  createdAt: string
} = {
  id: 1,
  name: '小号',
  uid: '10086',
  rateLimitMs: 1000,
  maxLength: 20,
  ownerId: 1,
  isOwner: true,
  createdAt: '',
}

function ok(body: unknown) {
  return new Response(JSON.stringify(body), {
    status: 200,
    headers: { 'Content-Type': 'application/json' },
  })
}

function stubFetch() {
  const f = vi.fn().mockImplementation((url: string) => {
    if (url === '/api/accounts') return Promise.resolve(ok([小号]))
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
