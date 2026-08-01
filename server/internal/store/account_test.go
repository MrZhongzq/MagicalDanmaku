package store

import (
	"context"
	"errors"
	"testing"
	"time"
)

// mustUser 建一个用户并返回 ID，账号需要 owner。
func mustUser(t *testing.T, s *Store, name string) int64 {
	t.Helper()
	u, err := s.CreateUser(context.Background(), name, "pw", false)
	if err != nil {
		t.Fatalf("创建用户 %s 报错: %v", name, err)
	}
	return u.ID
}

func TestCreateAndGetAccount(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	owner := mustUser(t, s, "张三")

	created, err := s.CreateAccount(ctx, AccountInput{
		Name:      "主播号",
		UID:       "12345",
		Cookie:    "SESSDATA=abc; bili_jct=def",
		RateLimit: 1500 * time.Millisecond,
		MaxLength: 40,
		OwnerID:   owner,
	})
	if err != nil {
		t.Fatalf("创建账号报错: %v", err)
	}
	if created.ID == 0 {
		t.Error("新账号应有非零 ID")
	}

	got, err := s.GetAccountByName(ctx, "主播号")
	if err != nil {
		t.Fatalf("查询账号报错: %v", err)
	}
	if got.Cookie != "SESSDATA=abc; bili_jct=def" {
		t.Errorf("Cookie = %q", got.Cookie)
	}
	if got.RateLimit != 1500*time.Millisecond {
		t.Errorf("RateLimit = %v, 期望 1.5s", got.RateLimit)
	}
	if got.UID != "12345" || got.MaxLength != 40 || got.OwnerID != owner {
		t.Errorf("账号 = %+v", got)
	}
}

func TestCreateAccountAppliesDefaults(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	owner := mustUser(t, s, "张三")

	a, err := s.CreateAccount(ctx, AccountInput{
		Name: "小号", Cookie: "c", OwnerID: owner,
	})
	if err != nil {
		t.Fatalf("创建账号报错: %v", err)
	}
	if a.RateLimit != 1500*time.Millisecond {
		t.Errorf("未指定时 RateLimit = %v, 期望默认 1.5s", a.RateLimit)
	}
	if a.MaxLength != 40 {
		t.Errorf("未指定时 MaxLength = %d, 期望默认 40（B 站上限）", a.MaxLength)
	}
}

func TestGetAccountNotFound(t *testing.T) {
	s := testStore(t)
	_, err := s.GetAccountByName(context.Background(), "没这个号")
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("应返回 ErrNotFound，实际: %v", err)
	}
}

func TestCreateAccountRejectsDuplicateName(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	owner := mustUser(t, s, "张三")

	in := AccountInput{Name: "主播号", Cookie: "c", OwnerID: owner}
	if _, err := s.CreateAccount(ctx, in); err != nil {
		t.Fatalf("首次创建报错: %v", err)
	}
	if _, err := s.CreateAccount(ctx, in); !errors.Is(err, ErrDuplicate) {
		t.Errorf("重名应返回 ErrDuplicate，实际: %v", err)
	}
}

func TestCreateAccountRejectsEmptyCookie(t *testing.T) {
	s := testStore(t)
	owner := mustUser(t, s, "张三")
	_, err := s.CreateAccount(context.Background(), AccountInput{
		Name: "主播号", OwnerID: owner,
	})
	if err == nil {
		t.Error("空 Cookie 应被拒绝：没有 Cookie 的账号什么都干不了")
	}
}

func TestUpsertAccountUpdatesInsteadOfFailing(t *testing.T) {
	// import 要能反复跑，同一份 YAML 导两次结果必须一致
	s := testStore(t)
	ctx := context.Background()
	owner := mustUser(t, s, "张三")

	first, err := s.UpsertAccount(ctx, AccountInput{
		Name: "主播号", Cookie: "old", RateLimit: time.Second, OwnerID: owner,
	})
	if err != nil {
		t.Fatalf("首次 upsert 报错: %v", err)
	}
	second, err := s.UpsertAccount(ctx, AccountInput{
		Name: "主播号", Cookie: "new", RateLimit: 2 * time.Second, OwnerID: owner,
	})
	if err != nil {
		t.Fatalf("二次 upsert 报错: %v", err)
	}

	if first.ID != second.ID {
		t.Errorf("upsert 应更新同一行，ID 从 %d 变成了 %d", first.ID, second.ID)
	}
	if second.Cookie != "new" || second.RateLimit != 2*time.Second {
		t.Errorf("upsert 后 = %+v", second)
	}

	all, err := s.ListAccounts(ctx)
	if err != nil {
		t.Fatalf("列出账号报错: %v", err)
	}
	if len(all) != 1 {
		t.Errorf("账号数 = %d, 期望 1", len(all))
	}
}

