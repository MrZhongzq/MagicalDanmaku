package store

import (
	"context"
	"encoding/json"
	"testing"
	"time"
)

// fixedTime 是测试用的固定时刻。用固定值而非 time.Now，
// 时间相关的断言才不会在跨秒时随机失败。
var fixedTime = time.Date(2026, 7, 31, 20, 30, 0, 0, time.UTC)

func TestInsertAndQueryActivity(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	accID := mustAccount(t, s, "小号")
	b, err := s.UpsertBinding(ctx, accID, "1706666491")
	if err != nil {
		t.Fatalf("创建绑定报错: %v", err)
	}

	rows := []ActivityRow{{
		AccountID: accID, BindingID: &b.ID, RoomID: "1706666491",
		Kind: ActivityEvent, EventType: "danmaku",
		UserUID: "10086", UserName: "张三",
		Detail:     []byte(`{"text":"求歌单"}`),
		OccurredAt: fixedTime,
	}}
	if err := s.InsertActivity(ctx, rows); err != nil {
		t.Fatalf("写入业务日志报错: %v", err)
	}

	got, err := s.QueryActivity(ctx, ActivityQuery{AccountID: accID})
	if err != nil {
		t.Fatalf("查询业务日志报错: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("记录数 = %d, 期望 1", len(got))
	}
	r := got[0]
	if r.EventType != "danmaku" || r.UserName != "张三" || r.UserUID != "10086" {
		t.Errorf("记录 = %+v", r)
	}
	if !r.OccurredAt.Equal(fixedTime) {
		t.Errorf("时间 = %v, 期望 %v", r.OccurredAt, fixedTime)
	}
}

// detail 是 JSONB 列，写进去要能原样读出来
func TestActivityDetailRoundTrips(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	accID := mustAccount(t, s, "小号")

	detail := map[string]any{"giftName": "小花花", "count": 10, "totalCoin": 1000}
	raw, err := json.Marshal(detail)
	if err != nil {
		t.Fatalf("序列化报错: %v", err)
	}

	if err := s.InsertActivity(ctx, []ActivityRow{{
		AccountID: accID, Kind: ActivityEvent, EventType: "gift",
		Detail: raw, OccurredAt: fixedTime,
	}}); err != nil {
		t.Fatalf("写入报错: %v", err)
	}

	got, err := s.QueryActivity(ctx, ActivityQuery{AccountID: accID})
	if err != nil {
		t.Fatalf("查询报错: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("记录数 = %d", len(got))
	}

	var back map[string]any
	if err := json.Unmarshal(got[0].Detail, &back); err != nil {
		t.Fatalf("解析 detail 报错: %v (原始 %s)", err, got[0].Detail)
	}
	if back["giftName"] != "小花花" {
		t.Errorf("detail = %v", back)
	}
}

func TestInsertActivityAcceptsNilDetail(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	accID := mustAccount(t, s, "小号")

	if err := s.InsertActivity(ctx, []ActivityRow{{
		AccountID: accID, Kind: ActivityEvent, EventType: "user_enter",
		OccurredAt: fixedTime,
	}}); err != nil {
		t.Fatalf("detail 为 nil 时应能写入: %v", err)
	}
	got, err := s.QueryActivity(ctx, ActivityQuery{AccountID: accID})
	if err != nil {
		t.Fatalf("查询报错: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("记录数 = %d", len(got))
	}
}

func TestInsertActivityEmptySliceIsNoop(t *testing.T) {
	s := testStore(t)
	if err := s.InsertActivity(context.Background(), nil); err != nil {
		t.Errorf("空批次不该报错: %v", err)
	}
}

func TestInsertActivityBatch(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	accID := mustAccount(t, s, "小号")

	rows := make([]ActivityRow, 500)
	for i := range rows {
		rows[i] = ActivityRow{
			AccountID: accID, Kind: ActivityEvent, EventType: "danmaku",
			OccurredAt: fixedTime.Add(time.Duration(i) * time.Second),
		}
	}
	if err := s.InsertActivity(ctx, rows); err != nil {
		t.Fatalf("批量写入报错: %v", err)
	}

	got, err := s.QueryActivity(ctx, ActivityQuery{AccountID: accID, Limit: 1000})
	if err != nil {
		t.Fatalf("查询报错: %v", err)
	}
	if len(got) != 500 {
		t.Errorf("记录数 = %d, 期望 500", len(got))
	}
}

// 事件与动作在同一条时间线上，才能直接看到因果
func TestActivityEventAndActionShareTimeline(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	accID := mustAccount(t, s, "小号")
	b, err := s.UpsertBinding(ctx, accID, "123")
	if err != nil {
		t.Fatalf("创建绑定报错: %v", err)
	}

	if err := s.InsertActivity(ctx, []ActivityRow{
		{
			AccountID: accID, BindingID: &b.ID, Kind: ActivityEvent,
			EventType: "danmaku", UserName: "张三", OccurredAt: fixedTime,
		},
		{
			AccountID: accID, BindingID: &b.ID, Kind: ActivityAction,
			ActionType: "danmaku", RuleName: "关键词回复",
			OccurredAt: fixedTime.Add(3 * time.Second),
		},
	}); err != nil {
		t.Fatalf("写入报错: %v", err)
	}

	got, err := s.QueryActivity(ctx, ActivityQuery{BindingID: b.ID})
	if err != nil {
		t.Fatalf("查询报错: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("记录数 = %d, 期望 2", len(got))
	}
	// 按时间倒序：最新的在前
	if got[0].Kind != ActivityAction || got[1].Kind != ActivityEvent {
		t.Errorf("应按时间倒序返回，实际 %s / %s", got[0].Kind, got[1].Kind)
	}
	if got[0].RuleName != "关键词回复" {
		t.Errorf("动作记录应带规则名，实际 %q", got[0].RuleName)
	}
}

func TestQueryActivityFiltersByKind(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	accID := mustAccount(t, s, "小号")

	if err := s.InsertActivity(ctx, []ActivityRow{
		{AccountID: accID, Kind: ActivityEvent, EventType: "danmaku", OccurredAt: fixedTime},
		{AccountID: accID, Kind: ActivityAction, ActionType: "danmaku", OccurredAt: fixedTime},
	}); err != nil {
		t.Fatalf("写入报错: %v", err)
	}

	got, err := s.QueryActivity(ctx, ActivityQuery{AccountID: accID, Kind: ActivityAction})
	if err != nil {
		t.Fatalf("查询报错: %v", err)
	}
	if len(got) != 1 || got[0].Kind != ActivityAction {
		t.Errorf("按 kind 过滤失败: %+v", got)
	}
}

func TestQueryActivityFiltersByEventType(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	accID := mustAccount(t, s, "小号")

	if err := s.InsertActivity(ctx, []ActivityRow{
		{AccountID: accID, Kind: ActivityEvent, EventType: "danmaku", OccurredAt: fixedTime},
		{AccountID: accID, Kind: ActivityEvent, EventType: "gift", OccurredAt: fixedTime},
	}); err != nil {
		t.Fatalf("写入报错: %v", err)
	}

	got, err := s.QueryActivity(ctx, ActivityQuery{AccountID: accID, EventType: "gift"})
	if err != nil {
		t.Fatalf("查询报错: %v", err)
	}
	if len(got) != 1 || got[0].EventType != "gift" {
		t.Errorf("按事件类型过滤失败: %+v", got)
	}
}

func TestQueryActivityFiltersByTimeRange(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	accID := mustAccount(t, s, "小号")

	if err := s.InsertActivity(ctx, []ActivityRow{
		{AccountID: accID, Kind: ActivityEvent, EventType: "danmaku",
			OccurredAt: fixedTime.Add(-2 * time.Hour)},
		{AccountID: accID, Kind: ActivityEvent, EventType: "danmaku",
			OccurredAt: fixedTime},
	}); err != nil {
		t.Fatalf("写入报错: %v", err)
	}

	got, err := s.QueryActivity(ctx, ActivityQuery{
		AccountID: accID, Since: fixedTime.Add(-time.Hour),
	})
	if err != nil {
		t.Fatalf("查询报错: %v", err)
	}
	if len(got) != 1 {
		t.Errorf("按时间过滤后记录数 = %d, 期望 1", len(got))
	}
}

func TestQueryActivityDefaultLimit(t *testing.T) {
	// 不设 limit 就全表扫，一个活跃房间一天几万行
	s := testStore(t)
	ctx := context.Background()
	accID := mustAccount(t, s, "小号")

	rows := make([]ActivityRow, 150)
	for i := range rows {
		rows[i] = ActivityRow{
			AccountID: accID, Kind: ActivityEvent, EventType: "danmaku",
			OccurredAt: fixedTime.Add(time.Duration(i) * time.Second),
		}
	}
	if err := s.InsertActivity(ctx, rows); err != nil {
		t.Fatalf("写入报错: %v", err)
	}

	got, err := s.QueryActivity(ctx, ActivityQuery{AccountID: accID})
	if err != nil {
		t.Fatalf("查询报错: %v", err)
	}
	if len(got) != 100 {
		t.Errorf("默认上限应为 100，实际返回 %d 条", len(got))
	}
}

// 「每个账号一份业务日志」在库里是 account_id 列，查询时必须真的隔离
func TestQueryActivityIsolatedPerAccount(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	main := mustAccount(t, s, "主播号")
	sub := mustAccount(t, s, "小号")

	if err := s.InsertActivity(ctx, []ActivityRow{
		{AccountID: main, Kind: ActivityEvent, EventType: "danmaku", OccurredAt: fixedTime},
		{AccountID: sub, Kind: ActivityEvent, EventType: "gift", OccurredAt: fixedTime},
	}); err != nil {
		t.Fatalf("写入报错: %v", err)
	}

	got, err := s.QueryActivity(ctx, ActivityQuery{AccountID: main})
	if err != nil {
		t.Fatalf("查询报错: %v", err)
	}
	if len(got) != 1 || got[0].EventType != "danmaku" {
		t.Errorf("账号间应隔离: %+v", got)
	}
}

func TestPurgeActivityBefore(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	accID := mustAccount(t, s, "小号")

	if err := s.InsertActivity(ctx, []ActivityRow{
		{AccountID: accID, Kind: ActivityEvent, EventType: "danmaku",
			OccurredAt: fixedTime.Add(-48 * time.Hour)},
		{AccountID: accID, Kind: ActivityEvent, EventType: "danmaku",
			OccurredAt: fixedTime},
	}); err != nil {
		t.Fatalf("写入报错: %v", err)
	}

	n, err := s.PurgeActivityBefore(ctx, fixedTime.Add(-24*time.Hour))
	if err != nil {
		t.Fatalf("清理报错: %v", err)
	}
	if n != 1 {
		t.Errorf("清理条数 = %d, 期望 1", n)
	}

	got, err := s.QueryActivity(ctx, ActivityQuery{AccountID: accID})
	if err != nil {
		t.Fatalf("查询报错: %v", err)
	}
	if len(got) != 1 {
		t.Errorf("清理后应剩 1 条，实际 %d 条", len(got))
	}
}

// 删账号时业务日志跟着走，删绑定时日志保留但 binding_id 置空
func TestActivityCascadeOnAccountDeleteAndBindingDelete(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	accID := mustAccount(t, s, "小号")
	b, err := s.UpsertBinding(ctx, accID, "123")
	if err != nil {
		t.Fatalf("创建绑定报错: %v", err)
	}

	if err := s.InsertActivity(ctx, []ActivityRow{{
		AccountID: accID, BindingID: &b.ID, Kind: ActivityEvent,
		EventType: "danmaku", OccurredAt: fixedTime,
	}}); err != nil {
		t.Fatalf("写入报错: %v", err)
	}

	if err := s.DeleteBinding(ctx, "小号", "123"); err != nil {
		t.Fatalf("删除绑定报错: %v", err)
	}
	got, err := s.QueryActivity(ctx, ActivityQuery{AccountID: accID})
	if err != nil {
		t.Fatalf("查询报错: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("删绑定后日志应保留（统计要用），实际 %d 条", len(got))
	}
	if got[0].BindingID != nil {
		t.Errorf("binding_id 应置空，实际 %v", *got[0].BindingID)
	}

	if err := s.DeleteAccount(ctx, "小号"); err != nil {
		t.Fatalf("删除账号报错: %v", err)
	}
	var n int
	if err := s.pool.QueryRow(ctx, `SELECT count(*) FROM activity_logs`).Scan(&n); err != nil {
		t.Fatalf("统计报错: %v", err)
	}
	if n != 0 {
		t.Errorf("删账号应连带清空其日志，实际剩 %d 条", n)
	}
}
