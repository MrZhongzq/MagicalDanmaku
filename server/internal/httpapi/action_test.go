package httpapi_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"sync"
	"testing"

	"github.com/MrZhongzq/MagicalDanmaku/server/internal/connector"
	"github.com/MrZhongzq/MagicalDanmaku/server/internal/httpapi"
	"github.com/MrZhongzq/MagicalDanmaku/server/internal/perm"
)

// fakeRuntime 记录被调用的动作，避免测试真连 B 站。
type fakeRuntime struct {
	mu        sync.Mutex
	danmaku   []string
	blocked   []string
	unblocked []string
	err       error
	state     connector.State
}

func (f *fakeRuntime) SendDanmaku(_ context.Context, text string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.err != nil {
		return f.err
	}
	f.danmaku = append(f.danmaku, text)
	return nil
}

func (f *fakeRuntime) Block(_ context.Context, uid string, _ int) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.err != nil {
		return f.err
	}
	f.blocked = append(f.blocked, uid)
	return nil
}

func (f *fakeRuntime) Unblock(_ context.Context, uid string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.err != nil {
		return f.err
	}
	f.unblocked = append(f.unblocked, uid)
	return nil
}

func (f *fakeRuntime) State() connector.State {
	if f.state == "" {
		return connector.StateConnected
	}
	return f.state
}

func (f *fakeRuntime) snapshot() ([]string, []string, []string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]string{}, f.danmaku...),
		append([]string{}, f.blocked...),
		append([]string{}, f.unblocked...)
}

func TestSendDanmaku(t *testing.T) {
	srv, st, api := newTestServerWithAPI(t)
	c := loginAs(t, srv, st, "张三", false)
	bid := mustBindingFor(t, st, "张三", "小号", "123")
	if err := st.Grant(context.Background(), "张三", "小号", "123",
		[]perm.Permission{perm.DanmakuSend}); err != nil {
		t.Fatalf("授权报错: %v", err)
	}

	rt := &fakeRuntime{}
	api.SetRuntime(map[int64]httpapi.BindingRuntime{bid: rt})

	resp := jsonRequest(t, c, "POST", srv.URL+"/api/bindings/"+itoa(bid)+"/danmaku",
		`{"text":"大家好"}`)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("状态码 = %d", resp.StatusCode)
	}

	sent, _, _ := rt.snapshot()
	if len(sent) != 1 || sent[0] != "大家好" {
		t.Errorf("发出的弹幕 = %v", sent)
	}
}

func TestSendDanmakuRequiresPermission(t *testing.T) {
	srv, st, api := newTestServerWithAPI(t)
	loginAs(t, srv, st, "张三", false)
	bid := mustBindingFor(t, st, "张三", "小号", "123")
	api.SetRuntime(map[int64]httpapi.BindingRuntime{bid: &fakeRuntime{}})

	li := loginAs(t, srv, st, "李四", false)
	if err := st.Grant(context.Background(), "李四", "小号", "123",
		[]perm.Permission{perm.EventRead}); err != nil {
		t.Fatalf("授权报错: %v", err)
	}

	resp := jsonRequest(t, li, "POST", srv.URL+"/api/bindings/"+itoa(bid)+"/danmaku",
		`{"text":"我没权限"}`)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusForbidden {
		t.Errorf("状态码 = %d, 期望 403", resp.StatusCode)
	}
}

func TestSendDanmakuRejectsEmpty(t *testing.T) {
	srv, st, api := newTestServerWithAPI(t)
	c := loginAs(t, srv, st, "张三", false)
	bid := mustBindingFor(t, st, "张三", "小号", "123")
	if err := st.Grant(context.Background(), "张三", "小号", "123",
		[]perm.Permission{perm.DanmakuSend}); err != nil {
		t.Fatalf("授权报错: %v", err)
	}
	api.SetRuntime(map[int64]httpapi.BindingRuntime{bid: &fakeRuntime{}})

	resp := jsonRequest(t, c, "POST", srv.URL+"/api/bindings/"+itoa(bid)+"/danmaku",
		`{"text":"   "}`)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusUnprocessableEntity {
		t.Errorf("状态码 = %d, 期望 422", resp.StatusCode)
	}
}

