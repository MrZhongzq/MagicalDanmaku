# P3 多租户数据层 实施计划 · Part 3（Task 11–14）

> 接 `2026-07-31-p3-data-layer-part2.md`。Global Constraints 见 `2026-07-31-p3-data-layer.md`。
>
> 本部分实现两类日志。**系统日志走文件，业务日志进库**：数据库连不上时「数据库连不上」这条日志本身还得写得出来，而弹幕与礼物记录要能被 P4 展示、被 P5 统计。

**贯穿本部分的一条硬约束：** 业务日志的写入绝不能阻塞弹幕处理。活跃房间每秒几十条事件，同步 INSERT 会把数据库延迟压到规则引擎的关键路径上。**缓冲满时丢弃日志并计数**——丢日志可以接受，漏欢迎不行。

---

## Task 11: 业务日志的存储侧

**Files:**
- Create: `server/internal/store/activity.go`
- Create: `server/internal/store/activity_test.go`

**Interfaces:**
- Consumes: `store.Store`、`store.Account`、`store.Binding`（Part 1、2）
- Produces:
  - `type store.ActivityKind string`，常量 `store.ActivityEvent = "event"`、`store.ActivityAction = "action"`
  - `type store.ActivityRow struct { AccountID int64; BindingID *int64; RoomID string; Kind ActivityKind; EventType, ActionType, RuleName, UserUID, UserName string; Detail []byte; OccurredAt time.Time }`
  - `type store.ActivityRecord struct { ID int64; ActivityRow }`
  - `type store.ActivityQuery struct { AccountID, BindingID int64; Kind ActivityKind; EventType string; Since, Until time.Time; Limit int }`
  - `func (s *Store) InsertActivity(ctx context.Context, rows []ActivityRow) error`
  - `func (s *Store) QueryActivity(ctx context.Context, q ActivityQuery) ([]ActivityRecord, error)`
  - `func (s *Store) PurgeActivityBefore(ctx context.Context, t time.Time) (int64, error)`

- [ ] **Step 1: 写失败的测试**

创建 `server/internal/store/activity_test.go`：

```go
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
```

- [ ] **Step 2: 跑测试确认失败**

```bash
export MAGICD_TEST_DATABASE_URL='postgres://magicd:magicd@localhost:5433/magicd?sslmode=disable'
cd server; go test ./internal/store/ -run TestInsertAndQueryActivity 2>&1 | tail -5; echo "退出码=$?"
```

- [ ] **Step 3: 实现**

创建 `server/internal/store/activity.go`：

```go
package store

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

// defaultActivityLimit 是查询未指定条数时的上限。
//
// 不设上限就是全表扫：一个活跃房间一天几万行。
const defaultActivityLimit = 100

// ActivityKind 区分记录的是收到的事件还是机器人执行的动作。
type ActivityKind string

// 两种业务日志。
const (
	ActivityEvent  ActivityKind = "event"  // 收到的事件
	ActivityAction ActivityKind = "action" // 规则触发的动作
)

// ActivityRow 是一条待写入的业务日志。
//
// 事件与动作合并进同一张表，是为了在一条时间线上直接看到因果：
//
//	10:31:02  event   danmaku   张三: 求歌单
//	10:31:02  action  danmaku   规则「关键词回复」→ 歌单在主播的动态里哦~
//
// 分成两张表就得靠时间戳自己拼。
type ActivityRow struct {
	AccountID  int64
	BindingID  *int64 // 可空：绑定被删除后置空，日志本身保留
	RoomID     string
	Kind       ActivityKind
	EventType  string // Kind 为 event 时填
	ActionType string // Kind 为 action 时填
	RuleName   string // Kind 为 action 时填，哪条规则触发的
	UserUID    string
	UserName   string
	Detail     []byte // JSON，可为 nil
	OccurredAt time.Time
}

// ActivityRecord 是读出来的业务日志，比 ActivityRow 多一个主键。
type ActivityRecord struct {
	ID int64
	ActivityRow
}

// ActivityQuery 是业务日志的查询条件。零值字段表示不限制。
type ActivityQuery struct {
	AccountID int64
	BindingID int64
	Kind      ActivityKind
	EventType string
	Since     time.Time
	Until     time.Time
	Limit     int // 0 表示用默认上限
}

// activityColumns 是插入用的列顺序，CopyFrom 依赖它与取值顺序一致。
var activityColumns = []string{
	"account_id", "binding_id", "room_id", "kind",
	"event_type", "action_type", "rule_name",
	"user_uid", "user_name", "detail", "occurred_at",
}

// InsertActivity 批量写入业务日志。
//
// 用 COPY 而非逐条 INSERT：写入器每 200 毫秒或攒够 500 条就调一次，
// 逐条发会让往返次数变成瓶颈。
func (s *Store) InsertActivity(ctx context.Context, rows []ActivityRow) error {
	if len(rows) == 0 {
		return nil
	}

	values := make([][]any, len(rows))
	for i, r := range rows {
		values[i] = []any{
			r.AccountID, r.BindingID, r.RoomID, string(r.Kind),
			r.EventType, r.ActionType, r.RuleName,
			r.UserUID, r.UserName, r.Detail, r.OccurredAt,
		}
	}

	_, err := s.pool.CopyFrom(ctx,
		pgx.Identifier{"activity_logs"}, activityColumns, pgx.CopyFromRows(values))
	if err != nil {
		return fmt.Errorf("store: 写入 %d 条业务日志失败: %w", len(rows), err)
	}
	return nil
}

// QueryActivity 按条件查询业务日志，按时间倒序返回。
func (s *Store) QueryActivity(ctx context.Context, q ActivityQuery) ([]ActivityRecord, error) {
	var where []string
	var args []any

	add := func(cond string, val any) {
		args = append(args, val)
		where = append(where, fmt.Sprintf(cond, len(args)))
	}
	if q.AccountID != 0 {
		add("account_id = $%d", q.AccountID)
	}
	if q.BindingID != 0 {
		add("binding_id = $%d", q.BindingID)
	}
	if q.Kind != "" {
		add("kind = $%d", string(q.Kind))
	}
	if q.EventType != "" {
		add("event_type = $%d", q.EventType)
	}
	if !q.Since.IsZero() {
		add("occurred_at >= $%d", q.Since)
	}
	if !q.Until.IsZero() {
		add("occurred_at <= $%d", q.Until)
	}

	limit := q.Limit
	if limit <= 0 {
		limit = defaultActivityLimit
	}
	args = append(args, limit)

	sql := `SELECT id, account_id, binding_id, room_id, kind,
	               event_type, action_type, rule_name,
	               user_uid, user_name, detail, occurred_at
	        FROM activity_logs`
	if len(where) > 0 {
		sql += " WHERE " + strings.Join(where, " AND ")
	}
	sql += fmt.Sprintf(" ORDER BY occurred_at DESC, id DESC LIMIT $%d", len(args))

	rows, err := s.pool.Query(ctx, sql, args...)
	if err != nil {
		return nil, fmt.Errorf("store: 查询业务日志失败: %w", err)
	}
	defer rows.Close()

	var out []ActivityRecord
	for rows.Next() {
		var r ActivityRecord
		var kind string
		if err := rows.Scan(&r.ID, &r.AccountID, &r.BindingID, &r.RoomID, &kind,
			&r.EventType, &r.ActionType, &r.RuleName,
			&r.UserUID, &r.UserName, &r.Detail, &r.OccurredAt); err != nil {
			return nil, fmt.Errorf("store: 读取业务日志失败: %w", err)
		}
		r.Kind = ActivityKind(kind)
		out = append(out, r)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("store: 查询业务日志失败: %w", err)
	}
	return out, nil
}

// PurgeActivityBefore 删除指定时刻之前的业务日志，返回删除条数。
func (s *Store) PurgeActivityBefore(ctx context.Context, t time.Time) (int64, error) {
	tag, err := s.pool.Exec(ctx, `DELETE FROM activity_logs WHERE occurred_at < $1`, t)
	if err != nil {
		return 0, fmt.Errorf("store: 清理业务日志失败: %w", err)
	}
	return tag.RowsAffected(), nil
}
```

- [ ] **Step 4: 跑测试确认通过**

```bash
cd server; go test ./internal/store/ -run 'Activity|Purge' -v 2>&1 | tail -35; echo "退出码=$?"
```

