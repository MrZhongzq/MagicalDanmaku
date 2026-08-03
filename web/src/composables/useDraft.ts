import { computed, onUnmounted, ref, type ComputedRef, type Ref } from 'vue'
import { onBeforeRouteLeave } from 'vue-router'
import { ApiError, request } from '@/api'
import type { Rule, RuleView } from '@/api/rule-types'

/**
 * useDraft 是「弹幕姬」「自定义弹幕姬」这类有草稿态的页面共用的保存交互。
 *
 * 用户的要求（Task 13 简报）：右上角一个保存按键，改动只进内存草稿，
 * 按下保存才写库、才让规则引擎拿到新配置。封装三件事：
 *
 * 1. 草稿与「已保存基线」的比较 → dirty
 * 2. 保存流程：GET 现有规则 → 合并 → PUT 写库 → POST reload 让引擎生效
 * 3. 离开拦截：onBeforeRouteLeave（应用内路由跳转）+ beforeunload（刷新/关标签页）
 *
 * ## 整组替换的坑：合并而不是覆盖
 *
 * `PUT /api/bindings/{id}/rules` 是整组替换——发过去什么，库里就只剩什么。
 * 弹幕姬页管全部内置规则（`Danmaku.vue` 的 `OWNED_RULE_NAMES`，
 * 目前九条），自定义弹幕姬页管用户自建的若干条，两边各自调用这个
 * composable、各自传一份 `isOwned`。保存时：
 *
 *   1. 先 GET 一次现有的全部规则
 *   2. 用 `isOwned` 挑出「不归本页管」的规则，原样保留
 *   3. 加上 `buildRules()` 组装出的「本页管的」规则
 *   4. 一起 PUT 回去
 *
 * 没有这一步的话，弹幕姬页保存一次就会把自定义弹幕姬页建的规则全部
 * 删掉（反过来也一样）——这是本模块最容易踩、后果也最重的坑。
 *
 * ## 第 2 步（reload）失败时 dirty 照样归假
 *
 * PUT 成功但 POST reload 失败，是一个真实的中间态：库已经改了，但规则
 * 引擎还在跑旧配置。`dirty` 回答的是「草稿和数据库是否一致」，PUT 一旦
 * 成功这件事就是真的，不该被 reload 的结果污染——这两件事分别由
 * `dirty` 与 `partialFailureMessage` 独立表达：
 *
 *   - **早期版本曾经让 dirty 保持真**，理由是「怕操作者以为已经完全
 *     生效」。但真机反馈（P5-1）显示这弄巧成拙：用户看到的是「已保存到
 *     数据库，但重载失败」的黄条**同时**挂着右上角「有未保存的改动」，
 *     两个信号叠在一起读起来像是保存本身失败了、数据没进库——而实际上
 *     数据已经落库，只是没生效。离开页面时还会为一份已经保存好的草稿
 *     弹出「确定要离开吗」，进一步坐实这种误导。
 *   - 现在的取舍：PUT 成功就 `markSaved()`，`dirty` 归假；「还没生效」
 *     这件事**只**由 `partialFailureMessage` 表达——页面据此渲染一条
 *     独立的持久提示，不依赖 SaveBar 的 dirty 标签。
 */

export interface UseDraftOptions {
  /** 当前绑定 id 的取值函数；未选中直播间时返回 null，save() 直接跳过。 */
  bindingId: () => number | null
  /** 计算当前草稿状态的序列化快照，用于跟「已保存基线」比较得出 dirty。 */
  snapshot: () => string
  /**
   * 判断一条从后端 GET 回来的规则（按 name）是否「归本页管」。
   *
   * 归本页管的规则在合并时会被 `buildRules()` 组装出的新版本整条替换；
   * 不归本页管的原样保留，避免整组替换误删别的页面管理的规则。
   */
  isOwned: (name: string) => boolean
  /** 把当前草稿组装成本页要写回的规则数组。 */
  buildRules: () => Rule[]
}

export interface UseDraftReturn {
  dirty: ComputedRef<boolean>
  saving: Ref<boolean>
  /**
   * 第 2 步（POST reload）失败后的提示文案，形如「重载失败，仍在用
   * 上一份配置运行: ...」——原样来自后端，别自己改写。
   * 还没保存过、或者最近一次保存两步都成功时为 null。
   */
  partialFailureMessage: Ref<string | null>
  /** 把当前草稿视为「已保存的基线」。loadRules() 加载完成后调用，重置 dirty 判定起点。 */
  markSaved: () => void
  /**
   * 保存：GET 现有规则 → 合并 → PUT → POST reload。
   *
   * 任一步失败都会把异常原样抛出去，调用方自己 catch 并展示
   * `e instanceof ApiError ? e.message : ...`——错误文案的呈现是页面的事，
   * 这里只负责流程与状态。
   */
  save: () => Promise<void>
}

