package store

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
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
}

// Label 返回用于日志的标识，形如 "小号@1706666491"。
func (b Binding) Label() string {
	return b.AccountName + "@" + b.RoomID
}

// bindingColumns 连表带出账号名，Label 才有内容可用。
const bindingColumns = `b.id, b.account_id, a.name, b.room_id, b.enabled, b.created_at, b.updated_at`

func scanBinding(row pgx.Row) (*Binding, error) {
	var b Binding
	if err := row.Scan(&b.ID, &b.AccountID, &b.AccountName, &b.RoomID,
		&b.Enabled, &b.CreatedAt, &b.UpdatedAt); err != nil {
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
