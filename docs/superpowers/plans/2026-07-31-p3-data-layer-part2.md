# P3 多租户数据层 实施计划 · Part 2（Task 5–10）

> 接 `2026-07-31-p3-data-layer.md`。Global Constraints 与文件结构见该文件，此处不重复。
>
> 本部分实现领域表的读写：账号、绑定与冷却组、规则、授权、脚本 KV 与禁言名单，最后拼出运行期配置的载入入口。

**前置：** Task 1–4 已完成。每个任务开始前确认本地数据库在跑：

```bash
docker compose -f docker-compose.dev.yml up -d; echo "退出码=$?"
export MAGICD_TEST_DATABASE_URL='postgres://magicd:magicd@localhost:5433/magicd?sslmode=disable'
```

---

## Task 5: 账号表

**Files:**
- Create: `server/internal/store/account.go`
- Create: `server/internal/store/account_test.go`

**Interfaces:**
- Consumes: `store.Store`、`store.ErrNotFound`、`store.ErrDuplicate`、`store.isUniqueViolation`、`store.CreateUser`（Task 3、4）
- Produces:
  - `type store.Account struct { ID int64; Name, UID, Cookie string; RateLimit time.Duration; MaxLength int; OwnerID int64; CreatedAt, UpdatedAt time.Time }`
  - `type store.AccountInput struct { Name, UID, Cookie string; RateLimit time.Duration; MaxLength int; OwnerID int64 }`
  - `func (s *Store) CreateAccount(ctx context.Context, in AccountInput) (*Account, error)`
  - `func (s *Store) UpsertAccount(ctx context.Context, in AccountInput) (*Account, error)`
  - `func (s *Store) GetAccountByName(ctx context.Context, name string) (*Account, error)`
  - `func (s *Store) ListAccounts(ctx context.Context) ([]Account, error)`
  - `func (s *Store) UpdateAccountCookie(ctx context.Context, name, cookie, uid string) error`
  - `func (s *Store) DeleteAccount(ctx context.Context, name string) error`

- [ ] **Step 1: 写失败的测试**

创建 `server/internal/store/account_test.go`：

```go
package store

import (
	"context"
	"errors"
	"testing"
	"time"
)

// mustUser 建一个用户并返回 ID，账号需要 owner。
func mustUser(t *testing.T, s *Store, name string) int64 {
	t.Helper()
	u, err := s.CreateUser(context.Background(), name, "pw", false)
	if err != nil {
		t.Fatalf("创建用户 %s 报错: %v", name, err)
	}
	return u.ID
}

func TestCreateAndGetAccount(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	owner := mustUser(t, s, "张三")

	created, err := s.CreateAccount(ctx, AccountInput{
		Name:      "主播号",
		UID:       "12345",
		Cookie:    "SESSDATA=abc; bili_jct=def",
		RateLimit: 1500 * time.Millisecond,
		MaxLength: 40,
		OwnerID:   owner,
	})
	if err != nil {
		t.Fatalf("创建账号报错: %v", err)
	}
	if created.ID == 0 {
		t.Error("新账号应有非零 ID")
	}

	got, err := s.GetAccountByName(ctx, "主播号")
	if err != nil {
		t.Fatalf("查询账号报错: %v", err)
	}
	if got.Cookie != "SESSDATA=abc; bili_jct=def" {
		t.Errorf("Cookie = %q", got.Cookie)
	}
	if got.RateLimit != 1500*time.Millisecond {
		t.Errorf("RateLimit = %v, 期望 1.5s", got.RateLimit)
	}
	if got.UID != "12345" || got.MaxLength != 40 || got.OwnerID != owner {
		t.Errorf("账号 = %+v", got)
	}
}

func TestCreateAccountAppliesDefaults(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	owner := mustUser(t, s, "张三")

	a, err := s.CreateAccount(ctx, AccountInput{
		Name: "小号", Cookie: "c", OwnerID: owner,
	})
	if err != nil {
		t.Fatalf("创建账号报错: %v", err)
	}
	if a.RateLimit != 1500*time.Millisecond {
		t.Errorf("未指定时 RateLimit = %v, 期望默认 1.5s", a.RateLimit)
	}
	if a.MaxLength != 40 {
		t.Errorf("未指定时 MaxLength = %d, 期望默认 40（B 站上限）", a.MaxLength)
	}
}

func TestGetAccountNotFound(t *testing.T) {
	s := testStore(t)
	_, err := s.GetAccountByName(context.Background(), "没这个号")
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("应返回 ErrNotFound，实际: %v", err)
	}
}

func TestCreateAccountRejectsDuplicateName(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	owner := mustUser(t, s, "张三")

	in := AccountInput{Name: "主播号", Cookie: "c", OwnerID: owner}
	if _, err := s.CreateAccount(ctx, in); err != nil {
		t.Fatalf("首次创建报错: %v", err)
	}
	if _, err := s.CreateAccount(ctx, in); !errors.Is(err, ErrDuplicate) {
		t.Errorf("重名应返回 ErrDuplicate，实际: %v", err)
	}
}

func TestCreateAccountRejectsEmptyCookie(t *testing.T) {
	s := testStore(t)
	owner := mustUser(t, s, "张三")
	_, err := s.CreateAccount(context.Background(), AccountInput{
		Name: "主播号", OwnerID: owner,
	})
	if err == nil {
		t.Error("空 Cookie 应被拒绝：没有 Cookie 的账号什么都干不了")
	}
}

func TestUpsertAccountUpdatesInsteadOfFailing(t *testing.T) {
	// import 要能反复跑，同一份 YAML 导两次结果必须一致
	s := testStore(t)
	ctx := context.Background()
	owner := mustUser(t, s, "张三")

	first, err := s.UpsertAccount(ctx, AccountInput{
		Name: "主播号", Cookie: "old", RateLimit: time.Second, OwnerID: owner,
	})
	if err != nil {
		t.Fatalf("首次 upsert 报错: %v", err)
	}
	second, err := s.UpsertAccount(ctx, AccountInput{
		Name: "主播号", Cookie: "new", RateLimit: 2 * time.Second, OwnerID: owner,
	})
	if err != nil {
		t.Fatalf("二次 upsert 报错: %v", err)
	}

	if first.ID != second.ID {
		t.Errorf("upsert 应更新同一行，ID 从 %d 变成了 %d", first.ID, second.ID)
	}
	if second.Cookie != "new" || second.RateLimit != 2*time.Second {
		t.Errorf("upsert 后 = %+v", second)
	}

	all, err := s.ListAccounts(ctx)
	if err != nil {
		t.Fatalf("列出账号报错: %v", err)
	}
	if len(all) != 1 {
		t.Errorf("账号数 = %d, 期望 1", len(all))
	}
}

func TestUpdateAccountCookie(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	owner := mustUser(t, s, "张三")

	if _, err := s.CreateAccount(ctx, AccountInput{
		Name: "主播号", Cookie: "old", UID: "1", OwnerID: owner,
	}); err != nil {
		t.Fatalf("创建账号报错: %v", err)
	}

	if err := s.UpdateAccountCookie(ctx, "主播号", "fresh", "999"); err != nil {
		t.Fatalf("更新 Cookie 报错: %v", err)
	}
	got, err := s.GetAccountByName(ctx, "主播号")
	if err != nil {
		t.Fatalf("查询账号报错: %v", err)
	}
	if got.Cookie != "fresh" || got.UID != "999" {
		t.Errorf("更新后 = %+v", got)
	}
}

func TestUpdateAccountCookieOnMissingAccount(t *testing.T) {
	s := testStore(t)
	err := s.UpdateAccountCookie(context.Background(), "没这个号", "c", "1")
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("应返回 ErrNotFound，实际: %v", err)
	}
}

func TestListAccountsOrderedByID(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	owner := mustUser(t, s, "张三")

	for _, n := range []string{"主播号", "小号", "大号"} {
		if _, err := s.CreateAccount(ctx, AccountInput{
			Name: n, Cookie: "c", OwnerID: owner,
		}); err != nil {
			t.Fatalf("创建 %s 报错: %v", n, err)
		}
	}
	as, err := s.ListAccounts(ctx)
	if err != nil {
		t.Fatalf("列出账号报错: %v", err)
	}
	if len(as) != 3 || as[0].Name != "主播号" || as[2].Name != "大号" {
		t.Errorf("列表 = %+v", as)
	}
}

func TestDeleteAccount(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	owner := mustUser(t, s, "张三")

	if _, err := s.CreateAccount(ctx, AccountInput{
		Name: "主播号", Cookie: "c", OwnerID: owner,
	}); err != nil {
		t.Fatalf("创建账号报错: %v", err)
	}
	if err := s.DeleteAccount(ctx, "主播号"); err != nil {
		t.Fatalf("删除账号报错: %v", err)
	}
	if _, err := s.GetAccountByName(ctx, "主播号"); !errors.Is(err, ErrNotFound) {
		t.Errorf("删除后应查不到，实际: %v", err)
	}
}

func TestDeleteMissingAccount(t *testing.T) {
	s := testStore(t)
	if err := s.DeleteAccount(context.Background(), "没这个号"); !errors.Is(err, ErrNotFound) {
		t.Errorf("应返回 ErrNotFound，实际: %v", err)
	}
}
```

- [ ] **Step 2: 跑测试确认失败**

```bash
cd server; go test ./internal/store/ -run TestCreateAndGetAccount 2>&1 | tail -5; echo "退出码=$?"
```

预期：编译失败（`AccountInput` 未定义）。

- [ ] **Step 3: 实现**

创建 `server/internal/store/account.go`：

