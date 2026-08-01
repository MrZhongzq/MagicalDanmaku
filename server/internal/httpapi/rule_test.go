package httpapi_test

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/MrZhongzq/MagicalDanmaku/server/internal/perm"
	"github.com/MrZhongzq/MagicalDanmaku/server/internal/store"
)

const sampleRuleJSON = `{
	"name":"关键词回复",
	"on":["danmaku"],
	"when":{"field":"text","op":"contains","value":"求歌单"},
	"cooldown":"30s",
	"do":[{"type":"danmaku","template":["歌单在动态里"]}]
}`

func grantWrite(t *testing.T, st *store.Store, user, account, room string) {
	t.Helper()
	if err := st.Grant(context.Background(), user, account, room,
		[]perm.Permission{perm.RuleRead, perm.RuleWrite}); err != nil {
		t.Fatalf("授权报错: %v", err)
	}
}

func TestCreateAndListRules(t *testing.T) {
	srv, st := newTestServer(t)
	c := loginAs(t, srv, st, "张三", false)
	bid := mustBindingFor(t, st, "张三", "小号", "123")
	grantWrite(t, st, "张三", "小号", "123")

	create := jsonRequest(t, c, "POST", srv.URL+"/api/bindings/"+itoa(bid)+"/rules", sampleRuleJSON)
	defer create.Body.Close()
	if create.StatusCode != http.StatusCreated {
		t.Fatalf("创建状态码 = %d, 期望 201", create.StatusCode)
	}

	list := jsonRequest(t, c, "GET", srv.URL+"/api/bindings/"+itoa(bid)+"/rules", "")
	defer list.Body.Close()

	var got []map[string]any
	if err := json.NewDecoder(list.Body).Decode(&got); err != nil {
		t.Fatalf("解析报错: %v", err)
	}
	if len(got) != 1 || got[0]["name"] != "关键词回复" {
		t.Errorf("规则列表 = %+v", got)
	}
	// 从列表读回来的规则必须带上 name 与 enabled（它们是列，不在 JSONB 里）
	if _, ok := got[0]["enabled"]; !ok {
		t.Error("列表里的规则缺少 enabled 字段")
	}
}

// 非法规则不许进库：写进去了，机器人每次启动都会炸
func TestCreateRuleRejectsInvalid(t *testing.T) {
	srv, st := newTestServer(t)
	c := loginAs(t, srv, st, "张三", false)
	bid := mustBindingFor(t, st, "张三", "小号", "123")
	grantWrite(t, st, "张三", "小号", "123")

	for _, bad := range []struct {
		name string
		body string
	}{
		{"未知事件类型", `{"name":"x","on":["没有这种事件"],"do":[{"type":"log"}]}`},
		{"空动作列表", `{"name":"x","on":["danmaku"],"do":[]}`},
		{"非法正则", `{"name":"x","on":["danmaku"],"when":{"field":"text","op":"regex","value":"([unclosed"},"do":[{"type":"log"}]}`},
		{"非法 cron", `{"name":"x","schedule":"不是cron","do":[{"type":"log"}]}`},
		{"on 与 schedule 并存", `{"name":"x","on":["danmaku"],"schedule":"0 * * * * *","do":[{"type":"log"}]}`},
	} {
		t.Run(bad.name, func(t *testing.T) {
			resp := jsonRequest(t, c, "POST", srv.URL+"/api/bindings/"+itoa(bid)+"/rules", bad.body)
			defer resp.Body.Close()
			if resp.StatusCode != http.StatusUnprocessableEntity {
				t.Errorf("状态码 = %d, 期望 422", resp.StatusCode)
			}
			var body map[string]string
			_ = json.NewDecoder(resp.Body).Decode(&body)
			if body["error"] == "" {
				t.Error("应给出人能看懂的错误说明")
			}
		})
	}
}

func TestValidateRuleDoesNotSave(t *testing.T) {
	srv, st := newTestServer(t)
	c := loginAs(t, srv, st, "张三", false)
	bid := mustBindingFor(t, st, "张三", "小号", "123")
	grantWrite(t, st, "张三", "小号", "123")

	resp := jsonRequest(t, c, "POST",
		srv.URL+"/api/bindings/"+itoa(bid)+"/rules/validate", sampleRuleJSON)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("状态码 = %d", resp.StatusCode)
	}

	rs, err := st.ListRules(context.Background(), bid)
	if err != nil {
		t.Fatalf("列规则报错: %v", err)
	}
	if len(rs) != 0 {
		t.Errorf("validate 不该保存，实际库里有 %d 条", len(rs))
	}
}

