# P3 多租户数据层 实施计划 · Part 4（Task 15–19）

> 接 `2026-07-31-p3-data-layer-part3.md`。Global Constraints 见 `2026-07-31-p3-data-layer.md`。
>
> 本部分把存储层接到命令行与 `run`，并补齐 CI 与文档。做完 P3 就完成了：`magicd run` 不再需要 `config.yaml`。

**对设计文档 §9 的一处补充：** 增加 `magicd binding` 子命令。原清单里创建绑定只能靠 `import`，那意味着扫码登录一个新账号后想加个直播间，得先写一份 YAML。这不合理。

---

## Task 15: 数据库连接与管理类子命令

**Files:**
- Create: `server/cmd/magicd/db.go`
- Create: `server/cmd/magicd/migrate.go`
- Create: `server/cmd/magicd/user.go`
- Create: `server/cmd/magicd/binding.go`
- Create: `server/cmd/magicd/grant.go`
- Create: `server/cmd/magicd/db_test.go`
- Modify: `server/cmd/magicd/main.go`（新增子命令分发、装配系统日志、更新用法）

**Interfaces:**
- Consumes: `store.Open`、`store.Migrate`、`store.SchemaVersion`、`store.LatestSchemaVersion`、`store.ErrSchemaOutdated`、`store.EnsureAdmin`、用户/账号/绑定/授权的全部方法；`perm.ParseList`、`perm.Strings`；`logging.SetupSystem`、`logging.SystemOptionsFromEnv`
- Produces:
  - `func addDBFlag(fs *flag.FlagSet) *string`
  - `func openStore(ctx context.Context, dsn string) (*store.Store, error)`
  - `func openStoreChecked(ctx context.Context, dsn string) (*store.Store, error)` —— 额外校验 schema 版本
  - `func parseBindingRef(s string) (accountName, roomID string, err error)` —— 解析 `小号@1706666491`
  - `func readPassword(prompt string) (string, error)`
  - 子命令入口 `runMigrate`、`runUser`、`runBinding`、`runGrant`、`runRevoke`

- [ ] **Step 1: 写不需要数据库的失败测试**

创建 `server/cmd/magicd/db_test.go`：

```go
package main

import (
	"strings"
	"testing"
)

func TestParseBindingRef(t *testing.T) {
	name, room, err := parseBindingRef("小号@1706666491")
	if err != nil {
		t.Fatalf("解析报错: %v", err)
	}
	if name != "小号" || room != "1706666491" {
		t.Errorf("= %q / %q", name, room)
	}
}

func TestParseBindingRefRejectsMissingAt(t *testing.T) {
	_, _, err := parseBindingRef("小号")
	if err == nil {
		t.Fatal("缺少 @ 应报错")
	}
	// 报错要给出正确写法，否则用户只能猜
	if !strings.Contains(err.Error(), "@") {
		t.Errorf("错误信息应示范格式，实际: %v", err)
	}
}

func TestParseBindingRefRejectsEmptyParts(t *testing.T) {
	for _, s := range []string{"@123", "小号@", "@"} {
		if _, _, err := parseBindingRef(s); err == nil {
			t.Errorf("%q 应报错", s)
		}
	}
}

// 账号名里可能带 @，取最后一个 @ 作为分隔符
func TestParseBindingRefSplitsOnLastAt(t *testing.T) {
	name, room, err := parseBindingRef("a@b@123")
	if err != nil {
		t.Fatalf("解析报错: %v", err)
	}
	if name != "a@b" || room != "123" {
		t.Errorf("= %q / %q, 期望 \"a@b\" / \"123\"", name, room)
	}
}

func TestOpenStoreRequiresDSN(t *testing.T) {
	t.Setenv("MAGICD_DATABASE_URL", "")
	_, err := openStore(t.Context(), "")
	if err == nil {
		t.Fatal("没有连接串应报错")
	}
	if !strings.Contains(err.Error(), "MAGICD_DATABASE_URL") {
		t.Errorf("错误信息应提示怎么配置，实际: %v", err)
	}
}
```

- [ ] **Step 2: 跑测试确认失败**

```bash
cd server; go test ./cmd/magicd/ -run 'ParseBindingRef|OpenStore' 2>&1 | tail -5; echo "退出码=$?"
```

- [ ] **Step 3: 实现数据库连接助手**

创建 `server/cmd/magicd/db.go`：

```go
package main

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"os"
	"strings"

	"golang.org/x/term"

	"github.com/MrZhongzq/MagicalDanmaku/server/internal/store"
)

// addDBFlag 给子命令挂上 -db 标志。
func addDBFlag(fs *flag.FlagSet) *string {
	return fs.String("db", "", "PostgreSQL 连接串，留空则读环境变量 MAGICD_DATABASE_URL")
}

// openStore 连接数据库。dsn 为空时回落到环境变量。
func openStore(ctx context.Context, dsn string) (*store.Store, error) {
	if dsn == "" {
		dsn = os.Getenv("MAGICD_DATABASE_URL")
	}
	if dsn == "" {
		return nil, fmt.Errorf("未指定数据库连接串。设置环境变量 MAGICD_DATABASE_URL 或用 -db 传入，例如：\n" +
			"  export MAGICD_DATABASE_URL='postgres://magicd:magicd@localhost:5432/magicd?sslmode=disable'")
	}
	return store.Open(ctx, dsn)
}

// openStoreChecked 连接数据库并校验 schema 版本。
//
// 版本落后就拒绝启动而不是自动迁移：多实例部署下，让每个实例各自
// 决定何时改表是危险的。
func openStoreChecked(ctx context.Context, dsn string) (*store.Store, error) {
	s, err := openStore(ctx, dsn)
	if err != nil {
		return nil, err
	}

	current, err := s.SchemaVersion(ctx)
	if err != nil {
		s.Close()
		return nil, err
	}
	latest, err := store.LatestSchemaVersion()
	if err != nil {
		s.Close()
		return nil, err
	}
	if current < latest {
		s.Close()
		return nil, fmt.Errorf("%w（当前 %d，需要 %d）", store.ErrSchemaOutdated, current, latest)
	}
	return s, nil
}

// parseBindingRef 解析形如 "小号@1706666491" 的绑定引用。
//
// 从最后一个 @ 切分：账号名是用户起的，可能自带 @。
func parseBindingRef(s string) (accountName, roomID string, err error) {
	i := strings.LastIndex(s, "@")
	if i < 0 {
		return "", "", fmt.Errorf("绑定要写成「账号名@房间号」的形式，例如 小号@1706666491，实际收到 %q", s)
	}
	accountName = strings.TrimSpace(s[:i])
	roomID = strings.TrimSpace(s[i+1:])
	if accountName == "" || roomID == "" {
		return "", "", fmt.Errorf("绑定 %q 的账号名或房间号为空，应写成「账号名@房间号」", s)
	}
	return accountName, roomID, nil
}

// readPassword 从终端读密码，不回显。
//
// 终端不可用时（管道、CI）回落到读一行明文：让脚本能用，
// 代价是密码会出现在进程的标准输入里。
func readPassword(prompt string) (string, error) {
	fmt.Print(prompt)

	fd := int(os.Stdin.Fd())
	if term.IsTerminal(fd) {
		b, err := term.ReadPassword(fd)
		fmt.Println()
		if err != nil {
			return "", fmt.Errorf("读取密码失败: %w", err)
		}
		return string(b), nil
	}

	line, err := bufio.NewReader(os.Stdin).ReadString('\n')
	if err != nil && line == "" {
		return "", fmt.Errorf("读取密码失败: %w", err)
	}
	return strings.TrimRight(line, "\r\n"), nil
}
```

`golang.org/x/term` 是 `golang.org/x/crypto` 的兄弟包，纯 Go：

```bash
cd server; go get golang.org/x/term@latest; go mod tidy; echo "退出码=$?"
```

- [ ] **Step 4: 实现 migrate 子命令**

创建 `server/cmd/magicd/migrate.go`：

```go
package main

import (
	"context"
	"flag"
	"fmt"
)

// runMigrate 建表或升级 schema，并在空库上创建首个管理员。
func runMigrate(args []string) error {
	fs := flag.NewFlagSet("migrate", flag.ExitOnError)
	dsn := addDBFlag(fs)
	if err := fs.Parse(args); err != nil {
		return err
	}

	ctx := context.Background()
	// 用 openStore 而非 openStoreChecked：migrate 正是用来消除版本落后的
	s, err := openStore(ctx, *dsn)
	if err != nil {
		return err
	}
	defer s.Close()

	before, err := s.SchemaVersion(ctx)
	if err != nil {
		return err
	}
	if err := s.Migrate(ctx); err != nil {
		return err
	}
	after, err := s.SchemaVersion(ctx)
	if err != nil {
		return err
	}

	if before == after {
		fmt.Printf("schema 已是最新版本（v%d），无需迁移\n", after)
	} else {
		fmt.Printf("schema 已从 v%d 升级到 v%d\n", before, after)
	}

	// 造数据与改表分开：migrate 可以反复跑，建管理员只在空库上发生一次
	name, pass, created, err := s.EnsureAdmin(ctx)
	if err != nil {
		return err
	}
	if created {
		fmt.Println()
		fmt.Println("已创建管理员账户，密码只显示这一次，请立即保存：")
		fmt.Printf("  用户名: %s\n", name)
		fmt.Printf("  密码:   %s\n", pass)
		fmt.Println()
		fmt.Printf("改密码：magicd user passwd %s\n", name)
	}
	return nil
}
```

- [ ] **Step 5: 实现 user 子命令**

创建 `server/cmd/magicd/user.go`：

```go
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"text/tabwriter"
)

const userUsage = `用法:
  magicd user add <用户名> [--admin]   创建用户，交互式设置密码
  magicd user passwd <用户名>          修改密码
  magicd user list                     列出全部用户