预期：全部 PASS。特别留意 `TestActivityDetailRoundTrips`——它验证 `[]byte` 能正确写进 JSONB 列。

- [ ] **Step 5: 提交**

```bash
cd server; gofmt -l . ; go vet ./internal/store/; echo "退出码=$?"
git add server/internal/store/
git commit -m "$(cat <<'EOF'
feat: 业务日志入库

事件与动作合并进同一张表，用 kind 区分，这样一条时间线上就能直接
看到因果：收到弹幕 → 规则触发 → 发出回复。分成两张表得靠时间戳
自己拼。

写入用 COPY：写入器每 200 毫秒或攒够 500 条调一次，逐条 INSERT 会
让往返次数变成瓶颈。查询默认限 100 条——不设上限就是全表扫，一个
活跃房间一天几万行。

Co-Authored-By: Claude Opus 5 <noreply@anthropic.com>
EOF
)"
```

---

## Task 12: 业务日志的异步批量写入器

**Files:**
- Create: `server/internal/logging/activity.go`
- Create: `server/internal/logging/activity_test.go`

**Interfaces:**
- Consumes: `store.ActivityRow`（Task 11）
- Produces:
  - `type logging.ActivityWriterOptions struct { Flush func(context.Context, []store.ActivityRow) error; BufferSize, BatchSize int; Interval, DropReportInterval time.Duration; Logger *slog.Logger }`
  - `func logging.NewActivityWriter(opts ActivityWriterOptions) *ActivityWriter`
  - `func (w *ActivityWriter) Enqueue(row store.ActivityRow)` —— 非阻塞
  - `func (w *ActivityWriter) Close()` —— 停止并冲刷剩余
  - `func (w *ActivityWriter) Dropped() int64`

- [ ] **Step 1: 写失败的测试**

创建 `server/internal/logging/activity_test.go`：

```go
package logging_test

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/MrZhongzq/MagicalDanmaku/server/internal/logging"
	"github.com/MrZhongzq/MagicalDanmaku/server/internal/store"
)

// collector 是记录 flush 调用的假实现。
type collector struct {
	mu     sync.Mutex
	rows   []store.ActivityRow
	calls  int           // flush 被调用的次数
	err    error         // 非 nil 时每次 flush 都失败
	notify chan struct{} // 每次 flush 后发一个信号，测试用来等待而不必 sleep
}

func newCollector() *collector {
	return &collector{notify: make(chan struct{}, 64)}
}

func (c *collector) flush(_ context.Context, rows []store.ActivityRow) error {
	c.mu.Lock()
	c.calls++
	if c.err == nil {
		c.rows = append(c.rows, rows...)
	}
	err := c.err
	c.mu.Unlock()

	select {
	case c.notify <- struct{}{}:
	default:
	}
	return err
}

func (c *collector) snapshot() ([]store.ActivityRow, int) {
	c.mu.Lock()
	defer c.mu.Unlock()
	out := make([]store.ActivityRow, len(c.rows))
	copy(out, c.rows)
	return out, c.calls
}

// waitFlush 等一次 flush 发生，超时则让测试失败。
func (c *collector) waitFlush(t *testing.T) {
	t.Helper()
	select {
	case <-c.notify:
	case <-time.After(3 * time.Second):
		t.Fatal("等待 flush 超时")
	}
}

func TestActivityWriterFlushesOnClose(t *testing.T) {
	c := newCollector()
	w := logging.NewActivityWriter(logging.ActivityWriterOptions{
		Flush:     c.flush,
		BatchSize: 1000,      // 高到不会因条数触发
		Interval:  time.Hour, // 长到不会因时间触发
	})

	w.Enqueue(store.ActivityRow{EventType: "danmaku"})
	w.Enqueue(store.ActivityRow{EventType: "gift"})
	w.Close()

	rows, calls := c.snapshot()
	if len(rows) != 2 {
		t.Fatalf("Close 应冲刷剩余，实际写出 %d 条（flush 调用 %d 次）", len(rows), calls)
	}
	if rows[0].EventType != "danmaku" || rows[1].EventType != "gift" {
		t.Errorf("顺序不对: %+v", rows)
	}
}

func TestActivityWriterFlushesOnBatchSize(t *testing.T) {
	c := newCollector()
	w := logging.NewActivityWriter(logging.ActivityWriterOptions{
		Flush:     c.flush,
		BatchSize: 3,
		Interval:  time.Hour,
	})
	defer w.Close()

	for i := 0; i < 3; i++ {
		w.Enqueue(store.ActivityRow{EventType: "danmaku"})
	}
	c.waitFlush(t)

	rows, _ := c.snapshot()
	if len(rows) != 3 {
		t.Errorf("攒够 3 条应立即写出，实际 %d 条", len(rows))
	}
}

func TestActivityWriterFlushesOnInterval(t *testing.T) {
	c := newCollector()
	w := logging.NewActivityWriter(logging.ActivityWriterOptions{
		Flush:     c.flush,
		BatchSize: 1000,
		Interval:  50 * time.Millisecond,
	})
	defer w.Close()

	w.Enqueue(store.ActivityRow{EventType: "danmaku"})
	c.waitFlush(t)

	rows, _ := c.snapshot()
	if len(rows) != 1 {
		t.Errorf("到时间应写出，实际 %d 条", len(rows))
	}
}

func TestActivityWriterDoesNotFlushEmptyBatches(t *testing.T) {
	// 没有事件时不该每 200 毫秒空转一次数据库
	c := newCollector()
	w := logging.NewActivityWriter(logging.ActivityWriterOptions{
		Flush:     c.flush,
		BatchSize: 1000,
		Interval:  20 * time.Millisecond,
	})
	time.Sleep(150 * time.Millisecond)
	w.Close()

	_, calls := c.snapshot()
	if calls != 0 {
		t.Errorf("空缓冲不该触发 flush，实际调用了 %d 次", calls)
	}
}

// 缓冲满时丢弃并计数——丢日志可以接受，漏欢迎不行
func TestActivityWriterDropsWhenBufferFull(t *testing.T) {
	blocked := make(chan struct{})
	release := make(chan struct{})
	var once sync.Once

	w := logging.NewActivityWriter(logging.ActivityWriterOptions{
		Flush: func(context.Context, []store.ActivityRow) error {
			once.Do(func() { close(blocked) })
			<-release // 卡住写入 goroutine，缓冲会被填满
			return nil
		},
		BufferSize: 4,
		BatchSize:  1,
		Interval:   time.Hour,
	})

	w.Enqueue(store.ActivityRow{EventType: "first"})
	<-blocked // 确认写入 goroutine 已经卡在 flush 里

	// 缓冲只有 4 个位置，塞 200 条必然溢出
	for i := 0; i < 200; i++ {
		w.Enqueue(store.ActivityRow{EventType: "danmaku"})
	}

	if w.Dropped() == 0 {
		t.Error("缓冲满时应丢弃并计数")
	}

	close(release)
	w.Close()
}

// Enqueue 绝不能阻塞：它跑在规则引擎的关键路径上
func TestActivityWriterEnqueueNeverBlocks(t *testing.T) {
	release := make(chan struct{})
	w := logging.NewActivityWriter(logging.ActivityWriterOptions{
		Flush: func(context.Context, []store.ActivityRow) error {
			<-release
			return nil
		},
		BufferSize: 2,
		BatchSize:  1,
		Interval:   time.Hour,
	})

	done := make(chan struct{})
	go func() {
		for i := 0; i < 5000; i++ {
			w.Enqueue(store.ActivityRow{EventType: "danmaku"})
		}
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("Enqueue 阻塞了：它跑在规则引擎的关键路径上，绝不能阻塞")
	}

	close(release)
	w.Close()
}

func TestActivityWriterSurvivesFlushError(t *testing.T) {
	c := newCollector()
	c.err = context.DeadlineExceeded

	w := logging.NewActivityWriter(logging.ActivityWriterOptions{
		Flush:     c.flush,
		BatchSize: 1,
		Interval:  time.Hour,
	})

	w.Enqueue(store.ActivityRow{EventType: "danmaku"})
	c.waitFlush(t)

	// 写入失败后还能继续接收，不该 panic 也不该卡死
	w.Enqueue(store.ActivityRow{EventType: "gift"})
	c.waitFlush(t)
	w.Close()

	_, calls := c.snapshot()
	if calls < 2 {
		t.Errorf("写入失败后应继续工作，flush 只被调用了 %d 次", calls)
	}
}

func TestActivityWriterCloseIsIdempotent(t *testing.T) {
	c := newCollector()
	w := logging.NewActivityWriter(logging.ActivityWriterOptions{Flush: c.flush})
	w.Close()
	w.Close() // 不应 panic
}

func TestActivityWriterEnqueueAfterCloseIsSafe(t *testing.T) {
	c := newCollector()
	w := logging.NewActivityWriter(logging.ActivityWriterOptions{Flush: c.flush})
	w.Close()
	w.Enqueue(store.ActivityRow{EventType: "danmaku"}) // 不应 panic 或写入已关闭的 channel
}

func TestActivityWriterConcurrentEnqueue(t *testing.T) {
	c := newCollector()
	w := logging.NewActivityWriter(logging.ActivityWriterOptions{
		Flush:      c.flush,
		BufferSize: 8192,
		BatchSize:  100,
		Interval:   10 * time.Millisecond,
	})

	var wg sync.WaitGroup
	for g := 0; g < 8; g++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < 200; i++ {
				w.Enqueue(store.ActivityRow{EventType: "danmaku"})
			}
		}()
	}
	wg.Wait()
	w.Close()

	rows, _ := c.snapshot()
	if int64(len(rows))+w.Dropped() != 1600 {
		t.Errorf("写出 %d 条 + 丢弃 %d 条 ≠ 投递的 1600 条", len(rows), w.Dropped())
	}
}
```

