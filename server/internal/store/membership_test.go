package store

import (
	"context"
	"errors"
	"testing"

	"github.com/MrZhongzq/MagicalDanmaku/server/internal/perm"
)

func TestGrantAndCan(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()

	uid := mustUser(t, s, "李四")
	bid := mustBinding(t, s, "小号", "123")

	if err := s.Grant(ctx, "李四", "小号", "123",
		[]perm.Permission{perm.RuleRead, perm.RuleWrite}); err != nil {
		t.Fatalf("授权报错: %v", err)
	}

	ok, err := s.Can(ctx, uid, bid, perm.RuleWrite)
	if err != nil {
		t.Fatalf("Can 报错: %v", err)
	}
	if !ok {
		t.Error("已授予的权限点应通过")
	}

	ok, err = s.Can(ctx, uid, bid, perm.UserBlock)
	if err != nil {
		t.Fatalf("Can 报错: %v", err)
	}
	if ok {
		t.Error("未授予的权限点不应通过")
	}
}

// 授权单位是绑定：能改甲房间的规则，不代表能碰乙房间
func TestPermissionDoesNotLeakAcrossBindings(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()

	uid := mustUser(t, s, "李四")
	accID := mustAccount(t, s, "小号")
	b1, err := s.UpsertBinding(ctx, accID, "甲")
	if err != nil {
		t.Fatalf("创建绑定甲报错: %v", err)
	}
	b2, err := s.UpsertBinding(ctx, accID, "乙")
	if err != nil {
		t.Fatalf("创建绑定乙报错: %v", err)
	}

	if err := s.Grant(ctx, "李四", "小号", "甲",
		[]perm.Permission{perm.RuleWrite}); err != nil {
		t.Fatalf("授权报错: %v", err)
	}

	ok, err := s.Can(ctx, uid, b1.ID, perm.RuleWrite)
	if err != nil {
		t.Fatalf("Can 报错: %v", err)
	}
	if !ok {
		t.Error("绑定甲上应有权限")
	}

	ok, err = s.Can(ctx, uid, b2.ID, perm.RuleWrite)
	if err != nil {
		t.Fatalf("Can 报错: %v", err)
	}
	if ok {
		t.Error("绑定乙上不该有权限——授权单位是绑定，不是账号")
	}
}

func TestAdminBypassesAllChecks(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()

	admin, err := s.CreateUser(ctx, "管理员", "pw", true)
	if err != nil {
		t.Fatalf("创建管理员报错: %v", err)
	}
	bid := mustBinding(t, s, "小号", "123")

	for _, p := range perm.All() {
		ok, err := s.Can(ctx, admin.ID, bid, p)
		if err != nil {
			t.Fatalf("Can(%s) 报错: %v", p, err)
		}
		if !ok {
			t.Errorf("管理员应绕过 %s 的检查", p)
		}
	}
}

func TestCanWithoutMembershipIsFalse(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()

	uid := mustUser(t, s, "路人")
	bid := mustBinding(t, s, "小号", "123")

	ok, err := s.Can(ctx, uid, bid, perm.RuleRead)
	if err != nil {
		t.Fatalf("Can 报错: %v", err)
	}
	if ok {
		t.Error("没有授权记录时应一律拒绝")
	}
}

func TestGrantReplacesPreviousPermissions(t *testing.T) {
	// 重新授权是「设定为这些」，不是「再加上这些」
	s := testStore(t)
	ctx := context.Background()

	uid := mustUser(t, s, "李四")
	bid := mustBinding(t, s, "小号", "123")

	if err := s.Grant(ctx, "李四", "小号", "123",
		[]perm.Permission{perm.RuleRead, perm.RuleWrite}); err != nil {
		t.Fatalf("首次授权报错: %v", err)
	}
	if err := s.Grant(ctx, "李四", "小号", "123",
		[]perm.Permission{perm.EventRead}); err != nil {
		t.Fatalf("二次授权报错: %v", err)
	}

	ok, err := s.Can(ctx, uid, bid, perm.RuleWrite)
	if err != nil {
		t.Fatalf("Can 报错: %v", err)
	}
	if ok {
		t.Error("重新授权应替换而非累加")
	}
	ok, err = s.Can(ctx, uid, bid, perm.EventRead)
	if err != nil {
		t.Fatalf("Can 报错: %v", err)
	}
	if !ok {
		t.Error("新授予的权限点应生效")
	}
}

func TestRevoke(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()

	uid := mustUser(t, s, "李四")
	bid := mustBinding(t, s, "小号", "123")

	if err := s.Grant(ctx, "李四", "小号", "123",
		[]perm.Permission{perm.RuleRead}); err != nil {
		t.Fatalf("授权报错: %v", err)
	}
	if err := s.Revoke(ctx, "李四", "小号", "123"); err != nil {
		t.Fatalf("撤销报错: %v", err)
	}

	ok, err := s.Can(ctx, uid, bid, perm.RuleRead)
	if err != nil {
		t.Fatalf("Can 报错: %v", err)
	}
	if ok {
		t.Error("撤销后不应再有权限")
	}
}

func TestRevokeMissingMembership(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	mustUser(t, s, "李四")
	mustBinding(t, s, "小号", "123")

	if err := s.Revoke(ctx, "李四", "小号", "123"); !errors.Is(err, ErrNotFound) {
		t.Errorf("应返回 ErrNotFound，实际: %v", err)
	}
}

func TestGrantRejectsUnknownUser(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	mustBinding(t, s, "小号", "123")

	err := s.Grant(ctx, "查无此人", "小号", "123", []perm.Permission{perm.RuleRead})
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("应返回 ErrNotFound，实际: %v", err)
	}
}

