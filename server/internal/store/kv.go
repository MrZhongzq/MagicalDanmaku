package store

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5"
)

// kvTimeout 是脚本存储单次操作的上限。
//
// rules.Storage 的签名不能带 ctx（它要被 goja 同步调用），但函数体里可以
// 自己加。没有这个上限的话，数据库挂住时这个调用永不返回：goja 的
// 200ms Interrupt 拦不住已经进入 Go 的阻塞调用，而这条调用链一路
// 持有引擎锁并挂在 run 的 WaitGroup 上——Ctrl+C 会直接卡死，
// 连 pgxpool.Close() 都等不到连接归还。
const kvTimeout = 3 * time.Second

// BindingStorage 是某个绑定的键值存储，注入给规则脚本使用。
//
// 实现 rules.Storage。该接口的方法不带 ctx 也不返回 error——它要被
// goja 从 JS 里同步调用，签名必须简单。因此这里自带一个有界 context
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
	ctx, cancel := context.WithTimeout(context.Background(), kvTimeout)
	defer cancel()

	var v string
	err := b.store.pool.QueryRow(ctx,
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
	ctx, cancel := context.WithTimeout(context.Background(), kvTimeout)
	defer cancel()

	_, err := b.store.pool.Exec(ctx, `
		INSERT INTO kv_store (binding_id, key, value) VALUES ($1, $2, $3)
		ON CONFLICT (binding_id, key) DO UPDATE SET
			value = EXCLUDED.value, updated_at = now()`,
		b.bindingID, key, value)
	if err != nil {
		b.log.Error("写入脚本存储失败", "binding_id", b.bindingID, "key", key, "err", err)
	}
}