- [ ] **Step 2: 跑测试确认失败**

```bash
cd server; go test ./internal/logging/ 2>&1 | tail -5; echo "退出码=$?"
```

预期：`no Go files in .../internal/logging`。

- [ ] **Step 3: 实现**

创建 `server/internal/logging/activity.go`：

```go
// Package logging 装配两类日志。
//
// 系统日志走 stderr 与滚动文件（见 system.go）：数据库连不上时，
// 「数据库连不上」这条日志本身还得写得出来。
//
// 业务日志进数据库（见本文件）：P4 要展示、P5 要统计，必须结构化可查。
package logging

import (
	"context"
	"log/slog"
	"sync"
	"sync/atomic"
	"time"

	"github.com/MrZhongzq/MagicalDanmaku/server/internal/store"
)

// 写入器的默认参数。
const (
	defaultBufferSize         = 4096
	defaultBatchSize          = 500
	defaultInterval           = 200 * time.Millisecond
	defaultDropReportInterval = 30 * time.Second
	flushTimeout              = 10 * time.Second
)

// ActivityWriterOptions 配置业务日志写入器。
type ActivityWriterOptions struct {
	// Flush 把一批日志落库。注入而非直接持有 *store.Store，
	// 是为了让写入器的批量与丢弃逻辑不需要数据库就能测。
	Flush func(context.Context, []store.ActivityRow) error

	BufferSize         int           // 缓冲条数，0 用默认
	BatchSize          int           // 攒够多少条就写一次，0 用默认
	Interval           time.Duration // 攒不够也最多等这么久，0 用默认
	DropReportInterval time.Duration // 多久汇总一次丢弃量，0 用默认
	Logger             *slog.Logger
}

// ActivityWriter 把业务日志异步批量写进数据库。
//
// 活跃房间每秒几十条事件，同步 INSERT 会把数据库延迟压到规则引擎的
// 关键路径上。因此 Enqueue 永不阻塞：缓冲满了就丢弃并计数。
//
// 丢日志可以接受，漏欢迎不行——这个优先级不要改。
type ActivityWriter struct {
	ch      chan store.ActivityRow
	flush   func(context.Context, []store.ActivityRow) error
	batch   int
	tick    time.Duration
	report  time.Duration
	log     *slog.Logger
	dropped atomic.Int64

	stop      chan struct{}
	done      chan struct{}
	closeOnce sync.Once
}

// NewActivityWriter 创建写入器并启动后台 goroutine。
func NewActivityWriter(opts ActivityWriterOptions) *ActivityWriter {
	if opts.BufferSize <= 0 {
		opts.BufferSize = defaultBufferSize
	}
	if opts.BatchSize <= 0 {
		opts.BatchSize = defaultBatchSize
	}
	if opts.Interval <= 0 {
		opts.Interval = defaultInterval
	}
	if opts.DropReportInterval <= 0 {
		opts.DropReportInterval = defaultDropReportInterval
	}
	if opts.Logger == nil {
		opts.Logger = slog.Default()
	}

	w := &ActivityWriter{
		ch:     make(chan store.ActivityRow, opts.BufferSize),
		flush:  opts.Flush,
		batch:  opts.BatchSize,
		tick:   opts.Interval,
		report: opts.DropReportInterval,
		log:    opts.Logger,
		stop:   make(chan struct{}),
		done:   make(chan struct{}),
	}
	go w.run()
	return w
}

// Enqueue 投递一条业务日志。永不阻塞：缓冲满则丢弃并计数。
func (w *ActivityWriter) Enqueue(row store.ActivityRow) {
	select {
	case <-w.stop:
		// 已关闭，直接丢弃：往关掉的写入器里投递不该 panic
		return
	default:
	}

	select {
	case w.ch <- row:
	default:
		w.dropped.Add(1)
	}
}

// Dropped 返回累计丢弃条数。
func (w *ActivityWriter) Dropped() int64 {
	return w.dropped.Load()
}

// Close 停止接收并冲刷剩余日志。可重复调用。
func (w *ActivityWriter) Close() {
	w.closeOnce.Do(func() {
		close(w.stop)
		<-w.done
		if n := w.dropped.Load(); n > 0 {
			w.log.Warn("业务日志有丢弃", "累计丢弃", n)
		}
	})
}

// run 是后台写入循环。
func (w *ActivityWriter) run() {
	defer close(w.done)

	ticker := time.NewTicker(w.tick)
	defer ticker.Stop()
	reporter := time.NewTicker(w.report)
	defer reporter.Stop()

	buf := make([]store.ActivityRow, 0, w.batch)
	lastReported := int64(0)

	for {
		select {
		case row := <-w.ch:
			buf = append(buf, row)
			if len(buf) >= w.batch {
				buf = w.write(buf)
			}

		case <-ticker.C:
			// 空缓冲不写：没有事件时不该每 200 毫秒空转一次数据库
			if len(buf) > 0 {
				buf = w.write(buf)
			}

		case <-reporter.C:
			if n := w.dropped.Load(); n > lastReported {
				w.log.Warn("业务日志缓冲已满，部分记录被丢弃",
					"本轮丢弃", n-lastReported, "累计丢弃", n)
				lastReported = n
			}

		case <-w.stop:
			// 排空 channel 里剩下的，再写最后一批
			for {
				select {
				case row := <-w.ch:
					buf = append(buf, row)
					if len(buf) >= w.batch {
						buf = w.write(buf)
					}
					continue
				default:
				}
				break
			}
			if len(buf) > 0 {
				w.write(buf)
			}
			return
		}
	}
}

// write 落一批日志，返回清空后的缓冲以复用底层数组。
//
// 写入失败只记日志：业务日志不是关键路径，失败不该拖垮机器人。
func (w *ActivityWriter) write(buf []store.ActivityRow) []store.ActivityRow {
	ctx, cancel := context.WithTimeout(context.Background(), flushTimeout)
	defer cancel()

	if err := w.flush(ctx, buf); err != nil {
		w.log.Error("写入业务日志失败", "条数", len(buf), "err", err)
	}
	return buf[:0]
}
```

- [ ] **Step 4: 跑测试确认通过（含竞态检测）**

```bash
cd server; go test ./internal/logging/ -v 2>&1 | tail -30; echo "退出码=$?"
cd server; go test -race -count=3 ./internal/logging/ 2>&1 | tail -10; echo "退出码=$?"
```

预期：两次都 PASS。这个包全是并发逻辑，`-race` 必须干净。

- [ ] **Step 5: 提交**

