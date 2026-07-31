package logging_test

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/MrZhongzq/MagicalDanmaku/server/internal/logging"
	"github.com/MrZhongzq/MagicalDanmaku/server/internal/store"
)

// collector 是记录 flush 调用的假实现。
type collector struct {
	mu     sync.Mutex
	rows   []store.ActivityRow
	calls  int           // flush 被调用的次数
	err    error         // 非 nil 时每次 flush 都失败
	notify chan struct{} // 每次 flush 后发一个信号，测试用来等待而不必 sleep
}

func newCollector() *collector {
	return &collector{notify: make(chan struct{}, 64)}
}

func (c *collector) flush(_ context.Context, rows []store.ActivityRow) error {
	c.mu.Lock()
	c.calls++
	if c.err == nil {
		c.rows = append(c.rows, rows...)
	}
	err := c.err
	c.mu.Unlock()

	select {
	case c.notify <- struct{}{}:
	default:
	}
	return err
}

func (c *collector) snapshot() ([]store.ActivityRow, int) {
	c.mu.Lock()
	defer c.mu.Unlock()
	out := make([]store.ActivityRow, len(c.rows))
	copy(out, c.rows)
	return out, c.calls
}

// waitFlush 等一次 flush 发生，超时则让测试失败。
func (c *collector) waitFlush(t *testing.T) {
	t.Helper()
	select {
	case <-c.notify:
	case <-time.After(3 * time.Second):
		t.Fatal("等待 flush 超时")
	}
}

func TestActivityWriterFlushesOnClose(t *testing.T) {
	c := newCollector()
	w := logging.NewActivityWriter(logging.ActivityWriterOptions{
		Flush:     c.flush,
		BatchSize: 1000,      // 高到不会因条数触发
		Interval:  time.Hour, // 长到不会因时间触发
	})

	w.Enqueue(store.ActivityRow{EventType: "danmaku"})
	w.Enqueue(store.ActivityRow{EventType: "gift"})
	w.Close()

	rows, calls := c.snapshot()
	if len(rows) != 2 {
		t.Fatalf("Close 应冲刷剩余，实际写出 %d 条（flush 调用 %d 次）", len(rows), calls)
	}
	if rows[0].EventType != "danmaku" || rows[1].EventType != "gift" {
		t.Errorf("顺序不对: %+v", rows)
	}
}

func TestActivityWriterFlushesOnBatchSize(t *testing.T) {
	c := newCollector()
	w := logging.NewActivityWriter(logging.ActivityWriterOptions{
		Flush:     c.flush,
		BatchSize: 3,
		Interval:  time.Hour,
	})
	defer w.Close()

	for i := 0; i < 3; i++ {
		w.Enqueue(store.ActivityRow{EventType: "danmaku"})
	}
	c.waitFlush(t)

	rows, _ := c.snapshot()
	if len(rows) != 3 {
		t.Errorf("攒够 3 条应立即写出，实际 %d 条", len(rows))
	}
}

func TestActivityWriterFlushesOnInterval(t *testing.T) {
	c := newCollector()
	w := logging.NewActivityWriter(logging.ActivityWriterOptions{
		Flush:     c.flush,
		BatchSize: 1000,
		Interval:  50 * time.Millisecond,
	})
	defer w.Close()

	w.Enqueue(store.ActivityRow{EventType: "danmaku"})
	c.waitFlush(t)

	rows, _ := c.snapshot()
	if len(rows) != 1 {
		t.Errorf("到时间应写出，实际 %d 条", len(rows))
	}
}

func TestActivityWriterDoesNotFlushEmptyBatches(t *testing.T) {
	// 没有事件时不该每 200 毫秒空转一次数据库
	c := newCollector()
	w := logging.NewActivityWriter(logging.ActivityWriterOptions{
		Flush:     c.flush,
		BatchSize: 1000,
		Interval:  20 * time.Millisecond,
	})
	time.Sleep(150 * time.Millisecond)
	w.Close()

	_, calls := c.snapshot()
	if calls != 0 {
		t.Errorf("空缓冲不该触发 flush，实际调用了 %d 次", calls)
	}
}

