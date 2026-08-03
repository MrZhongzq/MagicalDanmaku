package store

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
)

// 直播间开播状态的三个取值（对应迁移 004 里 live_status 列的 CHECK
// 约束）。语义与账号登录态（LoginState*）完全对称：unknown 同时表示
// "从未检测过"与"最近一次探测本身失败"，网络不通/被风控不等于真的
// 没开播，两者不能共用同一个值——否则界面会把"拿不到"误报成"确认
// 没开播"，这正是 P5-2 需求反复强调的一条红线。
const (
	RoomLiveUnknown = "unknown" // 尚未检测过，或最近一次探测失败（探测本身出错，而非确认未开播）
	RoomLiveLiving  = "living"  // 最近一次检测确认正在直播
	RoomLiveOffline = "offline" // 最近一次检测确认未开播（含轮播中）
)

// Binding 是「账号-直播间」组合，也是规则引擎的运行单元。
//
// 同一直播间被两个账号连接时是两条独立绑定，各自有独立的连接、
// 规则集与冷却状态，互不知道对方存在。
type Binding struct {
	ID          int64
	AccountID   int64
	AccountName string
	RoomID      string
	Enabled     bool
	CreatedAt   time.Time
	UpdatedAt   time.Time

	// LiveStatus 是最近一次直播间开播状态检测的结果，见 RoomLiveLiving 等常量。
	LiveStatus string
	// LiveCheckedAt 是最近一次尝试检测的时间（无论检测成功与否）；
	// 从未检测过为 nil。
	LiveCheckedAt *time.Time
	// AnchorUID 是主播 UID（不是房间号）。探测成功前为空串。
	AnchorUID string
	// AnchorName 是主播昵称。探测成功前为空串；探测失败时保留上一次
	// 已知值，不会被空值覆盖（见 UpdateBindingRoomStatus 的说明）。
	AnchorName string
}

// Label 返回用于日志的标识，形如 "小号@1706666491"。
func (b Binding) Label() string {
	return b.AccountName + "@" + b.RoomID
}

// bindingColumns 连表带出账号名，Label 才有内容可用。
const bindingColumns = `b.id, b.account_id, a.name, b.room_id, b.enabled, b.created_at, b.updated_at,
	b.live_status, b.live_checked_at, b.anchor_uid, b.anchor_name`

func scanBinding(row pgx.Row) (*Binding, error) {
	var b Binding
	if err := row.Scan(&b.ID, &b.AccountID, &b.AccountName, &b.RoomID,
		&b.Enabled, &b.CreatedAt, &b.UpdatedAt,
		&b.LiveStatus, &b.LiveCheckedAt, &b.AnchorUID, &b.AnchorName); err != nil {
		return nil, err
	}
	return &b, nil
}

// UpsertBinding 创建或取回「账号-直播间」绑定。
//
// 幂等：已存在则原样返回，不改动 enabled——重新导入配置不该
// 把用户手动停用的绑定又打开。
func (s *Store) UpsertBinding(ctx context.Context, accountID int64, roomID string) (*Binding, error) {
	if roomID == "" {
		return nil, fmt.Errorf("store: 房间号不能为空")
	}
	_, err := s.pool.Exec(ctx, `
		INSERT INTO bindings (account_id, room_id) VALUES ($1, $2)
		ON CONFLICT (account_id, room_id) DO NOTHING`, accountID, roomID)
	if err != nil {
		return nil, fmt.Errorf("store: 创建绑定失败: %w", err)
	}

	b, err := scanBinding(s.pool.QueryRow(ctx, `
		SELECT `+bindingColumns+`
		FROM bindings b JOIN accounts a ON a.id = b.account_id
		WHERE b.account_id = $1 AND b.room_id = $2`, accountID, roomID))
	if err != nil {
		return nil, fmt.Errorf("store: 读取绑定失败: %w", err)
	}
	return b, nil
}

// GetBinding 按账号名与房间号查绑定。
func (s *Store) GetBinding(ctx context.Context, accountName, roomID string) (*Binding, error) {
	b, err := scanBinding(s.pool.QueryRow(ctx, `
		SELECT `+bindingColumns+`
		FROM bindings b JOIN accounts a ON a.id = b.account_id
		WHERE a.name = $1 AND b.room_id = $2`, accountName, roomID))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, fmt.Errorf("store: 绑定 %s@%s 不存在: %w", accountName, roomID, ErrNotFound)
	}
	if err != nil {
		return nil, fmt.Errorf("store: 查询绑定失败: %w", err)
	}
	return b, nil
}

