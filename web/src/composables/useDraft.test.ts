/* eslint-disable vue/one-component-per-file --
 * 这是测试文件，不是页面组件：两个 defineComponent 都是轻量宿主壳
 * （一个给普通 mountDraftHost 用，一个专门给"离开拦截"那组测试套一层
 * 真实 vue-router 用），拆成两个文件反而更难对照着看。
 */
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { defineComponent, h, ref, type Ref } from 'vue'
import { mount, flushPromises } from '@vue/test-utils'
import { createRouter, createMemoryHistory, RouterView } from 'vue-router'
import { useDraft, type UseDraftOptions, type UseDraftReturn } from './useDraft'
import type { Rule, RuleView } from '@/api/rule-types'

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
 * mountDraftHost 用一个渲染空 div 的宿主组件把 useDraft 跑起来，拿到真实的
 * UseDraftReturn（引用，不经模板插值），这样断言 `.value` 时不用纠结
 * Vue 模板层面的 ref 自动解包规则。
 */
function mountDraftHost(options: UseDraftOptions) {
  let result!: UseDraftReturn
  const Host = defineComponent({
    setup() {
      result = useDraft(options)
      return () => h('div')
    },
  })
  const wrapper = mount(Host)
  return { wrapper, draft: result }
}

beforeEach(() => {
  vi.unstubAllGlobals()
  vi.restoreAllMocks()
})

describe('useDraft：dirty 与 markSaved', () => {
  it('初始时 dirty 为假；草稿变化后为真；markSaved 之后回到假', () => {
    const state = ref('初始草稿')
    const { draft } = mountDraftHost({
      bindingId: () => 1,
      snapshot: () => state.value,
      isOwned: () => false,
      buildRules: () => [],
    })

    expect(draft.dirty.value).toBe(false)

    state.value = '改过的草稿'
    expect(draft.dirty.value).toBe(true)

    draft.markSaved()
    expect(draft.dirty.value).toBe(false)

    // markSaved 之后基线是"改过的草稿"，再改一次要能重新检测到
    state.value = '又改了一次'
    expect(draft.dirty.value).toBe(true)
  })
})

describe('useDraft：save() 正常流程——GET → 合并 → PUT → POST reload', () => {
  it('依次发出三个请求，PUT 请求体是"保留非本页规则 + 本页新建的规则"，成功后 dirty 归假', async () => {
    const existingRules: RuleView[] = [
      {
        name: '别的页面的规则',
        position: 0,
        on: ['gift'],
        do: [{ type: 'danmaku', template: ['x'] }],
      },
      {
        name: '本页管的规则',
        position: 1,
        enabled: false,
        on: ['danmaku'],
        do: [{ type: 'danmaku', template: ['旧的'] }],
      },
    ]
    const rebuilt: Rule = {
      name: '本页管的规则',
      enabled: true,
      on: ['danmaku'],
      do: [{ type: 'danmaku', template: ['新的'] }],
    }

    const calls: { url: string; init?: RequestInit }[] = []
    const f = vi.fn().mockImplementation((url: string, init?: RequestInit) => {
      calls.push({ url, init })
      if (
        url === '/api/bindings/1/rules' &&
        (!init || init.method === undefined || init.method === 'GET')
      ) {
        return Promise.resolve(ok(existingRules))
      }
      if (url === '/api/bindings/1/rules' && init?.method === 'PUT') {
        return Promise.resolve(ok({ status: 'ok' }))
      }
      if (url === '/api/bindings/1/reload' && init?.method === 'POST') {
        return Promise.resolve(ok({ status: 'ok' }))
      }
      throw new Error(`unexpected fetch: ${init?.method ?? 'GET'} ${url}`)
    })
    vi.stubGlobal('fetch', f)

    const state = ref('a')
    const { draft } = mountDraftHost({
      bindingId: () => 1,
      snapshot: () => state.value,
      isOwned: (name) => name === '本页管的规则',
      buildRules: () => [rebuilt],
    })
    state.value = 'b' // 制造 dirty，模拟用户改过草稿
    expect(draft.dirty.value).toBe(true)

    await draft.save()

    expect(calls.map((c) => `${c.init?.method ?? 'GET'} ${c.url}`)).toEqual([
      'GET /api/bindings/1/rules',
      'PUT /api/bindings/1/rules',
      'POST /api/bindings/1/reload',
    ])

    const putBody = JSON.parse(calls[1].init!.body as string) as Rule[]
    expect(putBody).toEqual([
      { name: '别的页面的规则', on: ['gift'], do: [{ type: 'danmaku', template: ['x'] }] },
      rebuilt,
    ])
    // GET 回来的 RuleView 带 position，PUT 回去的必须去掉——后端用
    // DisallowUnknownFields()，带着 position 会被 422 拒收。
    expect('position' in putBody[0]).toBe(false)

    expect(draft.dirty.value).toBe(false)
    expect(draft.partialFailureMessage.value).toBeNull()
  })

  it('bindingId 为 null（未选中直播间）时 save() 直接返回，不发任何请求', async () => {
    const f = vi.fn()
    vi.stubGlobal('fetch', f)
    const { draft } = mountDraftHost({
      bindingId: () => null,
      snapshot: () => 'x',
      isOwned: () => false,
      buildRules: () => [],
    })
    await draft.save()
    expect(f).not.toHaveBeenCalled()
  })
})