// 缓冲满时丢弃并计数——丢日志可以接受，漏欢迎不行
func TestActivityWriterDropsWhenBufferFull(t *testing.T) {
	blocked := make(chan struct{})
	release := make(chan struct{})
	var once sync.Once

	w := logging.NewActivityWriter(logging.ActivityWriterOptions{
		Flush: func(context.Context, []store.ActivityRow) error {
			once.Do(func() { close(blocked) })
			<-release // 卡住写入 goroutine，缓冲会被填满
			return nil
		},
		BufferSize: 4,
		BatchSize:  1,
		Interval:   time.Hour,
	})

	w.Enqueue(store.ActivityRow{EventType: "first"})
	<-blocked // 确认写入 goroutine 已经卡在 flush 里

	// 缓冲只有 4 个位置，塞 200 条必然溢出
	for i := 0; i < 200; i++ {
		w.Enqueue(store.ActivityRow{EventType: "danmaku"})
	}

	if w.Dropped() == 0 {
		t.Error("缓冲满时应丢弃并计数")
	}

	close(release)
	w.Close()
}

// Enqueue 绝不能阻塞：它跑在规则引擎的关键路径上
func TestActivityWriterEnqueueNeverBlocks(t *testing.T) {
	release := make(chan struct{})
	w := logging.NewActivityWriter(logging.ActivityWriterOptions{
		Flush: func(context.Context, []store.ActivityRow) error {
			<-release
			return nil
		},
		BufferSize: 2,
		BatchSize:  1,
		Interval:   time.Hour,
	})

	done := make(chan struct{})
	go func() {
		for i := 0; i < 5000; i++ {
			w.Enqueue(store.ActivityRow{EventType: "danmaku"})
		}
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("Enqueue 阻塞了：它跑在规则引擎的关键路径上，绝不能阻塞")
	}

	close(release)
	w.Close()
}

func TestActivityWriterSurvivesFlushError(t *testing.T) {
	c := newCollector()
	c.err = context.DeadlineExceeded

	w := logging.NewActivityWriter(logging.ActivityWriterOptions{
		Flush:     c.flush,
		BatchSize: 1,
		Interval:  time.Hour,
	})

	w.Enqueue(store.ActivityRow{EventType: "danmaku"})
	c.waitFlush(t)

	// 写入失败后还能继续接收，不该 panic 也不该卡死
	w.Enqueue(store.ActivityRow{EventType: "gift"})
	c.waitFlush(t)
	w.Close()

	_, calls := c.snapshot()
	if calls < 2 {
		t.Errorf("写入失败后应继续工作，flush 只被调用了 %d 次", calls)
	}
}

// 落库失败整批都没了，必须计入 Dropped()——否则这个计数器就不能代表
// 实际丢失量。这条路径是可达的：CopyFrom 全批原子，一行坏数据
// （比如已被删除的 binding_id）会让整批一起失败，每个 flush 周期复发。
func TestActivityWriterCountsDroppedOnFlushError(t *testing.T) {
	const n = 5
	w := logging.NewActivityWriter(logging.ActivityWriterOptions{
		Flush: func(context.Context, []store.ActivityRow) error {
			return context.DeadlineExceeded // 恒失败
		},
		BatchSize: 1000,
		Interval:  time.Hour,
	})

	for i := 0; i < n; i++ {
		w.Enqueue(store.ActivityRow{EventType: "danmaku"})
	}
	w.Close()

	if got := w.Dropped(); got < n {
		t.Errorf("flush 恒失败时应把整批计入 Dropped()，投递 %d 条，Dropped()=%d", n, got)
	}
}