`

// runUser 分发 user 的子命令。
func runUser(args []string) error {
	if len(args) == 0 {
		fmt.Fprint(os.Stderr, userUsage)
		return fmt.Errorf("user 需要一个子命令")
	}
	switch args[0] {
	case "add":
		return runUserAdd(args[1:])
	case "passwd":
		return runUserPasswd(args[1:])
	case "list":
		return runUserList(args[1:])
	default:
		fmt.Fprint(os.Stderr, userUsage)
		return fmt.Errorf("未知的 user 子命令: %s", args[0])
	}
}

func runUserAdd(args []string) error {
	fs := flag.NewFlagSet("user add", flag.ExitOnError)
	dsn := addDBFlag(fs)
	admin := fs.Bool("admin", false, "创建为管理员，绕过全部权限检查")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 1 {
		return fmt.Errorf("用法: magicd user add <用户名> [--admin]")
	}
	username := fs.Arg(0)

	pass, err := readPassword(fmt.Sprintf("为 %s 设置密码: ", username))
	if err != nil {
		return err
	}
	again, err := readPassword("再输入一次: ")
	if err != nil {
		return err
	}
	if pass != again {
		return fmt.Errorf("两次输入的密码不一致")
	}

	ctx := context.Background()
	s, err := openStoreChecked(ctx, *dsn)
	if err != nil {
		return err
	}
	defer s.Close()

	u, err := s.CreateUser(ctx, username, pass, *admin)
	if err != nil {
		return err
	}
	role := "普通用户"
	if u.IsAdmin {
		role = "管理员"
	}
	fmt.Printf("已创建%s %s（ID %d）\n", role, u.Username, u.ID)
	return nil
}

func runUserPasswd(args []string) error {
	fs := flag.NewFlagSet("user passwd", flag.ExitOnError)
	dsn := addDBFlag(fs)
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 1 {
		return fmt.Errorf("用法: magicd user passwd <用户名>")
	}
	username := fs.Arg(0)

	pass, err := readPassword(fmt.Sprintf("为 %s 设置新密码: ", username))
	if err != nil {
		return err
	}
	again, err := readPassword("再输入一次: ")
	if err != nil {
		return err
	}
	if pass != again {
		return fmt.Errorf("两次输入的密码不一致")
	}

	ctx := context.Background()
	s, err := openStoreChecked(ctx, *dsn)
	if err != nil {
		return err
	}
	defer s.Close()

	if err := s.SetPassword(ctx, username, pass); err != nil {
		return err
	}
	fmt.Printf("%s 的密码已修改\n", username)
	return nil
}

func runUserList(args []string) error {
	fs := flag.NewFlagSet("user list", flag.ExitOnError)
	dsn := addDBFlag(fs)
	if err := fs.Parse(args); err != nil {
		return err
	}

	ctx := context.Background()
	s, err := openStoreChecked(ctx, *dsn)
	if err != nil {
		return err
	}
	defer s.Close()

	us, err := s.ListUsers(ctx)
	if err != nil {
		return err
	}
	if len(us) == 0 {
		fmt.Println("还没有任何用户。运行 magicd migrate 会创建首个管理员。")
		return nil
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "ID\t用户名\t角色\t创建时间")
	for _, u := range us {
		role := "普通用户"
		if u.IsAdmin {
			role = "管理员"
		}
		fmt.Fprintf(w, "%d\t%s\t%s\t%s\n",
			u.ID, u.Username, role, u.CreatedAt.Format("2006-01-02 15:04"))
	}
	return w.Flush()
}
```

- [ ] **Step 6: 实现 binding 与 account 子命令**

创建 `server/cmd/magicd/binding.go`：

```go
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"text/tabwriter"
)

const bindingUsage = `用法:
  magicd binding add <账号名> <房间号>   让账号连接一个直播间
  magicd binding list                    列出全部绑定
  magicd binding rm <账号名@房间号>      删除绑定及其规则
  magicd binding enable  <账号名@房间号>
  magicd binding disable <账号名@房间号>
`

// runBinding 分发 binding 的子命令。
func runBinding(args []string) error {
	if len(args) == 0 {
		fmt.Fprint(os.Stderr, bindingUsage)
		return fmt.Errorf("binding 需要一个子命令")
	}
	switch args[0] {
	case "add":
		return runBindingAdd(args[1:])
	case "list":
		return runBindingList(args[1:])
	case "rm":
		return runBindingRemove(args[1:])
	case "enable":
		return runBindingSetEnabled(args[1:], true)
	case "disable":
		return runBindingSetEnabled(args[1:], false)
	default:
		fmt.Fprint(os.Stderr, bindingUsage)
		return fmt.Errorf("未知的 binding 子命令: %s", args[0])
	}
}

func runBindingAdd(args []string) error {
	fs := flag.NewFlagSet("binding add", flag.ExitOnError)
	dsn := addDBFlag(fs)
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 2 {
		return fmt.Errorf("用法: magicd binding add <账号名> <房间号>")
	}

	ctx := context.Background()
	s, err := openStoreChecked(ctx, *dsn)
	if err != nil {
		return err
	}
	defer s.Close()

	acc, err := s.GetAccountByName(ctx, fs.Arg(0))
	if err != nil {
		return err
	}
	b, err := s.UpsertBinding(ctx, acc.ID, fs.Arg(1))
	if err != nil {
		return err
	}
	fmt.Printf("已添加绑定 %s（ID %d）\n", b.Label(), b.ID)
	return nil
}

func runBindingList(args []string) error {
	fs := flag.NewFlagSet("binding list", flag.ExitOnError)
	dsn := addDBFlag(fs)
	if err := fs.Parse(args); err != nil {
		return err
	}

	ctx := context.Background()
	s, err := openStoreChecked(ctx, *dsn)
	if err != nil {
		return err
	}
	defer s.Close()

	bs, err := s.ListBindings(ctx)
	if err != nil {
		return err
	}
	if len(bs) == 0 {
		fmt.Println("还没有任何绑定。先 magicd login --save <账号名>，再 magicd binding add <账号名> <房间号>。")
		return nil
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "绑定\t状态\t规则数")
	for _, b := range bs {
		status := "启用"
		if !b.Enabled {
			status = "停用"
		}
		rs, err := s.ListRules(ctx, b.ID)
		if err != nil {
			return err
		}
		fmt.Fprintf(w, "%s\t%s\t%d\n", b.Label(), status, len(rs))
	}
	return w.Flush()
}

func runBindingRemove(args []string) error {
	fs := flag.NewFlagSet("binding rm", flag.ExitOnError)
	dsn := addDBFlag(fs)
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 1 {
		return fmt.Errorf("用法: magicd binding rm <账号名@房间号>")
	}
	name, room, err := parseBindingRef(fs.Arg(0))
	if err != nil {
		return err
	}

	ctx := context.Background()
	s, err := openStoreChecked(ctx, *dsn)
	if err != nil {
		return err
	}
	defer s.Close()

	if err := s.DeleteBinding(ctx, name, room); err != nil {
		return err
	}
	fmt.Printf("已删除绑定 %s@%s 及其规则\n", name, room)
	return nil
}

func runBindingSetEnabled(args []string, enabled bool) error {
	verb := "停用"
	if enabled {
		verb = "启用"
	}
	fs := flag.NewFlagSet("binding "+verb, flag.ExitOnError)
	dsn := addDBFlag(fs)
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 1 {
		return fmt.Errorf("用法: magicd binding enable|disable <账号名@房间号>")
	}
	name, room, err := parseBindingRef(fs.Arg(0))
	if err != nil {
		return err
	}

	ctx := context.Background()
	s, err := openStoreChecked(ctx, *dsn)
	if err != nil {
		return err
	}
	defer s.Close()

	if err := s.SetBindingEnabled(ctx, name, room, enabled); err != nil {
		return err
	}
	fmt.Printf("已%s绑定 %s@%s\n", verb, name, room)
	return nil
}

// runAccountList 列出全部 B 站账号。不打印 Cookie。
func runAccountList(args []string) error {
	fs := flag.NewFlagSet("account list", flag.ExitOnError)
	dsn := addDBFlag(fs)
	if err := fs.Parse(args); err != nil {
		return err
	}

	ctx := context.Background()
	s, err := openStoreChecked(ctx, *dsn)
	if err != nil {
		return err
	}
	defer s.Close()

	as, err := s.ListAccounts(ctx)
	if err != nil {
		return err
	}
	if len(as) == 0 {
		fmt.Println("还没有任何 B 站账号。运行 magicd login --save <账号名> --owner <用户名>。")
		return nil
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "账号名\tUID\t发送间隔\t字数上限")
	for _, a := range as {
		// 不打印 Cookie：它等同于账号密码，不该出现在终端回滚缓冲里
		fmt.Fprintf(w, "%s\t%s\t%v\t%d\n", a.Name, a.UID, a.RateLimit, a.MaxLength)
	}
	return w.Flush()
}

// runAccount 分发 account 的子命令。
func runAccount(args []string) error {
	if len(args) == 0 || args[0] != "list" {
		fmt.Fprintln(os.Stderr, "用法: magicd account list")
		return fmt.Errorf("account 目前只支持 list 子命令")
	}
	return runAccountList(args[1:])
}
```

- [ ] **Step 7: 实现 grant 与 revoke**

创建 `server/cmd/magicd/grant.go`：

