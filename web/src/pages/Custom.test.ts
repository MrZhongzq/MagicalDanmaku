import { beforeEach, describe, expect, it, vi } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import { NSelect, NSwitch } from 'naive-ui'

// Custom 是「自定义弹幕姬」页，P4-2 最难的组件的落地场景。核心断言分三层：
//   1. 纯函数：isCustomRule（排除内置七条）、buildCustomRule/parseCustomRuleDraft
//      互为逆过程、buildCustomAction 按 type 只取相关字段
//   2. 元数据必须来自 GET /api/meta/*，不硬编码——同 Task 8 MemberEditor
//      的既定模式，用"后端只回一部分时界面也只渲染那一部分"来证明
//   3. 挂载级交互：新增/删除规则、条件树开关、动作增删
const messageMock = { success: vi.fn(), error: vi.fn(), warning: vi.fn(), info: vi.fn() }
const warningMock = vi.fn()

vi.mock('naive-ui', async () => {
  const actual = await vi.importActual<typeof import('naive-ui')>('naive-ui')
  return {
    ...actual,
    useMessage: () => messageMock,
    useDialog: () => ({ warning: warningMock }),
  }
})

const Custom = await import('./Custom.vue')
const {
  isCustomRule,
  BUILTIN_RULE_NAMES,
  buildCustomRule,
  buildCustomAction,
  buildCustomSchedule,
  defaultCustomRuleDraft,
  defaultActionDraft,
  parseCustomRuleDraft,
} = Custom
const { useBindingsStore } = await import('@/stores/bindings')

type RuleView = import('@/api/rule-types').RuleView

function ok(body: unknown, status = 200) {
  return new Response(JSON.stringify(body), {
    status,
    headers: { 'Content-Type': 'application/json' },
  })
}

const 绑定 = {
  id: 1,
  accountId: 1,
  accountName: '小号',
  roomId: '9000',
  enabled: true,
  ruleCount: 2,
  permissions: ['rule:read', 'rule:write'] as const,
}

function setupStores() {
  setActivePinia(createPinia())
  const bindings = useBindingsStore()
  bindings.list = [{ ...绑定, permissions: [...绑定.permissions] }]
  bindings.select(1)
  return { bindings }
}

const FULL_EVENT_TYPES = [
  { value: 'danmaku', label: '弹幕' },
  { value: 'gift', label: '礼物' },
  { value: 'user_enter', label: '进入直播间' },
]
const FULL_ACTION_TYPES = [
  { value: 'danmaku', label: '发送弹幕' },
  { value: 'block', label: '禁言' },
  { value: 'script', label: '执行脚本' },
  { value: 'log', label: '只记日志（调试规则用）' },
]
const FULL_OPERATORS = [
  { value: 'eq', label: '等于' },
  { value: 'in', label: '属于列表之一' },
]
const FULL_AGGREGATE_BY = [
  { value: 'type', label: '按事件类型：窗口内全部合成一条' },
  { value: 'user', label: '按类型 + 用户：仅去重不聚合' },
]

function err(status: number, message: string) {
  return new Response(JSON.stringify({ error: message }), {
    status,
    headers: { 'Content-Type': 'application/json' },
  })
}

/**
 * 每次都返回新 Response 实例——Response.body 只能读一次。
 *
 * `putResponse`/`reloadResponse` 默认成功，Task 13 的失败路径测试用它们
 * 覆盖成 422，演练"第 1 步失败"“第 2 步失败”两种场景。
 */
