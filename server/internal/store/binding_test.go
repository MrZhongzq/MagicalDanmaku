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

// TestNewBindingHasUnknownLiveStatus 验证新绑定的默认开播状态是
// unknown（尚未检测过），不是 offline——加绑定的那一刻还没探测过
// 直播间，"没检测过"与"确认没开播"是两回事，不能用同一个值表示。
func TestNewBindingHasUnknownLiveStatus(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	accID := mustAccount(t, s, "小号")

	b, err := s.UpsertBinding(ctx, accID, "123")
	if err != nil {
		t.Fatalf("创建绑定报错: %v", err)
	}
	if b.LiveStatus != RoomLiveUnknown {
		t.Errorf("LiveStatus = %q, 期望 %q", b.LiveStatus, RoomLiveUnknown)
	}
	if b.LiveCheckedAt != nil {
		t.Errorf("LiveCheckedAt 应为 nil（从未检测过），实际 %v", b.LiveCheckedAt)
	}
	if b.AnchorUID != "" || b.AnchorName != "" {
		t.Errorf("主播身份应为空，实际 uid=%q name=%q", b.AnchorUID, b.AnchorName)
	}
}

// TestUpdateBindingRoomStatusWritesLiveAndOffline 验证探测成功时
// live_status 与主播身份都被正确写入。
func TestUpdateBindingRoomStatusWritesLiveAndOffline(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	accID := mustAccount(t, s, "小号")
	b, err := s.UpsertBinding(ctx, accID, "123")
	if err != nil {
		t.Fatalf("创建绑定报错: %v", err)
	}

	if err := s.UpdateBindingRoomStatus(ctx, b.ID, RoomLiveLiving, "20285041", "舞月雅白"); err != nil {
		t.Fatalf("写入直播间状态报错: %v", err)
	}
	got, err := s.GetBindingByID(ctx, b.ID)
	if err != nil {
		t.Fatalf("查询绑定报错: %v", err)
	}
	if got.LiveStatus != RoomLiveLiving {
		t.Errorf("LiveStatus = %q, 期望 %q", got.LiveStatus, RoomLiveLiving)
	}
	if got.AnchorUID != "20285041" || got.AnchorName != "舞月雅白" {
		t.Errorf("主播身份 = uid=%q name=%q", got.AnchorUID, got.AnchorName)
	}
	if got.LiveCheckedAt == nil {
		t.Error("LiveCheckedAt 应被写入")
	}
}

// TestUpdateBindingRoomStatusUnknownDoesNotClobberAnchor 覆盖探测失败
// （调用方传空的 anchorUID/anchorName）时，不能用空值把上一次探测
// 成功时留下的主播身份抹掉——网络抖动/风控这类瞬时失败不该让界面上
// 已经显示出来的主播昵称突然消失。
func TestUpdateBindingRoomStatusUnknownDoesNotClobberAnchor(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	accID := mustAccount(t, s, "小号")
	b, err := s.UpsertBinding(ctx, accID, "123")
	if err != nil {
		t.Fatalf("创建绑定报错: %v", err)
	}
	if err := s.UpdateBindingRoomStatus(ctx, b.ID, RoomLiveLiving, "20285041", "舞月雅白"); err != nil {
		t.Fatalf("首次写入报错: %v", err)
	}

	// 模拟下一轮探测失败：状态降级为 unknown，但没有新的主播身份可写
	if err := s.UpdateBindingRoomStatus(ctx, b.ID, RoomLiveUnknown, "", ""); err != nil {
		t.Fatalf("二次写入报错: %v", err)
	}

	got, err := s.GetBindingByID(ctx, b.ID)
	if err != nil {
		t.Fatalf("查询绑定报错: %v", err)
	}
	if got.LiveStatus != RoomLiveUnknown {
		t.Errorf("LiveStatus = %q, 期望 %q", got.LiveStatus, RoomLiveUnknown)
	}
	if got.AnchorUID != "20285041" || got.AnchorName != "舞月雅白" {
		t.Errorf("探测失败不该抹掉上一次已知的主播身份，实际 uid=%q name=%q",
			got.AnchorUID, got.AnchorName)
	}
}

// TestUpdateBindingRoomStatusRejectsInvalidValue 验证非法的三态取值
// 会被拒绝，而不是悄悄写进库里破坏 CHECK 约束假定的取值范围。
func TestUpdateBindingRoomStatusRejectsInvalidValue(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	accID := mustAccount(t, s, "小号")
	b, err := s.UpsertBinding(ctx, accID, "123")
	if err != nil {
		t.Fatalf("创建绑定报错: %v", err)
	}
	if err := s.UpdateBindingRoomStatus(ctx, b.ID, "非法值", "", ""); err == nil {
		t.Error("非法的状态取值应报错")
	}
}

// TestGetBindingByIDNotFound 验证查不到的绑定返回 ErrNotFound。
func TestGetBindingByIDNotFound(t *testing.T) {
	s := testStore(t)
	if _, err := s.GetBindingByID(context.Background(), 999999); !errors.Is(err, ErrNotFound) {
		t.Errorf("应返回 ErrNotFound，实际: %v", err)
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
