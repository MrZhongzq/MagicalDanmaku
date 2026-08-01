// Package event 定义平台无关的归一化直播事件模型。
package event

// Platform 标识事件来源的直播平台。
type Platform string

// PlatformBilibili 表示哔哩哔哩直播。
const PlatformBilibili Platform = "bilibili"

// Type 是归一化后的事件类型。
// 87 个 B 站 CMD 收敛到这 18 种类型。
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
)
