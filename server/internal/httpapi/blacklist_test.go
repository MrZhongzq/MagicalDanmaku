package httpapi_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"testing"

	"github.com/MrZhongzq/MagicalDanmaku/server/internal/httpapi"
	"github.com/MrZhongzq/MagicalDanmaku/server/internal/perm"
)

// TestBlacklistAndUnblacklist 是拉黑/取消拉黑的成功路径：账号所有者
// （张三）对自己的绑定发起请求，应当命中 rt.Blacklist/rt.Unblacklist。
func TestBlacklistAndUnblacklist(t *testing.T) {
	srv, st, api := newTestServerWithAPI(t)
	c := loginAs(t, srv, st, "张三", false)
	bid := mustBindingFor(t, st, "张三", "小号", "123")
	rt := &fakeRuntime{}
	api.SetRuntime(map[int64]httpapi.BindingRuntime{bid: rt})

	blockResp := jsonRequest(t, c, "POST", srv.URL+"/api/bindings/"+itoa(bid)+"/blacklist",
		`{"uid":"10086"}`)
	blockResp.Body.Close()
	if blockResp.StatusCode != http.StatusOK {
		t.Fatalf("拉黑状态码 = %d", blockResp.StatusCode)
	}

	unblockResp := jsonRequest(t, c, "POST", srv.URL+"/api/bindings/"+itoa(bid)+"/unblacklist",
		`{"uid":"10086"}`)
	unblockResp.Body.Close()
	if unblockResp.StatusCode != http.StatusOK {
		t.Fatalf("取消拉黑状态码 = %d", unblockResp.StatusCode)
	}

	if len(rt.blacklisted) != 1 || rt.blacklisted[0] != "10086" {
		t.Errorf("blacklisted = %v", rt.blacklisted)
	}
	if len(rt.unblacklisted) != 1 || rt.unblacklisted[0] != "10086" {
		t.Errorf("unblacklisted = %v", rt.unblacklisted)
	}
}

// TestBlacklistRequiresAccountOwnerNotUserBlock 钉住这次任务的核心权限
// 决定：拉黑是账号级操作，走「账号所有者或管理员」（isAccountOwner），
// **不是** user:block 权限点。持有 user:block（普通房管）不该能拉黑——
// 「主播可以禁言和拉黑，房管只能禁言」。
func TestBlacklistRequiresAccountOwnerNotUserBlock(t *testing.T) {
	srv, st, api := newTestServerWithAPI(t)
	loginAs(t, srv, st, "张三", false)
	bid := mustBindingFor(t, st, "张三", "小号", "123")
	api.SetRuntime(map[int64]httpapi.BindingRuntime{bid: &fakeRuntime{}})

	// 李四被授予了 user:block——按房管权限他能禁言，但不该能拉黑
	li := loginAs(t, srv, st, "李四", false)
	if err := st.Grant(context.Background(), "李四", "小号", "123",
		[]perm.Permission{perm.UserBlock}); err != nil {
		t.Fatalf("授权报错: %v", err)
	}

	resp := jsonRequest(t, li, "POST", srv.URL+"/api/bindings/"+itoa(bid)+"/blacklist",
		`{"uid":"10086"}`)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusForbidden {
		t.Errorf("持有 user:block 但非账号所有者，拉黑状态码 = %d, 期望 403", resp.StatusCode)
	}
}

// TestBlacklistNoVisibilityReturns404 与 requirePerm 的约定一致：完全
// 不可见的绑定要回 404 而不是 403，防止靠状态码枚举出部署里有哪些绑定。
func TestBlacklistNoVisibilityReturns404(t *testing.T) {
	srv, st, api := newTestServerWithAPI(t)
	loginAs(t, srv, st, "张三", false)
	bid := mustBindingFor(t, st, "张三", "小号", "123")
	api.SetRuntime(map[int64]httpapi.BindingRuntime{bid: &fakeRuntime{}})

	li := loginAs(t, srv, st, "李四", false)
	resp := jsonRequest(t, li, "POST", srv.URL+"/api/bindings/"+itoa(bid)+"/blacklist",
		`{"uid":"10086"}`)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("无可见性时状态码 = %d, 期望 404", resp.StatusCode)
	}
}

// TestBlacklistAllowsAdmin 管理员总能操作，与其它账号级判定一致。
func TestBlacklistAllowsAdmin(t *testing.T) {
	srv, st, api := newTestServerWithAPI(t)
	loginAs(t, srv, st, "张三", false)
	bid := mustBindingFor(t, st, "张三", "小号", "123")
	api.SetRuntime(map[int64]httpapi.BindingRuntime{bid: &fakeRuntime{}})

	admin := loginAs(t, srv, st, "管理员", true)
	resp := jsonRequest(t, admin, "POST", srv.URL+"/api/bindings/"+itoa(bid)+"/blacklist",
		`{"uid":"10086"}`)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("管理员拉黑状态码 = %d, 期望 200", resp.StatusCode)
	}
}

