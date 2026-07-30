package cmdmap

import (
	"encoding/json"

	"github.com/MrZhongzq/MagicalDanmaku/server/internal/event"
)

func init() {
	Register("GUARD_BUY", mapGuardBuy)
	Register("USER_TOAST_MSG", mapUserToastMsg)
}

// guardName 把大航海等级转成中文名。
func guardName(level int) string {
	switch level {
	case event.GuardGovernor:
		return "总督"
	case event.GuardAdmiral:
		return "提督"
	case event.GuardCaptain:
		return "舰长"
	default:
		return ""
	}
}

// mapGuardBuy 解析新购大航海消息。
func mapGuardBuy(ctx Context, raw json.RawMessage) ([]event.Event, error) {
	data, err := unmarshalData(raw, "GUARD_BUY")
	if err != nil {
		return nil, err
	}

	level := int(getInt64(data, "guard_level"))
	name := getString(data, "gift_name")
	if name == "" {
		name = guardName(level)
	}

	g := event.GuardBuy{
		User: event.User{
			UID:        getString(data, "uid"),
			Username:   getString(data, "username"),
			GuardLevel: level,
			Medal:      medalFrom(data),
		},
		GuardLevel: level,
		GuardName:  name,
		Count:      int(getInt64(data, "num")),
		Price:      getInt64(data, "price"),
		IsRenew:    false,
	}

	ts := timeFromUnixSec(getInt64(data, "start_time"))
	return []event.Event{NewEvent(ctx, event.TypeGuardBuy, ts, g, raw)}, nil
}

// mapUserToastMsg 解析大航海续费消息。
//
// USER_TOAST_MSG 与 GUARD_BUY 描述同一类业务动作，归一化为同一事件类型，
// 用 IsRenew 区分新购与续费。
func mapUserToastMsg(ctx Context, raw json.RawMessage) ([]event.Event, error) {
	data, err := unmarshalData(raw, "USER_TOAST_MSG")
	if err != nil {
		return nil, err
	}

	level := int(getInt64(data, "guard_level"))
	name := getString(data, "role_name")
	if name == "" {
		name = guardName(level)
	}

	g := event.GuardBuy{
		User: event.User{
			UID:        getString(data, "uid"),
			Username:   getString(data, "username"),
			GuardLevel: level,
			Medal:      medalFrom(data),
		},
		GuardLevel: level,
		GuardName:  name,
		Count:      int(getInt64(data, "num")),
		Price:      getInt64(data, "price"),
		IsRenew:    true,
	}

	ts := timeFromUnixSec(getInt64(data, "start_time"))
	return []event.Event{NewEvent(ctx, event.TypeGuardBuy, ts, g, raw)}, nil
}
