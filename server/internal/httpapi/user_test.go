package httpapi_test

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"
)

func TestListUsersRequiresAdmin(t *testing.T) {
	srv, st := newTestServer(t)
	c := loginAs(t, srv, st, "张三", false)

	resp := jsonRequest(t, c, "GET", srv.URL+"/api/users", "")
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusForbidden {
		t.Errorf("普通用户列用户状态码 = %d, 期望 403", resp.StatusCode)
	}
}

func TestListUsersAsAdmin(t *testing.T) {
	srv, st := newTestServer(t)
	admin := loginAs(t, srv, st, "管理员", true)
	loginAs(t, srv, st, "张三", false)

	resp := jsonRequest(t, admin, "GET", srv.URL+"/api/users", "")
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("状态码 = %d", resp.StatusCode)
	}

	var got []map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&got); err != nil {
		t.Fatalf("解析报错: %v", err)
	}
	if len(got) != 2 {
		t.Errorf("用户数 = %d, 期望 2", len(got))
	}
	for _, u := range got {
		for _, k := range []string{"passwordHash", "password_hash", "password"} {
			if _, ok := u[k]; ok {
				t.Errorf("用户列表泄漏了 %q", k)
			}
		}
	}
}

func TestCreateUserAsAdmin(t *testing.T) {
	srv, st := newTestServer(t)
	admin := loginAs(t, srv, st, "管理员", true)

	resp := jsonRequest(t, admin, "POST", srv.URL+"/api/users",
		`{"username":"新人","password":"pw123456","isAdmin":false}`)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("状态码 = %d, 期望 201", resp.StatusCode)
	}

	if _, err := st.VerifyPassword(context.Background(), "新人", "pw123456"); err != nil {
		t.Errorf("新建用户应能用该密码登录: %v", err)
	}
}

func TestCreateUserRejectsDuplicate(t *testing.T) {
	srv, st := newTestServer(t)
	admin := loginAs(t, srv, st, "管理员", true)

	body := `{"username":"新人","password":"pw123456","isAdmin":false}`
	r1 := jsonRequest(t, admin, "POST", srv.URL+"/api/users", body)
	r1.Body.Close()

	r2 := jsonRequest(t, admin, "POST", srv.URL+"/api/users", body)
	defer r2.Body.Close()
	if r2.StatusCode != http.StatusConflict {
		t.Errorf("重名状态码 = %d, 期望 409", r2.StatusCode)
	}
}

func TestCreateUserRequiresAdmin(t *testing.T) {
	srv, st := newTestServer(t)
	c := loginAs(t, srv, st, "张三", false)

	resp := jsonRequest(t, c, "POST", srv.URL+"/api/users",
		`{"username":"新人","password":"pw123456","isAdmin":false}`)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusForbidden {
		t.Errorf("普通用户建用户状态码 = %d, 期望 403", resp.StatusCode)
	}
}

// 普通用户不能给自己提权
func TestCreateUserCannotSelfEscalate(t *testing.T) {
	srv, st := newTestServer(t)
	c := loginAs(t, srv, st, "张三", false)

	resp := jsonRequest(t, c, "POST", srv.URL+"/api/users",
		`{"username":"我的马甲","password":"pw123456","isAdmin":true}`)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusForbidden {
		t.Errorf("状态码 = %d, 期望 403", resp.StatusCode)
	}
}

func TestChangeOwnPasswordRequiresOldPassword(t *testing.T) {
	srv, st := newTestServer(t)
	c := loginAs(t, srv, st, "张三", false)

	bad := jsonRequest(t, c, "POST", srv.URL+"/api/users/张三/password",
		`{"oldPassword":"错的","newPassword":"新密码123"}`)
	bad.Body.Close()
	if bad.StatusCode != http.StatusUnauthorized {
		t.Errorf("旧密码错时状态码 = %d, 期望 401", bad.StatusCode)
	}

	ok := jsonRequest(t, c, "POST", srv.URL+"/api/users/张三/password",
		`{"oldPassword":"pw-张三","newPassword":"新密码123"}`)
	ok.Body.Close()
	if ok.StatusCode != http.StatusOK {
		t.Errorf("旧密码对时状态码 = %d, 期望 200", ok.StatusCode)
	}
	if _, err := st.VerifyPassword(context.Background(), "张三", "新密码123"); err != nil {
		t.Errorf("新密码应生效: %v", err)
	}
}

