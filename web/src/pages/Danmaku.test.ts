import { beforeEach, describe, expect, it, vi } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'

// Danmaku 页最核心的一条：进房欢迎/礼物答谢在后端各是一条固定 name 的
// spec.Rule，前端靠 name 从 `GET /api/bindings/{id}/rules` 里「认领」
// 已保存的配置，认领不到就退回默认值（一个新绑定本来就不该有这两条规则）。
// 同 Moderation.test.ts 一样，顶掉 naive-ui 的 useMessage 以断言提示内容。
const messageMock = { success: vi.fn(), error: vi.fn(), warning: vi.fn(), info: vi.fn() }

vi.mock('naive-ui', async () => {
  const actual = await vi.importActual<typeof import('naive-ui')>('naive-ui')
  return {
    ...actual,
    useMessage: () => messageMock,
  }
})

const Danmaku = await import('./Danmaku.vue')
const {
  claimRule,
  buildEnterCondition,
  parseEnterFilter,
  buildEnterRule,
  parseEnterDraft,
  buildGiftRule,
  parseGiftDraft,
  defaultEnterDraft,
  defaultGiftDraft,
  secondsFromDuration,
  ENTER_RULE_NAME,
  GIFT_RULE_NAME,
} = Danmaku
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

async function mountDanmaku() {
  const wrapper = mount(Danmaku.default)
  await flushPromises()
  return wrapper
}

beforeEach(() => {
  vi.unstubAllGlobals()
  messageMock.success.mockClear()
  messageMock.error.mockClear()
  messageMock.warning.mockClear()
  messageMock.info.mockClear()
})

describe('Danmaku 纯函数：条件拼装与还原', () => {
  it('buildEnterCondition 在什么都没选时返回 undefined（不该生成空 when）', () => {
    expect(
      buildEnterCondition(
        { wearMedalOnly: false, minMedalLevel: null, guardOnly: false, guardTier: 'captain' },
        '9000',
      ),
    ).toBeUndefined()
  })

  it('buildEnterCondition 只选"佩戴粉丝牌"时拼出 isLighted + 本房间 roomId 两条', () => {
    const cond = buildEnterCondition(
      { wearMedalOnly: true, minMedalLevel: null, guardOnly: false, guardTier: 'captain' },
      '9000',
    )
    expect(cond).toEqual({
      all: [
        { field: 'user.medal.isLighted', op: 'eq', value: true },
        { field: 'user.medal.roomId', op: 'eq', value: '9000' },
      ],
    })
  })

  it('buildEnterCondition 三个筛选都选时拼成一棵 all 树，且能被 parseEnterFilter 还原', () => {
    const filter = {
      wearMedalOnly: true,
      minMedalLevel: 5,
      guardOnly: true,
      guardTier: 'admiral' as const,
    }
    const cond = buildEnterCondition(filter, '9000')
    expect(parseEnterFilter(cond)).toEqual(filter)
  })

  it('大航海档位用 in 一组数值而不是 gte——因为编号越小档位越高', () => {
    const cond = buildEnterCondition(
      { wearMedalOnly: false, minMedalLevel: null, guardOnly: true, guardTier: 'governor' },
      '9000',
    )
    expect(cond).toEqual({ field: 'user.guardLevel', op: 'in', value: [1] })
  })

  it('secondsFromDuration 解析复合时长字符串，解析不出单位时回落 fallback', () => {
    expect(secondsFromDuration('1m30s', 0)).toBe(90)
    expect(secondsFromDuration('2m', 0)).toBe(120)
    expect(secondsFromDuration(undefined, 42)).toBe(42)
    expect(secondsFromDuration('乱写的', 42)).toBe(42)
  })
})