```bash
cd server; gofmt -l . ; go vet ./internal/logging/; echo "退出码=$?"
git add server/internal/logging/
git commit -m "$(cat <<'EOF'
feat: 业务日志的异步批量写入器

Enqueue 永不阻塞：它跑在规则引擎的关键路径上，活跃房间每秒几十条
事件，同步 INSERT 会把数据库延迟压到规则处理上。缓冲满时丢弃并计数，
每 30 秒向系统日志汇总一次。

丢日志可以接受，漏欢迎不行——这个优先级写死在代码注释里。

flush 以函数注入而非直接持有 *store.Store，批量与丢弃逻辑因此不需要
数据库就能测。

Co-Authored-By: Claude Opus 5 <noreply@anthropic.com>
EOF
)"
```

---

## Task 13: 规则引擎的业务日志钩子

引擎现在只往 slog 里写日志。要让业务日志进库，需要一个钩子：引擎收到事件时记一条，执行动作后再记一条，**两条共用一条时间线**。

**Files:**
- Create: `server/internal/rules/activity.go`
- Create: `server/internal/rules/activity_test.go`
- Create: `server/internal/logging/sink.go`
- Create: `server/internal/logging/sink_test.go`
- Modify: `server/internal/rules/engine.go`（`EngineOptions` 加字段、`NewEngine` 传递、`Handle` 记录）
- Modify: `server/internal/rules/executor.go`（`ExecutorOptions` 加字段、`Execute` 记录）

**Interfaces:**
- Consumes: `event.Event`、`rules.Action`、`rules.Trigger`、`rules.VarsFromEvent`、`rules.LookupPath`、`store.ActivityRow`、`logging.ActivityWriter`（Task 12）
- Produces:
  - `type rules.ActivitySink interface { RecordEvent(ev event.Event); RecordAction(ruleName string, a Action, tr Trigger, err error) }`
  - `rules.EngineOptions.Activity ActivitySink`、`rules.ExecutorOptions.Activity ActivitySink`
  - `func (w *logging.ActivityWriter) Sink(accountID int64, bindingID int64, roomID string) *logging.Sink`
  - `logging.Sink` 实现 `rules.ActivitySink`
  - `func logging.DefaultLoggedEventTypes() map[event.Type]bool`

- [ ] **Step 1: 写 rules 侧的失败测试**

创建 `server/internal/rules/activity_test.go`：

```go
package rules_test

import (
	"sync"
	"testing"
	"time"

	"github.com/MrZhongzq/MagicalDanmaku/server/internal/event"
	"github.com/MrZhongzq/MagicalDanmaku/server/internal/rules"
)

// recordingSink 记录引擎报上来的事件与动作。
type recordingSink struct {
	mu      sync.Mutex
	events  []event.Event
	actions []recordedAction
}

type recordedAction struct {
	rule   string
	action rules.Action
	err    error
}

func (s *recordingSink) RecordEvent(ev event.Event) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.events = append(s.events, ev)
}

func (s *recordingSink) RecordAction(ruleName string, a rules.Action, _ rules.Trigger, err error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.actions = append(s.actions, recordedAction{ruleName, a, err})
}

func (s *recordingSink) snapshot() ([]event.Event, []recordedAction) {
	s.mu.Lock()
	defer s.mu.Unlock()
	evs := make([]event.Event, len(s.events))
	copy(evs, s.events)
	as := make([]recordedAction, len(s.actions))
	copy(as, s.actions)
	return evs, as
}

// fakeBot 记录发出去的弹幕。
type fakeBot struct {
	mu   sync.Mutex
	sent []string
	err  error
}

func (b *fakeBot) SendDanmaku(text string) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.err != nil {
		return b.err
	}
	b.sent = append(b.sent, text)
	return nil
}

func (b *fakeBot) Block(string, int) error { return nil }

// danmakuEvent 构造一条弹幕事件。
func danmakuEvent(uid, name, text string) event.Event {
	return event.Event{
		Type: event.TypeDanmaku,
		Payload: event.Danmaku{
			User: event.User{UID: uid, Username: name},
			Text: text,
		},
	}
}

func TestEngineRecordsEvent(t *testing.T) {
	sink := &recordingSink{}
	eng, err := rules.NewEngine(rules.EngineOptions{
		Label:    "小号@123",
		RoomID:   "123",
		Bot:      &fakeBot{},
		Activity: sink,
		Rules: []rules.Rule{{
			Name: "回复", Enabled: true, On: []event.Type{event.TypeDanmaku},
			Do: []rules.Action{{Type: rules.ActionLog}},
		}},
	})
	if err != nil {
		t.Fatalf("创建引擎报错: %v", err)
	}
	defer eng.Close()

	eng.Handle(danmakuEvent("10086", "张三", "求歌单"))

	evs, _ := sink.snapshot()
	if len(evs) != 1 {
		t.Fatalf("应记录 1 条事件，实际 %d 条", len(evs))
	}
	if evs[0].Type != event.TypeDanmaku {
		t.Errorf("事件类型 = %s", evs[0].Type)
	}
}

// 不匹配任何规则的事件也要记录：业务日志是完整的房间流水，
// 不是「触发过规则的事件」的子集
func TestEngineRecordsUnmatchedEvents(t *testing.T) {
	sink := &recordingSink{}
	eng, err := rules.NewEngine(rules.EngineOptions{
		RoomID:   "123",
		Bot:      &fakeBot{},
		Activity: sink,
		Rules: []rules.Rule{{
			Name: "只管礼物", Enabled: true, On: []event.Type{event.TypeGift},
			Do: []rules.Action{{Type: rules.ActionLog}},
		}},
	})
	if err != nil {
		t.Fatalf("创建引擎报错: %v", err)
	}
	defer eng.Close()

	eng.Handle(danmakuEvent("10086", "张三", "你好"))

	evs, actions := sink.snapshot()
	if len(evs) != 1 {
		t.Errorf("未命中规则的事件也应记录，实际 %d 条", len(evs))
	}
	if len(actions) != 0 {
		t.Errorf("未命中规则不该有动作记录，实际 %d 条", len(actions))
	}
}

func TestEngineRecordsAction(t *testing.T) {
	sink := &recordingSink{}
	bot := &fakeBot{}
	eng, err := rules.NewEngine(rules.EngineOptions{
		RoomID:   "123",
		Bot:      bot,
		Activity: sink,
		Rules: []rules.Rule{{
			Name: "关键词回复", Enabled: true, On: []event.Type{event.TypeDanmaku},
			Do: []rules.Action{{Type: rules.ActionDanmaku, Template: []string{"歌单在动态里"}}},
		}},
	})
	if err != nil {
		t.Fatalf("创建引擎报错: %v", err)
	}
	defer eng.Close()

	eng.Handle(danmakuEvent("10086", "张三", "求歌单"))

	_, actions := sink.snapshot()
	if len(actions) != 1 {
		t.Fatalf("应记录 1 条动作，实际 %d 条", len(actions))
	}
	if actions[0].rule != "关键词回复" {
		t.Errorf("规则名 = %q", actions[0].rule)
	}
	if actions[0].action.Type != rules.ActionDanmaku {
		t.Errorf("动作类型 = %s", actions[0].action.Type)
	}
	if actions[0].err != nil {
		t.Errorf("成功的动作 err 应为 nil，实际 %v", actions[0].err)
	}
}

// 失败的动作也要记录，且带上错误——「为什么没发出去」正是要查的
func TestEngineRecordsFailedAction(t *testing.T) {
	sink := &recordingSink{}
	bot := &fakeBot{err: errSendFailed}
	eng, err := rules.NewEngine(rules.EngineOptions{
		RoomID:   "123",
		Bot:      bot,
		Activity: sink,
		Rules: []rules.Rule{{
			Name: "回复", Enabled: true, On: []event.Type{event.TypeDanmaku},
			Do: []rules.Action{{Type: rules.ActionDanmaku, Template: []string{"你好"}}},
		}},
	})
	if err != nil {
		t.Fatalf("创建引擎报错: %v", err)
	}
	defer eng.Close()

	eng.Handle(danmakuEvent("10086", "张三", "hi"))

	_, actions := sink.snapshot()
	if len(actions) != 1 {
		t.Fatalf("失败的动作也应记录，实际 %d 条", len(actions))
	}
	if actions[0].err == nil {
		t.Error("失败的动作应带上错误")
	}
}

func TestEngineWithoutSinkDoesNotPanic(t *testing.T) {
	// Activity 为 nil 是常见配置（比如单机跑不接数据库）
	eng, err := rules.NewEngine(rules.EngineOptions{
		RoomID: "123",
		Bot:    &fakeBot{},
		Rules: []rules.Rule{{
			Name: "回复", Enabled: true, On: []event.Type{event.TypeDanmaku},
			Do: []rules.Action{{Type: rules.ActionLog}},
		}},
	})
	if err != nil {
		t.Fatalf("创建引擎报错: %v", err)
	}
	defer eng.Close()

	eng.Handle(danmakuEvent("10086", "张三", "hi")) // 不应 panic
}

// 合并窗口的规则：每个原始事件各记一条，动作只在窗口结算时记一条
func TestEngineRecordsEachEventButOneAggregatedAction(t *testing.T) {
	sink := &recordingSink{}
	eng, err := rules.NewEngine(rules.EngineOptions{
		RoomID:   "123",
		Bot:      &fakeBot{},
		Activity: sink,
		Rules: []rules.Rule{{
			Name: "进场欢迎", Enabled: true, On: []event.Type{event.TypeUserEnter},
			Aggregate: &rules.AggregateSpec{Window: 30 * time.Millisecond, By: rules.AggregateByType},
			Do:        []rules.Action{{Type: rules.ActionDanmaku, Template: []string{"欢迎"}}},
		}},
	})
	if err != nil {
		t.Fatalf("创建引擎报错: %v", err)
	}

	for _, uid := range []string{"1", "2", "3"} {
		eng.Handle(event.Event{
			Type:    event.TypeUserEnter,
			Payload: event.UserEnter{User: event.User{UID: uid, Username: "观众" + uid}},
		})
	}
	eng.Close() // Close 会结算未决窗口

	evs, actions := sink.snapshot()
	if len(evs) != 3 {
		t.Errorf("每个原始事件各记一条，实际 %d 条", len(evs))
	}
	if len(actions) != 1 {
		t.Errorf("合并后只该有 1 条动作记录，实际 %d 条", len(actions))
	}
}
```

