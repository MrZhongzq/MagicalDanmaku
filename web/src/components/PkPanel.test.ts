import { describe, expect, it } from 'vitest'
import { mount } from '@vue/test-utils'
import PkPanel, { PK_RULE_NAME, buildPkRule, defaultPkDraft, parsePkDraft } from './PkPanel.vue'

// PkPanel 是「PK 播报」整块——设计文档 §7.2 要的对面数据（主播昵称、
// 直播间人数、大航海总数/在线数）与串门欢迎，后端 event.Battle
// （server/internal/event/payload.go 第 119-121 行）目前只有一个
// SubCommand 字符串字段，什么都取不到。这组测试要证明：
//   1. 纯函数层面，buildPkRule 组装出的规则形状是对的（name/on 固定，
//      只覆盖"PK 匹配信息"，不把 visit 相关字段塞进规则里）
//   2. 组件层面，控件全部渲染且不 disabled（评审要看得到、点得动）
//   3. 悬空标记（"待后端支持"标签）确实出现在两个子块上

describe('PkPanel 纯函数：build/parse 往返', () => {
  it('buildPkRule 用固定 name 与 on: ["battle"]，do 里只装 matchTemplates', () => {
    const draft = defaultPkDraft()
    const rule = buildPkRule(draft)
    expect(rule.name).toBe(PK_RULE_NAME)
    expect(rule.on).toEqual(['battle'])
    expect(rule.do).toEqual([{ type: 'danmaku', template: draft.matchTemplates }])
    // 串门欢迎相关字段完全不出现在规则里——它们连触发事件类型都没定下来
    expect(JSON.stringify(rule)).not.toContain('visit')
  })

  it('parsePkDraft(null) 返回默认草稿', () => {
    expect(parsePkDraft(null)).toEqual(defaultPkDraft())
  })

  it('parsePkDraft 还原 enabled 与 matchTemplates，announce 四项与 visit 两项保持默认（后端无对应字段）', () => {
    const savedRule = {
      name: PK_RULE_NAME,
      enabled: true,
      on: ['battle'],
      do: [{ type: 'danmaku', template: ['已保存的PK播报模板'] }],
    }
    const draft = parsePkDraft(savedRule)
    expect(draft.enabled).toBe(true)
    expect(draft.matchTemplates).toEqual(['已保存的PK播报模板'])
    const defaults = defaultPkDraft()
    expect(draft.announceOpponentName).toBe(defaults.announceOpponentName)
    expect(draft.announceRoomCount).toBe(defaults.announceRoomCount)
    expect(draft.announceGuardTotal).toBe(defaults.announceGuardTotal)
    expect(draft.announceGuardOnline).toBe(defaults.announceGuardOnline)
    expect(draft.visitGreetingEnabled).toBe(defaults.visitGreetingEnabled)
    expect(draft.visitTemplates).toEqual(defaults.visitTemplates)
  })
})

describe('PkPanel 组件：控件全部渲染，且不 disabled', () => {
  it('两个"待后端支持"标签都出现（PK匹配信息、PK串门欢迎各一个）', () => {
    const wrapper = mount(PkPanel, { props: { modelValue: defaultPkDraft() } })
    const tags = wrapper.findAll('.n-tag').filter((t) => t.text() === '待后端支持')
    expect(tags.length).toBe(2)
  })

  it('四个"对面数据"复选框、匹配模板输入框、串门欢迎开关与模板输入框都能交互', () => {
    const wrapper = mount(PkPanel, { props: { modelValue: defaultPkDraft() } })

    const checkboxLabels = ['对面主播昵称', '对面直播间人数', '对面大航海总数', '对面大航海在线数']
    for (const label of checkboxLabels) {
      const checkbox = wrapper.findAll('.n-checkbox').find((c) => c.text() === label)
      expect(checkbox, `复选框「${label}」应该存在`).toBeTruthy()
      expect(checkbox!.classes().join(' ')).not.toContain('n-checkbox--disabled')
    }

    const matchInput = wrapper.find('input[placeholder="PK播报语模板"]')
    expect(matchInput.exists()).toBe(true)
    expect(matchInput.attributes('disabled')).toBeUndefined()

    const visitInput = wrapper.find('input[placeholder="串门欢迎语模板"]')
    expect(visitInput.exists()).toBe(true)
    expect(visitInput.attributes('disabled')).toBeUndefined()
  })

  it('勾选"对面主播昵称"后 emit 出的草稿正确翻转该字段，其余字段不变', async () => {
    const draft = defaultPkDraft()
    const wrapper = mount(PkPanel, { props: { modelValue: draft } })

    const checkbox = wrapper.findAll('.n-checkbox').find((c) => c.text() === '对面主播昵称')
    await checkbox!.trigger('click')

    const emitted = wrapper.emitted('update:modelValue')
    expect(emitted).toBeTruthy()
    const patched = emitted![emitted!.length - 1][0] as ReturnType<typeof defaultPkDraft>
    expect(patched.announceOpponentName).toBe(false)
    expect(patched.matchTemplates).toEqual(draft.matchTemplates)
  })
})