```go
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

// accountColumns 是唯一列出 cookie 列的地方。
//
// Cookie 明文存储，读取路径必须收在一处：将来要加密时，
// 改这一处的 SELECT 与 scanAccount 就够了。
const accountColumns = `id, name, uid, cookie, rate_limit_ms, max_length, owner_id, created_at, updated_at`

// scanAccount 是 accountColumns 对应的唯一扫描逻辑。
func scanAccount(row pgx.Row) (*Account, error) {
	var a Account
	var ms int
	if err := row.Scan(&a.ID, &a.Name, &a.UID, &a.Cookie, &ms,
		&a.MaxLength, &a.OwnerID, &a.CreatedAt, &a.UpdatedAt); err != nil {
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
		in.Name, in.UID, in.Cookie, in.rateLimitMS(), in.maxLength(), in.OwnerID))
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
			uid           = EXCLUDED.uid,
			cookie        = EXCLUDED.cookie,
			rate_limit_ms = EXCLUDED.rate_limit_ms,
			max_length    = EXCLUDED.max_length,
			owner_id      = EXCLUDED.owner_id,
			updated_at    = now()
		RETURNING `+accountColumns,
		in.Name, in.UID, in.Cookie, in.rateLimitMS(), in.maxLength(), in.OwnerID))
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
		cookie, uid, name)
	if err != nil {
		return fmt.Errorf("store: 更新账号 %q 的 Cookie 失败: %w", name, err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("store: 账号 %q 不存在: %w", name, ErrNotFound)
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
```

- [ ] **Step 4: 跑测试确认通过**

```bash
cd server; go test ./internal/store/ -run 'Account' -v 2>&1 | tail -30; echo "退出码=$?"
```

预期：全部 PASS。

- [ ] **Step 5: 提交**

```bash
cd server; gofmt -l . ; go vet ./internal/store/; echo "退出码=$?"
git add server/internal/store/
git commit -m "$(cat <<'EOF'
feat: 新增账号表读写

Cookie 明文存储，读取路径收在 accountColumns 与 scanAccount 两处，
将来要加密只改这里。

UpsertAccount 让 import 可以反复跑：同一份 YAML 导两次结果一致。

Co-Authored-By: Claude Opus 5 <noreply@anthropic.com>
EOF
)"
```

---

## Task 6: 绑定与冷却组

**Files:**
- Create: `server/internal/store/binding.go`
- Create: `server/internal/store/binding_test.go`

**Interfaces:**
- Consumes: `store.Account`、`store.GetAccountByName`（Task 5）
- Produces:
  - `type store.Binding struct { ID, AccountID int64; AccountName, RoomID string; Enabled bool; CreatedAt, UpdatedAt time.Time }`
  - `func (b Binding) Label() string` —— 形如 `"小号@1706666491"`
  - `func (s *Store) UpsertBinding(ctx context.Context, accountID int64, roomID string) (*Binding, error)`
  - `func (s *Store) GetBinding(ctx context.Context, accountName, roomID string) (*Binding, error)`
  - `func (s *Store) ListBindings(ctx context.Context) ([]Binding, error)`
  - `func (s *Store) SetBindingEnabled(ctx context.Context, accountName, roomID string, enabled bool) error`
  - `func (s *Store) DeleteBinding(ctx context.Context, accountName, roomID string) error`
  - `func (s *Store) SetCooldownGroups(ctx context.Context, bindingID int64, groups map[string]time.Duration) error`
  - `func (s *Store) CooldownGroups(ctx context.Context, bindingID int64) (map[string]time.Duration, error)`

- [ ] **Step 1: 写失败的测试**

创建 `server/internal/store/binding_test.go`：

```go
package store

import (
	"context"
	"errors"
	"testing"
	"time"
)

// mustAccount 建一个账号并返回 ID。
func mustAccount(t *testing.T, s *Store, name string) int64 {
	t.Helper()
	owner := mustUser(t, s, "owner_"+name)
	a, err := s.CreateAccount(context.Background(), AccountInput{
		Name: name, Cookie: "c", OwnerID: owner,
	})
	if err != nil {
		t.Fatalf("创建账号 %s 报错: %v", name, err)
	}
	return a.ID
}

func TestUpsertAndGetBinding(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	accID := mustAccount(t, s, "小号")

	b, err := s.UpsertBinding(ctx, accID, "1706666491")
	if err != nil {
		t.Fatalf("创建绑定报错: %v", err)
	}
	if b.ID == 0 || !b.Enabled {
		t.Errorf("新绑定 = %+v, 应有非零 ID 且默认启用", b)
	}

	got, err := s.GetBinding(ctx, "小号", "1706666491")
	if err != nil {
		t.Fatalf("查询绑定报错: %v", err)
	}
	if got.ID != b.ID || got.AccountName != "小号" || got.RoomID != "1706666491" {
		t.Errorf("绑定 = %+v", got)
	}
}

func TestBindingLabel(t *testing.T) {
	b := Binding{AccountName: "小号", RoomID: "1706666491"}
	if b.Label() != "小号@1706666491" {
		t.Errorf("Label() = %q, 期望 \"小号@1706666491\"", b.Label())
	}
}

func TestUpsertBindingIsIdempotent(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	accID := mustAccount(t, s, "小号")

	first, err := s.UpsertBinding(ctx, accID, "123")
	if err != nil {
		t.Fatalf("首次报错: %v", err)
	}
	second, err := s.UpsertBinding(ctx, accID, "123")
	if err != nil {
		t.Fatalf("二次报错: %v", err)
	}
	if first.ID != second.ID {
		t.Errorf("同一账号同一房间应是同一行，ID 从 %d 变成 %d", first.ID, second.ID)
	}
}

// 同一直播间被两个账号连接时是两条独立绑定，各自有独立的规则与冷却状态
func TestSameRoomTwoAccountsAreSeparateBindings(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	main := mustAccount(t, s, "主播号")
	sub := mustAccount(t, s, "小号")

	b1, err := s.UpsertBinding(ctx, main, "1706666491")
	if err != nil {
		t.Fatalf("主播号绑定报错: %v", err)
	}
	b2, err := s.UpsertBinding(ctx, sub, "1706666491")
	if err != nil {
		t.Fatalf("小号绑定报错: %v", err)
	}
	if b1.ID == b2.ID {
		t.Error("两个账号连同一房间应是两条独立绑定")
	}
}

func TestGetBindingNotFound(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	mustAccount(t, s, "小号")

	if _, err := s.GetBinding(ctx, "小号", "不存在的房间"); !errors.Is(err, ErrNotFound) {
		t.Errorf("房间不存在应返回 ErrNotFound，实际: %v", err)
	}
	if _, err := s.GetBinding(ctx, "没这个号", "123"); !errors.Is(err, ErrNotFound) {
		t.Errorf("账号不存在应返回 ErrNotFound，实际: %v", err)
	}
}

func TestSetBindingEnabled(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	accID := mustAccount(t, s, "小号")

	if _, err := s.UpsertBinding(ctx, accID, "123"); err != nil {
		t.Fatalf("创建绑定报错: %v", err)
	}
	if err := s.SetBindingEnabled(ctx, "小号", "123", false); err != nil {
		t.Fatalf("停用绑定报错: %v", err)
	}
	got, err := s.GetBinding(ctx, "小号", "123")
	if err != nil {
		t.Fatalf("查询绑定报错: %v", err)
	}
	if got.Enabled {
		t.Error("停用后 Enabled 应为 false")
	}
}

func TestListBindingsIncludesAccountName(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	main := mustAccount(t, s, "主播号")
	sub := mustAccount(t, s, "小号")

	if _, err := s.UpsertBinding(ctx, main, "111"); err != nil {
		t.Fatalf("创建绑定报错: %v", err)
	}
	if _, err := s.UpsertBinding(ctx, sub, "222"); err != nil {
		t.Fatalf("创建绑定报错: %v", err)
	}

	bs, err := s.ListBindings(ctx)
	if err != nil {
		t.Fatalf("列出绑定报错: %v", err)
	}
	if len(bs) != 2 {
		t.Fatalf("绑定数 = %d, 期望 2", len(bs))
	}
	if bs[0].AccountName != "主播号" || bs[1].AccountName != "小号" {
		t.Errorf("账号名未带出: %+v", bs)
	}
}

func TestDeleteBinding(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	accID := mustAccount(t, s, "小号")

	if _, err := s.UpsertBinding(ctx, accID, "123"); err != nil {
		t.Fatalf("创建绑定报错: %v", err)
	}
	if err := s.DeleteBinding(ctx, "小号", "123"); err != nil {
		t.Fatalf("删除绑定报错: %v", err)
	}
	if _, err := s.GetBinding(ctx, "小号", "123"); !errors.Is(err, ErrNotFound) {
		t.Errorf("删除后应查不到，实际: %v", err)
	}
}

func TestSetAndGetCooldownGroups(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	accID := mustAccount(t, s, "小号")
	b, err := s.UpsertBinding(ctx, accID, "123")
	if err != nil {
		t.Fatalf("创建绑定报错: %v", err)
	}

	want := map[string]time.Duration{
		"greeting": 5 * time.Second,
		"thanks":   2 * time.Second,
	}
	if err := s.SetCooldownGroups(ctx, b.ID, want); err != nil {
		t.Fatalf("写入冷却组报错: %v", err)
	}

	got, err := s.CooldownGroups(ctx, b.ID)
	if err != nil {
		t.Fatalf("读取冷却组报错: %v", err)
	}
	if len(got) != 2 || got["greeting"] != 5*time.Second || got["thanks"] != 2*time.Second {
		t.Errorf("冷却组 = %v, 期望 %v", got, want)
	}
}

func TestSetCooldownGroupsReplacesInsteadOfMerging(t *testing.T) {
	// 从界面上删掉一个冷却组，就该真的消失，而不是残留
	s := testStore(t)
	ctx := context.Background()
	accID := mustAccount(t, s, "小号")
	b, err := s.UpsertBinding(ctx, accID, "123")
	if err != nil {
		t.Fatalf("创建绑定报错: %v", err)
	}

	if err := s.SetCooldownGroups(ctx, b.ID, map[string]time.Duration{
		"greeting": 5 * time.Second, "thanks": 2 * time.Second,
	}); err != nil {
		t.Fatalf("首次写入报错: %v", err)
	}
	if err := s.SetCooldownGroups(ctx, b.ID, map[string]time.Duration{
		"greeting": time.Second,
	}); err != nil {
		t.Fatalf("二次写入报错: %v", err)
	}

	got, err := s.CooldownGroups(ctx, b.ID)
	if err != nil {
		t.Fatalf("读取冷却组报错: %v", err)
	}
	if len(got) != 1 {
		t.Errorf("冷却组应被整组替换，实际 = %v", got)
	}
	if got["greeting"] != time.Second {
		t.Errorf("greeting = %v, 期望 1s", got["greeting"])
	}
}

func TestCooldownGroupsEmptyReturnsEmptyMap(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	accID := mustAccount(t, s, "小号")
	b, err := s.UpsertBinding(ctx, accID, "123")
	if err != nil {
		t.Fatalf("创建绑定报错: %v", err)
	}

	got, err := s.CooldownGroups(ctx, b.ID)
	if err != nil {
		t.Fatalf("读取冷却组报错: %v", err)
	}
	if got == nil {
		t.Error("未配置时应返回空 map 而非 nil，调用方不该被迫判空")
	}
	if len(got) != 0 {
		t.Errorf("未配置时应为空，实际 = %v", got)
	}
}
```

- [ ] **Step 2: 跑测试确认失败**

```bash
cd server; go test ./internal/store/ -run TestUpsertAndGetBinding 2>&1 | tail -5; echo "退出码=$?"
```

预期：编译失败。

- [ ] **Step 3: 实现**

创建 `server/internal/store/binding.go`：

```go
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
```

- [ ] **Step 4: 跑测试确认通过**

```bash
cd server; go test ./internal/store/ -run 'Binding|CooldownGroups' -v 2>&1 | tail -30; echo "退出码=$?"
```

- [ ] **Step 5: 提交**

```bash
cd server; gofmt -l . ; go vet ./internal/store/; echo "退出码=$?"
git add server/internal/store/
git commit -m "$(cat <<'EOF'
feat: 新增绑定与冷却组读写

UpsertBinding 幂等且不改动 enabled——重新导入配置不该把用户手动
停用的绑定又打开。

SetCooldownGroups 整组替换而非合并：从界面上删掉一个冷却组，
就该真的消失。

Co-Authored-By: Claude Opus 5 <noreply@anthropic.com>
EOF
)"
```

---

## Task 7: 规则表

规则体以 JSONB 存，但 `name` 与 `enabled` 提到列上。**同一个值绝不能同时存在两处**——JSONB 里必须不含这两个字段，否则界面改了列、JSONB 没跟着改，两边就开始漂移。拆装收在本文件的 `splitRule` 与 `assembleRule` 两个函数里。

**Files:**
- Create: `server/internal/store/rule.go`
- Create: `server/internal/store/rule_test.go`

**Interfaces:**
- Consumes: `store.Binding`（Task 6）、`spec.Rule`、`spec.Rule.ToRule()`（Task 2）
- Produces:
  - `type store.RuleRecord struct { ID, BindingID int64; Name string; Enabled bool; Position int; Spec spec.Rule }`
  - `func (r RuleRecord) Domain() (rules.Rule, error)`
  - `func (s *Store) SaveRule(ctx context.Context, bindingID int64, position int, r spec.Rule) (*RuleRecord, error)`
  - `func (s *Store) ListRules(ctx context.Context, bindingID int64) ([]RuleRecord, error)`
  - `func (s *Store) GetRule(ctx context.Context, bindingID int64, name string) (*RuleRecord, error)`
  - `func (s *Store) SetRuleEnabled(ctx context.Context, bindingID int64, name string, enabled bool) error`
  - `func (s *Store) DeleteRule(ctx context.Context, bindingID int64, name string) error`
  - `func (s *Store) ReplaceRules(ctx context.Context, bindingID int64, rs []spec.Rule) error`

- [ ] **Step 1: 写失败的测试**

创建 `server/internal/store/rule_test.go`：

```go
package store

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/MrZhongzq/MagicalDanmaku/server/internal/rules"
	"github.com/MrZhongzq/MagicalDanmaku/server/internal/rules/spec"
)

