package httpapi_test

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/MrZhongzq/MagicalDanmaku/server/internal/perm"
)

func TestCooldownGroupsRoundTrip(t *testing.T) {
	srv, st := newTestServer(t)
	c := loginAs(t, srv, st, "张三", false)
	bid := mustBindingFor(t, st, "张三", "小号", "123")
	grantWrite(t, st, "张三", "小号", "123")

	put := jsonRequest(t, c, "PUT", srv.URL+"/api/bindings/"+itoa(bid)+"/cooldown-groups",
		`{"greeting":5000,"thanks":2000}`)
	defer put.Body.Close()
	if put.StatusCode != http.StatusOK {
		t.Fatalf("状态码 = %d", put.StatusCode)
	}

	get := jsonRequest(t, c, "GET", srv.URL+"/api/bindings/"+itoa(bid)+"/cooldown-groups", "")
	defer get.Body.Close()

	var got map[string]int64
	if err := json.NewDecoder(get.Body).Decode(&got); err != nil {
		t.Fatalf("解析报错: %v", err)
	}
	if got["greeting"] != 5000 || got["thanks"] != 2000 {
		t.Errorf("冷却组 = %v", got)
	}

	inDB, err := st.CooldownGroups(context.Background(), bid)
	if err != nil {
		t.Fatalf("查冷却组报错: %v", err)
	}
	if inDB["greeting"] != 5*time.Second {
		t.Errorf("库里的 greeting = %v, 期望 5s", inDB["greeting"])
	}
}

// 整组替换：从界面删掉一个组，它就该真的消失
func TestCooldownGroupsReplaceRemoves(t *testing.T) {
	srv, st := newTestServer(t)
	c := loginAs(t, srv, st, "张三", false)
	bid := mustBindingFor(t, st, "张三", "小号", "123")
	grantWrite(t, st, "张三", "小号", "123")

	r1 := jsonRequest(t, c, "PUT", srv.URL+"/api/bindings/"+itoa(bid)+"/cooldown-groups",
		`{"greeting":5000,"thanks":2000}`)
	r1.Body.Close()
	r2 := jsonRequest(t, c, "PUT", srv.URL+"/api/bindings/"+itoa(bid)+"/cooldown-groups",
		`{"greeting":1000}`)
	r2.Body.Close()

	got, err := st.CooldownGroups(context.Background(), bid)
	if err != nil {
		t.Fatalf("查冷却组报错: %v", err)
	}
	if len(got) != 1 || got["greeting"] != time.Second {
		t.Errorf("整组替换后 = %v", got)
	}
}

func TestCooldownGroupsRejectNegative(t *testing.T) {
	srv, st := newTestServer(t)
	c := loginAs(t, srv, st, "张三", false)
	bid := mustBindingFor(t, st, "张三", "小号", "123")
	grantWrite(t, st, "张三", "小号", "123")

	resp := jsonRequest(t, c, "PUT", srv.URL+"/api/bindings/"+itoa(bid)+"/cooldown-groups",
		`{"greeting":-1}`)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusUnprocessableEntity {
		t.Errorf("负数间隔状态码 = %d, 期望 422", resp.StatusCode)
	}
}

// 大到会让 time.Duration 溢出的毫秒数必须被拦住。
//
// time.Duration 是 int64 纳秒，`time.Duration(ms) * time.Millisecond`
// 在 ms 超过约 9.2e12 时溢出成负数——只查 ms < 0 是拦不住的，
// 溢出发生在那道检查**之后**。这条测试钉的就是那个缺口。
func TestCooldownGroupsRejectOverflow(t *testing.T) {
	srv, st := newTestServer(t)
	c := loginAs(t, srv, st, "张三", false)
	bid := mustBindingFor(t, st, "张三", "小号", "123")
	grantWrite(t, st, "张三", "小号", "123")

	url := srv.URL + "/api/bindings/" + itoa(bid) + "/cooldown-groups"
	// 9223372036854 毫秒 ≈ 2^63/1e6，乘以 time.Millisecond 后翻负
	resp := jsonRequest(t, c, "PUT", url, `{"greeting":9223372036854}`)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusUnprocessableEntity {
		t.Fatalf("溢出级间隔状态码 = %d, 期望 422", resp.StatusCode)
	}

	// 真正要防的是它变成负数存进去
	groups, err := st.CooldownGroups(context.Background(), bid)
	if err != nil {
		t.Fatalf("读冷却组报错: %v", err)
	}
	if d, ok := groups["greeting"]; ok {
		t.Errorf("不该被存下来，实际存了 %v", d)
	}
}

// 冷却组名要按 trim 后的形式存。
//
// 规则体里写的是 cooldownGroup: 甲，这边原样存成 "  甲  " 就永远
// 匹配不上，而且不报任何错——用户只会看到「冷却怎么不生效」。
func TestCooldownGroupNamesAreTrimmed(t *testing.T) {
	srv, st := newTestServer(t)
	c := loginAs(t, srv, st, "张三", false)
	bid := mustBindingFor(t, st, "张三", "小号", "123")
	grantWrite(t, st, "张三", "小号", "123")

	resp := jsonRequest(t, c, "PUT", srv.URL+"/api/bindings/"+itoa(bid)+"/cooldown-groups",
		`{"  问候  ":5000}`)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("状态码 = %d", resp.StatusCode)
	}

	groups, err := st.CooldownGroups(context.Background(), bid)
	if err != nil {
		t.Fatalf("读冷却组报错: %v", err)
	}
	if _, ok := groups["问候"]; !ok {
		t.Errorf("应以 trim 后的名字存储，实际存了 %v", groups)
	}
}

