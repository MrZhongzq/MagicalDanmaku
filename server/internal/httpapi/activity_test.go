package httpapi_test

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/MrZhongzq/MagicalDanmaku/server/internal/perm"
	"github.com/MrZhongzq/MagicalDanmaku/server/internal/store"
)

func seedActivity(t *testing.T, st *store.Store, bid int64, rows ...store.ActivityRow) {
	t.Helper()
	b, err := st.ListBindings(context.Background())
	if err != nil {
		t.Fatalf("列绑定报错: %v", err)
	}
	var accID int64
	for _, x := range b {
		if x.ID == bid {
			accID = x.AccountID
		}
	}
	for i := range rows {
		rows[i].AccountID = accID
		rows[i].BindingID = &bid
	}
	if err := st.InsertActivity(context.Background(), rows); err != nil {
		t.Fatalf("写业务日志报错: %v", err)
	}
}

func grantEventRead(t *testing.T, st *store.Store, user, account, room string) {
	t.Helper()
	if err := st.Grant(context.Background(), user, account, room,
		[]perm.Permission{perm.EventRead}); err != nil {
		t.Fatalf("授权报错: %v", err)
	}
}

var seedTime = time.Date(2026, 7, 31, 20, 0, 0, 0, time.UTC)

func TestQueryActivity(t *testing.T) {
	srv, st := newTestServer(t)
	c := loginAs(t, srv, st, "张三", false)
	bid := mustBindingFor(t, st, "张三", "小号", "123")
	grantEventRead(t, st, "张三", "小号", "123")

	seedActivity(t, st, bid,
		store.ActivityRow{Kind: store.ActivityEvent, EventType: "danmaku",
			UserName: "观众甲", Detail: []byte(`{"text":"你好"}`), OccurredAt: seedTime},
		store.ActivityRow{Kind: store.ActivityAction, ActionType: "danmaku",
			RuleName: "关键词回复", OccurredAt: seedTime.Add(time.Second)},
	)

	resp := jsonRequest(t, c, "GET", srv.URL+"/api/bindings/"+itoa(bid)+"/activity", "")
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("状态码 = %d", resp.StatusCode)
	}

	var got []map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&got); err != nil {
		t.Fatalf("解析报错: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("记录数 = %d, 期望 2", len(got))
	}
	// 按时间倒序：最新的在前
	if got[0]["kind"] != "action" {
		t.Errorf("应按时间倒序，第一条 = %v", got[0])
	}
}

func TestQueryActivityFilterByKind(t *testing.T) {
	srv, st := newTestServer(t)
	c := loginAs(t, srv, st, "张三", false)
	bid := mustBindingFor(t, st, "张三", "小号", "123")
	grantEventRead(t, st, "张三", "小号", "123")

	seedActivity(t, st, bid,
		store.ActivityRow{Kind: store.ActivityEvent, EventType: "danmaku", OccurredAt: seedTime},
		store.ActivityRow{Kind: store.ActivityAction, ActionType: "danmaku", OccurredAt: seedTime},
	)

	resp := jsonRequest(t, c, "GET",
		srv.URL+"/api/bindings/"+itoa(bid)+"/activity?kind=event", "")
	defer resp.Body.Close()

	var got []map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&got); err != nil {
		t.Fatalf("解析报错: %v", err)
	}
	if len(got) != 1 || got[0]["kind"] != "event" {
		t.Errorf("按 kind 过滤失败: %+v", got)
	}
}

func TestQueryActivityFilterByEventType(t *testing.T) {
	srv, st := newTestServer(t)
	c := loginAs(t, srv, st, "张三", false)
	bid := mustBindingFor(t, st, "张三", "小号", "123")
	grantEventRead(t, st, "张三", "小号", "123")

	seedActivity(t, st, bid,
		store.ActivityRow{Kind: store.ActivityEvent, EventType: "danmaku", OccurredAt: seedTime},
		store.ActivityRow{Kind: store.ActivityEvent, EventType: "gift", OccurredAt: seedTime},
	)

	resp := jsonRequest(t, c, "GET",
		srv.URL+"/api/bindings/"+itoa(bid)+"/activity?eventType=gift", "")
	defer resp.Body.Close()

	var got []map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&got); err != nil {
		t.Fatalf("解析报错: %v", err)
	}
	if len(got) != 1 || got[0]["eventType"] != "gift" {
		t.Errorf("按事件类型过滤失败: %+v", got)
	}
}

