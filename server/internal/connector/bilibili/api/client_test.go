package api

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/MrZhongzq/MagicalDanmaku/server/internal/connector/bilibili/auth"
)

const testCookie = "SESSDATA=xyz; bili_jct=tok123; DedeUserID=42"

func newTestClient(t *testing.T, h http.HandlerFunc) (*Client, *httptest.Server) {
	t.Helper()
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)

	sess, err := auth.ParseSession(testCookie)
	if err != nil {
		t.Fatalf("ParseSession 失败: %v", err)
	}
	return New(sess, WithHTTPClient(srv.Client())), srv
}

func TestGetJSONSendsCookieAndHeaders(t *testing.T) {
	var gotCookie, gotUA, gotReferer string
	c, srv := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		gotCookie = r.Header.Get("Cookie")
		gotUA = r.Header.Get("User-Agent")
		gotReferer = r.Header.Get("Referer")
		w.Write([]byte(`{"code":0,"message":"0","data":{"ok":true}}`))
	})

	var out struct {
		OK bool `json:"ok"`
	}
	if err := c.GetJSON(context.Background(), srv.URL, nil, false, &out); err != nil {
		t.Fatalf("GetJSON 失败: %v", err)
	}
	if !out.OK {
		t.Error("data 未被解出")
	}
	if !strings.Contains(gotCookie, "SESSDATA=xyz") {
		t.Errorf("Cookie = %q", gotCookie)
	}
	if gotUA == "" {
		t.Error("必须携带 User-Agent，否则易触发风控")
	}
	if !strings.Contains(gotReferer, "bilibili.com") {
		t.Errorf("Referer = %q", gotReferer)
	}
}

func TestGetJSONReturnsAPIError(t *testing.T) {
	c, srv := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"code":-101,"message":"账号未登录"}`))
	})

	err := c.GetJSON(context.Background(), srv.URL, nil, false, nil)
	if err == nil {
		t.Fatal("应当返回错误")
	}
	var apiErr *APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("错误类型 = %T, 期望 *APIError", err)
	}
	if apiErr.Code != -101 {
		t.Errorf("Code = %d", apiErr.Code)
	}
	if !strings.Contains(apiErr.Error(), "账号未登录") {
		t.Errorf("错误信息应含服务端消息，实际 %q", apiErr.Error())
	}
}

func TestIsRiskControl(t *testing.T) {
	if !IsRiskControl(&APIError{Code: -352}) {
		t.Error("-352 应判定为风控")
	}
	if IsRiskControl(&APIError{Code: -101}) {
		t.Error("-101 不应判定为风控")
	}
	if IsRiskControl(errors.New("其他错误")) {
		t.Error("非 APIError 不应判定为风控")
	}
}

func TestGetJSONSignsWhenRequested(t *testing.T) {
	var gotQuery url.Values
	c, srv := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		gotQuery = r.URL.Query()
		w.Write([]byte(`{"code":0,"data":{}}`))
	})
	c.Signer().SetMixinKey("abcdefghijklmnopqrstuvwxyz012345")

	params := url.Values{}
	params.Set("id", "123")
	if err := c.GetJSON(context.Background(), srv.URL, params, true, nil); err != nil {
		t.Fatalf("GetJSON 失败: %v", err)
	}
	if gotQuery.Get("w_rid") == "" {
		t.Error("签名请求应带 w_rid")
	}
	if gotQuery.Get("wts") == "" {
		t.Error("签名请求应带 wts")
	}
	if gotQuery.Get("id") != "123" {
		t.Errorf("原参数丢失: %v", gotQuery)
	}
}

func TestPostFormAddsCSRF(t *testing.T) {
	var gotForm url.Values
	c, srv := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		r.ParseForm()
		gotForm = r.PostForm
		w.Write([]byte(`{"code":0,"data":{}}`))
	})

	form := url.Values{}
	form.Set("msg", "你好")
	if err := c.PostForm(context.Background(), srv.URL, form, nil); err != nil {
		t.Fatalf("PostForm 失败: %v", err)
	}
	if gotForm.Get("csrf") != "tok123" {
		t.Errorf("csrf = %q", gotForm.Get("csrf"))
	}
	if gotForm.Get("csrf_token") != "tok123" {
		t.Errorf("csrf_token = %q", gotForm.Get("csrf_token"))
	}
	if gotForm.Get("msg") != "你好" {
		t.Errorf("msg = %q", gotForm.Get("msg"))
	}
}

func TestRefreshNavSetsMixinKey(t *testing.T) {
	c, srv := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"code":0,"data":{"wbi_img":{
			"img_url":"https://i0.hdslb.com/bfs/wbi/0123456789abcdef0123456789abcdef.png",
			"sub_url":"https://i0.hdslb.com/bfs/wbi/fedcba9876543210fedcba9876543210.png"
		}}}`))
	})
	c.SetBaseURL("nav", srv.URL)

	if !c.Signer().NeedsRefresh() {
		t.Fatal("前置条件错误：新客户端应需要刷新")
	}
	if err := c.RefreshNav(context.Background()); err != nil {
		t.Fatalf("RefreshNav 失败: %v", err)
	}
	if c.Signer().NeedsRefresh() {
		t.Error("刷新后不应再需要刷新")
	}
}