func TestValidateRuleReportsError(t *testing.T) {
	srv, st := newTestServer(t)
	c := loginAs(t, srv, st, "张三", false)
	bid := mustBindingFor(t, st, "张三", "小号", "123")
	grantWrite(t, st, "张三", "小号", "123")

	resp := jsonRequest(t, c, "POST", srv.URL+"/api/bindings/"+itoa(bid)+"/rules/validate",
		`{"name":"x","on":["没有这种事件"],"do":[{"type":"log"}]}`)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusUnprocessableEntity {
		t.Errorf("状态码 = %d, 期望 422", resp.StatusCode)
	}
}

func TestReplaceAllRules(t *testing.T) {
	srv, st := newTestServer(t)
	c := loginAs(t, srv, st, "张三", false)
	bid := mustBindingFor(t, st, "张三", "小号", "123")
	grantWrite(t, st, "张三", "小号", "123")

	first := jsonRequest(t, c, "POST", srv.URL+"/api/bindings/"+itoa(bid)+"/rules", sampleRuleJSON)
	first.Body.Close()

	resp := jsonRequest(t, c, "PUT", srv.URL+"/api/bindings/"+itoa(bid)+"/rules",
		`[{"name":"另一条","on":["gift"],"do":[{"type":"log"}]}]`)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("状态码 = %d", resp.StatusCode)
	}

	rs, err := st.ListRules(context.Background(), bid)
	if err != nil {
		t.Fatalf("列规则报错: %v", err)
	}
	if len(rs) != 1 || rs[0].Name != "另一条" {
		t.Errorf("整组替换后 = %+v", rs)
	}
}

// 整批里有一条非法就整批不落库
func TestReplaceRulesIsAtomic(t *testing.T) {
	srv, st := newTestServer(t)
	c := loginAs(t, srv, st, "张三", false)
	bid := mustBindingFor(t, st, "张三", "小号", "123")
	grantWrite(t, st, "张三", "小号", "123")

	first := jsonRequest(t, c, "POST", srv.URL+"/api/bindings/"+itoa(bid)+"/rules", sampleRuleJSON)
	first.Body.Close()

	resp := jsonRequest(t, c, "PUT", srv.URL+"/api/bindings/"+itoa(bid)+"/rules",
		`[{"name":"好的","on":["gift"],"do":[{"type":"log"}]},
		  {"name":"坏的","on":["没有这种事件"],"do":[{"type":"log"}]}]`)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusUnprocessableEntity {
		t.Fatalf("状态码 = %d, 期望 422", resp.StatusCode)
	}

	rs, err := st.ListRules(context.Background(), bid)
	if err != nil {
		t.Fatalf("列规则报错: %v", err)
	}
	if len(rs) != 1 || rs[0].Name != "关键词回复" {
		t.Errorf("失败的整组替换不该动原有规则，实际 %+v", rs)
	}
}

// 整组替换里出现重名，要指出是第几条。
//
// ReplaceRules 内部也会拒，但它只知道名字不知道第几条。整批失败却
// 不说哪条错，在一屏几十条规则里是很难查的。
func TestReplaceRulesReportsDuplicateName(t *testing.T) {
	srv, st := newTestServer(t)
	c := loginAs(t, srv, st, "张三", false)
	bid := mustBindingFor(t, st, "张三", "小号", "123")
	grantWrite(t, st, "张三", "小号", "123")

	resp := jsonRequest(t, c, "PUT", srv.URL+"/api/bindings/"+itoa(bid)+"/rules",
		`[{"name":"甲","on":["gift"],"do":[{"type":"log"}]},
		  {"name":"乙","on":["gift"],"do":[{"type":"log"}]},
		  {"name":"甲","on":["gift"],"do":[{"type":"log"}]}]`)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusUnprocessableEntity {
		t.Fatalf("状态码 = %d, 期望 422", resp.StatusCode)
	}

	var body map[string]string
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("解析报错: %v", err)
	}
	// 要说清是第 3 条，不能只说「名字重复」
	if !strings.Contains(body["error"], "第 3 条") {
		t.Errorf("错误文案应指出是第 3 条，实际: %q", body["error"])
	}
}

