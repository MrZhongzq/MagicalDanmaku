package store

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
)

// testStore 为每个测试建一个独立 schema，测完 drop。
//
// 独立 schema 而非独立数据库：建库要连 postgres 库、慢且需要额外权限，
// 而 schema 是同库内的命名空间，建删都是毫秒级。
//
// 注意：目前不要给用到它的测试加 t.Parallel()。迁移用的
// migrateLockKey 是数据库级的 advisory lock，并行跑时每个 testStore
// 的 Migrate 都会在这把锁上互相等待，只会串行执行、并不会变快；
// 另外 schemaNameFor 把所有非 ASCII 字符折叠成 `_`，两个仅在中文等
// 非 ASCII 字符上不同的测试名会撞成同一个 schema，并行下一个测试的
// DROP SCHEMA ... CASCADE 可能删掉另一个测试正在用的 schema。
func testStore(t *testing.T) *Store {
	t.Helper()

	dsn := os.Getenv("MAGICD_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("未设置 MAGICD_TEST_DATABASE_URL，跳过存储层测试。\n" +
			"本地起库：docker compose -f docker-compose.dev.yml up -d\n" +
			"然后：export MAGICD_TEST_DATABASE_URL='postgres://magicd:magicd@localhost:5433/magicd?sslmode=disable'")
	}

	schema := schemaNameFor(t.Name())
	ctx := context.Background()

	// 先用默认 search_path 建 schema
	admin, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatalf("连接数据库失败: %v", err)
	}
	if _, err := admin.Exec(ctx, `DROP SCHEMA IF EXISTS `+schema+` CASCADE`); err != nil {
		admin.Close()
		t.Fatalf("清理旧 schema 失败: %v", err)
	}
	if _, err := admin.Exec(ctx, `CREATE SCHEMA `+schema); err != nil {
		admin.Close()
		t.Fatalf("创建 schema 失败: %v", err)
	}

	// schema 建成功后立即注册清理，而不是等 OpenWithSchema/Migrate 都
	// 成功之后才注册：后两者若失败会走 t.Fatalf 直接退出当前 goroutine，
	// 若 Cleanup 还没注册，schema 就没人清理、留在库里越积越多。
	// s 在 Cleanup 注册时还是 nil，等下面赋值后 Cleanup 才会看到非 nil
	// 的值——Go 闭包捕获的是变量本身，不是注册那一刻的值。
	var s *Store
	t.Cleanup(func() {
		if s != nil {
			s.Close()
		}
		if _, err := admin.Exec(context.Background(),
			`DROP SCHEMA IF EXISTS `+schema+` CASCADE`); err != nil {
			t.Logf("清理 schema %s 失败: %v", schema, err)
		}
		admin.Close()
	})

	s, err = OpenWithSchema(ctx, dsn, schema)
	if err != nil {
		t.Fatalf("打开存储失败: %v", err)
	}
	if err := s.Migrate(ctx); err != nil {
		t.Fatalf("迁移失败: %v", err)
	}

	return s
}

// schemaNameFor 把测试名转成合法的 PostgreSQL 标识符。
//
// t.Name() 含斜杠与中文，且 PG 标识符上限 63 字节，都要处理。
func schemaNameFor(name string) string {
	var b strings.Builder
	b.WriteString("t_")
	for _, r := range name {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			b.WriteRune(r)
		case r >= 'A' && r <= 'Z':
			b.WriteRune(r + 32)
		default:
			b.WriteByte('_')
		}
	}
	s := b.String()
	if len(s) > 50 {
		// 截断会撞名，补一段长度做区分
		s = fmt.Sprintf("%s_%d", s[:44], len(name))
	}
	return s
}
