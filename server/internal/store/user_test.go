package store

import (
	"context"
	"errors"
	"strings"
	"testing"
)

func TestCreateAndGetUser(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()

	created, err := s.CreateUser(ctx, "张三", "hunter2", false)
	if err != nil {
		t.Fatalf("创建用户报错: %v", err)
	}
	if created.ID == 0 {
		t.Error("新用户应有非零 ID")
	}
	if created.IsAdmin {
		t.Error("isAdmin 传 false 时不应是管理员")
	}

	got, err := s.GetUserByName(ctx, "张三")
	if err != nil {
		t.Fatalf("查询用户报错: %v", err)
	}
	if got.ID != created.ID || got.Username != "张三" {
		t.Errorf("查到的用户 = %+v", got)
	}
}

func TestGetUserNotFound(t *testing.T) {
	s := testStore(t)
	_, err := s.GetUserByName(context.Background(), "不存在的人")
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("应返回 ErrNotFound，实际: %v", err)
	}
}

func TestCreateUserRejectsDuplicate(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()

	if _, err := s.CreateUser(ctx, "张三", "a", false); err != nil {
		t.Fatalf("首次创建报错: %v", err)
	}
	_, err := s.CreateUser(ctx, "张三", "b", false)
	if !errors.Is(err, ErrDuplicate) {
		t.Errorf("重名应返回 ErrDuplicate，实际: %v", err)
	}
}

func TestPasswordIsHashedNotStored(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()

	if _, err := s.CreateUser(ctx, "张三", "hunter2", false); err != nil {
		t.Fatalf("创建用户报错: %v", err)
	}

	var hash string
	if err := s.pool.QueryRow(ctx,
		`SELECT password_hash FROM users WHERE username = '张三'`).Scan(&hash); err != nil {
		t.Fatalf("读取哈希报错: %v", err)
	}
	if strings.Contains(hash, "hunter2") {
		t.Error("密码不得以任何形式明文出现在库里")
	}
	if !strings.HasPrefix(hash, "$2") {
		t.Errorf("应是 bcrypt 哈希，实际前缀: %.4s", hash)
	}
}

func TestVerifyPasswordAcceptsCorrect(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()

	if _, err := s.CreateUser(ctx, "张三", "hunter2", false); err != nil {
		t.Fatalf("创建用户报错: %v", err)
	}
	u, err := s.VerifyPassword(ctx, "张三", "hunter2")
	if err != nil {
		t.Fatalf("正确密码应通过: %v", err)
	}
	if u.Username != "张三" {
		t.Errorf("返回的用户 = %+v", u)
	}
}

func TestVerifyPasswordRejectsWrong(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()

	if _, err := s.CreateUser(ctx, "张三", "hunter2", false); err != nil {
		t.Fatalf("创建用户报错: %v", err)
	}
	_, err := s.VerifyPassword(ctx, "张三", "wrong")
	if !errors.Is(err, ErrBadCredentials) {
		t.Errorf("错误密码应返回 ErrBadCredentials，实际: %v", err)
	}
}

func TestVerifyPasswordHidesWhetherUserExists(t *testing.T) {
	// 用户名不存在与密码错误返回同一个错误，否则接口就成了用户名枚举器
	s := testStore(t)
	ctx := context.Background()

	if _, err := s.CreateUser(ctx, "张三", "hunter2", false); err != nil {
		t.Fatalf("创建用户报错: %v", err)
	}
	_, errNoUser := s.VerifyPassword(ctx, "不存在的人", "whatever")
	_, errBadPass := s.VerifyPassword(ctx, "张三", "wrong")

	if !errors.Is(errNoUser, ErrBadCredentials) {
		t.Errorf("用户不存在时也应返回 ErrBadCredentials，实际: %v", errNoUser)
	}
	if errNoUser.Error() != errBadPass.Error() {
		t.Errorf("两种失败的错误信息应完全一致:\n  无此人: %v\n  密码错: %v", errNoUser, errBadPass)
	}
}

func TestSetPasswordChangesIt(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()

	if _, err := s.CreateUser(ctx, "张三", "old", false); err != nil {
		t.Fatalf("创建用户报错: %v", err)
	}
	if err := s.SetPassword(ctx, "张三", "new"); err != nil {
		t.Fatalf("改密码报错: %v", err)
	}
	if _, err := s.VerifyPassword(ctx, "张三", "new"); err != nil {
		t.Errorf("新密码应通过: %v", err)
	}
	if _, err := s.VerifyPassword(ctx, "张三", "old"); !errors.Is(err, ErrBadCredentials) {
		t.Error("旧密码应失效")
	}
}

func TestSetPasswordOnMissingUser(t *testing.T) {
	s := testStore(t)
	err := s.SetPassword(context.Background(), "不存在的人", "x")
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("应返回 ErrNotFound，实际: %v", err)
	}
}

func TestCreateUserRejectsEmptyPassword(t *testing.T) {
	s := testStore(t)
	if _, err := s.CreateUser(context.Background(), "张三", "", false); err == nil {
		t.Error("空密码应被拒绝")
	}
}

func TestCreateUserRejectsEmptyUsername(t *testing.T) {
	s := testStore(t)
	if _, err := s.CreateUser(context.Background(), "  ", "x", false); err == nil {
		t.Error("空用户名应被拒绝")
	}
}

