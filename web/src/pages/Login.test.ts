import { describe, expect, it } from 'vitest'
import { defineComponent, h } from 'vue'
import { mount } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import { createRouter, createMemoryHistory } from 'vue-router'
import { NLayout, NMessageProvider } from 'naive-ui'
import Login from './Login.vue'

// Login.vue 用 useMessage() 提示登录失败，要求外层有 NMessageProvider——
// 与 Shell.test.ts 处理同一类依赖的方式一致（那是 App.vue 实际负责套的层）。
const Wrapped = defineComponent({
  render: () => h(NMessageProvider, null, { default: () => h(Login) }),
})

// 反馈原话：登录页外层背景是白的、卡片是黑的，看着像没做完。全站的深色
// 背景实际是 NLayout 这类组件按 App.vue 的 darkTheme 主题刷出来的（见
// Shell.vue），不是某处写死的颜色值——登录页此前用裸 div 撑满全屏，
// 没有任何组件负责刷色，浏览器默认白色背景就露出来了。这里钉住登录页
// 换成了同样的 NLayout，不是继续用裸 div。
function testRouter() {
  return createRouter({
    history: createMemoryHistory(),
    routes: [
      { path: '/login', name: 'login', component: Login },
      { path: '/accounts', name: 'accounts', component: { template: '<div/>' } },
    ],
  })
}

describe('Login 外层背景跟随全站深色主题，不再是裸 div', () => {
  it('登录卡片外层是 NLayout（自动刷成主题背景色），而不是没有背景色的裸 div', async () => {
    setActivePinia(createPinia())
    const router = testRouter()
    await router.push('/login')
    await router.isReady()
    const wrapper = mount(Wrapped, { global: { plugins: [router] } })

    expect(wrapper.findComponent(NLayout).exists()).toBe(true)
  })

  it('登录表单与提示文案仍然正常渲染（回归：换外层容器不能把内容换没了）', async () => {
    setActivePinia(createPinia())
    const router = testRouter()
    await router.push('/login')
    await router.isReady()
    const wrapper = mount(Wrapped, { global: { plugins: [router] } })

    expect(wrapper.text()).toContain('神奇弹幕')
    expect(wrapper.text()).toContain('magicd migrate')
    expect(wrapper.findAll('button').some((b) => b.text() === '登录')).toBe(true)
  })
})
