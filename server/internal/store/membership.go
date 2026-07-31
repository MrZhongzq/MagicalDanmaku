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
// 三条通路，任一成立即放行：
//
//  1. 管理员——绕过全部检查
//  2. 该绑定所属账号的所有者，且 p 不是 perm.MemberManage——所有者
//     已经能删账号、删绑定、换 Cookie，却不能停用自己的绑定，那是
//     不一致而不是安全。但那些既有权力全是收缩性的（能清空别人的
//     访问），推不出凭空赋予一个新人访问的委派权，所以这条通路对
//     MemberManage 不生效，规则定义在 perm.OwnerBypass，此处只引用
//  3. memberships 表里有行且包含这个权限点
//
// 没有命中以上任何一条时一律拒绝——默认拒绝，不给「忘了配就等于
// 放行」留口子。
//
// permissions 必须用 @> 而不是 = ANY：只有 @> 能走 GIN 索引，
// = ANY 是逐行的数组展开，PostgreSQL 不会把它改写成可索引的形式，
// memberships_permissions_idx 那个索引对它完全不起作用，写错了不
// 报错，只是退化成全表扫描。
func (s *Store) Can(ctx context.Context, userID, bindingID int64, p perm.Permission) (bool, error) {
	// 所有者通路不覆盖 member:manage，规则定义在 perm.OwnerBypass，
	// 这里不重复判断逻辑，只把结果当参数传给 SQL。
	ownerBypass := perm.OwnerBypass(p)

	var ok bool
	err := s.pool.QueryRow(ctx, `
		SELECT EXISTS (SELECT 1 FROM users WHERE id = $1 AND is_admin)
		    OR ($4 AND EXISTS (
				SELECT 1 FROM bindings b
				JOIN accounts a ON a.id = b.account_id
				WHERE b.id = $2 AND a.owner_id = $1
			))
		    OR EXISTS (
				SELECT 1 FROM memberships
				WHERE user_id = $1 AND binding_id = $2
				  AND permissions @> ARRAY[$3::text]
			)`, userID, bindingID, string(p), ownerBypass).Scan(&ok)
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
