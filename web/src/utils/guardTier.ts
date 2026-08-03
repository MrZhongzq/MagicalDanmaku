/**
 * guardTier 是「大航海档位」筛选的共享定义——进房欢迎（Danmaku.vue 的
 * EnterFilter）与 PK 串门欢迎（PkPanel.vue 的 PkVisitFilter，P5-5 7b）
 * 都要用同一套「舰长/提督/总督」档位下拉，不能各写一份、容易在数值
 * 映射上漂开（P4-4 已经在类似的「登记点分散」问题上吃过亏）。
 *
 * 抽成独立模块而不是让 PkPanel.vue 从 Danmaku.vue 里 import：Danmaku.vue
 * 本身要从 PkPanel.vue 导入 PK_RULE_NAME/PK_VISIT_RULE_NAME，反过来
 * 让 PkPanel.vue 也依赖 Danmaku.vue 会造成两个页面级 SFC 之间的循环
 * 依赖——技术上 ESM 不是不能处理循环引用，但两个页面级组件互相依赖
 * 对方的内部实现细节是明显的坏味道，抽出这个无状态的纯数据模块更干净。
 */

// 大航海档位：event.GuardGovernor=1 / GuardAdmiral=2 / GuardCaptain=3。
// 数值越小档位越高（总督最贵、数值最小），"及以上" 因此要用 in 一组数值
// 而不是简单的 gte——gte 在这种反向编号下语义会反过来。
export type GuardTier = 'captain' | 'admiral' | 'governor'

/** 每个档位对应「这个档位及以上」包含的 guardLevel 数值集合。 */
export const GUARD_TIER_VALUES: Record<GuardTier, number[]> = {
  captain: [1, 2, 3], // 舰长即可：三档都算
  admiral: [1, 2], // 提督及以上
  governor: [1], // 仅总督
}

export const GUARD_TIER_OPTIONS: { label: string; value: GuardTier }[] = [
  { label: '舰长即可（不限档位）', value: 'captain' },
  { label: '提督及以上', value: 'admiral' },
  { label: '仅总督', value: 'governor' },
]

/** arraysEqualUnordered 判断两个数值集合忽略顺序后是否相等。 */
export function arraysEqualUnordered(a: number[], b: unknown): boolean {
  if (!Array.isArray(b)) return false
  if (a.length !== b.length) return false
  const sortedA = [...a].sort()
  const sortedB = [...b].sort()
  return sortedA.every((v, i) => v === sortedB[i])
}

/**
 * guardTierFromValues 从一组 guardLevel 数值反推是哪个档位——
 * parseEnterFilter/parsePkVisitFilter 还原已保存条件时用。找不到匹配的
 * 档位（值集合不是任何一档的标准形状）时返回 undefined。
 */
export function guardTierFromValues(value: unknown): GuardTier | undefined {
  return (Object.keys(GUARD_TIER_VALUES) as GuardTier[]).find((k) =>
    arraysEqualUnordered(GUARD_TIER_VALUES[k], value),
  )
}
