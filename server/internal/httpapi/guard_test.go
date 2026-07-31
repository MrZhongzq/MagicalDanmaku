package httpapi_test

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"testing"
	"time"

	"github.com/MrZhongzq/MagicalDanmaku/server/internal/perm"
	"github.com/MrZhongzq/MagicalDanmaku/server/internal/store"
)

// mustBindingFor 建「用户拥有的账号 + 直播间」，返回绑定 ID。
func mustBindingFor(t *testing.T, st *store.Store, ownerName, accountName, roomID string) int64 {
	t.Helper()
	ctx := context.Background()
	owner, err := st.GetUserByName(ctx, ownerName)
	if err != nil {
		t.Fatalf("查用户 %s 报错: %v", ownerName, err)
	}
	acc, err := st.CreateAccount(ctx, store.AccountInput{
		Name: accountName, Cookie: "SESSDATA=x", OwnerID: owner.ID,
		RateLimit: time.Second, MaxLength: 40,
	})
	if err != nil {
		t.Fatalf("建账号 %s 报错: %v", accountName, err)
	}
	b, err := st.UpsertBinding(ctx, acc.ID, roomID)
	if err != nil {
		t.Fatalf("建绑定报错: %v", err)
	}
	return b.ID
}

func TestGuardRejectsWithoutPermission(t *testing.T) {
	srv, st := newTestServer(t)
	owner := loginAs(t, srv, st, "张三", false)
	_ = owner
	bid := mustBindingFor(t, st, "张三", "小号", "123")

	// 李四没有任何授权
	li := loginAs(t, srv, st, "李四", false)
	resp := jsonRequest(t, li, "GET", srv.URL+"/api/test/guarded/"+itoa(bid), "")
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusForbidden {
		t.Errorf("无授权访问状态码 = %d, 期望 403", resp.StatusCode)
	}
}

func TestGuardAllowsWithPermission(t *testing.T) {
	srv, st := newTestServer(t)
	loginAs(t, srv, st, "张三", false)
	bid := mustBindingFor(t, st, "张三", "小号", "123")

	li := loginAs(t, srv, st, "李四", false)
	if err := st.Grant(context.Background(), "李四", "小号", "123",
		[]perm.Permission{perm.RuleRead}); err != nil {
		t.Fatalf("授权报错: %v", err)
	}

	resp := jsonRequest(t, li, "GET", srv.URL+"/api/test/guarded/"+itoa(bid), "")
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("有授权访问状态码 = %d, 期望 200", resp.StatusCode)
	}
}

func TestGuardAllowsAdmin(t *testing.T) {
	srv, st := newTestServer(t)
	loginAs(t, srv, st, "张三", false)
	bid := mustBindingFor(t, st, "张三", "小号", "123")

	admin := loginAs(t, srv, st, "管理员", true)
	resp := jsonRequest(t, admin, "GET", srv.URL+"/api/test/guarded/"+itoa(bid), "")
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("管理员访问状态码 = %d, 期望 200", resp.StatusCode)
	}
}

func TestGuardRequiresAuth(t *testing.T) {
	srv, st := newTestServer(t)
	loginAs(t, srv, st, "张三", false)
	bid := mustBindingFor(t, st, "张三", "小号", "123")

	resp, err := http.Get(srv.URL + "/api/test/guarded/" + itoa(bid))
	if err != nil {
		t.Fatalf("请求报错: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("未登录状态码 = %d, 期望 401", resp.StatusCode)
	}
}

// 不存在的绑定必须 404 而不是 403 或 500
func TestGuardUnknownBinding(t *testing.T) {
	srv, st := newTestServer(t)
	admin := loginAs(t, srv, st, "管理员", true)

	resp := jsonRequest(t, admin, "GET", srv.URL+"/api/test/guarded/999999", "")
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("不存在的绑定状态码 = %d, 期望 404", resp.StatusCode)
	}
}

func TestGuardMalformedBindingID(t *testing.T) {
	srv, st := newTestServer(t)
	admin := loginAs(t, srv, st, "管理员", true)

	resp := jsonRequest(t, admin, "GET", srv.URL+"/api/test/guarded/不是数字", "")
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound && resp.StatusCode != http.StatusUnprocessableEntity {
		t.Errorf("非法绑定 ID 状态码 = %d, 期望 404 或 422", resp.StatusCode)
	}
}

