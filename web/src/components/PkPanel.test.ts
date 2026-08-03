import { describe, expect, it } from 'vitest'
import { mount } from '@vue/test-utils'
import PkPanel, {
  PK_RULE_NAME,
  PK_VISIT_RULE_NAME,
  buildPkRule,
  buildPkVisitRule,
  defaultPkDraft,
  parsePkDraft,
} from './PkPanel.vue'

// PkPanel 是「PK 播报」整块——P4-4 Task 7 之前后端完全没有对面数据，
// 这组测试当时只能证明「界面能画」。现在 event.PkMember 带真实的
// Online/GuardTotal/GuardOnline 快照，rules.VarsFromEvent 把它们展开成
// pk.opponent.*，PKPipeline 把 StartPK/FetchOpponentSnapshots 接进了
// 实时事件流并合成一个「快照就绪」的标记事件；串门方向也有了独立的事件
// 类型。这组测试改为证明：
//   1. buildPkRule/buildPkVisitRule 组装出的规则用的是真实字段名，
//      且分别绑定正确的触发条件（when 锁定快照就绪事件 / on 绑定欢迎
//      方向的事件类型）
//   2. 两条规则真的会被保存（parsePkDraft 能从两条已保存规则里完整
//      还原草稿）
//   3. 组件层面控件全部渲染且不 disabled，不再挂"待后端支持"标签

