import { describe, expect, it } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import { NRadioGroup, NSelect } from 'naive-ui'

// ConditionTree 是整个 P4-2 最难的一个组件：spec.Condition 递归树的可视化
// 编辑器。测试分三层：
//   1. 纯函数（kindOf/pruneCondition/defaultConditionForKind/值类型转换）——
//      逻辑最容易出错也最容易测的地方，覆盖尽量穷尽
//   2. 挂载级：操作符清单必须来自 props（不能硬编码，同 Task 8 MemberEditor
//      的模式）、切换形态、增删子节点
//   3. 递归组装的变异测试：三层嵌套条件，深处叶子的一次编辑要能正确
//      传导到根节点 emit 出的完整结构，兄弟分支不能丢
const {
  default: ConditionTree,
  kindOf,
  pruneCondition,
  defaultConditionForKind,
  defaultLeafCondition,
  detectValueKind,
  detectListElementKind,
  buildLeafValue,
} = await import('./ConditionTree.vue')

type Condition = import('@/api/rule-types').Condition

/** 与 GET /api/meta/operators 的真实返回形状一致：11 个操作符。 */
const FULL_OPERATORS = [
  { value: 'eq', label: '等于' },
  { value: 'ne', label: '不等于' },
  { value: 'gt', label: '大于' },
  { value: 'gte', label: '大于等于' },
  { value: 'lt', label: '小于' },
  { value: 'lte', label: '小于等于' },
  { value: 'contains', label: '包含' },
  { value: 'prefix', label: '以……开头' },
  { value: 'suffix', label: '以……结尾' },
  { value: 'regex', label: '匹配正则' },
  { value: 'in', label: '属于列表之一' },
]

const FIELD_OPTIONS = [
  { label: 'user.uid（用户 UID）', value: 'user.uid' },
  { label: 'user.guardLevel（大航海档位）', value: 'user.guardLevel' },
]

function mountTree(modelValue: Condition, removable = false) {
  return mount(ConditionTree, {
    props: {
      modelValue,
      operators: FULL_OPERATORS,
      fieldOptions: FIELD_OPTIONS,
      removable,
    },
  })
}

// ============================================================
// 一、纯函数
// ============================================================

describe('kindOf：判断五种互斥形态 + empty', () => {
  it.each([
    [{ field: 'user.uid', op: 'eq', value: 1 }, 'leaf'],
    [{ all: [] }, 'all'],
    [{ any: [] }, 'any'],
    [{ not: { field: 'a', op: 'eq', value: 1 } }, 'not'],
    [{ script: '' }, 'script'],
    [{ script: 'true' }, 'script'],
    [{}, 'empty'],
  ] as const)('%o → %s', (c, expected) => {
    expect(kindOf(c as Condition)).toBe(expected)
  })
})

describe('pruneCondition：把编辑态收拢成能过后端 Validate() 的形状', () => {
  it('field 为空的叶子视为"还没填完"，剪掉', () => {
    expect(pruneCondition({ field: '', op: 'eq', value: '' })).toBeUndefined()
  })

  it('field 非空的叶子原样保留', () => {
    expect(pruneCondition({ field: 'user.uid', op: 'eq', value: '123' })).toEqual({
      field: 'user.uid',
      op: 'eq',
      value: '123',
    })
  })

  it('all: [] 剪成 undefined（等于没有条件），而不是原样送出空数组', () => {
    expect(pruneCondition({ all: [] })).toBeUndefined()
  })

  it('all 只剩一个有效子节点时收拢成那个子节点本身，不再包一层 all', () => {
    const c: Condition = {
      all: [
        { field: '', op: 'eq', value: '' }, // 空叶子，会被剪掉
        { field: 'user.uid', op: 'eq', value: '5' },
      ],
    }
    expect(pruneCondition(c)).toEqual({ field: 'user.uid', op: 'eq', value: '5' })
  })

  it('all 剩两个及以上有效子节点时保留容器', () => {
    const c: Condition = {
      all: [
        { field: 'user.uid', op: 'eq', value: '5' },
        { field: 'gift.name', op: 'eq', value: '礼物' },
      ],
    }
    expect(pruneCondition(c)).toEqual({
      all: [
        { field: 'user.uid', op: 'eq', value: '5' },
        { field: 'gift.name', op: 'eq', value: '礼物' },
      ],
    })
  })

  it('any 的剪枝规则与 all 对称', () => {
    expect(pruneCondition({ any: [{ field: '', op: 'eq', value: '' }] })).toBeUndefined()
  })

  it('not 的子节点剪空后，整个 not 一起消失', () => {
    expect(pruneCondition({ not: { field: '', op: 'eq', value: '' } })).toBeUndefined()
  })

  it('not 的子节点非空时保留', () => {
    expect(pruneCondition({ not: { field: 'user.uid', op: 'eq', value: '5' } })).toEqual({
      not: { field: 'user.uid', op: 'eq', value: '5' },
    })
  })

  it('script 为空白字符串视为"还没写"，剪掉', () => {
    expect(pruneCondition({ script: '   ' })).toBeUndefined()
  })

  it('script 非空时保留', () => {
    expect(pruneCondition({ script: 'user.guardLevel > 0' })).toEqual({
      script: 'user.guardLevel > 0',
    })
  })

  it('empty（五个字段全缺省）直接消失', () => {
    expect(pruneCondition({})).toBeUndefined()
  })

  it('三层嵌套且全部有效时，剪枝不改变结构（幂等）', () => {
    const c: Condition = {
      all: [
        {
          any: [
            { field: 'user.uid', op: 'eq', value: 'a' },
            { field: 'gift.name', op: 'eq', value: 'b' },
          ],
        },
        { not: { field: 'guard.level', op: 'eq', value: 3 } },
      ],
    }
    expect(pruneCondition(c)).toEqual(c)
  })

  it('嵌套中某一分支剪空后，剩下一个分支时外层 all 收拢', () => {
    const c: Condition = {
      all: [
        { any: [{ field: '', op: 'eq', value: '' }] }, // 整个 any 会消失
        { not: { field: 'guard.level', op: 'eq', value: 3 } },
      ],
    }
    expect(pruneCondition(c)).toEqual({ not: { field: 'guard.level', op: 'eq', value: 3 } })
  })
})