```go
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strings"
	"text/tabwriter"

	"github.com/MrZhongzq/MagicalDanmaku/server/internal/perm"
)

// runGrant 授予某用户对某绑定的权限点。
func runGrant(args []string) error {
	fs := flag.NewFlagSet("grant", flag.ExitOnError)
	dsn := addDBFlag(fs)
	list := fs.Bool("list", false, "列出全部合法权限点后退出")
	if err := fs.Parse(args); err != nil {
		return err
	}

	if *list {
		fmt.Println("合法的权限点：")
		for _, p := range perm.All() {
			fmt.Printf("  %s\n", p)
		}
		return nil
	}

	if fs.NArg() != 3 {
		return fmt.Errorf("用法: magicd grant <用户名> <账号名@房间号> <权限点,...>\n" +
			"权限点清单: magicd grant -list")
	}
	username := fs.Arg(0)
	accName, roomID, err := parseBindingRef(fs.Arg(1))
	if err != nil {
		return err
	}
	ps, err := perm.ParseList(fs.Arg(2))
	if err != nil {
		return err
	}

	ctx := context.Background()
	s, err := openStoreChecked(ctx, *dsn)
	if err != nil {
		return err
	}
	defer s.Close()

	if err := s.Grant(ctx, username, accName, roomID, ps); err != nil {
		return err
	}
	// 说明是替换而非累加，避免用户以为原有权限还在
	fmt.Printf("%s 在 %s@%s 上的权限已设为: %s\n",
		username, accName, roomID, strings.Join(perm.Strings(ps), ", "))
	return nil
}

// runRevoke 撤销某用户对某绑定的全部权限。
func runRevoke(args []string) error {
	fs := flag.NewFlagSet("revoke", flag.ExitOnError)
	dsn := addDBFlag(fs)
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 2 {
		return fmt.Errorf("用法: magicd revoke <用户名> <账号名@房间号>")
	}
	username := fs.Arg(0)
	accName, roomID, err := parseBindingRef(fs.Arg(1))
	if err != nil {
		return err
	}

	ctx := context.Background()
	s, err := openStoreChecked(ctx, *dsn)
	if err != nil {
		return err
	}
	defer s.Close()

	if err := s.Revoke(ctx, username, accName, roomID); err != nil {
		return err
	}
	fmt.Printf("已撤销 %s 在 %s@%s 上的全部权限\n", username, accName, roomID)
	return nil
}

// runPerms 列出某用户已有的全部授权。
func runPerms(args []string) error {
	fs := flag.NewFlagSet("perms", flag.ExitOnError)
	dsn := addDBFlag(fs)
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 1 {
		return fmt.Errorf("用法: magicd perms <用户名>")
	}

	ctx := context.Background()
	s, err := openStoreChecked(ctx, *dsn)
	if err != nil {
		return err
	}
	defer s.Close()

	u, err := s.GetUserByName(ctx, fs.Arg(0))
	if err != nil {
		return err
	}
	if u.IsAdmin {
		fmt.Printf("%s 是管理员，对全部绑定拥有全部权限\n", u.Username)
		return nil
	}

	ms, err := s.ListMemberships(ctx, u.Username)
	if err != nil {
		return err
	}
	if len(ms) == 0 {
		fmt.Printf("%s 还没有任何授权\n", u.Username)
		return nil
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "绑定\t权限点")
	for _, m := range ms {
		fmt.Fprintf(w, "%s@%s\t%s\n",
			m.AccountName, m.RoomID, strings.Join(perm.Strings(m.Permissions), ", "))
	}
	return w.Flush()
}

// runCan 检查某用户对某绑定是否拥有某个权限点。
//
// 排障用：「为什么李四改不了规则」这类问题，直接问一句比对着
// perms 的输出人肉比对可靠。
func runCan(args []string) error {
	fs := flag.NewFlagSet("can", flag.ExitOnError)
	dsn := addDBFlag(fs)
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 3 {
		return fmt.Errorf("用法: magicd can <用户名> <账号名@房间号> <权限点>")
	}
	accName, roomID, err := parseBindingRef(fs.Arg(1))
	if err != nil {
		return err
	}
	p, err := perm.Parse(fs.Arg(2))
	if err != nil {
		return err
	}

	ctx := context.Background()
	s, err := openStoreChecked(ctx, *dsn)
	if err != nil {
		return err
	}
	defer s.Close()

	u, err := s.GetUserByName(ctx, fs.Arg(0))
	if err != nil {
		return err
	}
	b, err := s.GetBinding(ctx, accName, roomID)
	if err != nil {
		return err
	}

	ok, err := s.Can(ctx, u.ID, b.ID, p)
	if err != nil {
		return err
	}
	if ok {
		reason := ""
		if u.IsAdmin {
			reason = "（管理员绕过全部检查）"
		}
		fmt.Printf("是：%s 在 %s 上拥有 %s%s\n", u.Username, b.Label(), p, reason)
		return nil
	}
	fmt.Printf("否：%s 在 %s 上没有 %s\n", u.Username, b.Label(), p)
	fmt.Printf("授予：magicd grant %s %s %s\n", u.Username, b.Label(), p)
	return nil
}
```

- [ ] **Step 8: 更新 main.go**

修改 `server/cmd/magicd/main.go`：把 `usage` 常量整体替换，并在 `main` 里装配系统日志、新增子命令分发。

```go
// Command magicd 是神奇弹幕的服务端可执行文件。
//
// 配置的唯一真相是 PostgreSQL。YAML 只是导入入口，run 不读它。
package main

import (
	"fmt"
	"os"

	"github.com/MrZhongzq/MagicalDanmaku/server/internal/buildinfo"
	"github.com/MrZhongzq/MagicalDanmaku/server/internal/logging"
)

const usage = `magicd —— 神奇弹幕服务端

配置存在 PostgreSQL 里。先设置连接串：
  export MAGICD_DATABASE_URL='postgres://magicd:magicd@localhost:5432/magicd?sslmode=disable'

初次使用:
  magicd migrate                              建表，并在空库上创建管理员
  magicd login --save 小号 --owner admin       扫码登录一个 B 站账号并入库
  magicd binding add 小号 1706666491           让这个账号连接一个直播间
  magicd import -c config.yaml --owner admin   或者：直接导入现成的 YAML
  magicd run                                   启动机器人

用户与授权:
  magicd user add <用户名> [--admin]           创建用户
  magicd user passwd <用户名>                  修改密码
  magicd user list                             列出用户
  magicd grant <用户名> <账号名@房间号> <权限点,...>
  magicd revoke <用户名> <账号名@房间号>
  magicd perms <用户名>                        查看某人的授权
  magicd can <用户名> <账号名@房间号> <权限点>   检查某人有没有某个权限
  magicd grant -list                           列出全部权限点

账号与绑定:
  magicd account list                          列出 B 站账号
  magicd binding add <账号名> <房间号>
  magicd binding list
  magicd binding rm|enable|disable <账号名@房间号>

排障:
  magicd login [-o cookie.txt]                 扫码登录，Cookie 写文件（YAML 路径用）
  magicd probe -room <房间号> [-cookie-file cookie.txt] [-type <事件类型>]
                                [-dump <CMD名>] [-dump-file dump.jsonl]
        连接直播间并打印实时事件流；-dump 可把指定 CMD 的原始 JSON 落盘
  magicd version                               显示版本信息

环境变量:
  MAGICD_DATABASE_URL        PostgreSQL 连接串
  MAGICD_LOG_LEVEL           debug / info / warn / error，默认 info
  MAGICD_LOG_FILE            系统日志文件路径，留空则只写 stderr
  MAGICD_LOG_RETENTION_DAYS  业务日志保留天数，默认 30，0 表示不清理
`

func main() {
	if len(os.Args) < 2 {
		fmt.Fprint(os.Stderr, usage)
		os.Exit(2)
	}

	// 系统日志在分发子命令之前装配好，这样连接失败之类的错误也有去处
	closer, err := logging.SetupSystem(logging.SystemOptionsFromEnv())
	if err != nil {
		fmt.Fprintf(os.Stderr, "错误: %v\n", err)
		os.Exit(1)
	}
	defer func() { _ = closer.Close() }()

	switch os.Args[1] {
	case "login":
		err = runLogin(os.Args[2:])
	case "probe":
		err = runProbe(os.Args[2:])
	case "run":
		err = runRun(os.Args[2:])
	case "migrate":
		err = runMigrate(os.Args[2:])
	case "import":
		err = runImport(os.Args[2:])
	case "user":
		err = runUser(os.Args[2:])
	case "account":
		err = runAccount(os.Args[2:])
	case "binding":
		err = runBinding(os.Args[2:])
	case "grant":
		err = runGrant(os.Args[2:])
	case "revoke":
		err = runRevoke(os.Args[2:])
	case "perms":
		err = runPerms(os.Args[2:])
	case "can":
		err = runCan(os.Args[2:])
	case "version", "-v", "--version":
		fmt.Println(buildinfo.Get().Detail())
		return
	case "-h", "--help", "help":
		fmt.Print(usage)
		return
	default:
		fmt.Fprintf(os.Stderr, "未知的子命令: %s\n\n%s", os.Args[1], usage)
		os.Exit(2)
	}

	if err != nil {
		fmt.Fprintf(os.Stderr, "错误: %v\n", err)
		os.Exit(1)
	}
}
```

`main.go` 引用了 `runImport`（Task 16）与改签名的 `runRun`（Task 18）。为了让本任务能独立编译通过，先在 `server/cmd/magicd/import.go` 里放一个占位实现，Task 16 再把它换掉：

```go
package main

import "fmt"

// runImport 在 Task 16 实现。
func runImport([]string) error {
	return fmt.Errorf("import 子命令尚未实现")
}
```

- [ ] **Step 9: 跑测试与手工验证**

```bash
cd server; go build ./... ; go test ./cmd/magicd/ -v -run 'ParseBindingRef|OpenStore' 2>&1 | tail -20; echo "退出码=$?"
```

手工走一遍：

```bash
export MAGICD_DATABASE_URL='postgres://magicd:magicd@localhost:5433/magicd?sslmode=disable'
cd server; go run ./cmd/magicd migrate; echo "退出码=$?"
cd server; go run ./cmd/magicd user list; echo "退出码=$?"
cd server; go run ./cmd/magicd grant -list; echo "退出码=$?"
cd server; go run ./cmd/magicd binding list; echo "退出码=$?"
cd server; go run ./cmd/magicd perms admin; echo "退出码=$?"
```

预期：`migrate` 打印 schema 版本与管理员的一次性密码；`user list` 显示 admin；`grant -list` 列出七个权限点；`binding list` 提示还没有绑定；`perms admin` 说明管理员对全部绑定拥有全部权限。

再验证版本检查确实拦得住：

```bash
cd server; go run ./cmd/magicd user list -db 'postgres://magicd:magicd@localhost:5433/postgres?sslmode=disable' 2>&1 | tail -3; echo "退出码=$?"
```

预期：报错提示先运行 `magicd migrate`（那个库里没建过表）。

- [ ] **Step 10: 提交**

```bash
cd server; gofmt -l . ; go vet ./... ; echo "退出码=$?"
git add server/cmd/magicd/ server/go.mod server/go.sum
git commit -m "$(cat <<'EOF'
feat: 新增数据库管理类子命令

migrate / user / account / binding / grant / revoke / perms / can。

除 migrate 外都走 openStoreChecked：schema 版本落后就拒绝执行而非
自动迁移——多实例部署下让每个实例各自决定何时改表是危险的。

account list 不打印 Cookie：它等同于账号密码，不该留在终端回滚缓冲里。
grant 的成功提示写「已设为」而非「已授予」，因为它是替换而非累加。

设计文档 §9 的清单里没有 binding 子命令，但缺了它，扫码登录新账号后
想加个直播间就得先写一份 YAML，不合理，故补上。

Co-Authored-By: Claude Opus 5 <noreply@anthropic.com>
EOF
)"
```

---

## Task 16: import 子命令

**Files:**
- Modify: `server/cmd/magicd/import.go`（替换 Task 15 的占位实现）
- Create: `server/cmd/magicd/import_test.go`
- Create: `server/internal/store/import.go`
- Create: `server/internal/store/import_test.go`

**Interfaces:**
- Consumes: `config.Load`、`config.Config`（Task 2）；`spec.Rule`；`store.UpsertAccount`、`UpsertBinding`、`SetCooldownGroups`、`ReplaceRules`、`GetUserByName`
- Produces:
  - `type store.ImportResult struct { Accounts, Bindings, Rules int }`
  - `func (s *Store) ImportConfig(ctx context.Context, ownerID int64, accounts []ImportAccount) (*ImportResult, error)`
  - `type store.ImportAccount struct { Name, Cookie string; RateLimit time.Duration; MaxLength int; Rooms []ImportRoom }`
  - `type store.ImportRoom struct { RoomID string; CooldownGroups map[string]time.Duration; Rules []spec.Rule }`

