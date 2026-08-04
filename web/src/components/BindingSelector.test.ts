import { describe, expect, it } from 'vitest'
import { mount } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import { NButton, NSelect, NSpin } from 'naive-ui'
import BindingSelector from './BindingSelector.vue'
import { useAuthStore } from '@/stores/auth'
import { useBindingsStore } from '@/stores/bindings'
import type { Binding } from '@/api'

// BindingSelector 是从 Shell.vue 头部抽出来的唯一实现，铺到各页面正文里，
// 落地用户原话「每个栏目都要账号+直播间选择」。这里既要钉住基本的展示/切换，
// 也要钉住 P4 定下的硬规矩——权限不足只提示、绝不锁死选择器本身。
const 绑定A: Binding = {
  id: 1,
  accountId: 1,
  accountName: '小号',
  roomId: '123',
  enabled: true,
  ruleCount: 0,
  permissions: ['rule:read'],
  isOwner: true,
  liveStatus: 'unknown',
  liveCheckedAt: null,
  anchorUid: '',
  anchorName: '',
}

const 绑定B: Binding = {
  ...绑定A,
  id: 2,
  accountName: '大号',
  roomId: '456',
  enabled: false,
  permissions: ['rule:read', 'rule:write'],
}

function setup() {
  setActivePinia(createPinia())
  const bindings = useBindingsStore()
  const auth = useAuthStore()
  auth.user = { id: 1, username: '张三', isAdmin: false, createdAt: '' }
  return { bindings, auth }
}

describe('BindingSelector 展示与切换', () => {
  it('候选项文案是「账号 @ 房间」，停用的绑定带（已停用）后缀', () => {
    const { bindings } = setup()
    bindings.list = [绑定A, 绑定B]
    bindings.select(1)
    const wrapper = mount(BindingSelector)

    const select = wrapper.findComponent(NSelect)
    const options = select.props('options') as { label: string; value: number }[]
    expect(options).toEqual([
      { label: '小号 @ 123', value: 1 },
      { label: '大号 @ 456（已停用）', value: 2 },
    ])
  })

  // P6 任务 1：用户原话「记房间号有点困难」——主播 UID/昵称在 P5-2
  // 已经取回来了（账号/直播间页显示的「主播 UID xxx · 桃酥Su--」用的
  // 就是同一份数据），选择器直接复用 binding.anchorName，不再调接口。
  it('主播昵称存在时，候选项文案在房间号后面追加「· 昵称」', () => {
    const { bindings } = setup()
    const 绑定含昵称: Binding = { ...绑定A, anchorName: '桃酥Su--' }
    bindings.list = [绑定含昵称]
    bindings.select(1)
    const wrapper = mount(BindingSelector)

    const select = wrapper.findComponent(NSelect)
    const options = select.props('options') as { label: string; value: number }[]
    expect(options[0].label).toBe('小号 @ 123 · 桃酥Su--')
  })

  // 降级要求：昵称拿不到时（探测失败、尚未探测过）仍要显示房间号，
  // 不能显示成空或「未知」把房间号挤掉——这是用户能找到自己直播间的
  // 唯一线索，昵称只是锦上添花。
  it('主播昵称拿不到时，仍然只显示「账号 @ 房间号」，不显示成空或「未知」', () => {
    const { bindings } = setup()
    bindings.list = [绑定A] // anchorName 是空串（尚未探测过/探测失败）
    bindings.select(1)
    const wrapper = mount(BindingSelector)

    const select = wrapper.findComponent(NSelect)
    const options = select.props('options') as { label: string; value: number }[]
    expect(options[0].label).toBe('小号 @ 123')
    expect(options[0].label).not.toContain('未知')
  })

  it('昵称 + 已停用 + 缺权限三者同时出现时，顺序是「账号 @ 房间号 · 昵称（已停用）（无权限）」', () => {
    const { bindings } = setup()
    // 绑定A 本来就没有 rule:write（permissions 只有 rule:read），
    // 这里再叠加已停用与昵称，凑齐三种后缀同时出现的情况。
    const 绑定丙: Binding = { ...绑定A, enabled: false, anchorName: '花花' }
    bindings.list = [绑定丙]
    bindings.select(1)
    const wrapper = mount(BindingSelector, { props: { requiredPerm: 'rule:write' } })

    const select = wrapper.findComponent(NSelect)
    const options = select.props('options') as { label: string; value: number }[]
    expect(options[0].label).toBe('小号 @ 123 · 花花（已停用）（无 rule:write 权限）')
  })

  it('选中值反映当前绑定，切换时调用 bindings.select', async () => {
    const { bindings } = setup()
    bindings.list = [绑定A, 绑定B]
    bindings.select(1)
    const wrapper = mount(BindingSelector)

    expect(wrapper.findComponent(NSelect).props('value')).toBe(1)

    wrapper.findComponent(NSelect).vm.$emit('update:value', 2)
    expect(bindings.currentId).toBe(2)
  })

  it('加载中显示 NSpin，不渲染 NSelect', () => {
    const { bindings } = setup()
    bindings.loading = true
    const wrapper = mount(BindingSelector)

    expect(wrapper.findComponent(NSpin).exists()).toBe(true)
    expect(wrapper.findComponent(NSelect).exists()).toBe(false)
  })

  it('加载失败时原样显示后端错误并给出重试按钮，点击重试触发 bindings.refresh', async () => {
    const { bindings } = setup()
    bindings.loadError = '数据库连不上'
    const wrapper = mount(BindingSelector)

    expect(wrapper.text()).toContain('数据库连不上')
    const retryBtn = wrapper.findAllComponents(NButton).find((b) => b.text() === '重试')
    expect(retryBtn).toBeTruthy()

    // refresh 会真的发请求，这里只关心「点了会调用它」，不关心网络结果，
    // 所以不 stub fetch、也不等 flushPromises——请求失败与否不影响这条断言。
    let refreshCalled = false
    bindings.refresh = async () => {
      refreshCalled = true
    }
    await retryBtn!.trigger('click')
    expect(refreshCalled).toBe(true)
  })
})