describe('PkPanel 纯函数：build/parse 往返', () => {
  it('buildPkRule 用固定 name/on，when 锁定 PK 接通瞬间合成的快照事件，模板用真实字段', () => {
    const draft = defaultPkDraft()
    const rule = buildPkRule(draft)
    expect(rule.name).toBe(PK_RULE_NAME)
    expect(rule.on).toEqual(['battle'])
    expect(rule.when).toEqual({
      field: 'battle.subCommand',
      op: 'eq',
      value: 'PK_OPPONENT_SNAPSHOT',
    })
    expect(rule.do).toEqual([{ type: 'danmaku', template: draft.matchTemplates, pick: 'random' }])
    // 默认模板必须用真实字段名，不能是渲染不出东西的占位符
    const tmpl = draft.matchTemplates.join('\n')
    expect(tmpl).toContain('.pk.opponent.uname')
    expect(tmpl).toContain('.pk.opponent.online')
    expect(tmpl).toContain('.pk.opponent.guardTotal')
  })

  it('buildPkVisitRule 只覆盖欢迎方向（pk_visit_from_opponent），不覆盖警示方向', () => {
    const draft = defaultPkDraft()
    draft.visitGreetingEnabled = true
    const rule = buildPkVisitRule(draft)
    expect(rule.name).toBe(PK_VISIT_RULE_NAME)
    expect(rule.enabled).toBe(true)
    expect(rule.on).toEqual(['pk_visit_from_opponent'])
    expect(rule.on).not.toContain('pk_visit_to_opponent')
    expect(rule.do).toEqual([{ type: 'danmaku', template: draft.visitTemplates, pick: 'random' }])
  })

  // P5-4 8：「每一条答谢/播报都要有」轮询/随机开关——PK 播报此前是唯一
  // 有多模板列表却没有这个选项的地方。
  it('buildPkRule/buildPkVisitRule：pickMode 直接进 do[0].pick，两者互不干扰', () => {
    const draft = { ...defaultPkDraft(), matchPickMode: 'sequential' as const }
    expect(buildPkRule(draft).do![0].pick).toBe('sequential')
    expect(buildPkVisitRule(draft).do![0].pick).toBe('random')

    const draft2 = { ...defaultPkDraft(), visitPickMode: 'sequential' as const }
    expect(buildPkVisitRule(draft2).do![0].pick).toBe('sequential')
    expect(buildPkRule(draft2).do![0].pick).toBe('random')
  })

  it('parsePkDraft 还原 matchPickMode/visitPickMode', () => {
    const matchRule = {
      name: PK_RULE_NAME,
      on: ['battle'],
      do: [{ type: 'danmaku', template: ['已保存'], pick: 'sequential' }],
    }
    const visitRule = {
      name: PK_VISIT_RULE_NAME,
      on: ['pk_visit_from_opponent'],
      do: [{ type: 'danmaku', template: ['已保存'], pick: 'sequential' }],
    }
    const draft = parsePkDraft(matchRule, visitRule)
    expect(draft.matchPickMode).toBe('sequential')
    expect(draft.visitPickMode).toBe('sequential')
  })

  it('buildPkVisitRule 带 aggregate，PK 接通瞬间对面多个不同的人涌入时不会各触发一条弹幕', () => {
    // 终审复审指出的 C-1 残留一半：后端的 welcomedFromOpponent 只堵死了
    // "同一个人刷屏"，堵不住"同一时刻不同的人一起涌入"——这条规则此前
    // 完全没有 aggregate，N 个不同的人 = N 条弹幕。跟 buildEnterRule
    // （Danmaku.vue）同款做法，加一个"窗口内全部合并"的 aggregate。
    const draft = defaultPkDraft()
    const rule = buildPkVisitRule(draft)
    expect(rule.aggregate).toBeTruthy()
    expect(rule.aggregate?.by).toBe('type')
    expect(rule.aggregate?.window).toMatch(/^\d+s$/)
  })

  it('buildPkVisitRule 的 enabled 直接反映 visitGreetingEnabled', () => {
    const draft = defaultPkDraft()
    expect(buildPkVisitRule(draft).enabled).toBe(false)
    draft.visitGreetingEnabled = true
    expect(buildPkVisitRule(draft).enabled).toBe(true)
  })

  it('parsePkDraft(null, null) 返回默认草稿', () => {
    expect(parsePkDraft(null, null)).toEqual(defaultPkDraft())
  })

  it('parsePkDraft 从两条已保存规则里分别还原 enabled/模板，两条规则各自独立', () => {
    const savedMatchRule = {
      name: PK_RULE_NAME,
      enabled: true,
      on: ['battle'],
      do: [{ type: 'danmaku', template: ['已保存的PK播报模板'] }],
    }
    const savedVisitRule = {
      name: PK_VISIT_RULE_NAME,
      enabled: true,
      on: ['pk_visit_from_opponent'],
      do: [{ type: 'danmaku', template: ['已保存的串门欢迎模板'] }],
    }
    const draft = parsePkDraft(savedMatchRule, savedVisitRule)
    expect(draft.enabled).toBe(true)
    expect(draft.matchTemplates).toEqual(['已保存的PK播报模板'])
    expect(draft.visitGreetingEnabled).toBe(true)
    expect(draft.visitTemplates).toEqual(['已保存的串门欢迎模板'])
  })

  it('parsePkDraft 只有 matchRule 时 visit 部分保持默认（两条规则相互独立）', () => {
    const savedMatchRule = {
      name: PK_RULE_NAME,
      enabled: true,
      on: ['battle'],
      do: [{ type: 'danmaku', template: ['x'] }],
    }
    const draft = parsePkDraft(savedMatchRule, null)
    const defaults = defaultPkDraft()
    expect(draft.visitGreetingEnabled).toBe(defaults.visitGreetingEnabled)
    expect(draft.visitTemplates).toEqual(defaults.visitTemplates)
  })
})

describe('PkPanel 组件：控件全部渲染，不再挂待后端支持标签', () => {
  it('不再出现"待后端支持"标签', () => {
    const wrapper = mount(PkPanel, { props: { modelValue: defaultPkDraft() } })
    const tags = wrapper.findAll('.n-tag').filter((t) => t.text() === '待后端支持')
    expect(tags.length).toBe(0)
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

  it('预览折叠区标题同时提到 PK 匹配信息与 PK 串门欢迎——两条规则各占一段预览', () => {
    // NCollapseItem 默认折叠、内容不在初始渲染的 DOM 里，这里只断言
    // 折叠区标题本身能反映"这里有两条规则"，具体的 JSON 内容由上面的
    // buildPkRule/buildPkVisitRule 纯函数测试覆盖，不必展开折叠面板
    // 重复断言一遍。
    const wrapper = mount(PkPanel, { props: { modelValue: defaultPkDraft() } })
    expect(wrapper.text()).toContain('PK 匹配信息 + PK 串门欢迎')
  })
})
