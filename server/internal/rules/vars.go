package rules

import (
	"strings"

	"github.com/MrZhongzq/MagicalDanmaku/server/internal/event"
)

// VarsFromEvent 把事件展开为条件求值与模板渲染共用的取值表。
//
// 这是全项目唯一的字段展开处。条件里写 "user.guardLevel"、模板里写
// "{{.user.guardLevel}}"，两者指向同一份数据，杜绝两套字段名各自演化。
//
// 约定：值为零值的可选字段（如未佩戴勋章）不写入表中，
// 使 LookupPath 能区分「字段不存在」与「字段值为零」。
func VarsFromEvent(ev event.Event) map[string]any {
	v := map[string]any{
		"type":      string(ev.Type),
		"roomId":    ev.RoomID,
		"timestamp": ev.Timestamp.Unix(),
	}

	switch p := ev.Payload.(type) {
	case event.Danmaku:
		v["user"] = userVars(p.User)
		v["text"] = p.Text
		v["danmaku"] = map[string]any{
			"color":      p.Color,
			"isEmoticon": p.IsEmoticon,
			"replyToUid": p.ReplyToUID,
		}
	case event.SuperChat:
		v["user"] = userVars(p.User)
		v["text"] = p.Text
		v["superChat"] = map[string]any{
			"id": p.ID, "price": p.Price, "duration": int64(p.Duration),
		}
	case event.Gift:
		v["user"] = userVars(p.User)
		v["gift"] = map[string]any{
			"id": p.GiftID, "name": p.GiftName, "count": p.Count,
			"coinType": p.CoinType, "totalCoin": p.TotalCoin, "action": p.Action,
		}
	case event.GiftCombo:
		v["user"] = userVars(p.User)
		v["gift"] = map[string]any{
			"id": p.GiftID, "name": p.GiftName, "count": p.Count,
			"totalCoin": p.TotalCoin, "comboId": p.ComboID,
		}
	case event.GuardBuy:
		v["user"] = userVars(p.User)
		v["guard"] = map[string]any{
			"level": int64(p.GuardLevel), "name": p.GuardName,
			"count": int64(p.Count), "price": p.Price, "isRenew": p.IsRenew,
		}
	case event.UserEnter:
		v["user"] = userVars(p.User)
	case event.UserFollow:
		v["user"] = userVars(p.User)
	case event.UserShare:
		v["user"] = userVars(p.User)
	case event.UserLike:
		v["user"] = userVars(p.User)
	case event.UserBlocked:
		v["user"] = userVars(p.User)
	case event.RoomChange:
		v["room"] = map[string]any{
			"title": p.Title, "areaName": p.AreaName, "parentAreaName": p.ParentAreaName,
		}
	case event.RoomStatsUpdate:
		stats := map[string]any{}
		if p.Fans != nil {
			stats["fans"] = *p.Fans
		}
		if p.FansClub != nil {
			stats["fansClub"] = *p.FansClub
		}
		if p.Watched != nil {
			stats["watched"] = *p.Watched
		}
		if p.LikeCount != nil {
			stats["likeCount"] = *p.LikeCount
		}
		v["stats"] = stats
	case event.OnlineRankUpdate:
		v["rank"] = map[string]any{"count": int64(p.Count)}
	case event.Battle:
		v["battle"] = map[string]any{"subCommand": p.SubCommand}
	case event.Unknown:
		v["unknown"] = map[string]any{"command": p.Command}
	}
	return v
}

// userVars 展开用户信息。零值的可选字段不写入。
func userVars(u event.User) map[string]any {
	m := map[string]any{
		"uid":         u.UID,
		"username":    u.Username,
		"guardLevel":  u.GuardLevel,
		"userLevel":   u.UserLevel,
		"wealthLevel": u.WealthLevel,
		"isAdmin":     u.IsAdmin,
	}
	if u.AvatarURL != "" {
		m["avatarUrl"] = u.AvatarURL
	}
	if u.Medal != nil {
		m["medal"] = map[string]any{
			"name":       u.Medal.Name,
			"level":      u.Medal.Level,
			"roomId":     u.Medal.RoomID,
			"anchorUid":  u.Medal.AnchorUID,
			"guardLevel": u.Medal.GuardLevel,
			"isLighted":  u.Medal.IsLighted,
		}
	}
	return m
}

// Variable 是一条可用于条件与模板的变量。
// json tag 不能省：这个结构体会经 /api/meta/variables 直接吐给前端，
// 不带 tag 的话字段名是大写的 Path/Label/Optional，与其余 /api/meta/*
// 接口（metaItem 用的是 value/label）风格不一致，前端只能跟着迁就。
type Variable struct {
	Path     string `json:"path"`     // 点分路径，如 "user.medal.isLighted"，与 LookupPath 的参数同形
	Label    string `json:"label"`    // 中文说明，供前端下拉框展示
	Optional bool   `json:"optional"` // 可能不存在（如未佩戴粉丝牌时没有 medal.*），配条件时仍可选用
}

