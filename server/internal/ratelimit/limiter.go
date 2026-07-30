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
	if l.d <= 0 {
		return ctx.Err()
	}

	l.mu.Lock()
	now := time.Now()
	wait := time.Duration(0)
	if now.Before(l.next) {
		wait = l.next.Sub(now)
		l.next = l.next.Add(l.d)
	} else {
		l.next = now.Add(l.d)
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