// mustBinding 建一个账号加绑定，返回绑定 ID。
func mustBinding(t *testing.T, s *Store, accountName, roomID string) int64 {
	t.Helper()
	accID := mustAccount(t, s, accountName)
	b, err := s.UpsertBinding(context.Background(), accID, roomID)
	if err != nil {
		t.Fatalf("创建绑定报错: %v", err)
	}
	return b.ID
}

// sampleRule 是测试用的一条完整规则。
func sampleRule() spec.Rule {
	return spec.Rule{
		Name: "舰长进场欢迎",
		On:   []string{"user_enter"},
		When: &spec.Condition{Field: "user.guardLevel", Op: ">", Value: 0},
		Aggregate: &spec.Aggregate{
			Window: spec.Duration(3 * time.Minute), MaxWait: spec.Duration(5 * time.Minute),
			MinCount: 4, By: "type",
		},
		CooldownGroup: "greeting",
		Do:            []spec.Action{{Type: "danmaku", Template: []string{"欢迎回家~"}}},
	}
}

func TestSaveAndGetRule(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	bid := mustBinding(t, s, "小号", "123")

	saved, err := s.SaveRule(ctx, bid, 0, sampleRule())
	if err != nil {
		t.Fatalf("保存规则报错: %v", err)
	}
	if saved.ID == 0 || saved.Name != "舰长进场欢迎" || !saved.Enabled {
		t.Errorf("保存结果 = %+v", saved)
	}

	got, err := s.GetRule(ctx, bid, "舰长进场欢迎")
	if err != nil {
		t.Fatalf("查询规则报错: %v", err)
	}
	if got.Spec.CooldownGroup != "greeting" {
		t.Errorf("CooldownGroup = %q", got.Spec.CooldownGroup)
	}
	if got.Spec.Aggregate == nil || time.Duration(got.Spec.Aggregate.Window) != 3*time.Minute {
		t.Errorf("Aggregate = %+v", got.Spec.Aggregate)
	}
}

// name 与 enabled 是列，JSONB 里必须没有——同一个值存两处必然漂移
func TestRuleJSONBExcludesNameAndEnabled(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	bid := mustBinding(t, s, "小号", "123")

	if _, err := s.SaveRule(ctx, bid, 0, sampleRule()); err != nil {
		t.Fatalf("保存规则报错: %v", err)
	}

	var raw []byte
	if err := s.pool.QueryRow(ctx,
		`SELECT spec FROM rules WHERE binding_id = $1`, bid).Scan(&raw); err != nil {
		t.Fatalf("读取 JSONB 报错: %v", err)
	}

	var m map[string]any
	if err := json.Unmarshal(raw, &m); err != nil {
		t.Fatalf("解析 JSONB 报错: %v", err)
	}
	if _, ok := m["name"]; ok {
		t.Error("JSONB 里不该有 name，它是列")
	}
	if _, ok := m["enabled"]; ok {
		t.Error("JSONB 里不该有 enabled，它是列")
	}
	if _, ok := m["on"]; !ok {
		t.Error("JSONB 里应有 on")
	}
}

// 从列读回来的 name/enabled 必须填进 Spec，调用方拿到的是完整规则
func TestGetRuleAssemblesNameAndEnabledFromColumns(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	bid := mustBinding(t, s, "小号", "123")

	if _, err := s.SaveRule(ctx, bid, 0, sampleRule()); err != nil {
		t.Fatalf("保存规则报错: %v", err)
	}
	if err := s.SetRuleEnabled(ctx, bid, "舰长进场欢迎", false); err != nil {
		t.Fatalf("停用规则报错: %v", err)
	}

	got, err := s.GetRule(ctx, bid, "舰长进场欢迎")
	if err != nil {
		t.Fatalf("查询规则报错: %v", err)
	}
	if got.Spec.Name != "舰长进场欢迎" {
		t.Errorf("Spec.Name = %q, 应从列填回", got.Spec.Name)
	}
	if got.Spec.Enabled == nil || *got.Spec.Enabled {
		t.Error("Spec.Enabled 应从列填回 false")
	}

	d, err := got.Domain()
	if err != nil {
		t.Fatalf("转领域模型报错: %v", err)
	}
	if d.Enabled {
		t.Error("领域模型的 Enabled 应为 false")
	}
}

func TestSaveRuleRejectsInvalidRule(t *testing.T) {
	// 非法规则不许进库：写进去了，run 每次启动都会炸
	s := testStore(t)
	ctx := context.Background()
	bid := mustBinding(t, s, "小号", "123")

	_, err := s.SaveRule(ctx, bid, 0, spec.Rule{
		Name: "坏规则",
		On:   []string{"没有这种事件"},
		Do:   []spec.Action{{Type: "log"}},
	})
	if err == nil {
		t.Error("未知事件类型的规则应被拒绝")
	}

	_, err = s.SaveRule(ctx, bid, 0, spec.Rule{Name: "空动作", On: []string{"danmaku"}})
	if err == nil {
		t.Error("空动作列表的规则应被拒绝")
	}
}

func TestSaveRuleUpdatesExistingByName(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	bid := mustBinding(t, s, "小号", "123")

	first, err := s.SaveRule(ctx, bid, 0, sampleRule())
	if err != nil {
		t.Fatalf("首次保存报错: %v", err)
	}

	r := sampleRule()
	r.CooldownGroup = "changed"
	second, err := s.SaveRule(ctx, bid, 0, r)
	if err != nil {
		t.Fatalf("二次保存报错: %v", err)
	}

	if first.ID != second.ID {
		t.Errorf("同名规则应更新同一行，ID 从 %d 变成 %d", first.ID, second.ID)
	}
	if second.Spec.CooldownGroup != "changed" {
		t.Errorf("CooldownGroup = %q, 期望 changed", second.Spec.CooldownGroup)
	}
}

// 规则名只需在单个绑定内唯一——同一条「进场欢迎」本来就会出现在多个绑定下
func TestSameRuleNameInDifferentBindings(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	b1 := mustBinding(t, s, "小号", "111")
	b2 := mustBinding(t, s, "大号", "222")

	if _, err := s.SaveRule(ctx, b1, 0, sampleRule()); err != nil {
		t.Fatalf("绑定一保存报错: %v", err)
	}
	if _, err := s.SaveRule(ctx, b2, 0, sampleRule()); err != nil {
		t.Fatalf("绑定二保存报错: %v", err)
	}

	r1, err := s.ListRules(ctx, b1)
	if err != nil {
		t.Fatalf("列出绑定一的规则报错: %v", err)
	}
	r2, err := s.ListRules(ctx, b2)
	if err != nil {
		t.Fatalf("列出绑定二的规则报错: %v", err)
	}
	if len(r1) != 1 || len(r2) != 1 {
		t.Errorf("两个绑定各应有 1 条规则，实际 %d 与 %d", len(r1), len(r2))
	}
}