// import 走的 YAML 里没有 uid 字段：UpsertAccount 传空 UID 时不该把
// login --save 扫码写入的 UID 抹掉。「先扫码入库、再导 YAML 补规则」
// 正是文档推荐的流程。
func TestUpsertAccountPreservesUIDWhenNotProvided(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	owner := mustUser(t, s, "张三")

	first, err := s.UpsertAccount(ctx, AccountInput{
		Name: "主播号", Cookie: "old", UID: "20285041", OwnerID: owner,
	})
	if err != nil {
		t.Fatalf("首次 upsert 报错: %v", err)
	}
	if first.UID != "20285041" {
		t.Fatalf("首次 upsert 后 UID = %q", first.UID)
	}

	// 模拟 import：不带 UID
	second, err := s.UpsertAccount(ctx, AccountInput{
		Name: "主播号", Cookie: "new", OwnerID: owner,
	})
	if err != nil {
		t.Fatalf("二次 upsert 报错: %v", err)
	}
	if second.UID != "20285041" {
		t.Errorf("空 UID 的 upsert 不该抹掉已有 UID，实际变成了 %q", second.UID)
	}
	if second.Cookie != "new" {
		t.Errorf("Cookie 仍应正常更新，实际 %q", second.Cookie)
	}
}

func TestUpdateAccountCookie(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	owner := mustUser(t, s, "张三")

	if _, err := s.CreateAccount(ctx, AccountInput{
		Name: "主播号", Cookie: "old", UID: "1", OwnerID: owner,
	}); err != nil {
		t.Fatalf("创建账号报错: %v", err)
	}

	if err := s.UpdateAccountCookie(ctx, "主播号", "fresh", "999"); err != nil {
		t.Fatalf("更新 Cookie 报错: %v", err)
	}
	got, err := s.GetAccountByName(ctx, "主播号")
	if err != nil {
		t.Fatalf("查询账号报错: %v", err)
	}
	if got.Cookie != "fresh" || got.UID != "999" {
		t.Errorf("更新后 = %+v", got)
	}
}

func TestUpdateAccountCookieOnMissingAccount(t *testing.T) {
	s := testStore(t)
	err := s.UpdateAccountCookie(context.Background(), "没这个号", "c", "1")
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("应返回 ErrNotFound，实际: %v", err)
	}
}

func TestListAccountsOrderedByID(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	owner := mustUser(t, s, "张三")

	for _, n := range []string{"主播号", "小号", "大号"} {
		if _, err := s.CreateAccount(ctx, AccountInput{
			Name: n, Cookie: "c", OwnerID: owner,
		}); err != nil {
			t.Fatalf("创建 %s 报错: %v", n, err)
		}
	}
	as, err := s.ListAccounts(ctx)
	if err != nil {
		t.Fatalf("列出账号报错: %v", err)
	}
	if len(as) != 3 || as[0].Name != "主播号" || as[2].Name != "大号" {
		t.Errorf("列表 = %+v", as)
	}
}

func TestDeleteAccount(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	owner := mustUser(t, s, "张三")

	if _, err := s.CreateAccount(ctx, AccountInput{
		Name: "主播号", Cookie: "c", OwnerID: owner,
	}); err != nil {
		t.Fatalf("创建账号报错: %v", err)
	}
	if err := s.DeleteAccount(ctx, "主播号"); err != nil {
		t.Fatalf("删除账号报错: %v", err)
	}
	if _, err := s.GetAccountByName(ctx, "主播号"); !errors.Is(err, ErrNotFound) {
		t.Errorf("删除后应查不到，实际: %v", err)
	}
}

func TestDeleteMissingAccount(t *testing.T) {
	s := testStore(t)
	if err := s.DeleteAccount(context.Background(), "没这个号"); !errors.Is(err, ErrNotFound) {
		t.Errorf("应返回 ErrNotFound，实际: %v", err)
	}
}

// 新建账号从未被检测过，登录态应是三态里的「未知」，而不是默认判定为有效
// 或失效——这两个判断都没有依据。
func TestCreateAccountDefaultsToUnknownLoginState(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	owner := mustUser(t, s, "张三")

	a, err := s.CreateAccount(ctx, AccountInput{Name: "主播号", Cookie: "c", OwnerID: owner})
	if err != nil {
		t.Fatalf("创建账号报错: %v", err)
	}
	if a.LoginState != LoginStateUnknown {
		t.Errorf("LoginState = %q, 期望 %q", a.LoginState, LoginStateUnknown)
	}
	if a.LoginCheckedAt != nil {
		t.Errorf("从未检测过，LoginCheckedAt 应为 nil，实际 %v", a.LoginCheckedAt)
	}
}

