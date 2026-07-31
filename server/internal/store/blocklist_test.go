package store

import (
	"context"
	"errors"
	"testing"
)

func TestAddAndListBlockList(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	uid := mustUser(t, s, "房管")
	bid := mustBinding(t, s, "小号", "123")

	if err := s.AddToBlockList(ctx, bid, "10086", "广告号", "刷屏加群", &uid); err != nil {
		t.Fatalf("加入名单报错: %v", err)
	}

	list, err := s.ListBlockList(ctx, bid)
	if err != nil {
		t.Fatalf("列出名单报错: %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("名单长度 = %d, 期望 1", len(list))
	}
	if list[0].UID != "10086" || list[0].Reason != "刷屏加群" {
		t.Errorf("记录 = %+v", list[0])
	}
	if list[0].CreatedBy == nil || *list[0].CreatedBy != uid {
		t.Errorf("操作人未记录: %+v", list[0].CreatedBy)
	}
}

func TestIsBlocked(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	bid := mustBinding(t, s, "小号", "123")

	if err := s.AddToBlockList(ctx, bid, "10086", "", "", nil); err != nil {
		t.Fatalf("加入名单报错: %v", err)
	}

	blocked, err := s.IsBlocked(ctx, bid, "10086")
	if err != nil {
		t.Fatalf("IsBlocked 报错: %v", err)
	}
	if !blocked {
		t.Error("名单内的 UID 应返回 true")
	}

	blocked, err = s.IsBlocked(ctx, bid, "99999")
	if err != nil {
		t.Fatalf("IsBlocked 报错: %v", err)
	}
	if blocked {
		t.Error("名单外的 UID 应返回 false")
	}
}

func TestBlockListIsolatedPerBinding(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	accID := mustAccount(t, s, "小号")

	b1, err := s.UpsertBinding(ctx, accID, "甲")
	if err != nil {
		t.Fatalf("创建绑定甲报错: %v", err)
	}
	b2, err := s.UpsertBinding(ctx, accID, "乙")
	if err != nil {
		t.Fatalf("创建绑定乙报错: %v", err)
	}

	if err := s.AddToBlockList(ctx, b1.ID, "10086", "", "", nil); err != nil {
		t.Fatalf("加入名单报错: %v", err)
	}

	blocked, err := s.IsBlocked(ctx, b2.ID, "10086")
	if err != nil {
		t.Fatalf("IsBlocked 报错: %v", err)
	}
	if blocked {
		t.Error("甲房间的禁言名单不该影响乙房间")
	}
}

func TestAddToBlockListTwiceUpdatesReason(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	bid := mustBinding(t, s, "小号", "123")

	if err := s.AddToBlockList(ctx, bid, "10086", "旧名字", "旧理由", nil); err != nil {
		t.Fatalf("首次加入报错: %v", err)
	}
	if err := s.AddToBlockList(ctx, bid, "10086", "新名字", "新理由", nil); err != nil {
		t.Fatalf("二次加入报错: %v", err)
	}

	list, err := s.ListBlockList(ctx, bid)
	if err != nil {
		t.Fatalf("列出名单报错: %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("重复加入不该产生两条，实际 %d 条", len(list))
	}
	if list[0].Reason != "新理由" || list[0].Username != "新名字" {
		t.Errorf("应更新为最新信息: %+v", list[0])
	}
}

func TestRemoveFromBlockList(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	bid := mustBinding(t, s, "小号", "123")

	if err := s.AddToBlockList(ctx, bid, "10086", "", "", nil); err != nil {
		t.Fatalf("加入名单报错: %v", err)
	}
	if err := s.RemoveFromBlockList(ctx, bid, "10086"); err != nil {
		t.Fatalf("移出名单报错: %v", err)
	}

	blocked, err := s.IsBlocked(ctx, bid, "10086")
	if err != nil {
		t.Fatalf("IsBlocked 报错: %v", err)
	}
	if blocked {
		t.Error("移出后应返回 false")
	}
}

func TestRemoveMissingFromBlockList(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	bid := mustBinding(t, s, "小号", "123")

	if err := s.RemoveFromBlockList(ctx, bid, "10086"); !errors.Is(err, ErrNotFound) {
		t.Errorf("应返回 ErrNotFound，实际: %v", err)
	}
}

// 操作人被删除时名单要留下，只是不知道是谁加的
func TestBlockListSurvivesUserDeletion(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()

	u, err := s.CreateUser(ctx, "房管", "pw", false)
	if err != nil {
		t.Fatalf("创建用户报错: %v", err)
	}
	bid := mustBinding(t, s, "小号", "123")
	if err := s.AddToBlockList(ctx, bid, "10086", "", "刷屏", &u.ID); err != nil {
		t.Fatalf("加入名单报错: %v", err)
	}

	if _, err := s.pool.Exec(ctx, `DELETE FROM users WHERE id = $1`, u.ID); err != nil {
		t.Fatalf("删除用户报错: %v", err)
	}

	list, err := s.ListBlockList(ctx, bid)
	if err != nil {
		t.Fatalf("列出名单报错: %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("名单应保留，实际 %d 条", len(list))
	}
	if list[0].CreatedBy != nil {
		t.Errorf("操作人应置空，实际 %v", *list[0].CreatedBy)
	}
}