// 全批次终审项【2a】：PUT 整组规则时，Suppress 指向的名字必须在这一批
// 里存在。
//
// 修复前只有 rules.NewEngine 做这项检查，而那要等到 POST /reload 才会
// 触发——PUT 本身会成功返回 200，库里就此存了一份非法配置。这是
// review 描述的两条现实可达路径之一：全新安装时，内置的七条规则只有
// 弹幕姬页保存过一次之后才真实存在，自定义弹幕姬页却可以更早地引用
// 一个还不存在的内置规则名。
func TestReplaceRulesRejectsInvalidSuppressReference(t *testing.T) {
	srv, st := newTestServer(t)
	c := loginAs(t, srv, st, "张三", false)
	bid := mustBindingFor(t, st, "张三", "小号", "123")
	grantWrite(t, st, "张三", "小号", "123")

	resp := jsonRequest(t, c, "PUT", srv.URL+"/api/bindings/"+itoa(bid)+"/rules",
		`[{"name":"只欢迎舰长","on":["user_enter"],"do":[{"type":"log"}],"suppress":["不存在的规则"]}]`)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusUnprocessableEntity {
		t.Fatalf("状态码 = %d, 期望 422", resp.StatusCode)
	}

	rs, err := st.ListRules(context.Background(), bid)
	if err != nil {
		t.Fatalf("列规则报错: %v", err)
	}
	if len(rs) != 0 {
		t.Errorf("非法的整组替换不该落库，实际 %d 条", len(rs))
	}
}

// suppress 引用同一批里确实存在的名字应当放行——不能矫枉过正。
func TestReplaceRulesAcceptsValidSuppressReference(t *testing.T) {
	srv, st := newTestServer(t)
	c := loginAs(t, srv, st, "张三", false)
	bid := mustBindingFor(t, st, "张三", "小号", "123")
	grantWrite(t, st, "张三", "小号", "123")

	resp := jsonRequest(t, c, "PUT", srv.URL+"/api/bindings/"+itoa(bid)+"/rules",
		`[{"name":"内置/进房欢迎","on":["user_enter"],"do":[{"type":"log"}]},
		  {"name":"只欢迎舰长","on":["user_enter"],"do":[{"type":"log"}],"suppress":["内置/进房欢迎"]}]`)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("状态码 = %d, 期望 200", resp.StatusCode)
	}

	rs, err := st.ListRules(context.Background(), bid)
	if err != nil {
		t.Fatalf("列规则报错: %v", err)
	}
	if len(rs) != 2 {
		t.Errorf("合法的整组替换应该落库，实际 %d 条", len(rs))
	}
}

// 全批次终审项【2b】：DELETE 一条被别的规则 Suppress 引用的规则应当被拒。
//
// 修复前没有任何检查会拦住这个操作——删掉之后，引用它的那条规则的
// suppress 就指向一个不存在的名字，库被写脏，要等到下次启动
// NewEngine 才会报错。
func TestDeleteRuleRejectsWhenSuppressedByAnotherRule(t *testing.T) {
	srv, st := newTestServer(t)
	c := loginAs(t, srv, st, "张三", false)
	bid := mustBindingFor(t, st, "张三", "小号", "123")
	grantWrite(t, st, "张三", "小号", "123")

	rules := srv.URL + "/api/bindings/" + itoa(bid) + "/rules"
	base := jsonRequest(t, c, "POST", rules,
		`{"name":"进房欢迎","on":["user_enter"],"do":[{"type":"log"}]}`)
	base.Body.Close()
	dependent := jsonRequest(t, c, "POST", rules,
		`{"name":"只欢迎舰长","on":["user_enter"],"do":[{"type":"log"}],"suppress":["进房欢迎"]}`)
	dependent.Body.Close()

	resp := jsonRequest(t, c, "DELETE", rules+"/进房欢迎", "")
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusUnprocessableEntity {
		t.Fatalf("状态码 = %d, 期望 422", resp.StatusCode)
	}
	var body map[string]string
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("解析报错: %v", err)
	}
	// 要说清是被哪条规则压制引用的，让用户知道去改哪一条
	if !strings.Contains(body["error"], "只欢迎舰长") {
		t.Errorf("错误文案应指出是哪条规则在压制引用，实际: %q", body["error"])
	}

	rs, err := st.ListRules(context.Background(), bid)
	if err != nil {
		t.Fatalf("列规则报错: %v", err)
	}
	if len(rs) != 2 {
		t.Errorf("被拒的删除不该真的删掉规则，实际剩 %d 条", len(rs))
	}
}

