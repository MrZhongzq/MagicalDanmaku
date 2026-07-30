package cmdmap

import (
	"encoding/base64"
	"encoding/json"
	"fmt"

	"google.golang.org/protobuf/encoding/protowire"

	"github.com/MrZhongzq/MagicalDanmaku/server/internal/event"
)

func init() {
	Register("INTERACT_WORD_V2", mapInteractWordV2)
}

// INTERACT_WORD_V2 的 protobuf 字段编号。
//
// B 站不公开 .proto，这里的编号是从原项目的 nanopb 生成头文件
// （services/live_services/bilibili/protobuf/interact_word_v2.pb.h）中提取的。
// 采用 protowire 手工解码而非 protoc 代码生成，避免引入额外工具链；
// 未知字段一律跳过，B 站新增字段不会导致解码失败。
const (
	iwUID        = 1
	iwUname      = 2
	iwMsgType    = 5
	iwRoomID     = 6
	iwTimestamp  = 7
	iwFansMedal  = 9
	iwGuardLevel = 16
	iwUserInfo   = 22

	fmTargetID = 1
	fmRoomID   = 12

	uiUID       = 1
	uiBase      = 2
	uiMedalInfo = 3
	uiWealth    = 4
	uiGuard     = 6

	biUname = 1
	biFace  = 2

	miMedalName  = 1
	miMedalLevel = 2
	miIsLighted  = 9
	miRUID       = 10
	miGuardLevel = 11

	wiLevel = 1

	giLevel = 1
)

// interactV2 是解码后的 INTERACT_WORD_V2 载荷。
type interactV2 struct {
	UID        uint64
	Uname      string
	MsgType    uint64
	Timestamp  uint64
	GuardLevel uint32

	MedalTargetID uint64
	MedalRoomID   uint32

	InfoUID     uint64
	Face        string
	InfoUname   string
	MedalName   string
	MedalLevel  uint32
	MedalRUID   uint64
	MedalGuard  uint32
	MedalLit    bool
	WealthLevel uint32
	GuardInfoLv uint32
}

// mapInteractWordV2 解析 protobuf 编码的互动消息。
//
// 自 2024 年起 B 站已停止下发 INTERACT_WORD v1，进场、关注、分享
// 三类事件全部改由本 CMD 承载，因此必须解码，不能走 Unknown 兜底。
func mapInteractWordV2(ctx Context, raw json.RawMessage) ([]event.Event, error) {
	data, err := unmarshalData(raw, "INTERACT_WORD_V2")
	if err != nil {
		return nil, err
	}

	b64 := getString(data, "pb")
	if b64 == "" {
		return nil, fmt.Errorf("cmdmap: INTERACT_WORD_V2 缺少 pb 字段")
	}
	pb, err := base64.StdEncoding.DecodeString(b64)
	if err != nil {
		return nil, fmt.Errorf("cmdmap: INTERACT_WORD_V2 的 pb 不是合法 base64: %w", err)
	}

	var iw interactV2
	if err := decodeInteractV2(pb, &iw); err != nil {
		return nil, err
	}

	u := event.User{
		UID:         formatUint(iw.UID),
		Username:    iw.Uname,
		AvatarURL:   iw.Face,
		WealthLevel: int(iw.WealthLevel),
	}
	// uinfo 里的信息更完整，优先采用。
	if iw.InfoUname != "" {
		u.Username = iw.InfoUname
	}
	if u.UID == "" || u.UID == "0" {
		u.UID = formatUint(iw.InfoUID)
	}
	// 大航海等级有三处来源，按可信度取用。
	switch {
	case iw.GuardInfoLv > 0:
		u.GuardLevel = int(iw.GuardInfoLv)
	case iw.GuardLevel > 0:
		u.GuardLevel = int(iw.GuardLevel)
	case iw.MedalGuard > 0:
		u.GuardLevel = int(iw.MedalGuard)
	}
	if iw.MedalLevel > 0 {
		u.Medal = &event.Medal{
			Name:       iw.MedalName,
			Level:      int(iw.MedalLevel),
			AnchorUID:  formatUint(iw.MedalRUID),
			RoomID:     formatUint(uint64(iw.MedalRoomID)),
			GuardLevel: int(iw.MedalGuard),
			IsLighted:  iw.MedalLit,
		}
		if u.Medal.AnchorUID == "" || u.Medal.AnchorUID == "0" {
			u.Medal.AnchorUID = formatUint(iw.MedalTargetID)
		}
	}

	var (
		typ event.Type
		p   event.Payload
	)
	switch iw.MsgType {
	case 2, 4: // 2 关注，4 特别关注
		typ, p = event.TypeUserFollow, event.UserFollow{User: u}
	case 3: // 分享直播间
		typ, p = event.TypeUserShare, event.UserShare{User: u}
	default: // 1 进入直播间，以及未知取值
		typ, p = event.TypeUserEnter, event.UserEnter{User: u}
	}

	ts := timeFromUnixSec(int64(iw.Timestamp))
	return []event.Event{NewEvent(ctx, typ, ts, p, raw)}, nil
}