// ListBindings 列出全部绑定，按账号与房间号排序。
func (s *Store) ListBindings(ctx context.Context) ([]Binding, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT `+bindingColumns+`
		FROM bindings b JOIN accounts a ON a.id = b.account_id
		ORDER BY a.id, b.id`)
	if err != nil {
		return nil, fmt.Errorf("store: 列出绑定失败: %w", err)
	}
	defer rows.Close()

	var out []Binding
	for rows.Next() {
		b, err := scanBinding(rows)
		if err != nil {
			return nil, fmt.Errorf("store: 读取绑定失败: %w", err)
		}
		out = append(out, *b)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("store: 列出绑定失败: %w", err)
	}
	return out, nil
}

// GetBindingByID 按主键查绑定。
//
// 与 GetBinding（按账号名+房间号）互补：cmd/magicd 的直播间状态心跳/
// 立即检测流程只有 bindingID（URL 路径参数与心跳循环遍历到的都是
// 主键），直接按主键索引查询，不必先 ListBindings 整表再线性挑。
func (s *Store) GetBindingByID(ctx context.Context, id int64) (*Binding, error) {
	b, err := scanBinding(s.pool.QueryRow(ctx, `
		SELECT `+bindingColumns+`
		FROM bindings b JOIN accounts a ON a.id = b.account_id
		WHERE b.id = $1`, id))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, fmt.Errorf("store: 绑定 %d 不存在: %w", id, ErrNotFound)
	}
	if err != nil {
		return nil, fmt.Errorf("store: 查询绑定失败: %w", err)
	}
	return b, nil
}

// UpdateBindingRoomStatus 写入一次直播间状态检测的结果。
//
// status 必须是 RoomLiveLiving/RoomLiveOffline/RoomLiveUnknown 三态之一，
// 由调用方（cmd/magicd 的心跳循环/立即检测）判定后传入——探测失败时
// 传 RoomLiveUnknown，绝不能把探测失败当作 RoomLiveOffline 写进来，
// 那等于把"拿不到"误报成"确认没开播"。
//
// anchorUID/anchorName 为空串时保留原值不覆盖：探测失败时没有新的
// 主播身份可写，不能用空值把上一次探测成功时留下的信息抹掉——网络
// 抖动/风控这类瞬时失败不该让界面上已经显示出来的主播昵称突然消失。
//
// 绑定不存在时不算错误：心跳循环遍历的是某一时刻的绑定快照，
// 绑定在检测期间被删掉是正常竞态，不该让整轮检测因此报错。
func (s *Store) UpdateBindingRoomStatus(ctx context.Context, bindingID int64, status, anchorUID, anchorName string) error {
	switch status {
	case RoomLiveLiving, RoomLiveOffline, RoomLiveUnknown:
	default:
		return fmt.Errorf("store: 非法的直播间状态取值 %q", status)
	}
	if _, err := s.pool.Exec(ctx, `
		UPDATE bindings SET
			live_status = $1,
			live_checked_at = now(),
			anchor_uid = CASE WHEN $2 = '' THEN anchor_uid ELSE $2 END,
			anchor_name = CASE WHEN $3 = '' THEN anchor_name ELSE $3 END
		WHERE id = $4`,
		status, anchorUID, anchorName, bindingID); err != nil {
		return fmt.Errorf("store: 写入绑定 %d 的直播间状态失败: %w", bindingID, err)
	}
	return nil
}

// SetBindingEnabled 启用或停用绑定。停用的绑定 run 时不会连接。
func (s *Store) SetBindingEnabled(ctx context.Context, accountName, roomID string, enabled bool) error {
	tag, err := s.pool.Exec(ctx, `
		UPDATE bindings b SET enabled = $1, updated_at = now()
		FROM accounts a
		WHERE a.id = b.account_id AND a.name = $2 AND b.room_id = $3`,
		enabled, accountName, roomID)
	if err != nil {
		return fmt.Errorf("store: 更新绑定状态失败: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("store: 绑定 %s@%s 不存在: %w", accountName, roomID, ErrNotFound)
	}
	return nil
}

// DeleteBinding 删除绑定，连带其规则、冷却组、KV 与禁言名单。
func (s *Store) DeleteBinding(ctx context.Context, accountName, roomID string) error {
	tag, err := s.pool.Exec(ctx, `
		DELETE FROM bindings b USING accounts a
		WHERE a.id = b.account_id AND a.name = $1 AND b.room_id = $2`,
		accountName, roomID)
	if err != nil {
		return fmt.Errorf("store: 删除绑定失败: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("store: 绑定 %s@%s 不存在: %w", accountName, roomID, ErrNotFound)
	}
	return nil
}

// SetCooldownGroups 整组替换某绑定的冷却组。
//
// 替换而非合并：从界面上删掉一个冷却组，就该真的消失。
func (s *Store) SetCooldownGroups(ctx context.Context, bindingID int64, groups map[string]time.Duration) error {
	err := pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx,
			`DELETE FROM cooldown_groups WHERE binding_id = $1`, bindingID); err != nil {
			return err
		}
		for name, d := range groups {
			if _, err := tx.Exec(ctx, `
				INSERT INTO cooldown_groups (binding_id, name, interval_ms)
				VALUES ($1, $2, $3)`,
				bindingID, name, int(d/time.Millisecond)); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return fmt.Errorf("store: 写入冷却组失败: %w", err)
	}
	return nil
}

// CooldownGroups 读出某绑定的冷却组。未配置时返回空 map 而非 nil。
func (s *Store) CooldownGroups(ctx context.Context, bindingID int64) (map[string]time.Duration, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT name, interval_ms FROM cooldown_groups WHERE binding_id = $1`, bindingID)
	if err != nil {
		return nil, fmt.Errorf("store: 读取冷却组失败: %w", err)
	}
	defer rows.Close()

	out := make(map[string]time.Duration)
	for rows.Next() {
		var name string
		var ms int
		if err := rows.Scan(&name, &ms); err != nil {
			return nil, fmt.Errorf("store: 读取冷却组失败: %w", err)
		}
		out[name] = time.Duration(ms) * time.Millisecond
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("store: 读取冷却组失败: %w", err)
	}
	return out, nil
}