/**
 * toWritableRule 把 GET 回来的 RuleView 转成能直接 PUT 回去的 Rule。
 *
 * **不能直接 `{ ...view }` 整个透传。** RuleView 比 Rule 多一个 `position`
 * 字段，PUT /api/bindings/{id}/rules 的处理器用了
 * `json.NewDecoder(...).DisallowUnknownFields()`（server/internal/httpapi/
 * rule_handler.go），带着 position 发过去会被后端当成「不认识的字段」
 * 直接 422 拒收。逐字段挑选而不是「解构剩余 + 丢一个」，是为了不管
 * RuleView 未来新增什么字段，这里都不会意外透传。
 *
 * **反过来的坑：这个名单本身也可能漏字段。** `spec.Rule` 新增字段时要
 * 走过四层——`spec` → `convert` → `rules`（引擎侧）→ 这里的白名单——
 * 前三层漏了字段编译期/校验期就会报错，唯独这里漏了不会有任何提示：
 * 编译通过、校验通过，只是保存时悄悄把这个字段丢了（`suppress` 就是
 * 这么漏掉的，见 useDraft.test.ts 里那条"合并回填不能把 suppress 丢掉"
 * 的用例）。新增字段时记得回来加一行。
 */
function toWritableRule(view: RuleView): Rule {
  const rule: Rule = { name: view.name, enabled: view.enabled }
  if (view.on !== undefined) rule.on = view.on
  if (view.schedule !== undefined) rule.schedule = view.schedule
  if (view.when !== undefined) rule.when = view.when
  if (view.aggregate !== undefined) rule.aggregate = view.aggregate
  if (view.cooldown !== undefined) rule.cooldown = view.cooldown
  if (view.cooldownGroup !== undefined) rule.cooldownGroup = view.cooldownGroup
  if (view.do !== undefined) rule.do = view.do
  if (view.suppress !== undefined) rule.suppress = view.suppress
  return rule
}

/** 应用内路由跳转时的确认文案——这条能自定义，因为是自己弹的确认框，不是 beforeunload。 */
const LEAVE_CONFIRM_MESSAGE = '有未保存的改动，确定要离开这个页面吗？'

export function useDraft(options: UseDraftOptions): UseDraftReturn {
  const baseline = ref(options.snapshot())
  const dirty = computed(() => options.snapshot() !== baseline.value)
  const saving = ref(false)
  const partialFailureMessage = ref<string | null>(null)

  function markSaved() {
    baseline.value = options.snapshot()
  }

  async function save() {
    const id = options.bindingId()
    if (id === null) return

    saving.value = true
    try {
      // 第 0/1 步：GET 现有规则、挑出不归本页管的部分。这一步失败或下面
      // PUT 失败，库和引擎都没被动过，不设置 partialFailureMessage。
      const existing = await request<RuleView[]>('GET', `/api/bindings/${id}/rules`)
      const kept = existing.filter((r) => !options.isOwned(r.name ?? '')).map(toWritableRule)
      const merged = [...kept, ...options.buildRules()]
      await request('PUT', `/api/bindings/${id}/rules`, merged)

      // PUT 一成功，草稿就已经等于数据库——dirty 该在这里归假，不必等
      // reload 的结果。见文件头「第 2 步失败时 dirty 照样归假」的说明。
      markSaved()

      // 第 2 步：reload。失败时库已经改了、引擎还没变，这里只负责记录
      // 提示文案再原样抛出，供页面单独渲染「已保存但未生效」的提示——
      // 这与上面的 markSaved 是两件独立的事。
      try {
        await request('POST', `/api/bindings/${id}/reload`)
      } catch (e) {
        partialFailureMessage.value = e instanceof ApiError ? e.message : String(e)
        throw e
      }

      partialFailureMessage.value = null
    } finally {
      saving.value = false
    }
  }

  // 离开拦截 1/2：应用内路由跳转。**必须在 setup 阶段同步调用**——
  // vue-router 靠 inject 拿当前组件对应的路由记录才能把守卫挂上去，
  // 放进 save() 的 .then 或任何异步回调里都不生效，且不会报错提示你哪里错了。
  onBeforeRouteLeave(() => {
    if (!dirty.value) return true
    return window.confirm(LEAVE_CONFIRM_MESSAGE)
  })

  // 离开拦截 2/2：整页刷新或关闭标签页。现代浏览器出于防钓鱼考虑，
  // beforeunload 不允许自定义确认框文案，只要 preventDefault 触发
  // 浏览器自己那句通用提示即可——传什么字符串给 returnValue 都不会显示，
  // 白费功夫也没有副作用。
  function handleBeforeUnload(e: BeforeUnloadEvent) {
    if (!dirty.value) return
    e.preventDefault()
    e.returnValue = ''
  }
  window.addEventListener('beforeunload', handleBeforeUnload)
  onUnmounted(() => window.removeEventListener('beforeunload', handleBeforeUnload))

  return { dirty, saving, partialFailureMessage, markSaved, save }
}