// 守卫按权限点区分：有 rule:read 不代表能过 rule:write 的守卫
func TestGuardDistinguishesPermissions(t *testing.T) {
	srv, st := newTestServer(t)
	loginAs(t, srv, st, "张三", false)
	bid := mustBindingFor(t, st, "张三", "小号", "123")

	li := loginAs(t, srv, st, "李四", false)
	if err := st.Grant(context.Background(), "李四", "小号", "123",
		[]perm.Permission{perm.RuleRead}); err != nil {
		t.Fatalf("授权报错: %v", err)
	}

	ok := jsonRequest(t, li, "GET", srv.URL+"/api/test/guarded/"+itoa(bid), "")
	ok.Body.Close()
	if ok.StatusCode != http.StatusOK {
		t.Errorf("rule:read 守卫应放行，状态码 = %d", ok.StatusCode)
	}

	no := jsonRequest(t, li, "POST", srv.URL+"/api/test/guarded-write/"+itoa(bid), "{}")
	no.Body.Close()
	if no.StatusCode != http.StatusForbidden {
		t.Errorf("rule:write 守卫应拒绝，状态码 = %d", no.StatusCode)
	}
}

// 可见范围：只能看到自己拥有的账号，以及自己在其某个绑定上有授权的账号
func TestVisibleBindingsFiltersByOwnershipAndGrant(t *testing.T) {
	srv, st := newTestServer(t)
	ctx := context.Background()

	loginAs(t, srv, st, "张三", false)
	loginAs(t, srv, st, "王五", false)
	mustBindingFor(t, st, "张三", "张三的号", "111")
	mustBindingFor(t, st, "王五", "王五的号", "222")

	li := loginAs(t, srv, st, "李四", false)
	if err := st.Grant(ctx, "李四", "张三的号", "111",
		[]perm.Permission{perm.EventRead}); err != nil {
		t.Fatalf("授权报错: %v", err)
	}

	resp := jsonRequest(t, li, "GET", srv.URL+"/api/test/visible-bindings", "")
	defer resp.Body.Close()

	var got []struct {
		AccountName string `json:"accountName"`
		RoomID      string `json:"roomId"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&got); err != nil {
		t.Fatalf("解析报错: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("李四应只看到 1 个绑定，实际 %d 个: %+v", len(got), got)
	}
	if got[0].AccountName != "张三的号" || got[0].RoomID != "111" {
		t.Errorf("看到的绑定 = %+v", got[0])
	}
}

func TestVisibleBindingsAdminSeesAll(t *testing.T) {
	srv, st := newTestServer(t)
	loginAs(t, srv, st, "张三", false)
	loginAs(t, srv, st, "王五", false)
	mustBindingFor(t, st, "张三", "张三的号", "111")
	mustBindingFor(t, st, "王五", "王五的号", "222")

	admin := loginAs(t, srv, st, "管理员", true)
	resp := jsonRequest(t, admin, "GET", srv.URL+"/api/test/visible-bindings", "")
	defer resp.Body.Close()

	var got []any
	if err := json.NewDecoder(resp.Body).Decode(&got); err != nil {
		t.Fatalf("解析报错: %v", err)
	}
	if len(got) != 2 {
		t.Errorf("管理员应看到全部 2 个绑定，实际 %d 个", len(got))
	}
}

func TestVisibleBindingsOwnerSeesOwn(t *testing.T) {
	srv, st := newTestServer(t)
	zhang := loginAs(t, srv, st, "张三", false)
	loginAs(t, srv, st, "王五", false)
	mustBindingFor(t, st, "张三", "张三的号", "111")
	mustBindingFor(t, st, "王五", "王五的号", "222")

	resp := jsonRequest(t, zhang, "GET", srv.URL+"/api/test/visible-bindings", "")
	defer resp.Body.Close()

	var got []struct {
		AccountName string `json:"accountName"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&got); err != nil {
		t.Fatalf("解析报错: %v", err)
	}
	if len(got) != 1 || got[0].AccountName != "张三的号" {
		t.Errorf("张三应只看到自己的账号，实际 %+v", got)
	}
}

// itoa 是 strconv.FormatInt 的短名，测试里到处用
func itoa(v int64) string { return strconvFormat(v) }

func strconvFormat(v int64) string {
	return strconv.FormatInt(v, 10)
}