func TestListRulesOrderedByPosition(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	bid := mustBinding(t, s, "小号", "123")

	for i, name := range []string{"第三条", "第一条", "第二条"} {
		pos := map[string]int{"第一条": 0, "第二条": 1, "第三条": 2}[name]
		r := spec.Rule{Name: name, On: []string{"danmaku"}, Do: []spec.Action{{Type: "log"}}}
		if _, err := s.SaveRule(ctx, bid, pos, r); err != nil {
			t.Fatalf("保存第 %d 条报错: %v", i, err)
		}
	}

	rs, err := s.ListRules(ctx, bid)
	if err != nil {
		t.Fatalf("列出规则报错: %v", err)
	}
	if len(rs) != 3 {
		t.Fatalf("规则数 = %d, 期望 3", len(rs))
	}
	want := []string{"第一条", "第二条", "第三条"}
	for i, w := range want {
		if rs[i].Name != w {
			t.Errorf("第 %d 条 = %q, 期望 %q", i, rs[i].Name, w)
		}
	}
}

func TestGetRuleNotFound(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	bid := mustBinding(t, s, "小号", "123")

	if _, err := s.GetRule(ctx, bid, "没这条规则"); !errors.Is(err, ErrNotFound) {
		t.Errorf("应返回 ErrNotFound，实际: %v", err)
	}
}

func TestDeleteRule(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	bid := mustBinding(t, s, "小号", "123")

	if _, err := s.SaveRule(ctx, bid, 0, sampleRule()); err != nil {
		t.Fatalf("保存规则报错: %v", err)
	}
	if err := s.DeleteRule(ctx, bid, "舰长进场欢迎"); err != nil {
		t.Fatalf("删除规则报错: %v", err)
	}
	if _, err := s.GetRule(ctx, bid, "舰长进场欢迎"); !errors.Is(err, ErrNotFound) {
		t.Errorf("删除后应查不到，实际: %v", err)
	}
}

func TestReplaceRulesDropsMissingOnes(t *testing.T) {
	// import 用：YAML 里删掉的规则，重新导入后库里也该没有
	s := testStore(t)
	ctx := context.Background()
	bid := mustBinding(t, s, "小号", "123")

	a := spec.Rule{Name: "甲", On: []string{"danmaku"}, Do: []spec.Action{{Type: "log"}}}
	b := spec.Rule{Name: "乙", On: []string{"gift"}, Do: []spec.Action{{Type: "log"}}}
	if err := s.ReplaceRules(ctx, bid, []spec.Rule{a, b}); err != nil {
		t.Fatalf("首次替换报错: %v", err)
	}
	if err := s.ReplaceRules(ctx, bid, []spec.Rule{a}); err != nil {
		t.Fatalf("二次替换报错: %v", err)
	}

	rs, err := s.ListRules(ctx, bid)
	if err != nil {
		t.Fatalf("列出规则报错: %v", err)
	}
	if len(rs) != 1 || rs[0].Name != "甲" {
		t.Errorf("替换后 = %+v, 期望只剩「甲」", rs)
	}
}

func TestReplaceRulesRejectsDuplicateNames(t *testing.T) {
	// 冷却按规则名记录，同绑定内重名会互相干扰
	s := testStore(t)
	ctx := context.Background()
	bid := mustBinding(t, s, "小号", "123")

	r := spec.Rule{Name: "甲", On: []string{"danmaku"}, Do: []spec.Action{{Type: "log"}}}
	if err := s.ReplaceRules(ctx, bid, []spec.Rule{r, r}); err == nil {
		t.Error("同绑定内重名应被拒绝")
	}
}

func TestReplaceRulesIsAtomic(t *testing.T) {
	// 中途有一条非法，整批都不该落库，否则会留下半套规则
	s := testStore(t)
	ctx := context.Background()
	bid := mustBinding(t, s, "小号", "123")

	good := spec.Rule{Name: "好的", On: []string{"danmaku"}, Do: []spec.Action{{Type: "log"}}}
	bad := spec.Rule{Name: "坏的", On: []string{"没有这种事件"}, Do: []spec.Action{{Type: "log"}}}

	if err := s.ReplaceRules(ctx, bid, []spec.Rule{good, bad}); err == nil {
		t.Fatal("含非法规则的批次应整体失败")
	}
	rs, err := s.ListRules(ctx, bid)
	if err != nil {
		t.Fatalf("列出规则报错: %v", err)
	}
	if len(rs) != 0 {
		t.Errorf("失败的批次不该留下任何规则，实际 %+v", rs)
	}
}

func TestRuleRecordDomainConversion(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	bid := mustBinding(t, s, "小号", "123")

	if _, err := s.SaveRule(ctx, bid, 0, sampleRule()); err != nil {
		t.Fatalf("保存规则报错: %v", err)
	}
	rec, err := s.GetRule(ctx, bid, "舰长进场欢迎")
	if err != nil {
		t.Fatalf("查询规则报错: %v", err)
	}

	d, err := rec.Domain()
	if err != nil {
		t.Fatalf("转领域模型报错: %v", err)
	}
	if d.When == nil || d.When.Op != "gt" {
		t.Errorf("操作符别名应已归一化，实际 %+v", d.When)
	}
	if d.Aggregate == nil || d.Aggregate.By != rules.AggregateByType {
		t.Errorf("Aggregate = %+v", d.Aggregate)
	}
	if d.Aggregate.MinCount != 4 {
		t.Errorf("MinCount = %d, 期望 4", d.Aggregate.MinCount)
	}
}
```

- [ ] **Step 2: 跑测试确认失败**

```bash
cd server; go test ./internal/store/ -run TestSaveAndGetRule 2>&1 | tail -5; echo "退出码=$?"
```

- [ ] **Step 3: 实现**

创建 `server/internal/store/rule.go`：

```go
package store

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"

	"github.com/MrZhongzq/MagicalDanmaku/server/internal/rules"
	"github.com/MrZhongzq/MagicalDanmaku/server/internal/rules/spec"
)

// RuleRecord 是一条存好的规则。
//
// Name 与 Enabled 既是列也出现在 Spec 里，但库里的 JSONB **不含**这两个
// 字段——同一个值存两处必然漂移。拆装只发生在 splitRule 与 assembleRule。
type RuleRecord struct {
	ID        int64
	BindingID int64
	Name      string
	Enabled   bool
	Position  int
	Spec      spec.Rule
}

// Domain 转成规则引擎用的领域模型，顺带完成校验。
func (r RuleRecord) Domain() (rules.Rule, error) {
	return r.Spec.ToRule()
}

// splitRule 把规则拆成「列」与「JSONB 主体」。
//
// 主体里清掉 Name 与 Enabled，因为它们已经是列。
func splitRule(r spec.Rule) (name string, enabled bool, body []byte, err error) {
	name = r.Name
	enabled = true
	if r.Enabled != nil {
		enabled = *r.Enabled
	}

	// 转换一次，非法规则不许进库——写进去了，run 每次启动都会炸
	if _, err := r.ToRule(); err != nil {
		return "", false, nil, fmt.Errorf("store: 规则 %q 非法: %w", name, err)
	}

	r.Name = ""
	r.Enabled = nil
	body, err = json.Marshal(r)
	if err != nil {
		return "", false, nil, fmt.Errorf("store: 序列化规则 %q 失败: %w", name, err)
	}
	return name, enabled, body, nil
}

// assembleRule 把列与 JSONB 主体拼回完整的 spec.Rule。
func assembleRule(name string, enabled bool, body []byte) (spec.Rule, error) {
	var r spec.Rule
	if err := json.Unmarshal(body, &r); err != nil {
		return r, fmt.Errorf("store: 解析规则 %q 失败: %w", name, err)
	}
	r.Name = name
	r.Enabled = &enabled
	return r, nil
}

func scanRule(row pgx.Row) (*RuleRecord, error) {
	var rec RuleRecord
	var body []byte
	if err := row.Scan(&rec.ID, &rec.BindingID, &rec.Name, &rec.Enabled,
		&rec.Position, &body); err != nil {
		return nil, err
	}
	s, err := assembleRule(rec.Name, rec.Enabled, body)
	if err != nil {
		return nil, err
	}
	rec.Spec = s
	return &rec, nil
}

const ruleColumns = `id, binding_id, name, enabled, position, spec`

// SaveRule 保存一条规则，同名则更新。
func (s *Store) SaveRule(ctx context.Context, bindingID int64, position int, r spec.Rule) (*RuleRecord, error) {
	name, enabled, body, err := splitRule(r)
	if err != nil {
		return nil, err
	}
	if name == "" {
		return nil, fmt.Errorf("store: 规则名不能为空")
	}

	rec, err := scanRule(s.pool.QueryRow(ctx, `
		INSERT INTO rules (binding_id, name, enabled, position, spec)
		VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (binding_id, name) DO UPDATE SET
			enabled    = EXCLUDED.enabled,
			position   = EXCLUDED.position,
			spec       = EXCLUDED.spec,
			updated_at = now()
		RETURNING `+ruleColumns,
		bindingID, name, enabled, position, body))
	if err != nil {
		return nil, fmt.Errorf("store: 保存规则 %q 失败: %w", name, err)
	}
	return rec, nil
}

// GetRule 按绑定与规则名查询。
func (s *Store) GetRule(ctx context.Context, bindingID int64, name string) (*RuleRecord, error) {
	rec, err := scanRule(s.pool.QueryRow(ctx,
		`SELECT `+ruleColumns+` FROM rules WHERE binding_id = $1 AND name = $2`,
		bindingID, name))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, fmt.Errorf("store: 规则 %q 不存在: %w", name, ErrNotFound)
	}
	if err != nil {
		return nil, fmt.Errorf("store: 查询规则失败: %w", err)
	}
	return rec, nil
}

