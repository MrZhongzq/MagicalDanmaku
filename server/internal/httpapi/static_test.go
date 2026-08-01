package httpapi_test

import (
	"io"
	"net/http"
	"strings"
	"testing"
)

func TestStaticServesIndex(t *testing.T) {
	srv, _ := newTestServer(t)

	resp, err := http.Get(srv.URL + "/")
	if err != nil {
		t.Fatalf("请求报错: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("状态码 = %d", resp.StatusCode)
	}
	body, _ := io.ReadAll(resp.Body)
	if !strings.Contains(string(body), "<html") {
		t.Errorf("根路径应返回 HTML，实际: %.200s", body)
	}
}

// SPA 回退：前端路由的路径要返回 index.html，让浏览器端路由接管
func TestStaticSPAFallback(t *testing.T) {
	srv, _ := newTestServer(t)

	resp, err := http.Get(srv.URL + "/bindings/7/rules")
	if err != nil {
		t.Fatalf("请求报错: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("状态码 = %d, 期望 200（SPA 回退）", resp.StatusCode)
	}
	body, _ := io.ReadAll(resp.Body)
	if !strings.Contains(string(body), "<html") {
		t.Errorf("SPA 路径应回退到 index.html，实际: %.200s", body)
	}
}

// /api 下的未知路径必须仍是 JSON 404，不能被 SPA 回退吞掉
func TestStaticDoesNotSwallowAPI404(t *testing.T) {
	srv, _ := newTestServer(t)

	resp, err := http.Get(srv.URL + "/api/没有这个接口")
	if err != nil {
		t.Fatalf("请求报错: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("状态码 = %d, 期望 404", resp.StatusCode)
	}
	if ct := resp.Header.Get("Content-Type"); !strings.HasPrefix(ct, "application/json") {
		t.Errorf("Content-Type = %q, API 的 404 必须仍是 JSON", ct)
	}
}

// 裸路径 /api（无尾斜杠）同样不能被 SPA 回退吞掉。
//
// 分流条件写成 HasPrefix(path, "/api/") 时，/api 不满足前缀会掉进
// 静态处理器拿到 200 + HTML。已有的那条测试用的是 /api/xxx，
// 自带斜杠，测不出这个变体。
func TestStaticDoesNotSwallowBareAPIPath(t *testing.T) {
	srv, _ := newTestServer(t)

	resp, err := http.Get(srv.URL + "/api")
	if err != nil {
		t.Fatalf("请求报错: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("状态码 = %d, 期望 404", resp.StatusCode)
	}
	if ct := resp.Header.Get("Content-Type"); !strings.HasPrefix(ct, "application/json") {
		body, _ := io.ReadAll(resp.Body)
		t.Errorf("Content-Type = %q, 期望 application/json；响应体: %.200s", ct, body)
	}
}
