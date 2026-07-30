package cmdmap

import "github.com/MrZhongzq/MagicalDanmaku/server/internal/event"

// parseMedalInfo 解析对象形式的勋章信息。
//
// B 站在不同 CMD 中用不同键名承载同一份数据：SEND_GIFT/SUPER_CHAT 用
// medal_info，INTERACT_WORD/GUARD_BUY 用 fans_medal，字段名基本一致。
// medal_level 为 0 表示用户未佩戴勋章，此时返回 nil。
func parseMedalInfo(m map[string]any) *event.Medal {
	if m == nil {
		return nil
	}
	level := int(getInt64(m, "medal_level"))
	if level == 0 {
		return nil
	}
	return &event.Medal{
		Name:       getString(m, "medal_name"),
		Level:      level,
		AnchorUID:  getString(m, "target_id"),
		AnchorName: getString(m, "anchor_uname"),
		RoomID:     getString(m, "anchor_roomid"),
		GuardLevel: int(getInt64(m, "guard_level")),
		IsLighted:  getBool(m, "is_lighted"),
	}
}

// medalFrom 依次尝试 medal_info 与 fans_medal 两个键。
func medalFrom(data map[string]any) *event.Medal {
	if m := parseMedalInfo(getObject(data, "medal_info")); m != nil {
		return m
	}
	return parseMedalInfo(getObject(data, "fans_medal"))
}

// parseUinfo 解析 2023 年后新增的 uinfo 对象，返回头像与荣耀等级。
// 该对象在 INTERACT_WORD、LIKE_INFO_V3_CLICK 等 CMD 中承载用户扩展信息。
func parseUinfo(data map[string]any) (avatar string, wealthLevel int) {
	uinfo := getObject(data, "uinfo")
	if uinfo == nil {
		return "", 0
	}
	if base := getObject(uinfo, "base"); base != nil {
		avatar = getString(base, "face")
	}
	if wealth := getObject(uinfo, "wealth"); wealth != nil {
		wealthLevel = int(getInt64(wealth, "level"))
	}
	return avatar, wealthLevel
}
