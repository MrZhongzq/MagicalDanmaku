// Package ratelimit 提供动作发送的限流机制。
//
// 本包只提供机制，不定策略：冷却通道、优先级、去重等业务策略
// 属于规则引擎（P2）的职责。
package ratelimit

import (
	"context"
	"sync"
	"time"
)

// Limiter 控制动作的发送节奏。
type Limiter interface {
	// Wait 阻塞到允许发送为止；ctx 取消时返回其错误。
	Wait(ctx context.Context) error

	// SetInterval 原地改最小发送间隔。**必须原地改，不能靠调用方换一个
	// 新实例**——这个 Limiter 通常被同一账号名下的多个绑定共享
	// （account.Account.Limiter），换一个新实例只有拿着新指针的那一方
	// 看得到，其余绑定仍握着旧实例，等于没变。供账号参数保存后热传播
	// 给运行中的绑定使用（P6 任务 3：此前 SetMaxLength/发送间隔只在
	// 装配时生效一次，改了不重启不生效）。
	SetInterval(d time.Duration)
}

// intervalLimiter 保证相邻两次放行之间至少间隔 d。
type intervalLimiter struct {
	mu   sync.Mutex
	d    time.Duration
	next time.Time
}

// NewInterval 创建一个最小间隔限流器。d 为 0 或负数时不做限制。
func NewInterval(d time.Duration) Limiter {
	return &intervalLimiter{d: d}
}

func (l *intervalLimiter) Wait(ctx context.Context) error {
	l.mu.Lock()
	d := l.d // 必须在锁内读：SetInterval 并发写这个字段，锁外读是数据竞争
	if d <= 0 {
		l.mu.Unlock()
		return ctx.Err()
	}

	now := time.Now()
	wait := time.Duration(0)
	if now.Before(l.next) {
		wait = l.next.Sub(now)
		l.next = l.next.Add(d)
	} else {
		l.next = now.Add(d)
	}
	l.mu.Unlock()

	if wait <= 0 {
		return ctx.Err()
	}
	t := time.NewTimer(wait)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-t.C:
		return nil
	}
}

// SetInterval 原地改间隔，并把已经排定的下一个放行时刻按新旧间隔的差值
// 重新锚定。
//
// 只改 d 不够：假设旧间隔是 2 秒，上一次 Wait 已经把 next 定在「上次放行
// 时刻 + 2 秒」，此时把间隔改成 50 毫秒，如果不动 next，下一次 Wait 依然
// 会傻等到那个按旧间隔算出来的时刻——用户刚保存的新间隔要再等满一个旧
// 周期才看得出效果，这与"热传播"的初衷矛盾。next-旧间隔 就是上一次实际
// 放行的时刻，加上新间隔即为按新节奏本该排到的下一个时刻，这样调小间隔
// 立刻生效、调大间隔也不会让新间隔生效前那一下提前放行。
//
// next 为零值（从未 Wait 过）时不做任何调整：首次调用永远立即放行，
// 这条约束不因为中途调用过 SetInterval 而改变。
func (l *intervalLimiter) SetInterval(d time.Duration) {
	l.mu.Lock()
	if !l.next.IsZero() {
		l.next = l.next.Add(d - l.d)
	}
	l.d = d
	l.mu.Unlock()
}
