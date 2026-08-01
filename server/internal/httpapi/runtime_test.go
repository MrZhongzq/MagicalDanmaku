package httpapi_test

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/MrZhongzq/MagicalDanmaku/server/internal/httpapi"
	"github.com/MrZhongzq/MagicalDanmaku/server/internal/store"
)

func TestRuntimeMetaReportsStale(t *testing.T) {
	srv, st, api := newTestServerWithAPI(t)
	c := loginAs(t, srv, st, "张三", false)
	bid := mustBindingFor(t, st, "张三", "小号", "123")

	// 机器人启动时该绑定的配置哈希，与当前数据库不一致
	api.SetConfigHash(map[int64]string{bid: "启动时的哈希"})
	api.SetRuntime(map[int64]httpapi.BindingRuntime{bid: &fakeRuntime{}})

	resp := jsonRequest(t, c, "GET", srv.URL+"/api/meta/runtime", "")
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("状态码 = %d", resp.StatusCode)
	}

	var body struct {
		ConfigStale bool `json:"configStale"`
		Bindings    []struct {
			ID          int64  `json:"id"`
			State       string `json:"state"`
			Running     bool   `json:"running"`
			ConfigStale bool   `json:"configStale"`
		} `json:"bindings"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("解析报错: %v", err)
	}
	if !body.ConfigStale {
		t.Error("配置哈希不一致时应报告 configStale=true，否则用户会以为改动已生效")
	}
	if len(body.Bindings) != 1 || !body.Bindings[0].Running {
		t.Errorf("绑定运行状态 = %+v", body.Bindings)
	}
	if !body.Bindings[0].ConfigStale {
		t.Error("该绑定自己的 configStale 也应为 true，前端要能指出具体是哪个绑定")
	}
}

func TestRuntimeMetaNotStaleWhenHashMatches(t *testing.T) {
	srv, st, api := newTestServerWithAPI(t)
	c := loginAs(t, srv, st, "张三", false)
	mustBindingFor(t, st, "张三", "小号", "123")

	// 用服务端自己算出来的哈希，模拟「刚启动、还没改过配置」
	current, err := api.CurrentConfigHash(t.Context())
	if err != nil {
		t.Fatalf("算哈希报错: %v", err)
	}
	api.SetConfigHash(current)

	resp := jsonRequest(t, c, "GET", srv.URL+"/api/meta/runtime", "")
	defer resp.Body.Close()

	var body struct {
		ConfigStale bool `json:"configStale"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("解析报错: %v", err)
	}
	if body.ConfigStale {
		t.Error("哈希一致时不该报告 stale")
	}
}

// 按绑定重载，不该把全库都重算进「已生效」的哈希里。
//
// 改了 A、B 两个绑定的配置后，只点了 A 的重载：A 的引擎确实换了，
// B 的引擎完全没变，仍在跑旧规则。若哈希是按全库重算再整体写回，
// A 重载成功后写回的哈希会连带把 B 也标成「已生效」——界面从此
// 声称一切已生效，而 B 仍在跑旧规则，不报错不告警。这正是这个
// 接口本该防的那件事本身。
func TestReloadOnlyClearsStaleForThatBinding(t *testing.T) {
	srv, st, api := newTestServerWithAPI(t)
	c := loginAs(t, srv, st, "张三", false)

	zhang, err := st.GetUserByName(context.Background(), "张三")
	if err != nil {
		t.Fatalf("查用户报错: %v", err)
	}
	acc, err := st.CreateAccount(context.Background(), store.AccountInput{
		Name: "小号", Cookie: "SESSDATA=x", OwnerID: zhang.ID,
		RateLimit: time.Second, MaxLength: 40,
	})
	if err != nil {
		t.Fatalf("建账号报错: %v", err)
	}
	bindingA, err := st.UpsertBinding(context.Background(), acc.ID, "111")
	if err != nil {
		t.Fatalf("建绑定 A 报错: %v", err)
	}
	bindingB, err := st.UpsertBinding(context.Background(), acc.ID, "222")
	if err != nil {
		t.Fatalf("建绑定 B 报错: %v", err)
	}

	fakeA := &fakeRuntime{}
	fakeB := &fakeRuntime{}
	api.SetRuntime(map[int64]httpapi.BindingRuntime{bindingA.ID: fakeA, bindingB.ID: fakeB})

	// 记录启动时的哈希基线：此刻两个绑定都对应数据库当前状态，均不 stale
	baseline, err := api.CurrentConfigHash(context.Background())
	if err != nil {
		t.Fatalf("算哈希报错: %v", err)
	}
	api.SetConfigHash(baseline)

	// 分别改 A 和 B 的冷却组配置——两者都该变 stale
	if err := st.SetCooldownGroups(context.Background(), bindingA.ID,
		map[string]time.Duration{"g": time.Second}); err != nil {
		t.Fatalf("改 A 配置报错: %v", err)
	}
	if err := st.SetCooldownGroups(context.Background(), bindingB.ID,
		map[string]time.Duration{"g": 2 * time.Second}); err != nil {
		t.Fatalf("改 B 配置报错: %v", err)
	}

	// 只重载 A
	resp := jsonRequest(t, c, "POST", srv.URL+"/api/bindings/"+itoa(bindingA.ID)+"/reload", "")
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("重载状态码 = %d", resp.StatusCode)
	}

	meta := jsonRequest(t, c, "GET", srv.URL+"/api/meta/runtime", "")
	defer meta.Body.Close()
	var body struct {
		Bindings []struct {
			ID          int64 `json:"id"`
			ConfigStale bool  `json:"configStale"`
		} `json:"bindings"`
	}
	if err := json.NewDecoder(meta.Body).Decode(&body); err != nil {
		t.Fatalf("解析报错: %v", err)
	}

	staleOf := map[int64]bool{}
	for _, bv := range body.Bindings {
		staleOf[bv.ID] = bv.ConfigStale
	}
	if staleOf[bindingA.ID] {
		t.Errorf("A 已重载，不该再是 stale")
	}
	if !staleOf[bindingB.ID] {
		t.Error("B 没被重载，仍应是 stale——它的引擎还在跑旧规则；" +
			"若这里是 false，说明重载 A 把 B 的哈希也顺带刷新了")
	}
}

func TestRuntimeMetaMarksNotRunning(t *testing.T) {
	srv, st, api := newTestServerWithAPI(t)
	c := loginAs(t, srv, st, "张三", false)
	mustBindingFor(t, st, "张三", "小号", "123")
	api.SetRuntime(map[int64]httpapi.BindingRuntime{}) // 空

	resp := jsonRequest(t, c, "GET", srv.URL+"/api/meta/runtime", "")
	defer resp.Body.Close()

	var body struct {
		Bindings []struct {
			Running bool `json:"running"`
		} `json:"bindings"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("解析报错: %v", err)
	}
	if len(body.Bindings) != 1 || body.Bindings[0].Running {
		t.Errorf("未运行的绑定应标记 running=false: %+v", body.Bindings)
	}
}
