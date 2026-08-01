import { describe, expect, it, vi } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import App from './App.vue'
import router from './router'

describe('App', () => {
  // App.vue 现在只是路由的容器（NConfigProvider + NMessageProvider +
  // NDialogProvider + RouterView），本身不再直接渲染文案。
  // 未登录时守卫会把人送去 /login，那里的登录卡片标题正是「神奇弹幕」，
  // 借这条路径验证 App 能正常挂上路由并渲染出页面，而不是一片空白。
  it('未登录时挂载能落到登录页', async () => {
    setActivePinia(createPinia())
    vi.stubGlobal(
      'fetch',
      vi.fn().mockResolvedValue(
        new Response(JSON.stringify({ error: '未登录' }), {
          status: 401,
          headers: { 'Content-Type': 'application/json' },
        }),
      ),
    )

    await router.push('/')
    await router.isReady()
    const wrapper = mount(App, { global: { plugins: [router] } })
    await flushPromises()

    expect(wrapper.text()).toContain('神奇弹幕')

    vi.unstubAllGlobals()
  })
})