导入的 YAML 里规则已经是 `spec.Rule`，但 `config.Parse` 返回的是转换过的 `rules.Rule`。为避免反向转换，**`import` 自己再解析一次 YAML 拿 `spec.Config`**：`config.Load` 负责校验配置树的形状并报出友好错误，`spec.Config` 提供原始的规则序列化形式。两次解析的代价可以忽略，换来的是不必写 `rules.Rule → spec.Rule` 的反向转换（那会是第二处字段展开，正是要避免的）。

- [ ] **Step 1: 写 store 侧的失败测试**

创建 `server/internal/store/import_test.go`：

```go
package store

import (
	"context"
	"testing"
	"time"

	"github.com/MrZhongzq/MagicalDanmaku/server/internal/rules/spec"
)

func sampleImport() []ImportAccount {
	return []ImportAccount{{
		Name: "主播号", Cookie: "SESSDATA=a", RateLimit: 1500 * time.Millisecond, MaxLength: 40,
		Rooms: []ImportRoom{{
			RoomID:         "1706666491",
			CooldownGroups: map[string]time.Duration{"moderation": 5 * time.Second},
			Rules: []spec.Rule{
				{Name: "礼物流水", On: []string{"gift"}, Do: []spec.Action{{Type: "log"}}},
			},
		}},
	}, {
		Name: "小号", Cookie: "SESSDATA=b", RateLimit: 1500 * time.Millisecond,
		Rooms: []ImportRoom{{
			RoomID: "1706666491",
			Rules: []spec.Rule{
				{Name: "进场欢迎", On: []string{"user_enter"},
					Do: []spec.Action{{Type: "danmaku", Template: []string{"欢迎"}}}},
				{Name: "礼物答谢", On: []string{"gift"},
					Do: []spec.Action{{Type: "danmaku", Template: []string{"谢谢"}}}},
			},
		}, {
			RoomID: "22222222",
			Rules: []spec.Rule{
				{Name: "打招呼", On: []string{"user_enter"},
					Do: []spec.Action{{Type: "danmaku", Template: []string{"你好"}}}},
			},
		}},
	}}
}

func TestImportConfigCreatesEverything(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	owner := mustUser(t, s, "张三")

	res, err := s.ImportConfig(ctx, owner, sampleImport())
	if err != nil {
		t.Fatalf("导入报错: %v", err)
	}
	if res.Accounts != 2 || res.Bindings != 3 || res.Rules != 4 {
		t.Errorf("统计 = %+v, 期望 2 账号 / 3 绑定 / 4 规则", res)
	}

	cfgs, err := s.LoadRunConfig(ctx)
	if err != nil {
		t.Fatalf("载入运行配置报错: %v", err)
	}
	if len(cfgs) != 3 {
		t.Fatalf("运行单元数 = %d, 期望 3", len(cfgs))
	}
}

// 同一份 YAML 导两次，结果必须一致
func TestImportConfigIsIdempotent(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	owner := mustUser(t, s, "张三")

	if _, err := s.ImportConfig(ctx, owner, sampleImport()); err != nil {
		t.Fatalf("首次导入报错: %v", err)
	}
	res, err := s.ImportConfig(ctx, owner, sampleImport())
	if err != nil {
		t.Fatalf("二次导入报错: %v", err)
	}
	if res.Accounts != 2 || res.Bindings != 3 || res.Rules != 4 {
		t.Errorf("二次导入统计 = %+v", res)
	}

	as, err := s.ListAccounts(ctx)
	if err != nil {
		t.Fatalf("列出账号报错: %v", err)
	}
	if len(as) != 2 {
		t.Errorf("账号数 = %d, 期望 2（不该翻倍）", len(as))
	}
	bs, err := s.ListBindings(ctx)
	if err != nil {
		t.Fatalf("列出绑定报错: %v", err)
	}
	if len(bs) != 3 {
		t.Errorf("绑定数 = %d, 期望 3（不该翻倍）", len(bs))
	}
}

// YAML 里删掉的规则，重新导入后库里也该没有
func TestImportConfigDropsRemovedRules(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	owner := mustUser(t, s, "张三")

	if _, err := s.ImportConfig(ctx, owner, sampleImport()); err != nil {
		t.Fatalf("首次导入报错: %v", err)
	}

	trimmed := sampleImport()
	trimmed[1].Rooms[0].Rules = trimmed[1].Rooms[0].Rules[:1] // 小号删掉「礼物答谢」
	if _, err := s.ImportConfig(ctx, owner, trimmed); err != nil {
		t.Fatalf("二次导入报错: %v", err)
	}

	b, err := s.GetBinding(ctx, "小号", "1706666491")
	if err != nil {
		t.Fatalf("查询绑定报错: %v", err)
	}
	rs, err := s.ListRules(ctx, b.ID)
	if err != nil {
		t.Fatalf("列出规则报错: %v", err)
	}
	if len(rs) != 1 || rs[0].Name != "进场欢迎" {
		t.Errorf("删掉的规则应消失，实际 %+v", rs)
	}
}

// 导入不该把用户手动停用的绑定又打开
func TestImportConfigPreservesDisabledBinding(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	owner := mustUser(t, s, "张三")

	if _, err := s.ImportConfig(ctx, owner, sampleImport()); err != nil {
		t.Fatalf("首次导入报错: %v", err)
	}
	if err := s.SetBindingEnabled(ctx, "小号", "22222222", false); err != nil {
		t.Fatalf("停用绑定报错: %v", err)
	}
	if _, err := s.ImportConfig(ctx, owner, sampleImport()); err != nil {
		t.Fatalf("二次导入报错: %v", err)
	}

	b, err := s.GetBinding(ctx, "小号", "22222222")
	if err != nil {
		t.Fatalf("查询绑定报错: %v", err)
	}
	if b.Enabled {
		t.Error("导入不该把手动停用的绑定又打开")
	}
}

func TestImportConfigRejectsInvalidRule(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	owner := mustUser(t, s, "张三")

	bad := sampleImport()
	bad[0].Rooms[0].Rules[0].On = []string{"没有这种事件"}

	if _, err := s.ImportConfig(ctx, owner, bad); err == nil {
		t.Fatal("含非法规则的导入应失败")
	}
}

func TestImportConfigSetsCooldownGroups(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	owner := mustUser(t, s, "张三")

	if _, err := s.ImportConfig(ctx, owner, sampleImport()); err != nil {
		t.Fatalf("导入报错: %v", err)
	}

	b, err := s.GetBinding(ctx, "主播号", "1706666491")
	if err != nil {
		t.Fatalf("查询绑定报错: %v", err)
	}
	g, err := s.CooldownGroups(ctx, b.ID)
	if err != nil {
		t.Fatalf("读取冷却组报错: %v", err)
	}
	if g["moderation"] != 5*time.Second {
		t.Errorf("冷却组 = %v", g)
	}
}
```

- [ ] **Step 2: 跑测试确认失败**

```bash
cd server; go test ./internal/store/ -run TestImportConfig 2>&1 | tail -5; echo "退出码=$?"
```

- [ ] **Step 3: 实现 store 侧**

创建 `server/internal/store/import.go`：

```go
package store

import (
	"context"
	"fmt"
	"time"

	"github.com/MrZhongzq/MagicalDanmaku/server/internal/rules/spec"
)

// ImportAccount 是待导入的一个账号及其直播间。
type ImportAccount struct {
	Name      string
	UID       string
	Cookie    string
	RateLimit time.Duration
	MaxLength int
	Rooms     []ImportRoom
}

// ImportRoom 是待导入的一个直播间配置。
type ImportRoom struct {
	RoomID         string
	CooldownGroups map[string]time.Duration
	Rules          []spec.Rule
}

// ImportResult 是导入的统计。
type ImportResult struct {
	Accounts int
	Bindings int
	Rules    int
}

// ImportConfig 把配置导入数据库，按名字 upsert。
//
// 幂等：同一份 YAML 导两次结果一致。规则整组替换，因此 YAML 里删掉的
// 规则重新导入后库里也会消失；绑定的 enabled 不动，导入不该把用户手动
// 停用的绑定又打开。
func (s *Store) ImportConfig(ctx context.Context, ownerID int64, accounts []ImportAccount) (*ImportResult, error) {
	res := &ImportResult{}

	for _, a := range accounts {
		acc, err := s.UpsertAccount(ctx, AccountInput{
			Name:      a.Name,
			UID:       a.UID,
			Cookie:    a.Cookie,
			RateLimit: a.RateLimit,
			MaxLength: a.MaxLength,
			OwnerID:   ownerID,
		})
		if err != nil {
			return nil, err
		}
		res.Accounts++

		for _, r := range a.Rooms {
			b, err := s.UpsertBinding(ctx, acc.ID, r.RoomID)
			if err != nil {
				return nil, err
			}
			res.Bindings++

			if err := s.SetCooldownGroups(ctx, b.ID, r.CooldownGroups); err != nil {
				return nil, err
			}
			if err := s.ReplaceRules(ctx, b.ID, r.Rules); err != nil {
				return nil, fmt.Errorf("导入 %s 的规则失败: %w", b.Label(), err)
			}
			res.Rules += len(r.Rules)
		}
	}
	return res, nil
}
```

- [ ] **Step 4: 实现 import 子命令**

替换 `server/cmd/magicd/import.go` 的全部内容：