// ListRules 按 position 顺序列出某绑定的全部规则。
func (s *Store) ListRules(ctx context.Context, bindingID int64) ([]RuleRecord, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT `+ruleColumns+` FROM rules WHERE binding_id = $1 ORDER BY position, id`,
		bindingID)
	if err != nil {
		return nil, fmt.Errorf("store: 列出规则失败: %w", err)
	}
	defer rows.Close()

	var out []RuleRecord
	for rows.Next() {
		rec, err := scanRule(rows)
		if err != nil {
			return nil, fmt.Errorf("store: 读取规则失败: %w", err)
		}
		out = append(out, *rec)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("store: 列出规则失败: %w", err)
	}
	return out, nil
}

// SetRuleEnabled 启停规则。只动列，不重写 JSONB。
func (s *Store) SetRuleEnabled(ctx context.Context, bindingID int64, name string, enabled bool) error {
	tag, err := s.pool.Exec(ctx, `
		UPDATE rules SET enabled = $1, updated_at = now()
		WHERE binding_id = $2 AND name = $3`, enabled, bindingID, name)
	if err != nil {
		return fmt.Errorf("store: 更新规则 %q 状态失败: %w", name, err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("store: 规则 %q 不存在: %w", name, ErrNotFound)
	}
	return nil
}

// DeleteRule 删除一条规则。
func (s *Store) DeleteRule(ctx context.Context, bindingID int64, name string) error {
	tag, err := s.pool.Exec(ctx,
		`DELETE FROM rules WHERE binding_id = $1 AND name = $2`, bindingID, name)
	if err != nil {
		return fmt.Errorf("store: 删除规则 %q 失败: %w", name, err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("store: 规则 %q 不存在: %w", name, ErrNotFound)
	}
	return nil
}

// ReplaceRules 用给定的规则集整体替换某绑定的规则。
//
// 供 import 使用：YAML 里删掉的规则，重新导入后库里也该没有。
// 整批在一个事务里，中途有一条非法就整体回滚——留下半套规则比
// 直接失败更难排查。
func (s *Store) ReplaceRules(ctx context.Context, bindingID int64, rs []spec.Rule) error {
	// 先校验并拆好，再开事务：非法输入不该浪费一次事务
	type prepared struct {
		name    string
		enabled bool
		body    []byte
	}
	items := make([]prepared, 0, len(rs))
	seen := make(map[string]bool, len(rs))

	for _, r := range rs {
		name, enabled, body, err := splitRule(r)
		if err != nil {
			return err
		}
		if name == "" {
			return fmt.Errorf("store: 规则名不能为空")
		}
		// 规则名只需在单个绑定内唯一，但同绑定内重名会让冷却互相干扰
		if seen[name] {
			return fmt.Errorf("store: 同一绑定下规则名 %q 重复", name)
		}
		seen[name] = true
		items = append(items, prepared{name, enabled, body})
	}

	err := pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, `DELETE FROM rules WHERE binding_id = $1`, bindingID); err != nil {
			return err
		}
		for i, it := range items {
			if _, err := tx.Exec(ctx, `
				INSERT INTO rules (binding_id, name, enabled, position, spec)
				VALUES ($1, $2, $3, $4, $5)`,
				bindingID, it.name, it.enabled, i, it.body); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return fmt.Errorf("store: 替换规则失败: %w", err)
	}
	return nil
}
```

- [ ] **Step 4: 跑测试确认通过**

```bash
cd server; go test ./internal/store/ -run 'Rule' -v 2>&1 | tail -40; echo "退出码=$?"
```

- [ ] **Step 5: 提交**

```bash
cd server; gofmt -l . ; go vet ./internal/store/; echo "退出码=$?"
git add server/internal/store/
git commit -m "$(cat <<'EOF'
feat: 新增规则表读写

规则体以 JSONB 存，name 与 enabled 提到列上且 JSONB 里不含这两个
字段——同一个值存两处必然漂移。拆装收在 splitRule 与 assembleRule。

非法规则不许进库：写进去了，run 每次启动都会炸。ReplaceRules 整批
在一个事务里，中途有一条非法就整体回滚。

Co-Authored-By: Claude Opus 5 <noreply@anthropic.com>
EOF
)"
```

---

## Task 8: 授权

**Files:**
- Create: `server/internal/store/membership.go`
- Create: `server/internal/store/membership_test.go`

**Interfaces:**
- Consumes: `perm.Permission`、`perm.Strings`（Task 1）；`store.Binding`（Task 6）
- Produces:
  - `type store.Membership struct { ID, UserID, BindingID int64; Username, AccountName, RoomID string; Permissions []perm.Permission }`
  - `func (s *Store) Grant(ctx context.Context, username, accountName, roomID string, ps []perm.Permission) error`
  - `func (s *Store) Revoke(ctx context.Context, username, accountName, roomID string) error`
  - `func (s *Store) ListMemberships(ctx context.Context, username string) ([]Membership, error)`
  - `func (s *Store) ListBindingMembers(ctx context.Context, accountName, roomID string) ([]Membership, error)`
  - `func (s *Store) Can(ctx context.Context, userID, bindingID int64, p perm.Permission) (bool, error)`

- [ ] **Step 1: 写失败的测试**

创建 `server/internal/store/membership_test.go`：

```go
package store

import (
	"context"
	"errors"
	"testing"

	"github.com/MrZhongzq/MagicalDanmaku/server/internal/perm"
)

func TestGrantAndCan(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()

	uid := mustUser(t, s, "李四")
	bid := mustBinding(t, s, "小号", "123")

	if err := s.Grant(ctx, "李四", "小号", "123",
		[]perm.Permission{perm.RuleRead, perm.RuleWrite}); err != nil {
		t.Fatalf("授权报错: %v", err)
	}

	ok, err := s.Can(ctx, uid, bid, perm.RuleWrite)
	if err != nil {
		t.Fatalf("Can 报错: %v", err)
	}
	if !ok {
		t.Error("已授予的权限点应通过")
	}

	ok, err = s.Can(ctx, uid, bid, perm.UserBlock)
	if err != nil {
		t.Fatalf("Can 报错: %v", err)
	}
	if ok {
		t.Error("未授予的权限点不应通过")
	}
}

// 授权单位是绑定：能改甲房间的规则，不代表能碰乙房间
func TestPermissionDoesNotLeakAcrossBindings(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()

	uid := mustUser(t, s, "李四")
	accID := mustAccount(t, s, "小号")
	b1, err := s.UpsertBinding(ctx, accID, "甲")
	if err != nil {
		t.Fatalf("创建绑定甲报错: %v", err)
	}
	b2, err := s.UpsertBinding(ctx, accID, "乙")
	if err != nil {
		t.Fatalf("创建绑定乙报错: %v", err)
	}

	if err := s.Grant(ctx, "李四", "小号", "甲",
		[]perm.Permission{perm.RuleWrite}); err != nil {
		t.Fatalf("授权报错: %v", err)
	}

	ok, err := s.Can(ctx, uid, b1.ID, perm.RuleWrite)
	if err != nil {
		t.Fatalf("Can 报错: %v", err)
	}
	if !ok {
		t.Error("绑定甲上应有权限")
	}

	ok, err = s.Can(ctx, uid, b2.ID, perm.RuleWrite)
	if err != nil {
		t.Fatalf("Can 报错: %v", err)
	}
	if ok {
		t.Error("绑定乙上不该有权限——授权单位是绑定，不是账号")
	}
}

func TestAdminBypassesAllChecks(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()

	admin, err := s.CreateUser(ctx, "管理员", "pw", true)
	if err != nil {
		t.Fatalf("创建管理员报错: %v", err)
	}
	bid := mustBinding(t, s, "小号", "123")

	for _, p := range perm.All() {
		ok, err := s.Can(ctx, admin.ID, bid, p)
		if err != nil {
			t.Fatalf("Can(%s) 报错: %v", p, err)
		}
		if !ok {
			t.Errorf("管理员应绕过 %s 的检查", p)
		}
	}
}

func TestCanWithoutMembershipIsFalse(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()

	uid := mustUser(t, s, "路人")
	bid := mustBinding(t, s, "小号", "123")

	ok, err := s.Can(ctx, uid, bid, perm.RuleRead)
	if err != nil {
		t.Fatalf("Can 报错: %v", err)
	}
	if ok {
		t.Error("没有授权记录时应一律拒绝")
	}
}

func TestGrantReplacesPreviousPermissions(t *testing.T) {
	// 重新授权是「设定为这些」，不是「再加上这些」
	s := testStore(t)
	ctx := context.Background()

	uid := mustUser(t, s, "李四")
	bid := mustBinding(t, s, "小号", "123")

	if err := s.Grant(ctx, "李四", "小号", "123",
		[]perm.Permission{perm.RuleRead, perm.RuleWrite}); err != nil {
		t.Fatalf("首次授权报错: %v", err)
	}
	if err := s.Grant(ctx, "李四", "小号", "123",
		[]perm.Permission{perm.EventRead}); err != nil {
		t.Fatalf("二次授权报错: %v", err)
	}

	ok, err := s.Can(ctx, uid, bid, perm.RuleWrite)
	if err != nil {
		t.Fatalf("Can 报错: %v", err)
	}
	if ok {
		t.Error("重新授权应替换而非累加")
	}
	ok, err = s.Can(ctx, uid, bid, perm.EventRead)
	if err != nil {
		t.Fatalf("Can 报错: %v", err)
	}
	if !ok {
		t.Error("新授予的权限点应生效")
	}
}

func TestRevoke(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()

	uid := mustUser(t, s, "李四")
	bid := mustBinding(t, s, "小号", "123")

	if err := s.Grant(ctx, "李四", "小号", "123",
		[]perm.Permission{perm.RuleRead}); err != nil {
		t.Fatalf("授权报错: %v", err)
	}
	if err := s.Revoke(ctx, "李四", "小号", "123"); err != nil {
		t.Fatalf("撤销报错: %v", err)
	}

	ok, err := s.Can(ctx, uid, bid, perm.RuleRead)
	if err != nil {
		t.Fatalf("Can 报错: %v", err)
	}
	if ok {
		t.Error("撤销后不应再有权限")
	}
}

func TestRevokeMissingMembership(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	mustUser(t, s, "李四")
	mustBinding(t, s, "小号", "123")

	if err := s.Revoke(ctx, "李四", "小号", "123"); !errors.Is(err, ErrNotFound) {
		t.Errorf("应返回 ErrNotFound，实际: %v", err)
	}
}

func TestGrantRejectsUnknownUser(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	mustBinding(t, s, "小号", "123")

	err := s.Grant(ctx, "查无此人", "小号", "123", []perm.Permission{perm.RuleRead})
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("应返回 ErrNotFound，实际: %v", err)
	}
}

func TestGrantRejectsUnknownBinding(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	mustUser(t, s, "李四")

	err := s.Grant(ctx, "李四", "没这个号", "123", []perm.Permission{perm.RuleRead})
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("应返回 ErrNotFound，实际: %v", err)
	}
}

func TestGrantRejectsEmptyPermissions(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	mustUser(t, s, "李四")
	mustBinding(t, s, "小号", "123")

	if err := s.Grant(ctx, "李四", "小号", "123", nil); err == nil {
		t.Error("空权限列表应被拒绝——那等于什么都没授权，语义含糊")
	}
}

func TestListMembershipsForUser(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()

	mustUser(t, s, "李四")
	accID := mustAccount(t, s, "小号")
	if _, err := s.UpsertBinding(ctx, accID, "甲"); err != nil {
		t.Fatalf("创建绑定甲报错: %v", err)
	}
	if _, err := s.UpsertBinding(ctx, accID, "乙"); err != nil {
		t.Fatalf("创建绑定乙报错: %v", err)
	}

	if err := s.Grant(ctx, "李四", "小号", "甲",
		[]perm.Permission{perm.RuleWrite}); err != nil {
		t.Fatalf("授权甲报错: %v", err)
	}
	if err := s.Grant(ctx, "李四", "小号", "乙",
		[]perm.Permission{perm.EventRead, perm.UserBlock}); err != nil {
		t.Fatalf("授权乙报错: %v", err)
	}

	ms, err := s.ListMemberships(ctx, "李四")
	if err != nil {
		t.Fatalf("列出授权报错: %v", err)
	}
	if len(ms) != 2 {
		t.Fatalf("授权数 = %d, 期望 2", len(ms))
	}
	if ms[0].RoomID != "甲" || len(ms[0].Permissions) != 1 {
		t.Errorf("第一条 = %+v", ms[0])
	}
	if len(ms[1].Permissions) != 2 {
		t.Errorf("第二条的权限点数 = %d, 期望 2", len(ms[1].Permissions))
	}
	if ms[0].AccountName != "小号" || ms[0].Username != "李四" {
		t.Errorf("账号名与用户名应带出: %+v", ms[0])
	}
}

func TestListBindingMembers(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()

	mustUser(t, s, "李四")
	mustUser(t, s, "王五")
	mustBinding(t, s, "小号", "123")

	if err := s.Grant(ctx, "李四", "小号", "123",
		[]perm.Permission{perm.RuleWrite}); err != nil {
		t.Fatalf("授权李四报错: %v", err)
	}
	if err := s.Grant(ctx, "王五", "小号", "123",
		[]perm.Permission{perm.UserBlock}); err != nil {
		t.Fatalf("授权王五报错: %v", err)
	}

	ms, err := s.ListBindingMembers(ctx, "小号", "123")
	if err != nil {
		t.Fatalf("列出成员报错: %v", err)
	}
	if len(ms) != 2 {
		t.Fatalf("成员数 = %d, 期望 2", len(ms))
	}
}

func TestDeletingUserRemovesMemberships(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()

	u, err := s.CreateUser(ctx, "李四", "pw", false)
	if err != nil {
		t.Fatalf("创建用户报错: %v", err)
	}
	mustBinding(t, s, "小号", "123")
	if err := s.Grant(ctx, "李四", "小号", "123",
		[]perm.Permission{perm.RuleRead}); err != nil {
		t.Fatalf("授权报错: %v", err)
	}

	if _, err := s.pool.Exec(ctx, `DELETE FROM users WHERE id = $1`, u.ID); err != nil {
		t.Fatalf("删除用户报错: %v", err)
	}

	var n int
	if err := s.pool.QueryRow(ctx, `SELECT count(*) FROM memberships`).Scan(&n); err != nil {
		t.Fatalf("统计授权报错: %v", err)
	}
	if n != 0 {
		t.Errorf("用户删除后授权应级联删除，实际剩 %d 条", n)
	}
}
```

- [ ] **Step 2: 跑测试确认失败**

```bash
cd server; go test ./internal/store/ -run TestGrantAndCan 2>&1 | tail -5; echo "退出码=$?"
```

- [ ] **Step 3: 实现**

创建 `server/internal/store/membership.go`：

```go
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
func (s *Store) Can(ctx context.Context, userID, bindingID int64, p perm.Permission) (bool, error) {
	var ok bool
	err := s.pool.QueryRow(ctx, `
		SELECT EXISTS (SELECT 1 FROM users WHERE id = $1 AND is_admin)
		    OR EXISTS (
				SELECT 1 FROM memberships
				WHERE user_id = $1 AND binding_id = $2 AND $3 = ANY(permissions)
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
```

- [ ] **Step 4: 跑测试确认通过**

```bash
cd server; go test ./internal/store/ -run 'Grant|Can|Revoke|Membership|Permission|Admin' -v 2>&1 | tail -40; echo "退出码=$?"
```

- [ ] **Step 5: 提交**

```bash
cd server; gofmt -l . ; go vet ./internal/store/; echo "退出码=$?"
git add server/internal/store/
git commit -m "$(cat <<'EOF'
feat: 新增按绑定授权的权限点检查

授权单位是「账号-直播间」绑定：能改小号在甲房间的规则，不代表能碰
乙房间。管理员绕过全部检查，没有授权记录时一律拒绝。

Grant 整组替换而非累加——重新授权的语义是「设定为这些」，累加会让
人以为撤掉了某项其实还在。

Co-Authored-By: Claude Opus 5 <noreply@anthropic.com>
EOF
)"
```

---

## Task 9: 脚本 KV 与禁言名单

**Files:**
- Create: `server/internal/store/kv.go`
- Create: `server/internal/store/blocklist.go`
- Create: `server/internal/store/kv_test.go`
- Create: `server/internal/store/blocklist_test.go`

**Interfaces:**
- Consumes: `store.Binding`（Task 6）、`rules.Storage` 接口（`internal/rules/script.go`，方法为 `Get(key string) (string, bool)` 与 `Set(key, value string)`）
- Produces:
  - `func (s *Store) BindingStorage(bindingID int64) *BindingStorage`
  - `type store.BindingStorage`，实现 `rules.Storage`
  - `type store.BlockedUser struct { ID int64; UID, Username, Reason string; CreatedBy *int64; CreatedAt time.Time }`
  - `func (s *Store) AddToBlockList(ctx context.Context, bindingID int64, uid, username, reason string, createdBy *int64) error`
  - `func (s *Store) RemoveFromBlockList(ctx context.Context, bindingID int64, uid string) error`
  - `func (s *Store) ListBlockList(ctx context.Context, bindingID int64) ([]BlockedUser, error)`
  - `func (s *Store) IsBlocked(ctx context.Context, bindingID int64, uid string) (bool, error)`

- [ ] **Step 1: 写失败的测试**

创建 `server/internal/store/kv_test.go`：

```go
package store

import (
	"context"
	"testing"

	"github.com/MrZhongzq/MagicalDanmaku/server/internal/rules"
)

func TestBindingStorageImplementsRulesStorage(t *testing.T) {
	// 编译期断言：脚本沙箱注入的就是这个接口
	var _ rules.Storage = (*BindingStorage)(nil)
}

func TestBindingStorageSetAndGet(t *testing.T) {
	s := testStore(t)
	bid := mustBinding(t, s, "小号", "123")
	st := s.BindingStorage(bid)

	st.Set("累计礼物", "42")

	v, ok := st.Get("累计礼物")
	if !ok {
		t.Fatal("刚写入的键应读得到")
	}
	if v != "42" {
		t.Errorf("值 = %q, 期望 42", v)
	}
}

func TestBindingStorageGetMissingReturnsFalse(t *testing.T) {
	s := testStore(t)
	bid := mustBinding(t, s, "小号", "123")
	st := s.BindingStorage(bid)

	v, ok := st.Get("没写过的键")
	if ok {
		t.Error("未写过的键应返回 false")
	}
	if v != "" {
		t.Errorf("未写过的键应返回空串，实际 %q", v)
	}
}

func TestBindingStorageSetOverwrites(t *testing.T) {
	s := testStore(t)
	bid := mustBinding(t, s, "小号", "123")
	st := s.BindingStorage(bid)

	st.Set("k", "旧")
	st.Set("k", "新")

	v, _ := st.Get("k")
	if v != "新" {
		t.Errorf("值 = %q, 期望「新」", v)
	}
}

// 每个绑定有独立的命名空间，小号在甲房间写的键不该被乙房间读到
func TestBindingStorageIsolatedPerBinding(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	accID := mustAccount(t, s, "小号")

	b1, err := s.UpsertBinding(ctx, accID, "甲")
	if err != nil {
		t.Fatalf("创建绑定甲报错: %v", err)
	}
	b2, err := s.UpsertBinding(ctx, accID, "乙")
	if err != nil {
		t.Fatalf("创建绑定乙报错: %v", err)
	}

	s.BindingStorage(b1.ID).Set("计数", "10")

	if v, ok := s.BindingStorage(b2.ID).Get("计数"); ok {
		t.Errorf("绑定乙不该读到绑定甲的键，实际读到 %q", v)
	}
}

// storage 是脚本用的，写失败不能让脚本崩掉——记日志并继续
func TestBindingStorageSurvivesWriteFailure(t *testing.T) {
	s := testStore(t)
	// 绑定不存在，外键会挡住写入
	st := s.BindingStorage(999999)

	st.Set("k", "v") // 不应 panic

	if _, ok := st.Get("k"); ok {
		t.Error("写入失败后不该读到值")
	}
}
```

创建 `server/internal/store/blocklist_test.go`：

```go
package store

import (
	"context"
	"errors"
	"testing"
)

func TestAddAndListBlockList(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	uid := mustUser(t, s, "房管")
	bid := mustBinding(t, s, "小号", "123")

	if err := s.AddToBlockList(ctx, bid, "10086", "广告号", "刷屏加群", &uid); err != nil {
		t.Fatalf("加入名单报错: %v", err)
	}

	list, err := s.ListBlockList(ctx, bid)
	if err != nil {
		t.Fatalf("列出名单报错: %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("名单长度 = %d, 期望 1", len(list))
	}
	if list[0].UID != "10086" || list[0].Reason != "刷屏加群" {
		t.Errorf("记录 = %+v", list[0])
	}
	if list[0].CreatedBy == nil || *list[0].CreatedBy != uid {
		t.Errorf("操作人未记录: %+v", list[0].CreatedBy)
	}
}

func TestIsBlocked(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	bid := mustBinding(t, s, "小号", "123")

	if err := s.AddToBlockList(ctx, bid, "10086", "", "", nil); err != nil {
		t.Fatalf("加入名单报错: %v", err)
	}

	blocked, err := s.IsBlocked(ctx, bid, "10086")
	if err != nil {
		t.Fatalf("IsBlocked 报错: %v", err)
	}
	if !blocked {
		t.Error("名单内的 UID 应返回 true")
	}

	blocked, err = s.IsBlocked(ctx, bid, "99999")
	if err != nil {
		t.Fatalf("IsBlocked 报错: %v", err)
	}
	if blocked {
		t.Error("名单外的 UID 应返回 false")
	}
}

func TestBlockListIsolatedPerBinding(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	accID := mustAccount(t, s, "小号")

	b1, err := s.UpsertBinding(ctx, accID, "甲")
	if err != nil {
		t.Fatalf("创建绑定甲报错: %v", err)
	}
	b2, err := s.UpsertBinding(ctx, accID, "乙")
	if err != nil {
		t.Fatalf("创建绑定乙报错: %v", err)
	}

	if err := s.AddToBlockList(ctx, b1.ID, "10086", "", "", nil); err != nil {
		t.Fatalf("加入名单报错: %v", err)
	}

	blocked, err := s.IsBlocked(ctx, b2.ID, "10086")
	if err != nil {
		t.Fatalf("IsBlocked 报错: %v", err)
	}
	if blocked {
		t.Error("甲房间的禁言名单不该影响乙房间")
	}
}

func TestAddToBlockListTwiceUpdatesReason(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	bid := mustBinding(t, s, "小号", "123")

	if err := s.AddToBlockList(ctx, bid, "10086", "旧名字", "旧理由", nil); err != nil {
		t.Fatalf("首次加入报错: %v", err)
	}
	if err := s.AddToBlockList(ctx, bid, "10086", "新名字", "新理由", nil); err != nil {
		t.Fatalf("二次加入报错: %v", err)
	}

	list, err := s.ListBlockList(ctx, bid)
	if err != nil {
		t.Fatalf("列出名单报错: %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("重复加入不该产生两条，实际 %d 条", len(list))
	}
	if list[0].Reason != "新理由" || list[0].Username != "新名字" {
		t.Errorf("应更新为最新信息: %+v", list[0])
	}
}

func TestRemoveFromBlockList(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	bid := mustBinding(t, s, "小号", "123")

	if err := s.AddToBlockList(ctx, bid, "10086", "", "", nil); err != nil {
		t.Fatalf("加入名单报错: %v", err)
	}
	if err := s.RemoveFromBlockList(ctx, bid, "10086"); err != nil {
		t.Fatalf("移出名单报错: %v", err)
	}

	blocked, err := s.IsBlocked(ctx, bid, "10086")
	if err != nil {
		t.Fatalf("IsBlocked 报错: %v", err)
	}
	if blocked {
		t.Error("移出后应返回 false")
	}
}

func TestRemoveMissingFromBlockList(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	bid := mustBinding(t, s, "小号", "123")

	if err := s.RemoveFromBlockList(ctx, bid, "10086"); !errors.Is(err, ErrNotFound) {
		t.Errorf("应返回 ErrNotFound，实际: %v", err)
	}
}

// 操作人被删除时名单要留下，只是不知道是谁加的
func TestBlockListSurvivesUserDeletion(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()

	u, err := s.CreateUser(ctx, "房管", "pw", false)
	if err != nil {
		t.Fatalf("创建用户报错: %v", err)
	}
	bid := mustBinding(t, s, "小号", "123")
	if err := s.AddToBlockList(ctx, bid, "10086", "", "刷屏", &u.ID); err != nil {
		t.Fatalf("加入名单报错: %v", err)
	}

	if _, err := s.pool.Exec(ctx, `DELETE FROM users WHERE id = $1`, u.ID); err != nil {
		t.Fatalf("删除用户报错: %v", err)
	}

	list, err := s.ListBlockList(ctx, bid)
	if err != nil {
		t.Fatalf("列出名单报错: %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("名单应保留，实际 %d 条", len(list))
	}
	if list[0].CreatedBy != nil {
		t.Errorf("操作人应置空，实际 %v", *list[0].CreatedBy)
	}
}
```

- [ ] **Step 2: 跑测试确认失败**

```bash
cd server; go test ./internal/store/ -run 'BindingStorage|BlockList' 2>&1 | tail -5; echo "退出码=$?"
```

- [ ] **Step 3: 实现 KV**

创建 `server/internal/store/kv.go`：

```go
package store

import (
	"context"
	"errors"
	"log/slog"

	"github.com/jackc/pgx/v5"
)

// BindingStorage 是某个绑定的键值存储，注入给规则脚本使用。
//
// 实现 rules.Storage。该接口的方法不带 ctx 也不返回 error——它要被
// goja 从 JS 里同步调用，签名必须简单。因此这里自带 context.Background
// 并把错误吞进日志：脚本里一次 storage.set 失败，不该让整条规则崩掉。
type BindingStorage struct {
	store     *Store
	bindingID int64
	log       *slog.Logger
}

// BindingStorage 返回某绑定的键值存储。
func (s *Store) BindingStorage(bindingID int64) *BindingStorage {
	return &BindingStorage{store: s, bindingID: bindingID, log: slog.Default()}
}

// Get 读取一个键。不存在时返回空串与 false。
func (b *BindingStorage) Get(key string) (string, bool) {
	var v string
	err := b.store.pool.QueryRow(context.Background(),
		`SELECT value FROM kv_store WHERE binding_id = $1 AND key = $2`,
		b.bindingID, key).Scan(&v)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", false
	}
	if err != nil {
		b.log.Error("读取脚本存储失败", "binding_id", b.bindingID, "key", key, "err", err)
		return "", false
	}
	return v, true
}

// Set 写入一个键。失败只记日志，不中断脚本。
func (b *BindingStorage) Set(key, value string) {
	_, err := b.store.pool.Exec(context.Background(), `
		INSERT INTO kv_store (binding_id, key, value) VALUES ($1, $2, $3)
		ON CONFLICT (binding_id, key) DO UPDATE SET
			value = EXCLUDED.value, updated_at = now()`,
		b.bindingID, key, value)
	if err != nil {
		b.log.Error("写入脚本存储失败", "binding_id", b.bindingID, "key", key, "err", err)
	}
}
```

- [ ] **Step 4: 实现禁言名单**

创建 `server/internal/store/blocklist.go`：

```go
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
```

- [ ] **Step 5: 跑测试确认通过**

```bash
cd server; go test ./internal/store/ -run 'BindingStorage|BlockList|Blocked' -v 2>&1 | tail -30; echo "退出码=$?"
```

- [ ] **Step 6: 提交**

```bash
cd server; gofmt -l . ; go vet ./internal/store/; echo "退出码=$?"
git add server/internal/store/
git commit -m "$(cat <<'EOF'
feat: 脚本 KV 与永久禁言名单入库

BindingStorage 实现 rules.Storage，把 P2 的内存存储换成按绑定隔离的
数据库表。该接口的方法不带 ctx 也不返回 error（要被 goja 同步调用），
因此写失败只记日志——脚本里一次 storage.set 失败不该让整条规则崩掉。

禁言名单的操作人用 ON DELETE SET NULL：人走了名单要留下，只是不知道
是谁加的。

Co-Authored-By: Claude Opus 5 <noreply@anthropic.com>
EOF
)"
```

---

## Task 10: 运行配置载入

把散在各表里的行拼成 `run` 需要的运行单元列表，字段与 P2 的 `config.Binding` 一一对应，只把 `CookieFile` 换成 `Cookie` 本身。

**Files:**
- Create: `server/internal/store/runconfig.go`
- Create: `server/internal/store/runconfig_test.go`

**Interfaces:**
- Consumes: Task 5–7、9 的全部读方法
- Produces:
  - `type store.RunConfig struct { BindingID, AccountID int64; AccountName, Cookie, RoomID string; RateLimit time.Duration; MaxLength int; CooldownGroups map[string]time.Duration; Rules []rules.Rule }`
  - `func (c RunConfig) Label() string`
  - `func (s *Store) LoadRunConfig(ctx context.Context) ([]RunConfig, error)`

- [ ] **Step 1: 写失败的测试**

创建 `server/internal/store/runconfig_test.go`：

```go
package store

import (
	"context"
	"testing"
	"time"

	"github.com/MrZhongzq/MagicalDanmaku/server/internal/rules/spec"
)

func TestLoadRunConfigAssemblesEverything(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()

	owner := mustUser(t, s, "张三")
	acc, err := s.CreateAccount(ctx, AccountInput{
		Name: "小号", UID: "42", Cookie: "SESSDATA=abc",
		RateLimit: 2 * time.Second, MaxLength: 30, OwnerID: owner,
	})
	if err != nil {
		t.Fatalf("创建账号报错: %v", err)
	}
	b, err := s.UpsertBinding(ctx, acc.ID, "1706666491")
	if err != nil {
		t.Fatalf("创建绑定报错: %v", err)
	}
	if err := s.SetCooldownGroups(ctx, b.ID, map[string]time.Duration{
		"greeting": 5 * time.Second,
	}); err != nil {
		t.Fatalf("写入冷却组报错: %v", err)
	}
	if _, err := s.SaveRule(ctx, b.ID, 0, spec.Rule{
		Name: "礼物答谢", On: []string{"gift"},
		Do: []spec.Action{{Type: "danmaku", Template: []string{"谢谢"}}},
	}); err != nil {
		t.Fatalf("保存规则报错: %v", err)
	}

	cfgs, err := s.LoadRunConfig(ctx)
	if err != nil {
		t.Fatalf("载入运行配置报错: %v", err)
	}
	if len(cfgs) != 1 {
		t.Fatalf("运行单元数 = %d, 期望 1", len(cfgs))
	}

	c := cfgs[0]
	if c.AccountName != "小号" || c.RoomID != "1706666491" {
		t.Errorf("绑定 = %s", c.Label())
	}
	if c.Cookie != "SESSDATA=abc" {
		t.Errorf("Cookie = %q", c.Cookie)
	}
	if c.RateLimit != 2*time.Second || c.MaxLength != 30 {
		t.Errorf("账号参数 = %v / %d", c.RateLimit, c.MaxLength)
	}
	if c.CooldownGroups["greeting"] != 5*time.Second {
		t.Errorf("冷却组 = %v", c.CooldownGroups)
	}
	if len(c.Rules) != 1 || c.Rules[0].Name != "礼物答谢" {
		t.Errorf("规则 = %+v", c.Rules)
	}
	if c.BindingID != b.ID || c.AccountID != acc.ID {
		t.Errorf("ID 未带出: binding=%d account=%d", c.BindingID, c.AccountID)
	}
}

func TestRunConfigLabel(t *testing.T) {
	c := RunConfig{AccountName: "小号", RoomID: "123"}
	if c.Label() != "小号@123" {
		t.Errorf("Label() = %q", c.Label())
	}
}

func TestLoadRunConfigSkipsDisabledBindings(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	accID := mustAccount(t, s, "小号")

	if _, err := s.UpsertBinding(ctx, accID, "111"); err != nil {
		t.Fatalf("创建绑定报错: %v", err)
	}
	if _, err := s.UpsertBinding(ctx, accID, "222"); err != nil {
		t.Fatalf("创建绑定报错: %v", err)
	}
	if err := s.SetBindingEnabled(ctx, "小号", "222", false); err != nil {
		t.Fatalf("停用绑定报错: %v", err)
	}

	cfgs, err := s.LoadRunConfig(ctx)
	if err != nil {
		t.Fatalf("载入运行配置报错: %v", err)
	}
	if len(cfgs) != 1 || cfgs[0].RoomID != "111" {
		t.Errorf("停用的绑定不该出现，实际 %+v", cfgs)
	}
}

// 停用的规则要带出来，由引擎自己跳过——引擎需要知道它存在才能在日志里报告
func TestLoadRunConfigIncludesDisabledRules(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	bid := mustBinding(t, s, "小号", "123")

	if _, err := s.SaveRule(ctx, bid, 0, spec.Rule{
		Name: "关着的", On: []string{"danmaku"}, Do: []spec.Action{{Type: "log"}},
	}); err != nil {
		t.Fatalf("保存规则报错: %v", err)
	}
	if err := s.SetRuleEnabled(ctx, bid, "关着的", false); err != nil {
		t.Fatalf("停用规则报错: %v", err)
	}

	cfgs, err := s.LoadRunConfig(ctx)
	if err != nil {
		t.Fatalf("载入运行配置报错: %v", err)
	}
	if len(cfgs) != 1 || len(cfgs[0].Rules) != 1 {
		t.Fatalf("规则应被带出: %+v", cfgs)
	}
	if cfgs[0].Rules[0].Enabled {
		t.Error("停用的规则 Enabled 应为 false")
	}
}

func TestLoadRunConfigPreservesRulePosition(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	bid := mustBinding(t, s, "小号", "123")

	if err := s.ReplaceRules(ctx, bid, []spec.Rule{
		{Name: "甲", On: []string{"danmaku"}, Do: []spec.Action{{Type: "log"}}},
		{Name: "乙", On: []string{"gift"}, Do: []spec.Action{{Type: "log"}}},
		{Name: "丙", On: []string{"guard_buy"}, Do: []spec.Action{{Type: "log"}}},
	}); err != nil {
		t.Fatalf("替换规则报错: %v", err)
	}

	cfgs, err := s.LoadRunConfig(ctx)
	if err != nil {
		t.Fatalf("载入运行配置报错: %v", err)
	}
	got := []string{cfgs[0].Rules[0].Name, cfgs[0].Rules[1].Name, cfgs[0].Rules[2].Name}
	want := []string{"甲", "乙", "丙"}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("规则顺序 = %v, 期望 %v", got, want)
		}
	}
}

