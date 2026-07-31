package httpapi_test

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"
)

func TestHealthEndpoint(t *testing.T) {
	srv, _ := newTestServer(t)

	resp, err := http.Get(srv.URL + "/api/health")
	if err != nil {
		t.Fatalf("请求报错: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("状态码 = %d, 期望 200", resp.StatusCode)
	}
	if ct := resp.Header.Get("Content-Type"); !strings.HasPrefix(ct, "application/json") {
		t.Errorf("Content-Type = %q", ct)
	}

	var body map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("解析响应报错: %v", err)
	}
	if body["status"] != "ok" {
		t.Errorf("响应体 = %v", body)
	}
}

func TestUnknownPathReturns404JSON(t *testing.T) {
	srv, _ := newTestServer(t)

	resp, err := http.Get(srv.URL + "/api/根本没有这个接口")
	if err != nil {
		t.Fatalf("请求报错: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("状态码 = %d, 期望 404", resp.StatusCode)
	}
	// 错误也必须是 JSON，前端才能统一处理
	var body map[string]string
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("404 响应不是合法 JSON: %v", err)
	}
	if body["error"] == "" {
		t.Errorf("错误响应缺少 error 字段: %v", body)
	}
}

// 方法不匹配必须是 405 而不是 404，否则前端分不清「路径错了」和「方法错了」
func TestWrongMethodReturns405(t *testing.T) {
	srv, _ := newTestServer(t)

	resp, err := http.Post(srv.URL+"/api/health", "application/json", strings.NewReader("{}"))
	if err != nil {
		t.Fatalf("请求报错: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusMethodNotAllowed {
		t.Errorf("状态码 = %d, 期望 405", resp.StatusCode)
	}
}

// 处理器 panic 不能带崩整个进程
func TestPanicInHandlerReturns500(t *testing.T) {
	srv, _ := newTestServer(t)

	resp, err := http.Get(srv.URL + "/api/test/panic")
	if err != nil {
		t.Fatalf("请求报错: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusInternalServerError {
		t.Errorf("状态码 = %d, 期望 500", resp.StatusCode)
	}

	// panic 之后服务必须还活着
	resp2, err := http.Get(srv.URL + "/api/health")
	if err != nil {
		t.Fatalf("panic 之后服务应仍可用: %v", err)
	}
	defer resp2.Body.Close()
	if resp2.StatusCode != http.StatusOK {
		t.Errorf("panic 之后 health = %d", resp2.StatusCode)
	}
}

// 500 响应绝不能把内部错误细节吐给客户端
func TestPanicResponseHidesDetails(t *testing.T) {
	srv, _ := newTestServer(t)

	resp, err := http.Get(srv.URL + "/api/test/panic")
	if err != nil {
		t.Fatalf("请求报错: %v", err)
	}
	defer resp.Body.Close()

	var body map[string]string
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("解析报错: %v", err)
	}
	if strings.Contains(body["error"], "panic") || strings.Contains(body["error"], "goroutine") {
		t.Errorf("500 响应泄漏了内部细节: %q", body["error"])
	}
}