func TestCooldownGroupsRequirePermissions(t *testing.T) {
	srv, st := newTestServer(t)
	loginAs(t, srv, st, "张三", false)
	bid := mustBindingFor(t, st, "张三", "小号", "123")

	li := loginAs(t, srv, st, "李四", false)
	if err := st.Grant(context.Background(), "李四", "小号", "123",
		[]perm.Permission{perm.RuleRead}); err != nil {
		t.Fatalf("授权报错: %v", err)
	}

	get := jsonRequest(t, li, "GET", srv.URL+"/api/bindings/"+itoa(bid)+"/cooldown-groups", "")
	get.Body.Close()
	if get.StatusCode != http.StatusOK {
		t.Errorf("rule:read 应能读，状态码 = %d", get.StatusCode)
	}

	put := jsonRequest(t, li, "PUT", srv.URL+"/api/bindings/"+itoa(bid)+"/cooldown-groups", `{}`)
	put.Body.Close()
	if put.StatusCode != http.StatusForbidden {
		t.Errorf("只有 rule:read 时写状态码 = %d, 期望 403", put.StatusCode)
	}
}

func TestBlockListCRUD(t *testing.T) {
	srv, st := newTestServer(t)
	c := loginAs(t, srv, st, "张三", false)
	bid := mustBindingFor(t, st, "张三", "小号", "123")
	if err := st.Grant(context.Background(), "张三", "小号", "123",
		[]perm.Permission{perm.UserBlock}); err != nil {
		t.Fatalf("授权报错: %v", err)
	}

	base := srv.URL + "/api/bindings/" + itoa(bid) + "/blocklist"

	add := jsonRequest(t, c, "POST", base,
		`{"uid":"10086","username":"广告号","reason":"刷屏加群"}`)
	defer add.Body.Close()
	if add.StatusCode != http.StatusCreated {
		t.Fatalf("状态码 = %d, 期望 201", add.StatusCode)
	}

	list := jsonRequest(t, c, "GET", base, "")
	defer list.Body.Close()
	var got []struct {
		UID      string `json:"uid"`
		Username string `json:"username"`
		Reason   string `json:"reason"`
	}
	if err := json.NewDecoder(list.Body).Decode(&got); err != nil {
		t.Fatalf("解析报错: %v", err)
	}
	if len(got) != 1 || got[0].UID != "10086" || got[0].Reason != "刷屏加群" {
		t.Errorf("名单 = %+v", got)
	}

	del := jsonRequest(t, c, "DELETE", base+"/10086", "")
	defer del.Body.Close()
	if del.StatusCode != http.StatusOK {
		t.Fatalf("删除状态码 = %d", del.StatusCode)
	}

	blocked, err := st.IsBlocked(context.Background(), bid, "10086")
	if err != nil {
		t.Fatalf("查名单报错: %v", err)
	}
	if blocked {
		t.Error("删除后不该还在名单里")
	}
}

func TestBlockListRecordsOperator(t *testing.T) {
	srv, st := newTestServer(t)
	c := loginAs(t, srv, st, "张三", false)
	bid := mustBindingFor(t, st, "张三", "小号", "123")
	if err := st.Grant(context.Background(), "张三", "小号", "123",
		[]perm.Permission{perm.UserBlock}); err != nil {
		t.Fatalf("授权报错: %v", err)
	}

	add := jsonRequest(t, c, "POST", srv.URL+"/api/bindings/"+itoa(bid)+"/blocklist",
		`{"uid":"10086"}`)
	add.Body.Close()

	list, err := st.ListBlockList(context.Background(), bid)
	if err != nil {
		t.Fatalf("查名单报错: %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("名单长度 = %d", len(list))
	}
	if list[0].CreatedBy == nil {
		t.Error("应记录操作人")
	}
}

func TestBlockListRequiresUserBlock(t *testing.T) {
	srv, st := newTestServer(t)
	loginAs(t, srv, st, "张三", false)
	bid := mustBindingFor(t, st, "张三", "小号", "123")

	li := loginAs(t, srv, st, "李四", false)
	if err := st.Grant(context.Background(), "李四", "小号", "123",
		[]perm.Permission{perm.RuleWrite}); err != nil {
		t.Fatalf("授权报错: %v", err)
	}

	base := srv.URL + "/api/bindings/" + itoa(bid) + "/blocklist"
	for _, tc := range []struct{ method, url, body string }{
		{"GET", base, ""},
		{"POST", base, `{"uid":"1"}`},
		{"DELETE", base + "/1", ""},
	} {
		resp := jsonRequest(t, li, tc.method, tc.url, tc.body)
		resp.Body.Close()
		if resp.StatusCode != http.StatusForbidden {
			t.Errorf("%s %s 状态码 = %d, 期望 403", tc.method, tc.url, resp.StatusCode)
		}
	}
}

func TestBlockListRejectsEmptyUID(t *testing.T) {
	srv, st := newTestServer(t)
	c := loginAs(t, srv, st, "张三", false)
	bid := mustBindingFor(t, st, "张三", "小号", "123")
	if err := st.Grant(context.Background(), "张三", "小号", "123",
		[]perm.Permission{perm.UserBlock}); err != nil {
		t.Fatalf("授权报错: %v", err)
	}

	resp := jsonRequest(t, c, "POST", srv.URL+"/api/bindings/"+itoa(bid)+"/blocklist", `{"uid":"  "}`)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusUnprocessableEntity {
		t.Errorf("状态码 = %d, 期望 422", resp.StatusCode)
	}
}
