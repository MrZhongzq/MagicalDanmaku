package store

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

// 账号参数的默认值。
const (
	defaultRateLimitMS = 1500 // B 站有 3 秒弹幕冷却，1.5 秒是发言与风控的折中
	defaultMaxLength   = 40   // B 站单条弹幕上限，汉字与英文都算 1 个字符
)

// 账号登录态的三个取值（对应迁移 003 里 login_state 列的 CHECK 约束）。
//
// 三态而非「有效布尔值 + 检测时间」：网络不通不等于账号掉线，
// 检测失败必须有独立于「登录已失效」的表示，否则一次网络抖动就会
// 在界面上显示成「账号已掉线」，把用户吓得去重新扫码。unknown 同时
// 表示「从未成功检测过」与「最近一次检测本身失败」——两者对用户
// 来说是同一种「不确定」，不需要再细分。
const (
	LoginStateUnknown = "unknown" // 尚未检测过，或最近一次检测失败（探测本身出错，而非确认未登录）
	LoginStateValid   = "valid"   // 最近一次检测确认登录有效
	LoginStateInvalid = "invalid" // 最近一次检测确认登录已失效（B 站 nav 接口返回 code=-101）
)

// Account 是机器人操作的 B 站账号。
//
// 与 User（使用本软件的人）是两回事：账号用 Cookie 操作直播间，
// 用户用密码登录管理界面。一个用户可以拥有多个账号。
//
// 账号不是可互换的资源，而是各有职责的参与者：主播号可能只做统计
// 与房管而不发言，小号负责欢迎答谢。因此不存在轮换或 fallback。
type Account struct {
	ID        int64
	Name      string
	UID       string
	Cookie    string        // 明文存储，见设计文档 §3.4
	RateLimit time.Duration // 该账号全部直播间共享的发送间隔
	MaxLength int
	OwnerID   int64
	CreatedAt time.Time
	UpdatedAt time.Time

	// LoginState 是最近一次登录态检测的结果，见 LoginStateValid 等常量。
	LoginState string
	// LoginCheckedAt 是最近一次尝试检测的时间（无论检测成功与否）；
	// 从未检测过为 nil。
	LoginCheckedAt *time.Time
}

// AccountInput 是创建或更新账号的入参。
type AccountInput struct {
	Name      string
	UID       string
	Cookie    string
	RateLimit time.Duration // 0 表示用默认值
	MaxLength int           // 0 表示用默认值
	OwnerID   int64
}

// accountColumns 是唯一列出 cookie 列的读取路径。
//
// Cookie 明文存储，读写各收在一处：读是这里加 scanAccount，
// 写是 encodeCookie。将来要加密只改这两处。
const accountColumns = `id, name, uid, cookie, rate_limit_ms, max_length, owner_id, created_at, updated_at, login_state, login_checked_at`

// encodeCookie 是 Cookie 写入数据库前的唯一通道。
//
// 今天它什么都不做——按既定的威胁模型，Cookie 明文存储（见设计文档 §3.4）。
// 它存在的意义是给将来的加密留一个收口：碰 cookie 列的 SQL 有三条
// （CreateAccount 的 INSERT、UpsertAccount 的 INSERT 与 ON CONFLICT、
// UpdateAccountCookie 的 UPDATE），但它们都只从这里取值，所以加密时
// 改这一个函数即可，不必去记「还有哪条 SQL 碰了 cookie」。
func encodeCookie(raw string) string {
	return raw
}

// scanAccount 是 accountColumns 对应的唯一扫描逻辑。
func scanAccount(row pgx.Row) (*Account, error) {
	var a Account
	var ms int
	if err := row.Scan(&a.ID, &a.Name, &a.UID, &a.Cookie, &ms,
		&a.MaxLength, &a.OwnerID, &a.CreatedAt, &a.UpdatedAt,
		&a.LoginState, &a.LoginCheckedAt); err != nil {
		return nil, err
	}
	a.RateLimit = time.Duration(ms) * time.Millisecond
	return &a, nil
}

// normalize 补默认值并做基本校验。
func (in *AccountInput) normalize() error {
	in.Name = strings.TrimSpace(in.Name)
	if in.Name == "" {
		return fmt.Errorf("store: 账号名不能为空")
	}
	if in.Cookie == "" {
		return fmt.Errorf("store: 账号 %q 的 Cookie 不能为空", in.Name)
	}
	if in.OwnerID == 0 {
		return fmt.Errorf("store: 账号 %q 必须指定所有者", in.Name)
	}
	return nil
}

// rateLimitMS 返回写库用的毫秒值，0 时用默认。
func (in AccountInput) rateLimitMS() int {
	if in.RateLimit <= 0 {
		return defaultRateLimitMS
	}
	return int(in.RateLimit / time.Millisecond)
}

// maxLength 返回写库用的字数上限，0 时用默认。
func (in AccountInput) maxLength() int {
	if in.MaxLength <= 0 {
		return defaultMaxLength
	}
	return in.MaxLength
}

// CreateAccount 创建账号。账号名已存在时返回 ErrDuplicate。
func (s *Store) CreateAccount(ctx context.Context, in AccountInput) (*Account, error) {
	if err := in.normalize(); err != nil {
		return nil, err
	}
	a, err := scanAccount(s.pool.QueryRow(ctx, `
		INSERT INTO accounts (name, uid, cookie, rate_limit_ms, max_length, owner_id)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING `+accountColumns,
		in.Name, in.UID, encodeCookie(in.Cookie), in.rateLimitMS(), in.maxLength(), in.OwnerID))
	if err != nil {
		if isUniqueViolation(err) {
			return nil, fmt.Errorf("store: 账号名 %q 已存在: %w", in.Name, ErrDuplicate)
		}
		return nil, fmt.Errorf("store: 创建账号失败: %w", err)
	}
	return a, nil
}