function stubFetch(opts: {
  rules?: RuleView[]
  onWrite?: (url: string, init: RequestInit) => void
  putResponse?: () => Response
  reloadResponse?: () => Response
}) {
  const rules = opts.rules ?? []
  const f = vi.fn().mockImplementation((url: string, init?: RequestInit) => {
    if (url === '/api/meta/event-types') return Promise.resolve(ok(FULL_EVENT_TYPES))
    if (url === '/api/meta/action-types') return Promise.resolve(ok(FULL_ACTION_TYPES))
    if (url === '/api/meta/operators') return Promise.resolve(ok(FULL_OPERATORS))
    if (url === '/api/meta/aggregate-by') return Promise.resolve(ok(FULL_AGGREGATE_BY))
    if (url === '/api/bindings/1/rules' && (!init || init.method === 'GET')) {
      return Promise.resolve(ok(rules))
    }
    if (init) opts.onWrite?.(url, init)
    if (url === '/api/bindings/1/rules' && init?.method === 'PUT' && opts.putResponse) {
      return Promise.resolve(opts.putResponse())
    }
    if (url === '/api/bindings/1/reload' && init?.method === 'POST' && opts.reloadResponse) {
      return Promise.resolve(opts.reloadResponse())
    }
    return Promise.resolve(ok({ status: 'ok' }))
  })
  vi.stubGlobal('fetch', f)
  return f
}

beforeEach(() => {
  vi.unstubAllGlobals()
  messageMock.success.mockClear()
  messageMock.error.mockClear()
  messageMock.warning.mockClear()
  messageMock.info.mockClear()
  warningMock.mockClear()
})

async function mountCustom() {
  const wrapper = mount(Custom.default)
  await flushPromises()
  return wrapper
}

// ============================================================
// 一、纯函数
// ============================================================

describe('isCustomRule：排除 Task 9/10 建立的七个内置规则名', () => {
  it.each(BUILTIN_RULE_NAMES)('%s 不算自定义规则', (name) => {
    expect(isCustomRule({ name, position: 0 })).toBe(false)
  })

  it('不在内置名单里的规则名算自定义规则', () => {
    expect(isCustomRule({ name: '舰长专属欢迎', position: 0 })).toBe(true)
  })

  it('恰好七个内置名字（覆盖 Task 9/10 全部固定名）', () => {
    expect(BUILTIN_RULE_NAMES).toHaveLength(7)
    expect(new Set(BUILTIN_RULE_NAMES).size).toBe(7) // 互不相同
  })
})

describe('buildCustomAction：按 type 只取相关字段', () => {
  it('danmaku 只带 template，过滤空白模板', () => {
    const a = buildCustomAction({
      type: 'danmaku',
      templates: ['你好', '  ', ''],
      hours: 1,
      script: '',
    })
    expect(a).toEqual({ type: 'danmaku', template: ['你好'] })
  })
  it('block 只带 hours', () => {
    const a = buildCustomAction({ type: 'block', templates: [], hours: 3, script: '' })
    expect(a).toEqual({ type: 'block', hours: 3 })
  })
  it('script 只带 script', () => {
    const a = buildCustomAction({ type: 'script', templates: [], hours: 1, script: 'return true' })
    expect(a).toEqual({ type: 'script', script: 'return true' })
  })
  it('log（或任何未知类型）不带额外字段', () => {
    const a = buildCustomAction({ type: 'log', templates: [], hours: 1, script: '' })
    expect(a).toEqual({ type: 'log' })
  })
})

describe('buildCustomSchedule', () => {
  it('interval 模式按分钟生成 6 段 cron', () => {
    const draft = {
      ...defaultCustomRuleDraft(),
      scheduleMode: 'interval' as const,
      intervalMinutes: 5,
    }
    expect(buildCustomSchedule(draft)).toBe('0 */5 * * * *')
  })
  it('cron 模式原样使用（去首尾空白）', () => {
    const draft = {
      ...defaultCustomRuleDraft(),
      scheduleMode: 'cron' as const,
      cronExpr: '  0 0 * * * *  ',
    }
    expect(buildCustomSchedule(draft)).toBe('0 0 * * * *')
  })
})

