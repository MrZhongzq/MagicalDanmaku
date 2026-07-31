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
