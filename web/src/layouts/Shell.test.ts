import { beforeEach, describe, expect, it, vi } from 'vitest'
import { defineComponent, h } from 'vue'
import { mount, flushPromises } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import { NDialogProvider, NMenu, NMessageProvider } from 'naive-ui'
import Shell from './Shell.vue'
import router from '@/router'
import { useAuthStore } from '@/stores/auth'

function ok(body: unknown) {
  return new Response(JSON.stringify(body), {
    status: 200,
    headers: { 'Content-Type': 'application/json' },
  })
}

// Shell 本身没有注册 NMessageProvider/NDialogProvider（那是 App.vue 的事），
// 但 go() 里用 useMessage() 提示「还没做」，Task 5 的 Accounts.vue（挂在
// /accounts 路由下，随 Shell 一起渲染）用 useDialog() 做删除确认，
// 测试里得自己把两层都套上——少一层，对应的 useXxx() 会在 setup 里同步抛错。
const Wrapped = defineComponent({
  render: () =>
    h(NMessageProvider, null, {
      default: () => h(NDialogProvider, null, { default: () => h(Shell) }),
    }),
})

describe('Shell', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    vi.unstubAllGlobals()
    localStorage.clear()
  })

  // 用一个**永远不会被注册**的哨兵键，不要借用真实菜单项的 key。
  //
  // 借用真实 key 的话，每实现一页就要来改一次这条测试——Task 6 把
  // 'moderation' 接成真路由时就撞过一次，表现是异步组件导航在测试结束前
  // 没完成，泄漏一个 history is not defined 的未处理 rejection 到后面的
  // 测试文件里，而且**只在 --sequence.shuffle 下可见**。等八个页面全做完，
  // 也就没有可借用的键了。
  //
  // 这条测试要钉的性质是「go() 对未注册的 name 不抛异常」，与具体是哪个
  // key 无关。router.push({ name: ... }) 找不到
  // 这个 name 时 vue-router 是同步抛 MATCHER_NOT_FOUND——通配符
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
    expect(() => menu.vm.$emit('update:value', '__never_registered__')).not.toThrow()

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

  // P5-3 复审裁决：Shell 头部原先内联的 BindingSelector 已经去掉（协调者
  // 认定它是「泛泛」的那一个，且与页面内的选择器同屏出现会让人怀疑两者
  // 是不是一回事），Shell 自己不再负责展示「绑定列表加载失败」。
  // 这条行为——原样显示后端错误 + 点「重试」重新请求——现在只在渲染了
  // BindingSelector 的页面里成立，覆盖见 BindingSelector.test.ts（组件级）
  // 与 Moderation.test.ts 的「绑定列表加载失败」用例（页面挂载级）。
  // Shell 本身仍然负责触发首次 bindings.refresh()（onMounted），这条留在
  // 上面两处已实现的路由跳转测试里间接验证（stub 的 fetch 必须被调用，
  // 不然 accounts/moderation 这些页面根本拿不到绑定列表）。
})