describe('defaultConditionForKind：切换形态时的初始形状', () => {
  it('leaf/empty → 空叶子', () => {
    expect(defaultConditionForKind('leaf')).toEqual(defaultLeafCondition())
    expect(defaultConditionForKind('empty')).toEqual(defaultLeafCondition())
  })
  it('all/any → 各带一个空叶子的容器', () => {
    expect(defaultConditionForKind('all')).toEqual({ all: [defaultLeafCondition()] })
    expect(defaultConditionForKind('any')).toEqual({ any: [defaultLeafCondition()] })
  })
  it('not → 带一个空叶子子节点', () => {
    expect(defaultConditionForKind('not')).toEqual({ not: defaultLeafCondition() })
  })
  it('script → 空字符串', () => {
    expect(defaultConditionForKind('script')).toEqual({ script: '' })
  })
})

describe('值类型识别与序列化', () => {
  it('detectValueKind 按 JS 运行时类型识别', () => {
    expect(detectValueKind(5)).toBe('number')
    expect(detectValueKind(true)).toBe('boolean')
    expect(detectValueKind([1, 2])).toBe('list')
    expect(detectValueKind('x')).toBe('string')
    expect(detectValueKind(undefined)).toBe('string')
  })

  it('detectListElementKind 看第一个元素；空列表默认文本', () => {
    expect(detectListElementKind([1, 2, 3])).toBe('number')
    expect(detectListElementKind(['a', 'b'])).toBe('string')
    expect(detectListElementKind([])).toBe('string')
  })

  it('buildLeafValue 数字列表：非法数字文本被过滤而不是产出 NaN', () => {
    const v = buildLeafValue({
      valueKind: 'list',
      stringValue: '',
      numberValue: 0,
      boolValue: true,
      listValues: ['1', '2', 'x', '3'],
      listElementKind: 'number',
    })
    expect(v).toEqual([1, 2, 3])
  })

  it('buildLeafValue 文本列表：原样保留', () => {
    const v = buildLeafValue({
      valueKind: 'list',
      stringValue: '',
      numberValue: 0,
      boolValue: true,
      listValues: ['舰长', '提督'],
      listElementKind: 'string',
    })
    expect(v).toEqual(['舰长', '提督'])
  })
})

// ============================================================
// 二、挂载级
// ============================================================

describe('ConditionTree 挂载：操作符清单来自 props，不硬编码', () => {
  it('后端只给 3 个操作符时，op 下拉就只有 3 项，不会自己补全成 11 个', async () => {
    const partial = FULL_OPERATORS.slice(0, 3)
    const wrapper = mount(ConditionTree, {
      props: {
        modelValue: defaultLeafCondition(),
        operators: partial,
        fieldOptions: FIELD_OPTIONS,
      },
    })
    await flushPromises()
    const selects = wrapper.findAllComponents(NSelect)
    // 叶子行的第二个 NSelect 是 op 选择器（第一个是 field，第三个是值类型）
    const opSelect = selects[1]
    const options = opSelect.props('options') as { value: string }[]
    expect(options.map((o) => o.value)).toEqual(['eq', 'ne', 'gt'])
  })
})

