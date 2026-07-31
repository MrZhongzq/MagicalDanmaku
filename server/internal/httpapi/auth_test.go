package httpapi_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"testing"

	"github.com/MrZhongzq/MagicalDanmaku/server/internal/store"
)

// loginAs 建一个用户并登录，返回带会话 Cookie 的客户端。
func loginAs(t *testing.T, srv *httptest.Server, st *store.Store, name string, admin bool) *http.Client {
	t.Helper()
	if _, err := st.CreateUser(context.Background(), name, "pw-"+name, admin); err != nil {
		t.Fatalf("创建用户 %s 报错: %v", name, err)
	}
	jar, err := cookiejar.New(nil)
	if err != nil {
		t.Fatalf("创建 cookiejar 报错: %v", err)
	}
	c := &http.Client{Jar: jar}
	resp := jsonRequest(t, c, "POST", srv.URL+"/api/auth/login",
		`{"username":"`+name+`","password":"pw-`+name+`"}`)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("登录 %s 失败，状态码 %d", name, resp.StatusCode)
	}
	return c
}

func TestLoginSetsSessionCookie(t *testing.T) {
	srv, st := newTestServer(t)
	if _, err := st.CreateUser(context.Background(), "张三", "hunter2", false); err != nil {
		t.Fatalf("创建用户报错: %v", err)
	}

	resp := jsonRequest(t, &http.Client{}, "POST", srv.URL+"/api/auth/login",
		`{"username":"张三","password":"hunter2"}`)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("状态码 = %d, 期望 200", resp.StatusCode)
	}

	var found *http.Cookie
	for _, c := range resp.Cookies() {
		if c.Name == "magicd_session" {
			found = c
		}
	}
	if found == nil {
		t.Fatal("没有种下会话 Cookie")
	}
	if !found.HttpOnly {
		t.Error("会话 Cookie 必须是 HttpOnly，否则 XSS 能直接读走")
	}
	if found.SameSite != http.SameSiteLaxMode {
		t.Errorf("SameSite = %v, 期望 Lax（这是本项目替代 CSRF token 的前提）", found.SameSite)
	}
}

// 登录响应体里绝不能有密码哈希之类的东西
func TestLoginResponseHasNoSecrets(t *testing.T) {
	srv, st := newTestServer(t)
	if _, err := st.CreateUser(context.Background(), "张三", "hunter2", false); err != nil {
		t.Fatalf("创建用户报错: %v", err)
	}

	resp := jsonRequest(t, &http.Client{}, "POST", srv.URL+"/api/auth/login",
		`{"username":"张三","password":"hunter2"}`)
	defer resp.Body.Close()

	var body map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("解析报错: %v", err)
	}
	for _, k := range []string{"passwordHash", "password_hash", "password"} {
		if _, ok := body[k]; ok {
			t.Errorf("响应体里出现了 %q", k)
		}
	}
	if body["username"] != "张三" {
		t.Errorf("响应体 = %v", body)
	}
}

func TestLoginWrongPassword(t *testing.T) {
	srv, st := newTestServer(t)
	if _, err := st.CreateUser(context.Background(), "张三", "hunter2", false); err != nil {
		t.Fatalf("创建用户报错: %v", err)
	}

	resp := jsonRequest(t, &http.Client{}, "POST", srv.URL+"/api/auth/login",
		`{"username":"张三","password":"错的"}`)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("状态码 = %d, 期望 401", resp.StatusCode)
	}
	if len(resp.Cookies()) > 0 {
		t.Error("登录失败不该种 Cookie")
	}
}

// 用户不存在与密码错误必须返回完全相同的响应，否则这个接口就是用户名枚举器
func TestLoginHidesWhetherUserExists(t *testing.T) {
	srv, st := newTestServer(t)
	if _, err := st.CreateUser(context.Background(), "张三", "hunter2", false); err != nil {
		t.Fatalf("创建用户报错: %v", err)
	}

	r1 := jsonRequest(t, &http.Client{}, "POST", srv.URL+"/api/auth/login",
		`{"username":"张三","password":"错的"}`)
	defer r1.Body.Close()
	var b1 map[string]string
	_ = json.NewDecoder(r1.Body).Decode(&b1)

	r2 := jsonRequest(t, &http.Client{}, "POST", srv.URL+"/api/auth/login",
		`{"username":"查无此人","password":"错的"}`)
	defer r2.Body.Close()
	var b2 map[string]string
	_ = json.NewDecoder(r2.Body).Decode(&b2)

	if r1.StatusCode != r2.StatusCode {
		t.Errorf("状态码不一致: %d vs %d", r1.StatusCode, r2.StatusCode)
	}
	if b1["error"] != b2["error"] {
		t.Errorf("错误信息不一致:\n  密码错: %q\n  无此人: %q", b1["error"], b2["error"])
	}
}

func TestMeRequiresAuth(t *testing.T) {
	srv, _ := newTestServer(t)

	resp, err := http.Get(srv.URL + "/api/auth/me")
	if err != nil {
		t.Fatalf("请求报错: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("未登录访问 /api/auth/me 状态码 = %d, 期望 401", resp.StatusCode)
	}
}

func TestMeReturnsUserAndMemberships(t *testing.T) {
	srv, st := newTestServer(t)
	c := loginAs(t, srv, st, "张三", false)

	resp := jsonRequest(t, c, "GET", srv.URL+"/api/auth/me", "")
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("状态码 = %d", resp.StatusCode)
	}

	var body struct {
		User struct {
			Username string `json:"username"`
			IsAdmin  bool   `json:"isAdmin"`
		} `json:"user"`
		Memberships []any `json:"memberships"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("解析报错: %v", err)
	}
	if body.User.Username != "张三" {
		t.Errorf("用户 = %+v", body.User)
	}
	if body.Memberships == nil {
		t.Error("memberships 应是空数组而非 null，前端不该被迫判空")
	}
}

func TestLogoutInvalidatesSession(t *testing.T) {
	srv, st := newTestServer(t)
	c := loginAs(t, srv, st, "张三", false)

	resp := jsonRequest(t, c, "POST", srv.URL+"/api/auth/logout", "")
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("登出状态码 = %d", resp.StatusCode)
	}

	after := jsonRequest(t, c, "GET", srv.URL+"/api/auth/me", "")
	defer after.Body.Close()
	if after.StatusCode != http.StatusUnauthorized {
		t.Errorf("登出后 /api/auth/me 状态码 = %d, 期望 401", after.StatusCode)
	}
}

func TestBogusCookieIsRejected(t *testing.T) {
	srv, _ := newTestServer(t)

	req, _ := http.NewRequest("GET", srv.URL+"/api/auth/me", nil)
	req.AddCookie(&http.Cookie{Name: "magicd_session", Value: "我编的"})
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("请求报错: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("伪造 Cookie 状态码 = %d, 期望 401", resp.StatusCode)
	}
}

// 登录必须是 POST：SameSite=Lax 不拦截跨站顶层 GET 导航
func TestLoginRejectsGet(t *testing.T) {
	srv, _ := newTestServer(t)

	resp, err := http.Get(srv.URL + "/api/auth/login")
	if err != nil {
		t.Fatalf("请求报错: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusMethodNotAllowed {
		t.Errorf("GET /api/auth/login 状态码 = %d, 期望 405", resp.StatusCode)
	}
}
