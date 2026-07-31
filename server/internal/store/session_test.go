package store

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

func TestCreateAndLookupSession(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	uid := mustUser(t, s, "张三")

	token, err := s.CreateSession(ctx, uid, time.Hour, "curl/8")
	if err != nil {
		t.Fatalf("创建会话报错: %v", err)
	}
	if len(token) < 32 {
		t.Errorf("令牌太短: %d 个字符", len(token))
	}

	u, err := s.LookupSession(ctx, token)
	if err != nil {
		t.Fatalf("查会话报错: %v", err)
	}
	if u.ID != uid || u.Username != "张三" {
		t.Errorf("查到的用户 = %+v", u)
	}
}

// 库里存的必须是哈希，不能是令牌原文
func TestSessionTokenStoredAsHash(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	uid := mustUser(t, s, "张三")

	token, err := s.CreateSession(ctx, uid, time.Hour, "")
	if err != nil {
		t.Fatalf("创建会话报错: %v", err)
	}

	var stored string
	if err := s.pool.QueryRow(ctx, `SELECT token_hash FROM sessions`).Scan(&stored); err != nil {
		t.Fatalf("读取报错: %v", err)
	}
	if stored == token {
		t.Error("库里存的是令牌原文，应当是哈希")
	}
	if strings.Contains(stored, token) {
		t.Error("令牌原文出现在存储值里")
	}
	if len(stored) != 64 {
		t.Errorf("SHA-256 十六进制应是 64 个字符，实际 %d", len(stored))
	}
}

func TestCreateSessionTokensAreUnique(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	uid := mustUser(t, s, "张三")

	seen := make(map[string]bool, 20)
	for i := 0; i < 20; i++ {
		token, err := s.CreateSession(ctx, uid, time.Hour, "")
		if err != nil {
			t.Fatalf("第 %d 次创建报错: %v", i, err)
		}
		if seen[token] {
			t.Fatalf("第 %d 次生成了重复的令牌", i)
		}
		seen[token] = true
	}
}

func TestLookupSessionUnknownToken(t *testing.T) {
	s := testStore(t)
	if _, err := s.LookupSession(context.Background(), "根本不存在的令牌"); !errors.Is(err, ErrNotFound) {
		t.Errorf("应返回 ErrNotFound，实际: %v", err)
	}
}

// 过期的会话必须查不到，哪怕行还在
func TestLookupSessionExpired(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	uid := mustUser(t, s, "张三")

	token, err := s.CreateSession(ctx, uid, -time.Minute, "")
	if err != nil {
		t.Fatalf("创建会话报错: %v", err)
	}
	if _, err := s.LookupSession(ctx, token); !errors.Is(err, ErrNotFound) {
		t.Errorf("过期会话应查不到，实际: %v", err)
	}
}

func TestDeleteSession(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	uid := mustUser(t, s, "张三")

	token, err := s.CreateSession(ctx, uid, time.Hour, "")
	if err != nil {
		t.Fatalf("创建会话报错: %v", err)
	}
	if err := s.DeleteSession(ctx, token); err != nil {
		t.Fatalf("删除会话报错: %v", err)
	}
	if _, err := s.LookupSession(ctx, token); !errors.Is(err, ErrNotFound) {
		t.Errorf("删除后应查不到，实际: %v", err)
	}
}

func TestDeleteSessionUnknownTokenIsNotAnError(t *testing.T) {
	// 登出一个已失效的会话不该报错——用户点登出，结果得到一个错误页，很蠢
	s := testStore(t)
	if err := s.DeleteSession(context.Background(), "不存在"); err != nil {
		t.Errorf("删除不存在的会话不该报错，实际: %v", err)
	}
}

// 改密码要能一次踢掉该用户的全部会话
func TestDeleteUserSessions(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	uid := mustUser(t, s, "张三")
	other := mustUser(t, s, "李四")

	var tokens []string
	for i := 0; i < 3; i++ {
		tk, err := s.CreateSession(ctx, uid, time.Hour, "")
		if err != nil {
			t.Fatalf("创建会话报错: %v", err)
		}
		tokens = append(tokens, tk)
	}
	otherToken, err := s.CreateSession(ctx, other, time.Hour, "")
	if err != nil {
		t.Fatalf("创建他人会话报错: %v", err)
	}

	n, err := s.DeleteUserSessions(ctx, uid)
	if err != nil {
		t.Fatalf("批量删除报错: %v", err)
	}
	if n != 3 {
		t.Errorf("删除数 = %d, 期望 3", n)
	}
	for i, tk := range tokens {
		if _, err := s.LookupSession(ctx, tk); !errors.Is(err, ErrNotFound) {
			t.Errorf("第 %d 个会话应已失效", i)
		}
	}
	if _, err := s.LookupSession(ctx, otherToken); err != nil {
		t.Errorf("不该动别人的会话: %v", err)
	}
}

func TestSessionCascadeOnUserDelete(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	u, err := s.CreateUser(ctx, "张三", "pw", false)
	if err != nil {
		t.Fatalf("创建用户报错: %v", err)
	}
	if _, err := s.CreateSession(ctx, u.ID, time.Hour, ""); err != nil {
		t.Fatalf("创建会话报错: %v", err)
	}
	if _, err := s.pool.Exec(ctx, `DELETE FROM users WHERE id = $1`, u.ID); err != nil {
		t.Fatalf("删除用户报错: %v", err)
	}

	var n int
	if err := s.pool.QueryRow(ctx, `SELECT count(*) FROM sessions`).Scan(&n); err != nil {
		t.Fatalf("统计报错: %v", err)
	}
	if n != 0 {
		t.Errorf("用户删除后会话应级联删除，实际剩 %d 条", n)
	}
}

func TestPurgeExpiredSessions(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	uid := mustUser(t, s, "张三")

	if _, err := s.CreateSession(ctx, uid, -time.Hour, ""); err != nil {
		t.Fatalf("创建过期会话报错: %v", err)
	}
	if _, err := s.CreateSession(ctx, uid, time.Hour, ""); err != nil {
		t.Fatalf("创建有效会话报错: %v", err)
	}

	n, err := s.PurgeExpiredSessions(ctx)
	if err != nil {
		t.Fatalf("清理报错: %v", err)
	}
	if n != 1 {
		t.Errorf("清理数 = %d, 期望 1", n)
	}

	var left int
	if err := s.pool.QueryRow(ctx, `SELECT count(*) FROM sessions`).Scan(&left); err != nil {
		t.Fatalf("统计报错: %v", err)
	}
	if left != 1 {
		t.Errorf("应剩 1 条有效会话，实际 %d", left)
	}
}

func TestSchemaVersionIsTwoAfterSessionsMigration(t *testing.T) {
	s := testStore(t)
	v, err := s.SchemaVersion(context.Background())
	if err != nil {
		t.Fatalf("查版本报错: %v", err)
	}
	if v != 2 {
		t.Errorf("schema 版本 = %d, 期望 2（新增了 002_sessions.sql）", v)
	}
}