describe('BindingSelector 权限提示：标注但绝不锁死', () => {
  // 自检变异 (b)：如果有人把这里改成「缺权限就 disabled」，这条测试必须变红。
  it('传入 requiredPerm 且当前账号在某绑定缺该权限时，选项文案标注缺权限，但选择器本身不 disabled', () => {
    const { bindings } = setup()
    bindings.list = [绑定A, 绑定B] // 绑定A 没有 rule:write，绑定B 有
    bindings.select(1)
    const wrapper = mount(BindingSelector, { props: { requiredPerm: 'rule:write' } })

    const select = wrapper.findComponent(NSelect)
    const options = select.props('options') as { label: string; value: number }[]
    expect(options[0].label).toContain('无 rule:write 权限')
    expect(options[1].label).not.toContain('无 rule:write 权限')

    // 核心硬性要求：不管权限够不够，选择器都必须可操作
    expect(select.props('disabled')).toBeFalsy()
    expect(wrapper.find('div[aria-disabled="true"]').exists()).toBe(false)
  })

  it('不传 requiredPerm 时选项文案不带任何权限标注', () => {
    const { bindings } = setup()
    bindings.list = [绑定A, 绑定B]
    bindings.select(1)
    const wrapper = mount(BindingSelector)

    const options = wrapper.findComponent(NSelect).props('options') as { label: string }[]
    options.forEach((o) => expect(o.label).not.toContain('权限'))
  })

  it('管理员即使 permissions 里没有该权限点，也不会被标注缺权限（后端对管理员放行全部权限点）', () => {
    const { bindings, auth } = setup()
    auth.user = { id: 1, username: '管理员', isAdmin: true, createdAt: '' }
    bindings.list = [绑定A] // permissions 里没有 rule:write
    bindings.select(1)
    const wrapper = mount(BindingSelector, { props: { requiredPerm: 'rule:write' } })

    const options = wrapper.findComponent(NSelect).props('options') as { label: string }[]
    expect(options[0].label).not.toContain('权限')
  })
})
