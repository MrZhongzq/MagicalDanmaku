// Package logging 装配两类日志。
//
// 系统日志走 stderr 与滚动文件（见 system.go）：数据库连不上时，
// 「数据库连不上」这条日志本身还得写得出来。
//
// 业务日志进数据库（见本文件）：P4 要展示、P5 要统计，必须结构化可查。
package logging

import (
	"context"
	"log/slog"
	"sync"
	"sync/atomic"
	"time"

	"github.com/MrZhongzq/MagicalDanmaku/server/internal/store"
)

// 写入器的默认参数。
const (
	defaultBufferSize         = 4096
	defaultBatchSize          = 500
	defaultInterval           = 200 * time.Millisecond
	defaultDropReportInterval = 30 * time.Second
	flushTimeout              = 10 * time.Second
)

// ActivityWriterOptions 配置业务日志写入器。
type ActivityWriterOptions struct {
	// Flush 把一批日志落库。注入而非直接持有 *store.Store，
	// 是为了让写入器的批量与丢弃逻辑不需要数据库就能测。
	//
	// 传入的切片只在本次调用期间有效：底层数组会被下一批复用。
	// 实现若需要在返回后继续持有这批数据，必须自己拷贝。
	Flush func(context.Context, []store.ActivityRow) error

	BufferSize         int           // 缓冲条数，0 用默认
	BatchSize          int           // 攒够多少条就写一次，0 用默认
	Interval           time.Duration // 攒不够也最多等这么久，0 用默认
	DropReportInterval time.Duration // 多久汇总一次丢弃量，0 用默认
	Logger             *slog.Logger
}

// ActivityWriter 把业务日志异步批量写进数据库。
//
// 活跃房间每秒几十条事件，同步 INSERT 会把数据库延迟压到规则引擎的
// 关键路径上。因此 Enqueue 永不阻塞：缓冲满了就丢弃并计数。
//
// 丢日志可以接受，漏欢迎不行——这个优先级不要改。
type ActivityWriter struct {
	ch      chan store.ActivityRow
	flush   func(context.Context, []store.ActivityRow) error
	batch   int
	tick    time.Duration
	report  time.Duration
	log     *slog.Logger
	dropped atomic.Int64

	stop      chan struct{}
	done      chan struct{}
	closeOnce sync.Once
}

// NewActivityWriter 创建写入器并启动后台 goroutine。
func NewActivityWriter(opts ActivityWriterOptions) *ActivityWriter {
	if opts.BufferSize <= 0 {
		opts.BufferSize = defaultBufferSize
	}
	if opts.BatchSize <= 0 {
		opts.BatchSize = defaultBatchSize
	}
	if opts.Interval <= 0 {
		opts.Interval = defaultInterval
	}
	if opts.DropReportInterval <= 0 {
		opts.DropReportInterval = defaultDropReportInterval
	}
	if opts.Logger == nil {
		opts.Logger = slog.Default()
	}

	w := &ActivityWriter{
		ch:     make(chan store.ActivityRow, opts.BufferSize),
		flush:  opts.Flush,
		batch:  opts.BatchSize,
		tick:   opts.Interval,
		report: opts.DropReportInterval,
		log:    opts.Logger,
		stop:   make(chan struct{}),
		done:   make(chan struct{}),
	}
	go w.run()
	return w
}

// Enqueue 投递一条业务日志。永不阻塞：缓冲满则丢弃并计数。
func (w *ActivityWriter) Enqueue(row store.ActivityRow) {
	select {
	case <-w.stop:
		// 已关闭，计入丢弃：约束是「丢弃并计数」，这条路径也不例外。
		// 不记日志——关停后可能有大量事件涌入，逐条打日志会淹没退出过程，
		// 计数会在 Close 的汇总里体现。
		w.dropped.Add(1)
		return
	default:
	}

	select {
	case w.ch <- row:
	default:
		w.dropped.Add(1)
	}
}

// Dropped 返回累计丢弃条数。
func (w *ActivityWriter) Dropped() int64 {
	return w.dropped.Load()
}

// Close 停止接收并冲刷剩余日志。可重复调用。
func (w *ActivityWriter) Close() {
	w.closeOnce.Do(func() {
		close(w.stop)
		<-w.done
		if n := w.dropped.Load(); n > 0 {
			w.log.Warn("业务日志有丢弃", "累计丢弃", n)
		}
	})
}

// run 是后台写入循环。
func (w *ActivityWriter) run() {
	defer close(w.done)

	ticker := time.NewTicker(w.tick)
	defer ticker.Stop()
	reporter := time.NewTicker(w.report)
	defer reporter.Stop()

	buf := make([]store.ActivityRow, 0, w.batch)
	lastReported := int64(0)

	for {
		select {
		case row := <-w.ch:
			buf = append(buf, row)
			if len(buf) >= w.batch {
				buf = w.write(buf)
			}

		case <-ticker.C:
			// 空缓冲不写：没有事件时不该每 200 毫秒空转一次数据库
			if len(buf) > 0 {
				buf = w.write(buf)
			}

		case <-reporter.C:
			if n := w.dropped.Load(); n > lastReported {
				w.log.Warn("业务日志缓冲已满，部分记录被丢弃",
					"本轮丢弃", n-lastReported, "累计丢弃", n)
				lastReported = n
			}

		case <-w.stop:
			// 排空 channel 里剩下的，再写最后一批
			for {
				select {
				case row := <-w.ch:
					buf = append(buf, row)
					if len(buf) >= w.batch {
						buf = w.write(buf)
					}
					continue
				default:
				}
				break
			}
			if len(buf) > 0 {
				w.write(buf)
			}
			// 排空之后仍可能有 Enqueue 在竞争窗口里投进来
			// （它先查 stop 再发送，两步之间可能被抢占），一并计入丢弃。
			// 这个清扫本身有固有的残余竞争——清扫之后还能再投进来——
			// 无法根除，也不值得为此再加一轮循环。
			for {
				select {
				case <-w.ch:
					w.dropped.Add(1)
					continue
				default:
				}
				break
			}
			return
		}
	}
}

// write 落一批日志，返回清空后的缓冲以复用底层数组。
//
// 写入失败只记日志：业务日志不是关键路径，失败不该拖垮机器人。
func (w *ActivityWriter) write(buf []store.ActivityRow) []store.ActivityRow {
	ctx, cancel := context.WithTimeout(context.Background(), flushTimeout)
	defer cancel()

	if err := w.flush(ctx, buf); err != nil {
		// 落库失败整批都没了，计入丢弃——设计承诺「丢弃要计数」，
		// 只记日志的话 Dropped() 就代表不了实际丢失量。
		// 这条路径是可达的：activity_logs.binding_id 有外键，run 期间
		// 删掉一个绑定后它的引擎还在投递，CopyFrom 全批原子，
		// 一行坏数据会让整批（含其他绑定的行）一起失败。
		w.dropped.Add(int64(len(buf)))
		w.log.Error("写入业务日志失败", "条数", len(buf), "err", err)
	}
	return buf[:0]
}