```go
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"gopkg.in/yaml.v3"

	"github.com/MrZhongzq/MagicalDanmaku/server/internal/rules/config"
	"github.com/MrZhongzq/MagicalDanmaku/server/internal/rules/spec"
	"github.com/MrZhongzq/MagicalDanmaku/server/internal/store"
)

// runImport 把 YAML 配置导入数据库。
//
// 数据库是唯一真相，YAML 只是导入入口。run 不读 YAML。
func runImport(args []string) error {
	fs := flag.NewFlagSet("import", flag.ExitOnError)
	dsn := addDBFlag(fs)
	cfgPath := fs.String("c", "", "YAML 配置文件路径（必填）")
	owner := fs.String("owner", "", "导入的账号归属于哪个用户（必填）")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *cfgPath == "" {
		return fmt.Errorf("必须用 -c 指定配置文件")
	}
	if *owner == "" {
		return fmt.Errorf("必须用 -owner 指定账号归属的用户，例如 -owner admin")
	}

	// 先用 config.Load 走一遍完整校验：它会把「账号名重复」「未知事件类型」
	// 之类的问题报成人能读懂的错误，而不是等写库时才炸。
	if _, err := config.Load(*cfgPath); err != nil {
		return err
	}

	// 再解析一次拿规则的原始序列化形式。规则要以 spec.Rule 入库，而
	// config.Load 返回的是转换后的领域模型；写一个反向转换会成为第二处
	// 字段展开，正是要避免的。多解析一次的代价可以忽略。
	raw, err := os.ReadFile(*cfgPath)
	if err != nil {
		return fmt.Errorf("读取配置文件 %s 失败: %w", *cfgPath, err)
	}
	var sc spec.Config
	if err := yaml.Unmarshal(raw, &sc); err != nil {
		return fmt.Errorf("解析配置文件 %s 失败: %w", *cfgPath, err)
	}

	accounts := make([]store.ImportAccount, 0, len(sc.Accounts))
	for _, a := range sc.Accounts {
		cookie, err := readCookieFile(a.CookieFile)
		if err != nil {
			return fmt.Errorf("账号 %q: %w", a.Name, err)
		}

		rooms := make([]store.ImportRoom, 0, len(a.Rooms))
		for _, r := range a.Rooms {
			groups := make(map[string]time.Duration, len(r.CooldownGroups))
			for k, v := range r.CooldownGroups {
				groups[k] = time.Duration(v)
			}
			rooms = append(rooms, store.ImportRoom{
				RoomID:         r.ID,
				CooldownGroups: groups,
				Rules:          r.Rules,
			})
		}

		accounts = append(accounts, store.ImportAccount{
			Name:      a.Name,
			Cookie:    cookie,
			RateLimit: time.Duration(a.RateLimit),
			MaxLength: a.MaxLength,
			Rooms:     rooms,
		})
	}

	ctx := context.Background()
	s, err := openStoreChecked(ctx, *dsn)
	if err != nil {
		return err
	}
	defer s.Close()

	u, err := s.GetUserByName(ctx, *owner)
	if err != nil {
		return err
	}

	res, err := s.ImportConfig(ctx, u.ID, accounts)
	if err != nil {
		return err
	}

	fmt.Printf("已导入 %d 个账号、%d 个绑定、%d 条规则，归属用户 %s\n",
		res.Accounts, res.Bindings, res.Rules, u.Username)
	fmt.Println("数据库现在是配置的唯一真相，magicd run 不再需要这份 YAML。")
	return nil
}

// readCookieFile 读出 Cookie 文件的内容。
func readCookieFile(path string) (string, error) {
	if path == "" {
		return "", fmt.Errorf("配置里没写 cookieFile")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("读取 Cookie 文件 %s 失败: %w", path, err)
	}
	cookie := strings.TrimSpace(string(data))
	if cookie == "" {
		return "", fmt.Errorf("Cookie 文件 %s 是空的", path)
	}
	return cookie, nil
}
```

- [ ] **Step 5: 写 import 子命令的失败测试**

创建 `server/cmd/magicd/import_test.go`：

```go
package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestImportRequiresConfigPath(t *testing.T) {
	err := runImport([]string{"-owner", "admin"})
	if err == nil {
		t.Fatal("缺少 -c 应报错")
	}
	if !strings.Contains(err.Error(), "-c") {
		t.Errorf("错误信息应提到 -c，实际: %v", err)
	}
}

func TestImportRequiresOwner(t *testing.T) {
	err := runImport([]string{"-c", "x.yaml"})
	if err == nil {
		t.Fatal("缺少 -owner 应报错")
	}
	if !strings.Contains(err.Error(), "owner") {
		t.Errorf("错误信息应提到 -owner，实际: %v", err)
	}
}

func TestImportRejectsMissingConfigFile(t *testing.T) {
	err := runImport([]string{"-c", "/不存在的路径/config.yaml", "-owner", "admin"})
	if err == nil {
		t.Fatal("配置文件不存在应报错")
	}
}

// 配置校验要在连数据库之前完成：配置写错就该立刻报错，
// 而不是等连上库才发现
func TestImportValidatesConfigBeforeConnecting(t *testing.T) {
	t.Setenv("MAGICD_DATABASE_URL", "")

	dir := t.TempDir()
	cfg := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(cfg, []byte(`
accounts:
  - name: 小号
    cookieFile: cookie.txt
    rooms:
      - id: "123"
        rules:
          - name: 坏规则
            on: [没有这种事件]
            do:
              - type: log
`), 0o600); err != nil {
		t.Fatalf("写配置文件报错: %v", err)
	}

	err := runImport([]string{"-c", cfg, "-owner", "admin"})
	if err == nil {
		t.Fatal("非法配置应报错")
	}
	if strings.Contains(err.Error(), "MAGICD_DATABASE_URL") {
		t.Errorf("应先报配置错误而非数据库未配置，实际: %v", err)
	}
}

func TestReadCookieFileRejectsEmpty(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "cookie.txt")
	if err := os.WriteFile(path, []byte("   \n"), 0o600); err != nil {
		t.Fatalf("写文件报错: %v", err)
	}
	if _, err := readCookieFile(path); err == nil {
		t.Error("空 Cookie 文件应报错")
	}
}

func TestReadCookieFileTrimsWhitespace(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "cookie.txt")
	if err := os.WriteFile(path, []byte("  SESSDATA=abc\n"), 0o600); err != nil {
		t.Fatalf("写文件报错: %v", err)
	}
	got, err := readCookieFile(path)
	if err != nil {
		t.Fatalf("读取报错: %v", err)
	}
	if got != "SESSDATA=abc" {
		t.Errorf("= %q", got)
	}
}
```

- [ ] **Step 6: 跑测试并手工验证**

```bash
cd server; go test ./internal/store/ -run TestImportConfig -v 2>&1 | tail -25; echo "退出码=$?"
cd server; go test ./cmd/magicd/ -run 'Import|CookieFile' -v 2>&1 | tail -25; echo "退出码=$?"
```

用真实的 `config.example.yaml` 走一遍（需要两个 Cookie 文件）：

```bash
export MAGICD_DATABASE_URL='postgres://magicd:magicd@localhost:5433/magicd?sslmode=disable'
cd server; printf 'SESSDATA=fake; bili_jct=fake; DedeUserID=1' > /tmp/cookie-main.txt
cd server; cp /tmp/cookie-main.txt /tmp/cookie-sub.txt
cd server; sed -e 's#cookie-main.txt#/tmp/cookie-main.txt#' -e 's#cookie-sub.txt#/tmp/cookie-sub.txt#' ../config.example.yaml > /tmp/config.yaml
cd server; go run ./cmd/magicd import -c /tmp/config.yaml -owner admin; echo "退出码=$?"
cd server; go run ./cmd/magicd binding list; echo "退出码=$?"
cd server; go run ./cmd/magicd import -c /tmp/config.yaml -owner admin; echo "退出码=$?"
cd server; go run ./cmd/magicd binding list; echo "退出码=$?"
```

预期：两次 `import` 的统计一致，两次 `binding list` 的行数也一致（幂等）。

清理测试留下的假 Cookie：

```bash
rm -f /tmp/cookie-main.txt /tmp/cookie-sub.txt /tmp/config.yaml; echo "退出码=$?"
```

- [ ] **Step 7: 提交**

```bash
cd server; gofmt -l . ; go vet ./... ; echo "退出码=$?"
git add server/cmd/magicd/ server/internal/store/
git commit -m "$(cat <<'EOF'
feat: 新增 import 子命令，把 YAML 导入数据库

先用 config.Load 走一遍完整校验，让「账号名重复」「未知事件类型」
之类的问题报成人能读懂的错误；再解析一次拿规则的原始序列化形式。
写一个 rules.Rule → spec.Rule 的反向转换会成为第二处字段展开，
正是要避免的，多解析一次的代价可以忽略。

导入是幂等 upsert：同一份 YAML 导两次结果一致。规则整组替换，因此
YAML 里删掉的规则会消失；绑定的 enabled 不动，导入不该把用户手动
停用的绑定又打开。

Co-Authored-By: Claude Opus 5 <noreply@anthropic.com>
EOF
)"
```

---

## Task 17: login 直接入库

**Files:**
- Modify: `server/cmd/magicd/login.go`
- Create: `server/cmd/magicd/login_test.go`

**Interfaces:**
- Consumes: `auth.NewQRLogin`、`auth.ParseSession`（既有）；`store.CreateAccount`、`UpdateAccountCookie`、`GetUserByName`、`GetAccountByName`、`ErrNotFound`
- Produces: `runLogin` 支持 `--save <账号名> --owner <用户名>`；`-o` 与直接输出的行为保持不变

- [ ] **Step 1: 写失败的测试**

创建 `server/cmd/magicd/login_test.go`：

```go
package main

import (
	"strings"
	"testing"
)

func TestLoginSaveRequiresOwnerWhenAccountIsNew(t *testing.T) {
	// 参数校验必须在扫码之前完成：让人扫完码才发现参数错了最气人
	err := runLogin([]string{"--save", "小号", "-o", "x.txt"})
	if err == nil {
		t.Fatal("--save 与 -o 同时给出应报错")
	}
	if !strings.Contains(err.Error(), "-o") {
		t.Errorf("错误信息应说明冲突，实际: %v", err)
	}
}

func TestLoginOwnerWithoutSaveIsRejected(t *testing.T) {
	err := runLogin([]string{"--owner", "admin"})
	if err == nil {
		t.Fatal("只给 --owner 不给 --save 应报错")
	}
	if !strings.Contains(err.Error(), "--save") {
		t.Errorf("错误信息应提到 --save，实际: %v", err)
	}
}
```

- [ ] **Step 2: 跑测试确认失败**

```bash
cd server; go test ./cmd/magicd/ -run TestLogin 2>&1 | tail -5; echo "退出码=$?"
```

预期：两个测试都失败——当前的 `runLogin` 会直接开始扫码。

- [ ] **Step 3: 实现**

修改 `server/cmd/magicd/login.go`。在文件顶部的 import 里加上 `"errors"` 与 `store` 包，然后把 `runLogin` 改成：

