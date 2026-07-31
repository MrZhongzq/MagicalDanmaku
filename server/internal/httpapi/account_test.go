package httpapi_test

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/MrZhongzq/MagicalDanmaku/server/internal/connector/bilibili/auth"
	"github.com/MrZhongzq/MagicalDanmaku/server/internal/httpapi"
	"github.com/MrZhongzq/MagicalDanmaku/server/internal/perm"
	"github.com/MrZhongzq/MagicalDanmaku/server/internal/store"
)

// fakeQR 是可控的扫码实现，避免测试打真实 B 站接口。
type fakeQR struct {
	key    string
	url    string
	result auth.PollResult
	err    error
}

func (f *fakeQR) Generate(context.Context) (*auth.QRCode, error) {
	if f.err != nil {
		return nil, f.err
	}
	return &auth.QRCode{Key: f.key, URL: f.url}, nil
}

func (f *fakeQR) Poll(_ context.Context, key string) (*auth.PollResult, error) {
	if f.err != nil {
		return nil, f.err
	}
	if key != f.key {
		return &auth.PollResult{Status: auth.PollExpired}, nil
	}
	r := f.result
	return &r, nil
}

// 账号列表绝不能带 Cookie
func TestListAccountsNeverLeaksCookie(t *testing.T) {
	srv, st := newTestServer(t)
	c := loginAs(t, srv, st, "张三", false)
	mustBindingFor(t, st, "张三", "小号", "123")

	resp := jsonRequest(t, c, "GET", srv.URL+"/api/accounts", "")
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("状态码 = %d", resp.StatusCode)
	}

	var raw json.RawMessage
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		t.Fatalf("解析报错: %v", err)
	}
	// 直接在原始 JSON 里找，比逐字段断言更难漏
	if bytesContains(raw, "SESSDATA") || bytesContains(raw, "cookie") {
		t.Errorf("账号列表泄漏了 Cookie: %s", raw)
	}
}

func bytesContains(b []byte, sub string) bool {
	return len(b) >= len(sub) && stringIndex(string(b), sub) >= 0
}

func stringIndex(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}

func TestListAccountsFiltersByVisibility(t *testing.T) {
	srv, st := newTestServer(t)
	loginAs(t, srv, st, "张三", false)
	loginAs(t, srv, st, "王五", false)
	mustBindingFor(t, st, "张三", "张三的号", "111")
	mustBindingFor(t, st, "王五", "王五的号", "222")

	li := loginAs(t, srv, st, "李四", false)
	if err := st.Grant(context.Background(), "李四", "张三的号", "111",
		[]perm.Permission{perm.EventRead}); err != nil {
		t.Fatalf("授权报错: %v", err)
	}

	resp := jsonRequest(t, li, "GET", srv.URL+"/api/accounts", "")
	defer resp.Body.Close()

	var got []struct {
		Name string `json:"name"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&got); err != nil {
		t.Fatalf("解析报错: %v", err)
	}
	if len(got) != 1 || got[0].Name != "张三的号" {
		t.Errorf("李四应只看到「张三的号」，实际 %+v", got)
	}
}

func TestQRCodeStartReturnsURL(t *testing.T) {
	srv, st, api := newTestServerWithAPI(t)
	api.SetQRLogin(&fakeQR{key: "KEY123", url: "https://qr.example/abc"})
	c := loginAs(t, srv, st, "张三", false)

	resp := jsonRequest(t, c, "POST", srv.URL+"/api/accounts/qrcode", `{"name":"小号"}`)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("状态码 = %d", resp.StatusCode)
	}

	var body struct {
		Key string `json:"key"`
		URL string `json:"url"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("解析报错: %v", err)
	}
	if body.Key != "KEY123" || body.URL != "https://qr.example/abc" {
		t.Errorf("响应 = %+v", body)
	}
}