describe('ConditionTree 挂载：切换形态与增删子节点', () => {
  // 注：数子节点数量用 DOM 里 `.condition-tree` 的个数，不用
  // `findAllComponents(ConditionTree)`——这是 @vue/test-utils 对自引用
  // 组件的一个已验证的局限：初始挂载时就存在于 props 里的嵌套结构，
  // findAllComponents 能正确找到（下面"递归组装"一节用的就是这个方式，
  // 而且必须用它才能拿到组件实例去触发 NSelect 的 emit）；但挂载后才由
  // 响应式更新新增的自引用子组件实例，findAllComponents 找不全
  // （实测只能找到 1 个，即便 DOM 里已经渲染出了 2 个 `.condition-tree`）。
  // 数"渲染了几个节点"这件事本身，DOM 计数更可靠也更贴近用户能看到的东西。
  it('默认叶子切到 all 后出现一个子节点，点"+ 添加"后变成两个', async () => {
    const wrapper = mountTree(defaultLeafCondition())
    const kindRadio = wrapper.findComponent(NRadioGroup)
    kindRadio.vm.$emit('update:value', 'all')
    await flushPromises()

    expect(wrapper.findAll('.condition-tree')).toHaveLength(2) // 根 + 1 子节点

    const addBtn = wrapper.findAll('button').find((b) => b.text().includes('添加'))
    expect(addBtn, '应该有"+ 添加"按钮').toBeTruthy()
    await addBtn!.trigger('click')
    await flushPromises()

    expect(wrapper.findAll('.condition-tree')).toHaveLength(3) // 根 + 2 子节点
    expect(wrapper.emitted('update:modelValue')?.at(-1)?.[0]).toEqual({
      all: [defaultLeafCondition(), defaultLeafCondition()],
    })
  })

  it('删除子节点后容器变空，出现"当前没有任何子条件"提示', async () => {
    const wrapper = mountTree({ all: [defaultLeafCondition()] })
    const removeBtn = wrapper.findAll('button').find((b) => b.text().includes('删除此条件'))
    expect(removeBtn, '子节点应该有"删除此条件"按钮').toBeTruthy()
    await removeBtn!.trigger('click')
    await flushPromises()

    expect(wrapper.text()).toContain('当前没有任何子条件')
    expect(wrapper.emitted('update:modelValue')?.at(-1)?.[0]).toEqual({ all: [] })
  })

  it('切到 not 只渲染一个子节点，没有"+ 添加"按钮', async () => {
    const wrapper = mountTree(defaultLeafCondition())
    wrapper.findComponent(NRadioGroup).vm.$emit('update:value', 'not')
    await flushPromises()

    expect(wrapper.findAll('.condition-tree')).toHaveLength(2) // 根 + 1 子节点
    expect(wrapper.findAll('button').find((b) => b.text().includes('添加'))).toBeUndefined()
  })

  it('选择 op=in 且当前值类型不是列表时，自动切换成列表类型', async () => {
    const wrapper = mountTree({ field: 'user.guardLevel', op: 'eq', value: '' })
    const opSelect = wrapper.findAllComponents(NSelect)[1]
    opSelect.vm.$emit('update:value', 'in')
    await flushPromises()

    expect(wrapper.emitted('update:modelValue')?.at(-1)?.[0]).toEqual({
      field: 'user.guardLevel',
      op: 'in',
      value: [],
    })
  })
})

// ============================================================
// 三、变异测试：递归组装（三层嵌套：all[ any[叶子,叶子], not[叶子] ]）
// ============================================================

describe('ConditionTree 递归组装：深层编辑要正确传导到根节点，兄弟分支不能丢', () => {
  it('编辑 not 分支里的叶子 op，根节点 emit 出的完整结构里 any 分支原样保留', async () => {
    const initial: Condition = {
      all: [
        {
          any: [
            { field: 'user.uid', op: 'eq', value: 'a' },
            { field: 'gift.name', op: 'eq', value: 'b' },
          ],
        },
        { not: { field: 'guard.level', op: 'eq', value: 3 } },
      ],
    }
    const wrapper = mountTree(initial)

    // 定位到 not 分支里那个叶子对应的 ConditionTree 实例
    const allInstances = wrapper.findAllComponents(ConditionTree)
    const target = allInstances.find((c) => {
      const mv = c.props('modelValue') as Condition
      return mv.field === 'guard.level'
    })
    expect(target, '应该能找到 guard.level 这个叶子节点的实例').toBeTruthy()

    // 该叶子行的第二个 NSelect 是 op 选择器
    const opSelect = target!.findAllComponents(NSelect)[1]
    opSelect.vm.$emit('update:value', 'ne')
    await flushPromises()

    const lastEmitted = wrapper.emitted('update:modelValue')?.at(-1)?.[0]
    expect(lastEmitted).toEqual({
      all: [
        {
          any: [
            { field: 'user.uid', op: 'eq', value: 'a' },
            { field: 'gift.name', op: 'eq', value: 'b' },
          ],
        },
        { not: { field: 'guard.level', op: 'ne', value: 3 } },
      ],
    })
  })
})