```go
// runLogin 执行扫码登录流程。
//
// 两种去处：--save 直接写进数据库（推荐），-o 写文件（YAML 导入路径用）。
func runLogin(args []string) error {
	fs := flag.NewFlagSet("login", flag.ExitOnError)
	out := fs.String("o", "", "把 Cookie 写入指定文件；留空则打印到标准输出")
	save := fs.String("save", "", "把 Cookie 直接存进数据库，值为账号名")
	owner := fs.String("owner", "", "新账号归属的用户名，与 --save 搭配")
	dsn := addDBFlag(fs)
	if err := fs.Parse(args); err != nil {
		return err
	}

	// 参数校验放在扫码之前：让人扫完码才发现参数错了最气人
	if *save != "" && *out != "" {
		return fmt.Errorf("--save 与 -o 不能同时使用：前者存数据库，后者写文件，选一个")
	}
	if *owner != "" && *save == "" {
		return fmt.Errorf("--owner 要与 --save 搭配使用")
	}

	// 数据库也先连上再扫码，同理
	var st *store.Store
	if *save != "" {
		var err error
		st, err = openStoreChecked(context.Background(), *dsn)
		if err != nil {
			return err
		}
		defer st.Close()

		// 已存在的账号只换 Cookie，不需要 owner；新账号必须指定
		if _, err := st.GetAccountByName(context.Background(), *save); err != nil {
			if !errors.Is(err, store.ErrNotFound) {
				return err
			}
			if *owner == "" {
				return fmt.Errorf("账号 %q 还不存在，创建它需要用 --owner 指定归属的用户", *save)
			}
			if _, err := st.GetUserByName(context.Background(), *owner); err != nil {
				return err
			}
		}
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	l := auth.NewQRLogin(nil)
	qr, err := l.Generate(ctx)
	if err != nil {
		return err
	}

	fmt.Println("请用哔哩哔哩手机客户端扫描下方二维码：")
	fmt.Println()
	art, err := renderQR(qr.URL)
	if err != nil {
		// 渲染失败不阻断登录：地址本身仍可用。
		fmt.Println("（二维码渲染失败，请手动打开下面的地址）")
	} else {
		fmt.Print(art)
	}
	fmt.Println()
	fmt.Println("若终端显示错乱，可复制以下地址到手机浏览器或二维码生成器：")
	fmt.Println("   " + qr.URL)
	fmt.Println()
	fmt.Println("等待扫码中，按 Ctrl+C 取消...")

	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	last := auth.PollWaiting
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
		}

		res, err := l.Poll(ctx, qr.Key)
		if err != nil {
			return err
		}
		if res.Status != last {
			switch res.Status {
			case auth.PollScanned:
				fmt.Println("已扫码，请在手机上确认登录...")
			case auth.PollExpired:
				return fmt.Errorf("二维码已失效，请重新运行 login")
			}
			last = res.Status
		}
		if res.Status != auth.PollSuccess {
			continue
		}

		// 校验拿到的 Cookie 可用
		sess, err := auth.ParseSession(res.Cookie)
		if err != nil {
			return fmt.Errorf("登录成功但 Cookie 不完整: %w", err)
		}
		fmt.Printf("登录成功，UID=%s\n", sess.UID)

		switch {
		case st != nil:
			return saveAccountCookie(ctx, st, *save, *owner, res.Cookie, sess.UID)
		case *out == "":
			fmt.Println(res.Cookie)
			return nil
		default:
			// 0600 权限：Cookie 等同于账号密码，不得让同机其他用户读到。
			if err := os.WriteFile(*out, []byte(res.Cookie), 0o600); err != nil {
				return fmt.Errorf("写入 %s 失败: %w", *out, err)
			}
			fmt.Printf("Cookie 已写入 %s\n", *out)
			return nil
		}
	}
}

// saveAccountCookie 把 Cookie 存进数据库：账号已存在就换 Cookie，
// 否则新建。
func saveAccountCookie(ctx context.Context, st *store.Store, name, owner, cookie, uid string) error {
	if _, err := st.GetAccountByName(ctx, name); err == nil {
		if err := st.UpdateAccountCookie(ctx, name, cookie, uid); err != nil {
			return err
		}
		fmt.Printf("账号 %q 的 Cookie 已更新\n", name)
		return nil
	} else if !errors.Is(err, store.ErrNotFound) {
		return err
	}

	u, err := st.GetUserByName(ctx, owner)
	if err != nil {
		return err
	}
	if _, err := st.CreateAccount(ctx, store.AccountInput{
		Name: name, UID: uid, Cookie: cookie, OwnerID: u.ID,
	}); err != nil {
		return err
	}
	fmt.Printf("已创建账号 %q，归属用户 %s\n", name, u.Username)
	fmt.Printf("下一步：magicd binding add %s <房间号>\n", name)
	return nil
}
```

- [ ] **Step 4: 跑测试确认通过**

```bash
cd server; go build ./... ; go test ./cmd/magicd/ -run TestLogin -v 2>&1 | tail -15; echo "退出码=$?"
```

- [ ] **Step 5: 手工验证参数校验**

```bash
export MAGICD_DATABASE_URL='postgres://magicd:magicd@localhost:5433/magicd?sslmode=disable'
cd server; go run ./cmd/magicd login --save 新号 2>&1 | tail -3; echo "退出码=$?"
```

预期：**不进入扫码流程**，直接报错说新账号需要 `--owner`。

```bash
cd server; go run ./cmd/magicd login --save 新号 --owner 查无此人 2>&1 | tail -3; echo "退出码=$?"
```

预期：同样不扫码，直接报「用户不存在」。

- [ ] **Step 6: 提交**

```bash
cd server; gofmt -l . ; go vet ./... ; echo "退出码=$?"
git add server/cmd/magicd/
git commit -m "$(cat <<'EOF'
feat: login 支持直接把 Cookie 存进数据库

--save <账号名> 存库，-o 写文件（YAML 导入路径用），两者互斥。

全部参数校验与数据库连接都放在扫码之前：让人扫完码才发现参数写错
或者用户名不存在，最气人。

已存在的账号只换 Cookie 不需要 --owner，新账号必须指定归属。

Co-Authored-By: Claude Opus 5 <noreply@anthropic.com>
EOF
)"
```

---

## Task 18: run 改从数据库读

**Files:**
- Modify: `server/cmd/magicd/run.go`（整体改写装配部分）
- Modify: `server/cmd/magicd/run_test.go`（删掉四个基于 `-c` 的测试，补新的）

**Interfaces:**
- Consumes: `store.LoadRunConfig`、`store.BindingStorage`、`store.InsertActivity`、`store.PurgeActivityBefore`；`logging.NewActivityWriter`、`(*ActivityWriter).Sink`；`rules.EngineOptions.Activity`、`.Storage`
- Produces: `runRun` 不再接受 `-c`，接受 `-db`

- [ ] **Step 1: 改测试**

修改 `server/cmd/magicd/run_test.go`：**删除** `TestRunRejectsMissingConfig`、`TestRunRejectsEmptyConfigFlag`、`TestRunRejectsMissingCookieFile`、`TestRunRejectsInvalidCookie` 四个测试（`run` 不再读 YAML），**保留** `TestRoomBotForwardsToBinding`、`TestRoomBotPropagatesError` 与 `bindingStub`、`contains`。

删掉这四个测试后 `path/filepath`、`os` 等 import 可能变成未使用，一并清理。

在文件末尾追加：

```go
func TestRunRequiresDatabase(t *testing.T) {
	t.Setenv("MAGICD_DATABASE_URL", "")
	err := runRun([]string{})
	if err == nil {
		t.Fatal("没有数据库连接串应报错")
	}
	if !contains(err.Error(), "MAGICD_DATABASE_URL") {
		t.Errorf("错误信息应提示怎么配置，实际: %v", err)
	}
}

func TestRunRejectsUnreachableDatabase(t *testing.T) {
	// 端口 1 上不会有 PostgreSQL
	err := runRun([]string{"-db", "postgres://x:y@127.0.0.1:1/z?sslmode=disable&connect_timeout=1"})
	if err == nil {
		t.Fatal("连不上数据库应报错")
	}
}

func TestRetentionDaysFromEnv(t *testing.T) {
	t.Setenv("MAGICD_LOG_RETENTION_DAYS", "7")
	if got := retentionDays(); got != 7 {
		t.Errorf("= %d, 期望 7", got)
	}
}

func TestRetentionDaysDefault(t *testing.T) {
	t.Setenv("MAGICD_LOG_RETENTION_DAYS", "")
	if got := retentionDays(); got != 30 {
		t.Errorf("默认应为 30，实际 %d", got)
	}
}

func TestRetentionDaysZeroMeansNoPurge(t *testing.T) {
	t.Setenv("MAGICD_LOG_RETENTION_DAYS", "0")
	if got := retentionDays(); got != 0 {
		t.Errorf("0 表示不清理，实际 %d", got)
	}
}

func TestRetentionDaysIgnoresGarbage(t *testing.T) {
	// 环境变量写错就退回默认值，不该让机器人起不来
	t.Setenv("MAGICD_LOG_RETENTION_DAYS", "三十天")
	if got := retentionDays(); got != 30 {
		t.Errorf("非法值应退回默认 30，实际 %d", got)
	}
}
```

- [ ] **Step 2: 跑测试确认失败**

```bash
cd server; go test ./cmd/magicd/ -run 'TestRun|Retention' 2>&1 | tail -5; echo "退出码=$?"
```

预期：编译失败（`retentionDays` 未定义）。

- [ ] **Step 3: 改写 run.go**

`roomBot`、`danmakuSender`、`accountRuntime` 三个类型与 `defaultRateLimit` 常量保留不动。把 `runRun` 与 `buildAccounts` 换成下面这版，并调整 import：删掉 `strings` 与 `config`，加上 `strconv`、`logging`、`store`。

