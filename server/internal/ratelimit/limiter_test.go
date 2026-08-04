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

// ---- SetInterval：账号参数热更新（P6 任务 3）----
//
// 限流器被同一账号名下全部绑定共享，账号保存新的发送间隔后必须能
// 原地生效，不能只对"往后新建的限流器"有效——那等价于还是要重启/
// 重新装配才生效，与本任务要修的问题（改了不重启不生效）是同一个洞。

// TestIntervalLimiterSetIntervalShrinksPendingWait 覆盖"调小间隔"：
// 已经排定的下一个放行时刻是按旧的大间隔算出来的，如果 SetInterval
// 只改内部的 d 而不把这个已排定的时刻重新锚定，第二次 Wait 仍然会
// 傻等满旧间隔，用户刚保存的新间隔要再等一整个旧周期才看得出效果。
func TestIntervalLimiterSetIntervalShrinksPendingWait(t *testing.T) {
	l := NewInterval(2 * time.Second)
	ctx := context.Background()
	if err := l.Wait(ctx); err != nil {
		t.Fatalf("首次 Wait 失败: %v", err)
	}
	l.SetInterval(50 * time.Millisecond)

	start := time.Now()
	if err := l.Wait(ctx); err != nil {
		t.Fatalf("第二次 Wait 失败: %v", err)
	}
	if d := time.Since(start); d > 300*time.Millisecond {
		t.Errorf("SetInterval 调小间隔后第二次 Wait 耗时 %v，期望约 50ms——"+
			"说明已排定的下一个放行时刻没有跟着新间隔重新锚定，要等满旧的 2 秒才生效", d)
	}
}

// TestIntervalLimiterSetIntervalExtendsPendingWait 是反方向：调大间隔，
// 第二次 Wait 应该按新的更长间隔等待，而不是沿用旧的短间隔提前放行。
func TestIntervalLimiterSetIntervalExtendsPendingWait(t *testing.T) {
	l := NewInterval(30 * time.Millisecond)
	ctx := context.Background()
	if err := l.Wait(ctx); err != nil {
		t.Fatalf("首次 Wait 失败: %v", err)
	}
	l.SetInterval(200 * time.Millisecond)

	start := time.Now()
	if err := l.Wait(ctx); err != nil {
		t.Fatalf("第二次 Wait 失败: %v", err)
	}
	if d := time.Since(start); d < 120*time.Millisecond {
		t.Errorf("SetInterval 调大间隔后第二次 Wait 耗时 %v，期望约 200ms——"+
			"说明沿用了旧的短间隔，新间隔没有生效", d)
	}
}

// TestIntervalLimiterSetIntervalBeforeFirstWaitDoesNotBlock 覆盖边界：
// 从未 Wait 过就先 SetInterval，不该因为重新锚定逻辑而意外产生等待——
// 首次调用永远立即放行，这条约束不因为中途改过间隔而改变。
func TestIntervalLimiterSetIntervalBeforeFirstWaitDoesNotBlock(t *testing.T) {
	l := NewInterval(2 * time.Second)
	l.SetInterval(50 * time.Millisecond)

	ctx := context.Background()
	start := time.Now()
	if err := l.Wait(ctx); err != nil {
		t.Fatalf("Wait 失败: %v", err)
	}
	if d := time.Since(start); d > 20*time.Millisecond {
		t.Errorf("从未 Wait 过时先 SetInterval 不该产生等待，实际 %v", d)
	}
}