在同一个文件顶部加上错误变量：

```go
var errSendFailed = errors.New("发送失败")
```

并在 import 里加 `"errors"`。

- [ ] **Step 2: 跑测试确认失败**

```bash
cd server; go test ./internal/rules/ -run TestEngineRecords 2>&1 | tail -5; echo "退出码=$?"
```

预期：编译失败（`EngineOptions` 没有 `Activity` 字段）。

- [ ] **Step 3: 定义钩子接口**

创建 `server/internal/rules/activity.go`：

```go
package rules

import "github.com/MrZhongzq/MagicalDanmaku/server/internal/event"

// ActivitySink 接收引擎的业务动向，用于生成业务日志。
//
// 两个方法都必须**立即返回**：它们跑在事件处理的关键路径上，
// 阻塞会直接拖慢弹幕响应。实现方应当把工作丢进队列而不是原地做完。
//
// 本包不关心日志去了哪里——落库、打文件还是丢掉，是实现方的事。
type ActivitySink interface {
	// RecordEvent 报告收到一个事件。无论是否命中规则都会调用：
	// 业务日志是完整的房间流水，不是「触发过规则的事件」的子集。
	RecordEvent(ev event.Event)

	// RecordAction 报告执行了一个动作。err 非 nil 表示动作失败，
	// 失败的动作同样要记——「为什么没发出去」正是要查的。
	RecordAction(ruleName string, a Action, tr Trigger, err error)
}

// nopSink 是未配置时的空实现。
//
// 用空实现而非到处判 nil：调用点散在热路径上，每处加一个 if
// 既啰嗦又容易漏。
type nopSink struct{}

func (nopSink) RecordEvent(event.Event)                     {}
func (nopSink) RecordAction(string, Action, Trigger, error) {}
```

- [ ] **Step 4: 接进 Engine**

修改 `server/internal/rules/engine.go`：

1. `EngineOptions` 加一个字段，放在 `Storage` 之后：

```go
	Storage        Storage       // 可为 nil，此时使用内存存储
	Activity       ActivitySink  // 可为 nil，此时不产生业务日志
	CooldownGroups map[string]time.Duration
```

2. `Engine` 结构体加一个字段，放在 `log` 之后：

```go
	cooldown *Cooldown
	log      *slog.Logger
	activity ActivitySink
```

3. `NewEngine` 里补默认值，与 `opts.Storage == nil` 那段并列：

```go
	if opts.Storage == nil {
		opts.Storage = NewMemStorage()
	}
	if opts.Activity == nil {
		opts.Activity = nopSink{}
	}
```

4. 构造 `e` 时带上：

```go
	e := &Engine{
		label:       opts.Label,
		roomID:      opts.RoomID,
		cooldown:    cd,
		log:         opts.Logger,
		activity:    opts.Activity,
		aggregators: make(map[string]*Aggregator),
		byName:      make(map[string]Rule, len(opts.Rules)),
	}
```

5. 构造 Executor 时把 sink 传下去：

```go
	e.executor = NewExecutor(ExecutorOptions{
		Bot:      opts.Bot,
		Renderer: NewRenderer(rand.New(rand.NewSource(time.Now().UnixNano()))),
		Script:   sandbox,
		Activity: opts.Activity,
		Logger:   opts.Logger,
	})
```

6. `Handle` 里，在 `closed` 检查之后、匹配之前记录事件：

```go
func (e *Engine) Handle(ev event.Event) {
	e.mu.Lock()
	defer e.mu.Unlock()

	if e.closed {
		return
	}

	// 无论是否命中规则都记录：业务日志是完整的房间流水
	e.activity.RecordEvent(ev)

	// 条件按单个事件求值
	tr := PassthroughTrigger(ev)
	matched := e.matcher.Match(tr)
	...
```

- [ ] **Step 5: 接进 Executor**

修改 `server/internal/rules/executor.go`：

1. `ExecutorOptions` 加字段：

```go
type ExecutorOptions struct {
	Bot               BotAPI
	Renderer          *Renderer
	Script            *Sandbox
	DefaultBlockHours int
	Activity          ActivitySink
	Logger            *slog.Logger
}
```

2. `Executor` 结构体加字段：

```go
type Executor struct {
	bot        BotAPI
	renderer   *Renderer
	script     *Sandbox
	blockHours int
	activity   ActivitySink
	log        *slog.Logger
}
```

3. `NewExecutor` 补默认值并赋值：

```go
	if opts.Logger == nil {
		opts.Logger = slog.Default()
	}
	if opts.Activity == nil {
		opts.Activity = nopSink{}
	}
	return &Executor{
		bot:        opts.Bot,
		renderer:   opts.Renderer,
		script:     opts.Script,
		blockHours: opts.DefaultBlockHours,
		activity:   opts.Activity,
		log:        opts.Logger,
	}
```

4. `Execute` 的循环里，每个动作执行完就上报：

```go
func (e *Executor) Execute(ctx context.Context, r Rule, tr Trigger) error {
	var errs []error

	for i, a := range r.Do {
		err := e.runAction(ctx, r.Name, a, tr)
		// 成功与失败都上报：「为什么没发出去」正是要查的
		e.activity.RecordAction(r.Name, a, tr, err)
		if err != nil {
			e.log.Warn("动作执行失败",
				"rule", r.Name, "action", i+1, "type", a.Type, "err", err)
			errs = append(errs, fmt.Errorf("第 %d 个动作(%s): %w", i+1, a.Type, err))
		}
	}
	...
```

- [ ] **Step 6: 跑 rules 测试确认通过**

```bash
cd server; go test ./internal/rules/ -v -run 'Engine|Executor' 2>&1 | tail -30; echo "退出码=$?"
cd server; go test ./internal/rules/... 2>&1 | tail -10; echo "退出码=$?"
```

预期：新测试 PASS，**且 P2 的全部既有测试仍然 PASS**（钩子是新增的可选项，不该改变任何现有行为）。

- [ ] **Step 7: 写 logging 侧的失败测试**

创建 `server/internal/logging/sink_test.go`：

