package httpapi_test

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
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

// 李四对这个绑定毫无可见性：不是所有者、没有任何授权记录、不是管理员。
//
// 期望值是 404 而不是 403：设计文档 §5 要求「不存在」与「对调用者
// 不可见」返回不可区分的响应，否则调用者能靠 403 vs 404 的差异
// 把 {binding} 从 1 遍历到 N，收集出全部账号名与房间号。
// 这条测试原先断言 403，是本次修复前遗留的错误期望值。
func TestGuardRejectsWithoutPermission(t *testing.T) {
	srv, st := newTestServer(t)
	owner := loginAs(t, srv, st, "张三", false)
	_ = owner
	bid := mustBindingFor(t, st, "张三", "小号", "123")

	// 李四没有任何授权
	li := loginAs(t, srv, st, "李四", false)
	resp := jsonRequest(t, li, "GET", srv.URL+"/api/test/guarded/"+itoa(bid), "")
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("无可见性访问状态码 = %d, 期望 404", resp.StatusCode)
	}
	var body map[string]string
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("解析报错: %v", err)
	}
	if strings.Contains(body["error"], "小号") || strings.Contains(body["error"], "123") {
		t.Errorf("404 响应体不该泄漏账号名或房间号: %q", body["error"])
	}
}

// 在其他绑定上有授权，不代表对这个绑定有可见性；那种用户访问应该
// 和完全陌生的用户一样收到 404，且响应体不带任何标签信息。
func TestGuardNoVisibilityOnThisBindingHidesLabel(t *testing.T) {
	srv, st := newTestServer(t)
	loginAs(t, srv, st, "张三", false)
	loginAs(t, srv, st, "王五", false)
	bidZhang := mustBindingFor(t, st, "张三", "张三的号", "111")
	mustBindingFor(t, st, "王五", "王五的号", "222")

	li := loginAs(t, srv, st, "李四", false)
	if err := st.Grant(context.Background(), "李四", "王五的号", "222",
		[]perm.Permission{perm.RuleRead}); err != nil {
		t.Fatalf("授权报错: %v", err)
	}

	// 李四对张三的绑定毫无可见性（他被授权的是王五的绑定）
	resp := jsonRequest(t, li, "GET", srv.URL+"/api/test/guarded/"+itoa(bidZhang), "")
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("状态码 = %d, 期望 404", resp.StatusCode)
	}
	var body map[string]string
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("解析报错: %v", err)
	}
	if strings.Contains(body["error"], "张三的号") || strings.Contains(body["error"], "111") {
		t.Errorf("404 响应体泄漏了账号名或房间号: %q", body["error"])
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

// 处理器自己写的「绑定不存在」文案不能被 withNotFoundJSON 中间件
// 改写成「接口 GET /xxx 不存在」。中间件只应替换 ServeMux 兜底的
// 纯文本 404/405，不该吞掉处理器已经写好的响应体。
func TestGuardUnknownBindingKeepsHandlerMessage(t *testing.T) {
	srv, st := newTestServer(t)
	admin := loginAs(t, srv, st, "管理员", true)

	resp := jsonRequest(t, admin, "GET", srv.URL+"/api/test/guarded/999999", "")
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("状态码 = %d, 期望 404", resp.StatusCode)
	}
	var body map[string]string
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("解析报错: %v", err)
	}
	if !strings.Contains(body["error"], "绑定") {
		t.Errorf("响应体应包含处理器自己写的「绑定」文案，实际 = %q", body["error"])
	}
	if strings.Contains(body["error"], "接口") {
		t.Errorf("响应体被中间件改写成了「接口不存在」，处理器的文案丢了: %q", body["error"])
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
