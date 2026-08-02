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

// TestPkInfoParsesMembers 验证 PK_INFO 是唯一携带参战方明细的 CMD。
// 断言必须按字段值比对（room_id/uid/uname/...），不能靠下标——
// 下标假设在真实多人 PK 里会把人对错。
func TestPkInfoParsesMembers(t *testing.T) {
	evs, err := Map(testCtx(), loadSample(t, "PK_INFO_basic"))
	if err != nil {
		t.Fatalf("Map 失败: %v", err)
	}
	if len(evs) != 1 || evs[0].Type != event.TypeBattle {
		t.Fatalf("结果错误: %+v", evs)
	}

	b := evs[0].Payload.(event.Battle)
	if b.SubCommand != "PK_INFO" {
		t.Errorf("SubCommand = %q", b.SubCommand)
	}
	if b.PkID != "398603316" {
		t.Errorf("PkID = %q, 期望 398603316", b.PkID)
	}
	if b.StartTime != 1785658231 {
		t.Errorf("StartTime = %d, 期望 1785658231", b.StartTime)
	}
	if b.EndTime != 1785658541 {
		t.Errorf("EndTime = %d, 期望 1785658541", b.EndTime)
	}
	if len(b.Members) != 2 {
		t.Fatalf("Members 数量 = %d, 期望 2", len(b.Members))
	}

	// 按 RoomID 找，不按下标找——下标在多人 PK 里不可靠。
	// 样本里 self（21452505）对应真实抓包中的被匹配方（match_info），
	// 它赢了这场 PK；这个真实关系必须原样保留，不能为了「self 在前」
	// 的直觉去调换 votes/is_winner，否则黄金样本会反过来教坏后续任务。
	var self, opponent *event.PkMember
	for i := range b.Members {
		m := &b.Members[i]
		switch m.RoomID {
		case "21452505":
			self = m
		case "33333":
			opponent = m
		}
	}
	if self == nil {
		t.Fatal("未找到 RoomID=21452505 的成员")
	}
	if self.UID != "11111111" || self.Username != "本房主播" {
		t.Errorf("self = %+v", self)
	}
	if self.Face != "http://i1.hdslb.com/bfs/face/aaa.jpg" {
		t.Errorf("self.Face = %q", self.Face)
	}
	if self.Votes != 1151 {
		t.Errorf("self.Votes = %d, 期望 1151", self.Votes)
	}
	if !self.IsWinner {
		t.Error("self.IsWinner 应为 true（样本里 is_winner=1，本房赢了这场 PK）")
	}

	if opponent == nil {
		t.Fatal("未找到 RoomID=33333 的成员")
	}
	if opponent.UID != "22222222" || opponent.Username != "对面主播" {
		t.Errorf("opponent = %+v", opponent)
	}
	if opponent.Votes != 65 {
		t.Errorf("opponent.Votes = %d, 期望 65", opponent.Votes)
	}
	if opponent.IsWinner {
		t.Error("opponent.IsWinner 应为 false（样本里 is_winner=0，对面输了）")
	}

	// 完整数据必须留在 Raw 里供 P6 使用
	if !json.Valid(evs[0].Raw) {
		t.Error("Raw 必须是合法 JSON")
	}
}

// TestPkInfoMultiPartyMembers 验证 members 数组可以多于两方，
// 且每一方都能按 RoomID 精确取出——不能假设固定两方或固定下标。
func TestPkInfoMultiPartyMembers(t *testing.T) {
	evs, err := Map(testCtx(), loadSample(t, "PK_INFO_multi"))
	if err != nil {
		t.Fatalf("Map 失败: %v", err)
	}
	b := evs[0].Payload.(event.Battle)
	if len(b.Members) != 3 {
		t.Fatalf("Members 数量 = %d, 期望 3", len(b.Members))
	}

	byRoom := make(map[string]event.PkMember, len(b.Members))
	for _, m := range b.Members {
		byRoom[m.RoomID] = m
	}

	want := map[string]struct {
		uname    string
		votes    int64
		isWinner bool
	}{
		"21452505": {"本房主播", 10, false},
		"33333":    {"对面甲", 20, false},
		"44444":    {"对面乙", 30, true},
	}
	for roomID, w := range want {
		m, ok := byRoom[roomID]
		if !ok {
			t.Fatalf("未找到 RoomID=%s 的成员", roomID)
		}
		if m.Username != w.uname || m.Votes != w.votes || m.IsWinner != w.isWinner {
			t.Errorf("RoomID=%s: got %+v, want uname=%q votes=%d isWinner=%v", roomID, m, w.uname, w.votes, w.isWinner)
		}
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
