package httpapi_test

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/MrZhongzq/MagicalDanmaku/server/internal/perm"
)

type bindingView struct {
	ID          int64    `json:"id"`
	AccountName string   `json:"accountName"`
	RoomID      string   `json:"roomId"`
	Enabled     bool     `json:"enabled"`
	RuleCount   int      `json:"ruleCount"`
	Permissions []string `json:"permissions"`
}

func TestListBindingsIncludesCallerPermissions(t *testing.T) {
	srv, st := newTestServer(t)
	loginAs(t, srv, st, "张三", false)
	mustBindingFor(t, st, "张三", "小号", "123")

	li := loginAs(t, srv, st, "李四", false)
	if err := st.Grant(context.Background(), "李四", "小号", "123",
		[]perm.Permission{perm.RuleRead, perm.EventRead}); err != nil {
		t.Fatalf("授权报错: %v", err)
	}

	resp := jsonRequest(t, li, "GET", srv.URL+"/api/bindings", "")
	defer resp.Body.Close()

	var got []bindingView
	if err := json.NewDecoder(resp.Body).Decode(&got); err != nil {
		t.Fatalf("解析报错: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("绑定数 = %d, 期望 1", len(got))
	}
	have := map[string]bool{}
	for _, p := range got[0].Permissions {
		have[p] = true
	}
	if !have["rule:read"] || !have["event:read"] {
		t.Errorf("权限点 = %v, 期望含 rule:read 与 event:read", got[0].Permissions)
	}
	if have["rule:write"] {
		t.Errorf("不该有 rule:write: %v", got[0].Permissions)
	}
}

func TestListBindingsAdminGetsAllPermissions(t *testing.T) {
	srv, st := newTestServer(t)
	loginAs(t, srv, st, "张三", false)
	mustBindingFor(t, st, "张三", "小号", "123")

	admin := loginAs(t, srv, st, "管理员", true)
	resp := jsonRequest(t, admin, "GET", srv.URL+"/api/bindings", "")
	defer resp.Body.Close()

	var got []bindingView
	if err := json.NewDecoder(resp.Body).Decode(&got); err != nil {
		t.Fatalf("解析报错: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("绑定数 = %d", len(got))
	}
	if len(got[0].Permissions) != len(perm.All()) {
		t.Errorf("管理员应拿到全部 %d 个权限点，实际 %d 个",
			len(perm.All()), len(got[0].Permissions))
	}
}

// 所有者在自己的绑定上应看到除 member:manage 外的全部权限点。
//
// 若这里返回 []，前端会把按钮全灰掉，而 PATCH 其实能成——
// 「列表说没权限、请求却成了」比直接报错更难查。
//
// member:manage 必须显式断言不在里面：只对个数比对，会被将来
// 加权限点时的巧合蒙混过去。把第三方拉进授权体系是管理员级别的
// 决定，不是账号所有权的附带品。
func TestListBindingsGivesOwnerAllPermissionsExceptMemberManage(t *testing.T) {
	srv, st := newTestServer(t)
	c := loginAs(t, srv, st, "张三", false)
	mustBindingFor(t, st, "张三", "小号", "123")

	resp := jsonRequest(t, c, "GET", srv.URL+"/api/bindings", "")
	defer resp.Body.Close()

	var got []bindingView
	if err := json.NewDecoder(resp.Body).Decode(&got); err != nil {
		t.Fatalf("解析报错: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("绑定数 = %d, 期望 1", len(got))
	}
	if len(got[0].Permissions) != len(perm.All())-1 {
		t.Errorf("所有者的权限点 = %v, 期望 %d 个（全部减去 member:manage）",
			got[0].Permissions, len(perm.All())-1)
	}
	for _, p := range got[0].Permissions {
		if p == string(perm.MemberManage) {
			t.Errorf("所有者不该凭所有权获得 member:manage: %v", got[0].Permissions)
		}
	}
}

func TestCreateBinding(t *testing.T) {
	srv, st := newTestServer(t)
	c := loginAs(t, srv, st, "张三", false)
	mustBindingFor(t, st, "张三", "小号", "111")

	resp := jsonRequest(t, c, "POST", srv.URL+"/api/bindings",
		`{"accountName":"小号","roomId":"222"}`)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("状态码 = %d, 期望 201", resp.StatusCode)
	}

	if _, err := st.GetBinding(context.Background(), "小号", "222"); err != nil {
		t.Errorf("绑定应已建好: %v", err)
	}
}

func TestCreateBindingRequiresAccountOwnership(t *testing.T) {
	srv, st := newTestServer(t)
	loginAs(t, srv, st, "张三", false)
	mustBindingFor(t, st, "张三", "小号", "111")

	li := loginAs(t, srv, st, "李四", false)
	resp := jsonRequest(t, li, "POST", srv.URL+"/api/bindings",
		`{"accountName":"小号","roomId":"222"}`)
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusCreated {
		t.Error("非所有者不该能给别人的账号加直播间")
	}
}

func TestCreateBindingRejectsEmptyRoom(t *testing.T) {
	srv, st := newTestServer(t)
	c := loginAs(t, srv, st, "张三", false)
	mustBindingFor(t, st, "张三", "小号", "111")

	resp := jsonRequest(t, c, "POST", srv.URL+"/api/bindings",
		`{"accountName":"小号","roomId":"  "}`)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusUnprocessableEntity {
		t.Errorf("空房间号状态码 = %d, 期望 422", resp.StatusCode)
	}
}

// 重复创建是幂等的，不该报错——重复点一下按钮不该看到红色报错
func TestCreateBindingIsIdempotent(t *testing.T) {
	srv, st := newTestServer(t)
	c := loginAs(t, srv, st, "张三", false)
	mustBindingFor(t, st, "张三", "小号", "111")

	body := `{"accountName":"小号","roomId":"222"}`
	r1 := jsonRequest(t, c, "POST", srv.URL+"/api/bindings", body)
	r1.Body.Close()
	r2 := jsonRequest(t, c, "POST", srv.URL+"/api/bindings", body)
	defer r2.Body.Close()
	if r2.StatusCode != http.StatusCreated && r2.StatusCode != http.StatusOK {
		t.Errorf("重复创建状态码 = %d, 不该报错", r2.StatusCode)
	}

	bs, err := st.ListBindings(context.Background())
	if err != nil {
		t.Fatalf("列绑定报错: %v", err)
	}
	if len(bs) != 2 { // 111 与 222
		t.Errorf("绑定数 = %d, 期望 2（不该重复）", len(bs))
	}
}

func TestToggleBindingRequiresRuleWrite(t *testing.T) {
	srv, st := newTestServer(t)
	loginAs(t, srv, st, "张三", false)
	bid := mustBindingFor(t, st, "张三", "小号", "123")

	li := loginAs(t, srv, st, "李四", false)
	if err := st.Grant(context.Background(), "李四", "小号", "123",
		[]perm.Permission{perm.RuleRead}); err != nil {
		t.Fatalf("授权报错: %v", err)
	}

	resp := jsonRequest(t, li, "PATCH", srv.URL+"/api/bindings/"+itoa(bid), `{"enabled":false}`)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusForbidden {
		t.Errorf("只有 rule:read 时状态码 = %d, 期望 403", resp.StatusCode)
	}
}

func TestToggleBinding(t *testing.T) {
	srv, st := newTestServer(t)
	c := loginAs(t, srv, st, "张三", false)
	bid := mustBindingFor(t, st, "张三", "小号", "123")

	resp := jsonRequest(t, c, "PATCH", srv.URL+"/api/bindings/"+itoa(bid), `{"enabled":false}`)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("状态码 = %d", resp.StatusCode)
	}

	b, err := st.GetBinding(context.Background(), "小号", "123")
	if err != nil {
		t.Fatalf("查绑定报错: %v", err)
	}
	if b.Enabled {
		t.Error("应已停用")
	}
}

func TestDeleteBindingByOwner(t *testing.T) {
	srv, st := newTestServer(t)
	c := loginAs(t, srv, st, "张三", false)
	bid := mustBindingFor(t, st, "张三", "小号", "123")

	resp := jsonRequest(t, c, "DELETE", srv.URL+"/api/bindings/"+itoa(bid), "")
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("状态码 = %d", resp.StatusCode)
	}
	if _, err := st.GetBinding(context.Background(), "小号", "123"); err == nil {
		t.Error("删除后应查不到")
	}
}

