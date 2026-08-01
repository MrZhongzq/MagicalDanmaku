package httpapi_test

import (
	"context"
	"net"
	"net/http"
	"net/http/cookiejar"
	"testing"
	"time"

	"github.com/MrZhongzq/MagicalDanmaku/server/internal/httpapi"
)

// Ctrl+C 时如果正开着一条 SSE（WebUI 日志页正常状态下就是），
// ListenAndServe 必须几乎立刻返回，而不是固定卡满 10 秒的关停预算。
//
// srv.Shutdown 只关监听器与 idle 连接，不取消在途请求的 context；
// handleStream 只在 r.Context().Done() 时退出循环，keepalive 是 30
// 秒一次。Server 用一个 shuttingDown 广播 channel 单独通知 SSE 提前
// 收摊——不能用 http.Server.BaseContext 把请求 ctx 系在主 ctx 上，
// 那会连带取消全部在途的普通请求，参见
// TestShutdownLetsInFlightRequestFinish。
func TestListenAndServeShutsDownQuicklyWithOpenSSE(t *testing.T) {
	_, st := newTestServer(t) // 只借它建好库 schema，不用它起的 httptest server

	if _, err := st.CreateUser(context.Background(), "张三", "hunter2", false); err != nil {
		t.Fatalf("建用户报错: %v", err)
	}
	bid := mustBindingFor(t, st, "张三", "小号", "123")
	grantEventRead(t, st, "张三", "小号", "123")

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("找空闲端口报错: %v", err)
	}
	addr := ln.Addr().String()
	ln.Close()

	api := httpapi.New(st, httpapi.Options{Addr: addr, SessionTTL: time.Hour})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() { done <- api.ListenAndServe(ctx) }()

	base := "http://" + addr
	jar, err := cookiejar.New(nil)
	if err != nil {
		t.Fatalf("建 cookiejar 报错: %v", err)
	}
	client := &http.Client{Jar: jar}

	// 等 ListenAndServe 里的监听器真正起来
	deadline := time.Now().Add(3 * time.Second)
	var up bool
	for time.Now().Before(deadline) {
		resp, err := client.Get(base + "/api/health")
		if err == nil {
			resp.Body.Close()
			up = true
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	if !up {
		t.Fatal("HTTP 服务在预期时间内没有起来")
	}

	loginResp := jsonRequest(t, client, "POST", base+"/api/auth/login",
		`{"username":"张三","password":"hunter2"}`)
	loginResp.Body.Close()
	if loginResp.StatusCode != http.StatusOK {
		t.Fatalf("登录状态码 = %d", loginResp.StatusCode)
	}

	// 建立一条不会主动结束的 SSE 连接
	streamCtx, streamCancel := context.WithCancel(context.Background())
	defer streamCancel()
	req, _ := http.NewRequestWithContext(streamCtx, "GET",
		base+"/api/bindings/"+itoa(bid)+"/stream", nil)
	for _, ck := range jar.Cookies(req.URL) {
		req.AddCookie(ck)
	}
	streamResp, err := client.Do(req)
	if err != nil {
		t.Fatalf("建立 SSE 连接报错: %v", err)
	}
	defer streamResp.Body.Close()
	if streamResp.StatusCode != http.StatusOK {
		t.Fatalf("SSE 状态码 = %d", streamResp.StatusCode)
	}

	// 等订阅真正注册完成，确保关停时确实有一条在途的 SSE 请求
	subDeadline := time.Now().Add(2 * time.Second)
	for api.Hub().SubscriberCount(bid) == 0 && time.Now().Before(subDeadline) {
		time.Sleep(5 * time.Millisecond)
	}
	if api.Hub().SubscriberCount(bid) == 0 {
		t.Fatal("SSE 订阅未注册，测试没有覆盖到「关停时有在途请求」这个场景")
	}

	start := time.Now()
	cancel() // 相当于 Ctrl+C

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("ListenAndServe 返回错误: %v", err)
		}
	case <-time.After(9 * time.Second):
		t.Fatal("ListenAndServe 在 9 秒内没有返回——SSE 挂住了整个关停预算")
	}
	elapsed := time.Since(start)

	// 期望远小于 10 秒的关停预算；给并发抖动留余量，2 秒足够宽松
	if elapsed >= 2*time.Second {
		t.Errorf("ListenAndServe 关停耗时 = %v，期望远小于 10 秒的关停预算", elapsed)
	}
}

// 关停时，已经在处理中的普通请求应当能正常做完，不该被腰斩。
//
// 用 http.Server.BaseContext 把请求 ctx 系在主 ctx 上会破坏这一点：
// 处理器普遍把 r.Context() 传给 store，关停瞬间它们会以
// context.Canceled 失败返回 500，而 srv.Shutdown 本来就是要给它们
// 一个自然完成的窗口。SSE 该提前收摊是 SSE 自己的特殊性（它是长
// 连接，不会自然结束），不代表所有请求都该被提前取消。
func TestShutdownLetsInFlightRequestFinish(t *testing.T) {
	_, st := newTestServer(t) // 只借它建好库 schema

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("找空闲端口报错: %v", err)
	}
	addr := ln.Addr().String()
	ln.Close()

	// EnableTestRoutes 打开 /api/test/slow：睡 300ms 再回 200，
	// 用来在它睡着的时候取消主 ctx，验证关停不会腰斩它。
	api := httpapi.New(st, httpapi.Options{Addr: addr, SessionTTL: time.Hour, EnableTestRoutes: true})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() { done <- api.ListenAndServe(ctx) }()

	base := "http://" + addr
	// 等 ListenAndServe 里的监听器真正起来
	deadline := time.Now().Add(3 * time.Second)
	var up bool
	for time.Now().Before(deadline) {
		resp, err := http.Get(base + "/api/health")
		if err == nil {
			resp.Body.Close()
			up = true
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	if !up {
		t.Fatal("HTTP 服务在预期时间内没有起来")
	}

	type result struct {
		status int
		err    error
	}
	resultCh := make(chan result, 1)
	go func() {
		resp, err := http.Get(base + "/api/test/slow")
		if err != nil {
			resultCh <- result{err: err}
			return
		}
		defer resp.Body.Close()
		resultCh <- result{status: resp.StatusCode}
	}()

	// 给请求一点时间真正进了 handler（此刻它正卡在 300ms 的 sleep 里），
	// 再取消主 ctx——相当于请求处理中途按下 Ctrl+C。
	time.Sleep(50 * time.Millisecond)
	cancel()

	select {
	case r := <-resultCh:
		if r.err != nil {
			t.Fatalf("在途请求应正常完成，而不是报错: %v", r.err)
		}
		if r.status != http.StatusOK {
			t.Errorf("在途请求状态码 = %d, 期望 200（关停不该腰斩它）", r.status)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("在途请求 5 秒内没有返回")
	}

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("ListenAndServe 返回错误: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("ListenAndServe 在 5 秒内没有返回")
	}
}
