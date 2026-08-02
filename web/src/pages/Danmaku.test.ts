import { beforeEach, describe, expect, it, vi } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import type { Permission } from '@/api'

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
  buildBroadcastRule,
  buildBroadcastSchedule,
  parseBroadcastDraft,
  defaultBroadcastDraft,
  BROADCAST_RULE_NAME,
  buildFollowRule,
  parseFollowDraft,
  defaultFollowDraft,
  FOLLOW_RULE_NAME,
  buildShareRule,
  defaultShareDraft,
  SHARE_RULE_NAME,
  buildGuardRule,
  parseGuardDraft,
  defaultGuardDraft,
  GUARD_RULE_NAME,
} = Danmaku
const { PK_RULE_NAME } = await import('@/components/PkPanel.vue')
const { useBindingsStore } = await import('@/stores/bindings')

type RuleView = import('@/api/rule-types').RuleView

function ok(body: unknown, status = 200) {
  return new Response(JSON.stringify(body), {
    status,
    headers: { 'Content-Type': 'application/json' },
  })
}

function err(status: number, message: string) {
  return new Response(JSON.stringify({ error: message }), {
    status,
    headers: { 'Content-Type': 'application/json' },
  })
}

/**
 * stubSaveFetch 给「保存」相关测试用：GET 规则列表（初次加载与 save()
 * 内部各调一次，返回同一份 `rules`）、PUT 整组替换、POST reload。
 *
 * PUT/POST 的响应可以单独覆盖，用来演练"第 1 步失败"“第 2 步失败”这类
 * 场景。**每次都要返回新的 Response 实例**——Response.body 只能读一次，
 * GET 在测试里至少会被调用两次（挂载时一次，保存时一次）。
 */
