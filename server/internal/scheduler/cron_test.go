package scheduler

import (
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestValidateSpecAcceptsSixFields(t *testing.T) {
	// 6 字段含秒：秒 分 时 日 月 周
	for _, spec := range []string{
		"*/1 * * * * *",
		"0 */5 * * * *",
		"30 0 12 * * *",
		"0 0 0 1 1 *",
	} {
		if err := ValidateSpec(spec); err != nil {
			t.Errorf("%q 应为合法表达式: %v", spec, err)
		}
	}
}

func TestValidateSpecRejectsInvalid(t *testing.T) {
	for _, spec := range []string{
		"",
		"不是表达式",
		"* * *",
		"99 * * * * *",
	} {
		if err := ValidateSpec(spec); err == nil {
			t.Errorf("%q 应被拒绝", spec)
		}
	}
}

func TestSchedulerRunsJob(t *testing.T) {
	s := New(nil)
	var n int32
	if err := s.Add("*/1 * * * * *", "每秒任务", func() {
		atomic.AddInt32(&n, 1)
	}); err != nil {
		t.Fatalf("Add 失败: %v", err)
	}

	s.Start()
	time.Sleep(2500 * time.Millisecond)
	s.Stop()

	if got := atomic.LoadInt32(&n); got < 2 {
		t.Errorf("2.5 秒内应至少触发 2 次，实际 %d", got)
	}
}

func TestSchedulerAddRejectsBadSpec(t *testing.T) {
	s := New(nil)
	if err := s.Add("坏表达式", "任务", func() {}); err == nil {
		t.Error("非法表达式应当报错")
	}
}

func TestSchedulerStopHaltsJobs(t *testing.T) {
	s := New(nil)
	var n int32
	s.Add("*/1 * * * * *", "任务", func() { atomic.AddInt32(&n, 1) })

	s.Start()
	time.Sleep(1200 * time.Millisecond)
	s.Stop()

	after := atomic.LoadInt32(&n)
	time.Sleep(1500 * time.Millisecond)
	if got := atomic.LoadInt32(&n); got != after {
		t.Errorf("Stop 后不应再触发，%d → %d", after, got)
	}
}

func TestSchedulerPanicInJobDoesNotCrash(t *testing.T) {
	s := New(nil)
	var ok int32
	// 第一个任务 panic，不得影响第二个
	s.Add("*/1 * * * * *", "会 panic 的任务", func() { panic("故意的") })
	s.Add("*/1 * * * * *", "正常任务", func() { atomic.AddInt32(&ok, 1) })

	s.Start()
	time.Sleep(2500 * time.Millisecond)
	s.Stop()

	if atomic.LoadInt32(&ok) < 2 {
		t.Errorf("panic 的任务不应拖垮其他任务，正常任务只跑了 %d 次", ok)
	}
}

func TestSchedulerConcurrentAdd(t *testing.T) {
	s := New(nil)
	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			s.Add("0 0 0 1 1 *", "任务", func() {})
		}()
	}
	wg.Wait()
}
