import { beforeEach, describe, expect, it, vi } from 'vitest'
import { defineComponent, h } from 'vue'
import { mount, flushPromises } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import { NMenu, NMessageProvider } from 'naive-ui'
import Shell from './Shell.vue'
import router from '@/router'
import { useAuthStore } from '@/stores/auth'

function ok(body: unknown) {
  return new Response(JSON.stringify(body), {
    status: 200,
    headers: { 'Content-Type': 'application/json' },
  })
}

// Shell 本身没有注册 NMessageProvider（那是 App.vue 的事），
// 但 go() 里用 useMessage() 提示「还没做」，测试里得自己套一层。
const Wrapped = defineComponent({
  render: () => h(NMessageProvider, null, { default: () => h(Shell) }),
})

describe('Shell', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    vi.unstubAllGlobals()
    localStorage.clear()
  })

  // 菜单里的「房管」「弹幕姬」等七项还没接路由。router.push({ name: 'moderation' })
  // 找不到这个 name 时 vue-router 是同步抛 MATCHER_NOT_FOUND——通配符
  // /:pathMatch(.*)* 只兜未解析的 path，不兜未解析的 name。go() 是从
  // NMenu 的 @update:value 事件调的，Vue 在 dev 模式下会把这个同步异常
  // 重新抛出去，所以点一下就是一条真实的未捕获错误。
  it('点未实现的菜单项不抛异常，只是停在原地', async () => {
    vi.stubGlobal(
      'fetch',
      vi.fn().mockImplementation(() => Promise.resolve(ok([]))),
    )

    const auth = useAuthStore()
    auth.user = { id: 1, username: '张三', isAdmin: false, createdAt: '' }
    // 跳过 fetchMe：路由守卫看 loading 才决定要不要再问一次后端
    auth.loading = false

    await router.push('/accounts')
    await router.isReady()

    const wrapper = mount(Wrapped, { global: { plugins: [router] } })
    await flushPromises()

    const menu = wrapper.findComponent(NMenu)
    expect(() => menu.vm.$emit('update:value', 'moderation')).not.toThrow()

    await flushPromises()
    // 没有跳转：路由还停在 accounts
    expect(router.currentRoute.value.name).toBe('accounts')
  })

  it('点已实现的菜单项正常跳转', async () => {
    vi.stubGlobal(
      'fetch',
      vi.fn().mockImplementation(() => Promise.resolve(ok([]))),
    )

    const auth = useAuthStore()
    auth.user = { id: 1, username: '张三', isAdmin: false, createdAt: '' }
    auth.loading = false

    await router.push('/accounts')
    await router.isReady()

    const wrapper = mount(Wrapped, { global: { plugins: [router] } })
    await flushPromises()

    const menu = wrapper.findComponent(NMenu)
    menu.vm.$emit('update:value', 'accounts')
    await flushPromises()

    expect(router.currentRoute.value.name).toBe('accounts')
  })
})
