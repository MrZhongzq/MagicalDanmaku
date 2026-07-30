package cmdmap

import (
	"encoding/json"
	"testing"

	"github.com/MrZhongzq/MagicalDanmaku/server/internal/event"
)

func TestBattleNormalized(t *testing.T) {
	evs, err := Map(testCtx(), loadSample(t, "PK_BATTLE_START_NEW_basic"))
	if err != nil {
		t.Fatalf("Map 失败: %v", err)
	}
	if len(evs) != 1 || evs[0].Type != event.TypeBattle {
		t.Fatalf("结果错误: %+v", evs)
	}

	b := evs[0].Payload.(event.Battle)
	if b.SubCommand != "PK_BATTLE_START_NEW" {
		t.Errorf("SubCommand = %q", b.SubCommand)
	}
	// 完整数据必须留在 Raw 里供 P6 使用
	if !json.Valid(evs[0].Raw) {
		t.Error("Raw 必须是合法 JSON")
	}
	var probe map[string]any
	if err := json.Unmarshal(evs[0].Raw, &probe); err != nil {
		t.Fatalf("Raw 解析失败: %v", err)
	}
	if probe["pk_id"] == nil {
		t.Error("Raw 中应保留 pk_id")
	}
}

func TestAllBattleCommandsRegistered(t *testing.T) {
	for _, name := range battleCommands {
		raw := json.RawMessage(`{"cmd":"` + name + `"}`)
		evs, err := Map(testCtx(), raw)
		if err != nil {
			t.Errorf("%s: Map 失败: %v", name, err)
			continue
		}
		if len(evs) != 1 || evs[0].Type != event.TypeBattle {
			t.Errorf("%s: 未归一化为 Battle，实际 %+v", name, evs)
		}
	}
}

func TestIgnoredCommandsProduceNoEvents(t *testing.T) {
	for _, name := range ignoredCommands {
		raw := json.RawMessage(`{"cmd":"` + name + `"}`)
		evs, err := Map(testCtx(), raw)
		if err != nil {
			t.Errorf("%s: Map 失败: %v", name, err)
			continue
		}
		if len(evs) != 0 {
			t.Errorf("%s: 应被忽略，实际产出 %d 个事件", name, len(evs))
		}
	}
}
