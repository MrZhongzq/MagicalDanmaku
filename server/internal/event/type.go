// Package event 定义平台无关的归一化直播事件模型。
package event

// Platform 标识事件来源的直播平台。
type Platform string

// PlatformBilibili 表示哔哩哔哩直播。
const PlatformBilibili Platform = "bilibili"

// Type 是归一化后的事件类型。
// 87 个 B 站 CMD 收敛到这 21 种类型（含 PK 串门信号 TypeVisitFromOpponent/
// TypeVisitToOpponent，两者不是 B 站 CMD 归一化的产物，是 P4-4 新增的
// 派生信号类型，见下方注释）。
type Type string

// 全部归一化事件类型。
const (
	TypeDanmaku          Type = "danmaku"            // 弹幕
	TypeSuperChat        Type = "super_chat"         // 醒目留言
	TypeSuperChatDelete  Type = "super_chat_delete"  // 醒目留言被删除
	TypeGift             Type = "gift"               // 礼物
	TypeGiftCombo        Type = "gift_combo"         // 礼物连击
	TypeGuardBuy         Type = "guard_buy"          // 上舰
	TypeUserEnter        Type = "user_enter"         // 用户进场
	TypeUserFollow       Type = "user_follow"        // 用户关注
	TypeUserShare        Type = "user_share"         // 用户分享
	TypeUserLike         Type = "user_like"          // 用户点赞
	TypeLiveStart        Type = "live_start"         // 开播
	TypeLiveStop         Type = "live_stop"          // 下播
	TypeRoomChange       Type = "room_change"        // 房间信息变更
	TypeUserBlocked      Type = "user_blocked"       // 用户被禁言
	TypeOnlineRankUpdate Type = "online_rank_update" // 高能榜变化
	TypeRoomStatsUpdate  Type = "room_stats_update"  // 房间统计数据变化
	TypeBattle           Type = "battle"             // PK 大乱斗（P6 消费）
	TypeManual           Type = "manual"             // 操作者从 WebUI 手动触发
	TypeUnknown          Type = "unknown"            // 未识别的 CMD

	// TypeVisitFromOpponent 和 TypeVisitToOpponent 是 PK 期间的「串门」
	// 信号，两个方向语义相反，故意用两个独立的 Type 而不是一个「串门」
	// 事件配布尔字段区分方向——漏判一个布尔就会把警示播成欢迎，成本
	// 极低而后果尴尬，两个不同的 Type 让这类调用方写错的成本变成
	// switch/if 里明摆着的类型不匹配。
	TypeVisitFromOpponent Type = "pk_visit_from_opponent" // 对面的人跑来我方串门（欢迎语气）
	TypeVisitToOpponent   Type = "pk_visit_to_opponent"   // 我方观众跑去对面串门（警示语气）
)

// allTypes 按声明顺序排列，AllTypes 依赖这个顺序。
//
// 手工维护而非反射枚举——const 块本身就是这份枚举唯一的权威来源，
// 这里只是把它变成一份可供测试遍历、可供 /api/meta/event-types 之类的
// 下发接口逐条比对的切片。新增一个 Type 常量却忘了把它加进这里，
// TestEventAllTypesMatchesConstBlock（type_test.go）会失败——这正是
// httpapi/meta_handler.go 的 aggregateByLabels 曾经踩过的坑（新增枚举值
// 时白名单式断言"至少包含哪几个"抓不住漏登记，见该文件 Important-1 的
// 教训），这里从一开始就按强断言的写法来。
var allTypes = []Type{
	TypeDanmaku, TypeSuperChat, TypeSuperChatDelete, TypeGift, TypeGiftCombo,
	TypeGuardBuy, TypeUserEnter, TypeUserFollow, TypeUserShare, TypeUserLike,
	TypeLiveStart, TypeLiveStop, TypeRoomChange, TypeUserBlocked,
	TypeOnlineRankUpdate, TypeRoomStatsUpdate, TypeBattle, TypeManual, TypeUnknown,
	TypeVisitFromOpponent, TypeVisitToOpponent,
}

// AllTypes 返回全部归一化事件类型的副本。
func AllTypes() []Type {
	out := make([]Type, len(allTypes))
	copy(out, allTypes)
	return out
}
