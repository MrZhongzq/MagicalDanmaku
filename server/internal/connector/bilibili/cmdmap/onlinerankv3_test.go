package cmdmap

import (
	"encoding/base64"
	"encoding/json"
	"testing"

	"github.com/MrZhongzq/MagicalDanmaku/server/internal/event"
)

// buildRankItem 构造一个 ONLINE_RANK_V3 的榜单项。
func buildRankItem(uid uint64, face, score, uname string, rank, guard uint64) []byte {
	it := pbVarint(nil, 1, uid)
	it = pbString(it, 2, face)
	it = pbString(it, 3, score)
	it = pbString(it, 4, uname)
	it = pbVarint(it, 5, rank)
	if guard > 0 { // 非舰长时 B 站直接不下发该字段
		it = pbVarint(it, 6, guard)
	}
	return it
}

// buildOnlineRankV3 构造一条 ONLINE_RANK_V3 消息。
func buildOnlineRankV3(rankType string, items ...[]byte) json.RawMessage {
	pb := pbString(nil, 1, rankType)
	for _, it := range items {
		pb = pbMessage(pb, 3, it)
	}
	payload := map[string]any{
		"cmd":  "ONLINE_RANK_V3",
		"data": map[string]any{"pb": base64.StdEncoding.EncodeToString(pb)},
	}
	raw, _ := json.Marshal(payload)
	return raw
}

func TestOnlineRankV3(t *testing.T) {
	raw := buildOnlineRankV3("online_rank",
		buildRankItem(356509358, "https://i2.hdslb.com/bfs/face/a.jpg", "55", "秽白君澜-许许的蓷", 1, 0),
		buildRankItem(695193977, "https://i2.hdslb.com/bfs/face/b.webp", "33", "世一糖爱音", 3, 3),
	)

	evs, err := Map(testCtx(), raw)
	if err != nil {
		t.Fatalf("Map 失败: %v", err)
	}
	if len(evs) != 1 || evs[0].Type != event.TypeOnlineRankUpdate {
		t.Fatalf("结果错误: %+v", evs)
	}

	r := evs[0].Payload.(event.OnlineRankUpdate)
	if r.Count != -1 {
		t.Errorf("本 CMD 不下发总人数，Count 应为 -1，实际 %d", r.Count)
	}
	if len(r.Top) != 2 {
		t.Fatalf("Top 项数 = %d, 期望 2", len(r.Top))
	}

	first := r.Top[0]
	if first.User.UID != "356509358" {
		t.Errorf("Top[0].UID = %q", first.User.UID)
	}
	if first.User.Username != "秽白君澜-许许的蓷" {
		t.Errorf("Top[0].Username = %q", first.User.Username)
	}
	if first.User.AvatarURL != "https://i2.hdslb.com/bfs/face/a.jpg" {
		t.Errorf("Top[0].AvatarURL = %q", first.User.AvatarURL)
	}
	if first.Rank != 1 {
		t.Errorf("Top[0].Rank = %d", first.Rank)
	}
	if first.Score != "55" {
		t.Errorf("Top[0].Score = %q", first.Score)
	}
	if first.User.GuardLevel != event.GuardNone {
		t.Errorf("Top[0].GuardLevel = %d，非舰长应为 0", first.User.GuardLevel)
	}

	second := r.Top[1]
	if second.Rank != 3 {
		t.Errorf("Top[1].Rank = %d", second.Rank)
	}
	if second.User.GuardLevel != event.GuardCaptain {
		t.Errorf("Top[1].GuardLevel = %d, 期望 3", second.User.GuardLevel)
	}
	if second.User.Username != "世一糖爱音" {
		t.Errorf("Top[1].Username = %q", second.User.Username)
	}
}

func TestOnlineRankV3EmptyList(t *testing.T) {
	raw := buildOnlineRankV3("online_rank")
	evs, err := Map(testCtx(), raw)
	if err != nil {
		t.Fatalf("Map 失败: %v", err)
	}
	if len(evs) != 1 || evs[0].Type != event.TypeOnlineRankUpdate {
		t.Fatalf("空榜单也应产出事件: %+v", evs)
	}
	if got := len(evs[0].Payload.(event.OnlineRankUpdate).Top); got != 0 {
		t.Errorf("Top 项数 = %d, 期望 0", got)
	}
}

func TestOnlineRankV3IgnoresUnknownFields(t *testing.T) {
	it := buildRankItem(123, "f", "10", "某人", 1, 0)
	it = pbString(it, 66, "未来新增字段")

	pb := pbString(nil, 1, "online_rank")
	pb = pbMessage(pb, 3, it)
	pb = pbVarint(pb, 99, 42) // 顶层未知字段

	payload := map[string]any{
		"cmd":  "ONLINE_RANK_V3",
		"data": map[string]any{"pb": base64.StdEncoding.EncodeToString(pb)},
	}
	raw, _ := json.Marshal(payload)

	evs, err := Map(testCtx(), raw)
	if err != nil {
		t.Fatalf("遇到未知字段不应报错: %v", err)
	}
	r := evs[0].Payload.(event.OnlineRankUpdate)
	if len(r.Top) != 1 || r.Top[0].User.Username != "某人" {
		t.Errorf("未知字段干扰了已知字段的解析: %+v", r.Top)
	}
}

func TestOnlineRankV3RejectsBadPayload(t *testing.T) {
	raw := json.RawMessage(`{"cmd":"ONLINE_RANK_V3","data":{"pb":"!!!not-base64!!!"}}`)
	if _, err := Map(testCtx(), raw); err == nil {
		t.Error("非法 base64 应当报错")
	}
}
