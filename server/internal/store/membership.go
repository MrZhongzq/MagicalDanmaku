package store

import (
	"context"
	"fmt"

	"github.com/MrZhongzq/MagicalDanmaku/server/internal/perm"
)

// Membership 是一条授权：某用户对某绑定拥有哪些权限点。
//
// 授权单位是「账号-直播间」绑定，与规则引擎的运行单元对齐。这样
// 「能改小号在甲房间的规则，但碰不到乙房间」可以直接表达。
type Membership struct {
	ID          int64
	UserID      int64
	BindingID   int64
	Username    string
	AccountName string
	RoomID      string
	Permissions []perm.Permission
}

// Grant 授予权限点。已有授权则整组替换。
//
// 替换而非累加：重新授权的语义是「设定为这些」，累加会让人以为
// 撤掉了某项其实还在。
func (s *Store) Grant(ctx context.Context, username, accountName, roomID string, ps []perm.Permission) error {
	if len(ps) == 0 {
		return fmt.Errorf("store: 权限点列表为空，至少要给一个")
	}

	u, err := s.GetUserByName(ctx, username)
	if err != nil {
		return err
	}
	b, err := s.GetBinding(ctx, accountName, roomID)
	if err != nil {
		return err
	}

	_, err = s.pool.Exec(ctx, `
		INSERT INTO memberships (user_id, binding_id, permissions)
		VALUES ($1, $2, $3)
		ON CONFLICT (user_id, binding_id) DO UPDATE SET
			permissions = EXCLUDED.permissions,
			updated_at  = now()`,
		u.ID, b.ID, perm.Strings(ps))
	if err != nil {
		return fmt.Errorf("store: 授权 %s → %s 失败: %w", username, b.Label(), err)
	}
	return nil
}

// Revoke 撤销某用户对某绑定的全部权限。
func (s *Store) Revoke(ctx context.Context, username, accountName, roomID string) error {
	tag, err := s.pool.Exec(ctx, `
		DELETE FROM memberships m
		USING users u, bindings b, accounts a
		WHERE m.user_id = u.id AND m.binding_id = b.id AND b.account_id = a.id
		  AND u.username = $1 AND a.name = $2 AND b.room_id = $3`,
		username, accountName, roomID)
	if err != nil {
		return fmt.Errorf("store: 撤销授权失败: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("store: %s 在 %s@%s 上没有授权记录: %w",
			username, accountName, roomID, ErrNotFound)
	}
	return nil
}

// Can 判断用户对绑定是否拥有某个权限点。
//
// 管理员绕过全部检查。没有授权记录时一律拒绝——默认拒绝，
// 不给「忘了配就等于放行」留口子。
//
// 数组条件必须写成 `permissions @> ARRAY[...]`，不能写
// `$3 = ANY(permissions)`：后者是逐行的数组展开，PostgreSQL 不会把它
// 改写成可索引的形式，memberships_permissions_idx 那个 GIN 索引对它
// 完全不起作用。两种写法语义相同，写错了不报错，只是退化成全表扫描。
func (s *Store) Can(ctx context.Context, userID, bindingID int64, p perm.Permission) (bool, error) {
	var ok bool
	err := s.pool.QueryRow(ctx, `
		SELECT EXISTS (SELECT 1 FROM users WHERE id = $1 AND is_admin)
		    OR EXISTS (
				SELECT 1 FROM memberships
				WHERE user_id = $1 AND binding_id = $2
				  AND permissions @> ARRAY[$3::text]
			)`, userID, bindingID, string(p)).Scan(&ok)
	if err != nil {
		return false, fmt.Errorf("store: 权限检查失败: %w", err)
	}
	return ok, nil
}

const membershipColumns = `m.id, m.user_id, m.binding_id, u.username, a.name, b.room_id, m.permissions`

// scanMemberships 读出结果集。两个 List 方法只差 WHERE 子句。
func (s *Store) scanMemberships(ctx context.Context, where string, args ...any) ([]Membership, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT `+membershipColumns+`
		FROM memberships m
		JOIN users    u ON u.id = m.user_id
		JOIN bindings b ON b.id = m.binding_id
		JOIN accounts a ON a.id = b.account_id
		WHERE `+where+`
		ORDER BY m.id`, args...)
	if err != nil {
		return nil, fmt.Errorf("store: 列出授权失败: %w", err)
	}
	defer rows.Close()

	var out []Membership
	for rows.Next() {
		var m Membership
		var ps []string
		if err := rows.Scan(&m.ID, &m.UserID, &m.BindingID,
			&m.Username, &m.AccountName, &m.RoomID, &ps); err != nil {
			return nil, fmt.Errorf("store: 读取授权失败: %w", err)
		}
		m.Permissions = make([]perm.Permission, len(ps))
		for i, p := range ps {
			m.Permissions[i] = perm.Permission(p)
		}
		out = append(out, m)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("store: 列出授权失败: %w", err)
	}
	return out, nil
}

// ListMemberships 列出某用户的全部授权。
func (s *Store) ListMemberships(ctx context.Context, username string) ([]Membership, error) {
	return s.scanMemberships(ctx, `u.username = $1`, username)
}

// ListBindingMembers 列出某绑定上被授权的全部用户。
func (s *Store) ListBindingMembers(ctx context.Context, accountName, roomID string) ([]Membership, error) {
	return s.scanMemberships(ctx, `a.name = $1 AND b.room_id = $2`, accountName, roomID)
}