describe('Danmaku 纯函数：build/parse 往返（组装成 spec.Rule 与从中还原草稿）', () => {
  it('buildEnterRule 组装出的规则用固定 name 与 on: ["user_enter"]', () => {
    const rule = buildEnterRule(defaultEnterDraft(), '9000')
    expect(rule.name).toBe(ENTER_RULE_NAME)
    expect(rule.on).toEqual(['user_enter'])
    expect(rule.do).toEqual([{ type: 'danmaku', template: defaultEnterDraft().singleTemplates }])
  })

  it('buildGiftRule 组装出的规则用固定 name 与 on: ["gift"]', () => {
    const rule = buildGiftRule(defaultGiftDraft())
    expect(rule.name).toBe(GIFT_RULE_NAME)
    expect(rule.on).toEqual(['gift'])
  })

  it('parseEnterDraft(null) 返回默认草稿——新绑定还没配过规则', () => {
    expect(parseEnterDraft(null)).toEqual(defaultEnterDraft())
  })

  it('parseEnterDraft 还原 enabled/aggregate/模板，multiTemplates 与 pickMode 保持默认（后端没有对应字段）', () => {
    const savedRule = {
      name: ENTER_RULE_NAME,
      enabled: false,
      on: ['user_enter'],
      aggregate: { window: '2m', maxWait: '5m', minCount: 4, by: 'type' },
      do: [{ type: 'danmaku', template: ['已保存单人模板A', '已保存单人模板B'] }],
    }
    const draft = parseEnterDraft(savedRule)
    expect(draft.enabled).toBe(false)
    expect(draft.groupMode).toBe('merge')
    expect(draft.windowSeconds).toBe(120)
    expect(draft.maxWaitSeconds).toBe(300)
    expect(draft.minCount).toBe(4)
    expect(draft.singleTemplates).toEqual(['已保存单人模板A', '已保存单人模板B'])
    // 后端没有 multiTemplates/pickMode 的落地字段，加载时无从恢复
    expect(draft.multiTemplates).toEqual(defaultEnterDraft().multiTemplates)
    expect(draft.pickMode).toBe('random')
  })

  it('parseGiftDraft 还原 groupMode=dedupeGift（by: "gift"）与模板', () => {
    const savedRule = {
      name: GIFT_RULE_NAME,
      enabled: true,
      on: ['gift'],
      aggregate: { window: '15s', by: 'gift' },
      do: [{ type: 'danmaku', template: ['已保存答谢模板'] }],
    }
    const draft = parseGiftDraft(savedRule)
    expect(draft.groupMode).toBe('dedupeGift')
    expect(draft.windowSeconds).toBe(15)
    expect(draft.templates).toEqual(['已保存答谢模板'])
  })
})

describe('Danmaku claimRule：按 name 从规则列表里认领', () => {
  it('列表里有同名规则时认领到它', () => {
    const rules = [
      { name: '别的规则', on: ['gift'] },
      { name: ENTER_RULE_NAME, on: ['user_enter'], enabled: false },
    ]
    expect(claimRule(rules, ENTER_RULE_NAME)).toEqual(rules[1])
  })

  it('列表里没有同名规则时返回 null，不是抛异常或返回第一条', () => {
    const rules = [{ name: '别的规则', on: ['gift'] }]
    expect(claimRule(rules, ENTER_RULE_NAME)).toBeNull()
  })
})

describe('Danmaku 页面：未选择直播间', () => {
  it('没有选中直播间时显示提示，不渲染规则表单', async () => {
    setActivePinia(createPinia())
    vi.stubGlobal('fetch', vi.fn())
    const wrapper = await mountDanmaku()
    expect(wrapper.text()).toContain('请先在顶部选择一个直播间')
    expect(wrapper.text()).not.toContain('进房欢迎')
  })
})