// 没有被任何规则压制引用的规则应当能正常删除——不能矫枉过正。
func TestDeleteRuleAllowsWhenNotSuppressedByAnyRule(t *testing.T) {
	srv, st := newTestServer(t)
	c := loginAs(t, srv, st, "张三", false)
	bid := mustBindingFor(t, st, "张三", "小号", "123")
	grantWrite(t, st, "张三", "小号", "123")

	create := jsonRequest(t, c, "POST", srv.URL+"/api/bindings/"+itoa(bid)+"/rules", sampleRuleJSON)
	create.Body.Close()

	resp := jsonRequest(t, c, "DELETE", srv.URL+"/api/bindings/"+itoa(bid)+"/rules/关键词回复", "")
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("状态码 = %d, 期望 200", resp.StatusCode)
	}
}

// PUT 一条**新**规则，要排到最后而不是插到最前。
//
// 位置决定谁先触发。写成 `pos := 0` 的话，PUT 一条新规则会插到现有
// 第一条前面——用户没要求改顺序，顺序却变了，而且不报任何错。
func TestPutNewRuleAppendsInsteadOfPrepending(t *testing.T) {
	srv, st := newTestServer(t)
	c := loginAs(t, srv, st, "张三", false)
	bid := mustBindingFor(t, st, "张三", "小号", "123")
	grantWrite(t, st, "张三", "小号", "123")

	first := jsonRequest(t, c, "POST", srv.URL+"/api/bindings/"+itoa(bid)+"/rules", sampleRuleJSON)
	first.Body.Close()

	resp := jsonRequest(t, c, "PUT", srv.URL+"/api/bindings/"+itoa(bid)+"/rules/后来的",
		`{"on":["gift"],"do":[{"type":"log"}]}`)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("状态码 = %d", resp.StatusCode)
	}

	rs, err := st.ListRules(context.Background(), bid) // 按 position 排序
	if err != nil {
		t.Fatalf("列规则报错: %v", err)
	}
	if len(rs) != 2 {
		t.Fatalf("规则数 = %d, 期望 2", len(rs))
	}
	if rs[0].Name != "关键词回复" || rs[1].Name != "后来的" {
		t.Errorf("顺序 = [%s, %s], 期望 [关键词回复, 后来的]——"+
			"PUT 一条新规则不该插到现有规则前面", rs[0].Name, rs[1].Name)
	}
}

// PUT 覆盖一条**已有**规则，要保住它原来的位置
func TestPutExistingRuleKeepsPosition(t *testing.T) {
	srv, st := newTestServer(t)
	c := loginAs(t, srv, st, "张三", false)
	bid := mustBindingFor(t, st, "张三", "小号", "123")
	grantWrite(t, st, "张三", "小号", "123")

	rules := srv.URL + "/api/bindings/" + itoa(bid) + "/rules"
	for _, n := range []string{"甲", "乙", "丙"} {
		r := jsonRequest(t, c, "POST", rules,
			`{"name":"`+n+`","on":["gift"],"do":[{"type":"log"}]}`)
		r.Body.Close()
	}

	resp := jsonRequest(t, c, "PUT", rules+"/乙",
		`{"on":["danmaku"],"do":[{"type":"log"}]}`)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("状态码 = %d", resp.StatusCode)
	}

	rs, err := st.ListRules(context.Background(), bid)
	if err != nil {
		t.Fatalf("列规则报错: %v", err)
	}
	if len(rs) != 3 || rs[0].Name != "甲" || rs[1].Name != "乙" || rs[2].Name != "丙" {
		t.Errorf("覆盖已有规则不该改变顺序，实际 %+v", rs)
	}
}