func TestLoadRunConfigEmptyDatabase(t *testing.T) {
	s := testStore(t)
	cfgs, err := s.LoadRunConfig(context.Background())
	if err != nil {
		t.Fatalf("空库应正常返回而非报错: %v", err)
	}
	if len(cfgs) != 0 {
		t.Errorf("空库应返回空列表，实际 %+v", cfgs)
	}
}

// 同一直播间被两个账号连接时是两条独立运行单元
func TestLoadRunConfigTwoAccountsSameRoom(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	main := mustAccount(t, s, "主播号")
	sub := mustAccount(t, s, "小号")

	if _, err := s.UpsertBinding(ctx, main, "1706666491"); err != nil {
		t.Fatalf("主播号绑定报错: %v", err)
	}
	if _, err := s.UpsertBinding(ctx, sub, "1706666491"); err != nil {
		t.Fatalf("小号绑定报错: %v", err)
	}

	cfgs, err := s.LoadRunConfig(ctx)
	if err != nil {
		t.Fatalf("载入运行配置报错: %v", err)
	}
	if len(cfgs) != 2 {
		t.Fatalf("运行单元数 = %d, 期望 2", len(cfgs))
	}
	if cfgs[0].Label() == cfgs[1].Label() {
		t.Error("两条运行单元的标签不该相同")
	}
}
```

- [ ] **Step 2: 跑测试确认失败**

```bash
cd server; go test ./internal/store/ -run TestLoadRunConfig 2>&1 | tail -5; echo "退出码=$?"
```

- [ ] **Step 3: 实现**

创建 `server/internal/store/runconfig.go`：

```go
package store