describe('buildCustomRule：on/schedule 互斥，when 经过剪枝', () => {
  it('triggerMode=on 时只带 on，不带 schedule', () => {
    const draft = { ...defaultCustomRuleDraft(), name: '测试', on: ['user_enter'] }
    const rule = buildCustomRule(draft)
    expect(rule.on).toEqual(['user_enter'])
    expect(rule.schedule).toBeUndefined()
  })

  it('triggerMode=schedule 时只带 schedule，不带 on', () => {
    const draft = {
      ...defaultCustomRuleDraft(),
      name: '测试',
      triggerMode: 'schedule' as const,
      scheduleMode: 'interval' as const,
      intervalMinutes: 10,
    }
    const rule = buildCustomRule(draft)
    expect(rule.schedule).toBe('0 */10 * * * *')
    expect(rule.on).toBeUndefined()
  })

  it('whenEnabled 为 false 时不带 when，即便 when 字段本身有内容', () => {
    const draft = {
      ...defaultCustomRuleDraft(),
      name: '测试',
      whenEnabled: false,
      when: { field: 'user.uid', op: 'eq', value: '1' },
    }
    expect(buildCustomRule(draft).when).toBeUndefined()
  })

  it('whenEnabled 为 true 时 when 经过剪枝——空叶子（field 为空）整体消失', () => {
    const draft = {
      ...defaultCustomRuleDraft(),
      name: '测试',
      whenEnabled: true,
      when: { field: '', op: 'eq', value: '' },
    }
    expect(buildCustomRule(draft).when).toBeUndefined()
  })

  it('whenEnabled 为 true 且条件有效时原样带上', () => {
    const draft = {
      ...defaultCustomRuleDraft(),
      name: '测试',
      whenEnabled: true,
      when: { field: 'user.uid', op: 'eq', value: '12345' },
    }
    expect(buildCustomRule(draft).when).toEqual({ field: 'user.uid', op: 'eq', value: '12345' })
  })

  it('aggregateEnabled 为 false 时不带 aggregate', () => {
    const draft = { ...defaultCustomRuleDraft(), name: '测试' }
    expect(buildCustomRule(draft).aggregate).toBeUndefined()
  })

  it('aggregateEnabled 为 true 时拼出 window/by，minCount<=1 时不带 minCount', () => {
    const draft = {
      ...defaultCustomRuleDraft(),
      name: '测试',
      aggregateEnabled: true,
      aggregateBy: 'user',
      windowSeconds: 30,
      minCount: 1,
    }
    expect(buildCustomRule(draft).aggregate).toEqual({ window: '30s', by: 'user' })
  })

  it('cooldownGroup 只在非空白时写入', () => {
    const draft = { ...defaultCustomRuleDraft(), name: '测试', cooldownGroup: '  ' }
    expect(buildCustomRule(draft).cooldownGroup).toBeUndefined()
  })
})

describe('parseCustomRuleDraft 是 buildCustomRule 的逆过程', () => {
  it('事件触发规则往返一致', () => {
    const rule: RuleView = {
      name: '舰长专属欢迎',
      enabled: true,
      on: ['user_enter'],
      when: { field: 'user.uid', op: 'eq', value: '12345' },
      do: [{ type: 'danmaku', template: ['欢迎回家'] }],
      position: 0,
    }
    const draft = parseCustomRuleDraft(rule)
    expect(draft.name).toBe('舰长专属欢迎')
    expect(draft.triggerMode).toBe('on')
    expect(draft.on).toEqual(['user_enter'])
    expect(draft.whenEnabled).toBe(true)
    expect(draft.when).toEqual({ field: 'user.uid', op: 'eq', value: '12345' })

    const rebuilt = buildCustomRule(draft)
    expect(rebuilt.on).toEqual(['user_enter'])
    expect(rebuilt.when).toEqual({ field: 'user.uid', op: 'eq', value: '12345' })
    expect(rebuilt.do).toEqual([{ type: 'danmaku', template: ['欢迎回家'] }])
  })

  it('定时规则（interval 形状的 cron）往返一致', () => {
    const rule: RuleView = {
      name: '整点报时',
      enabled: true,
      schedule: '0 */15 * * * *',
      do: [{ type: 'danmaku', template: ['整点啦'] }],
      position: 0,
    }
    const draft = parseCustomRuleDraft(rule)
    expect(draft.triggerMode).toBe('schedule')
    expect(draft.scheduleMode).toBe('interval')
    expect(draft.intervalMinutes).toBe(15)
    expect(buildCustomRule(draft).schedule).toBe('0 */15 * * * *')
  })

  it('非 interval 形状的 cron 落回 cron 模式，原样保留表达式', () => {
    const rule: RuleView = {
      name: '工作日播报',
      enabled: true,
      schedule: '0 0 9 * * 1-5',
      do: [{ type: 'log' }],
      position: 0,
    }
    const draft = parseCustomRuleDraft(rule)
    expect(draft.scheduleMode).toBe('cron')
    expect(draft.cronExpr).toBe('0 0 9 * * 1-5')
  })
})

