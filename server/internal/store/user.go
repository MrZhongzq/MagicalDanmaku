package store

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"golang.org/x/crypto/bcrypt"
)

// 存储层的哨兵错误。
var (
	ErrNotFound       = errors.New("store: 记录不存在")
	ErrDuplicate      = errors.New("store: 记录已存在")
	ErrBadCredentials = errors.New("store: 用户名或密码错误")
)

// defaultAdminName 是首次迁移时自动创建的管理员用户名。
const defaultAdminName = "admin"

// User 是系统用户，即使用本软件的人。
//
// 与 Account（机器人操作的 B 站账号）是两回事：用户用密码登录管理
// 界面，账号用 Cookie 操作直播间。
type User struct {
	ID        int64
	Username  string
	IsAdmin   bool
	CreatedAt time.Time
	UpdatedAt time.Time
}

// CreateUser 创建用户。密码以 bcrypt 哈希存储。
func (s *Store) CreateUser(ctx context.Context, username, password string, isAdmin bool) (*User, error) {
	username = strings.TrimSpace(username)
	if username == "" {
		return nil, fmt.Errorf("store: 用户名不能为空")
	}
	if password == "" {
		return nil, fmt.Errorf("store: 密码不能为空")
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return nil, fmt.Errorf("store: 生成密码哈希失败: %w", err)
	}

	var u User
	err = s.pool.QueryRow(ctx, `
		INSERT INTO users (username, password_hash, is_admin)
		VALUES ($1, $2, $3)
		RETURNING id, username, is_admin, created_at, updated_at`,
		username, string(hash), isAdmin,
	).Scan(&u.ID, &u.Username, &u.IsAdmin, &u.CreatedAt, &u.UpdatedAt)
	if err != nil {
		if isUniqueViolation(err) {
			return nil, fmt.Errorf("store: 用户名 %q 已被占用: %w", username, ErrDuplicate)
		}
		return nil, fmt.Errorf("store: 创建用户失败: %w", err)
	}
	return &u, nil
}

// GetUserByName 按用户名查用户。
func (s *Store) GetUserByName(ctx context.Context, username string) (*User, error) {
	var u User
	err := s.pool.QueryRow(ctx, `
		SELECT id, username, is_admin, created_at, updated_at
		FROM users WHERE username = $1`, username,
	).Scan(&u.ID, &u.Username, &u.IsAdmin, &u.CreatedAt, &u.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, fmt.Errorf("store: 用户 %q 不存在: %w", username, ErrNotFound)
	}
	if err != nil {
		return nil, fmt.Errorf("store: 查询用户失败: %w", err)
	}
	return &u, nil
}

// ListUsers 按创建顺序列出全部用户。
func (s *Store) ListUsers(ctx context.Context) ([]User, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id, username, is_admin, created_at, updated_at
		FROM users ORDER BY id`)
	if err != nil {
		return nil, fmt.Errorf("store: 列出用户失败: %w", err)
	}
	defer rows.Close()

	var out []User
	for rows.Next() {
		var u User
		if err := rows.Scan(&u.ID, &u.Username, &u.IsAdmin, &u.CreatedAt, &u.UpdatedAt); err != nil {
			return nil, fmt.Errorf("store: 读取用户失败: %w", err)
		}
		out = append(out, u)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("store: 列出用户失败: %w", err)
	}
	return out, nil
}

// SetPassword 修改密码。
func (s *Store) SetPassword(ctx context.Context, username, password string) error {
	if password == "" {
		return fmt.Errorf("store: 密码不能为空")
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("store: 生成密码哈希失败: %w", err)
	}

	tag, err := s.pool.Exec(ctx, `
		UPDATE users SET password_hash = $1, updated_at = now() WHERE username = $2`,
		string(hash), username)
	if err != nil {
		return fmt.Errorf("store: 修改密码失败: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("store: 用户 %q 不存在: %w", username, ErrNotFound)
	}
	return nil
}

// VerifyPassword 校验密码，成功则返回用户。
//
// 用户名不存在与密码错误返回完全相同的错误：区分开来，这个接口就成了
// 用户名枚举器。同理，用户不存在时也走一遍 bcrypt 比对，避免用响应
// 时间的差异泄露用户是否存在。
func (s *Store) VerifyPassword(ctx context.Context, username, password string) (*User, error) {
	var u User
	var hash string
	err := s.pool.QueryRow(ctx, `
		SELECT id, username, is_admin, created_at, updated_at, password_hash
		FROM users WHERE username = $1`, username,
	).Scan(&u.ID, &u.Username, &u.IsAdmin, &u.CreatedAt, &u.UpdatedAt, &hash)

	if errors.Is(err, pgx.ErrNoRows) {
		// 拿一个固定的合法 bcrypt 哈希做无用功，让耗时与真实路径相当
		_ = bcrypt.CompareHashAndPassword([]byte(dummyHash), []byte(password))
		return nil, ErrBadCredentials
	}
	if err != nil {
		return nil, fmt.Errorf("store: 查询用户失败: %w", err)
	}

	if err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)); err != nil {
		return nil, ErrBadCredentials
	}
	return &u, nil
}

// dummyHash 是一个固定的合法 bcrypt 哈希，其明文无关紧要——它只用来
// 在「用户不存在」的路径上消耗与真实比对相当的时间。
const dummyHash = "$2a$10$N9qo8uLOickgx2ZMRZoMyeIjZAgcfl7p92ldGxad68LJZdL17lhWy"

// CountUsers 返回用户总数。
func (s *Store) CountUsers(ctx context.Context) (int, error) {
	var n int
	if err := s.pool.QueryRow(ctx, `SELECT count(*) FROM users`).Scan(&n); err != nil {
		return 0, fmt.Errorf("store: 统计用户数失败: %w", err)
	}
	return n, nil
}

// EnsureAdmin 在库里一个用户都没有时创建管理员，并返回随机密码。
//
// 密码只在这一次返回，之后无法找回——库里只有哈希。调用方必须把它
// 打印出来。
func (s *Store) EnsureAdmin(ctx context.Context) (username, password string, created bool, err error) {
	n, err := s.CountUsers(ctx)
	if err != nil {
		return "", "", false, err
	}
	if n > 0 {
		return "", "", false, nil
	}

	pass, err := randomPassword()
	if err != nil {
		return "", "", false, err
	}
	if _, err := s.CreateUser(ctx, defaultAdminName, pass, true); err != nil {
		return "", "", false, err
	}
	return defaultAdminName, pass, true, nil
}

// randomPassword 生成 24 个字符的随机密码。
func randomPassword() (string, error) {
	b := make([]byte, 18) // base64 后正好 24 个字符
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("store: 生成随机密码失败: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

// isUniqueViolation 判断错误是否为唯一约束冲突。
func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}

// DeleteUser 删除用户。
//
// accounts.owner_id 是 ON DELETE RESTRICT，因此还拥有 B 站账号的用户
// 删不掉，会返回外键冲突——这是刻意的，避免留下无主的 Cookie。
func (s *Store) DeleteUser(ctx context.Context, id int64) (int64, error) {
	tag, err := s.pool.Exec(ctx, `DELETE FROM users WHERE id = $1`, id)
	if err != nil {
		return 0, fmt.Errorf("store: 删除用户失败: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return 0, fmt.Errorf("store: 用户不存在: %w", ErrNotFound)
	}
	return tag.RowsAffected(), nil
}

// IsForeignKeyViolation 判断错误是否为外键约束冲突。
func IsForeignKeyViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23503"
}