func TestUpdateAccountLoginStateWritesStateAndTimestamp(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	owner := mustUser(t, s, "张三")

	if _, err := s.CreateAccount(ctx, AccountInput{Name: "主播号", Cookie: "c", OwnerID: owner}); err != nil {
		t.Fatalf("创建账号报错: %v", err)
	}

	before := time.Now()
	if err := s.UpdateAccountLoginState(ctx, "主播号", LoginStateValid); err != nil {
		t.Fatalf("写入登录态报错: %v", err)
	}

	got, err := s.GetAccountByName(ctx, "主播号")
	if err != nil {
		t.Fatalf("查询账号报错: %v", err)
	}
	if got.LoginState != LoginStateValid {
		t.Errorf("LoginState = %q, 期望 %q", got.LoginState, LoginStateValid)
	}
	if got.LoginCheckedAt == nil || got.LoginCheckedAt.Before(before.Add(-time.Second)) {
		t.Errorf("LoginCheckedAt = %v, 期望接近 %v", got.LoginCheckedAt, before)
	}

	// 再写一次 invalid，验证状态可以被覆盖（不是只能单向流转）
	if err := s.UpdateAccountLoginState(ctx, "主播号", LoginStateInvalid); err != nil {
		t.Fatalf("二次写入登录态报错: %v", err)
	}
	got2, err := s.GetAccountByName(ctx, "主播号")
	if err != nil {
		t.Fatalf("查询账号报错: %v", err)
	}
	if got2.LoginState != LoginStateInvalid {
		t.Errorf("LoginState = %q, 期望 %q", got2.LoginState, LoginStateInvalid)
	}
}

func TestUpdateAccountLoginStateRejectsUnknownValue(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	owner := mustUser(t, s, "张三")
	if _, err := s.CreateAccount(ctx, AccountInput{Name: "主播号", Cookie: "c", OwnerID: owner}); err != nil {
		t.Fatalf("创建账号报错: %v", err)
	}

	if err := s.UpdateAccountLoginState(ctx, "主播号", "已过期"); err == nil {
		t.Error("非法的登录态取值应被拒绝")
	}
}

func TestUpdateAccountLoginStateOnMissingAccount(t *testing.T) {
	// 检测循环遍历账号列表期间账号被删掉是正常竞态，不应报错——
	// UpdateAccountCookie 遇到不存在的账号会报错是因为那是用户显式操作，
	// 这里是后台轮询，语义不同。
	s := testStore(t)
	if err := s.UpdateAccountLoginState(context.Background(), "没这个号", LoginStateValid); err != nil {
		t.Errorf("账号已不存在不应视为错误，实际: %v", err)
	}
}

// 换 Cookie 要把登录态重置为未知。
//
// 不重置的话，一个被标成「已失效」的账号重新扫码之后界面仍显示
// 「已失效」直到下一轮检测（最长 10 分钟）——用户刚扫完码却看到
// 「已失效」，会以为没成功又扫一遍。
func TestUpdateAccountCookieResetsLoginState(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()

	owner := mustUser(t, s, "张三")
	if _, err := s.CreateAccount(ctx, AccountInput{
		Name: "小号", Cookie: "old", OwnerID: owner,
	}); err != nil {
		t.Fatalf("建账号报错: %v", err)
	}
	if err := s.UpdateAccountLoginState(ctx, "小号", LoginStateInvalid); err != nil {
		t.Fatalf("写登录态报错: %v", err)
	}

	if err := s.UpdateAccountCookie(ctx, "小号", "SESSDATA=new", "10086"); err != nil {
		t.Fatalf("换 Cookie 报错: %v", err)
	}

	acc, err := s.GetAccountByName(ctx, "小号")
	if err != nil {
		t.Fatalf("查账号报错: %v", err)
	}
	if acc.LoginState != LoginStateUnknown {
		t.Errorf("换 Cookie 后 LoginState = %q, 期望 %q（新 Cookie 还没被探测过）",
			acc.LoginState, LoginStateUnknown)
	}
	if acc.LoginCheckedAt != nil {
		t.Errorf("换 Cookie 后 LoginCheckedAt 应为 nil，实际 %v", acc.LoginCheckedAt)
	}
}
