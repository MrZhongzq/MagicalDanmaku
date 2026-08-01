package httpapi_test

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/MrZhongzq/MagicalDanmaku/server/internal/perm"
	"github.com/MrZhongzq/MagicalDanmaku/server/internal/store"
)

func grantMemberManage(t *testing.T, st *store.Store, user, account, room string) {
	t.Helper()
	if err := st.Grant(context.Background(), user, account, room,
		[]perm.Permission{perm.MemberManage}); err != nil {
		t.Fatalf("授权报错: %v", err)
	}
}

func TestGrantAndListMembers(t *testing.T) {
	srv, st := newTestServer(t)
	c := loginAs(t, srv, st, "张三", false)
	bid := mustBindingFor(t, st, "张三", "小号", "123")
	grantMemberManage(t, st, "张三", "小号", "123")
	loginAs(t, srv, st, "李四", false)

	put := jsonRequest(t, c, "PUT", srv.URL+"/api/bindings/"+itoa(bid)+"/members/李四",
		`{"permissions":["rule:read","rule:write"]}`)
	defer put.Body.Close()
	if put.StatusCode != http.StatusOK {
		t.Fatalf("状态码 = %d", put.StatusCode)
	}

	list := jsonRequest(t, c, "GET", srv.URL+"/api/bindings/"+itoa(bid)+"/members", "")
	defer list.Body.Close()

	var got []struct {
		Username    string   `json:"username"`
		Permissions []string `json:"permissions"`
	}
	if err := json.NewDecoder(list.Body).Decode(&got); err != nil {
		t.Fatalf("解析报错: %v", err)
	}
	// 张三自己也有 member:manage，所以是两条
	found := false
	for _, m := range got {
		if m.Username == "李四" {
			found = true
			if len(m.Permissions) != 2 {
				t.Errorf("李四的权限点 = %v", m.Permissions)
			}
		}
	}
	if !found {
		t.Errorf("成员列表里没有李四: %+v", got)
	}
}

// 授权是替换而非累加：重新授权的语义是「设定为这些」
func TestGrantReplacesNotAccumulates(t *testing.T) {
	srv, st := newTestServer(t)
	c := loginAs(t, srv, st, "张三", false)
	bid := mustBindingFor(t, st, "张三", "小号", "123")
	grantMemberManage(t, st, "张三", "小号", "123")
	li := loginAs(t, srv, st, "李四", false)

	r1 := jsonRequest(t, c, "PUT", srv.URL+"/api/bindings/"+itoa(bid)+"/members/李四",
		`{"permissions":["rule:read","rule:write"]}`)
	r1.Body.Close()
	r2 := jsonRequest(t, c, "PUT", srv.URL+"/api/bindings/"+itoa(bid)+"/members/李四",
		`{"permissions":["event:read"]}`)
	r2.Body.Close()

	// 李四现在应该只有 event:read
	resp := jsonRequest(t, li, "GET", srv.URL+"/api/bindings/"+itoa(bid)+"/rules", "")
	resp.Body.Close()
	if resp.StatusCode != http.StatusForbidden {
		t.Errorf("rule:read 应已被撤销，读规则状态码 = %d, 期望 403", resp.StatusCode)
	}
}

func TestGrantRejectsUnknownPermission(t *testing.T) {
	srv, st := newTestServer(t)
	c := loginAs(t, srv, st, "张三", false)
	bid := mustBindingFor(t, st, "张三", "小号", "123")
	grantMemberManage(t, st, "张三", "小号", "123")
	loginAs(t, srv, st, "李四", false)

	resp := jsonRequest(t, c, "PUT", srv.URL+"/api/bindings/"+itoa(bid)+"/members/李四",
		`{"permissions":["rule:delete"]}`)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusUnprocessableEntity {
		t.Errorf("状态码 = %d, 期望 422", resp.StatusCode)
	}
	var body map[string]string
	_ = json.NewDecoder(resp.Body).Decode(&body)
	// 错误信息要列出合法值，否则用户只能猜
	if body["error"] == "" {
		t.Error("应给出错误说明")
	}
}

