package store

import (
	"context"
	"errors"
	"testing"
	"time"
)

// mustAccount 建一个账号并返回 ID。
func mustAccount(t *testing.T, s *Store, name string) int64 {
	t.Helper()
	owner := mustUser(t, s, "owner_"+name)
	a, err := s.CreateAccount(context.Background(), AccountInput{
		Name: name, Cookie: "c", OwnerID: owner,
	})
	if err != nil {
		t.Fatalf("创建账号 %s 报错: %v", name, err)
	}
	return a.ID
}

func TestUpsertAndGetBinding(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	accID := mustAccount(t, s, "小号")

	b, err := s.UpsertBinding(ctx, accID, "1706666491")
	if err != nil {
		t.Fatalf("创建绑定报错: %v", err)
	}
	if b.ID == 0 || !b.Enabled {
		t.Errorf("新绑定 = %+v, 应有非零 ID 且默认启用", b)
	}

	got, err := s.GetBinding(ctx, "小号", "1706666491")
	if err != nil {
		t.Fatalf("查询绑定报错: %v", err)
	}
	if got.ID != b.ID || got.AccountName != "小号" || got.RoomID != "1706666491" {
		t.Errorf("绑定 = %+v", got)
	}
}

func TestBindingLabel(t *testing.T) {
	b := Binding{AccountName: "小号", RoomID: "1706666491"}
	if b.Label() != "小号@1706666491" {
		t.Errorf("Label() = %q, 期望 \"小号@1706666491\"", b.Label())
	}
}

func TestUpsertBindingIsIdempotent(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	accID := mustAccount(t, s, "小号")

	first, err := s.UpsertBinding(ctx, accID, "123")
	if err != nil {
		t.Fatalf("首次报错: %v", err)
	}
	second, err := s.UpsertBinding(ctx, accID, "123")
	if err != nil {
		t.Fatalf("二次报错: %v", err)
	}
	if first.ID != second.ID {
		t.Errorf("同一账号同一房间应是同一行，ID 从 %d 变成 %d", first.ID, second.ID)
	}
}

// 同一直播间被两个账号连接时是两条独立绑定，各自有独立的规则与冷却状态
func TestSameRoomTwoAccountsAreSeparateBindings(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	main := mustAccount(t, s, "主播号")
	sub := mustAccount(t, s, "小号")

	b1, err := s.UpsertBinding(ctx, main, "1706666491")
	if err != nil {
		t.Fatalf("主播号绑定报错: %v", err)
	}
	b2, err := s.UpsertBinding(ctx, sub, "1706666491")
	if err != nil {
		t.Fatalf("小号绑定报错: %v", err)
	}
	if b1.ID == b2.ID {
		t.Error("两个账号连同一房间应是两条独立绑定")
	}
}

func TestGetBindingNotFound(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	mustAccount(t, s, "小号")

	if _, err := s.GetBinding(ctx, "小号", "不存在的房间"); !errors.Is(err, ErrNotFound) {
		t.Errorf("房间不存在应返回 ErrNotFound，实际: %v", err)
	}
	if _, err := s.GetBinding(ctx, "没这个号", "123"); !errors.Is(err, ErrNotFound) {
		t.Errorf("账号不存在应返回 ErrNotFound，实际: %v", err)
	}
}

func TestSetBindingEnabled(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	accID := mustAccount(t, s, "小号")

	if _, err := s.UpsertBinding(ctx, accID, "123"); err != nil {
		t.Fatalf("创建绑定报错: %v", err)
	}
	if err := s.SetBindingEnabled(ctx, "小号", "123", false); err != nil {
		t.Fatalf("停用绑定报错: %v", err)
	}
	got, err := s.GetBinding(ctx, "小号", "123")
	if err != nil {
		t.Fatalf("查询绑定报错: %v", err)
	}
	if got.Enabled {
		t.Error("停用后 Enabled 应为 false")
	}
}