// 绑定没在运行时（机器人没跑，或该绑定被停用）要给出能看懂的说明
func TestActionWhenBindingNotRunning(t *testing.T) {
	srv, st, api := newTestServerWithAPI(t)
	c := loginAs(t, srv, st, "张三", false)
	bid := mustBindingFor(t, st, "张三", "小号", "123")
	if err := st.Grant(context.Background(), "张三", "小号", "123",
		[]perm.Permission{perm.DanmakuSend}); err != nil {
		t.Fatalf("授权报错: %v", err)
	}
	api.SetRuntime(map[int64]httpapi.BindingRuntime{}) // 空的

	resp := jsonRequest(t, c, "POST", srv.URL+"/api/bindings/"+itoa(bid)+"/danmaku",
		`{"text":"你好"}`)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusServiceUnavailable {
		t.Errorf("状态码 = %d, 期望 503", resp.StatusCode)
	}
	var body map[string]string
	_ = json.NewDecoder(resp.Body).Decode(&body)
	if body["error"] == "" {
		t.Error("应给出人能看懂的说明")
	}
}

// 发送失败要如实回报，不能假装成功
func TestSendDanmakuReportsFailure(t *testing.T) {
	srv, st, api := newTestServerWithAPI(t)
	c := loginAs(t, srv, st, "张三", false)
	bid := mustBindingFor(t, st, "张三", "小号", "123")
	if err := st.Grant(context.Background(), "张三", "小号", "123",
		[]perm.Permission{perm.DanmakuSend}); err != nil {
		t.Fatalf("授权报错: %v", err)
	}
	api.SetRuntime(map[int64]httpapi.BindingRuntime{
		bid: &fakeRuntime{err: errors.New("B 站返回风控")},
	})

	resp := jsonRequest(t, c, "POST", srv.URL+"/api/bindings/"+itoa(bid)+"/danmaku",
		`{"text":"你好"}`)
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusOK {
		t.Error("发送失败不该返回 200")
	}
}

func TestBlockAndUnblock(t *testing.T) {
	srv, st, api := newTestServerWithAPI(t)
	c := loginAs(t, srv, st, "张三", false)
	bid := mustBindingFor(t, st, "张三", "小号", "123")
	if err := st.Grant(context.Background(), "张三", "小号", "123",
		[]perm.Permission{perm.UserBlock}); err != nil {
		t.Fatalf("授权报错: %v", err)
	}
	rt := &fakeRuntime{}
	api.SetRuntime(map[int64]httpapi.BindingRuntime{bid: rt})

	blockResp := jsonRequest(t, c, "POST", srv.URL+"/api/bindings/"+itoa(bid)+"/block",
		`{"uid":"10086","hours":1}`)
	blockResp.Body.Close()
	if blockResp.StatusCode != http.StatusOK {
		t.Fatalf("禁言状态码 = %d", blockResp.StatusCode)
	}

	unblockResp := jsonRequest(t, c, "POST", srv.URL+"/api/bindings/"+itoa(bid)+"/unblock",
		`{"uid":"10086"}`)
	unblockResp.Body.Close()
	if unblockResp.StatusCode != http.StatusOK {
		t.Fatalf("解禁状态码 = %d", unblockResp.StatusCode)
	}

	_, blocked, unblocked := rt.snapshot()
	if len(blocked) != 1 || blocked[0] != "10086" {
		t.Errorf("禁言记录 = %v", blocked)
	}
	if len(unblocked) != 1 || unblocked[0] != "10086" {
		t.Errorf("解禁记录 = %v", unblocked)
	}
}

func TestBlockRequiresUserBlockNotDanmakuSend(t *testing.T) {
	srv, st, api := newTestServerWithAPI(t)
	loginAs(t, srv, st, "张三", false)
	bid := mustBindingFor(t, st, "张三", "小号", "123")
	api.SetRuntime(map[int64]httpapi.BindingRuntime{bid: &fakeRuntime{}})

	li := loginAs(t, srv, st, "李四", false)
	if err := st.Grant(context.Background(), "李四", "小号", "123",
		[]perm.Permission{perm.DanmakuSend}); err != nil {
		t.Fatalf("授权报错: %v", err)
	}

	resp := jsonRequest(t, li, "POST", srv.URL+"/api/bindings/"+itoa(bid)+"/block",
		`{"uid":"10086","hours":1}`)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusForbidden {
		t.Errorf("有 danmaku:send 但无 user:block 时状态码 = %d, 期望 403", resp.StatusCode)
	}
}