func TestGrantRejectsEmptyPermissions(t *testing.T) {
	srv, st := newTestServer(t)
	c := loginAs(t, srv, st, "张三", false)
	bid := mustBindingFor(t, st, "张三", "小号", "123")
	grantMemberManage(t, st, "张三", "小号", "123")
	loginAs(t, srv, st, "李四", false)

	resp := jsonRequest(t, c, "PUT", srv.URL+"/api/bindings/"+itoa(bid)+"/members/李四",
		`{"permissions":[]}`)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusUnprocessableEntity {
		t.Errorf("空权限列表状态码 = %d, 期望 422（语义含糊，应显式撤销）", resp.StatusCode)
	}
}

func TestRevokeMember(t *testing.T) {
	srv, st := newTestServer(t)
	c := loginAs(t, srv, st, "张三", false)
	bid := mustBindingFor(t, st, "张三", "小号", "123")
	grantMemberManage(t, st, "张三", "小号", "123")
	li := loginAs(t, srv, st, "李四", false)

	put := jsonRequest(t, c, "PUT", srv.URL+"/api/bindings/"+itoa(bid)+"/members/李四",
		`{"permissions":["rule:read"]}`)
	put.Body.Close()

	del := jsonRequest(t, c, "DELETE", srv.URL+"/api/bindings/"+itoa(bid)+"/members/李四", "")
	defer del.Body.Close()
	if del.StatusCode != http.StatusOK {
		t.Fatalf("状态码 = %d", del.StatusCode)
	}

	// 撤销的是李四**最后一条**授权，所以他对这个绑定的可见性也一并没了。
	// 守卫按设计返回 404 而不是 403——「不存在」与「对你不可见」不可区分，
	// 否则拿绑定 ID 递增就能枚举出部署里有哪些绑定（设计文档 §5）。
	//
	// 想测 403 得让他还剩一条无关的授权。那条路径由
	// TestGrantReplacesNotAccumulates（重新授权成只剩 event:read 之后
	// 读规则拿 403）与 TestMemberEndpointsRequireMemberManage（持有
	// rule:write 却访问 member 端点拿 403）覆盖。
	resp := jsonRequest(t, li, "GET", srv.URL+"/api/bindings/"+itoa(bid)+"/rules", "")
	resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("撤光全部授权后状态码 = %d, 期望 404（连可见性都没了）", resp.StatusCode)
	}
}

func TestMemberEndpointsRequireMemberManage(t *testing.T) {
	srv, st := newTestServer(t)
	loginAs(t, srv, st, "张三", false)
	bid := mustBindingFor(t, st, "张三", "小号", "123")

	li := loginAs(t, srv, st, "李四", false)
	if err := st.Grant(context.Background(), "李四", "小号", "123",
		[]perm.Permission{perm.RuleWrite}); err != nil {
		t.Fatalf("授权报错: %v", err)
	}

	base := srv.URL + "/api/bindings/" + itoa(bid) + "/members"
	for _, tc := range []struct{ method, url, body string }{
		{"GET", base, ""},
		{"PUT", base + "/王五", `{"permissions":["rule:read"]}`},
		{"DELETE", base + "/王五", ""},
	} {
		resp := jsonRequest(t, li, tc.method, tc.url, tc.body)
		resp.Body.Close()
		if resp.StatusCode != http.StatusForbidden {
			t.Errorf("%s %s 状态码 = %d, 期望 403", tc.method, tc.url, resp.StatusCode)
		}
	}
}

// 给不存在的用户授权要 404 而不是静默成功
func TestGrantUnknownUser(t *testing.T) {
	srv, st := newTestServer(t)
	c := loginAs(t, srv, st, "张三", false)
	bid := mustBindingFor(t, st, "张三", "小号", "123")
	grantMemberManage(t, st, "张三", "小号", "123")

	resp := jsonRequest(t, c, "PUT", srv.URL+"/api/bindings/"+itoa(bid)+"/members/查无此人",
		`{"permissions":["rule:read"]}`)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("状态码 = %d, 期望 404", resp.StatusCode)
	}
}
