package api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/MrZhongzq/MagicalDanmaku/server/internal/connector/bilibili/auth"
)

// newTestClientForRelation 建一个指向假服务器的 Client，三个新接口
// （relationModify/accRelation/accInfo）全指到同一个假服务器。
func newTestClientForRelation(t *testing.T, h http.HandlerFunc) (*Client, *httptest.Server) {
	t.Helper()
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)

	sess, err := auth.ParseSession(testCookie)
	if err != nil {
		t.Fatalf("ParseSession 失败: %v", err)
	}
	c := New(sess, WithHTTPClient(srv.Client()))
	c.Signer().SetMixinKey("abcdefghijklmnopqrstuvwxyz012345")
	for _, n := range []string{"relationModify", "accRelation", "accInfo"} {
		c.SetBaseURL(n, srv.URL)
	}
	return c, srv
}

// TestClientBlacklistSendsDocumentedParams 钉住 HAR 实测的全部表单字段。
//
// 这些字段照抄自 task-p5-6-blackapi.md 的抓包记录，不做任何精简——
// 少发一个字段换来的是「有时候成功有时候被风控」，比多发几个字段的
// 代价高得多。act=5 是拉黑；这条测试如果 act 被错改成 6（取消拉黑）
// 会失败，是「自检变异 (a)」要钉住的第一条防线。
func TestClientBlacklistSendsDocumentedParams(t *testing.T) {
	var gotForm url.Values
	var gotQuery url.Values
	c, srv := newTestClientForRelation(t, func(w http.ResponseWriter, r *http.Request) {
		gotQuery = r.URL.Query()
		if err := r.ParseForm(); err != nil {
			t.Fatalf("解析表单失败: %v", err)
		}
		gotForm = r.PostForm
		w.Write([]byte(`{"code":0,"message":"OK","ttl":1}`))
	})
	_ = srv

	if err := c.Blacklist(context.Background(), "10086"); err != nil {
		t.Fatalf("Blacklist 失败: %v", err)
	}

	if got := gotForm.Get("fid"); got != "10086" {
		t.Errorf("fid = %q, 期望 10086", got)
	}
	if got := gotForm.Get("act"); got != "5" {
		t.Errorf("act = %q, 期望 5（拉黑）", got)
	}
	if got := gotForm.Get("re_src"); got != "11" {
		t.Errorf("re_src = %q, 期望 11", got)
	}
	if got := gotForm.Get("gaia_source"); got != "web_main" {
		t.Errorf("gaia_source = %q, 期望 web_main", got)
	}
	if got := gotForm.Get("spmid"); got != "333.1387.0.0" {
		t.Errorf("spmid = %q, 期望 333.1387.0.0", got)
	}
	if got := gotForm.Get("extend_content"); !strings.Contains(got, `"entity":"user"`) ||
		!strings.Contains(got, `"entity_id":10086`) {
		t.Errorf("extend_content = %q，应含 entity=user 与 entity_id=10086", got)
	}
	if got := gotForm.Get("csrf"); got != "tok123" {
		t.Errorf("csrf = %q", got)
	}
	// statistics 是 URL 查询参数，不是表单字段——HAR 里它挂在 URL 上
	if got := gotQuery.Get("statistics"); !strings.Contains(got, `"appId":100`) ||
		!strings.Contains(got, `"platform":5`) {
		t.Errorf("statistics 查询参数 = %q", got)
	}
}

// TestClientUnblacklistSendsAct6 与拉黑共用同一实现，只有 act 不同。
func TestClientUnblacklistSendsAct6(t *testing.T) {
	var gotForm url.Values
	c, _ := newTestClientForRelation(t, func(w http.ResponseWriter, r *http.Request) {
		r.ParseForm()
		gotForm = r.PostForm
		w.Write([]byte(`{"code":0,"message":"OK","ttl":1}`))
	})

	if err := c.Unblacklist(context.Background(), "10086"); err != nil {
		t.Fatalf("Unblacklist 失败: %v", err)
	}
	if got := gotForm.Get("act"); got != "6" {
		t.Errorf("act = %q, 期望 6（取消拉黑）", got)
	}
	if got := gotForm.Get("fid"); got != "10086" {
		t.Errorf("fid = %q", got)
	}
}

// TestClientBlacklistRejectsEmptyUID 防止拿空 UID 打一次没有意义的请求。
func TestClientBlacklistRejectsEmptyUID(t *testing.T) {
	c, _ := newTestClientForRelation(t, func(w http.ResponseWriter, r *http.Request) {
		t.Error("不该发出请求")
	})
	if err := c.Blacklist(context.Background(), ""); err == nil {
		t.Error("空 UID 应当报错")
	}
}

