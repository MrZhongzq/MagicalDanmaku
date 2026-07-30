package cmdmap

import (
	"encoding/base64"
	"encoding/json"
	"fmt"

	"google.golang.org/protobuf/encoding/protowire"

	"github.com/MrZhongzq/MagicalDanmaku/server/internal/event"
)

func init() {
	Register("ONLINE_RANK_V3", mapOnlineRankV3)
}

// ONLINE_RANK_V3 的 protobuf 字段编号。
//
// B 站不公开 .proto，原项目也未处理过该 CMD；以下编号由 2026-07-31
// 的真机抓包逐字段推定，推定依据见各字段注释。
const (
	orRankType = 1 // 榜单类型，如 "online_rank"
	orItem     = 3 // 重复的榜单项

	oriUID        = 1 // 用户 UID
	oriFace       = 2 // 头像地址
	oriScore      = 3 // 贡献值，字符串形式
	oriUname      = 4 // 昵称
	oriRank       = 5 // 名次，从 1 开始
	oriGuardLevel = 6 // 大航海等级；非舰长时 B 站直接不下发该字段
)

// mapOnlineRankV3 解析 protobuf 编码的高能榜榜单。
//
// 与 ONLINE_RANK_V2 一样归一化为 OnlineRankUpdate，但本 CMD 只带名次
// 列表不带总人数，因此 Count 固定为 -1，总人数仍由 ONLINE_RANK_COUNT 提供。
func mapOnlineRankV3(ctx Context, raw json.RawMessage) ([]event.Event, error) {
	pb, err := decodePBField(raw, "ONLINE_RANK_V3")
	if err != nil {
		return nil, err
	}

	top := make([]event.RankUser, 0, 8)
	err = walkFields(pb, func(num protowire.Number, _ protowire.Type, val []byte, _ uint64) error {
		if num != orItem {
			return nil
		}
		ru, err := decodeRankItem(val)
		if err != nil {
			return err
		}
		top = append(top, ru)
		return nil
	})
	if err != nil {
		return nil, err
	}

	r := event.OnlineRankUpdate{Count: -1, Top: top}
	return []event.Event{NewEvent(ctx, event.TypeOnlineRankUpdate, ctx.ReceivedAt, r, raw)}, nil
}

// decodeRankItem 解码单个榜单项。
func decodeRankItem(b []byte) (event.RankUser, error) {
	var ru event.RankUser
	err := walkFields(b, func(num protowire.Number, _ protowire.Type, val []byte, v uint64) error {
		switch num {
		case oriUID:
			ru.User.UID = formatUint(v)
		case oriFace:
			ru.User.AvatarURL = string(val)
		case oriScore:
			ru.Score = string(val)
		case oriUname:
			ru.User.Username = string(val)
		case oriRank:
			ru.Rank = int(v)
		case oriGuardLevel:
			ru.User.GuardLevel = int(v)
		}
		return nil
	})
	return ru, err
}

// decodePBField 从消息的 data.pb 字段解出 base64 编码的 protobuf 字节流。
//
// B 站近年新增的 CMD 多采用这种「JSON 外壳套 protobuf」的形式。
func decodePBField(raw json.RawMessage, cmdName string) ([]byte, error) {
	data, err := unmarshalData(raw, cmdName)
	if err != nil {
		return nil, err
	}
	b64 := getString(data, "pb")
	if b64 == "" {
		return nil, fmt.Errorf("cmdmap: %s 缺少 pb 字段", cmdName)
	}
	pb, err := base64.StdEncoding.DecodeString(b64)
	if err != nil {
		return nil, fmt.Errorf("cmdmap: %s 的 pb 不是合法 base64: %w", cmdName, err)
	}
	return pb, nil
}