```go
// defaultRetentionDays 是业务日志的默认保留天数。
const defaultRetentionDays = 30

// runRun 从数据库加载配置并启动机器人。
func runRun(args []string) error {
	fs := flag.NewFlagSet("run", flag.ExitOnError)
	dsn := addDBFlag(fs)
	if err := fs.Parse(args); err != nil {
		return err
	}

	log := slog.Default()
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	st, err := openStoreChecked(ctx, *dsn)
	if err != nil {
		return err
	}
	defer st.Close()

	cfgs, err := st.LoadRunConfig(ctx)
	if err != nil {
		return err
	}
	if len(cfgs) == 0 {
		return fmt.Errorf("数据库里没有任何启用的绑定。\n" +
			"先 magicd login --save <账号名> --owner <用户名>，再 magicd binding add <账号名> <房间号>；\n" +
			"或者用 magicd import -c config.yaml --owner <用户名> 导入现成的配置")
	}

	// 业务日志：一个写入器，每个绑定分一个带归属 ID 的 Sink
	activity := logging.NewActivityWriter(logging.ActivityWriterOptions{
		Flush:  st.InsertActivity,
		Logger: log,
	})

	runtimes, err := buildAccounts(ctx, cfgs, log)
	if err != nil {
		activity.Close()
		return err
	}

	sched := scheduler.New(log)
	var wg sync.WaitGroup
	var engines []*rules.Engine

	// 清理放在 defer 里，而不是只写在正常关停路径的末尾。
	//
	// 装配循环里有多个早返回点（房间信息获取失败、规则非法、定时任务注册
	// 失败），而此时前面的绑定可能已经建好引擎并跑起来了。只在末尾清理的话，
	// 那些引擎的 Close() 永远不会被调用——后果不是日志被丢弃，而是那批日志行
	// 根本不会产生：Close() 才是结算未决合并窗口的地方，不调用它，攒着的
	// 欢迎语既不会发出去，也不会有对应的动作日志。
	//
	// 顺序仍是「引擎全部关闭之后才关业务日志写入器」：引擎 Close 时结算窗口
	// 会产生日志，先关写入器就捞不到了。
	defer func() {
		for _, e := range engines {
			e.Close()
		}
		activity.Close()
	}()

	for _, c := range cfgs {
		rt := runtimes[c.AccountName]

		// 解析真实房间号：配置里可能填的是短号
		info, err := rt.api.RoomInfo(ctx, c.RoomID)
		if err != nil {
			return fmt.Errorf("账号 %q 获取直播间 %s 信息失败: %w", c.AccountName, c.RoomID, err)
		}

		// 限流统一由 account.Binding 负责，这里传空限流器，
		// 否则与 Binding 的等待叠加会让实际间隔翻倍。
		actions := bilibili.NewActions(rt.api, ratelimit.NewInterval(0))
		if c.MaxLength > 0 {
			actions.SetMaxLength(c.MaxLength)
		}
		binding := &account.Binding{
			Account: rt.acc,
			RoomID:  info.RoomID,
			Actions: actions,
		}

		engine, err := rules.NewEngine(rules.EngineOptions{
			Label:          binding.Label(),
			RoomID:         info.RoomID,
			Rules:          c.Rules,
			Bot:            &roomBot{binding: binding, ctx: ctx},
			Storage:        st.BindingStorage(c.BindingID),
			Activity:       activity.Sink(c.AccountID, c.BindingID, info.RoomID),
			CooldownGroups: c.CooldownGroups,
			Logger:         log,
		})
		if err != nil {
			return fmt.Errorf("%s 的规则非法: %w", binding.Label(), err)
		}
		engines = append(engines, engine)

		// 注册该绑定的定时规则
		for _, r := range engine.ScheduledRules() {
			name, eng := r.Name, engine
			if err := sched.Add(r.Schedule, binding.Label()+"/"+name, func() {
				eng.FireScheduled(name)
			}); err != nil {
				return err
			}
		}

		status := "未开播"
		if info.IsLiving() {
			status = "直播中"
		}
		enabled := 0
		for _, r := range c.Rules {
			if r.Enabled {
				enabled++
			}
		}
		log.Info("已配置绑定",
			"binding", binding.Label(),
			"title", info.Title,
			"status", status,
			"rules", len(c.Rules),
			"enabled", enabled)

		client := bilibili.NewClient(info.RoomID, rt.api, bilibili.WithLogger(log))

		wg.Add(2)
		go func(eng *rules.Engine, c *bilibili.Client) {
			defer wg.Done()
			for ev := range c.Events() {
				eng.Handle(ev)
			}
		}(engine, client)

		go func(label string, c *bilibili.Client) {
			defer wg.Done()
			// 单个绑定的连接出错不影响其他绑定，也不做账号切换
			if err := c.Run(ctx); err != nil && !errors.Is(err, context.Canceled) {
				log.Error("绑定连接退出", "binding", label, "err", err)
			}
		}(binding.Label(), client)
	}

	// 业务日志的定期清理
	if days := retentionDays(); days > 0 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			purgeLoop(ctx, st, days, log)
		}()
	}

	sched.Start()
	log.Info("机器人已启动", "绑定数", len(cfgs), "账号数", len(runtimes))
	fmt.Println("按 Ctrl+C 退出")

	<-ctx.Done()
	log.Info("正在退出...")

	sched.Stop()
	wg.Wait()
	// 引擎与写入器的关闭在函数顶部的 defer 里，那里同时覆盖了早返回路径
	log.Info("已退出")
	return nil
}

// buildAccounts 载入全部账号并初始化各自的运行时资源。
//
// 同一账号连接多个直播间时只建一份，保证限流器被真正共享。
func buildAccounts(ctx context.Context, cfgs []store.RunConfig, log *slog.Logger) (map[string]*accountRuntime, error) {
	out := make(map[string]*accountRuntime)

	for _, c := range cfgs {
		if _, ok := out[c.AccountName]; ok {
			continue
		}

		sess, err := auth.ParseSession(c.Cookie)
		if err != nil {
			return nil, fmt.Errorf("账号 %q 的 Cookie 无效，请重新扫码登录（magicd login --save %s）: %w",
				c.AccountName, c.AccountName, err)
		}

		interval := c.RateLimit
		if interval <= 0 {
			interval = defaultRateLimit
		}

		apiClient := api.New(sess)
		// wbi 签名每个账号刷新一次即可，其全部直播间共用
		if err := apiClient.RefreshNav(ctx); err != nil {
			return nil, fmt.Errorf("账号 %q 初始化签名失败: %w", c.AccountName, err)
		}

		out[c.AccountName] = &accountRuntime{
			acc: account.New(c.AccountName, sess, interval),
			api: apiClient,
		}
		log.Info("已载入账号", "name", c.AccountName, "uid", sess.UID, "发送间隔", interval)
	}
	return out, nil
}

// retentionDays 读业务日志的保留天数。
//
// 环境变量写错就退回默认值：一个日志保留期的笔误，不该让机器人起不来。
func retentionDays() int {
	s := os.Getenv("MAGICD_LOG_RETENTION_DAYS")
	if s == "" {
		return defaultRetentionDays
	}
	n, err := strconv.Atoi(s)
	if err != nil || n < 0 {
		return defaultRetentionDays
	}
	return n
}

// purgeLoop 每小时清理一次超期的业务日志。
func purgeLoop(ctx context.Context, st *store.Store, days int, log *slog.Logger) {
	ticker := time.NewTicker(time.Hour)
	defer ticker.Stop()

	purge := func() {
		cutoff := time.Now().AddDate(0, 0, -days)
		n, err := st.PurgeActivityBefore(ctx, cutoff)
		if err != nil {
			log.Error("清理业务日志失败", "err", err)
			return
		}
		if n > 0 {
			log.Info("已清理超期业务日志", "条数", n, "保留天数", days)
		}
	}

	purge() // 启动时先清一次，不必等满一小时
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			purge()
		}
	}
}
```

- [ ] **Step 4: 跑测试确认通过**

```bash
cd server; go build ./... ; go test ./cmd/magicd/ -v 2>&1 | tail -30; echo "退出码=$?"
```

- [ ] **Step 5: 端到端手工验证**

用一个真实房间跑起来（需要真实 Cookie 的账号已入库）：

```bash
export MAGICD_DATABASE_URL='postgres://magicd:magicd@localhost:5433/magicd?sslmode=disable'
cd server; go run ./cmd/magicd binding list; echo "退出码=$?"
cd server; timeout 60 go run ./cmd/magicd run; echo "退出码=$?"
```

预期：启动日志里每个绑定一行「已配置绑定」，带上规则数与启用数；60 秒后被 timeout 结束。

然后确认业务日志真的落库了：

```bash
docker compose -f docker-compose.dev.yml exec -T postgres \
  psql -U magicd -d magicd -c "SELECT kind, event_type, action_type, rule_name, user_name, occurred_at FROM activity_logs ORDER BY id DESC LIMIT 20;"
echo "退出码=$?"
```

预期：能看到弹幕、进场之类的事件行；若有规则被触发，还能看到紧随其后的 action 行。

再确认噪声事件确实没入库：

```bash
docker compose -f docker-compose.dev.yml exec -T postgres \
  psql -U magicd -d magicd -c "SELECT event_type, count(*) FROM activity_logs GROUP BY event_type ORDER BY 2 DESC;"
echo "退出码=$?"
```

预期：**没有** `online_rank_update` 与 `room_stats_update`。

- [ ] **Step 6: 全量回归**

```bash
cd server; go vet ./... ; gofmt -l . ; go test ./... 2>&1 | tail -20; echo "退出码=$?"
cd server; go test -race -count=2 ./... 2>&1 | tail -15; echo "退出码=$?"
```

- [ ] **Step 7: 提交**

```bash
git add server/cmd/magicd/
git commit -m "$(cat <<'EOF'
feat: run 改从数据库读配置

不再接受 -c，配置的唯一真相是数据库。schema 版本落后就拒绝启动，
不自动迁移。

规则脚本的 storage 从内存换成按绑定隔离的数据库表；每个绑定分到一个
带归属 ID 的业务日志 Sink，共用一个批量写入器。

退出顺序：先关引擎再关写入器——引擎 Close 会结算未决的合并窗口，
反过来的话那批欢迎语的日志就丢了。

保留期的环境变量写错时退回默认值：一个笔误不该让机器人起不来。

Co-Authored-By: Claude Opus 5 <noreply@anthropic.com>
EOF
)"
```

---

## Task 19: CI、文档与示例配置

**Files:**
- Modify: `.github/workflows/ci.yml`（加 PostgreSQL service）
- Modify: `README.md`
- Modify: `config.example.yaml`（顶部说明改成「导入入口」）
- Create: `docs/deployment.md`

**Interfaces:**
- Consumes: 前 18 个任务的全部成果
- Produces: CI 里真跑存储层测试；README 说明新的初始化流程

- [ ] **Step 1: 给 CI 加数据库**

修改 `.github/workflows/ci.yml`。现在的 `test` job 是第 25–66 行，把它的头部（第 25–28 行）：

```yaml
  test:
    name: 测试与静态检查
    runs-on: ubuntu-latest
    steps:
```

替换为：

```yaml
  test:
    name: 测试与静态检查
    runs-on: ubuntu-latest
    services:
      postgres:
        image: postgres:16-alpine
        env:
          POSTGRES_USER: magicd
          POSTGRES_PASSWORD: magicd
          POSTGRES_DB: magicd
        ports:
          - 5432:5432
        options: >-
          --health-cmd "pg_isready -U magicd"
          --health-interval 2s
          --health-timeout 3s
          --health-retries 15
    env:
      # 存储层测试必须真跑：写一份内存替身来避开数据库，等于把
      # 整个 SQL 层排除在测试之外，而 SQL 层正是 P3 唯一的新增风险面。
      MAGICD_TEST_DATABASE_URL: postgres://magicd:magicd@localhost:5432/magicd?sslmode=disable
    steps:
```

其余步骤（检查格式、静态检查、测试、竞态检测、确认依赖清单已整理）一律不动。

然后在「测试」步骤（现第 50–52 行）**之后**插入一步：

```yaml
      - name: 确认存储层测试没有被跳过
        working-directory: server
        run: |
          go test ./internal/store/ -v -count=1 -run TestMigrateCreatesAllTables 2>&1 | tee /tmp/store.log
          if grep -q -- "--- SKIP" /tmp/store.log; then
            echo "存储层测试被跳过了——MAGICD_TEST_DATABASE_URL 没生效" >&2
            exit 1
          fi
```

这一步不能省：环境变量配错时测试会静默全部跳过并报绿，那比没有测试更糟。