describe('useDraft：save() 合并逻辑【关键：钉死"整组替换不误删别的页面的规则"】', () => {
  it('本页只认领同名规则，不属于本页的规则原样保留在 PUT 请求体里', async () => {
    const otherPagesOwnRule: RuleView = {
      name: '用户自建的规则',
      position: 0,
      on: ['user_enter'],
      do: [{ type: 'danmaku', template: ['自建模板'] }],
    }
    let putBody: Rule[] | null = null
    const f = vi.fn().mockImplementation((url: string, init?: RequestInit) => {
      if (url === '/api/bindings/1/rules' && (!init || !init.method || init.method === 'GET')) {
        return Promise.resolve(ok([otherPagesOwnRule]))
      }
      if (url === '/api/bindings/1/rules' && init?.method === 'PUT') {
        putBody = JSON.parse(init.body as string) as Rule[]
        return Promise.resolve(ok({ status: 'ok' }))
      }
      if (url === '/api/bindings/1/reload' && init?.method === 'POST') {
        return Promise.resolve(ok({ status: 'ok' }))
      }
      throw new Error(`unexpected fetch: ${init?.method ?? 'GET'} ${url}`)
    })
    vi.stubGlobal('fetch', f)

    const { draft } = mountDraftHost({
      bindingId: () => 1,
      snapshot: () => 'x',
      // 本页只管"内置/进房欢迎"这一个名字——与 otherPagesOwnRule 的名字不同
      isOwned: (name) => name === '内置/进房欢迎',
      buildRules: () => [{ name: '内置/进房欢迎', enabled: true, on: ['user_enter'], do: [] }],
    })

    await draft.save()

    expect(putBody).not.toBeNull()
    // 这条断言是本测试的核心：不属于本页的"用户自建的规则"必须还在请求体里。
    const names = putBody!.map((r) => r.name)
    expect(names).toContain('用户自建的规则')
    expect(names).toContain('内置/进房欢迎')
    expect(putBody).toHaveLength(2)
  })
})

describe('useDraft：save() 第 1 步（PUT 写库）失败——库和引擎都没被动过', () => {
  it('抛出后端错误，dirty 保持真，partialFailureMessage 保持 null，且不会再去调 reload', async () => {
    const calls: string[] = []
    const f = vi.fn().mockImplementation((url: string, init?: RequestInit) => {
      calls.push(`${init?.method ?? 'GET'} ${url}`)
      if (url === '/api/bindings/1/rules' && (!init || !init.method || init.method === 'GET')) {
        return Promise.resolve(ok([]))
      }
      if (url === '/api/bindings/1/rules' && init?.method === 'PUT') {
        return Promise.resolve(err(422, '第 1 条规则(x)不合法: 正则非法'))
      }
      return Promise.resolve(ok({ status: 'ok' }))
    })
    vi.stubGlobal('fetch', f)

    const state = ref('a')
    const { draft } = mountDraftHost({
      bindingId: () => 1,
      snapshot: () => state.value,
      isOwned: () => true,
      buildRules: () => [{ name: 'x', enabled: true, on: ['gift'], do: [] }],
    })
    state.value = 'b'

    await expect(draft.save()).rejects.toThrow('第 1 条规则(x)不合法')

    expect(calls).toEqual(['GET /api/bindings/1/rules', 'PUT /api/bindings/1/rules'])
    expect(draft.dirty.value).toBe(true)
    expect(draft.partialFailureMessage.value).toBeNull()
  })
})

describe('useDraft：save() 第 2 步（reload）失败——库已经改了，引擎还在跑旧配置', () => {
  it('dirty 不归假，partialFailureMessage 原样带上后端"仍在用上一份配置运行"的安抚文案', async () => {
    const reloadErrorMessage = '重载失败，仍在用上一份配置运行: 规则 X 的正则非法'
    const f = vi.fn().mockImplementation((url: string, init?: RequestInit) => {
      if (url === '/api/bindings/1/rules' && (!init || !init.method || init.method === 'GET')) {
        return Promise.resolve(ok([]))
      }
      if (url === '/api/bindings/1/rules' && init?.method === 'PUT') {
        return Promise.resolve(ok({ status: 'ok' }))
      }
      if (url === '/api/bindings/1/reload' && init?.method === 'POST') {
        return Promise.resolve(err(422, reloadErrorMessage))
      }
      throw new Error('unexpected fetch')
    })
    vi.stubGlobal('fetch', f)

    const state = ref('a')
    const { draft } = mountDraftHost({
      bindingId: () => 1,
      snapshot: () => state.value,
      isOwned: () => true,
      buildRules: () => [{ name: 'x', enabled: true, on: ['gift'], do: [] }],
    })
    state.value = 'b'

    await expect(draft.save()).rejects.toThrow(reloadErrorMessage)

    expect(draft.dirty.value).toBe(true)
    expect(draft.partialFailureMessage.value).toBe(reloadErrorMessage)
  })
})

