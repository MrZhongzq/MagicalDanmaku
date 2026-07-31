package store

import (
	"context"
	"fmt"
	"time"
)

// BlockedUser 是永久禁言名单里的一条。
//
// 与 B 站自身的禁言时长无关：这是本地名单，用来在用户再次出现时
// 重新执行禁言。
type BlockedUser struct {
	ID        int64
	UID       string
	Username  string
	Reason    string
	CreatedBy *int64 // 操作人被删除后置空，名单本身保留
	CreatedAt time.Time
}

// AddToBlockList 把用户加入某绑定的禁言名单。已在名单里则更新信息。
func (s *Store) AddToBlockList(ctx context.Context, bindingID int64, uid, username, reason string, createdBy *int64) error {
	if uid == "" {
		return fmt.Errorf("store: 禁言名单的 UID 不能为空")
	}
	_, err := s.pool.Exec(ctx, `
		INSERT INTO block_list (binding_id, uid, username, reason, created_by)
		VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (binding_id, uid) DO UPDATE SET
			username   = EXCLUDED.username,
			reason     = EXCLUDED.reason,
			created_by = EXCLUDED.created_by`,
		bindingID, uid, username, reason, createdBy)
	if err != nil {
		return fmt.Errorf("store: 加入禁言名单失败: %w", err)
	}
	return nil
}

// RemoveFromBlockList 把用户移出禁言名单。
func (s *Store) RemoveFromBlockList(ctx context.Context, bindingID int64, uid string) error {
	tag, err := s.pool.Exec(ctx,
		`DELETE FROM block_list WHERE binding_id = $1 AND uid = $2`, bindingID, uid)
	if err != nil {
		return fmt.Errorf("store: 移出禁言名单失败: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("store: UID %s 不在禁言名单里: %w", uid, ErrNotFound)
	}
	return nil
}

// ListBlockList 列出某绑定的禁言名单。
func (s *Store) ListBlockList(ctx context.Context, bindingID int64) ([]BlockedUser, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id, uid, username, reason, created_by, created_at
		FROM block_list WHERE binding_id = $1 ORDER BY id`, bindingID)
	if err != nil {
		return nil, fmt.Errorf("store: 列出禁言名单失败: %w", err)
	}
	defer rows.Close()

	var out []BlockedUser
	for rows.Next() {
		var b BlockedUser
		if err := rows.Scan(&b.ID, &b.UID, &b.Username, &b.Reason,
			&b.CreatedBy, &b.CreatedAt); err != nil {
			return nil, fmt.Errorf("store: 读取禁言名单失败: %w", err)
		}
		out = append(out, b)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("store: 列出禁言名单失败: %w", err)
	}
	return out, nil
}

// IsBlocked 判断某 UID 是否在名单内。
func (s *Store) IsBlocked(ctx context.Context, bindingID int64, uid string) (bool, error) {
	var ok bool
	err := s.pool.QueryRow(ctx, `
		SELECT EXISTS (SELECT 1 FROM block_list WHERE binding_id = $1 AND uid = $2)`,
		bindingID, uid).Scan(&ok)
	if err != nil {
		return false, fmt.Errorf("store: 查询禁言名单失败: %w", err)
	}
	return ok, nil
}