// ============================================================
// 二、元数据来自 GET /api/meta/*，不硬编码
// ============================================================

describe('Custom 页元数据加载', () => {
  it('后端只给 2 个事件类型时，"监听的事件类型"下拉就只有 2 项', async () => {
    setupStores()
    stubFetch({
      rules: [
        { name: '测试规则', enabled: true, on: ['danmaku'], do: [{ type: 'log' }], position: 0 },
      ],
    })
    const wrapper = await mountCustom()

    const selects = wrapper.findAllComponents(NSelect)
    const onSelect = selects.find((s) => {
      const opts = s.props('options') as { value: string }[] | undefined
      return opts?.some((o) => o.value === 'danmaku')
    })
    expect(onSelect, '应该能找到"监听的事件类型"这个 NSelect').toBeTruthy()
    expect((onSelect!.props('options') as { value: string }[]).map((o) => o.value)).toEqual([
      'danmaku',
      'gift',
      'user_enter',
    ])
  })
})

// ============================================================
// 三、挂载级交互
// ============================================================

describe('Custom 页：加载时只展示自定义规则，内置规则被排除', () => {
  it('GET /api/bindings/1/rules 混有内置规则与自定义规则时，只有自定义规则出现在列表里', async () => {
    setupStores()
    stubFetch({
      rules: [
        {
          name: '内置/进房欢迎',
          enabled: true,
          on: ['user_enter'],
          do: [{ type: 'danmaku', template: ['x'] }],
          position: 0,
        },
        {
          name: '舰长专属欢迎',
          enabled: true,
          on: ['user_enter'],
          do: [{ type: 'danmaku', template: ['欢迎舰长'] }],
          position: 1,
        },
      ],
    })
    const wrapper = await mountCustom()

    // 规则名进了 NInput 的 value，不是纯文本节点，wrapper.text() 读不到，
    // 要看 input 元素的 value 属性。
    const inputValues = wrapper.findAll('input').map((i) => (i.element as HTMLInputElement).value)
    expect(inputValues).toContain('舰长专属欢迎')
    expect(inputValues).not.toContain('内置/进房欢迎')
    expect(wrapper.findAll('.rule-card')).toHaveLength(1) // 只有一张卡片，不是两张
  })
})

describe('Custom 页：新增与删除自定义规则', () => {
  it('没有自定义规则时显示空状态，点"+ 新增自定义规则"后出现一张新卡片', async () => {
    setupStores()
    stubFetch({ rules: [] })
    const wrapper = await mountCustom()

    expect(wrapper.text()).toContain('还没有自定义规则')

    const addBtn = wrapper.findAll('button').find((b) => b.text().includes('新增自定义规则'))
    expect(addBtn).toBeTruthy()
    await addBtn!.trigger('click')
    await flushPromises()

    expect(wrapper.findAll('.rule-card')).toHaveLength(1)
  })

  it('删除规则先弹确认，确认前卡片还在；确认后卡片消失', async () => {
    setupStores()
    stubFetch({
      rules: [
        { name: '待删除规则', enabled: true, on: ['danmaku'], do: [{ type: 'log' }], position: 0 },
      ],
    })
    const wrapper = await mountCustom()
    expect(wrapper.findAll('.rule-card')).toHaveLength(1)

    const deleteBtn = wrapper.findAll('button').find((b) => b.text() === '删除规则')
    await deleteBtn!.trigger('click')
    await flushPromises()

    expect(warningMock).toHaveBeenCalledTimes(1)
    expect(wrapper.findAll('.rule-card')).toHaveLength(1) // 确认前还在

    const opts = warningMock.mock.calls[0][0] as { onPositiveClick: () => void }
    opts.onPositiveClick()
    await flushPromises()

    expect(wrapper.findAll('.rule-card')).toHaveLength(0)
  })
})

