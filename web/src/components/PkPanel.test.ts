import { describe, expect, it } from 'vitest'
import { mount } from '@vue/test-utils'
import PkPanel, {
  PK_RULE_NAME,
  PK_VISIT_RULE_NAME,
  buildPkRule,
  buildPkVisitCondition,
  buildPkVisitRule,
  defaultPkDraft,
  defaultPkVisitFilter,
  parsePkDraft,
  parsePkVisitFilter,
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

  // P5-5 7a：变量说明改成表格，跟 Danmaku.vue 的 ENTER_TEMPLATE_VAR_ROWS/
  // GIFT_TEMPLATE_VAR_ROWS 同一种写法——断言表格真的渲染出了关键变量，
  // 不再只靠一整段 NTooltip 大白话。
  it('PK 匹配信息与 PK 串门欢迎都渲染出「可用变量」表格，覆盖关键字段', () => {
    const wrapper = mount(PkPanel, { props: { modelValue: defaultPkDraft() } })
    const tables = wrapper.findAll('.var-hint-table')
    expect(tables.length).toBe(2)

    const matchTableText = tables[0].text()
    expect(matchTableText).toContain('pk.opponent.uname')
    expect(matchTableText).toContain('pk.opponent.online')
    expect(matchTableText).toContain('pk.pkId')

    const visitTableText = tables[1].text()
    expect(visitTableText).toContain('user.username')
    expect(visitTableText).toContain('visit.opponentRoomId')
    expect(visitTableText).toContain('visit.matchedBy')
  })
})

describe('PkPanel 纯函数：7b 串门欢迎筛选（buildPkVisitCondition/parsePkVisitFilter）', () => {
  it('三个筛选都不勾选时不拼 when 条件', () => {
    expect(buildPkVisitCondition(defaultPkVisitFilter())).toBeUndefined()
  })

  it('只勾选"只欢迎佩戴对面粉丝牌的用户"时，只拼 visit.matchedBy == fan_medal 一条', () => {
    const filter = { ...defaultPkVisitFilter(), opponentMedalOnly: true }
    expect(buildPkVisitCondition(filter)).toEqual({
      field: 'visit.matchedBy',
      op: 'eq',
      value: 'fan_medal',
    })
  })

  it('只勾选"粉丝牌等级下限"时，拼出 matchedBy 前提 + level 两条 all 树', () => {
    const filter = {
      ...defaultPkVisitFilter(),
      minOpponentMedalLevelEnabled: true,
      minOpponentMedalLevel: 10,
    }
    expect(buildPkVisitCondition(filter)).toEqual({
      all: [
        { field: 'visit.matchedBy', op: 'eq', value: 'fan_medal' },
        { field: 'user.medal.level', op: 'gte', value: 10 },
      ],
    })
  })

  // minOpponentMedalLevelEnabled 为假时，即使 minOpponentMedalLevel 有值
  // 也不该拼出条件——跟 EnterFilter 的同名开关同一条纪律（P5-4 5a）。
  it('minOpponentMedalLevelEnabled 为假时，即使 minOpponentMedalLevel 有值也不拼条件', () => {
    const filter = {
      ...defaultPkVisitFilter(),
      minOpponentMedalLevelEnabled: false,
      minOpponentMedalLevel: 20,
    }
    expect(buildPkVisitCondition(filter)).toBeUndefined()
  })

  it('只勾选"只欢迎对面大航海"时，拼出 matchedBy 前提 + guardLevel in 一档数值', () => {
    const filter = {
      ...defaultPkVisitFilter(),
      opponentGuardOnly: true,
      opponentGuardTier: 'governor' as const,
    }
    expect(buildPkVisitCondition(filter)).toEqual({
      all: [
        { field: 'visit.matchedBy', op: 'eq', value: 'fan_medal' },
        { field: 'user.medal.guardLevel', op: 'in', value: [1] },
      ],
    })
  })

  // 【核心：条件叠加（AND），不是互斥】三个筛选同时勾选时，matchedBy
  // 前提只出现一次（不是三份重复），level/guardLevel 各自一条，AND 拼在
  // 同一棵 all 树里——这是这条 P5-5 7b 任务最核心的行为断言：如果有人
  // 把三个筛选实现成"选中最后一个生效"的互斥 radio，这条测试必须变红。
  it('三个筛选同时勾选时是 AND 叠加，matchedBy 前提只出现一次', () => {
    const filter = {
      opponentMedalOnly: true,
      minOpponentMedalLevelEnabled: true,
      minOpponentMedalLevel: 15,
      opponentGuardOnly: true,
      opponentGuardTier: 'admiral' as const,
    }
    const cond = buildPkVisitCondition(filter)
    expect(cond).toEqual({
      all: [
        { field: 'visit.matchedBy', op: 'eq', value: 'fan_medal' },
        { field: 'user.medal.level', op: 'gte', value: 15 },
        { field: 'user.medal.guardLevel', op: 'in', value: [1, 2] },
      ],
    })
    // matchedBy 前提只应该出现一次，不是每个筛选各自重复拼一遍。
    const all = (cond as { all: unknown[] }).all
    const matchedByLeaves = all.filter(
      (leaf) => (leaf as { field?: string }).field === 'visit.matchedBy',
    )
    expect(matchedByLeaves.length).toBe(1)
  })

  it('parsePkVisitFilter 是 buildPkVisitCondition 的逆过程（等级+大航海组合）', () => {
    const filter = {
      opponentMedalOnly: false,
      minOpponentMedalLevelEnabled: true,
      minOpponentMedalLevel: 8,
      opponentGuardOnly: true,
      opponentGuardTier: 'captain' as const,
    }
    const cond = buildPkVisitCondition(filter)
    expect(parsePkVisitFilter(cond)).toEqual(filter)
  })

  it('parsePkVisitFilter(undefined) 返回默认筛选（全不勾选）', () => {
    expect(parsePkVisitFilter(undefined)).toEqual(defaultPkVisitFilter())
  })

  it('大航海档位用 in 一组数值而不是 gte——因为编号越小档位越高（与 EnterFilter 同一条规则）', () => {
    const filter = {
      ...defaultPkVisitFilter(),
      opponentGuardOnly: true,
      opponentGuardTier: 'captain' as const,
    }
    // opponentGuardOnly 会连带拼出 matchedBy 前提，结果是 all 树，不是单条叶子
    // （跟 EnterFilter 的 guardOnly 不需要额外前提、直接是单条叶子不同）。
    const cond = buildPkVisitCondition(filter) as { all: { field: string; value: number[] }[] }
    const guardLeaf = cond.all.find((leaf) => leaf.field === 'user.medal.guardLevel')
    expect(guardLeaf?.value).toEqual([1, 2, 3])
  })
})