func TestToggleRuleEnabled(t *testing.T) {
	srv, st := newTestServer(t)
	c := loginAs(t, srv, st, "张三", false)
	bid := mustBindingFor(t, st, "张三", "小号", "123")
	grantWrite(t, st, "张三", "小号", "123")

	create := jsonRequest(t, c, "POST", srv.URL+"/api/bindings/"+itoa(bid)+"/rules", sampleRuleJSON)
	create.Body.Close()

	resp := jsonRequest(t, c, "PATCH",
		srv.URL+"/api/bindings/"+itoa(bid)+"/rules/关键词回复", `{"enabled":false}`)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("状态码 = %d", resp.StatusCode)
	}

	rec, err := st.GetRule(context.Background(), bid, "关键词回复")
	if err != nil {
		t.Fatalf("查规则报错: %v", err)
	}
	if rec.Enabled {
		t.Error("应已停用")
	}
}

func TestDeleteRule(t *testing.T) {
	srv, st := newTestServer(t)
	c := loginAs(t, srv, st, "张三", false)
	bid := mustBindingFor(t, st, "张三", "小号", "123")
	grantWrite(t, st, "张三", "小号", "123")

	create := jsonRequest(t, c, "POST", srv.URL+"/api/bindings/"+itoa(bid)+"/rules", sampleRuleJSON)
	create.Body.Close()

	resp := jsonRequest(t, c, "DELETE", srv.URL+"/api/bindings/"+itoa(bid)+"/rules/关键词回复", "")
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("状态码 = %d", resp.StatusCode)
	}

	rs, err := st.ListRules(context.Background(), bid)
	if err != nil {
		t.Fatalf("列规则报错: %v", err)
	}
	if len(rs) != 0 {
		t.Errorf("删除后应为空，实际 %d 条", len(rs))
	}
}

// 每个写接口都要有「只读权限被拒」的测试
//
// 被拒的是李四：他只有 rule:read，既不是管理员也不是「小号」的
// 所有者（所有者是张三）。Task 8 给 store.Can 加了账号所有者通路，
// 账号所有者对自己账号下的绑定自动拥有除 member:manage 外的全部
// 权限点——如果这里让张三来充当「被拒」的一方，测试会在实现完成后
// 假绿：张三作为所有者其实一直有 rule:write，403 从来不会真的发生。
func TestRuleWriteEndpointsRejectReadOnly(t *testing.T) {
	srv, st := newTestServer(t)
	loginAs(t, srv, st, "张三", false)
	bid := mustBindingFor(t, st, "张三", "小号", "123")

	li := loginAs(t, srv, st, "李四", false)
	if err := st.Grant(context.Background(), "李四", "小号", "123",
		[]perm.Permission{perm.RuleRead}); err != nil {
		t.Fatalf("授权报错: %v", err)
	}

	base := srv.URL + "/api/bindings/" + itoa(bid) + "/rules"
	for _, tc := range []struct{ method, url, body string }{
		{"POST", base, sampleRuleJSON},
		{"PUT", base, `[]`},
		{"PUT", base + "/关键词回复", sampleRuleJSON},
		{"PATCH", base + "/关键词回复", `{"enabled":false}`},
		{"DELETE", base + "/关键词回复", ""},
	} {
		resp := jsonRequest(t, li, tc.method, tc.url, tc.body)
		resp.Body.Close()
		if resp.StatusCode != http.StatusForbidden {
			t.Errorf("%s %s 状态码 = %d, 期望 403", tc.method, tc.url, resp.StatusCode)
		}
	}
}

// 被拒的是李四，同上一条的理由：不能用所有者张三来验证「无权限被拒」。
//
// 另外李四必须对这个绑定保有某种可见性（这里给了 event:read，
// 与 rule:read 无关的另一个权限点），否则守卫会按 guard.go 的
// 设计把「完全不可见」判成 404 而不是 403——见
// guard_test.go 的 TestGuardRejectsWithoutPermission。这条测试要
// 验证的是「可见但缺 rule:read」这一具体情形，用 404 场景来测会
// 文不对题。
func TestListRulesRejectsNoPermission(t *testing.T) {
	srv, st := newTestServer(t)
	loginAs(t, srv, st, "张三", false)
	bid := mustBindingFor(t, st, "张三", "小号", "123")

	li := loginAs(t, srv, st, "李四", false)
	if err := st.Grant(context.Background(), "李四", "小号", "123",
		[]perm.Permission{perm.EventRead}); err != nil {
		t.Fatalf("授权报错: %v", err)
	}
	resp := jsonRequest(t, li, "GET", srv.URL+"/api/bindings/"+itoa(bid)+"/rules", "")
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusForbidden {
		t.Errorf("状态码 = %d, 期望 403", resp.StatusCode)
	}
}
