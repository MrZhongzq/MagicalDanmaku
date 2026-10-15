package cmdmap

import (
	"encoding/json"

	"google.golang.org/protobuf/encoding/protowire"

	"github.com/MrZhongzq/MagicalDanmaku/server/internal/event"
)

func init() {
	Register("SEND_GIFT_V2", mapSendGiftV2)
}

// SEND_GIFT_V2（网页端类型 bilibili.live.gift.v1.SendGiftBroadcast）的
// protobuf 字段编号。
//
// B 站不公开 .proto，编号取自社区整理的定义（php-bilibili-danmu 的
// app/protobuf/SEND_GIFT_V2.proto，与 blivedm PR #86 讨论中的一致），
// 并逐个对照了 2026-10 在真实直播间抓到的报文。
const (
	sgUID        = 1
	sgUname      = 2
	sgFace       = 3
	sgGuardLevel = 5
	sgMedalInfo  = 8
	sgBlindGift  = 9
	sgGiftList   = 10 // repeated：盲盒爆出时一条广播带多份礼物
	sgSwitch     = 11
	sgWealthInfo = 13
	sgSenderInfo = 15

	// MedalInfo，与旧 JSON 的 medal_info 一一对应。
	smTargetID    = 1
	smAnchorUname = 3
	smAnchorRoom  = 4
	smLevel       = 5
	smName        = 6
	smIsLighted   = 11
	smGuardLevel  = 12

	// BlindGift
	sbOriginalID    = 2
	sbOriginalName  = 3
	sbOriginalPrice = 6
	sbTipPrice      = 7

	// GiftItem
	siGiftID    = 1
	siGiftName  = 2
	siNum       = 3
	siPrice     = 5
	siTotalCoin = 7
	siCoinType  = 8
	siTimestamp = 10
	siAction    = 18
	siTipPrice  = 36
)

// sendGiftV2 是解码后的广播里与礼物事件有关的部分。
type sendGiftV2 struct {
	User       event.User
	Shown      bool
	Blind      *event.BlindBox
	Gifts      []giftItemV2Data
	SenderName string
	SenderFace string
}

// giftItemV2Data 是 gift_list 里的一份礼物。
type giftItemV2Data struct {
	GiftID    int64
	GiftName  string
	Count     int64
	Price     int64
	TotalCoin int64
	CoinType  string
	Timestamp int64
	Action    string
	TipPrice  int64
}

// mapSendGiftV2 解析 protobuf 编码的送礼广播。
//
// B 站已停止下发 JSON 版 SEND_GIFT，送礼全部改由本 CMD 承载。每份礼物
// 产出一个与 SEND_GIFT 同构的 gift 事件，下游规则、统计无需感知版本。
func mapSendGiftV2(ctx Context, raw json.RawMessage) ([]event.Event, error) {
	pb, err := decodePBField(raw, "SEND_GIFT_V2")
	if err != nil {
		return nil, err
	}

	var sg sendGiftV2
	if err := decodeSendGiftV2(pb, &sg); err != nil {
		return nil, err
	}
	// 网页端在 switch 为 false 时不处理这条广播，这里保持一致。
	if !sg.Shown {
		return nil, nil
	}

	if sg.User.Username == "" {
		sg.User.Username = sg.SenderName
	}
	if sg.User.AvatarURL == "" {
		sg.User.AvatarURL = sg.SenderFace
	}

	evs := make([]event.Event, 0, len(sg.Gifts))
	for _, it := range sg.Gifts {
		g := event.Gift{
			User:      sg.User,
			GiftID:    it.GiftID,
			GiftName:  it.GiftName,
			Count:     it.Count,
			CoinType:  it.CoinType,
			TotalCoin: it.TotalCoin,
			Price:     it.Price,
			Action:    it.Action,
		}
		// 一条广播拆成多个事件时各持一份勋章，不共享指针。
		if sg.User.Medal != nil {
			m := *sg.User.Medal
			g.User.Medal = &m
		}
		if sg.Blind != nil {
			bb := *sg.Blind
			// 真实报文里 BlindGift.gift_tip_price 缺省，爆出价值在礼物条目
			// 自己的 gift_tip_price 上；两者都缺时回落到单价——旧版 SEND_GIFT
			// 里 price 与 gift_tip_price 恒等，event.BlindBox 也是这么约定的。
			if bb.TipPrice == 0 {
				bb.TipPrice = it.TipPrice
			}
			if bb.TipPrice == 0 {
				bb.TipPrice = it.Price
			}
			g.BlindBox = &bb
		}
		ts := timeFromUnixSec(it.Timestamp)
		evs = append(evs, NewEvent(ctx, event.TypeGift, ts, g, raw))
	}
	return evs, nil
}