// 改密码必须踢掉全部旧会话——改了密码而旧会话还能用，等于没改
func TestChangePasswordInvalidatesSessions(t *testing.T) {
	srv, st := newTestServer(t)
	c := loginAs(t, srv, st, "张三", false)

	resp := jsonRequest(t, c, "POST", srv.URL+"/api/users/张三/password",
		`{"oldPassword":"pw-张三","newPassword":"新密码123"}`)
	resp.Body.Close()

	after := jsonRequest(t, c, "GET", srv.URL+"/api/auth/me", "")
	defer after.Body.Close()
	if after.StatusCode != http.StatusUnauthorized {
		t.Errorf("改密码后旧会话应失效，实际状态码 = %d", after.StatusCode)
	}
}

func TestAdminChangesOthersPasswordWithoutOld(t *testing.T) {
	srv, st := newTestServer(t)
	admin := loginAs(t, srv, st, "管理员", true)
	loginAs(t, srv, st, "张三", false)

	resp := jsonRequest(t, admin, "POST", srv.URL+"/api/users/张三/password",
		`{"newPassword":"管理员设的"}`)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("状态码 = %d", resp.StatusCode)
	}
	if _, err := st.VerifyPassword(context.Background(), "张三", "管理员设的"); err != nil {
		t.Errorf("新密码应生效: %v", err)
	}
}

// 管理员改自己的密码同样要带旧密码。
//
// 会话 Cookie 被劫持时（XSS、无人值守的浏览器），这条校验是攻击者
// 把账号永久据为己有的最后一道门槛。管理员身份不该成为豁免理由。
func TestAdminChangingOwnPasswordStillNeedsOldPassword(t *testing.T) {
	srv, st := newTestServer(t)
	admin := loginAs(t, srv, st, "管理员", true)

	bad := jsonRequest(t, admin, "POST", srv.URL+"/api/users/管理员/password",
		`{"newPassword":"不带旧密码就想改"}`)
	bad.Body.Close()
	if bad.StatusCode != http.StatusUnauthorized {
		t.Errorf("管理员改自己密码不带旧密码，状态码 = %d, 期望 401", bad.StatusCode)
	}

	// 确认密码确实没被改掉
	if _, err := st.VerifyPassword(context.Background(), "管理员", "pw-管理员"); err != nil {
		t.Errorf("原密码应仍然有效: %v", err)
	}

	ok := jsonRequest(t, admin, "POST", srv.URL+"/api/users/管理员/password",
		`{"oldPassword":"pw-管理员","newPassword":"带了旧密码的新密码"}`)
	ok.Body.Close()
	if ok.StatusCode != http.StatusOK {
		t.Errorf("带了旧密码应通过，状态码 = %d", ok.StatusCode)
	}
	if _, err := st.VerifyPassword(context.Background(), "管理员", "带了旧密码的新密码"); err != nil {
		t.Errorf("新密码应生效: %v", err)
	}
}

// 普通用户不能改别人的密码
func TestUserCannotChangeOthersPassword(t *testing.T) {
	srv, st := newTestServer(t)
	loginAs(t, srv, st, "张三", false)
	li := loginAs(t, srv, st, "李四", false)

	resp := jsonRequest(t, li, "POST", srv.URL+"/api/users/张三/password",
		`{"oldPassword":"pw-张三","newPassword":"被李四改了"}`)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusForbidden {
		t.Errorf("状态码 = %d, 期望 403", resp.StatusCode)
	}
}

func TestDeleteUser(t *testing.T) {
	srv, st := newTestServer(t)
	admin := loginAs(t, srv, st, "管理员", true)
	loginAs(t, srv, st, "路人", false)

	resp := jsonRequest(t, admin, "DELETE", srv.URL+"/api/users/路人", "")
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("状态码 = %d", resp.StatusCode)
	}
	if _, err := st.GetUserByName(context.Background(), "路人"); err == nil {
		t.Error("删除后应查不到")
	}
}

// 还拥有 B 站账号的用户删不掉：P3 的外键是 ON DELETE RESTRICT，
// 避免留下无主的 Cookie
func TestDeleteUserWithAccountsFails(t *testing.T) {
	srv, st := newTestServer(t)
	admin := loginAs(t, srv, st, "管理员", true)
	loginAs(t, srv, st, "张三", false)
	mustBindingFor(t, st, "张三", "小号", "123")

	resp := jsonRequest(t, admin, "DELETE", srv.URL+"/api/users/张三", "")
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusConflict {
		t.Errorf("状态码 = %d, 期望 409", resp.StatusCode)
	}
}

func TestDeleteSelfIsRejected(t *testing.T) {
	// 管理员删掉自己会把系统锁死（可能一个管理员都不剩）
	srv, st := newTestServer(t)
	admin := loginAs(t, srv, st, "管理员", true)

	resp := jsonRequest(t, admin, "DELETE", srv.URL+"/api/users/管理员", "")
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusConflict {
		t.Errorf("删除自己状态码 = %d, 期望 409", resp.StatusCode)
	}
}