另外把顶部 `on.push.paths` 与 `on.pull_request.paths` 各加一行 `- "docker-compose.dev.yml"`，让本地开发库的定义变更也能触发 CI。

- [ ] **Step 2: 本地验证 CI 配置语法**

```bash
cd /f/danmuku; python -c "import yaml,sys; yaml.safe_load(open('.github/workflows/ci.yml')); print('YAML 合法')"; echo "退出码=$?"
```

- [ ] **Step 3: 更新 config.example.yaml 的说明**

把 `config.example.yaml` 顶部的用法注释（第 1–12 行）换成：

```yaml
# 神奇弹幕配置示例
#
# 这份 YAML 是**导入入口**，不是运行时配置。数据库才是配置的唯一真相，
# magicd run 不读它。
#
# 用法：
#   export MAGICD_DATABASE_URL='postgres://magicd:magicd@localhost:5432/magicd?sslmode=disable'
#   magicd migrate                              # 建表，并创建管理员
#   magicd login -o cookie-main.txt             # 每个账号各扫码登录一次
#   magicd login -o cookie-sub.txt
#   cp config.example.yaml config.yaml
#   magicd import -c config.yaml --owner admin  # 导进数据库
#   magicd run                                  # 启动
#
# 也可以完全不用 YAML：
#   magicd login --save 小号 --owner admin
#   magicd binding add 小号 1706666491
#   # 规则等 P4 的 WebUI 做好后在网页上配
#
# 导入是幂等的：同一份 YAML 导两次结果一致。YAML 里删掉的规则，
# 重新导入后库里也会消失。
#
# 结构是三层嵌套：账号 → 直播间 → 规则。
# 每个「账号-直播间」组合是一个独立的运行单元，有自己的连接、规则与
# 冷却状态。同一直播间被两个账号连接时是两条独立连接，互不知道对方存在。
#
# 不做账号轮换：某个账号失效时只记录错误日志，不会让别的账号顶替。
```

其余内容（`accounts:` 及以下）保持不变。

- [ ] **Step 4: 写部署文档**

创建 `docs/deployment.md`：

````markdown
# 部署

## 依赖

- PostgreSQL 14 或更高
- 一个 `magicd` 二进制（六平台预编译包见 Releases）

PostgreSQL 是硬依赖。这是 P3 明确权衡后的选择：Go 侧仍是单文件、六平台
交叉编译不受影响，但「下载即跑」没有了——单人自用也需要先有一个数据库。

## 快速开始

```bash
# 1. 起一个 PostgreSQL（已有的话跳过）
docker run -d --name magicd-pg \
  -e POSTGRES_USER=magicd -e POSTGRES_PASSWORD=改成你自己的密码 \
  -e POSTGRES_DB=magicd -p 5432:5432 postgres:16-alpine

# 2. 配置连接串
export MAGICD_DATABASE_URL='postgres://magicd:改成你自己的密码@localhost:5432/magicd?sslmode=disable'

# 3. 建表。空库上会创建管理员并打印一次性密码，记下来
magicd migrate

# 4. 扫码登录一个 B 站账号
magicd login --save 小号 --owner admin

# 5. 让它连接一个直播间
magicd binding add 小号 1706666491

# 6. 启动
magicd run
```

规则的配置目前有两条路：写 YAML 然后 `magicd import`，或者等 P4 的
WebUI。示例见仓库根目录的 `config.example.yaml`。

## 环境变量

| 变量 | 默认 | 说明 |
|---|---|---|
| `MAGICD_DATABASE_URL` | 无 | PostgreSQL 连接串，必填 |
| `MAGICD_LOG_LEVEL` | `info` | `debug` / `info` / `warn` / `error` |
| `MAGICD_LOG_FILE` | 空 | 系统日志文件路径，留空则只写 stderr |
| `MAGICD_LOG_RETENTION_DAYS` | `30` | 业务日志保留天数，0 表示不清理 |

## 两类日志

**系统日志**走 stderr 与可选的滚动文件：启动、连接、重连、报错。
数据库连不上时，「数据库连不上」这条日志本身还得写得出来，所以它不进库。

**业务日志**进数据库的 `activity_logs` 表：收到的事件与机器人执行的动作
在同一条时间线上，可按账号查询。

```sql
-- 小号今天干了什么
SELECT occurred_at, kind, event_type, action_type, rule_name, user_name
FROM activity_logs
WHERE account_id = (SELECT id FROM accounts WHERE name = '小号')
  AND occurred_at > now() - interval '1 day'
ORDER BY occurred_at DESC;
```

排行榜与房间统计事件不入库——它们每 8 秒一条且没有分析价值。

极端流量下业务日志会被主动丢弃，丢弃量汇总进系统日志。**这是设计行为**：
丢日志可以接受，漏欢迎不行。

## 安全须知

**Cookie 以明文存储在 `accounts.cookie` 列里。** 它等同于账号密码。

两条直接后果：

1. `pg_dump` 的备份文件里是明文 Cookie，按密码的标准保管
2. 若 PostgreSQL 不在本机且连接未启用 TLS，Cookie 每次读取都会明文过网络。
   远程数据库请在连接串里加 `sslmode=require`

这是按「WebUI 与本机只被受信任的人操作」的威胁模型做的选择。

## 升级

```bash
magicd migrate   # 换二进制后先跑这个
magicd run
```

`run` 发现 schema 版本落后会**拒绝启动**而非自动迁移——多实例部署下，
让每个实例各自决定何时改表是危险的。

迁移只做前向，不提供回滚脚本。升级前请备份：

```bash
pg_dump -U magicd magicd > magicd-backup.sql
```

## 权限

授权单位是「账号-直播间」绑定，不设固定角色，直接给权限点：

```bash
magicd grant -list                                 # 看有哪些权限点
magicd grant 李四 小号@1706666491 rule:read,rule:write
magicd perms 李四                                   # 看某人有什么权限
magicd revoke 李四 小号@1706666491
```

`magicd grant` 是**替换**而非累加：重新授权的语义是「设定为这些」。

管理员（`magicd user add <名字> --admin`）绕过全部检查。
````

- [ ] **Step 5: 更新 README**

现在的 `README.md` 是原作者的项目归档公告，没有用法章节。**不要改动原有内容**——那是原作者的声明与致谢。在文件**末尾追加**一节：

````markdown

---

# 本分支：Go 重写版

原项目的 Qt/C++ 桌面程序被完整重写为 Go 编写的无头服务端，前后端分离，
移除全部收费限制。

## 快速开始

需要 PostgreSQL 14+。完整部署说明见 [docs/deployment.md](docs/deployment.md)。

```bash
export MAGICD_DATABASE_URL='postgres://magicd:magicd@localhost:5432/magicd?sslmode=disable'
magicd migrate                         # 建表，记下打印出的管理员密码
magicd login --save 小号 --owner admin  # 扫码登录一个 B 站账号
magicd binding add 小号 1706666491      # 让它连接一个直播间
magicd run                             # 启动
```

已有 `config.yaml` 的话：`magicd import -c config.yaml --owner admin`。

配置的唯一真相是数据库，`magicd run` 不读 YAML——YAML 只是导入入口。

## 与原项目的差异

- **平台**：只做 B 站；win / macOS / Linux × amd64 / arm64，加 Docker
- **形态**：无头服务端，不依赖窗口；管理界面走 Web
- **收费限制**：全部移除
- **已删除**：`www/` OBS 浏览器源托管、五子棋等 extension、点歌姬与音乐
  播放器、ChatGPT AI 聊天、Qt 桌面 UI、直播间运营动作、语音播报
- **多账号**：是职责分工而非轮换。主播号可以只做统计与房管而不发言，
  小号负责欢迎答谢；某个账号失效时只记录错误日志，不会让别的账号顶替
````

- [ ] **Step 6: 全量回归**

```bash
cd server; go build ./... ; go vet ./... ; gofmt -l . ; echo "退出码=$?"
export MAGICD_TEST_DATABASE_URL='postgres://magicd:magicd@localhost:5433/magicd?sslmode=disable'
cd server; go test ./... 2>&1 | tail -25; echo "退出码=$?"
cd server; go test -race -count=2 ./... 2>&1 | tail -15; echo "退出码=$?"
```

确认无数据库时整套测试仍然全绿（只是跳过存储层）：

```bash
cd server; env -u MAGICD_TEST_DATABASE_URL go test ./... 2>&1 | tail -25; echo "退出码=$?"
```

六平台交叉编译：

```bash
cd server; for os in windows darwin linux; do for arch in amd64 arm64; do
  CGO_ENABLED=0 GOOS=$os GOARCH=$arch go build -o /dev/null ./... || echo "失败: $os/$arch"
done; done; echo "退出码=$?"
```

- [ ] **Step 7: 提交**

```bash
git add .github/workflows/ci.yml README.md config.example.yaml docs/deployment.md
git commit -m "$(cat <<'EOF'
ci: 让 CI 真跑存储层测试；补部署文档

GitHub Actions 起一个 PostgreSQL service。额外加一步断言存储层测试
没有被跳过——环境变量配错时测试会静默全部跳过并报绿，那比没有测试
更糟。

部署文档写明两条 Cookie 明文存储的直接后果：pg_dump 备份里是明文；
远程数据库不开 TLS 时 Cookie 每次读取都过明文网络。

config.example.yaml 的定位改为「导入入口」——数据库才是配置的唯一
真相，run 不读 YAML。

Co-Authored-By: Claude Opus 5 <noreply@anthropic.com>
EOF
)"
```

---

## P3 完成检查

全部 19 个任务做完后，逐条确认：

- [ ] `magicd migrate` 在空库上建表并打印管理员密码
- [ ] `magicd login --save` 能把 Cookie 直接存进数据库
- [ ] `magicd binding add` 能加直播间
- [ ] `magicd import -c config.yaml --owner admin` 幂等，导两次结果一致
- [ ] `magicd run` 不需要 `-c`，从数据库读配置
- [ ] `magicd grant / revoke / perms / can` 能管并检查权限点
- [ ] 业务日志落进 `activity_logs`，事件与动作在同一条时间线
- [ ] `online_rank_update` 与 `room_stats_update` **没有**入库
- [ ] 系统日志写 stderr，配了 `MAGICD_LOG_FILE` 时也写文件
- [ ] `internal/rules/config` 的测试一行未改且全部通过
- [ ] `go test ./...` 在有数据库与无数据库两种情况下都是绿的
- [ ] `go test -race -count=2 ./...` 干净
- [ ] 六平台 `CGO_ENABLED=0` 交叉编译通过
- [ ] CI 里存储层测试真的在跑，不是被跳过

做完后更新 `docs/superpowers/specs/2026-07-29-p0-protocol-core-design.md` 附录 C 的路线图，把 P3 标为完成。