describe('Custom 页：动作增删，至少保留一条', () => {
  it('新建规则默认带一条 danmaku 动作，"删除此动作"在只剩一条时禁用', async () => {
    setupStores()
    stubFetch({ rules: [] })
    const wrapper = await mountCustom()

    const addRuleBtn = wrapper.findAll('button').find((b) => b.text().includes('新增自定义规则'))
    await addRuleBtn!.trigger('click')
    await flushPromises()

    const removeActionBtn = wrapper.findAll('button').find((b) => b.text() === '删除此动作')
    expect(removeActionBtn, '应该有"删除此动作"按钮').toBeTruthy()
    expect(removeActionBtn!.attributes('disabled')).toBeDefined()

    const addActionBtn = wrapper.findAll('button').find((b) => b.text().includes('添加动作'))
    await addActionBtn!.trigger('click')
    await flushPromises()

    const removeActionBtns = wrapper.findAll('button').filter((b) => b.text() === '删除此动作')
    expect(removeActionBtns).toHaveLength(2)
    expect(removeActionBtns[0].attributes('disabled')).toBeUndefined()
  })
})

describe('Custom 页：条件开关控制 ConditionTree 是否出现', () => {
  it('默认不启用条件时不渲染 ConditionTree；打开开关后出现', async () => {
    setupStores()
    stubFetch({ rules: [] })
    const wrapper = await mountCustom()

    const addRuleBtn = wrapper.findAll('button').find((b) => b.text().includes('新增自定义规则'))
    await addRuleBtn!.trigger('click')
    await flushPromises()

    expect(wrapper.find('.condition-tree').exists()).toBe(false)

    const switches = wrapper.findAllComponents(NSwitch)
    // 第一个 switch 是规则启停，第二个是"触发条件"的启用开关（按模板里出现顺序）
    const whenSwitch = switches[1]
    whenSwitch.vm.$emit('update:value', true)
    await flushPromises()

    expect(wrapper.find('.condition-tree').exists()).toBe(true)
  })
})

describe('Custom 页：排除通用规则——渲染出来但不参与组装（悬空）', () => {
  it('多选框选中内置规则名后，规则 JSON 预览里不包含这个选择', async () => {
    setupStores()
    stubFetch({ rules: [] })
    const wrapper = await mountCustom()

    const addRuleBtn = wrapper.findAll('button').find((b) => b.text().includes('新增自定义规则'))
    await addRuleBtn!.trigger('click')
    await flushPromises()

    expect(wrapper.text()).toContain('待后端支持')
    // NSelect 的候选项只在下拉展开时才进 DOM 文本，直接读 options prop 更可靠
    const excludeSelect = wrapper
      .findAllComponents(NSelect)
      .find(
        (s) =>
          (s.props('options') as { value: string }[] | undefined)?.length ===
          BUILTIN_RULE_NAMES.length,
      )
    expect(excludeSelect, '应该能找到"排除通用规则"这个 NSelect').toBeTruthy()
    const optionValues = (excludeSelect!.props('options') as { value: string }[]).map(
      (o) => o.value,
    )
    BUILTIN_RULE_NAMES.forEach((n) => expect(optionValues).toContain(n))

    // NCollapseItem 默认懒渲染，内容要展开后才出现在 DOM 里；实际绑定点击
    // 事件的是 header-main 这个子元素，点外层 header 容器不会触发展开。
    const collapseHeader = wrapper.find('.n-collapse-item__header-main')
    await collapseHeader.trigger('click')
    await flushPromises()
    expect(wrapper.find('.json-preview').text()).not.toContain('excludeBuiltinRules')
  })
})

