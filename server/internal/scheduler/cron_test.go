package scheduler

import (
	"io"
	"log/slog"
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

// 同名任务重复注册，只应留下最后那一个。
//
// 热重载时同一条规则可能改了 cron 表达式。旧条目不移除的话，两个
// 条目都指向新引擎，规则会在新旧两个时刻各触发一次——多发一条弹幕，
// 而且不报任何错。
func TestSchedulerAddReplacesSameName(t *testing.T) {
	s := New(slog.New(slog.NewTextHandler(io.Discard, nil)))

	var mu sync.Mutex
	hits := map[string]int{}
	record := func(which string) func() {
		return func() {
			mu.Lock()
			hits[which]++
			mu.Unlock()
		}
	}

	if err := s.Add("@every 100ms", "甲/问候", record("旧")); err != nil {
		t.Fatalf("注册报错: %v", err)
	}
	if err := s.Add("@every 100ms", "甲/问候", record("新")); err != nil {
		t.Fatalf("重复注册报错: %v", err)
	}

	s.Start()
	defer s.Stop()
	time.Sleep(350 * time.Millisecond)

	mu.Lock()
	defer mu.Unlock()
	if hits["旧"] != 0 {
		t.Errorf("旧条目仍在触发 %d 次，同名注册应当先移除它", hits["旧"])
	}
	if hits["新"] == 0 {
		t.Error("新条目一次都没触发")
	}
}

// 按前缀移除：热重载以绑定为单位重建，任务名就是「绑定标签/规则名」。
func TestSchedulerRemoveByPrefix(t *testing.T) {
	s := New(slog.New(slog.NewTextHandler(io.Discard, nil)))

	var mu sync.Mutex
	hits := map[string]int{}
	record := func(which string) func() {
		return func() {
			mu.Lock()
			hits[which]++
			mu.Unlock()
		}
	}

	if err := s.Add("@every 100ms", "甲@123/问候", record("甲")); err != nil {
		t.Fatalf("注册报错: %v", err)
	}
	if err := s.Add("@every 100ms", "乙@456/问候", record("乙")); err != nil {
		t.Fatalf("注册报错: %v", err)
	}

	if n := s.RemoveByPrefix("甲@123/"); n != 1 {
		t.Errorf("移除数 = %d, 期望 1", n)
	}

	s.Start()
	defer s.Stop()
	time.Sleep(350 * time.Millisecond)

	mu.Lock()
	defer mu.Unlock()
	if hits["甲"] != 0 {
		t.Errorf("甲已被移除却触发了 %d 次", hits["甲"])
	}
	if hits["乙"] == 0 {
		t.Error("乙不该被前缀「甲@123/」波及")
	}
}

// 移除不存在的前缀是空操作，不报错、不 panic
func TestSchedulerRemoveByPrefixNoMatch(t *testing.T) {
	s := New(slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err := s.Add("@every 1h", "甲/问候", func() {}); err != nil {
		t.Fatalf("注册报错: %v", err)
	}
	if n := s.RemoveByPrefix("没有这个绑定/"); n != 0 {
		t.Errorf("移除数 = %d, 期望 0", n)
	}
}
