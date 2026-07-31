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
