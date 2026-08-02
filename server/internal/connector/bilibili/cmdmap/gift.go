package cmdmap

import (
	"encoding/json"
	"fmt"

	"github.com/MrZhongzq/MagicalDanmaku/server/internal/event"
)

func init() {
	Register("SEND_GIFT", mapSendGift)
	Register("COMBO_SEND", mapComboSend)
}

// unmarshalData 提取消息的 data 对象。多数 CMD 的业务字段都在这一层。
func unmarshalData(raw json.RawMessage, cmdName string) (map[string]any, error) {
	var msg struct {
		Data map[string]any `json:"data"`
	}
	if err := json.Unmarshal(raw, &msg); err != nil {
		return nil, fmt.Errorf("cmdmap: %s 解析失败: %w", cmdName, err)
	}
	if msg.Data == nil {
		return nil, fmt.Errorf("cmdmap: %s 缺少 data 字段", cmdName)
	}
	return msg.Data, nil
}

// mapSendGift 解析送礼消息。
func mapSendGift(ctx Context, raw json.RawMessage) ([]event.Event, error) {
	data, err := unmarshalData(raw, "SEND_GIFT")
	if err != nil {
		return nil, err
	}

	g := event.Gift{
		User: event.User{
			UID:        getString(data, "uid"),
			Username:   getString(data, "uname"),
			AvatarURL:  getString(data, "face"),
			GuardLevel: int(getInt64(data, "guard_level")),
			Medal:      medalFrom(data),
		},
		GiftID:    getInt64(data, "giftId"),
		GiftName:  getString(data, "giftName"),
		Count:     getInt64(data, "num"),
		CoinType:  getString(data, "coin_type"),
		TotalCoin: getInt64(data, "total_coin"),
		Price:     getInt64(data, "price"),
		Action:    getString(data, "action"),
		BlindBox:  blindBoxFrom(data),
	}

	ts := timeFromUnixSec(getInt64(data, "timestamp"))
	return []event.Event{NewEvent(ctx, event.TypeGift, ts, g, raw)}, nil
}

// blindBoxFrom 解析 blind_gift 字段。该字段为 null 表示这不是盲盒。
func blindBoxFrom(data map[string]any) *event.BlindBox {
	bg := getObject(data, "blind_gift")
	if bg == nil {
		return nil
	}
	return &event.BlindBox{
		Name:     getString(bg, "original_gift_name"),
		GiftID:   getInt64(bg, "original_gift_id"),
		Price:    getInt64(bg, "original_gift_price"),
		TipPrice: getInt64(bg, "gift_tip_price"),
	}
}

// mapComboSend 解析礼物连击汇总消息。
//
// 注意：COMBO_SEND 与其对应的多条 SEND_GIFT 是重复计数关系，
// 二者的合并去重属于 P2 规则引擎的职责，P0 只如实投递两种事件。
func mapComboSend(ctx Context, raw json.RawMessage) ([]event.Event, error) {
	data, err := unmarshalData(raw, "COMBO_SEND")
	if err != nil {
		return nil, err
	}

	count := getInt64(data, "combo_num")
	if count == 0 {
		count = getInt64(data, "total_num")
	}

	c := event.GiftCombo{
		User: event.User{
			UID:        getString(data, "uid"),
			Username:   getString(data, "uname"),
			GuardLevel: int(getInt64(data, "guard_level")),
			Medal:      medalFrom(data),
		},
		GiftID:    getInt64(data, "gift_id"),
		GiftName:  getString(data, "gift_name"),
		Count:     count,
		ComboID:   getString(data, "batch_combo_id"),
		TotalCoin: getInt64(data, "combo_total_coin"),
	}

	return []event.Event{NewEvent(ctx, event.TypeGiftCombo, ctx.ReceivedAt, c, raw)}, nil
}