func TestListBindingsIncludesAccountName(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	main := mustAccount(t, s, "主播号")
	sub := mustAccount(t, s, "小号")

	if _, err := s.UpsertBinding(ctx, main, "111"); err != nil {
		t.Fatalf("创建绑定报错: %v", err)
	}
	if _, err := s.UpsertBinding(ctx, sub, "222"); err != nil {
		t.Fatalf("创建绑定报错: %v", err)
	}

	bs, err := s.ListBindings(ctx)
	if err != nil {
		t.Fatalf("列出绑定报错: %v", err)
	}
	if len(bs) != 2 {
		t.Fatalf("绑定数 = %d, 期望 2", len(bs))
	}
	if bs[0].AccountName != "主播号" || bs[1].AccountName != "小号" {
		t.Errorf("账号名未带出: %+v", bs)
	}
}

func TestDeleteBinding(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	accID := mustAccount(t, s, "小号")

	if _, err := s.UpsertBinding(ctx, accID, "123"); err != nil {
		t.Fatalf("创建绑定报错: %v", err)
	}
	if err := s.DeleteBinding(ctx, "小号", "123"); err != nil {
		t.Fatalf("删除绑定报错: %v", err)
	}
	if _, err := s.GetBinding(ctx, "小号", "123"); !errors.Is(err, ErrNotFound) {
		t.Errorf("删除后应查不到，实际: %v", err)
	}
}

func TestSetAndGetCooldownGroups(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	accID := mustAccount(t, s, "小号")
	b, err := s.UpsertBinding(ctx, accID, "123")
	if err != nil {
		t.Fatalf("创建绑定报错: %v", err)
	}

	want := map[string]time.Duration{
		"greeting": 5 * time.Second,
		"thanks":   2 * time.Second,
	}
	if err := s.SetCooldownGroups(ctx, b.ID, want); err != nil {
		t.Fatalf("写入冷却组报错: %v", err)
	}

	got, err := s.CooldownGroups(ctx, b.ID)
	if err != nil {
		t.Fatalf("读取冷却组报错: %v", err)
	}
	if len(got) != 2 || got["greeting"] != 5*time.Second || got["thanks"] != 2*time.Second {
		t.Errorf("冷却组 = %v, 期望 %v", got, want)
	}
}

func TestSetCooldownGroupsReplacesInsteadOfMerging(t *testing.T) {
	// 从界面上删掉一个冷却组，就该真的消失，而不是残留
	s := testStore(t)
	ctx := context.Background()
	accID := mustAccount(t, s, "小号")
	b, err := s.UpsertBinding(ctx, accID, "123")
	if err != nil {
		t.Fatalf("创建绑定报错: %v", err)
	}

	if err := s.SetCooldownGroups(ctx, b.ID, map[string]time.Duration{
		"greeting": 5 * time.Second, "thanks": 2 * time.Second,
	}); err != nil {
		t.Fatalf("首次写入报错: %v", err)
	}
	if err := s.SetCooldownGroups(ctx, b.ID, map[string]time.Duration{
		"greeting": time.Second,
	}); err != nil {
		t.Fatalf("二次写入报错: %v", err)
	}

	got, err := s.CooldownGroups(ctx, b.ID)
	if err != nil {
		t.Fatalf("读取冷却组报错: %v", err)
	}
	if len(got) != 1 {
		t.Errorf("冷却组应被整组替换，实际 = %v", got)
	}
	if got["greeting"] != time.Second {
		t.Errorf("greeting = %v, 期望 1s", got["greeting"])
	}
}

func TestCooldownGroupsEmptyReturnsEmptyMap(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	accID := mustAccount(t, s, "小号")
	b, err := s.UpsertBinding(ctx, accID, "123")
	if err != nil {
		t.Fatalf("创建绑定报错: %v", err)
	}

	got, err := s.CooldownGroups(ctx, b.ID)
	if err != nil {
		t.Fatalf("读取冷却组报错: %v", err)
	}
	if got == nil {
		t.Error("未配置时应返回空 map 而非 nil，调用方不该被迫判空")
	}
	if len(got) != 0 {
		t.Errorf("未配置时应为空，实际 = %v", got)
	}
}