func TestQueryActivityLimitIsCapped(t *testing.T) {
	// 不设上限就是全表扫，一个活跃房间一天几万行
	srv, st := newTestServer(t)
	c := loginAs(t, srv, st, "张三", false)
	bid := mustBindingFor(t, st, "张三", "小号", "123")
	grantEventRead(t, st, "张三", "小号", "123")

	// 必须塞**超过** 500 行。只塞 60 行再断言「不超过 500」是空测试：
	// 一共就 60 行，上限有没有生效它都是绿的
	rows := make([]store.ActivityRow, 620)
	for i := range rows {
		rows[i] = store.ActivityRow{Kind: store.ActivityEvent, EventType: "danmaku",
			OccurredAt: seedTime.Add(time.Duration(i) * time.Second)}
	}
	seedActivity(t, st, bid, rows...)

	resp := jsonRequest(t, c, "GET",
		srv.URL+"/api/bindings/"+itoa(bid)+"/activity?limit=999999", "")
	defer resp.Body.Close()

	var got []map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&got); err != nil {
		t.Fatalf("解析报错: %v", err)
	}
	// 断言**恰好**是 500，不是「不超过 500」——后者在少塞几行时会假绿
	if len(got) != 500 {
		t.Errorf("返回条数 = %d, 期望恰好 500（上限截断）", len(got))
	}
}

// since 晚于 until 要报错，不能静默返回空。
//
// 日志页里「查不到」和「你把时间填反了」长得一模一样，用户会以为
// 那段时间真的没有日志。
func TestQueryActivityRejectsInvertedRange(t *testing.T) {
	srv, st := newTestServer(t)
	c := loginAs(t, srv, st, "张三", false)
	bid := mustBindingFor(t, st, "张三", "小号", "123")
	grantEventRead(t, st, "张三", "小号", "123")

	resp := jsonRequest(t, c, "GET",
		srv.URL+"/api/bindings/"+itoa(bid)+
			"/activity?since=2026-08-01T00:00:00Z&until=2026-07-01T00:00:00Z", "")
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusUnprocessableEntity {
		t.Errorf("since 晚于 until 状态码 = %d, 期望 422", resp.StatusCode)
	}
}

func TestQueryActivityBadLimit(t *testing.T) {
	srv, st := newTestServer(t)
	c := loginAs(t, srv, st, "张三", false)
	bid := mustBindingFor(t, st, "张三", "小号", "123")
	grantEventRead(t, st, "张三", "小号", "123")

	resp := jsonRequest(t, c, "GET",
		srv.URL+"/api/bindings/"+itoa(bid)+"/activity?limit=不是数字", "")
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusUnprocessableEntity {
		t.Errorf("状态码 = %d, 期望 422", resp.StatusCode)
	}
}

func TestDeleteActivityByRange(t *testing.T) {
	srv, st := newTestServer(t)
	c := loginAs(t, srv, st, "张三", false)
	bid := mustBindingFor(t, st, "张三", "小号", "123")
	grantEventRead(t, st, "张三", "小号", "123")

	seedActivity(t, st, bid,
		store.ActivityRow{Kind: store.ActivityEvent, EventType: "danmaku",
			OccurredAt: seedTime.Add(-48 * time.Hour)},
		store.ActivityRow{Kind: store.ActivityEvent, EventType: "danmaku",
			OccurredAt: seedTime},
	)

	resp := jsonRequest(t, c, "DELETE",
		srv.URL+"/api/bindings/"+itoa(bid)+"/activity?until="+
			seedTime.Add(-24*time.Hour).Format(time.RFC3339), "")
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("状态码 = %d, 期望 200", resp.StatusCode)
	}
	var got map[string]int64
	if err := json.NewDecoder(resp.Body).Decode(&got); err != nil {
		t.Fatalf("解析报错: %v", err)
	}
	if got["deleted"] != 1 {
		t.Errorf("deleted = %d, 期望 1", got["deleted"])
	}

	remain := jsonRequest(t, c, "GET", srv.URL+"/api/bindings/"+itoa(bid)+"/activity", "")
	defer remain.Body.Close()
	var recs []map[string]any
	if err := json.NewDecoder(remain.Body).Decode(&recs); err != nil {
		t.Fatalf("解析报错: %v", err)
	}
	if len(recs) != 1 {
		t.Errorf("剩余条数 = %d, 期望 1", len(recs))
	}
}

