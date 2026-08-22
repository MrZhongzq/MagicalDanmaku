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
		// GUARD_BUY 没有任何能区分开通与续费的字段：2026-08-22 真机上让
		// 同一个账号先开通再续费，两次的 GUARD_BUY 逐字段一致，连 price
		// 都同样是 198000（续费实际扣 168000）。所以这里只能报「不知道」。
		IsRenew:    false,
		RenewKnown: false,
	}

	ts := timeFromUnixSec(getInt64(data, "start_time"))
	return []event.Event{NewEvent(ctx, event.TypeGuardBuy, ts, g, raw)}, nil
}

// toastOpTypeRenew 是 USER_TOAST_MSG.op_type 里代表「续费」的取值。
//
// 取值含义由真机成对抓包确定（2026-08-22，同一账号在同一房间先开通后
// 续费）：op_type=1 的那条 toast_msg 写的是「开通了舰长」，op_type=2 的
// 那条写的是「续费了舰长」。不按 toast_msg 的文案判，是因为那是给人看的
// 展示文本，B 站随时可以改措辞；op_type 是枚举。
const toastOpTypeRenew = 2

// mapUserToastMsg 解析大航海开通/续费消息。
//
// USER_TOAST_MSG 与 GUARD_BUY 描述的是**同一次购买**（不是「一个新购一个
// 续费」），归一化为同一事件类型。这里曾经把 IsRenew 写死成 true，前提是
// 「USER_TOAST_MSG 就是续费消息」——真机数据证伪了这个前提，开通时这条
// 照样会来。真正的判据是 op_type。
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
		IsRenew:    getInt64(data, "op_type") == toastOpTypeRenew,
		RenewKnown: true,
	}

	ts := timeFromUnixSec(getInt64(data, "start_time"))
	return []event.Event{NewEvent(ctx, event.TypeGuardBuy, ts, g, raw)}, nil
}