import (
	"context"
	"fmt"
	"time"

	"github.com/MrZhongzq/MagicalDanmaku/server/internal/rules"
)

// RunConfig 是一个「账号-直播间」绑定的完整运行配置。
//
// 字段与 P2 的 config.Binding 一一对应，只把 CookieFile 换成了 Cookie
// 本身——run 的装配逻辑因此不必改动。
type RunConfig struct {
	BindingID      int64
	AccountID      int64
	AccountName    string
	Cookie         string
	RoomID         string
	RateLimit      time.Duration
	MaxLength      int
	CooldownGroups map[string]time.Duration
	Rules          []rules.Rule
}

// Label 返回用于日志的标识，形如 "小号@1706666491"。
func (c RunConfig) Label() string {
	return c.AccountName + "@" + c.RoomID
}

// LoadRunConfig 载入全部启用的绑定及其规则与冷却组。
//
// 停用的绑定直接跳过；停用的规则照常带出，由引擎自己跳过——引擎需要
// 知道它存在，才能在日志里报告「共 5 条规则，3 条启用」。
func (s *Store) LoadRunConfig(ctx context.Context) ([]RunConfig, error) {
	bindings, err := s.ListBindings(ctx)
	if err != nil {
		return nil, err
	}

	// 账号可能被多个绑定共用，只查一次
	accounts := make(map[int64]*Account)

	out := make([]RunConfig, 0, len(bindings))
	for _, b := range bindings {
		if !b.Enabled {
			continue
		}

		acc, ok := accounts[b.AccountID]
		if !ok {
			acc, err = s.GetAccountByName(ctx, b.AccountName)
			if err != nil {
				return nil, err
			}
			accounts[b.AccountID] = acc
		}

		groups, err := s.CooldownGroups(ctx, b.ID)
		if err != nil {
			return nil, err
		}

		recs, err := s.ListRules(ctx, b.ID)
		if err != nil {
			return nil, err
		}
		rs := make([]rules.Rule, 0, len(recs))
		for _, rec := range recs {
			r, err := rec.Domain()
			if err != nil {
				return nil, fmt.Errorf("store: %s 的规则 %q 非法: %w", b.Label(), rec.Name, err)
			}
			rs = append(rs, r)
		}

		out = append(out, RunConfig{
			BindingID:      b.ID,
			AccountID:      b.AccountID,
			AccountName:    b.AccountName,
			Cookie:         acc.Cookie,
			RoomID:         b.RoomID,
			RateLimit:      acc.RateLimit,
			MaxLength:      acc.MaxLength,
			CooldownGroups: groups,
			Rules:          rs,
		})
	}
	return out, nil
}
```

- [ ] **Step 4: 跑测试确认通过**

```bash
cd server; go test ./internal/store/ -run TestLoadRunConfig -v 2>&1 | tail -25; echo "退出码=$?"
```

- [ ] **Step 5: 跑整包并加竞态检测**

```bash
cd server; go test ./internal/store/ 2>&1 | tail -10; echo "退出码=$?"
cd server; go test -race ./internal/store/ 2>&1 | tail -10; echo "退出码=$?"
```

预期：两次都 PASS。

- [ ] **Step 6: 提交**

```bash
cd server; gofmt -l . ; go vet ./... ; echo "退出码=$?"
git add server/internal/store/
git commit -m "$(cat <<'EOF'
feat: 从数据库载入运行配置

RunConfig 的字段与 P2 的 config.Binding 一一对应，只把 CookieFile
换成 Cookie 本身，run 的装配逻辑因此不必改动。

停用的绑定跳过，停用的规则照常带出——引擎需要知道它存在，才能在
日志里报告「共 5 条规则，3 条启用」。

Co-Authored-By: Claude Opus 5 <noreply@anthropic.com>
EOF
)"
```

---

**Part 2 到此结束。** 继续 Task 11–14，见 `2026-07-31-p3-data-layer-part3.md`。