// formatUint 把无符号整数转成字符串，0 返回空串。
func formatUint(v uint64) string {
	if v == 0 {
		return ""
	}
	return fmt.Sprintf("%d", v)
}

// decodeInteractV2 解码顶层消息。
func decodeInteractV2(b []byte, out *interactV2) error {
	return walkFields(b, func(num protowire.Number, typ protowire.Type, val []byte, v uint64) error {
		switch num {
		case iwUID:
			out.UID = v
		case iwUname:
			out.Uname = string(val)
		case iwMsgType:
			out.MsgType = v
		case iwTimestamp:
			out.Timestamp = v
		case iwGuardLevel:
			out.GuardLevel = uint32(v)
		case iwFansMedal:
			return decodeFansMedal(val, out)
		case iwUserInfo:
			return decodeUserInfo(val, out)
		}
		return nil
	})
}

// decodeFansMedal 解码 fans_medal 子消息。
func decodeFansMedal(b []byte, out *interactV2) error {
	return walkFields(b, func(num protowire.Number, typ protowire.Type, val []byte, v uint64) error {
		switch num {
		case fmTargetID:
			out.MedalTargetID = v
		case fmRoomID:
			out.MedalRoomID = uint32(v)
		}
		return nil
	})
}

// decodeUserInfo 解码 uinfo 子消息及其嵌套结构。
func decodeUserInfo(b []byte, out *interactV2) error {
	return walkFields(b, func(num protowire.Number, typ protowire.Type, val []byte, v uint64) error {
		switch num {
		case uiUID:
			out.InfoUID = v
		case uiBase:
			return walkFields(val, func(n protowire.Number, _ protowire.Type, val []byte, _ uint64) error {
				switch n {
				case biUname:
					out.InfoUname = string(val)
				case biFace:
					out.Face = string(val)
				}
				return nil
			})
		case uiMedalInfo:
			return walkFields(val, func(n protowire.Number, _ protowire.Type, val []byte, v uint64) error {
				switch n {
				case miMedalName:
					out.MedalName = string(val)
				case miMedalLevel:
					out.MedalLevel = uint32(v)
				case miIsLighted:
					out.MedalLit = v != 0
				case miRUID:
					out.MedalRUID = v
				case miGuardLevel:
					out.MedalGuard = uint32(v)
				}
				return nil
			})
		case uiWealth:
			return walkFields(val, func(n protowire.Number, _ protowire.Type, _ []byte, v uint64) error {
				if n == wiLevel {
					out.WealthLevel = uint32(v)
				}
				return nil
			})
		case uiGuard:
			return walkFields(val, func(n protowire.Number, _ protowire.Type, _ []byte, v uint64) error {
				if n == giLevel {
					out.GuardInfoLv = uint32(v)
				}
				return nil
			})
		}
		return nil
	})
}

// fieldFunc 处理一个 protobuf 字段。
// val 在 BytesType 时为字节内容，v 在 VarintType 时为数值。
type fieldFunc func(num protowire.Number, typ protowire.Type, val []byte, v uint64) error

// walkFields 遍历 protobuf 消息的所有字段，未知字段自动跳过。
func walkFields(b []byte, fn fieldFunc) error {
	for len(b) > 0 {
		num, typ, n := protowire.ConsumeTag(b)
		if n < 0 {
			return fmt.Errorf("cmdmap: protobuf 标签解析失败: %w", protowire.ParseError(n))
		}
		b = b[n:]

		switch typ {
		case protowire.VarintType:
			v, n := protowire.ConsumeVarint(b)
			if n < 0 {
				return fmt.Errorf("cmdmap: protobuf varint 解析失败: %w", protowire.ParseError(n))
			}
			b = b[n:]
			if err := fn(num, typ, nil, v); err != nil {
				return err
			}
		case protowire.BytesType:
			val, n := protowire.ConsumeBytes(b)
			if n < 0 {
				return fmt.Errorf("cmdmap: protobuf bytes 解析失败: %w", protowire.ParseError(n))
			}
			b = b[n:]
			if err := fn(num, typ, val, 0); err != nil {
				return err
			}
		case protowire.Fixed32Type:
			v, n := protowire.ConsumeFixed32(b)
			if n < 0 {
				return fmt.Errorf("cmdmap: protobuf fixed32 解析失败: %w", protowire.ParseError(n))
			}
			b = b[n:]
			if err := fn(num, typ, nil, uint64(v)); err != nil {
				return err
			}
		case protowire.Fixed64Type:
			v, n := protowire.ConsumeFixed64(b)
			if n < 0 {
				return fmt.Errorf("cmdmap: protobuf fixed64 解析失败: %w", protowire.ParseError(n))
			}
			b = b[n:]
			if err := fn(num, typ, nil, v); err != nil {
				return err
			}
		default:
			// 组类型（已废弃）等，跳过整个字段。
			n := protowire.ConsumeFieldValue(num, typ, b)
			if n < 0 {
				return fmt.Errorf("cmdmap: protobuf 未知线型跳过失败: %w", protowire.ParseError(n))
			}
			b = b[n:]
		}
	}
	return nil
}