// commonVariables 是任意事件都会产出的公共字段。
//
// count/users/gifts 是例外：它们不是 VarsFromEvent 本身产出的，而是
// 合并窗口（见 aggregate.go 的 mergeBuckets/PassthroughTrigger）算完之后
// 才补进 Vars 的，所以标 Optional——用真实事件跑 VarsFromEvent 永远看
// 不到它们，但它们是配置聚合规则、写礼物答谢模板时用户真实用得到的路径。
//
// **这是 TestVariableCatalogMatchesVarsFromEvent 的天然盲区**：那条测试
// 只跟 VarsFromEvent 的实际产出对照，而聚合期变量本来就不在那边产出，
// 全靠这里人工标 Optional 混过去——加一个聚合期变量却忘了在这里补一条，
// 测试不会报红，只会在 /api/meta/variables 和条件构建器下拉里悄悄
// 少一项（gifts 就是这么漏掉的，直到全批次终审才补上）。新增聚合期
// 变量时记得回来加一行，并检查 mergeBuckets/PassthroughTrigger 两处
// 是否都填了它。
var commonVariables = []Variable{
	{Path: "type", Label: "事件类型"},
	{Path: "roomId", Label: "直播间号"},
	{Path: "timestamp", Label: "事件时间戳（Unix 秒）"},
	{Path: "count", Label: "合并窗口内的事件数量（仅聚合规则触发时存在）", Optional: true},
	{Path: "users", Label: "合并窗口内涉及的用户昵称列表（仅聚合规则触发时存在）", Optional: true},
	{Path: "gifts", Label: "合并窗口内涉及的礼物名列表，去重（仅聚合规则触发时存在）", Optional: true},
}

// userVariables 是 userVars 展开产出的字段，路径不带前缀——
// 使用处按事件类型套上 "user." 前缀（见 withPrefix）。
//
// 这份清单必须和 userVars 函数的实现同步：那边加一个字段，这边就要
// 加一条，否则要么用户配不出用到新字段的条件，要么清单里有一条
// VarsFromEvent 从不产出的死路径。TestVariableCatalogMatchesVarsFromEvent
// 会在两者漂开时报红。
var userVariables = []Variable{
	{Path: "uid", Label: "用户 UID"},
	{Path: "username", Label: "用户昵称"},
	{Path: "guardLevel", Label: "大航海等级（0 无/1 总督/2 提督/3 舰长）"},
	{Path: "userLevel", Label: "用户等级（UL）"},
	{Path: "wealthLevel", Label: "荣耀等级"},
	{Path: "isAdmin", Label: "是否房管"},
	{Path: "avatarUrl", Label: "头像地址", Optional: true},
	{Path: "medal.name", Label: "粉丝勋章名称", Optional: true},
	{Path: "medal.level", Label: "粉丝勋章等级", Optional: true},
	{Path: "medal.roomId", Label: "粉丝勋章所属直播间号", Optional: true},
	{Path: "medal.anchorUid", Label: "粉丝勋章所属主播 UID", Optional: true},
	{Path: "medal.guardLevel", Label: "粉丝勋章对应的大航海等级", Optional: true},
	{Path: "medal.isLighted", Label: "粉丝勋章是否点亮", Optional: true},
}

// withPrefix 给一组变量的 Path 统一加前缀，返回新切片（不改原切片）。
func withPrefix(prefix string, vars []Variable) []Variable {
	out := make([]Variable, len(vars))
	for i, v := range vars {
		out[i] = Variable{Path: prefix + "." + v.Path, Label: v.Label, Optional: v.Optional}
	}
	return out
}

