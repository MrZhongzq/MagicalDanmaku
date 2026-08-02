package cmdmap

import (
	"encoding/json"

	"github.com/MrZhongzq/MagicalDanmaku/server/internal/event"
)

// battleCommands 是全部 PK 大乱斗相关 CMD。
//
// P0 只把它们归一化为 TypeBattle 并保留原始数据，不解释业务语义。
// PK 的状态机、偷塔、串门等逻辑属于 P6，届时从 Event.Raw 取数即可，
// 无需改动本文件。
var battleCommands = []string{
	"PK_BATTLE_PRE",
	"PK_BATTLE_PRE_NEW",
	"PK_BATTLE_START",
	"PK_BATTLE_START_NEW",
	"PK_BATTLE_PROCESS",
	"PK_BATTLE_PROCESS_NEW",
	"PK_BATTLE_RANK_CHANGE",
	"PK_BATTLE_FINAL_PROCESS",
	"PK_BATTLE_END",
	"PK_BATTLE_SETTLE",
	"PK_BATTLE_SETTLE_NEW",
	"PK_BATTLE_SETTLE_USER",
	"PK_BATTLE_SETTLE_V2",
	"PK_BATTLE_PUNISH_END",
	"PK_BATTLE_MATCH_TIMEOUT",
	"PK_BATTLE_ENTRANCE",
	"PK_BATTLE_VIDEO_PUNISH_BEGIN",
	"PK_BATTLE_VIDEO_PUNISH_END",
	"PK_LOTTERY_START",
}

func init() {
	for _, name := range battleCommands {
		Register(name, mapBattle)
	}
	// PK_INFO 单独注册：它是唯一携带参战方明细（data.members[]/pk_basic）
	// 的 CMD，battleCommands 里其余 CMD 都没有这份数据，只能归一化 SubCommand。
	Register("PK_INFO", mapPkInfo)
}

// mapBattle 把 PK 相关 CMD 归一化为 Battle 事件。
func mapBattle(ctx Context, raw json.RawMessage) ([]event.Event, error) {
	b := event.Battle{SubCommand: CommandOf(raw)}
	return []event.Event{NewEvent(ctx, event.TypeBattle, ctx.ReceivedAt, b, raw)}, nil
}

// mapPkInfo 解析 PK_INFO，产出带完整参战方明细的 Battle 事件。
//
// members 全量原样交出，一个不过滤——「谁是自己」需要拿 Event.RoomID
// 跟每个 member.RoomID 比对，那是规则层才知道的业务身份，协议层故意
// 不做这个判断（另见 vars.go 的 TODO）。
//
// 不用 init_info/match_info 判断自己/对面：那两个字段的真实语义是
// 「发起方/被匹配方」，跟「自己/对面」是两回事，混用会把主动发起 PK
// 的主播错认成对面。
func mapPkInfo(ctx Context, raw json.RawMessage) ([]event.Event, error) {
	data, err := unmarshalData(raw, "PK_INFO")
	if err != nil {
		return nil, err
	}

	basic := getObject(data, "pk_basic")
	rawMembers := getArray(data, "members")
	members := make([]event.PkMember, 0, len(rawMembers))
	for _, item := range rawMembers {
		m, ok := item.(map[string]any)
		if !ok {
			continue
		}
		members = append(members, event.PkMember{
			RoomID:   getString(m, "room_id"),
			UID:      getString(m, "uid"),
			Username: getString(m, "uname"),
			Face:     getString(m, "face"),
			Votes:    getInt64(m, "votes"),
			IsWinner: getBool(m, "is_winner"),
		})
	}

	b := event.Battle{
		SubCommand: CommandOf(raw),
		PkID:       getString(basic, "pk_id"),
		Members:    members,
		StartTime:  getInt64(basic, "start_time"),
		EndTime:    getInt64(basic, "end_time"),
	}
	return []event.Event{NewEvent(ctx, event.TypeBattle, ctx.ReceivedAt, b, raw)}, nil
}