describe('PkPanel 组件：7b 三个筛选控件独立勾选、条件叠加', () => {
  it('三个筛选复选框都渲染且默认不勾选', () => {
    const wrapper = mount(PkPanel, { props: { modelValue: defaultPkDraft() } })
    const labels = [
      '只欢迎佩戴对面主播粉丝牌的用户',
      '只欢迎对面粉丝牌等级达到',
      '只欢迎对面大航海（舰长/提督/总督）用户',
    ]
    for (const label of labels) {
      const checkbox = wrapper.findAll('.n-checkbox').find((c) => c.text().includes(label))
      expect(checkbox, `复选框「${label}」应该存在`).toBeTruthy()
    }
  })

  it('勾选"只欢迎对面大航海"后 emit 出的草稿正确翻转该字段，其余筛选不受影响', async () => {
    const draft = defaultPkDraft()
    const wrapper = mount(PkPanel, { props: { modelValue: draft } })

    const checkbox = wrapper
      .findAll('.n-checkbox')
      .find((c) => c.text().includes('只欢迎对面大航海（舰长/提督/总督）用户'))
    await checkbox!.trigger('click')

    const emitted = wrapper.emitted('update:modelValue')
    expect(emitted).toBeTruthy()
    const patched = emitted![emitted!.length - 1][0] as ReturnType<typeof defaultPkDraft>
    expect(patched.visitFilter.opponentGuardOnly).toBe(true)
    expect(patched.visitFilter.opponentMedalOnly).toBe(false)
    expect(patched.visitFilter.minOpponentMedalLevelEnabled).toBe(false)
  })

  it('buildPkVisitRule 把 visitFilter 拼进 when 条件里，保存后能被 parsePkDraft 还原', () => {
    const draft = defaultPkDraft()
    draft.visitFilter = {
      opponentMedalOnly: false,
      minOpponentMedalLevelEnabled: true,
      minOpponentMedalLevel: 12,
      opponentGuardOnly: false,
      opponentGuardTier: 'captain',
    }
    const rule = buildPkVisitRule(draft)
    expect(rule.when).toEqual({
      all: [
        { field: 'visit.matchedBy', op: 'eq', value: 'fan_medal' },
        { field: 'user.medal.level', op: 'gte', value: 12 },
      ],
    })

    const restored = parsePkDraft(null, rule)
    expect(restored.visitFilter).toEqual(draft.visitFilter)
  })
})