// 有 rule:write 不等于能删绑定——删绑定会带走全部规则与授权，
// 是账号所有权级别的操作
func TestDeleteBindingRequiresOwnership(t *testing.T) {
	srv, st := newTestServer(t)
	loginAs(t, srv, st, "张三", false)
	bid := mustBindingFor(t, st, "张三", "小号", "123")

	li := loginAs(t, srv, st, "李四", false)
	if err := st.Grant(context.Background(), "李四", "小号", "123",
		[]perm.Permission{perm.RuleWrite}); err != nil {
		t.Fatalf("授权报错: %v", err)
	}

	resp := jsonRequest(t, li, "DELETE", srv.URL+"/api/bindings/"+itoa(bid), "")
	defer resp.Body.Close()
	// 403 而不是 404：李四对这个绑定有可见性（他有 rule:write），
	// 告诉他「存在但你不是所有者」不算泄漏
	if resp.StatusCode != http.StatusForbidden {
		t.Errorf("有 rule:write 时状态码 = %d, 期望 403", resp.StatusCode)
	}
	if _, err := st.GetBinding(context.Background(), "小号", "123"); err != nil {
		t.Error("绑定不该被删掉")
	}
}

// 与这个绑定毫无关系的人删它，必须收到 404 而不是 403。
//
// DELETE 走的是 requireAuth 不是 requirePerm，没有守卫替它做可见性
// 判断。若无条件回 403，拿绑定 ID 从 1 递增试一遍就能枚举出部署里
// 有哪些绑定——「不存在」与「存在但不归你」必须不可区分。
func TestDeleteBindingByStrangerLooksLikeNotFound(t *testing.T) {
	srv, st := newTestServer(t)
	loginAs(t, srv, st, "张三", false)
	bid := mustBindingFor(t, st, "张三", "小号", "123")

	wu := loginAs(t, srv, st, "王五", false) // 无任何授权、不拥有任何账号

	resp := jsonRequest(t, wu, "DELETE", srv.URL+"/api/bindings/"+itoa(bid), "")
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("陌生人删绑定状态码 = %d, 期望 404（403 会泄漏该绑定存在）", resp.StatusCode)
	}

	var body map[string]string
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("解析报错: %v", err)
	}
	// 文案也不能泄漏：不得出现账号名、房间号，或「所有者」这类
	// 暗示「它存在，只是不归你」的措辞
	for _, leak := range []string{"小号", "123", "所有者"} {
		if strings.Contains(body["error"], leak) {
			t.Errorf("错误文案泄漏了 %q: %q", leak, body["error"])
		}
	}

	if _, err := st.GetBinding(context.Background(), "小号", "123"); err != nil {
		t.Error("绑定不该被删掉")
	}
}
