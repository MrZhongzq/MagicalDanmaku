// Package store 是 PostgreSQL 存储层。
//
// 只有这一个实现，因此不做接口抽象——为不存在的第二实现提前抽接口，
// 只会换来一层无人受益的间接。需要替身的上层测试请在自己的测试文件里
// 定义所需的最小接口，Go 的隐式接口正是为此而生。
package store

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Store 持有连接池。并发安全，全进程共用一个即可。
type Store struct {
	pool *pgxpool.Pool
}

// Open 连接数据库。dsn 形如
// postgres://user:pass@host:5432/dbname?sslmode=disable
//
// 注意 Cookie 以明文存储：若数据库不在本机且未启用 TLS，
// Cookie 每次读取都会明文过网络。
func Open(ctx context.Context, dsn string) (*Store, error) {
	return OpenWithSchema(ctx, dsn, "")
}

// OpenWithSchema 连接数据库并指定 schema。schema 为空则用连接默认值。
//
// 指定 schema 的能力服务于测试隔离：每个测试用例在自己的 schema 里建表，
// 测完整个 drop 掉，因此可以并行且互不干扰。httpapi 包的测试也需要它，
// 所以它是导出的。
func OpenWithSchema(ctx context.Context, dsn, schema string) (*Store, error) {
	cfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		return nil, fmt.Errorf("store: 数据库连接串非法: %w", err)
	}
	if schema != "" {
		cfg.ConnConfig.RuntimeParams["search_path"] = schema
	}

	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("store: 连接数据库失败: %w", err)
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("store: 数据库不可达: %w", err)
	}
	return &Store{pool: pool}, nil
}

// Close 关闭连接池。
func (s *Store) Close() {
	s.pool.Close()
}