func TestQRCodeStartRequiresName(t *testing.T) {
	srv, st, api := newTestServerWithAPI(t)
	api.SetQRLogin(&fakeQR{key: "K", url: "u"})
	c := loginAs(t, srv, st, "张三", false)

	resp := jsonRequest(t, c, "POST", srv.URL+"/api/accounts/qrcode", `{"name":"  "}`)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusUnprocessableEntity {
		t.Errorf("空账号名状态码 = %d, 期望 422", resp.StatusCode)
	}
}

func TestQRCodePollWaiting(t *testing.T) {
	srv, st, api := newTestServerWithAPI(t)
	api.SetQRLogin(&fakeQR{key: "K", url: "u", result: auth.PollResult{Status: auth.PollWaiting}})
	c := loginAs(t, srv, st, "张三", false)

	start := jsonRequest(t, c, "POST", srv.URL+"/api/accounts/qrcode", `{"name":"小号"}`)
	start.Body.Close()

	resp := jsonRequest(t, c, "POST", srv.URL+"/api/accounts/qrcode/K", "")
	defer resp.Body.Close()

	var body struct {
		Status string `json:"status"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("解析报错: %v", err)
	}
	if body.Status != "waiting" {
		t.Errorf("status = %q, 期望 waiting", body.Status)
	}
}

// 扫码成功时账号在服务端建好，Cookie 绝不回传
func TestQRCodePollSuccessCreatesAccount(t *testing.T) {
	srv, st, api := newTestServerWithAPI(t)
	cookie := "SESSDATA=abc; bili_jct=def; DedeUserID=10086"
	api.SetQRLogin(&fakeQR{key: "K", url: "u",
		result: auth.PollResult{Status: auth.PollSuccess, Cookie: cookie}})
	c := loginAs(t, srv, st, "张三", false)

	start := jsonRequest(t, c, "POST", srv.URL+"/api/accounts/qrcode", `{"name":"小号"}`)
	start.Body.Close()

	resp := jsonRequest(t, c, "POST", srv.URL+"/api/accounts/qrcode/K", "")
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("状态码 = %d", resp.StatusCode)
	}

	var raw json.RawMessage
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		t.Fatalf("解析报错: %v", err)
	}
	if bytesContains(raw, "SESSDATA") {
		t.Errorf("扫码响应泄漏了 Cookie: %s", raw)
	}

	acc, err := st.GetAccountByName(context.Background(), "小号")
	if err != nil {
		t.Fatalf("账号应已建好: %v", err)
	}
	if acc.Cookie != cookie {
		t.Errorf("库里的 Cookie = %q", acc.Cookie)
	}
	if acc.UID != "10086" {
		t.Errorf("UID = %q, 期望 10086（应从 Cookie 解析）", acc.UID)
	}
}

// 已存在的账号走换 Cookie 而不是重复建号
func TestQRCodePollSuccessUpdatesExisting(t *testing.T) {
	srv, st, api := newTestServerWithAPI(t)
	c := loginAs(t, srv, st, "张三", false)
	mustBindingFor(t, st, "张三", "小号", "123")

	newCookie := "SESSDATA=new; bili_jct=x; DedeUserID=999"
	api.SetQRLogin(&fakeQR{key: "K", url: "u",
		result: auth.PollResult{Status: auth.PollSuccess, Cookie: newCookie}})

	start := jsonRequest(t, c, "POST", srv.URL+"/api/accounts/qrcode", `{"name":"小号"}`)
	start.Body.Close()
	resp := jsonRequest(t, c, "POST", srv.URL+"/api/accounts/qrcode/K", "")
	resp.Body.Close()

	acc, err := st.GetAccountByName(context.Background(), "小号")
	if err != nil {
		t.Fatalf("查账号报错: %v", err)
	}
	if acc.Cookie != newCookie {
		t.Errorf("Cookie 应被更新，实际 %q", acc.Cookie)
	}

	all, err := st.ListAccounts(context.Background())
	if err != nil {
		t.Fatalf("列账号报错: %v", err)
	}
	if len(all) != 1 {
		t.Errorf("不该重复建号，实际 %d 个账号", len(all))
	}
}

// 别人发起的扫码，我拿着 key 也换不出账号
func TestQRCodePollRejectsOtherUsersKey(t *testing.T) {
	srv, st, api := newTestServerWithAPI(t)
	api.SetQRLogin(&fakeQR{key: "K", url: "u",
		result: auth.PollResult{Status: auth.PollSuccess, Cookie: "SESSDATA=x; DedeUserID=1"}})

	zhang := loginAs(t, srv, st, "张三", false)
	start := jsonRequest(t, zhang, "POST", srv.URL+"/api/accounts/qrcode", `{"name":"小号"}`)
	start.Body.Close()

	li := loginAs(t, srv, st, "李四", false)
	resp := jsonRequest(t, li, "POST", srv.URL+"/api/accounts/qrcode/K", "")
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound && resp.StatusCode != http.StatusForbidden {
		t.Errorf("别人的扫码会话状态码 = %d, 期望 404 或 403", resp.StatusCode)
	}
	if _, err := st.GetAccountByName(context.Background(), "小号"); err == nil {
		t.Error("不该替别人把账号建出来")
	}
}

func TestQRCodePollUnknownKey(t *testing.T) {
	srv, st, api := newTestServerWithAPI(t)
	api.SetQRLogin(&fakeQR{key: "K", url: "u"})
	c := loginAs(t, srv, st, "张三", false)

	resp := jsonRequest(t, c, "POST", srv.URL+"/api/accounts/qrcode/没这个KEY", "")
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("状态码 = %d, 期望 404", resp.StatusCode)
	}
}

func TestPatchAccountRequiresAccountManage(t *testing.T) {
	srv, st := newTestServer(t)
	loginAs(t, srv, st, "张三", false)
	mustBindingFor(t, st, "张三", "小号", "123")

	li := loginAs(t, srv, st, "李四", false)
	resp := jsonRequest(t, li, "PATCH", srv.URL+"/api/accounts/小号", `{"rateLimitMs":2000}`)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusForbidden && resp.StatusCode != http.StatusNotFound {
		t.Errorf("无权限状态码 = %d, 期望 403 或 404", resp.StatusCode)
	}
}

func TestPatchAccountByOwner(t *testing.T) {
	srv, st := newTestServer(t)
	c := loginAs(t, srv, st, "张三", false)
	mustBindingFor(t, st, "张三", "小号", "123")

	resp := jsonRequest(t, c, "PATCH", srv.URL+"/api/accounts/小号",
		`{"rateLimitMs":2500,"maxLength":30}`)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("状态码 = %d", resp.StatusCode)
	}

	acc, err := st.GetAccountByName(context.Background(), "小号")
	if err != nil {
		t.Fatalf("查账号报错: %v", err)
	}
	if acc.RateLimit != 2500*time.Millisecond || acc.MaxLength != 30 {
		t.Errorf("账号参数 = %v / %d", acc.RateLimit, acc.MaxLength)
	}
}

func TestDeleteAccountByOwner(t *testing.T) {
	srv, st := newTestServer(t)
	c := loginAs(t, srv, st, "张三", false)
	mustBindingFor(t, st, "张三", "小号", "123")

	resp := jsonRequest(t, c, "DELETE", srv.URL+"/api/accounts/小号", "")
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("状态码 = %d", resp.StatusCode)
	}
	if _, err := st.GetAccountByName(context.Background(), "小号"); err == nil {
		t.Error("删除后应查不到")
	}
}

func TestDeleteAccountByStranger(t *testing.T) {
	srv, st := newTestServer(t)
	loginAs(t, srv, st, "张三", false)
	mustBindingFor(t, st, "张三", "小号", "123")

	li := loginAs(t, srv, st, "李四", false)
	resp := jsonRequest(t, li, "DELETE", srv.URL+"/api/accounts/小号", "")
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusOK {
		t.Error("非所有者不该能删账号")
	}
	if _, err := st.GetAccountByName(context.Background(), "小号"); err != nil {
		t.Error("账号不该被删掉")
	}
}

var _ = httpapi.Options{} // 保证 import 被使用
var _ = store.AccountInput{}