```go
package logging_test

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/MrZhongzq/MagicalDanmaku/server/internal/event"
	"github.com/MrZhongzq/MagicalDanmaku/server/internal/logging"
	"github.com/MrZhongzq/MagicalDanmaku/server/internal/rules"
	"github.com/MrZhongzq/MagicalDanmaku/server/internal/store"
)

// newTestWriter 建一个同步冲刷的写入器，方便断言。
func newTestWriter(c *collector) *logging.ActivityWriter {
	return logging.NewActivityWriter(logging.ActivityWriterOptions{
		Flush:     c.flush,
		BatchSize: 1000,
		Interval:  time.Hour,
	})
}

func TestSinkImplementsRulesActivitySink(t *testing.T) {
	var _ rules.ActivitySink = (*logging.Sink)(nil)
}

func TestSinkRecordsDanmakuEvent(t *testing.T) {
	c := newCollector()
	w := newTestWriter(c)
	bid := int64(7)
	s := w.Sink(3, bid, "1706666491")

	s.RecordEvent(event.Event{
		Type: event.TypeDanmaku,
		Payload: event.Danmaku{
			User: event.User{UID: "10086", Username: "张三"},
			Text: "求歌单",
		},
	})
	w.Close()

	rows, _ := c.snapshot()
	if len(rows) != 1 {
		t.Fatalf("应写出 1 条，实际 %d 条", len(rows))
	}
	r := rows[0]
	if r.AccountID != 3 || r.BindingID == nil || *r.BindingID != bid {
		t.Errorf("归属 ID 不对: account=%d binding=%v", r.AccountID, r.BindingID)
	}
	if r.RoomID != "1706666491" {
		t.Errorf("RoomID = %q", r.RoomID)
	}
	if r.Kind != store.ActivityEvent || r.EventType != "danmaku" {
		t.Errorf("kind/type = %s / %s", r.Kind, r.EventType)
	}
	if r.UserUID != "10086" || r.UserName != "张三" {
		t.Errorf("用户信息未提取: uid=%q name=%q", r.UserUID, r.UserName)
	}
	if r.OccurredAt.IsZero() {
		t.Error("时间戳不能为零值")
	}

	var detail map[string]any
	if err := json.Unmarshal(r.Detail, &detail); err != nil {
		t.Fatalf("detail 应是合法 JSON: %v (原始 %s)", err, r.Detail)
	}
}

// 排行榜每 8 秒一条且没有分析价值，不记
func TestSinkSkipsNoiseEvents(t *testing.T) {
	c := newCollector()
	w := newTestWriter(c)
	s := w.Sink(1, 1, "123")

	s.RecordEvent(event.Event{Type: event.TypeOnlineRankUpdate})
	s.RecordEvent(event.Event{Type: event.TypeRoomStatsUpdate})
	s.RecordEvent(event.Event{Type: event.TypeUnknown})
	w.Close()

	rows, _ := c.snapshot()
	if len(rows) != 0 {
		t.Errorf("噪声事件不该入库，实际写出 %d 条: %+v", len(rows), rows)
	}
}

func TestSinkRecordsAllBusinessEventTypes(t *testing.T) {
	want := []event.Type{
		event.TypeDanmaku, event.TypeSuperChat, event.TypeGift, event.TypeGiftCombo,
		event.TypeGuardBuy, event.TypeUserEnter, event.TypeUserFollow,
		event.TypeUserShare, event.TypeUserLike, event.TypeUserBlocked,
	}
	logged := logging.DefaultLoggedEventTypes()
	for _, tp := range want {
		if !logged[tp] {
			t.Errorf("业务事件 %q 应被记录", tp)
		}
	}
	if len(logged) != len(want) {
		t.Errorf("默认记录的类型数 = %d, 期望 %d: %v", len(logged), len(want), logged)
	}
}

func TestSinkRecordsAction(t *testing.T) {
	c := newCollector()
	w := newTestWriter(c)
	s := w.Sink(3, 7, "123")

	s.RecordAction("关键词回复",
		rules.Action{Type: rules.ActionDanmaku, Template: []string{"歌单在动态里"}},
		rules.Trigger{
			Type: event.TypeDanmaku,
			Vars: map[string]any{
				"user":  map[string]any{"uid": "10086", "username": "张三"},
				"count": 1,
			},
		}, nil)
	w.Close()

	rows, _ := c.snapshot()
	if len(rows) != 1 {
		t.Fatalf("应写出 1 条，实际 %d 条", len(rows))
	}
	r := rows[0]
	if r.Kind != store.ActivityAction {
		t.Errorf("kind = %s, 期望 action", r.Kind)
	}
	if r.ActionType != "danmaku" || r.RuleName != "关键词回复" {
		t.Errorf("动作信息 = %s / %s", r.ActionType, r.RuleName)
	}
	if r.UserUID != "10086" || r.UserName != "张三" {
		t.Errorf("用户信息应从 Vars 里取: uid=%q name=%q", r.UserUID, r.UserName)
	}
}

func TestSinkRecordsFailedActionWithError(t *testing.T) {
	c := newCollector()
	w := newTestWriter(c)
	s := w.Sink(3, 7, "123")

	s.RecordAction("回复",
		rules.Action{Type: rules.ActionDanmaku},
		rules.Trigger{Type: event.TypeDanmaku, Vars: map[string]any{}},
		context.DeadlineExceeded)
	w.Close()

	rows, _ := c.snapshot()
	if len(rows) != 1 {
		t.Fatalf("失败的动作也应写出，实际 %d 条", len(rows))
	}

	var detail map[string]any
	if err := json.Unmarshal(rows[0].Detail, &detail); err != nil {
		t.Fatalf("解析 detail 报错: %v", err)
	}
	if _, ok := detail["error"]; !ok {
		t.Errorf("失败的动作应把错误写进 detail，实际 %v", detail)
	}
}

// 动作全都要记：机器人干了什么是这份日志的核心，不做类型筛选
func TestSinkRecordsEveryActionType(t *testing.T) {
	c := newCollector()
	w := newTestWriter(c)
	s := w.Sink(3, 7, "123")

	for _, at := range []rules.ActionType{
		rules.ActionDanmaku, rules.ActionBlock, rules.ActionScript, rules.ActionLog,
	} {
		s.RecordAction("规则", rules.Action{Type: at},
			rules.Trigger{Type: event.TypeDanmaku, Vars: map[string]any{}}, nil)
	}
	w.Close()

	rows, _ := c.snapshot()
	if len(rows) != 4 {
		t.Errorf("四种动作都该记，实际 %d 条", len(rows))
	}
}

// 同一个写入器分出的多个 Sink 各自带自己的归属 ID
func TestSinkPerBindingAttribution(t *testing.T) {
	c := newCollector()
	w := newTestWriter(c)

	w.Sink(1, 10, "甲").RecordEvent(event.Event{
		Type: event.TypeDanmaku, Payload: event.Danmaku{Text: "a"},
	})
	w.Sink(2, 20, "乙").RecordEvent(event.Event{
		Type: event.TypeDanmaku, Payload: event.Danmaku{Text: "b"},
	})
	w.Close()

	rows, _ := c.snapshot()
	if len(rows) != 2 {
		t.Fatalf("应写出 2 条，实际 %d 条", len(rows))
	}
	if rows[0].AccountID != 1 || rows[0].RoomID != "甲" {
		t.Errorf("第一条归属不对: %+v", rows[0])
	}
	if rows[1].AccountID != 2 || rows[1].RoomID != "乙" {
		t.Errorf("第二条归属不对: %+v", rows[1])
	}
}
```

- [ ] **Step 8: 实现 Sink**

创建 `server/internal/logging/sink.go`：