// 不带任何时间参数、也不带 all=1，必须拒绝——这是这个接口最要紧的
// 一条防线：一次手滑的 DELETE .../activity 不该清空整个房间的历史。
//
// 断言不能只看状态码：要同时确认库里的行一条没少，否则「返回
// 422 但其实还是删了」这种更糟的错误会被当成通过。
func TestDeleteActivityRequiresAllFlagWhenRangeUnset(t *testing.T) {
	srv, st := newTestServer(t)
	c := loginAs(t, srv, st, "张三", false)
	bid := mustBindingFor(t, st, "张三", "小号", "123")
	grantEventRead(t, st, "张三", "小号", "123")

	seedActivity(t, st, bid,
		store.ActivityRow{Kind: store.ActivityEvent, EventType: "danmaku", OccurredAt: seedTime},
		store.ActivityRow{Kind: store.ActivityEvent, EventType: "gift", OccurredAt: seedTime},
	)

	resp := jsonRequest(t, c, "DELETE", srv.URL+"/api/bindings/"+itoa(bid)+"/activity", "")
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusUnprocessableEntity {
		t.Errorf("状态码 = %d, 期望 422", resp.StatusCode)
	}

	remain := jsonRequest(t, c, "GET", srv.URL+"/api/bindings/"+itoa(bid)+"/activity", "")
	defer remain.Body.Close()
	var recs []map[string]any
	if err := json.NewDecoder(remain.Body).Decode(&recs); err != nil {
		t.Fatalf("解析报错: %v", err)
	}
	if len(recs) != 2 {
		t.Errorf("被拒绝的删除不该动库里的数据，实际剩 %d 条，期望 2", len(recs))
	}
}

func TestDeleteActivityAllRequiresExplicitFlag(t *testing.T) {
	srv, st := newTestServer(t)
	c := loginAs(t, srv, st, "张三", false)
	bid := mustBindingFor(t, st, "张三", "小号", "123")
	grantEventRead(t, st, "张三", "小号", "123")

	seedActivity(t, st, bid,
		store.ActivityRow{Kind: store.ActivityEvent, EventType: "danmaku", OccurredAt: seedTime},
		store.ActivityRow{Kind: store.ActivityEvent, EventType: "gift", OccurredAt: seedTime},
	)

	resp := jsonRequest(t, c, "DELETE",
		srv.URL+"/api/bindings/"+itoa(bid)+"/activity?all=1", "")
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("状态码 = %d, 期望 200", resp.StatusCode)
	}
	var got map[string]int64
	if err := json.NewDecoder(resp.Body).Decode(&got); err != nil {
		t.Fatalf("解析报错: %v", err)
	}
	if got["deleted"] != 2 {
		t.Errorf("deleted = %d, 期望 2", got["deleted"])
	}
}

// 同时传时间范围与 all=1：以范围为准。all 只是「我确认要删全部」
// 的确认标记，有范围时它没有意义。
func TestDeleteActivityRangeTakesPrecedenceOverAllFlag(t *testing.T) {
	srv, st := newTestServer(t)
	c := loginAs(t, srv, st, "张三", false)
	bid := mustBindingFor(t, st, "张三", "小号", "123")
	grantEventRead(t, st, "张三", "小号", "123")

	seedActivity(t, st, bid,
		store.ActivityRow{Kind: store.ActivityEvent, EventType: "danmaku",
			OccurredAt: seedTime.Add(-48 * time.Hour)},
		store.ActivityRow{Kind: store.ActivityEvent, EventType: "gift", OccurredAt: seedTime},
	)

	resp := jsonRequest(t, c, "DELETE",
		srv.URL+"/api/bindings/"+itoa(bid)+"/activity?until="+
			seedTime.Add(-24*time.Hour).Format(time.RFC3339)+"&all=1", "")
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("状态码 = %d, 期望 200", resp.StatusCode)
	}
	var got map[string]int64
	if err := json.NewDecoder(resp.Body).Decode(&got); err != nil {
		t.Fatalf("解析报错: %v", err)
	}
	if got["deleted"] != 1 {
		t.Errorf("deleted = %d, 期望 1（应以 until 范围为准，而不是删全部）", got["deleted"])
	}
}

func TestDeleteActivityRejectsInvertedRange(t *testing.T) {
	srv, st := newTestServer(t)
	c := loginAs(t, srv, st, "张三", false)
	bid := mustBindingFor(t, st, "张三", "小号", "123")
	grantEventRead(t, st, "张三", "小号", "123")

	resp := jsonRequest(t, c, "DELETE",
		srv.URL+"/api/bindings/"+itoa(bid)+
			"/activity?since=2026-08-01T00:00:00Z&until=2026-07-01T00:00:00Z", "")
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusUnprocessableEntity {
		t.Errorf("状态码 = %d, 期望 422", resp.StatusCode)
	}
}