// decodeSendGiftV2 解码顶层消息。
func decodeSendGiftV2(b []byte, out *sendGiftV2) error {
	return walkFields(b, func(num protowire.Number, _ protowire.Type, val []byte, v uint64) error {
		switch num {
		case sgUID:
			out.User.UID = formatUint(v)
		case sgUname:
			out.User.Username = string(val)
		case sgFace:
			out.User.AvatarURL = string(val)
		case sgGuardLevel:
			out.User.GuardLevel = int(v)
		case sgSwitch:
			out.Shown = v != 0
		case sgMedalInfo:
			m, err := decodeMedalInfoV2(val)
			out.User.Medal = m
			return err
		case sgBlindGift:
			bb, err := decodeBlindGiftV2(val)
			out.Blind = bb
			return err
		case sgGiftList:
			var it giftItemV2Data
			if err := decodeGiftItemV2(val, &it); err != nil {
				return err
			}
			out.Gifts = append(out.Gifts, it)
		case sgWealthInfo:
			return walkFields(val, func(n protowire.Number, _ protowire.Type, _ []byte, v uint64) error {
				if n == wiLevel {
					out.User.WealthLevel = int(v)
				}
				return nil
			})
		case sgSenderInfo:
			return walkFields(val, func(n protowire.Number, _ protowire.Type, val []byte, _ uint64) error {
				if n != uiBase {
					return nil
				}
				return walkFields(val, func(n protowire.Number, _ protowire.Type, val []byte, _ uint64) error {
					switch n {
					case biUname:
						out.SenderName = string(val)
					case biFace:
						out.SenderFace = string(val)
					}
					return nil
				})
			})
		}
		return nil
	})
}

// decodeMedalInfoV2 解码佩戴的勋章。等级为 0（含空消息）表示没戴，返回 nil，
// 与 parseMedalInfo 的约定一致。
func decodeMedalInfoV2(b []byte) (*event.Medal, error) {
	var m event.Medal
	err := walkFields(b, func(num protowire.Number, _ protowire.Type, val []byte, v uint64) error {
		switch num {
		case smTargetID:
			m.AnchorUID = formatUint(v)
		case smAnchorUname:
			m.AnchorName = string(val)
		case smAnchorRoom:
			m.RoomID = formatUint(v)
		case smLevel:
			m.Level = int(v)
		case smName:
			m.Name = string(val)
		case smIsLighted:
			m.IsLighted = v != 0
		case smGuardLevel:
			m.GuardLevel = int(v)
		}
		return nil
	})
	if err != nil || m.Level == 0 {
		return nil, err
	}
	return &m, nil
}

// decodeBlindGiftV2 解码盲盒信息。original_gift_id 为 0 表示不是盲盒。
func decodeBlindGiftV2(b []byte) (*event.BlindBox, error) {
	var bb event.BlindBox
	err := walkFields(b, func(num protowire.Number, _ protowire.Type, val []byte, v uint64) error {
		switch num {
		case sbOriginalID:
			bb.GiftID = int64(v)
		case sbOriginalName:
			bb.Name = string(val)
		case sbOriginalPrice:
			bb.Price = int64(v)
		case sbTipPrice:
			bb.TipPrice = int64(v)
		}
		return nil
	})
	if err != nil || bb.GiftID == 0 {
		return nil, err
	}
	return &bb, nil
}

// decodeGiftItemV2 解码 gift_list 里的一份礼物。
func decodeGiftItemV2(b []byte, out *giftItemV2Data) error {
	return walkFields(b, func(num protowire.Number, _ protowire.Type, val []byte, v uint64) error {
		switch num {
		case siGiftID:
			out.GiftID = int64(v)
		case siGiftName:
			out.GiftName = string(val)
		case siNum:
			out.Count = int64(v)
		case siPrice:
			out.Price = int64(v)
		case siTotalCoin:
			out.TotalCoin = int64(v)
		case siCoinType:
			out.CoinType = string(val)
		case siTimestamp:
			out.Timestamp = int64(v)
		case siAction:
			out.Action = string(val)
		case siTipPrice:
			out.TipPrice = int64(v)
		}
		return nil
	})
}
