// 后端接口的类型定义，手写维护。
//
// 不做代码生成：后端是 Go，生成 TS 要引入 OpenAPI 或 tygo 这类工具链，
// 而接口总共三十来个、字段都很浅，维护一条生成管线比手写贵。
//
// **代价是后端改字段时要手动同步这里。** 改后端的人不会自动想到改这里，
// 所以两处不一致的表现是运行期 undefined，而不是编译错误。
// 后端各个 *View 结构体（accountView / bindingView / ruleView /
// memberView / blockedView / activityView）是权威，以它们为准。

/** 六个权限点。与后端 internal/perm 的常量一一对应。 */
export type Permission =
  'rule:read' | 'rule:write' | 'danmaku:send' | 'user:block' | 'member:manage' | 'event:read'

export interface User {
  id: number
  username: string
  isAdmin: boolean
  createdAt: string
}

/** 当前用户的一条授权。`GET /api/auth/me` 返回它们的数组。 */
export interface Membership {
  bindingId: number
  accountName: string
  roomId: string
  permissions: Permission[]
}

/**
 * `GET /api/auth/me` 的响应。
 *
 * **注意它不是裸的 User。** 后端返回 `{user, memberships}` 两个字段，
 * 直接当成 User 用的话 `username` 与 `isAdmin` 都会是 undefined——
 * 界面看起来登录成功但用户名空着、管理员的管理入口消失，而且不报任何错。
 */
export interface MeResponse {
  user: User
  memberships: Membership[]
}

/**
 * 账号登录态的三个取值。与后端 `store.LoginState*` 常量一一对应。
 *
 * **`unknown` 不等于失效**：网络不通、探测本身出错时也会落在这一档，
 * 与「valid」「invalid」并列的第三态，不是「invalid 的弱化版」。把它当作
 * 「已失效」显示会在断网时把用户吓得去重新扫码——账号可能什么问题都没有。
 */
export type LoginState = 'valid' | 'invalid' | 'unknown'

export interface Account {
  id: number
  name: string
  uid: string
  rateLimitMs: number
  maxLength: number
  ownerId: number
  /**
   * 调用者是不是这个账号的所有者（管理员也为 true）。
   *
   * 前端靠它决定显不显示编辑与删除按钮。**不要拿 ownerId 去比**——
   * 账号级操作的判定在后端是「所有者或管理员」，前端重算一遍必然漂。
   */
  isOwner: boolean
  createdAt: string
  // 注意：后端刻意不返回 cookie 字段。前端不该有任何地方去接它。

  /** 最近一次登录态检测的结果，由后端每 10 分钟的检测循环写入。 */
  loginState: LoginState
  /**
   * 最近一次检测发生的时间。`null` 表示这个账号从未被检测过——
   * 刚建号、或后端进程刚启动、检测循环的首次探测还没跑到它。
   */
  loginCheckedAt: string | null
}

export interface Binding {
  id: number
  accountId: number
  accountName: string
  roomId: string
  enabled: boolean
  ruleCount: number
  /** 调用者在这个绑定上拥有的权限点，前端据此决定显示哪些按钮。 */
  permissions: Permission[]
}

export interface BlockedUser {
  id: number
  uid: string
  username: string
  reason: string
  createdBy: number | null
  createdAt: string
}

export type ActivityKind = 'event' | 'action'

export interface Activity {
  id: number
  kind: ActivityKind
  eventType: string
  actionType: string
  ruleName: string
  userUid: string
  userName: string
  detail: unknown
  occurredAt: string
}

export interface Member {
  username: string
  permissions: Permission[]
}

/** 统计聚合的维度：按日 / 按场次。与后端 `store.StatsBy*` 常量一一对应。 */
export type StatsDimension = 'day' | 'session'

/**
 * `GET /api/bindings/{id}/stats` 返回的一行聚合统计。
 *
 * 字段名照抄后端 `httpapi.statsView`（`stats_handler.go`），逐字段核对过。
 *
 * **`liveSeconds` 有历史数据缺口**：`live_start`/`live_stop` 是这次才加进
 * `logging/sink.go` 的入库白名单的，在此之前的数据里没有这两类事件，
 * 更早的直播时长永远算不出来、会是 0——不代表那天没有开播。
 */
export interface StatsBucket {
  /** 分桶标识：`by=day` 时是日期（如 `2026-08-01`），`by=session` 时是场次标识。 */
  bucket: string
  danmakuCount: number
  enterCount: number
  giftCount: number
  giftKinds: number
  guardCount: number
  liveSeconds: number
}
