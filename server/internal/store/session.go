package store

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
)

// sessionTokenBytes 是会话令牌的随机字节数。
const sessionTokenBytes = 32

// Session 是一条登录会话。
//
// 令牌原文只在创建时返回一次，库里存的是它的 SHA-256：
// 会话验证只需判断相等，不需还原原文，哈希是免费的。
// 这一点与 B 站 Cookie 不同——那个必须能还原才能拿去请求 B 站。
type Session struct {
	UserID    int64
	CreatedAt time.Time
	ExpiresAt time.Time
	UserAgent string
}

// hashToken 计算令牌的存储形式。
func hashToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

// CreateSession 新建一条会话，返回令牌原文。原文只此一次可得。
func (s *Store) CreateSession(ctx context.Context, userID int64, ttl time.Duration, userAgent string) (string, error) {
	b := make([]byte, sessionTokenBytes)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("store: 生成会话令牌失败: %w", err)
	}
	token := base64.RawURLEncoding.EncodeToString(b)

	_, err := s.pool.Exec(ctx, `
		INSERT INTO sessions (token_hash, user_id, expires_at, user_agent)
		VALUES ($1, $2, $3, $4)`,
		hashToken(token), userID, time.Now().Add(ttl), userAgent)
	if err != nil {
		return "", fmt.Errorf("store: 创建会话失败: %w", err)
	}
	return token, nil
}

// LookupSession 用令牌换用户。令牌无效或已过期都返回 ErrNotFound。
//
// 过期判断放在 SQL 里而不是读出来再比：过期的会话与不存在的会话
// 对调用方是同一件事，没必要让它自己判。
func (s *Store) LookupSession(ctx context.Context, token string) (*User, error) {
	var u User
	err := s.pool.QueryRow(ctx, `
		SELECT u.id, u.username, u.is_admin, u.created_at, u.updated_at
		FROM sessions s JOIN users u ON u.id = s.user_id
		WHERE s.token_hash = $1 AND s.expires_at > now()`,
		hashToken(token),
	).Scan(&u.ID, &u.Username, &u.IsAdmin, &u.CreatedAt, &u.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, fmt.Errorf("store: 会话无效或已过期: %w", ErrNotFound)
	}
	if err != nil {
		return nil, fmt.Errorf("store: 查询会话失败: %w", err)
	}
	return &u, nil
}

// DeleteSession 撤销一条会话。令牌不存在不算错误——
// 用户点登出却收到一个错误页，很蠢。
func (s *Store) DeleteSession(ctx context.Context, token string) error {
	if _, err := s.pool.Exec(ctx,
		`DELETE FROM sessions WHERE token_hash = $1`, hashToken(token)); err != nil {
		return fmt.Errorf("store: 撤销会话失败: %w", err)
	}
	return nil
}

// DeleteUserSessions 撤销某用户的全部会话，返回撤销条数。改密码时用。
func (s *Store) DeleteUserSessions(ctx context.Context, userID int64) (int64, error) {
	tag, err := s.pool.Exec(ctx, `DELETE FROM sessions WHERE user_id = $1`, userID)
	if err != nil {
		return 0, fmt.Errorf("store: 撤销用户会话失败: %w", err)
	}
	return tag.RowsAffected(), nil
}

// PurgeExpiredSessions 清理过期会话，返回清理条数。
func (s *Store) PurgeExpiredSessions(ctx context.Context) (int64, error) {
	tag, err := s.pool.Exec(ctx, `DELETE FROM sessions WHERE expires_at <= now()`)
	if err != nil {
		return 0, fmt.Errorf("store: 清理过期会话失败: %w", err)
	}
	return tag.RowsAffected(), nil
}