// TestBlacklistRejectsEmptyUID 与禁言/解禁一致的输入校验。
func TestBlacklistRejectsEmptyUID(t *testing.T) {
	srv, st, api := newTestServerWithAPI(t)
	c := loginAs(t, srv, st, "张三", false)
	bid := mustBindingFor(t, st, "张三", "小号", "123")
	api.SetRuntime(map[int64]httpapi.BindingRuntime{bid: &fakeRuntime{}})

	resp := jsonRequest(t, c, "POST", srv.URL+"/api/bindings/"+itoa(bid)+"/blacklist", `{"uid":"  "}`)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusUnprocessableEntity {
		t.Errorf("状态码 = %d, 期望 422", resp.StatusCode)
	}
}

// TestBlacklistReportsFailure 拉黑失败要如实回报——这是不可逆的对外
// 操作，包装成「操作失败，请重试」等于把唯一有用的信息删掉。
func TestBlacklistReportsFailure(t *testing.T) {
	srv, st, api := newTestServerWithAPI(t)
	c := loginAs(t, srv, st, "张三", false)
	bid := mustBindingFor(t, st, "张三", "小号", "123")
	api.SetRuntime(map[int64]httpapi.BindingRuntime{
		bid: &fakeRuntime{err: errors.New("你已经拉黑过对方了")},
	})

	resp := jsonRequest(t, c, "POST", srv.URL+"/api/bindings/"+itoa(bid)+"/blacklist", `{"uid":"10086"}`)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadGateway {
		t.Errorf("状态码 = %d, 期望 502", resp.StatusCode)
	}
	var body map[string]string
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("解析报错: %v", err)
	}
	if body["error"] == "" || !strings.Contains(body["error"], "拉黑过") {
		t.Errorf("错误信息应原样带上服务端消息，实际: %q", body["error"])
	}
}

// TestBlacklistWhenBindingNotRunning 绑定没在运行时要给出能看懂的说明，
// 与其它即时动作一致。
func TestBlacklistWhenBindingNotRunning(t *testing.T) {
	srv, st, api := newTestServerWithAPI(t)
	c := loginAs(t, srv, st, "张三", false)
	bid := mustBindingFor(t, st, "张三", "小号", "123")
	api.SetRuntime(map[int64]httpapi.BindingRuntime{})

	resp := jsonRequest(t, c, "POST", srv.URL+"/api/bindings/"+itoa(bid)+"/blacklist", `{"uid":"10086"}`)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusServiceUnavailable {
		t.Errorf("状态码 = %d, 期望 503", resp.StatusCode)
	}
}

// TestBlacklistStatus 回读状态：attribute==128 换算出的 blacklisted 与
// 自动回填的昵称都要透出。
func TestBlacklistStatus(t *testing.T) {
	srv, st, api := newTestServerWithAPI(t)
	c := loginAs(t, srv, st, "张三", false)
	bid := mustBindingFor(t, st, "张三", "小号", "123")
	api.SetRuntime(map[int64]httpapi.BindingRuntime{
		bid: &fakeRuntime{blacklistedAttr: true, blacklistedName: "坏人甲"},
	})

	resp := jsonRequest(t, c, "GET", srv.URL+"/api/bindings/"+itoa(bid)+"/blacklist-status?uid=10086", "")
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("状态码 = %d", resp.StatusCode)
	}
	var body struct {
		UID         string `json:"uid"`
		Blacklisted bool   `json:"blacklisted"`
		Nickname    string `json:"nickname"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("解析报错: %v", err)
	}
	if !body.Blacklisted || body.Nickname != "坏人甲" || body.UID != "10086" {
		t.Errorf("body = %+v", body)
	}
}

// TestBlacklistStatusRequiresAccountOwner 状态回读同样是账号级操作，
// 与拉黑本身共用同一道权限判定。
func TestBlacklistStatusRequiresAccountOwner(t *testing.T) {
	srv, st, api := newTestServerWithAPI(t)
	loginAs(t, srv, st, "张三", false)
	bid := mustBindingFor(t, st, "张三", "小号", "123")
	api.SetRuntime(map[int64]httpapi.BindingRuntime{bid: &fakeRuntime{}})

	li := loginAs(t, srv, st, "李四", false)
	if err := st.Grant(context.Background(), "李四", "小号", "123",
		[]perm.Permission{perm.UserBlock}); err != nil {
		t.Fatalf("授权报错: %v", err)
	}

	resp := jsonRequest(t, li, "GET", srv.URL+"/api/bindings/"+itoa(bid)+"/blacklist-status?uid=10086", "")
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusForbidden {
		t.Errorf("状态码 = %d, 期望 403", resp.StatusCode)
	}
}
