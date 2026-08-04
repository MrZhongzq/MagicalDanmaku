package httpapi_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"sync"
	"testing"

	"github.com/MrZhongzq/MagicalDanmaku/server/internal/connector"
	"github.com/MrZhongzq/MagicalDanmaku/server/internal/httpapi"
	"github.com/MrZhongzq/MagicalDanmaku/server/internal/perm"
)

// fakeRuntime 记录被调用的动作，避免测试真连 B 站。
type fakeRuntime struct {
	mu            sync.Mutex
	danmaku       []string
	blocked       []string
	unblocked     []string
	blacklisted   []string
	unblacklisted []string
	err           error
	state         connector.State

	// blacklistedAttr/blacklistedName 是 BlacklistStatus 的固定回读结果。
	blacklistedAttr bool
	blacklistedName string
	statusErr       error

	reloadErr  error
	reloadHits int
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

func (f *fakeRuntime) Blacklist(_ context.Context, uid string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.err != nil {
		return f.err
	}
	f.blacklisted = append(f.blacklisted, uid)
	return nil
}

func (f *fakeRuntime) Unblacklist(_ context.Context, uid string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.err != nil {
		return f.err
	}
	f.unblacklisted = append(f.unblacklisted, uid)
	return nil
}

func (f *fakeRuntime) BlacklistStatus(_ context.Context, uid string) (bool, string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.statusErr != nil {
		return false, "", f.statusErr
	}
	return f.blacklistedAttr, f.blacklistedName, nil
}

func (f *fakeRuntime) Nickname(_ context.Context, uid string) (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.statusErr != nil {
		return "", f.statusErr
	}
	return f.blacklistedName, nil
}

func (f *fakeRuntime) State() connector.State {
	if f.state == "" {
		return connector.StateConnected
	}
	return f.state
}

func (f *fakeRuntime) Reload(ctx context.Context) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.reloadHits++
	return f.reloadErr
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

// 重载失败要回 422 并说明原因，而不是 500。
//
// 机器人没有停——旧引擎还在跑。操作者需要看到「哪条规则错了」。
func TestReloadReportsFailure(t *testing.T) {
	srv, st, api := newTestServerWithAPI(t)
	c := loginAs(t, srv, st, "张三", false)
	bid := mustBindingFor(t, st, "张三", "小号", "123")
	fake := &fakeRuntime{reloadErr: errors.New("规则 关键词回复 的正则非法")}
	api.SetRuntime(map[int64]httpapi.BindingRuntime{bid: fake})

	resp := jsonRequest(t, c, "POST", srv.URL+"/api/bindings/"+itoa(bid)+"/reload", "")
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusUnprocessableEntity {
		t.Fatalf("状态码 = %d, 期望 422", resp.StatusCode)
	}

	var body map[string]string
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("解析报错: %v", err)
	}
	if !strings.Contains(body["error"], "正则非法") {
		t.Errorf("错误文案应带上具体原因，实际: %q", body["error"])
	}
	// 「仍在用上一份配置运行」这句话是操作者最需要的安抚
	if !strings.Contains(body["error"], "仍在用上一份配置") {
		t.Errorf("错误文案应说明机器人没有停，实际: %q", body["error"])
	}
}

func TestReloadSucceeds(t *testing.T) {
	srv, st, api := newTestServerWithAPI(t)
	c := loginAs(t, srv, st, "张三", false)
	bid := mustBindingFor(t, st, "张三", "小号", "123")
	fake := &fakeRuntime{}
	api.SetRuntime(map[int64]httpapi.BindingRuntime{bid: fake})

	resp := jsonRequest(t, c, "POST", srv.URL+"/api/bindings/"+itoa(bid)+"/reload", "")
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("状态码 = %d", resp.StatusCode)
	}
	if fake.reloadHits != 1 {
		t.Errorf("Reload 被调用 %d 次, 期望 1", fake.reloadHits)
	}
}

// 重载改的是规则，所以要 rule:write 而不是 rule:read
func TestReloadRequiresRuleWrite(t *testing.T) {
	srv, st, api := newTestServerWithAPI(t)
	loginAs(t, srv, st, "张三", false)
	bid := mustBindingFor(t, st, "张三", "小号", "123")
	api.SetRuntime(map[int64]httpapi.BindingRuntime{bid: &fakeRuntime{}})

	li := loginAs(t, srv, st, "李四", false)
	// 给一个**无关**的权限点：零授权的话他对这个绑定完全不可见，
	// 守卫会回 404 而不是 403
	if err := st.Grant(context.Background(), "李四", "小号", "123",
		[]perm.Permission{perm.RuleRead}); err != nil {
		t.Fatalf("授权报错: %v", err)
	}

	resp := jsonRequest(t, li, "POST", srv.URL+"/api/bindings/"+itoa(bid)+"/reload", "")
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusForbidden {
		t.Errorf("只有 rule:read 时状态码 = %d, 期望 403", resp.StatusCode)
	}
}