// VariableCatalog 返回按事件类型分组的变量清单，供前端条件构建器/模板
// 编辑器渲染下拉框用。
//
// **它与 VarsFromEvent 必须一起改。** 前端的条件构建器靠它渲染下拉，
// 清单漏了某个路径，用户就配不出用到那个路径的条件；清单里有而
// VarsFromEvent 不产出，用户会配出永远不匹配且不报错的条件。
// TestVariableCatalogMatchesVarsFromEvent 用真实事件跑 VarsFromEvent，
// 把实际产出的键路径与这里声明的逐条对照，两边漂开就会红。
//
// common 是任意事件类型都有的字段（type/roomId/timestamp，以及只在
// 合并窗口聚合后才有的 count/users），不在 byEvent 的各分组里重复。
// byEvent 只覆盖 VarsFromEvent 的 switch 里实际有分支、会产出额外字段
// 的事件类型；像 live_start/live_stop/super_chat_delete/manual 这些
// switch 里没有分支的类型不出现在 byEvent 里——它们除了公共字段之外
// 没有别的变量可选。
func VariableCatalog() (common []Variable, byEvent map[event.Type][]Variable) {
	user := withPrefix("user", userVariables)

	byEvent = map[event.Type][]Variable{
		event.TypeDanmaku: append(append([]Variable{}, user...), []Variable{
			{Path: "text", Label: "弹幕正文"},
			{Path: "danmaku.color", Label: "弹幕颜色（十六进制，如 #ffffff）"},
			{Path: "danmaku.isEmoticon", Label: "是否为表情弹幕"},
			{Path: "danmaku.replyToUid", Label: "被 @ 回复的用户 UID"},
		}...),
		event.TypeSuperChat: append(append([]Variable{}, user...), []Variable{
			{Path: "text", Label: "醒目留言正文"},
			{Path: "superChat.id", Label: "醒目留言 ID"},
			{Path: "superChat.price", Label: "价格（元）"},
			{Path: "superChat.duration", Label: "展示秒数"},
		}...),
		event.TypeGift: append(append([]Variable{}, user...), []Variable{
			{Path: "gift.id", Label: "礼物 ID"},
			{Path: "gift.name", Label: "礼物名称"},
			{Path: "gift.count", Label: "礼物数量"},
			{Path: "gift.coinType", Label: "瓜子类型（gold 金瓜子 / silver 银瓜子）"},
			{Path: "gift.totalCoin", Label: "总价值（瓜子）"},
			{Path: "gift.action", Label: "动作描述（如「投喂」）"},
		}...),
		event.TypeGiftCombo: append(append([]Variable{}, user...), []Variable{
			{Path: "gift.id", Label: "礼物 ID"},
			{Path: "gift.name", Label: "礼物名称"},
			{Path: "gift.count", Label: "礼物数量（连击汇总）"},
			{Path: "gift.totalCoin", Label: "总价值（瓜子）"},
			{Path: "gift.comboId", Label: "连击 ID"},
		}...),
		event.TypeGuardBuy: append(append([]Variable{}, user...), []Variable{
			{Path: "guard.level", Label: "大航海等级（1 总督/2 提督/3 舰长）"},
			{Path: "guard.name", Label: "大航海名称（如「舰长」）"},
			{Path: "guard.count", Label: "购买月数"},
			{Path: "guard.price", Label: "单价（金瓜子）"},
			{Path: "guard.isRenew", Label: "是否为续费（false 为新购）"},
		}...),
		event.TypeUserEnter:   append([]Variable{}, user...),
		event.TypeUserFollow:  append([]Variable{}, user...),
		event.TypeUserShare:   append([]Variable{}, user...),
		event.TypeUserLike:    append([]Variable{}, user...),
		event.TypeUserBlocked: append([]Variable{}, user...),
		event.TypeRoomChange: {
			{Path: "room.title", Label: "直播间标题"},
			{Path: "room.areaName", Label: "分区名称"},
			{Path: "room.parentAreaName", Label: "父分区名称"},
		},
		event.TypeRoomStatsUpdate: {
			{Path: "stats.fans", Label: "粉丝数", Optional: true},
			{Path: "stats.fansClub", Label: "粉丝团人数", Optional: true},
			{Path: "stats.watched", Label: "累计看过人数", Optional: true},
			{Path: "stats.likeCount", Label: "点赞数", Optional: true},
		},
		event.TypeOnlineRankUpdate: {
			{Path: "rank.count", Label: "高能榜总人数（未知时为 -1）"},
		},
		event.TypeBattle: {
			{Path: "battle.subCommand", Label: "PK 原始 CMD 名（P0 只归一化不解释）"},
		},
		event.TypeUnknown: {
			{Path: "unknown.command", Label: "未识别事件的原始 CMD 名"},
		},
	}
	return commonVariables, byEvent
}

// LookupPath 按点分路径取值，如 "user.medal.level"。
// 路径不存在时返回 (nil, false)。
func LookupPath(vars map[string]any, path string) (any, bool) {
	if path == "" {
		return nil, false
	}
	parts := strings.Split(path, ".")

	var cur any = vars
	for _, p := range parts {
		m, ok := cur.(map[string]any)
		if !ok {
			return nil, false
		}
		cur, ok = m[p]
		if !ok {
			return nil, false
		}
	}
	return cur, true
}

// MergeVars 把 src 逐字段合并进 dst。
//
// 合并规则：dst 中缺失的键直接补上；已存在的键，只有当 dst 的值为
// 零值而 src 非零值时才覆盖。嵌套的 map 递归合并。
//
// 这条规则解决了 P0 联调发现的进场重复问题：ENTRY_EFFECT 只有 UID
// 没有昵称，INTERACT_WORD_V2 信息完整，两者合并后得到完整记录。
func MergeVars(dst, src map[string]any) {
	for k, sv := range src {
		dv, exists := dst[k]
		if !exists {
			dst[k] = sv
			continue
		}
		// 嵌套 map 递归合并
		if dm, ok := dv.(map[string]any); ok {
			if sm, ok := sv.(map[string]any); ok {
				MergeVars(dm, sm)
				continue
			}
		}
		if isZeroValue(dv) && !isZeroValue(sv) {
			dst[k] = sv
		}
	}
}

// isZeroValue 判断是否为「空」值：空串、0、false、nil。
func isZeroValue(v any) bool {
	switch t := v.(type) {
	case nil:
		return true
	case string:
		return t == ""
	case bool:
		return !t
	case int:
		return t == 0
	case int64:
		return t == 0
	case float64:
		return t == 0
	default:
		return false
	}
}