```go
package logging

import (
	"encoding/json"
	"time"

	"github.com/MrZhongzq/MagicalDanmaku/server/internal/event"
	"github.com/MrZhongzq/MagicalDanmaku/server/internal/rules"
	"github.com/MrZhongzq/MagicalDanmaku/server/internal/store"
)

// loggedEventTypes 是默认入库的事件类型。
//
// 排行榜（ONLINE_RANK_UPDATE）与房间统计（ROOM_STATS_UPDATE）每 8 秒
// 一条且没有分析价值，不记；未知事件同理——它们的用途是补映射，
// 那由 magicd dump 覆盖。
var loggedEventTypes = map[event.Type]bool{
	event.TypeDanmaku:     true,
	event.TypeSuperChat:   true,
	event.TypeGift:        true,
	event.TypeGiftCombo:   true,
	event.TypeGuardBuy:    true,
	event.TypeUserEnter:   true,
	event.TypeUserFollow:  true,
	event.TypeUserShare:   true,
	event.TypeUserLike:    true,
	event.TypeUserBlocked: true,
}

// DefaultLoggedEventTypes 返回默认入库的事件类型集合的副本。
func DefaultLoggedEventTypes() map[event.Type]bool {
	out := make(map[event.Type]bool, len(loggedEventTypes))
	for k, v := range loggedEventTypes {
		out[k] = v
	}
	return out
}

// Sink 把某个绑定的事件与动作转成业务日志行，实现 rules.ActivitySink。
//
// 每个绑定一个 Sink，共用同一个 ActivityWriter：归属 ID 在这里附上，
// 批量写入的调度只有一份。
type Sink struct {
	w         *ActivityWriter
	accountID int64
	bindingID int64
	roomID    string
	types     map[event.Type]bool
	now       func() time.Time
}

// Sink 为某个绑定创建日志接收器。
func (w *ActivityWriter) Sink(accountID, bindingID int64, roomID string) *Sink {
	return &Sink{
		w:         w,
		accountID: accountID,
		bindingID: bindingID,
		roomID:    roomID,
		types:     loggedEventTypes,
		now:       time.Now,
	}
}

// SetLoggedTypes 覆盖默认的事件类型过滤。
func (s *Sink) SetLoggedTypes(types map[event.Type]bool) {
	s.types = types
}

// RecordEvent 记录一个收到的事件。噪声类型直接丢弃，不进队列。
func (s *Sink) RecordEvent(ev event.Event) {
	if !s.types[ev.Type] {
		return
	}

	vars := rules.VarsFromEvent(ev)
	uid, _ := rules.LookupPath(vars, "user.uid")
	name, _ := rules.LookupPath(vars, "user.username")

	// Payload 而非 Raw：Raw 是完整的 B 站 JSON，体量是 Payload 的数倍，
	// 而排障场景已经由 magicd dump 覆盖。
	detail, err := json.Marshal(ev.Payload)
	if err != nil {
		detail = nil
	}

	s.w.Enqueue(store.ActivityRow{
		AccountID:  s.accountID,
		BindingID:  &s.bindingID,
		RoomID:     s.roomID,
		Kind:       store.ActivityEvent,
		EventType:  string(ev.Type),
		UserUID:    toStr(uid),
		UserName:   toStr(name),
		Detail:     detail,
		OccurredAt: s.now(),
	})
}

// RecordAction 记录一个执行过的动作。
//
// 不做类型筛选：机器人干了什么是这份日志的核心。
func (s *Sink) RecordAction(ruleName string, a rules.Action, tr rules.Trigger, err error) {
	uid, _ := rules.LookupPath(tr.Vars, "user.uid")
	name, _ := rules.LookupPath(tr.Vars, "user.username")

	detail := map[string]any{}
	if n, ok := tr.Vars["count"]; ok {
		detail["count"] = n
	}
	if us, ok := tr.Vars["users"]; ok {
		detail["users"] = us
	}
	if len(a.Template) > 0 {
		detail["template"] = a.Template
	}
	if a.Hours > 0 {
		detail["hours"] = a.Hours
	}
	if err != nil {
		// 「为什么没发出去」正是事后要查的
		detail["error"] = err.Error()
	}

	raw, mErr := json.Marshal(detail)
	if mErr != nil {
		raw = nil
	}

	s.w.Enqueue(store.ActivityRow{
		AccountID:  s.accountID,
		BindingID:  &s.bindingID,
		RoomID:     s.roomID,
		Kind:       store.ActivityAction,
		EventType:  string(tr.Type),
		ActionType: string(a.Type),
		RuleName:   ruleName,
		UserUID:    toStr(uid),
		UserName:   toStr(name),
		Detail:     raw,
		OccurredAt: s.now(),
	})
}

// toStr 把 Vars 里取出的任意值转成字符串，非字符串一律当空。
func toStr(v any) string {
	s, _ := v.(string)
	return s
}
```

- [ ] **Step 9: 跑测试确认通过**

```bash
cd server; go test ./internal/logging/ -v 2>&1 | tail -35; echo "退出码=$?"
cd server; go test -race ./internal/logging/ ./internal/rules/... 2>&1 | tail -10; echo "退出码=$?"
```

- [ ] **Step 10: 跑全量回归**

```bash
cd server; go build ./... ; go vet ./... ; gofmt -l . ; go test ./... 2>&1 | tail -25; echo "退出码=$?"
```

预期：全部 PASS。P2 的既有测试一条不能挂。

- [ ] **Step 11: 提交**

```bash
git add server/internal/rules/ server/internal/logging/
git commit -m "$(cat <<'EOF'
feat: 规则引擎的业务日志钩子

ActivitySink 的两个方法都必须立即返回——它们跑在事件处理的关键
路径上。实现方（logging.Sink）把行丢进队列就走，实际落库由
ActivityWriter 的后台 goroutine 批量完成。

未配置时用 nopSink 而非到处判 nil：调用点散在热路径上，每处加一个
if 既啰嗦又容易漏。

无论是否命中规则都记录事件：业务日志是完整的房间流水，不是「触发过
规则的事件」的子集。失败的动作同样记录并带上错误——「为什么没发
出去」正是事后要查的。

Co-Authored-By: Claude Opus 5 <noreply@anthropic.com>
EOF
)"
```

---

## Task 14: 系统日志

**Files:**
- Create: `server/internal/logging/system.go`
- Create: `server/internal/logging/system_test.go`
- Modify: `server/go.mod`、`server/go.sum`

**Interfaces:**
- Consumes: 无
- Produces:
  - `type logging.SystemOptions struct { Level, File string; MaxSizeMB, MaxBackups, MaxAgeDays int; JSON bool }`
  - `func logging.SystemOptionsFromEnv() SystemOptions`
  - `func logging.SetupSystem(opts SystemOptions) (io.Closer, error)`

- [ ] **Step 1: 加依赖**

```bash
cd server; go get gopkg.in/natefinch/lumberjack.v2@latest; go mod tidy; echo "退出码=$?"
cd server; CGO_ENABLED=0 GOOS=darwin GOARCH=arm64 go build ./... ; echo "退出码=$?"
```

- [ ] **Step 2: 写失败的测试**

创建 `server/internal/logging/system_test.go`：

