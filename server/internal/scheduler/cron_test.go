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

// 同名任务重复注册，cron 里只应留下一个条目。
//
// **直接查 cron 的条目表，不靠等它触发。** robfig/cron 的 @every 对
// 小于 1 秒的间隔会静默取整到 1 秒并对齐整秒边界（见其
// constantdelay.go），靠 sleep 判断「触发了几次」取决于测试启动那一刻
// 的秒内相位，是不稳的。而要断言的性质本来就是「旧条目没了」——那是
// 可以直接看的，不必绕道计时。
func TestSchedulerAddReplacesSameName(t *testing.T) {
	s := New(slog.New(slog.NewTextHandler(io.Discard, nil)))

	if err := s.Add("0 0 * * * *", "甲@123/问候", func() {}); err != nil {
		t.Fatalf("注册报错: %v", err)
	}
	first := s.ids["甲@123/问候"]

	// 同一条规则改了 cron 表达式——这正是热重载里会发生的事
	if err := s.Add("0 30 * * * *", "甲@123/问候", func() {}); err != nil {
		t.Fatalf("重复注册报错: %v", err)
	}

	if got := len(s.cron.Entries()); got != 1 {
		t.Errorf("cron 条目数 = %d, 期望 1——留着旧条目的话，两个条目都指向"+
			"新引擎，同一条规则会在新旧两个时刻各触发一次", got)
	}
	if got := len(s.ids); got != 1 {
		t.Errorf("ids 表大小 = %d, 期望 1", got)
	}
	if s.ids["甲@123/问候"] == first {
		t.Error("EntryID 没变，说明第二次 Add 根本没有重新注册")
	}
}

// 按前缀移除：热重载以绑定为单位重建，任务名就是「绑定标签/规则名」。
func TestSchedulerRemoveByPrefix(t *testing.T) {
	s := New(slog.New(slog.NewTextHandler(io.Discard, nil)))

	// 「甲@1234/问候」是故意放的：前缀带尾斜杠才不会把它误伤，
	// 少了那个斜杠就是一条静默删错别人任务的 bug
	for _, name := range []string{
		"甲@123/问候", "甲@123/答谢", "甲@1234/问候", "乙@456/问候",
	} {
		if err := s.Add("0 0 * * * *", name, func() {}); err != nil {
			t.Fatalf("注册 %s 报错: %v", name, err)
		}
	}

	if n := s.RemoveByPrefix("甲@123/"); n != 2 {
		t.Errorf("移除数 = %d, 期望 2", n)
	}
	if got := len(s.cron.Entries()); got != 2 {
		t.Errorf("cron 条目数 = %d, 期望 2", got)
	}
	for _, name := range []string{"甲@123/问候", "甲@123/答谢"} {
		if _, ok := s.ids[name]; ok {
			t.Errorf("%s 应已被移除", name)
		}
	}
	for _, name := range []string{"甲@1234/问候", "乙@456/问候"} {
		if _, ok := s.ids[name]; !ok {
			t.Errorf("%s 不该被前缀「甲@123/」波及", name)
		}
	}
}

// 移除不存在的前缀是空操作，不报错、不 panic
func TestSchedulerRemoveByPrefixNoMatch(t *testing.T) {
	s := New(slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err := s.Add("0 0 * * * *", "甲/问候", func() {}); err != nil {
		t.Fatalf("注册报错: %v", err)
	}
	if n := s.RemoveByPrefix("没有这个绑定/"); n != 0 {
		t.Errorf("移除数 = %d, 期望 0", n)
	}
	if got := len(s.cron.Entries()); got != 1 {
		t.Errorf("cron 条目数 = %d, 期望 1（不该误删）", got)
	}
}