describe('useDraft：离开拦截——应用内路由跳转（onBeforeRouteLeave）', () => {
  function setupRouterHost(dirtyRef: Ref<boolean>) {
    let draft!: UseDraftReturn
    const HostPage = defineComponent({
      setup() {
        draft = useDraft({
          bindingId: () => 1,
          snapshot: () => (dirtyRef.value ? 'dirty' : 'clean'),
          isOwned: () => false,
          buildRules: () => [],
        })
        // 基线固定为 'clean'，这样 dirtyRef 直接决定 dirty 的值
        return () => h('div')
      },
    })
    const router = createRouter({
      history: createMemoryHistory(),
      routes: [
        { path: '/a', component: HostPage },
        { path: '/b', component: { render: () => h('div') } },
      ],
    })
    return { router, getDraft: () => draft }
  }

  it('dirty 为真时离开会先弹确认框；确认框返回 false 就取消导航', async () => {
    const dirtyRef = ref(false)
    const { router } = setupRouterHost(dirtyRef)
    await router.push('/a')
    await router.isReady()
    mount({ render: () => h(RouterView) }, { global: { plugins: [router] } })

    dirtyRef.value = true
    const confirmSpy = vi.spyOn(window, 'confirm').mockReturnValue(false)

    await router.push('/b')
    await flushPromises()

    expect(confirmSpy).toHaveBeenCalledTimes(1)
    expect(router.currentRoute.value.path).toBe('/a') // 导航被取消，还在原地
  })

  it('确认框返回 true 就放行导航', async () => {
    // 基线在 useDraft 初始化那一刻捕获，必须先挂载（此时 dirtyRef 还是
    // false，基线是 'clean'），再把 dirtyRef 改成 true 制造 dirty——
    // 如果一开始就是 true，基线会直接是 'dirty'，dirty 反而算出假。
    const dirtyRef = ref(false)
    const { router } = setupRouterHost(dirtyRef)
    await router.push('/a')
    await router.isReady()
    mount({ render: () => h(RouterView) }, { global: { plugins: [router] } })
    dirtyRef.value = true

    const confirmSpy = vi.spyOn(window, 'confirm').mockReturnValue(true)
    await router.push('/b')
    await flushPromises()

    expect(confirmSpy).toHaveBeenCalledTimes(1)
    expect(router.currentRoute.value.path).toBe('/b')
  })

  it('dirty 为假时离开不弹确认框，直接放行', async () => {
    const dirtyRef = ref(false)
    const { router } = setupRouterHost(dirtyRef)
    await router.push('/a')
    await router.isReady()
    mount({ render: () => h(RouterView) }, { global: { plugins: [router] } })

    const confirmSpy = vi.spyOn(window, 'confirm')
    await router.push('/b')
    await flushPromises()

    expect(confirmSpy).not.toHaveBeenCalled()
    expect(router.currentRoute.value.path).toBe('/b')
  })
})

describe('useDraft：离开拦截——整页刷新/关闭标签页（beforeunload）', () => {
  it('dirty 为真时阻止默认行为（浏览器自己弹通用确认框，文案不可控）', () => {
    const addSpy = vi.spyOn(window, 'addEventListener')
    const state = ref('a')
    const { draft } = mountDraftHost({
      bindingId: () => 1,
      snapshot: () => state.value,
      isOwned: () => false,
      buildRules: () => [],
    })
    state.value = 'b'
    expect(draft.dirty.value).toBe(true)

    const call = addSpy.mock.calls.find(([type]) => type === 'beforeunload')
    expect(call, '应该注册了 beforeunload 监听').toBeTruthy()
    const handler = call![1] as (e: Event) => void

    const fakeEvent = { preventDefault: vi.fn() } as unknown as BeforeUnloadEvent
    handler(fakeEvent)
    expect(fakeEvent.preventDefault).toHaveBeenCalled()
  })

  it('dirty 为假时不阻止默认行为', () => {
    const addSpy = vi.spyOn(window, 'addEventListener')
    const { draft } = mountDraftHost({
      bindingId: () => 1,
      snapshot: () => 'x',
      isOwned: () => false,
      buildRules: () => [],
    })
    expect(draft.dirty.value).toBe(false)

    const call = addSpy.mock.calls.find(([type]) => type === 'beforeunload')
    const handler = call![1] as (e: Event) => void
    const fakeEvent = { preventDefault: vi.fn() } as unknown as BeforeUnloadEvent
    handler(fakeEvent)
    expect(fakeEvent.preventDefault).not.toHaveBeenCalled()
  })

  it('组件卸载时移除 beforeunload 监听，不留内存泄漏', () => {
    const removeSpy = vi.spyOn(window, 'removeEventListener')
    const { wrapper } = mountDraftHost({
      bindingId: () => 1,
      snapshot: () => 'x',
      isOwned: () => false,
      buildRules: () => [],
    })
    wrapper.unmount()
    expect(removeSpy.mock.calls.some(([type]) => type === 'beforeunload')).toBe(true)
  })
})
