package httpapi_test

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/MrZhongzq/MagicalDanmaku/server/internal/event"
	"github.com/MrZhongzq/MagicalDanmaku/server/internal/perm"
	"github.com/MrZhongzq/MagicalDanmaku/server/internal/rules"
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

// TestMetaEventTypesNonEmpty 曾经是白名单式弱断言（只检查「至少包含
// 哪几个」+「总数不少于 10」），跟 TestMetaAggregateBy 修复前是同一类
// 结构性缺陷：新增或删除一个 event.Type 都不会让它报红，除非恰好撞在
// 白名单列出的那几个值上——这正是本文件里 TestMetaEventTypesIncludesManual
// /TestMetaEventTypesIncludesPKVisitDirections 这两条「事后补丁」测试
// 存在的原因，它们是这个弱点被踩过之后一条条追加上去的。改成跟
// event.AllTypes() 逐条双向对照之后，这两条补丁测试的断言范围已被
// 完全覆盖，但保留它们不撤——各自的注释记录了当年具体踩过的坑，删掉
// 会丢失这段历史。
func TestMetaEventTypesNonEmpty(t *testing.T) {
	srv, st := newTestServer(t)
	c := loginAs(t, srv, st, "张三", false)

	got := fetchMeta(t, c, srv.URL+"/api/meta/event-types")
	all := event.AllTypes()
	if len(got) != len(all) {
		t.Fatalf("事件类型数 = %d, 期望 %d", len(got), len(all))
	}
	have := make(map[string]bool, len(got))
	for _, it := range got {
		have[it.Value] = true
		if it.Label == "" {
			t.Errorf("事件类型 %q 缺少中文说明", it.Value)
		}
	}
	for _, want := range all {
		if !have[string(want)] {
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

// TestMetaEventTypesIncludesPKVisitDirections 是 Task 7 三处登记之一：
// 缺了这一处，规则页的 on 下拉框里选不到这两个类型，用户没法配出订阅
// PK 串门信号的规则——即使规则层本身已经支持（spec/convert.go 的
// knownEventTypes 也登记了它们）。
func TestMetaEventTypesIncludesPKVisitDirections(t *testing.T) {
	srv, st := newTestServer(t)
	c := loginAs(t, srv, st, "张三", false)

	got := fetchMeta(t, c, srv.URL+"/api/meta/event-types")
	have := make(map[string]string, len(got))
	for _, it := range got {
		have[it.Value] = it.Label
	}
	for _, want := range []string{"pk_visit_from_opponent", "pk_visit_to_opponent"} {
		label, ok := have[want]
		if !ok {
			t.Errorf("元数据缺少事件类型 %q", want)
			continue
		}
		if label == "" {
			t.Errorf("事件类型 %q 缺少中文说明", want)
		}
	}
}

// TestMetaActionTypes 是白名单式弱断言改成强断言的第二处（第一处见
// TestMetaAggregateBy 上方的说明）：跟 rules.AllActionTypes() 逐条
// 双向对照，而不是只检查「至少包含哪几个」——后者的结构性缺陷是新增
// 动作类型时即使 meta_handler.go 忘了同步，这条测试也不会报红。
func TestMetaActionTypes(t *testing.T) {
	srv, st := newTestServer(t)
	c := loginAs(t, srv, st, "张三", false)

	got := fetchMeta(t, c, srv.URL+"/api/meta/action-types")
	all := rules.AllActionTypes()
	if len(got) != len(all) {
		t.Fatalf("动作类型数 = %d, 期望 %d", len(got), len(all))
	}
	have := make(map[string]bool, len(got))
	for _, it := range got {
		have[it.Value] = true
		if it.Label == "" {
			t.Errorf("动作类型 %q 缺少中文说明", it.Value)
		}
	}
	for _, want := range all {
		if !have[string(want)] {
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

// variablesResponse 镜像 /api/meta/variables 的响应结构。
type variablesResponse struct {
	Common  []variableItem            `json:"common"`
	ByEvent map[string][]variableItem `json:"byEvent"`
}

type variableItem struct {
	Path     string `json:"path"`
	Label    string `json:"label"`
	Optional bool   `json:"optional"`
}

func fetchMetaVariables(t *testing.T, c *http.Client, url string) variablesResponse {
	t.Helper()
	resp := jsonRequest(t, c, "GET", url, "")
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("%s 状态码 = %d", url, resp.StatusCode)
	}
	var out variablesResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatalf("解析 %s 报错: %v", url, err)
	}
	return out
}

// 前端条件构建器手抄的那份变量清单（ConditionTree.vue 的
// COMMON_FIELD_OPTIONS）就是本任务要消灭的第二处定义：这个接口必须
// 把 rules.VariableCatalog() 原样下发，公共字段与按事件类型分组的
// 字段都要能取到。
func TestMetaVariables(t *testing.T) {
	srv, st := newTestServer(t)
	c := loginAs(t, srv, st, "张三", false)

	got := fetchMetaVariables(t, c, srv.URL+"/api/meta/variables")

	commonPaths := make(map[string]bool, len(got.Common))
	for _, v := range got.Common {
		commonPaths[v.Path] = true
		if v.Label == "" {
			t.Errorf("公共变量 %q 缺少中文说明", v.Path)
		}
	}
	for _, want := range []string{"type", "roomId", "timestamp"} {
		if !commonPaths[want] {
			t.Errorf("公共变量缺少 %q", want)
		}
	}

	danmakuVars, ok := got.ByEvent["danmaku"]
	if !ok || len(danmakuVars) == 0 {
		t.Fatal("按事件类型分组的变量缺少 danmaku")
	}
	danmakuPaths := make(map[string]bool, len(danmakuVars))
	for _, v := range danmakuVars {
		danmakuPaths[v.Path] = true
		if v.Label == "" {
			t.Errorf("danmaku 下的变量 %q 缺少中文说明", v.Path)
		}
	}
	for _, want := range []string{"user.uid", "text", "danmaku.color"} {
		if !danmakuPaths[want] {
			t.Errorf("danmaku 分组缺少变量 %q", want)
		}
	}
	// 礼物特有的字段不该出现在弹幕分组下：选了「弹幕」事件后不该还能
	// 配出用 gift.name 的条件。
	if danmakuPaths["gift.name"] {
		t.Error("danmaku 分组不应包含 gift.name")
	}

	giftVars, ok := got.ByEvent["gift"]
	if !ok || len(giftVars) == 0 {
		t.Fatal("按事件类型分组的变量缺少 gift")
	}
	giftPaths := make(map[string]bool, len(giftVars))
	for _, v := range giftVars {
		giftPaths[v.Path] = true
	}
	for _, want := range []string{"user.uid", "gift.name", "gift.count"} {
		if !giftPaths[want] {
			t.Errorf("gift 分组缺少变量 %q", want)
		}
	}
}

// TestMetaAggregateBy 是终审 Important-1 的回归测试。
//
// 旧版本只检查「至少包含 type/user/gift 这三个」——这是一种结构上不
// 可能发现新增值漏登记的白名单式断言：aggregateByLabels 曾经是一份
// 独立于 rules.AggregateBy 常量的手抄清单，Task 3 新增
// AggregateByBlindBox 之后，这份清单没有同步，导致自定义规则页的
// 「分组方式」下拉框里选不出「盲盒」，而这条测试全程绿灯，因为它压根
// 不会去检查「清单里是不是有多出来的项漏了」。
//
// 改成跟 TestMetaPermissionsMatchesPermPackage 同一种强度的写法：先比
// len，再跟权威来源（rules.AllAggregateBy()）逐条双向对照——多一个、
// 少一个都会报红。
func TestMetaAggregateBy(t *testing.T) {
	srv, st := newTestServer(t)
	c := loginAs(t, srv, st, "张三", false)

	got := fetchMeta(t, c, srv.URL+"/api/meta/aggregate-by")
	all := rules.AllAggregateBy()
	if len(got) != len(all) {
		t.Fatalf("分组方式数 = %d, 期望 %d", len(got), len(all))
	}
	have := make(map[string]bool, len(got))
	for _, it := range got {
		have[it.Value] = true
		if it.Label == "" {
			t.Errorf("分组方式 %q 缺少中文说明", it.Value)
		}
	}
	for _, want := range all {
		if !have[string(want)] {
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
		"/api/meta/variables",
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
