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
}

// mapBattle 把 PK 相关 CMD 归一化为 Battle 事件。
func mapBattle(ctx Context, raw json.RawMessage) ([]event.Event, error) {
	b := event.Battle{SubCommand: CommandOf(raw)}
	return []event.Event{NewEvent(ctx, event.TypeBattle, ctx.ReceivedAt, b, raw)}, nil
}