```go
package logging_test

import (
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/MrZhongzq/MagicalDanmaku/server/internal/logging"
)

func TestSetupSystemWritesToFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "magicd.log")

	closer, err := logging.SetupSystem(logging.SystemOptions{
		Level: "info", File: path, JSON: true,
	})
	if err != nil {
		t.Fatalf("装配系统日志报错: %v", err)
	}

	slog.Info("测试消息", "键", "值")
	if err := closer.Close(); err != nil {
		t.Fatalf("关闭报错: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("读取日志文件报错: %v", err)
	}
	if !strings.Contains(string(data), "测试消息") {
		t.Errorf("日志文件里没有这条消息: %s", data)
	}
	if !strings.Contains(string(data), `"键":"值"`) {
		t.Errorf("结构化字段未写入: %s", data)
	}
}

func TestSetupSystemWithoutFileStillWorks(t *testing.T) {
	// 不配文件时只写 stderr，这是单机跑的默认形态
	closer, err := logging.SetupSystem(logging.SystemOptions{Level: "info"})
	if err != nil {
		t.Fatalf("无文件时应正常装配: %v", err)
	}
	slog.Info("只写 stderr")
	if err := closer.Close(); err != nil {
		t.Errorf("关闭报错: %v", err)
	}
}

func TestSetupSystemRespectsLevel(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "magicd.log")

	closer, err := logging.SetupSystem(logging.SystemOptions{
		Level: "warn", File: path, JSON: true,
	})
	if err != nil {
		t.Fatalf("装配报错: %v", err)
	}

	slog.Debug("调试消息")
	slog.Info("信息消息")
	slog.Warn("警告消息")
	if err := closer.Close(); err != nil {
		t.Fatalf("关闭报错: %v", err)
	}

	data, _ := os.ReadFile(path)
	s := string(data)
	if strings.Contains(s, "调试消息") || strings.Contains(s, "信息消息") {
		t.Errorf("warn 级别不该记录 debug/info: %s", s)
	}
	if !strings.Contains(s, "警告消息") {
		t.Errorf("warn 级别应记录 warn: %s", s)
	}
}

func TestSetupSystemRejectsUnknownLevel(t *testing.T) {
	_, err := logging.SetupSystem(logging.SystemOptions{Level: "详细"})
	if err == nil {
		t.Fatal("未知级别应报错")
	}
	// 报错要列出合法值，否则用户只能翻文档
	if !strings.Contains(err.Error(), "warn") {
		t.Errorf("错误信息应列出合法级别，实际: %v", err)
	}
}

func TestSetupSystemEmptyLevelDefaultsToInfo(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "magicd.log")

	closer, err := logging.SetupSystem(logging.SystemOptions{File: path, JSON: true})
	if err != nil {
		t.Fatalf("空级别应默认为 info: %v", err)
	}
	slog.Info("信息消息")
	slog.Debug("调试消息")
	if err := closer.Close(); err != nil {
		t.Fatalf("关闭报错: %v", err)
	}

	data, _ := os.ReadFile(path)
	s := string(data)
	if !strings.Contains(s, "信息消息") {
		t.Errorf("默认应记录 info: %s", s)
	}
	if strings.Contains(s, "调试消息") {
		t.Errorf("默认不该记录 debug: %s", s)
	}
}

func TestSetupSystemCreatesParentDirectory(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "logs", "sub", "magicd.log")

	closer, err := logging.SetupSystem(logging.SystemOptions{File: path})
	if err != nil {
		t.Fatalf("应自动建目录: %v", err)
	}
	slog.Info("消息")
	if err := closer.Close(); err != nil {
		t.Fatalf("关闭报错: %v", err)
	}

	if _, err := os.Stat(path); err != nil {
		t.Errorf("日志文件未创建: %v", err)
	}
}

func TestSystemOptionsFromEnv(t *testing.T) {
	t.Setenv("MAGICD_LOG_LEVEL", "debug")
	t.Setenv("MAGICD_LOG_FILE", "/tmp/x.log")

	o := logging.SystemOptionsFromEnv()
	if o.Level != "debug" {
		t.Errorf("Level = %q", o.Level)
	}
	if o.File != "/tmp/x.log" {
		t.Errorf("File = %q", o.File)
	}
}

func TestSystemOptionsFromEnvDefaults(t *testing.T) {
	t.Setenv("MAGICD_LOG_LEVEL", "")
	t.Setenv("MAGICD_LOG_FILE", "")

	o := logging.SystemOptionsFromEnv()
	if o.Level != "info" {
		t.Errorf("默认级别 = %q, 期望 info", o.Level)
	}
	if o.File != "" {
		t.Errorf("默认不写文件，实际 %q", o.File)
	}
	if o.MaxSizeMB <= 0 || o.MaxBackups <= 0 {
		t.Errorf("轮转参数应有默认值: %+v", o)
	}
}
```

- [ ] **Step 3: 跑测试确认失败**

```bash
cd server; go test ./internal/logging/ -run TestSetupSystem 2>&1 | tail -5; echo "退出码=$?"
```

- [ ] **Step 4: 实现**

创建 `server/internal/logging/system.go`：

```go
package logging

import (
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	lumberjack "gopkg.in/natefinch/lumberjack.v2"
)

// 日志轮转的默认参数。长期运行的机器人写 INFO 日志会无限增长，
// 轮转不是可选项。
const (
	defaultMaxSizeMB  = 50
	defaultMaxBackups = 5
	defaultMaxAgeDays = 30
)

// SystemOptions 配置系统日志。
//
// 系统日志走 stderr 与文件而非数据库：数据库连不上时，
// 「数据库连不上」这条日志本身还得写得出来。
type SystemOptions struct {
	Level      string // debug / info / warn / error，空则用 info
	File       string // 空则只写 stderr
	MaxSizeMB  int
	MaxBackups int
	MaxAgeDays int
	JSON       bool // 写 JSON 而非文本
}

// SystemOptionsFromEnv 从环境变量读配置。
func SystemOptionsFromEnv() SystemOptions {
	o := SystemOptions{
		Level:      os.Getenv("MAGICD_LOG_LEVEL"),
		File:       os.Getenv("MAGICD_LOG_FILE"),
		MaxSizeMB:  defaultMaxSizeMB,
		MaxBackups: defaultMaxBackups,
		MaxAgeDays: defaultMaxAgeDays,
	}
	if o.Level == "" {
		o.Level = "info"
	}
	return o
}

// noopCloser 是无文件时返回的空关闭器。
type noopCloser struct{}

func (noopCloser) Close() error { return nil }

// SetupSystem 装配系统日志并设为 slog 的默认 Logger。
//
// 返回的 Closer 用于关闭日志文件；无文件时关闭是空操作。
func SetupSystem(opts SystemOptions) (io.Closer, error) {
	level, err := parseLevel(opts.Level)
	if err != nil {
		return nil, err
	}

	// stderr 永远写：容器里 docker logs 看的就是它
	writers := []io.Writer{os.Stderr}
	closer := io.Closer(noopCloser{})

	if opts.File != "" {
		if dir := filepath.Dir(opts.File); dir != "" && dir != "." {
			if err := os.MkdirAll(dir, 0o755); err != nil {
				return nil, fmt.Errorf("logging: 创建日志目录 %s 失败: %w", dir, err)
			}
		}
		lj := &lumberjack.Logger{
			Filename:   opts.File,
			MaxSize:    orDefault(opts.MaxSizeMB, defaultMaxSizeMB),
			MaxBackups: orDefault(opts.MaxBackups, defaultMaxBackups),
			MaxAge:     orDefault(opts.MaxAgeDays, defaultMaxAgeDays),
			Compress:   true,
		}
		writers = append(writers, lj)
		closer = lj
	}

	out := io.MultiWriter(writers...)
	handlerOpts := &slog.HandlerOptions{Level: level}

	var h slog.Handler
	if opts.JSON {
		h = slog.NewJSONHandler(out, handlerOpts)
	} else {
		h = slog.NewTextHandler(out, handlerOpts)
	}
	slog.SetDefault(slog.New(h))
	return closer, nil
}

// parseLevel 把级别名转成 slog.Level。
func parseLevel(s string) (slog.Level, error) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "", "info":
		return slog.LevelInfo, nil
	case "debug":
		return slog.LevelDebug, nil
	case "warn", "warning":
		return slog.LevelWarn, nil
	case "error":
		return slog.LevelError, nil
	default:
		return 0, fmt.Errorf("logging: 未知的日志级别 %q，合法值为 debug, info, warn, error", s)
	}
}

// orDefault 在值非正时返回默认值。
func orDefault(v, def int) int {
	if v <= 0 {
		return def
	}
	return v
}
```

- [ ] **Step 5: 跑测试确认通过**

```bash
cd server; go test ./internal/logging/ -v 2>&1 | tail -30; echo "退出码=$?"
```

注意：这些测试会调用 `slog.SetDefault` 改全局状态，因此**不能并行**。若出现串扰，在每个 `SetupSystem` 测试里加 `t.Cleanup(func() { slog.SetDefault(slog.New(slog.NewTextHandler(os.Stderr, nil))) })` 恢复默认。

- [ ] **Step 6: 全量回归与交叉编译**

```bash
cd server; go vet ./... ; gofmt -l . ; go test ./... 2>&1 | tail -20; echo "退出码=$?"
cd server; for os in windows darwin linux; do for arch in amd64 arm64; do
  CGO_ENABLED=0 GOOS=$os GOARCH=$arch go build -o /dev/null ./... || echo "失败: $os/$arch"
done; done; echo "退出码=$?"
```

- [ ] **Step 7: 提交**

```bash
git add server/internal/logging/ server/go.mod server/go.sum
git commit -m "$(cat <<'EOF'
feat: 系统日志走 stderr 与滚动文件

系统日志不进数据库：数据库连不上时，「数据库连不上」这条日志本身
还得写得出来。stderr 永远写，容器里 docker logs 看的就是它。

用 lumberjack 做轮转——长期运行的机器人写 INFO 日志会无限增长，
轮转不是可选项。

Co-Authored-By: Claude Opus 5 <noreply@anthropic.com>
EOF
)"
```

---

**Part 3 到此结束。** 继续 Task 15–19，见 `2026-07-31-p3-data-layer-part4.md`。