func TestListUsersOrderedByID(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()

	for _, n := range []string{"张三", "李四", "王五"} {
		if _, err := s.CreateUser(ctx, n, "x", false); err != nil {
			t.Fatalf("创建 %s 报错: %v", n, err)
		}
	}
	us, err := s.ListUsers(ctx)
	if err != nil {
		t.Fatalf("列出用户报错: %v", err)
	}
	if len(us) != 3 {
		t.Fatalf("用户数 = %d, 期望 3", len(us))
	}
	if us[0].Username != "张三" || us[2].Username != "王五" {
		t.Errorf("顺序不对: %v", []string{us[0].Username, us[1].Username, us[2].Username})
	}
}

func TestEnsureAdminCreatesOnEmptyDatabase(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()

	name, pass, created, err := s.EnsureAdmin(ctx, "")
	if err != nil {
		t.Fatalf("EnsureAdmin 报错: %v", err)
	}
	if !created {
		t.Fatal("空库上应创建管理员")
	}
	if name != "admin" {
		t.Errorf("管理员用户名 = %q, 期望 admin", name)
	}
	if len(pass) < 16 {
		t.Errorf("随机密码太短: %d 个字符", len(pass))
	}

	u, err := s.VerifyPassword(ctx, name, pass)
	if err != nil {
		t.Fatalf("打印出来的密码应当能登录: %v", err)
	}
	if !u.IsAdmin {
		t.Error("自动创建的应是管理员")
	}
}

func TestEnsureAdminSkipsWhenUsersExist(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()

	if _, err := s.CreateUser(ctx, "张三", "x", false); err != nil {
		t.Fatalf("创建用户报错: %v", err)
	}
	_, _, created, err := s.EnsureAdmin(ctx, "")
	if err != nil {
		t.Fatalf("EnsureAdmin 报错: %v", err)
	}
	if created {
		t.Error("已有用户时不应再创建管理员")
	}
}

// TestEnsureAdminSkipsWhenUsersExistEvenWithPassword 验证 password 参数
// 不能被当作"改已有管理员密码"的后门——库里已有用户时，即使调用方
// 传了个合法密码，也必须是空操作，不能悄悄把它设成新密码。
func TestEnsureAdminSkipsWhenUsersExistEvenWithPassword(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()

	if _, err := s.CreateUser(ctx, "张三", "原密码", false); err != nil {
		t.Fatalf("创建用户报错: %v", err)
	}
	_, _, created, err := s.EnsureAdmin(ctx, "一个足够强的新密码123")
	if err != nil {
		t.Fatalf("EnsureAdmin 报错: %v", err)
	}
	if created {
		t.Error("已有用户时不应再创建管理员")
	}
	if _, err := s.GetUserByName(ctx, "admin"); !errors.Is(err, ErrNotFound) {
		t.Errorf("不该凭空多出一个 admin 账户，查询结果: %v", err)
	}
}

// TestEnsureAdminUsesExplicitPassword 验证传入非空密码时，用的就是这个
// 密码，而不是仍然生成随机密码——无头部署靠这条路径把初始密码钉死成
// MAGICD_ADMIN_PASSWORD 里配的值。
func TestEnsureAdminUsesExplicitPassword(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()

	name, pass, created, err := s.EnsureAdmin(ctx, "一个足够强的密码123")
	if err != nil {
		t.Fatalf("EnsureAdmin 报错: %v", err)
	}
	if !created {
		t.Fatal("空库上应创建管理员")
	}
	if pass != "一个足够强的密码123" {
		t.Errorf("返回的密码 = %q, 期望原样是调用方传入的密码", pass)
	}
	if _, err := s.VerifyPassword(ctx, name, "一个足够强的密码123"); err != nil {
		t.Fatalf("传入的密码应当能登录: %v", err)
	}
}

// TestEnsureAdminRejectsWeakPassword 验证密码强度下限：太短的密码必须
// 报错拒绝，而不是悄悄建出一个"123"就能登录的管理员账户。
func TestEnsureAdminRejectsWeakPassword(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()

	_, _, created, err := s.EnsureAdmin(ctx, "123")
	if !errors.Is(err, ErrWeakPassword) {
		t.Errorf("err = %v, 期望 ErrWeakPassword", err)
	}
	if created {
		t.Error("密码太弱时不该创建管理员")
	}
	if _, err := s.GetUserByName(ctx, "admin"); !errors.Is(err, ErrNotFound) {
		t.Errorf("密码太弱时不该留下任何 admin 账户，查询结果: %v", err)
	}
}

func TestEnsureAdminPasswordsDiffer(t *testing.T) {
	// 随机密码必须真随机，写死的常量等于没有密码
	var p1, p2 string
	t.Run("第一次", func(t *testing.T) {
		s := testStore(t)
		_, p, _, err := s.EnsureAdmin(context.Background(), "")
		if err != nil {
			t.Fatalf("EnsureAdmin 报错: %v", err)
		}
		p1 = p
	})
	t.Run("第二次", func(t *testing.T) {
		s := testStore(t)
		_, p, _, err := s.EnsureAdmin(context.Background(), "")
		if err != nil {
			t.Fatalf("EnsureAdmin 报错: %v", err)
		}
		p2 = p
	})
	if p1 == "" || p2 == "" {
		t.Skip("子测试被跳过（无数据库）")
	}
	if p1 == p2 {
		t.Error("两次生成的随机密码相同")
	}
}