// 未使用到的 defaultActionDraft 导出也在别处（Custom.vue 内部）用到，这里顺带确认默认值形状。
describe('defaultActionDraft', () => {
  it('默认是 danmaku 类型，带一条模板', () => {
    const a = defaultActionDraft()
    expect(a.type).toBe('danmaku')
    expect(a.templates.length).toBeGreaterThan(0)
  })
})

// ============================================================
// Task 13：保存与热重载的统一交互
// ============================================================

async function addRuleWithName(wrapper: Awaited<ReturnType<typeof mountCustom>>, name: string) {
  const addBtn = wrapper.findAll('button').find((b) => b.text().includes('新增自定义规则'))
  await addBtn!.trigger('click')
  await flushPromises()
  const nameInput = wrapper.find('input[placeholder="规则名（如：舰长专属欢迎）"]')
  await nameInput.setValue(name)
  await flushPromises()
}

describe('Custom 页：保存（Task 13 接上 useDraft）', () => {
  it('点击「保存并生效」依次 GET → PUT → POST reload，成功后 dirty 归假、弹出成功提示', async () => {
    setupStores()
    const f = stubFetch({ rules: [] })
    const wrapper = await mountCustom()

    await addRuleWithName(wrapper, '新规则')

    const saveBtn = () => wrapper.findAll('button').find((b) => b.text() === '保存并生效')!
    const callsBefore = f.mock.calls.length
    await saveBtn().trigger('click')
    await flushPromises()

    const writeCalls = f.mock.calls.slice(callsBefore).map((call) => {
      const [url, init] = call as [string, RequestInit?]
      return `${init?.method ?? 'GET'} ${url}`
    })
    expect(writeCalls).toEqual([
      'GET /api/bindings/1/rules',
      'PUT /api/bindings/1/rules',
      'POST /api/bindings/1/reload',
    ])
    expect(wrapper.text()).not.toContain('有未保存的改动')
    expect(messageMock.success).toHaveBeenCalledWith('已保存并生效')
  })

  it(
    '【关键：钉死"整组替换不误删别的页面的规则"，反过来的方向】库里已有一条弹幕姬页管的内置规则时，' +
      '从自定义弹幕姬页保存，PUT 请求体里那条内置规则必须还在、且内容原样不变',
    async () => {
      const builtinRule: RuleView = {
        name: '内置/进房欢迎',
        position: 0,
        enabled: true,
        on: ['user_enter'],
        do: [{ type: 'danmaku', template: ['欢迎 {{.user.username}} 来到直播间~'] }],
      }
      let putBody: RuleView[] | null = null
      setupStores()
      stubFetch({
        rules: [builtinRule],
        onWrite: (url, init) => {
          if (url === '/api/bindings/1/rules' && init.method === 'PUT') {
            putBody = JSON.parse(init.body as string) as RuleView[]
          }
        },
      })
      const wrapper = await mountCustom()

      await addRuleWithName(wrapper, '新自定义规则')

      const saveBtn = () => wrapper.findAll('button').find((b) => b.text() === '保存并生效')!
      await saveBtn().trigger('click')
      await flushPromises()

      expect(putBody).not.toBeNull()
      const kept = putBody!.find((r) => r.name === '内置/进房欢迎')
      expect(kept, '弹幕姬页管的内置规则必须原样保留').toBeTruthy()
      expect(kept).toEqual({
        name: '内置/进房欢迎',
        enabled: true,
        on: ['user_enter'],
        do: [{ type: 'danmaku', template: ['欢迎 {{.user.username}} 来到直播间~'] }],
      })
      expect(putBody!.map((r) => r.name)).toContain('新自定义规则')
    },
  )

  it('第 1 步（PUT 写库）失败：弹出后端错误原文，dirty 保持真', async () => {
    setupStores()
    stubFetch({
      rules: [],
      putResponse: () => err(422, '第 1 条规则(新规则)不合法: 正则非法'),
    })
    const wrapper = await mountCustom()
    await addRuleWithName(wrapper, '新规则')

    const saveBtn = () => wrapper.findAll('button').find((b) => b.text() === '保存并生效')!
    await saveBtn().trigger('click')
    await flushPromises()

    expect(messageMock.error).toHaveBeenCalledWith('第 1 条规则(新规则)不合法: 正则非法')
    expect(wrapper.text()).toContain('有未保存的改动')
    expect(wrapper.text()).not.toContain('已保存到数据库，但重载失败')
  })

  it(
    '第 2 步（reload）失败：dirty 不归假，界面出现"已保存到数据库，但重载失败"的持久提示，' +
      '且原样带上后端"仍在用上一份配置运行"的安抚文案',
    async () => {
      const reloadErrorMessage = '重载失败，仍在用上一份配置运行: 规则 新规则 的正则非法'
      setupStores()
      stubFetch({
        rules: [],
        reloadResponse: () => err(422, reloadErrorMessage),
      })
      const wrapper = await mountCustom()
      await addRuleWithName(wrapper, '新规则')

      const saveBtn = () => wrapper.findAll('button').find((b) => b.text() === '保存并生效')!
      await saveBtn().trigger('click')
      await flushPromises()

      expect(messageMock.error).toHaveBeenCalledWith(reloadErrorMessage)
      expect(wrapper.text()).toContain('有未保存的改动')
      expect(wrapper.text()).toContain('已保存到数据库，但重载失败')
      expect(wrapper.text()).toContain(reloadErrorMessage)
    },
  )

  it(
    '【审查追加：钉住"第三态不跨绑定泄漏"】在绑定 A 上保存触发第三态提示后切到绑定 B，' +
      '提示必须清空',
    async () => {
      const reloadErrorMessage = '重载失败，仍在用上一份配置运行: 规则 新规则 的正则非法'
      setupStores()
      const bindings = useBindingsStore()
      bindings.list = [
        { ...绑定, id: 1, roomId: '9000', permissions: [...绑定.permissions] },
        { ...绑定, id: 2, roomId: '9001', permissions: [...绑定.permissions] },
      ]
      bindings.select(1)

      const f = vi.fn().mockImplementation((url: string, init?: RequestInit) => {
        const method = init?.method ?? 'GET'
        if (url === '/api/meta/event-types') return Promise.resolve(ok(FULL_EVENT_TYPES))
        if (url === '/api/meta/action-types') return Promise.resolve(ok(FULL_ACTION_TYPES))
        if (url === '/api/meta/operators') return Promise.resolve(ok(FULL_OPERATORS))
        if (url === '/api/meta/aggregate-by') return Promise.resolve(ok(FULL_AGGREGATE_BY))
        if (url === '/api/bindings/1/rules' && method === 'GET') return Promise.resolve(ok([]))
        if (url === '/api/bindings/1/rules' && method === 'PUT') {
          return Promise.resolve(ok({ status: 'ok' }))
        }
        if (url === '/api/bindings/1/reload' && method === 'POST') {
          return Promise.resolve(err(422, reloadErrorMessage))
        }
        if (url === '/api/bindings/2/rules' && method === 'GET') return Promise.resolve(ok([]))
        throw new Error(`unexpected fetch: ${method} ${url}`)
      })
      vi.stubGlobal('fetch', f)

      const wrapper = await mountCustom()
      await addRuleWithName(wrapper, '新规则')

      const saveBtn = () => wrapper.findAll('button').find((b) => b.text() === '保存并生效')!
      await saveBtn().trigger('click')
      await flushPromises()
      expect(wrapper.text()).toContain('已保存到数据库，但重载失败')

      bindings.select(2)
      await flushPromises()

      expect(wrapper.text()).not.toContain('已保存到数据库，但重载失败')
    },
  )
})
