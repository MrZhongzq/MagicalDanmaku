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