describe('Danmaku 页面：认领已保存配置（核心场景）', () => {
  it('GET 规则列表里有同名的"内置/进房欢迎"时，单人模板输入框显示已保存的内容', async () => {
    const savedRules: Partial<RuleView>[] = [
      {
        position: 0,
        name: ENTER_RULE_NAME,
        enabled: true,
        on: ['user_enter'],
        do: [{ type: 'danmaku', template: ['已保存单人模板A', '已保存单人模板B'] }],
      },
    ]
    vi.stubGlobal(
      'fetch',
      vi.fn().mockImplementation(() => Promise.resolve(ok(savedRules))),
    )
    setupStores()
    const wrapper = await mountDanmaku()

    const inputs = wrapper.findAll('input[placeholder="单人欢迎语模板"]')
    const values = inputs.map((i) => (i.element as HTMLInputElement).value)
    expect(values).toEqual(['已保存单人模板A', '已保存单人模板B'])
  })

  it('规则列表为空（新绑定）时退回默认模板，而不是空白', async () => {
    vi.stubGlobal(
      'fetch',
      vi.fn().mockImplementation(() => Promise.resolve(ok([]))),
    )
    setupStores()
    const wrapper = await mountDanmaku()

    const inputs = wrapper.findAll('input[placeholder="单人欢迎语模板"]')
    const values = inputs.map((i) => (i.element as HTMLInputElement).value)
    expect(values).toEqual(defaultEnterDraft().singleTemplates)
  })

  it('切换直播间会重新拉取并重新认领', async () => {
    const f = vi.fn().mockImplementation((url: string) => {
      if (url.includes('/bindings/1/rules')) {
        return Promise.resolve(
          ok([
            {
              position: 0,
              name: ENTER_RULE_NAME,
              on: ['user_enter'],
              do: [{ type: 'danmaku', template: ['房间1的模板'] }],
            },
          ]),
        )
      }
      if (url.includes('/bindings/2/rules')) {
        return Promise.resolve(
          ok([
            {
              position: 0,
              name: ENTER_RULE_NAME,
              on: ['user_enter'],
              do: [{ type: 'danmaku', template: ['房间2的模板'] }],
            },
          ]),
        )
      }
      return Promise.resolve(ok([]))
    })
    vi.stubGlobal('fetch', f)
    const { bindings } = setupStores()
    bindings.list = [
      { ...绑定, id: 1, roomId: '9000', permissions: [...绑定.permissions] },
      { ...绑定, id: 2, roomId: '9001', permissions: [...绑定.permissions] },
    ]
    bindings.select(1)
    const wrapper = await mountDanmaku()
    expect(
      wrapper
        .findAll('input[placeholder="单人欢迎语模板"]')
        .map((i) => (i.element as HTMLInputElement).value),
    ).toEqual(['房间1的模板'])

    bindings.select(2)
    await flushPromises()
    expect(
      wrapper
        .findAll('input[placeholder="单人欢迎语模板"]')
        .map((i) => (i.element as HTMLInputElement).value),
    ).toEqual(['房间2的模板'])
  })
})

describe('Danmaku 页面：四处悬空控件全部渲染，且都不 disabled', () => {
  it('五处"待后端支持"标签都出现（轮询x2、多人模板、盲盒单列、盲盒盈亏）', async () => {
    vi.stubGlobal(
      'fetch',
      vi.fn().mockImplementation(() => Promise.resolve(ok([]))),
    )
    setupStores()
    const wrapper = await mountDanmaku()

    const tags = wrapper.findAll('.n-tag').filter((t) => t.text() === '待后端支持')
    expect(tags.length).toBe(5)
  })

  it('轮询单选框、多人模板输入框、盲盒复选框都能正常交互（不是 disabled）', async () => {
    vi.stubGlobal(
      'fetch',
      vi.fn().mockImplementation(() => Promise.resolve(ok([]))),
    )
    setupStores()
    const wrapper = await mountDanmaku()

    const multiInput = wrapper.find('input[placeholder="多人合并欢迎语模板"]')
    expect(multiInput.exists()).toBe(true)
    expect(multiInput.attributes('disabled')).toBeUndefined()

    const checkboxLabels = ['盲盒礼物单独一类，不并入常规答谢', '盲盒盈亏统计']
    for (const label of checkboxLabels) {
      const checkbox = wrapper
        .findAll('.n-checkbox')
        .find((c) => c.text().includes(label.replace(/，.*/, '')))
      expect(checkbox, `复选框「${label}」应该存在`).toBeTruthy()
      expect(checkbox!.classes().join(' ')).not.toContain('n-checkbox--disabled')
    }
  })
})

describe('Danmaku 页面：dirty 与保存留白', () => {
  it('刚加载完成时 dirty 为假，保存按钮 disabled；改动草稿后 dirty 变真、按钮可点，但点击不发任何请求', async () => {
    const f = vi.fn().mockImplementation(() => Promise.resolve(ok([])))
    vi.stubGlobal('fetch', f)
    setupStores()
    const wrapper = await mountDanmaku()

    const saveBtn = () => wrapper.findAll('button').find((b) => b.text() === '保存并生效')
    expect(saveBtn()!.attributes('disabled')).toBeDefined()

    const firstTemplateInput = wrapper.find('input[placeholder="单人欢迎语模板"]')
    await firstTemplateInput.setValue('改过的模板')
    await flushPromises()

    expect(wrapper.text()).toContain('有未保存的改动')
    expect(saveBtn()!.attributes('disabled')).toBeUndefined()

    const callsBefore = f.mock.calls.length
    await saveBtn()!.trigger('click')
    await flushPromises()
    // Task 13 才接后端保存，本任务点了保存也不该发出任何请求
    expect(f.mock.calls.length).toBe(callsBefore)
  })
})
