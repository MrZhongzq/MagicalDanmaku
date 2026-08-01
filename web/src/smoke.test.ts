import { describe, expect, it, vi } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import App from './App.vue'
import router from './router'

describe('App', () => {
  // App.vue 现在只是路由的容器（NConfigProvider + NMessageProvider +
  // NDialogProvider + RouterView），本身不再直接渲染文案。
  // 未登录时守卫会把人送去 /login，借这条路径验证 App 能正常挂上路由
  // 并渲染出页面，而不是一片空白。
  //
  // 断言用 .login-wrap 这个结构标记而不是「神奇弹幕」这句文案——
  // 文案是登录卡片标题，将来改了会让这条本该测路由/守卫的 smoke test
  // 无端变红，看起来跟真正改动的东西毫不相关。
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

    expect(wrapper.find('.login-wrap').exists()).toBe(true)

    vi.unstubAllGlobals()
  })
})
