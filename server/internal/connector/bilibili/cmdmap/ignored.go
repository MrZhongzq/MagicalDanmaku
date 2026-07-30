package cmdmap

import (
	"encoding/json"

	"github.com/MrZhongzq/MagicalDanmaku/server/internal/event"
)

// ignoredCommands 是与弹幕机器人无关的高频噪声 CMD。
//
// 这是本项目中唯一允许丢弃消息的场景，因此必须逐条显式列举，
// 不得使用前缀或通配匹配——否则 B 站新增的有用 CMD 会被误吞。
// 未列出的 CMD 一律走 Unknown 兜底，仍会投递给上层。
var ignoredCommands = []string{
	"WIDGET_BANNER",                     // 活动横幅
	"ROOM_BANNER",                       // 房间横幅
	"ACTIVITY_BANNER_UPDATE_V2",         // 活动横幅更新
	"PANEL",                             // 面板数据
	"ONLINERANK",                        // 旧版高能榜，已被 ONLINE_RANK_V2 取代
	"STOP_LIVE_ROOM_LIST",               // 全站下播房间列表广播
	"NOTICE_MSG",                        // 全站通告广播
	"HOT_ROOM_NOTIFY",                   // 热门房间提示
	"WIDGET_GIFT_STAR_PROCESS",          // 礼物星球进度
	"LIVE_INTERACTIVE_GAME",             // 互动游戏内部消息
	"POPULARITY_RED_POCKET_START",       // 红包活动
	"POPULARITY_RED_POCKET_NEW",         //
	"POPULARITY_RED_POCKET_WINNER_LIST", //
	"AREA_RANK_CHANGED",                 // 分区排行变化
	"COMMON_NOTICE_DANMAKU",             // 系统通知弹幕
	"LOG_IN_NOTICE",                     // 登录提示
	"SPREAD_SHOW_FEET_V2",               // 推广位
	"RECOMMEND_CARD",                    // 推荐卡片
}

func init() {
	for _, name := range ignoredCommands {
		Register(name, mapIgnored)
	}
}

// mapIgnored 静默丢弃噪声消息。
func mapIgnored(_ Context, _ json.RawMessage) ([]event.Event, error) {
	return nil, nil
}
