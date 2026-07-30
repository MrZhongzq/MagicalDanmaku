package ratelimit

import (
	"context"
	"testing"
	"time"
)

func TestIntervalLimiterFirstCallIsImmediate(t *testing.T) {
	l := NewInterval(100 * time.Millisecond)
	start := time.Now()
	if err := l.Wait(context.Background()); err != nil {
		t.Fatalf("Wait 失败: %v", err)
	}
	if d := time.Since(start); d > 20*time.Millisecond {
		t.Errorf("首次调用应立即返回，实际耗时 %v", d)
	}
}

func TestIntervalLimiterEnforcesGap(t *testing.T) {
	l := NewInterval(80 * time.Millisecond)
	ctx := context.Background()

	if err := l.Wait(ctx); err != nil {
		t.Fatalf("Wait 失败: %v", err)
	}
	start := time.Now()
	if err := l.Wait(ctx); err != nil {
		t.Fatalf("Wait 失败: %v", err)
	}
	if d := time.Since(start); d < 60*time.Millisecond {
		t.Errorf("第二次调用应等待约 80ms，实际 %v", d)
	}
}

func TestIntervalLimiterRespectsContext(t *testing.T) {
	l := NewInterval(5 * time.Second)
	ctx := context.Background()
	if err := l.Wait(ctx); err != nil {
		t.Fatalf("Wait 失败: %v", err)
	}

	ctx2, cancel := context.WithTimeout(ctx, 50*time.Millisecond)
	defer cancel()
	if err := l.Wait(ctx2); err == nil {
		t.Error("ctx 超时后 Wait 应返回错误")
	}
}

func TestIntervalLimiterZeroIsNoop(t *testing.T) {
	l := NewInterval(0)
	ctx := context.Background()
	start := time.Now()
	for i := 0; i < 5; i++ {
		if err := l.Wait(ctx); err != nil {
			t.Fatalf("Wait 失败: %v", err)
		}
	}
	if d := time.Since(start); d > 20*time.Millisecond {
		t.Errorf("间隔为 0 时不应等待，实际 %v", d)
	}
}