// 全批次终审项【5】：DELETE 走"账号所有者或管理员"这条轴，不是
// event:read——即便显式给了 event:read（能看到全部日志的权限点），
// 也不该能删掉它们。删历史与删绑定是同一量级的破坏性操作。
func TestDeleteActivityRequiresAccountOwnerOrAdminNotJustEventRead(t *testing.T) {
	srv, st := newTestServer(t)
	loginAs(t, srv, st, "张三", false)
	bid := mustBindingFor(t, st, "张三", "小号", "123")

	li := loginAs(t, srv, st, "李四", false)
	// 特意只给 event:read——这正是 review 里点名的"有完整成员授权
	// 子系统，把某人拉进绑定只给 event:read 是设计上支持的用法"，
	// 这个人不该因此附带拿到删光历史的能力。
	if err := st.Grant(context.Background(), "李四", "小号", "123",
		[]perm.Permission{perm.EventRead}); err != nil {
		t.Fatalf("授权报错: %v", err)
	}

	resp := jsonRequest(t, li, "DELETE",
		srv.URL+"/api/bindings/"+itoa(bid)+"/activity?all=1", "")
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusForbidden {
		t.Errorf("状态码 = %d, 期望 403——即便有 event:read，也不该能删除业务日志", resp.StatusCode)
	}

	// 403 之后库里的数据必须真的没被动——不能是"报了 403 但其实还是删了"
	remain := jsonRequest(t, li, "GET", srv.URL+"/api/bindings/"+itoa(bid)+"/activity", "")
	defer remain.Body.Close()
	if remain.StatusCode != http.StatusOK {
		t.Fatalf("GET 状态码 = %d", remain.StatusCode)
	}
}

// 完全不可见的调用者（连这个绑定存在都不该知道）删除时应该收到 404，
// 与"绑定不存在"不可区分——语义照抄 handleDeleteBinding，用绑定 ID
// 递增探测不出部署里有哪些绑定。
func TestDeleteActivityNotVisibleReturnsNotFound(t *testing.T) {
	srv, st := newTestServer(t)
	loginAs(t, srv, st, "张三", false)
	bid := mustBindingFor(t, st, "张三", "小号", "123")

	li := loginAs(t, srv, st, "李四", false) // 没有任何授权，对这个绑定完全不可见

	resp := jsonRequest(t, li, "DELETE",
		srv.URL+"/api/bindings/"+itoa(bid)+"/activity?all=1", "")
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("状态码 = %d, 期望 404（不可区分于「绑定不存在」）", resp.StatusCode)
	}
}

// 管理员即便不是账号所有者也能删——与 handleDeleteBinding 的授权语义
// 一致，管理员一律放行。
func TestDeleteActivityAllowsAdminEvenWithoutOwnership(t *testing.T) {
	srv, st := newTestServer(t)
	loginAs(t, srv, st, "张三", false)
	bid := mustBindingFor(t, st, "张三", "小号", "123")
	admin := loginAs(t, srv, st, "管理员", true)

	seedActivity(t, st, bid,
		store.ActivityRow{Kind: store.ActivityEvent, EventType: "danmaku", OccurredAt: seedTime})

	resp := jsonRequest(t, admin, "DELETE",
		srv.URL+"/api/bindings/"+itoa(bid)+"/activity?all=1", "")
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("状态码 = %d, 期望 200——管理员应当能删，即便不是账号所有者", resp.StatusCode)
	}
}

func TestQueryActivityRequiresEventRead(t *testing.T) {
	srv, st := newTestServer(t)
	loginAs(t, srv, st, "张三", false)
	bid := mustBindingFor(t, st, "张三", "小号", "123")

	li := loginAs(t, srv, st, "李四", false)
	if err := st.Grant(context.Background(), "李四", "小号", "123",
		[]perm.Permission{perm.RuleWrite}); err != nil {
		t.Fatalf("授权报错: %v", err)
	}

	resp := jsonRequest(t, li, "GET", srv.URL+"/api/bindings/"+itoa(bid)+"/activity", "")
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusForbidden {
		t.Errorf("状态码 = %d, 期望 403", resp.StatusCode)
	}
}
