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

/**
 * 直播间开播状态的三个取值。与后端 `store.RoomLive*` 常量一一对应，
 * 语义与 `LoginState` 完全对称。
 *
 * **`unknown` 不等于「未开播」**：探测本身失败（网络不通、被风控）也会
 * 落在这一档，与「living」「offline」并列的第三态。绝不能把它显示成
 * 「未开播」——那是在用一个看起来正常的错误答案骗用户，`RoomInfo`/
 * `RoomStatus` 探测失败与「确认没开播」必须在界面上分开表达。
 */
export type RoomLiveStatus = 'living' | 'offline' | 'unknown'

export interface Binding {
  id: number
  accountId: number
  accountName: string
  roomId: string
  enabled: boolean
  ruleCount: number
  /** 调用者在这个绑定上拥有的权限点，前端据此决定显示哪些按钮。 */
  permissions: Permission[]
  /**
   * 调用者是不是这个绑定所属账号的所有者（管理员也为 true）。
   *
   * **拉黑（P5-6）走这个字段，不走 permissions。** 拉黑是账号级操作，
   * 与直播间无关——房管持有 `user:block` 能代为禁言，但不能代表账号
   * 本人拉黑一个陌生人，这条边界与权限点体系是两条独立的轴。不要用
   * `permissions.includes('user:block')` 来判断能不能拉黑，那是禁言
   * 的权限，不是拉黑的。
   */
  isOwner: boolean

  /**
   * 最近一次直播间开播状态检测的结果，由后端 60 秒一轮的心跳循环，
   * 或者新增绑定时的立即检测写入。
   */
  liveStatus: RoomLiveStatus
  /** 最近一次检测发生的时间。`null` 表示这个绑定从未被检测过。 */
  liveCheckedAt: string | null
  /** 主播 UID——注意不是 `roomId`（房间号）。探测成功前为空串。 */
  anchorUid: string
  /** 主播昵称。探测成功前为空串；探测失败时保留上一次已知值。 */
  anchorName: string
}

/**
 * `GET /api/bindings/{id}/blacklist-status?uid=` 的响应。
 *
 * `blacklisted` 是 B 站 `x/space/wbi/acc/relation` 的 `attribute==128`
 * 换算结果（后端已经做了判定，前端不重新解释 attribute 原始值）。
 * `nickname` 是尽力而为的自动回填，查不到时是空串，不代表查询失败。
 */
export interface BlacklistStatus {
  uid: string
  blacklisted: boolean
  nickname: string
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
  /**
   * 本分桶内全部盲盒礼物的盈亏之和，单位 1/100 电池，可为负。按每一条
   * 盲盒送礼事件的原始电池数量累加，不按礼物名分组——同一电池消耗量
   * 在不同盲盒池可能对应不同的开出礼物，礼物名不是稳定的价值锚点。
   * 「元」的换算只在展示层做——**除以 1000，不是 100**（1 电池 = 0.1
   * 元，原始值是 1/100 电池），换算逻辑在 `Stats.vue` 的
   * `formatBlindBoxProfit`，必须跟 `server/internal/rules/aggregate.go`
   * 的 `formatYuan` 用同一个系数。后端一律给 1/100 电池的原始整数，
   * 见 `server/internal/store/stats.go` 的 `StatsBucket.BlindBoxProfit`。
   */
  blindBoxProfit: number
  /**
   * 本分桶内**主播实际收到**的电池总量，单位 1/100 电池，含盲盒爆出的
   * 礼物（用户原话："总计今天主播收到的电池数量"，不分来源）。
   *
   * **单行电池价值 = price × count，不是 total_coin**——两者含义不同：
   * `total_coin` 是送礼人这次投喂付出的电池（盲盒场景下是盲盒售价，
   * 与爆出什么无关，同一种盲盒不管开出什么都恒定）；`price` 是这条礼物
   * 本身的价值（盲盒场景下等于爆出礼物的价值），才是主播实际到账的钱。
   * 普通礼物（非盲盒）两者恒等，只有盲盒场景会分叉。
   *
   * 判据是"这条礼物的电池价值是不是 0"，不是"是不是金瓜子礼物"——后端
   * 用"排除法"：只有 `coin_type` 是 `silver`（银瓜子）记 0，其余（含
   * 盲盒）一律计入。「元」的换算只在展示层做，同 `blindBoxProfit` 一样
   * 除以 1000。
   */
  giftCoins: number
}

/**
 * `GET /api/bindings/{id}/gifts` 返回的一行礼物明细，按礼物名分组。
 *
 * **不含盲盒**——与 `giftCoins`（含盲盒）不同，这张明细表是"哪个礼物
 * 名贡献了多少"，盲盒爆出的礼物名不是稳定的价值锚点，混进来会让同一个
 * 名字在盲盒场景与非盲盒场景下代表完全不同的东西，见后端
 * `store.QueryGiftBreakdown` 的注释。这张表的 `coins` 之和因此不等于
 * `giftCoins`，不是 bug。
 *
 * 字段名照抄后端 `httpapi.giftBreakdownView`（`stats_handler.go`）。
 */
export interface GiftBreakdownRow {
  giftName: string
  /**
   * 为真表示这一行是**盲盒爆出的**礼物，不是观众直接投喂的。
   *
   * 同一个礼物名在两种来源下是**两行**（后端按 giftName + 是否盲盒分组），
   * 所以表格的行键必须是 `giftName + blindBox`，只用 giftName 会撞键。
   *
   * 盲盒行为什么要出现在这张表里：「当日电池到账」卡片含盲盒爆出的礼物
   * （那是主播的真实收入），明细如果排除盲盒，用户看到的两个数字就永远
   * 对不上（真机上是 423 电池 vs 只能加出 103）。
   */
  blindBox: boolean
  /** 这个礼物名下的送礼数量之和（礼物本身的数量，不是事件行数）。 */
  count: number
  /**
   * 这个礼物名下全部行的"电池价值"之和（price × count，不是
   * total_coin），单位 1/100 电池——判据见 `StatsBucket.giftCoins` 的
   * 注释（排除法，只排除银瓜子）。银瓜子礼物的数量仍然照常统计在
   * `count` 里，`coins` 里那部分贡献恒为 0，两者不是同一回事。
   */
  coins: number
}