// TestClientBlacklistSurfacesAPIError B 站业务错误要原样透出，
// 调用方（httpapi 的手动操作日志）需要这个错误去记「失败」。
func TestClientBlacklistSurfacesAPIError(t *testing.T) {
	c, _ := newTestClientForRelation(t, func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"code":22106,"message":"你已经拉黑过对方了"}`))
	})
	err := c.Blacklist(context.Background(), "10086")
	if err == nil {
		t.Fatal("应当返回错误")
	}
	if !strings.Contains(err.Error(), "拉黑过") {
		t.Errorf("错误应含服务端消息，实际 %v", err)
	}
}

// TestClientRelationAttributeParsesBlacklisted 钉住"白捡"的状态回读接口。
//
// 128 是用户在自己号上连续三次请求实测出来的"已拉黑"取值（拉黑前 2、
// 拉黑后 128、取消拉黑后 0）。这条测试如果 IsBlacklisted 的判据被
// 改错（比如改成 == 2 或者 != 0）会失败，是「自检变异 (c)」要钉住的
// 防线。
func TestClientRelationAttributeParsesBlacklisted(t *testing.T) {
	var gotQuery url.Values
	c, _ := newTestClientForRelation(t, func(w http.ResponseWriter, r *http.Request) {
		gotQuery = r.URL.Query()
		w.Write([]byte(`{"code":0,"message":"0","data":{"relation":{"attribute":128}}}`))
	})

	attr, err := c.RelationAttribute(context.Background(), "10086")
	if err != nil {
		t.Fatalf("RelationAttribute 失败: %v", err)
	}
	if attr != 128 {
		t.Errorf("attribute = %d, 期望 128", attr)
	}
	if !IsBlacklisted(attr) {
		t.Error("attribute=128 应判定为已拉黑")
	}
	if gotQuery.Get("mid") != "10086" {
		t.Errorf("mid = %q", gotQuery.Get("mid"))
	}
	// GET 接口需要 wbi 签名
	if gotQuery.Get("w_rid") == "" || gotQuery.Get("wts") == "" {
		t.Error("acc/relation 应带 wbi 签名（w_rid/wts）")
	}
}

// TestClientRelationAttributeNonBlacklistedValues 覆盖「关注中」「无关系」
// 两个已实测的取值，确认它们都不会被误判成已拉黑。
func TestClientRelationAttributeNonBlacklistedValues(t *testing.T) {
	for _, tc := range []struct {
		name string
		attr int
	}{
		{"关注中", 2},
		{"无关系", 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c, _ := newTestClientForRelation(t, func(w http.ResponseWriter, r *http.Request) {
				w.Write([]byte(`{"code":0,"data":{"relation":{"attribute":` +
					itoaForTest(tc.attr) + `}}}`))
			})
			attr, err := c.RelationAttribute(context.Background(), "10086")
			if err != nil {
				t.Fatalf("RelationAttribute 失败: %v", err)
			}
			if IsBlacklisted(attr) {
				t.Errorf("attribute=%d 不应判定为已拉黑", attr)
			}
		})
	}
}

func itoaForTest(n int) string {
	if n == 0 {
		return "0"
	}
	s := ""
	for n > 0 {
		s = string(rune('0'+n%10)) + s
		n /= 10
	}
	return s
}

// TestClientNickname 用于「加入禁言名单/拉黑确认」时自动回填昵称。
//
// 不是 HAR 验证过的接口——那次抓包只覆盖了拉黑流程；这是只读查询，
// 失败只是昵称留空，不影响拉黑本身的正确性（见 relation.go 的注释）。
func TestClientNickname(t *testing.T) {
	var gotQuery url.Values
	c, _ := newTestClientForRelation(t, func(w http.ResponseWriter, r *http.Request) {
		gotQuery = r.URL.Query()
		w.Write([]byte(`{"code":0,"data":{"name":"测试昵称"}}`))
	})

	name, err := c.Nickname(context.Background(), "10086")
	if err != nil {
		t.Fatalf("Nickname 失败: %v", err)
	}
	if name != "测试昵称" {
		t.Errorf("name = %q", name)
	}
	if gotQuery.Get("mid") != "10086" {
		t.Errorf("mid = %q", gotQuery.Get("mid"))
	}
	if gotQuery.Get("w_rid") == "" {
		t.Error("acc/info 应带 wbi 签名")
	}
}
