package store

import (
	"context"
	"embed"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/jackc/pgx/v5"
)

//go:embed migrations/*.sql
var migrationFS embed.FS

// migrateLockKey 是迁移用的 advisory lock 键，取 "magi" 的 ASCII。
// 值本身不重要，只要全项目一致。
const migrateLockKey int64 = 0x6d616769

// ErrSchemaOutdated 表示数据库 schema 版本落后于二进制。
//
// run 遇到它应拒绝启动而非自动迁移：多实例部署下，让每个实例
// 各自决定何时改表是危险的。
var ErrSchemaOutdated = errors.New("store: 数据库 schema 版本落后，请先运行 magicd migrate")

// migration 是一个待执行的迁移文件。
type migration struct {
	version int
	name    string
	sql     string
}

// Migrate 把 schema 升到最新版本。
//
// 只做前向迁移，不实现回滚：回滚脚本在实践中几乎从不被执行，
// 却要一直维护；真出问题时恢复备份比跑回滚脚本可靠。
func (s *Store) Migrate(ctx context.Context) error {
	ms, err := loadMigrations()
	if err != nil {
		return err
	}

	// 迁移期间独占一条连接：advisory lock 是会话级的，
	// 用连接池会把 lock 和 unlock 发到两条不同的连接上。
	conn, err := s.pool.Acquire(ctx)
	if err != nil {
		return fmt.Errorf("store: 获取连接失败: %w", err)
	}
	defer conn.Release()

	// 防止多个实例同时启动时并发建表
	if _, err := conn.Exec(ctx, `SELECT pg_advisory_lock($1)`, migrateLockKey); err != nil {
		return fmt.Errorf("store: 获取迁移锁失败: %w", err)
	}
	defer func() {
		_, _ = conn.Exec(context.WithoutCancel(ctx),
			`SELECT pg_advisory_unlock($1)`, migrateLockKey)
	}()

	if _, err := conn.Exec(ctx, `
		CREATE TABLE IF NOT EXISTS schema_migrations (
			version    INTEGER     PRIMARY KEY,
			name       TEXT        NOT NULL,
			applied_at TIMESTAMPTZ NOT NULL DEFAULT now()
		)`); err != nil {
		return fmt.Errorf("store: 创建迁移记录表失败: %w", err)
	}

	applied := make(map[int]bool)
	rows, err := conn.Query(ctx, `SELECT version FROM schema_migrations`)
	if err != nil {
		return fmt.Errorf("store: 读取迁移记录失败: %w", err)
	}
	for rows.Next() {
		var v int
		if err := rows.Scan(&v); err != nil {
			rows.Close()
			return fmt.Errorf("store: 读取迁移记录失败: %w", err)
		}
		applied[v] = true
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return fmt.Errorf("store: 读取迁移记录失败: %w", err)
	}

	for _, m := range ms {
		if applied[m.version] {
			continue
		}
		// 每个迁移单独一个事务：失败即整体回滚，不留半截 schema
		err := pgx.BeginFunc(ctx, conn, func(tx pgx.Tx) error {
			if _, err := tx.Exec(ctx, m.sql); err != nil {
				return err
			}
			_, err := tx.Exec(ctx,
				`INSERT INTO schema_migrations (version, name) VALUES ($1, $2)`,
				m.version, m.name)
			return err
		})
		if err != nil {
			return fmt.Errorf("store: 执行迁移 %03d_%s 失败: %w", m.version, m.name, err)
		}
	}
	return nil
}

// SchemaVersion 返回已应用的最高迁移版本。未迁移过的库返回 0。
//
// 先单独查一次表是否存在：PostgreSQL 在计划阶段就会因为表不存在而报错，
// 把存在性判断塞进同一条语句的 WHERE 里救不回来。两次往返换一个不靠
// 匹配错误信息文本的判断，值得。
func (s *Store) SchemaVersion(ctx context.Context) (int, error) {
	var exists bool
	if err := s.pool.QueryRow(ctx,
		`SELECT to_regclass(current_schema() || '.schema_migrations') IS NOT NULL`,
	).Scan(&exists); err != nil {
		return 0, fmt.Errorf("store: 检查迁移记录表失败: %w", err)
	}
	if !exists {
		return 0, nil
	}

	var v *int // 空表时 max() 返回 NULL
	if err := s.pool.QueryRow(ctx,
		`SELECT max(version) FROM schema_migrations`).Scan(&v); err != nil {
		return 0, fmt.Errorf("store: 查询 schema 版本失败: %w", err)
	}
	if v == nil {
		return 0, nil
	}
	return *v, nil
}

// LatestSchemaVersion 返回二进制里内置的最高迁移版本。
func LatestSchemaVersion() (int, error) {
	ms, err := loadMigrations()
	if err != nil {
		return 0, err
	}
	if len(ms) == 0 {
		return 0, nil
	}
	return ms[len(ms)-1].version, nil
}

// loadMigrations 读出内嵌的迁移文件并按版本号排序。
func loadMigrations() ([]migration, error) {
	entries, err := migrationFS.ReadDir("migrations")
	if err != nil {
		return nil, fmt.Errorf("store: 读取内嵌迁移失败: %w", err)
	}

	out := make([]migration, 0, len(entries))
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".sql") {
			continue
		}
		// 文件名形如 001_init.sql
		base := strings.TrimSuffix(e.Name(), ".sql")
		num, name, ok := strings.Cut(base, "_")
		if !ok {
			return nil, fmt.Errorf("store: 迁移文件名 %q 不合规，应形如 001_init.sql", e.Name())
		}
		v, err := strconv.Atoi(num)
		if err != nil {
			return nil, fmt.Errorf("store: 迁移文件名 %q 的版本号非法: %w", e.Name(), err)
		}
		data, err := migrationFS.ReadFile("migrations/" + e.Name())
		if err != nil {
			return nil, fmt.Errorf("store: 读取迁移 %s 失败: %w", e.Name(), err)
		}
		out = append(out, migration{version: v, name: name, sql: string(data)})
	}

	sort.Slice(out, func(i, j int) bool { return out[i].version < out[j].version })
	for i := range out {
		if out[i].version != i+1 {
			return nil, fmt.Errorf("store: 迁移版本号不连续，第 %d 个是 %d", i+1, out[i].version)
		}
	}
	return out, nil
}
