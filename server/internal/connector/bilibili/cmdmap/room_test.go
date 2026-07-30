package cmdmap

import (
	"testing"

	"github.com/MrZhongzq/MagicalDanmaku/server/internal/event"
)

func TestLiveStart(t *testing.T) {
	evs, err := Map(testCtx(), loadSample(t, "LIVE_basic"))
	if err != nil {
		t.Fatalf("Map 失败: %v", err)
	}
	if len(evs) != 1 || evs[0].Type != event.TypeLiveStart {
		t.Fatalf("结果错误: %+v", evs)
	}
	if _, ok := evs[0].Payload.(event.LiveStart); !ok {
		t.Fatalf("载荷类型 = %T", evs[0].Payload)
	}
	// live_time 在顶层，应被用作事件时间
	if got := evs[0].Timestamp.Unix(); got != 1700000000 {
		t.Errorf("Timestamp = %d, 期望 1700000000", got)
	}
}

func TestLiveStop(t *testing.T) {
	evs, err := Map(testCtx(), loadSample(t, "PREPARING_basic"))
	if err != nil {
		t.Fatalf("Map 失败: %v", err)
	}
	if evs[0].Type != event.TypeLiveStop {
		t.Fatalf("Type = %s, 期望 %s", evs[0].Type, event.TypeLiveStop)
	}
}

func TestRoomChange(t *testing.T) {
	evs, err := Map(testCtx(), loadSample(t, "ROOM_CHANGE_basic"))
	if err != nil {
		t.Fatalf("Map 失败: %v", err)
	}
	r := evs[0].Payload.(event.RoomChange)
	if r.Title != "今天也在唱歌" {
		t.Errorf("Title = %q", r.Title)
	}
	if r.AreaID != "21" || r.AreaName != "视频唱见" {
		t.Errorf("Area = %q/%q", r.AreaID, r.AreaName)
	}
	if r.ParentAreaID != "1" || r.ParentAreaName != "娱乐" {
		t.Errorf("ParentArea = %q/%q", r.ParentAreaID, r.ParentAreaName)
	}
}

func TestRoomBlockMsg(t *testing.T) {
	evs, err := Map(testCtx(), loadSample(t, "ROOM_BLOCK_MSG_basic"))
	if err != nil {
		t.Fatalf("Map 失败: %v", err)
	}
	if evs[0].Type != event.TypeUserBlocked {
		t.Fatalf("Type = %s", evs[0].Type)
	}
	b := evs[0].Payload.(event.UserBlocked)
	if b.User.UID != "99999999" || b.User.Username != "被禁言的人" {
		t.Errorf("User = %+v", b.User)
	}
}

func TestOnlineRankCount(t *testing.T) {
	evs, err := Map(testCtx(), loadSample(t, "ONLINE_RANK_COUNT_basic"))
	if err != nil {
		t.Fatalf("Map 失败: %v", err)
	}
	r := evs[0].Payload.(event.OnlineRankUpdate)
	if r.Count != 233 {
		t.Errorf("Count = %d, 期望 233", r.Count)
	}
	if len(r.Top) != 0 {
		t.Errorf("Top 应为空，实际 %d 项", len(r.Top))
	}
}

func TestOnlineRankV2(t *testing.T) {
	evs, err := Map(testCtx(), loadSample(t, "ONLINE_RANK_V2_basic"))
	if err != nil {
		t.Fatalf("Map 失败: %v", err)
	}
	r := evs[0].Payload.(event.OnlineRankUpdate)
	if r.Count != -1 {
		t.Errorf("未下发总数时 Count 应为 -1，实际 %d", r.Count)
	}
	if len(r.Top) != 2 {
		t.Fatalf("Top 项数 = %d, 期望 2", len(r.Top))
	}
	if r.Top[0].User.UID != "111" || r.Top[0].Rank != 1 || r.Top[0].Score != "12000" {
		t.Errorf("Top[0] = %+v", r.Top[0])
	}
	if r.Top[0].User.GuardLevel != event.GuardCaptain {
		t.Errorf("Top[0].GuardLevel = %d", r.Top[0].User.GuardLevel)
	}
	if r.Top[1].User.Username != "榜二" {
		t.Errorf("Top[1].Username = %q", r.Top[1].User.Username)
	}
}

func TestRoomRealTimeMessageUpdate(t *testing.T) {
	evs, err := Map(testCtx(), loadSample(t, "ROOM_REAL_TIME_MESSAGE_UPDATE_basic"))
	if err != nil {
		t.Fatalf("Map 失败: %v", err)
	}
	s := evs[0].Payload.(event.RoomStatsUpdate)
	if s.Fans == nil || *s.Fans != 12345 {
		t.Errorf("Fans = %v", s.Fans)
	}
	if s.FansClub == nil || *s.FansClub != 678 {
		t.Errorf("FansClub = %v", s.FansClub)
	}
	if s.Watched != nil {
		t.Errorf("本 CMD 不含 Watched，应为 nil，实际 %v", *s.Watched)
	}
}

func TestWatchedChange(t *testing.T) {
	evs, err := Map(testCtx(), loadSample(t, "WATCHED_CHANGE_basic"))
	if err != nil {
		t.Fatalf("Map 失败: %v", err)
	}
	s := evs[0].Payload.(event.RoomStatsUpdate)
	if s.Watched == nil || *s.Watched != 4567 {
		t.Errorf("Watched = %v", s.Watched)
	}
	if s.Fans != nil {
		t.Error("本 CMD 不含 Fans，应为 nil")
	}
}
