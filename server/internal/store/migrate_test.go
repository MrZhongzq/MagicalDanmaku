package store

import (
	"context"
	"testing"
)

func TestMigrateCreatesAllTables(t *testing.T) {
	s := testStore(t) // testStore 内部已跑过 Migrate
	ctx := context.Background()

	want := []string{
		"users", "accounts", "bindings", "memberships", "rules",
		"cooldown_groups", "kv_store", "block_list", "activity_logs",
		"schema_migrations",
	}
	for _, table := range want {
		var exists bool
		err := s.pool.QueryRow(ctx, `
			SELECT EXISTS (
				SELECT 1 FROM information_schema.tables
				WHERE table_schema = current_schema() AND table_name = $1
			)`, table).Scan(&exists)
		if err != nil {
			t.Fatalf("查询表 %s 是否存在时报错: %v", table, err)
		}
		if !exists {
			t.Errorf("表 %s 未被创建", table)
		}
	}
}

func TestMigrateIsIdempotent(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()

	// 再跑一次不应报错，也不应重复执行
	if err := s.Migrate(ctx); err != nil {
		t.Fatalf("重复迁移报错: %v", err)
	}

	var n int
	if err := s.pool.QueryRow(ctx, `SELECT count(*) FROM schema_migrations`).Scan(&n); err != nil {
		t.Fatalf("查询迁移记录报错: %v", err)
	}
	want, err := LatestSchemaVersion()
	if err != nil {
		t.Fatalf("LatestSchemaVersion 报错: %v", err)
	}
	if n != want {
		t.Errorf("迁移记录数 = %d, 期望 %d（应等于最新版本号）", n, want)
	}
}

func TestSchemaVersionReportsLatest(t *testing.T) {
	s := testStore(t)
	v, err := s.SchemaVersion(context.Background())
	if err != nil {
		t.Fatalf("SchemaVersion 报错: %v", err)
	}
	want, err := LatestSchemaVersion()
	if err != nil {
		t.Fatalf("LatestSchemaVersion 报错: %v", err)
	}
	if v != want {
		t.Errorf("版本 = %d, 期望 %d", v, want)
	}
}

func TestSchemaVersionOnEmptyDatabaseIsZero(t *testing.T) {
	// 未迁移过的库里没有 schema_migrations 表，应返回 0 而非报错
	s := testStore(t)
	ctx := context.Background()

	if _, err := s.pool.Exec(ctx, `DROP TABLE schema_migrations`); err != nil {
		t.Fatalf("删表报错: %v", err)
	}
	v, err := s.SchemaVersion(ctx)
	if err != nil {
		t.Fatalf("SchemaVersion 报错: %v", err)
	}
	if v != 0 {
		t.Errorf("版本 = %d, 期望 0", v)
	}
}

func TestForeignKeyOwnerIsRestrictNotCascade(t *testing.T) {
	// 删掉一个还拥有账号的用户应当报错，而不是留下无主的 Cookie
	s := testStore(t)
	ctx := context.Background()

	var userID int64
	err := s.pool.QueryRow(ctx, `
		INSERT INTO users (username, password_hash) VALUES ('张三', 'x') RETURNING id`).Scan(&userID)
	if err != nil {
		t.Fatalf("建用户报错: %v", err)
	}
	_, err = s.pool.Exec(ctx, `
		INSERT INTO accounts (name, cookie, owner_id) VALUES ('主播号', 'c', $1)`, userID)
	if err != nil {
		t.Fatalf("建账号报错: %v", err)
	}

	if _, err := s.pool.Exec(ctx, `DELETE FROM users WHERE id = $1`, userID); err == nil {
		t.Error("删除仍拥有账号的用户应被外键拒绝")
	}
}

func TestForeignKeyBindingChildrenCascade(t *testing.T) {
	// 删绑定应带走它的规则、冷却组、KV 与禁言名单
	s := testStore(t)
	ctx := context.Background()

	var userID, accountID, bindingID int64
	if err := s.pool.QueryRow(ctx, `
		INSERT INTO users (username, password_hash) VALUES ('张三', 'x') RETURNING id`).Scan(&userID); err != nil {
		t.Fatalf("建用户报错: %v", err)
	}
	if err := s.pool.QueryRow(ctx, `
		INSERT INTO accounts (name, cookie, owner_id) VALUES ('主播号', 'c', $1) RETURNING id`,
		userID).Scan(&accountID); err != nil {
		t.Fatalf("建账号报错: %v", err)
	}
	if err := s.pool.QueryRow(ctx, `
		INSERT INTO bindings (account_id, room_id) VALUES ($1, '123') RETURNING id`,
		accountID).Scan(&bindingID); err != nil {
		t.Fatalf("建绑定报错: %v", err)
	}
	if _, err := s.pool.Exec(ctx, `
		INSERT INTO rules (binding_id, name, spec) VALUES ($1, 'r', '{}')`, bindingID); err != nil {
		t.Fatalf("建规则报错: %v", err)
	}

	if _, err := s.pool.Exec(ctx, `DELETE FROM bindings WHERE id = $1`, bindingID); err != nil {
		t.Fatalf("删绑定报错: %v", err)
	}

	var n int
	if err := s.pool.QueryRow(ctx, `SELECT count(*) FROM rules`).Scan(&n); err != nil {
		t.Fatalf("查规则数报错: %v", err)
	}
	if n != 0 {
		t.Errorf("绑定删除后规则应被级联删除，实际剩 %d 条", n)
	}
}
