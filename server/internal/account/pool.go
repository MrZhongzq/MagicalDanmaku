// Package account 管理多个 B 站账号的轮换发言。
package account

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync"

	"github.com/MrZhongzq/MagicalDanmaku/server/internal/connector"
	"github.com/MrZhongzq/MagicalDanmaku/server/internal/connector/bilibili"
)

// ErrNoHealthyAccount 表示没有可用账号。
var ErrNoHealthyAccount = errors.New("account: 没有可用的账号")

// Account 是一个可发言的账号。
type Account struct {
	Name    string
	Actions connector.Actions
}

// Pool 轮询多个账号发送动作，绕开单账号的频率限制。
//
// 账号被 P0 的 IsFatal 判定为致命错误（-101 未登录、-111 csrf 失效、
// 1003 已被禁言）后移出轮换。全部账号失效时返回错误而非静默丢弃——
// 静默失败会让使用者以为机器人在正常工作。
type Pool struct {
	log *slog.Logger

	mu      sync.Mutex
	entries []*entry
	next    int
}

// entry 是池中的一个账号及其健康状态。
type entry struct {
	acc     Account
	healthy bool
}

// New 创建账号池。
func New(accounts []Account, log *slog.Logger) *Pool {
	if log == nil {
		log = slog.Default()
	}
	p := &Pool{log: log}
	for _, a := range accounts {
		p.entries = append(p.entries, &entry{acc: a, healthy: true})
	}
	return p
}

// Healthy 返回当前可用账号数。
func (p *Pool) Healthy() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	n := 0
	for _, e := range p.entries {
		if e.healthy {
			n++
		}
	}
	return n
}

// SendDanmaku 轮询选账号发送弹幕，遇致命错误自动切换下一个。
func (p *Pool) SendDanmaku(ctx context.Context, roomID, text string) error {
	return p.do(ctx, func(a connector.Actions) error {
		return a.SendDanmaku(ctx, connector.SendDanmakuRequest{RoomID: roomID, Text: text})
	})
}

// Block 轮询选账号执行禁言。
func (p *Pool) Block(ctx context.Context, roomID, uid string, hours int) error {
	return p.do(ctx, func(a connector.Actions) error {
		return a.BlockUser(ctx, connector.BlockRequest{RoomID: roomID, UID: uid, Hours: hours})
	})
}

// do 在健康账号上执行操作，遇致命错误剔除该账号并重试下一个。
//
// 最多尝试与账号数相同的次数，避免全部失效时无限循环。
func (p *Pool) do(ctx context.Context, fn func(connector.Actions) error) error {
	total := len(p.entries)
	if total == 0 {
		return ErrNoHealthyAccount
	}

	var lastErr error
	for i := 0; i < total; i++ {
		e := p.pick()
		if e == nil {
			if lastErr != nil {
				return fmt.Errorf("account: 全部账号均已失效: %w", lastErr)
			}
			return ErrNoHealthyAccount
		}

		err := fn(e.acc.Actions)
		if err == nil {
			return nil
		}
		lastErr = err

		if bilibili.IsFatal(err) {
			p.markUnhealthy(e)
			p.log.Error("账号已失效，移出轮换",
				"account", e.acc.Name, "err", err, "healthy", p.Healthy())
			continue // 换下一个账号重试
		}
		// 可重试错误（如 10030 发送过快）：保留账号，直接上报
		return err
	}

	if lastErr != nil {
		return fmt.Errorf("account: 全部账号均已失效: %w", lastErr)
	}
	return ErrNoHealthyAccount
}

// pick 轮询取下一个健康账号，全部失效时返回 nil。
func (p *Pool) pick() *entry {
	p.mu.Lock()
	defer p.mu.Unlock()

	n := len(p.entries)
	for i := 0; i < n; i++ {
		e := p.entries[p.next%n]
		p.next = (p.next + 1) % n
		if e.healthy {
			return e
		}
	}
	return nil
}

// markUnhealthy 把账号标记为失效。
func (p *Pool) markUnhealthy(e *entry) {
	p.mu.Lock()
	defer p.mu.Unlock()
	e.healthy = false
}