// UpsertAccount 按账号名创建或更新。
//
// import 要能反复跑：同一份 YAML 导两次，结果必须一致。
func (s *Store) UpsertAccount(ctx context.Context, in AccountInput) (*Account, error) {
	if err := in.normalize(); err != nil {
		return nil, err
	}
	a, err := scanAccount(s.pool.QueryRow(ctx, `
		INSERT INTO accounts (name, uid, cookie, rate_limit_ms, max_length, owner_id)
		VALUES ($1, $2, $3, $4, $5, $6)
		ON CONFLICT (name) DO UPDATE SET
			-- uid 用 CASE 而非直接覆盖：import 走的 YAML 里没有 uid 字段，
			-- 直接覆盖会把 login --save 扫码时写入的 UID 抹成空串，
			-- 而「先扫码入库、再导 YAML 补规则」正是文档推荐的流程。
			uid           = CASE WHEN EXCLUDED.uid = '' THEN accounts.uid ELSE EXCLUDED.uid END,
			cookie        = EXCLUDED.cookie,
			rate_limit_ms = EXCLUDED.rate_limit_ms,
			max_length    = EXCLUDED.max_length,
			owner_id      = EXCLUDED.owner_id,
			updated_at    = now()
		RETURNING `+accountColumns,
		in.Name, in.UID, encodeCookie(in.Cookie), in.rateLimitMS(), in.maxLength(), in.OwnerID))
	if err != nil {
		return nil, fmt.Errorf("store: 写入账号 %q 失败: %w", in.Name, err)
	}
	return a, nil
}

// GetAccountByName 按账号名查询。
func (s *Store) GetAccountByName(ctx context.Context, name string) (*Account, error) {
	a, err := scanAccount(s.pool.QueryRow(ctx,
		`SELECT `+accountColumns+` FROM accounts WHERE name = $1`, name))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, fmt.Errorf("store: 账号 %q 不存在: %w", name, ErrNotFound)
	}
	if err != nil {
		return nil, fmt.Errorf("store: 查询账号失败: %w", err)
	}
	return a, nil
}

// ListAccounts 按创建顺序列出全部账号。
func (s *Store) ListAccounts(ctx context.Context) ([]Account, error) {
	rows, err := s.pool.Query(ctx, `SELECT `+accountColumns+` FROM accounts ORDER BY id`)
	if err != nil {
		return nil, fmt.Errorf("store: 列出账号失败: %w", err)
	}
	defer rows.Close()

	var out []Account
	for rows.Next() {
		a, err := scanAccount(rows)
		if err != nil {
			return nil, fmt.Errorf("store: 读取账号失败: %w", err)
		}
		out = append(out, *a)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("store: 列出账号失败: %w", err)
	}
	return out, nil
}

// UpdateAccountCookie 换 Cookie。扫码重新登录后调用。
func (s *Store) UpdateAccountCookie(ctx context.Context, name, cookie, uid string) error {
	if cookie == "" {
		return fmt.Errorf("store: Cookie 不能为空")
	}
	tag, err := s.pool.Exec(ctx, `
		UPDATE accounts SET cookie = $1, uid = $2, updated_at = now() WHERE name = $3`,
		encodeCookie(cookie), uid, name)
	if err != nil {
		return fmt.Errorf("store: 更新账号 %q 的 Cookie 失败: %w", name, err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("store: 账号 %q 不存在: %w", name, ErrNotFound)
	}
	return nil
}

// UpdateAccountLoginState 写入一次登录态检测的结果。
//
// state 必须是 LoginStateValid/LoginStateInvalid/LoginStateUnknown 之一，
// 由调用方（cmd/magicd 的检测循环）判定后传入——探测失败时传
// LoginStateUnknown，绝不能把探测失败当作 LoginStateInvalid 写进来，
// 那等于把「网络不通」误报成「账号掉线」。
//
// 账号不存在时不算错误：检测循环遍历的是某一时刻的账号快照，
// 账号在检测期间被删掉是正常竞态，不该让整轮检测因此报错。
func (s *Store) UpdateAccountLoginState(ctx context.Context, name, state string) error {
	switch state {
	case LoginStateValid, LoginStateInvalid, LoginStateUnknown:
	default:
		return fmt.Errorf("store: 非法的登录态取值 %q", state)
	}
	if _, err := s.pool.Exec(ctx, `
		UPDATE accounts SET login_state = $1, login_checked_at = now() WHERE name = $2`,
		state, name); err != nil {
		return fmt.Errorf("store: 写入账号 %q 的登录态失败: %w", name, err)
	}
	return nil
}

// DeleteAccount 删除账号，连带其全部绑定、规则与业务日志。
func (s *Store) DeleteAccount(ctx context.Context, name string) error {
	tag, err := s.pool.Exec(ctx, `DELETE FROM accounts WHERE name = $1`, name)
	if err != nil {
		return fmt.Errorf("store: 删除账号 %q 失败: %w", name, err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("store: 账号 %q 不存在: %w", name, ErrNotFound)
	}
	return nil
}