func TestGrantRejectsUnknownBinding(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	mustUser(t, s, "李四")

	err := s.Grant(ctx, "李四", "没这个号", "123", []perm.Permission{perm.RuleRead})
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("应返回 ErrNotFound，实际: %v", err)
	}
}

func TestGrantRejectsEmptyPermissions(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	mustUser(t, s, "李四")
	mustBinding(t, s, "小号", "123")

	if err := s.Grant(ctx, "李四", "小号", "123", nil); err == nil {
		t.Error("空权限列表应被拒绝——那等于什么都没授权，语义含糊")
	}
}

func TestListMembershipsForUser(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()

	mustUser(t, s, "李四")
	accID := mustAccount(t, s, "小号")
	if _, err := s.UpsertBinding(ctx, accID, "甲"); err != nil {
		t.Fatalf("创建绑定甲报错: %v", err)
	}
	if _, err := s.UpsertBinding(ctx, accID, "乙"); err != nil {
		t.Fatalf("创建绑定乙报错: %v", err)
	}

	if err := s.Grant(ctx, "李四", "小号", "甲",
		[]perm.Permission{perm.RuleWrite}); err != nil {
		t.Fatalf("授权甲报错: %v", err)
	}
	if err := s.Grant(ctx, "李四", "小号", "乙",
		[]perm.Permission{perm.EventRead, perm.UserBlock}); err != nil {
		t.Fatalf("授权乙报错: %v", err)
	}

	ms, err := s.ListMemberships(ctx, "李四")
	if err != nil {
		t.Fatalf("列出授权报错: %v", err)
	}
	if len(ms) != 2 {
		t.Fatalf("授权数 = %d, 期望 2", len(ms))
	}
	if ms[0].RoomID != "甲" || len(ms[0].Permissions) != 1 {
		t.Errorf("第一条 = %+v", ms[0])
	}
	if len(ms[1].Permissions) != 2 {
		t.Errorf("第二条的权限点数 = %d, 期望 2", len(ms[1].Permissions))
	}
	if ms[0].AccountName != "小号" || ms[0].Username != "李四" {
		t.Errorf("账号名与用户名应带出: %+v", ms[0])
	}
}

func TestListBindingMembers(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()

	mustUser(t, s, "李四")
	mustUser(t, s, "王五")
	mustBinding(t, s, "小号", "123")

	if err := s.Grant(ctx, "李四", "小号", "123",
		[]perm.Permission{perm.RuleWrite}); err != nil {
		t.Fatalf("授权李四报错: %v", err)
	}
	if err := s.Grant(ctx, "王五", "小号", "123",
		[]perm.Permission{perm.UserBlock}); err != nil {
		t.Fatalf("授权王五报错: %v", err)
	}

	ms, err := s.ListBindingMembers(ctx, "小号", "123")
	if err != nil {
		t.Fatalf("列出成员报错: %v", err)
	}
	if len(ms) != 2 {
		t.Fatalf("成员数 = %d, 期望 2", len(ms))
	}
}

// 账号所有者对自己账号下的绑定拥有全部权限点。
//
// 所有者已经能删账号、删绑定、换 Cookie。不给他 rule:write 的话，
// 他能把绑定整个删掉却不能把它停用——那是不一致，不是安全。
func TestOwnerHasAllPermissionsOnOwnBindings(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()

	owner := mustUser(t, s, "张三")
	a, err := s.CreateAccount(ctx, AccountInput{
		Name: "主播号", Cookie: "c", OwnerID: owner,
	})
	if err != nil {
		t.Fatalf("创建账号报错: %v", err)
	}
	b, err := s.UpsertBinding(ctx, a.ID, "123")
	if err != nil {
		t.Fatalf("创建绑定报错: %v", err)
	}

	// 所有者没有任何 memberships 行
	for _, p := range perm.All() {
		ok, err := s.Can(ctx, owner, b.ID, p)
		if err != nil {
			t.Fatalf("Can(%s) 报错: %v", p, err)
		}
		if !ok {
			t.Errorf("所有者应拥有 %s", p)
		}
	}
}

// 所有权不跨账号：拥有甲账号不等于能碰乙账号的绑定
func TestOwnershipDoesNotLeakAcrossAccounts(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()

	zhang := mustUser(t, s, "张三")
	other := mustAccount(t, s, "别人的号") // mustAccount 建的所有者是 owner_别人的号
	b, err := s.UpsertBinding(ctx, other, "999")
	if err != nil {
		t.Fatalf("创建绑定报错: %v", err)
	}

	ok, err := s.Can(ctx, zhang, b.ID, perm.RuleWrite)
	if err != nil {
		t.Fatalf("Can 报错: %v", err)
	}
	if ok {
		t.Error("拥有别的账号不该给你这个账号的绑定上的权限")
	}
}

func TestDeletingUserRemovesMemberships(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()

	u, err := s.CreateUser(ctx, "李四", "pw", false)
	if err != nil {
		t.Fatalf("创建用户报错: %v", err)
	}
	mustBinding(t, s, "小号", "123")
	if err := s.Grant(ctx, "李四", "小号", "123",
		[]perm.Permission{perm.RuleRead}); err != nil {
		t.Fatalf("授权报错: %v", err)
	}

	if _, err := s.pool.Exec(ctx, `DELETE FROM users WHERE id = $1`, u.ID); err != nil {
		t.Fatalf("删除用户报错: %v", err)
	}

	var n int
	if err := s.pool.QueryRow(ctx, `SELECT count(*) FROM memberships`).Scan(&n); err != nil {
		t.Fatalf("统计授权报错: %v", err)
	}
	if n != 0 {
		t.Errorf("用户删除后授权应级联删除，实际剩 %d 条", n)
	}
}
