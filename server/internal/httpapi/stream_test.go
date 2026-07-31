package httpapi_test

import (
	"bufio"
	"context"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/MrZhongzq/MagicalDanmaku/server/internal/event"
	"github.com/MrZhongzq/MagicalDanmaku/server/internal/perm"
)

func TestStreamRequiresEventRead(t *testing.T) {
	srv, st := newTestServer(t)
	loginAs(t, srv, st, "张三", false)
	bid := mustBindingFor(t, st, "张三", "小号", "123")

	// 李四要对这个绑定有任意可见性（这里给 rule:write），403 才不会
	// 和「绑定不存在」的 404 混在一起——这与 activity 接口的同名测试
	// （activity_test.go: TestQueryActivityRequiresEventRead）是同一套
	// 判定逻辑，唯一实现在 guard.go 的 requirePerm 里。
	li := loginAs(t, srv, st, "李四", false)
	if err := st.Grant(context.Background(), "李四", "小号", "123",
		[]perm.Permission{perm.RuleWrite}); err != nil {
		t.Fatalf("授权报错: %v", err)
	}

	resp := jsonRequest(t, li, "GET", srv.URL+"/api/bindings/"+itoa(bid)+"/stream", "")
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusForbidden {
		t.Errorf("状态码 = %d, 期望 403", resp.StatusCode)
	}
}

func TestStreamDeliversEvent(t *testing.T) {
	srv, st, api := newTestServerWithAPI(t)
	c := loginAs(t, srv, st, "张三", false)
	bid := mustBindingFor(t, st, "张三", "小号", "123")
	grantEventRead(t, st, "张三", "小号", "123")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	req, _ := http.NewRequestWithContext(ctx, "GET",
		srv.URL+"/api/bindings/"+itoa(bid)+"/stream", nil)
	for _, ck := range c.Jar.Cookies(req.URL) {
		req.AddCookie(ck)
	}
	resp, err := c.Do(req)
	if err != nil {
		t.Fatalf("建立 SSE 连接报错: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("状态码 = %d", resp.StatusCode)
	}
	if ct := resp.Header.Get("Content-Type"); !strings.HasPrefix(ct, "text/event-stream") {
		t.Fatalf("Content-Type = %q, 期望 text/event-stream", ct)
	}

	// 等订阅注册完成再发，否则可能发在订阅之前
	deadline := time.Now().Add(2 * time.Second)
	for api.Hub().SubscriberCount(bid) == 0 && time.Now().Before(deadline) {
		time.Sleep(5 * time.Millisecond)
	}
	if api.Hub().SubscriberCount(bid) == 0 {
		t.Fatal("订阅未注册")
	}

	api.Hub().Publish(bid, event.Event{
		ID: event.NewID(), Type: event.TypeDanmaku,
		Payload: event.Danmaku{User: event.User{Username: "观众甲"}, Text: "你好"},
	})

	sc := bufio.NewScanner(resp.Body)
	found := false
	for sc.Scan() {
		line := sc.Text()
		if strings.HasPrefix(line, "data:") && strings.Contains(line, "观众甲") {
			found = true
			break
		}
	}
	if !found {
		t.Error("没有收到包含该弹幕的 data 行")
	}
}

// 客户端断开后订阅必须被清掉，否则每次刷新页面都泄漏一个订阅者
func TestStreamUnsubscribesOnDisconnect(t *testing.T) {
	srv, st, api := newTestServerWithAPI(t)
	c := loginAs(t, srv, st, "张三", false)
	bid := mustBindingFor(t, st, "张三", "小号", "123")
	grantEventRead(t, st, "张三", "小号", "123")

	ctx, cancel := context.WithCancel(context.Background())
	req, _ := http.NewRequestWithContext(ctx, "GET",
		srv.URL+"/api/bindings/"+itoa(bid)+"/stream", nil)
	for _, ck := range c.Jar.Cookies(req.URL) {
		req.AddCookie(ck)
	}
	resp, err := c.Do(req)
	if err != nil {
		t.Fatalf("建立 SSE 连接报错: %v", err)
	}

	deadline := time.Now().Add(2 * time.Second)
	for api.Hub().SubscriberCount(bid) == 0 && time.Now().Before(deadline) {
		time.Sleep(5 * time.Millisecond)
	}
	if api.Hub().SubscriberCount(bid) != 1 {
		t.Fatalf("订阅者数 = %d, 期望 1", api.Hub().SubscriberCount(bid))
	}

	cancel()
	resp.Body.Close()

	deadline = time.Now().Add(3 * time.Second)
	for api.Hub().SubscriberCount(bid) != 0 && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	if n := api.Hub().SubscriberCount(bid); n != 0 {
		t.Errorf("断开后订阅者数 = %d, 期望 0（每次刷新页面都会泄漏一个）", n)
	}
}

var _ = perm.EventRead
