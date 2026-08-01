package httpapi_test

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/MrZhongzq/MagicalDanmaku/server/internal/perm"
)

type metaItem struct {
	Value string `json:"value"`
	Label string `json:"label"`
}

func fetchMeta(t *testing.T, c *http.Client, url string) []metaItem {
	t.Helper()
	resp := jsonRequest(t, c, "GET", url, "")
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("%s 状态码 = %d", url, resp.StatusCode)
	}
	var out []metaItem
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatalf("解析 %s 报错: %v", url, err)
	}
	return out
}

// 元数据接口下发的权限点必须与 perm.All() 完全一致，
// 少一个前端就渲染不出那个复选框
func TestMetaPermissionsMatchesPermPackage(t *testing.T) {
	srv, st := newTestServer(t)
	c := loginAs(t, srv, st, "张三", false)

	got := fetchMeta(t, c, srv.URL+"/api/meta/permissions")
	if len(got) != len(perm.All()) {
		t.Fatalf("权限点数 = %d, 期望 %d", len(got), len(perm.All()))
	}
	have := make(map[string]bool, len(got))
	for _, it := range got {
		have[it.Value] = true
		if it.Label == "" {
			t.Errorf("权限点 %q 缺少中文说明", it.Value)
		}
	}
	for _, p := range perm.All() {
		if !have[string(p)] {
			t.Errorf("元数据缺少权限点 %q", p)
		}
	}
}

func TestMetaEventTypesNonEmpty(t *testing.T) {
	srv, st := newTestServer(t)
	c := loginAs(t, srv, st, "张三", false)

	got := fetchMeta(t, c, srv.URL+"/api/meta/event-types")
	if len(got) < 10 {
		t.Errorf("事件类型太少: %d", len(got))
	}
	have := make(map[string]bool, len(got))
	for _, it := range got {
		have[it.Value] = true
		if it.Label == "" {
			t.Errorf("事件类型 %q 缺少中文说明", it.Value)
		}
	}
	for _, want := range []string{"danmaku", "gift", "guard_buy", "user_enter", "super_chat"} {
		if !have[want] {
			t.Errorf("元数据缺少事件类型 %q", want)
		}
	}
}

// 手动操作（event.TypeManual）已经进 activity_logs 且 eventType="manual"，
// 但日志页的筛选下拉框里选不到它，因为 eventTypeLabels 里漏了这一项。
func TestMetaEventTypesIncludesManual(t *testing.T) {
	srv, st := newTestServer(t)
	c := loginAs(t, srv, st, "张三", false)

	got := fetchMeta(t, c, srv.URL+"/api/meta/event-types")
	for _, it := range got {
		if it.Value == "manual" {
			if it.Label != "手动操作" {
				t.Errorf("manual 的标签 = %q, 期望 手动操作", it.Label)
			}
			return
		}
	}
	t.Error("元数据缺少事件类型 manual（手动操作），日志页筛选框里选不到它")
}

func TestMetaActionTypes(t *testing.T) {
	srv, st := newTestServer(t)
	c := loginAs(t, srv, st, "张三", false)

	got := fetchMeta(t, c, srv.URL+"/api/meta/action-types")
	have := make(map[string]bool, len(got))
	for _, it := range got {
		have[it.Value] = true
	}
	for _, want := range []string{"danmaku", "block", "script", "log"} {
		if !have[want] {
			t.Errorf("元数据缺少动作类型 %q", want)
		}
	}
}

func TestMetaOperators(t *testing.T) {
	srv, st := newTestServer(t)
	c := loginAs(t, srv, st, "张三", false)

	got := fetchMeta(t, c, srv.URL+"/api/meta/operators")
	have := make(map[string]bool, len(got))
	for _, it := range got {
		have[it.Value] = true
	}
	for _, want := range []string{"eq", "ne", "gt", "gte", "lt", "lte",
		"contains", "prefix", "suffix", "regex", "in"} {
		if !have[want] {
			t.Errorf("元数据缺少操作符 %q", want)
		}
	}
}

func TestMetaAggregateBy(t *testing.T) {
	srv, st := newTestServer(t)
	c := loginAs(t, srv, st, "张三", false)

	got := fetchMeta(t, c, srv.URL+"/api/meta/aggregate-by")
	have := make(map[string]bool, len(got))
	for _, it := range got {
		have[it.Value] = true
	}
	for _, want := range []string{"type", "user", "gift"} {
		if !have[want] {
			t.Errorf("元数据缺少分组方式 %q", want)
		}
	}
}

// 元数据不是公开信息，未登录不给看
func TestMetaRequiresAuth(t *testing.T) {
	srv, _ := newTestServer(t)

	for _, path := range []string{
		"/api/meta/permissions", "/api/meta/event-types",
		"/api/meta/action-types", "/api/meta/operators", "/api/meta/aggregate-by",
	} {
		resp, err := http.Get(srv.URL + path)
		if err != nil {
			t.Fatalf("请求 %s 报错: %v", path, err)
		}
		resp.Body.Close()
		if resp.StatusCode != http.StatusUnauthorized {
			t.Errorf("%s 未登录状态码 = %d, 期望 401", path, resp.StatusCode)
		}
	}
}