func TestActivityWriterCloseIsIdempotent(t *testing.T) {
	c := newCollector()
	w := logging.NewActivityWriter(logging.ActivityWriterOptions{Flush: c.flush})
	w.Close()
	w.Close() // 不应 panic
}

func TestActivityWriterEnqueueAfterCloseIsSafe(t *testing.T) {
	c := newCollector()
	w := logging.NewActivityWriter(logging.ActivityWriterOptions{Flush: c.flush})
	w.Close()
	w.Enqueue(store.ActivityRow{EventType: "danmaku"}) // 不应 panic 或写入已关闭的 channel
}

func TestActivityWriterConcurrentEnqueue(t *testing.T) {
	c := newCollector()
	w := logging.NewActivityWriter(logging.ActivityWriterOptions{
		Flush:      c.flush,
		BufferSize: 8192,
		BatchSize:  100,
		Interval:   10 * time.Millisecond,
	})

	var wg sync.WaitGroup
	for g := 0; g < 8; g++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < 200; i++ {
				w.Enqueue(store.ActivityRow{EventType: "danmaku"})
			}
		}()
	}
	wg.Wait()
	w.Close()

	rows, _ := c.snapshot()
	if int64(len(rows))+w.Dropped() != 1600 {
		t.Errorf("写出 %d 条 + 丢弃 %d 条 ≠ 投递的 1600 条", len(rows), w.Dropped())
	}
}

// 排空是确定性可测的：让首次 flush 阻塞住写入 goroutine，
// 后续投递就只能堆在 channel 里。若删掉停止分支的排空逻辑，
// 这些行会全部丢失，本测试必然失败。
func TestActivityWriterDrainsChannelOnClose(t *testing.T) {
	c := newCollector()
	release := make(chan struct{})
	blocked := make(chan struct{})
	var once sync.Once

	w := logging.NewActivityWriter(logging.ActivityWriterOptions{
		Flush: func(ctx context.Context, rows []store.ActivityRow) error {
			once.Do(func() { close(blocked) })
			<-release
			return c.flush(ctx, rows)
		},
		BufferSize: 64,
		BatchSize:  1,         // 首条立即触发 flush，把写入 goroutine 卡住
		Interval:   time.Hour, // 不让定时器插手
	})

	w.Enqueue(store.ActivityRow{EventType: "第一条"})
	<-blocked // 确认写入 goroutine 已卡在 flush 里

	// 这些只能堆在 channel 里，因为写入 goroutine 动不了
	for i := 0; i < 20; i++ {
		w.Enqueue(store.ActivityRow{EventType: "堆积"})
	}

	close(release)
	w.Close()

	rows, _ := c.snapshot()
	if int64(len(rows))+w.Dropped() != 21 {
		t.Errorf("写出 %d 条 + 丢弃 %d 条 ≠ 投递的 21 条", len(rows), w.Dropped())
	}
	if w.Dropped() != 0 {
		t.Errorf("缓冲 64 装得下 21 条，不该有丢弃，实际丢了 %d 条", w.Dropped())
	}
}

// Enqueue 与 Close 并发是真实关停场景，必须在 -race 下跑过。
// 本测试不断言条数（关停时刻本就不确定），只断言不 panic、不卡死。
func TestActivityWriterCloseDuringConcurrentEnqueue(t *testing.T) {
	c := newCollector()
	w := logging.NewActivityWriter(logging.ActivityWriterOptions{
		Flush:      c.flush,
		BufferSize: 256,
		BatchSize:  16,
		Interval:   time.Millisecond,
	})

	var wg sync.WaitGroup
	for g := 0; g < 4; g++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < 500; i++ {
				w.Enqueue(store.ActivityRow{EventType: "danmaku"})
			}
		}()
	}

	// 生产者仍在跑的时候关闭
	time.Sleep(5 * time.Millisecond)
	w.Close()
	wg.Wait() // 关闭后继续投递不得 panic
}