function stubSaveFetch(opts: {
  rules?: RuleView[]
  putResponse?: () => Response
  reloadResponse?: () => Response
  onPut?: (body: unknown) => void
}) {
  const rules = opts.rules ?? []
  const f = vi.fn().mockImplementation((url: string, init?: RequestInit) => {
    const method = init?.method ?? 'GET'
    if (url === '/api/bindings/1/rules' && method === 'GET') {
      return Promise.resolve(ok(rules))
    }
    if (url === '/api/bindings/1/rules' && method === 'PUT') {
      opts.onPut?.(JSON.parse(init!.body as string))
      return Promise.resolve(opts.putResponse ? opts.putResponse() : ok({ status: 'ok' }))
    }
    if (url === '/api/bindings/1/reload' && method === 'POST') {
      return Promise.resolve(opts.reloadResponse ? opts.reloadResponse() : ok({ status: 'ok' }))
    }
    throw new Error(`stubSaveFetch 没处理这个请求: ${method} ${url}`)
  })
  vi.stubGlobal('fetch', f)
  return f
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

/** permissions 可覆盖默认的 ['rule:read', 'rule:write']——用于钉「缺 rule:write 时顶警告」的测试。 */
function setupStores(permissions: Permission[] = [...绑定.permissions]) {
  setActivePinia(createPinia())
  const bindings = useBindingsStore()
  bindings.list = [{ ...绑定, permissions }]
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
  it('buildEnterRule 组装出的规则用固定 name 与 on: ["user_enter"]，do[0] 带 template/templateMulti/pick 三项', () => {
    const rule = buildEnterRule(defaultEnterDraft(), '9000')
    expect(rule.name).toBe(ENTER_RULE_NAME)
    expect(rule.on).toEqual(['user_enter'])
    expect(rule.do).toEqual([
      {
        type: 'danmaku',
        template: defaultEnterDraft().singleTemplates,
        templateMulti: defaultEnterDraft().multiTemplates,
        pick: 'random',
      },
    ])
  })

  it('buildGiftRule 组装出的规则用固定 name 与 on: ["gift"]', () => {
    const rule = buildGiftRule(defaultGiftDraft())
    expect(rule.name).toBe(GIFT_RULE_NAME)
    expect(rule.on).toEqual(['gift'])
  })

  // 全批次终审项【3b】：默认模板应体现多礼物合并（用 gifts，而不是只取
  // 第一件礼物名字的 .gift.name）。server/internal/rules/aggregate.go
  // 的 mergeBuckets 早就在填 vars["gifts"]，join 函数也早就存在
  // （server/internal/rules/template.go 的 funcMap），默认模板没跟上。
  it('defaultGiftDraft 的默认模板用 join .gifts 体现多礼物合并，而不是只取第一件的 .gift.name', () => {
    const template = defaultGiftDraft().templates[0]
    expect(template).toContain('.gifts')
    expect(template).not.toContain('.gift.name')
  })

  describe('Pick/TemplateMulti：P4-3 接通的两个字段', () => {
    it('buildEnterRule：pickMode="sequential" 时 do[0].pick 是 "sequential"', () => {
      const draft = { ...defaultEnterDraft(), pickMode: 'sequential' as const }
      expect(buildEnterRule(draft, '9000').do![0].pick).toBe('sequential')
    })

    it('buildEnterRule：多人模板过滤空白后写进 do[0].templateMulti', () => {
      const draft = { ...defaultEnterDraft(), multiTemplates: ['多人模板A', '  ', ''] }
      expect(buildEnterRule(draft, '9000').do![0].templateMulti).toEqual(['多人模板A'])
    })

    it('buildEnterRule：进房欢迎恒带 aggregate，不会撞上「templateMulti 无 aggregate」的后端校验', () => {
      // 断言 aggregate 字段确实总是存在——这是「templateMulti 天然安全」的前提，
      // 见 Danmaku.vue 文件头 P4-3 说明。
      expect(buildEnterRule(defaultEnterDraft(), '9000').aggregate).toBeTruthy()
    })

    it('buildGiftRule/buildBroadcastRule：pickMode 直接进 do[0].pick', () => {
      expect(
        buildGiftRule({ ...defaultGiftDraft(), pickMode: 'sequential' as const }).do![0].pick,
      ).toBe('sequential')
      expect(
        buildBroadcastRule({ ...defaultBroadcastDraft(), pickMode: 'sequential' as const }).do![0]
          .pick,
      ).toBe('sequential')
      expect(buildGiftRule(defaultGiftDraft()).do![0].pick).toBe('random')
      expect(buildBroadcastRule(defaultBroadcastDraft()).do![0].pick).toBe('random')
    })
  })

  it('parseEnterDraft(null) 返回默认草稿——新绑定还没配过规则', () => {
    expect(parseEnterDraft(null)).toEqual(defaultEnterDraft())
  })

  it('parseEnterDraft 还原 enabled/aggregate/模板；已保存规则没有 templateMulti/pick 时回落默认值（与旧配置兼容）', () => {
    const savedRule = {
      name: ENTER_RULE_NAME,
      enabled: false,
      on: ['user_enter'],
      aggregate: { window: '2m', minCount: 4, by: 'type' },
      do: [{ type: 'danmaku', template: ['已保存单人模板A', '已保存单人模板B'] }],
    }
    const draft = parseEnterDraft(savedRule)
    expect(draft.enabled).toBe(false)
    expect(draft.groupMode).toBe('merge')
    expect(draft.windowSeconds).toBe(120)
    expect(draft.minCount).toBe(4)
    expect(draft.singleTemplates).toEqual(['已保存单人模板A', '已保存单人模板B'])
    // 已保存规则没有 templateMulti 字段（旧配置），回落默认值；pick 未写等同 "random"。
    expect(draft.multiTemplates).toEqual(defaultEnterDraft().multiTemplates)
    expect(draft.pickMode).toBe('random')
  })

  it('parseEnterDraft 还原真实存在的 templateMulti 与 pick="sequential"', () => {
    const savedRule = {
      name: ENTER_RULE_NAME,
      enabled: true,
      on: ['user_enter'],
      aggregate: { window: '2m', by: 'type' },
      do: [
        {
          type: 'danmaku',
          template: ['单人模板'],
          templateMulti: ['多人模板A', '多人模板B'],
          pick: 'sequential',
        },
      ],
    }
    const draft = parseEnterDraft(savedRule)
    expect(draft.multiTemplates).toEqual(['多人模板A', '多人模板B'])
    expect(draft.pickMode).toBe('sequential')
  })

  it('parseEnterDraft：pick 为空字符串时等同 "random"（与历史配置兼容）', () => {
    const savedRule = {
      name: ENTER_RULE_NAME,
      on: ['user_enter'],
      aggregate: { window: '2m', by: 'type' },
      do: [{ type: 'danmaku', template: ['模板'], pick: '' }],
    }
    expect(parseEnterDraft(savedRule).pickMode).toBe('random')
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

describe('Danmaku 页面：一处悬空控件全部渲染，且都不 disabled', () => {
  it('四处"待后端支持"标签都出现（P4-3 接通了轮询/多人模板后，只剩盲盒 x2 + PK x2）', async () => {
    vi.stubGlobal(
      'fetch',
      vi.fn().mockImplementation(() => Promise.resolve(ok([]))),
    )
    setupStores()
    const wrapper = await mountDanmaku()

    const tags = wrapper.findAll('.n-tag').filter((t) => t.text() === '待后端支持')
    // 礼物答谢的盲盒单列、盲盒盈亏 = 2；PkPanel 的 PK匹配信息、PK串门欢迎 = 2
    expect(tags.length).toBe(4)
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

// ---- 全分支终审第 4 条：只有 rule:read 的成员应该被提前警告，而不是把整页配完才被后端 403 打回 ----
describe('Danmaku 页面：rule:write 权限门禁（提示但不锁面板）', () => {
  it('缺 rule:write 时顶部出现警告，但控件都不 disabled', async () => {
    vi.stubGlobal(
      'fetch',
      vi.fn().mockImplementation(() => Promise.resolve(ok([]))),
    )
    setupStores(['rule:read']) // 没有 rule:write
    const wrapper = await mountDanmaku()

    expect(wrapper.text()).toContain('你在这个直播间没有 rule:write 权限')

    const enterSwitch = wrapper.find('input[placeholder="单人欢迎语模板"]')
    expect(enterSwitch.exists()).toBe(true)
    expect(enterSwitch.attributes('disabled')).toBeUndefined()
  })

  it('有 rule:write 权限时不显示警告条', async () => {
    vi.stubGlobal(
      'fetch',
      vi.fn().mockImplementation(() => Promise.resolve(ok([]))),
    )
    setupStores()
    const wrapper = await mountDanmaku()

    expect(wrapper.text()).not.toContain('没有 rule:write 权限')
  })
})

describe('Danmaku 页面：dirty', () => {
  it('刚加载完成时 dirty 为假，保存按钮 disabled；改动草稿后 dirty 变真、按钮可点', async () => {
    stubSaveFetch({ rules: [] })
    setupStores()
    const wrapper = await mountDanmaku()

    const saveBtn = () => wrapper.findAll('button').find((b) => b.text() === '保存并生效')
    expect(saveBtn()!.attributes('disabled')).toBeDefined()

    const firstTemplateInput = wrapper.find('input[placeholder="单人欢迎语模板"]')
    await firstTemplateInput.setValue('改过的模板')
    await flushPromises()

    expect(wrapper.text()).toContain('有未保存的改动')
    expect(saveBtn()!.attributes('disabled')).toBeUndefined()
  })
})

// ============================================================
// Task 13：保存与热重载的统一交互
// ============================================================

describe('Danmaku 页面：保存（Task 13 接上 useDraft）', () => {
  it('点击「保存并生效」依次 GET → PUT → POST reload，成功后 dirty 归假、弹出成功提示', async () => {
    const f = stubSaveFetch({ rules: [] })
    setupStores()
    const wrapper = await mountDanmaku()

    const firstTemplateInput = wrapper.find('input[placeholder="单人欢迎语模板"]')
    await firstTemplateInput.setValue('改过的模板')
    await flushPromises()

    const saveBtn = () => wrapper.findAll('button').find((b) => b.text() === '保存并生效')!
    const callsBefore = f.mock.calls.length
    await saveBtn().trigger('click')
    await flushPromises()

    // 挂载时已经有一次 GET，点保存之后应该再追加 GET + PUT + POST 三次
    expect(f.mock.calls.length).toBe(callsBefore + 3)
    const methodsAndUrls = f.mock.calls.slice(callsBefore).map((call) => {
      const [url, init] = call as [string, RequestInit?]
      return `${init?.method ?? 'GET'} ${url}`
    })
    expect(methodsAndUrls).toEqual([
      'GET /api/bindings/1/rules',
      'PUT /api/bindings/1/rules',
      'POST /api/bindings/1/reload',
    ])

    expect(wrapper.text()).not.toContain('有未保存的改动')
    expect(saveBtn().attributes('disabled')).toBeDefined()
    expect(messageMock.success).toHaveBeenCalledWith('已保存并生效')
  })

  it(
    '【关键：钉死"整组替换不误删别的页面的规则"】库里已有一条自定义弹幕姬页建的规则时，' +
      '从弹幕姬页保存，PUT 请求体里这条自建规则必须还在',
    async () => {
      const customRule: RuleView = {
        name: '舰长专属欢迎',
        position: 0,
        on: ['user_enter'],
        when: { field: 'user.uid', op: 'eq', value: '12345' },
        do: [{ type: 'danmaku', template: ['欢迎舰长回家'] }],
      }
      let putBody: RuleView[] | null = null
      stubSaveFetch({
        rules: [customRule],
        onPut: (body) => {
          putBody = body as RuleView[]
        },
      })
      setupStores()
      const wrapper = await mountDanmaku()

      const firstTemplateInput = wrapper.find('input[placeholder="单人欢迎语模板"]')
      await firstTemplateInput.setValue('改过的模板')
      await flushPromises()

      const saveBtn = () => wrapper.findAll('button').find((b) => b.text() === '保存并生效')!
      await saveBtn().trigger('click')
      await flushPromises()

      expect(putBody).not.toBeNull()
      const names = putBody!.map((r) => r.name)
      // 核心断言：自定义弹幕姬页建的这条规则不属于弹幕姬页管的七个内置
      // 名字，保存时必须原样保留，不能被弹幕姬页的整组替换悄悄删掉。
      expect(names).toContain('舰长专属欢迎')
      expect(names).toContain(ENTER_RULE_NAME)
    },
  )

  it('第 1 步（PUT 写库）失败：弹出后端错误原文，dirty 保持真（改动没丢）', async () => {
    stubSaveFetch({
      rules: [],
      putResponse: () => err(422, '第 1 条规则(内置/进房欢迎)不合法: 正则非法'),
    })
    setupStores()
    const wrapper = await mountDanmaku()

    const firstTemplateInput = wrapper.find('input[placeholder="单人欢迎语模板"]')
    await firstTemplateInput.setValue('改过的模板')
    await flushPromises()

    const saveBtn = () => wrapper.findAll('button').find((b) => b.text() === '保存并生效')!
    await saveBtn().trigger('click')
    await flushPromises()

    expect(messageMock.error).toHaveBeenCalledWith('第 1 条规则(内置/进房欢迎)不合法: 正则非法')
    expect(wrapper.text()).toContain('有未保存的改动')
    expect(saveBtn().attributes('disabled')).toBeUndefined()
    // 第 1 步失败，不是"保存了一半"，不该出现第三态提示
    expect(wrapper.text()).not.toContain('已保存到数据库，但重载失败')
  })

  it(
    '第 2 步（reload）失败：库已经改了、引擎还在跑旧配置——' +
      'dirty 不归假，界面出现"已保存到数据库，但重载失败"的持久提示，' +
      '且原样带上后端"仍在用上一份配置运行"的安抚文案',
    async () => {
      const reloadErrorMessage = '重载失败，仍在用上一份配置运行: 规则 内置/进房欢迎 的正则非法'
      stubSaveFetch({
        rules: [],
        reloadResponse: () => err(422, reloadErrorMessage),
      })
      setupStores()
      const wrapper = await mountDanmaku()

      const firstTemplateInput = wrapper.find('input[placeholder="单人欢迎语模板"]')
      await firstTemplateInput.setValue('改过的模板')
      await flushPromises()

      const saveBtn = () => wrapper.findAll('button').find((b) => b.text() === '保存并生效')!
      await saveBtn().trigger('click')
      await flushPromises()

      expect(messageMock.error).toHaveBeenCalledWith(reloadErrorMessage)
      // 用户的判断：不能让 dirty 归假——归假会让操作者以为已经完全生效，
      // 但引擎其实还在跑旧规则。
      expect(wrapper.text()).toContain('有未保存的改动')
      expect(saveBtn().attributes('disabled')).toBeUndefined()
      expect(wrapper.text()).toContain('已保存到数据库，但重载失败')
      expect(wrapper.text()).toContain(reloadErrorMessage)
    },
  )

  it(
    '【审查追加：钉住"第三态不跨绑定泄漏"】在绑定 A 上保存触发第三态提示后切到绑定 B，' +
      '提示必须清空——不清的话，操作者在 B 房间关掉这条提示，会把 A 房间那个' +
      '"引擎其实还在跑旧配置"的未解决信号一并清没',
    async () => {
      const reloadErrorMessage = '重载失败，仍在用上一份配置运行: 规则 内置/进房欢迎 的正则非法'
      const f = vi.fn().mockImplementation((url: string, init?: RequestInit) => {
        const method = init?.method ?? 'GET'
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
      const { bindings } = setupStores()
      bindings.list = [
        { ...绑定, id: 1, roomId: '9000', permissions: [...绑定.permissions] },
        { ...绑定, id: 2, roomId: '9001', permissions: [...绑定.permissions] },
      ]
      bindings.select(1)
      const wrapper = await mountDanmaku()

      // 在绑定 A（id=1）上触发一次「PUT 成功但 reload 失败」
      const firstTemplateInput = wrapper.find('input[placeholder="单人欢迎语模板"]')
      await firstTemplateInput.setValue('改过的模板')
      await flushPromises()
      const saveBtn = () => wrapper.findAll('button').find((b) => b.text() === '保存并生效')!
      await saveBtn().trigger('click')
      await flushPromises()
      expect(wrapper.text()).toContain('已保存到数据库，但重载失败')

      // 切到绑定 B（id=2）——loadRules 成功之后，A 房间的第三态提示必须清空
      bindings.select(2)
      await flushPromises()

      expect(wrapper.text()).not.toContain('已保存到数据库，但重载失败')
    },
  )
})

// ============================================================
// Task 10：PK 播报、轮播消息、其他答谢（关注/分享/上舰）
// ============================================================

describe('Danmaku 纯函数：轮播消息（真功能，schedule 驱动）', () => {
  it('buildBroadcastSchedule 在 interval 模式下生成 6 段 cron', () => {
    const draft = {
      ...defaultBroadcastDraft(),
      scheduleMode: 'interval' as const,
      intervalMinutes: 5,
    }
    expect(buildBroadcastSchedule(draft)).toBe('0 */5 * * * *')
  })

  it('buildBroadcastSchedule 在 cron 模式下原样使用用户填写的表达式', () => {
    const draft = {
      ...defaultBroadcastDraft(),
      scheduleMode: 'cron' as const,
      cronExpr: '0 0 */2 * * *',
    }
    expect(buildBroadcastSchedule(draft)).toBe('0 0 */2 * * *')
  })

  it(
    '【关键：变异测试验证过的一条】buildBroadcastRule 只产出 schedule，绝不同时产出 on —— ' +
      '否则后端 rules.Rule.Validate()（server/internal/rules/rule.go 第 61-68 行）会因 on/schedule 互斥拒收整条规则（422）',
    () => {
      const rule = buildBroadcastRule(defaultBroadcastDraft())
      expect(rule.schedule).toBeTruthy()
      expect(rule.on).toBeUndefined()
      expect(rule.name).toBe(BROADCAST_RULE_NAME)
    },
  )

  it('parseBroadcastDraft(null) 返回默认草稿', () => {
    expect(parseBroadcastDraft(null)).toEqual(defaultBroadcastDraft())
  })

  it('parseBroadcastDraft 还原 interval 模式的分钟数与模板', () => {
    const savedRule = {
      name: BROADCAST_RULE_NAME,
      enabled: false,
      schedule: '0 */15 * * * *',
      do: [{ type: 'danmaku', template: ['已保存的轮播模板'] }],
    }
    const draft = parseBroadcastDraft(savedRule)
    expect(draft.enabled).toBe(false)
    expect(draft.scheduleMode).toBe('interval')
    expect(draft.intervalMinutes).toBe(15)
    expect(draft.templates).toEqual(['已保存的轮播模板'])
  })

  it('parseBroadcastDraft 遇到非「按分钟间隔」形状的 cron 时落回 cron 自定义模式', () => {
    const savedRule = { name: BROADCAST_RULE_NAME, schedule: '0 30 9 * * 1', do: [] }
    const draft = parseBroadcastDraft(savedRule)
    expect(draft.scheduleMode).toBe('cron')
    expect(draft.cronExpr).toBe('0 30 9 * * 1')
  })
})

describe('Danmaku 纯函数：关注答谢 / 分享答谢 / 上舰答谢（真功能）', () => {
  it('buildFollowRule 用固定 name 与 on: ["user_follow"]', () => {
    const rule = buildFollowRule(defaultFollowDraft())
    expect(rule.name).toBe(FOLLOW_RULE_NAME)
    expect(rule.on).toEqual(['user_follow'])
  })

  it('buildShareRule 用固定 name 与 on: ["user_share"]', () => {
    const rule = buildShareRule(defaultShareDraft())
    expect(rule.name).toBe(SHARE_RULE_NAME)
    expect(rule.on).toEqual(['user_share'])
  })

  it('buildGuardRule 用固定 name 与 on: ["guard_buy"]，默认模板用 {{if .guard.isRenew}} 区分新购/续费', () => {
    const rule = buildGuardRule(defaultGuardDraft())
    expect(rule.name).toBe(GUARD_RULE_NAME)
    expect(rule.on).toEqual(['guard_buy'])
    expect(rule.do![0].template![0]).toContain('{{if .guard.isRenew}}')
    expect(rule.do![0].template![0]).toContain('{{.guard.count}}')
    expect(rule.do![0].template![0]).toContain('{{.guard.name}}')
  })

  it('parseGuardDraft 还原已保存的上舰答谢模板', () => {
    const savedRule = {
      name: GUARD_RULE_NAME,
      on: ['guard_buy'],
      do: [{ type: 'danmaku', template: ['已保存的上舰模板'] }],
    }
    expect(parseGuardDraft(savedRule).templates).toEqual(['已保存的上舰模板'])
  })

  it('parseFollowDraft(null) 与已保存规则的往返', () => {
    expect(parseFollowDraft(null)).toEqual(defaultFollowDraft())
    const savedRule = {
      name: FOLLOW_RULE_NAME,
      enabled: false,
      on: ['user_follow'],
      do: [{ type: 'danmaku', template: ['已保存的关注答谢模板'] }],
    }
    const draft = parseFollowDraft(savedRule)
    expect(draft.enabled).toBe(false)
    expect(draft.templates).toEqual(['已保存的关注答谢模板'])
  })
})

describe('Danmaku 页面：轮播消息 / 其他答谢 认领与渲染', () => {
  it('轮播消息、关注答谢、分享答谢、上舰答谢四个模板输入框都用默认值渲染', async () => {
    vi.stubGlobal(
      'fetch',
      vi.fn().mockImplementation(() => Promise.resolve(ok([]))),
    )
    setupStores()
    const wrapper = await mountDanmaku()

    const placeholders = ['轮播消息模板', '关注答谢模板', '分享答谢模板', '上舰答谢模板']
    for (const placeholder of placeholders) {
      const inputs = wrapper.findAll(`input[placeholder="${placeholder}"]`)
      expect(inputs.length, `「${placeholder}」应至少渲染一个输入框`).toBeGreaterThan(0)
    }
  })

  it('认领已保存的四条规则（轮播/关注/分享/上舰），模板输入框显示已保存内容', async () => {
    const savedRules = [
      {
        position: 0,
        name: BROADCAST_RULE_NAME,
        enabled: true,
        schedule: '0 */20 * * * *',
        do: [{ type: 'danmaku', template: ['房间专属轮播文案'] }],
      },
      {
        position: 1,
        name: FOLLOW_RULE_NAME,
        enabled: true,
        on: ['user_follow'],
        do: [{ type: 'danmaku', template: ['房间专属关注答谢'] }],
      },
      {
        position: 2,
        name: SHARE_RULE_NAME,
        enabled: true,
        on: ['user_share'],
        do: [{ type: 'danmaku', template: ['房间专属分享答谢'] }],
      },
      {
        position: 3,
        name: GUARD_RULE_NAME,
        enabled: true,
        on: ['guard_buy'],
        do: [{ type: 'danmaku', template: ['房间专属上舰答谢'] }],
      },
    ]
    vi.stubGlobal(
      'fetch',
      vi.fn().mockImplementation(() => Promise.resolve(ok(savedRules))),
    )
    setupStores()
    const wrapper = await mountDanmaku()

    const valueOf = (placeholder: string) =>
      (wrapper.find(`input[placeholder="${placeholder}"]`).element as HTMLInputElement).value

    expect(valueOf('轮播消息模板')).toBe('房间专属轮播文案')
    expect(valueOf('关注答谢模板')).toBe('房间专属关注答谢')
    expect(valueOf('分享答谢模板')).toBe('房间专属分享答谢')
    expect(valueOf('上舰答谢模板')).toBe('房间专属上舰答谢')
  })

  it('轮播消息的默认播放方式（随机抽取）与轮询选项在界面上明确说明，不藏在 tooltip 里', async () => {
    vi.stubGlobal(
      'fetch',
      vi.fn().mockImplementation(() => Promise.resolve(ok([]))),
    )
    setupStores()
    const wrapper = await mountDanmaku()
    expect(wrapper.text()).toContain('默认「随机抽取」')
    expect(wrapper.text()).toContain('选「轮询」则按顺序循环')
  })
})

describe('Danmaku 页面：PK 播报子组件正确挂载并纳入 claimRule/dirty', () => {
  it('claimRule 能用 PK_RULE_NAME 从规则列表里认领到 PK 播报规则', () => {
    const rules = [{ name: PK_RULE_NAME, on: ['battle'], enabled: false }]
    expect(claimRule(rules, PK_RULE_NAME)).toEqual(rules[0])
  })

  it('PkPanel 的"待后端支持"标签在整页里可见（PK匹配信息 + PK串门欢迎）', async () => {
    vi.stubGlobal(
      'fetch',
      vi.fn().mockImplementation(() => Promise.resolve(ok([]))),
    )
    setupStores()
    const wrapper = await mountDanmaku()
    expect(wrapper.text()).toContain('PK 播报')
    expect(wrapper.text()).toContain('PK 匹配信息')
    expect(wrapper.text()).toContain('PK 串门欢迎')
  })

  it('改动 PK 播报的匹配模板会让整页 dirty 变真（PkPanel 的改动会同步进页面草稿）', async () => {
    vi.stubGlobal(
      'fetch',
      vi.fn().mockImplementation(() => Promise.resolve(ok([]))),
    )
    setupStores()
    const wrapper = await mountDanmaku()

    const saveBtn = () => wrapper.findAll('button').find((b) => b.text() === '保存并生效')
    expect(saveBtn()!.attributes('disabled')).toBeDefined()

    const pkInput = wrapper.find('input[placeholder="PK播报语模板"]')
    await pkInput.setValue('改过的PK模板')
    await flushPromises()

    expect(saveBtn()!.attributes('disabled')).toBeUndefined()
  })
})
