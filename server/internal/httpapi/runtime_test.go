package httpapi_test

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/MrZhongzq/MagicalDanmaku/server/internal/httpapi"
)

func TestRuntimeMetaReportsStale(t *testing.T) {
	srv, st, api := newTestServerWithAPI(t)
	c := loginAs(t, srv, st, "张三", false)
	bid := mustBindingFor(t, st, "张三", "小号", "123")

	// 机器人启动时的配置哈希，与当前数据库不一致
	api.SetConfigHash("启动时的哈希")
	api.SetRuntime(map[int64]httpapi.BindingRuntime{bid: &fakeRuntime{}})

	resp := jsonRequest(t, c, "GET", srv.URL+"/api/meta/runtime", "")
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("状态码 = %d", resp.StatusCode)
	}

	var body struct {
		ConfigStale bool `json:"configStale"`
		Bindings    []struct {
			ID      int64  `json:"id"`
			State   string `json:"state"`
			Running bool   `json:"running"`
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
