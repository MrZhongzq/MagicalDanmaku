// Package scheduler 提供基于 cron 表达式的定时任务调度。
package scheduler

import (
	"fmt"
	"log/slog"

	"github.com/robfig/cron/v3"
)

// parser 解析 6 字段表达式：秒 分 时 日 月 周。
//
// 标准 5 字段 cron 最细只到分钟，表达不了「每 30 秒」这类常见需求，
// 因此启用秒级字段。
var parser = cron.NewParser(
	cron.Second | cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow | cron.Descriptor,
)

// ValidateSpec 校验 cron 表达式，供配置加载时预检。
func ValidateSpec(spec string) error {
	if spec == "" {
		return fmt.Errorf("scheduler: cron 表达式不能为空")
	}
	if _, err := parser.Parse(spec); err != nil {
		return fmt.Errorf("scheduler: 非法的 cron 表达式 %q: %w", spec, err)
	}
	return nil
}

// Scheduler 管理一组定时任务。
type Scheduler struct {
	cron *cron.Cron
	log  *slog.Logger
}

// New 创建调度器。
func New(log *slog.Logger) *Scheduler {
	if log == nil {
		log = slog.Default()
	}
	return &Scheduler{
		cron: cron.New(cron.WithParser(parser)),
		log:  log,
	}
}

// Add 注册一个定时任务。
//
// 任务内的 panic 会被捕获并记录，不得拖垮调度器或其他任务——
// 用户脚本的错误不该让整个机器人停摆。
func (s *Scheduler) Add(spec, name string, fn func()) error {
	if err := ValidateSpec(spec); err != nil {
		return err
	}

	wrapped := func() {
		defer func() {
			if r := recover(); r != nil {
				s.log.Error("定时任务发生 panic", "task", name, "panic", r)
			}
		}()
		fn()
	}

	if _, err := s.cron.AddFunc(spec, wrapped); err != nil {
		return fmt.Errorf("scheduler: 注册任务 %q 失败: %w", name, err)
	}
	s.log.Info("已注册定时任务", "task", name, "schedule", spec)
	return nil
}

// Start 开始调度，非阻塞。
func (s *Scheduler) Start() { s.cron.Start() }

// Stop 停止调度并等待运行中的任务结束。
func (s *Scheduler) Stop() {
	ctx := s.cron.Stop()
	<-ctx.Done()
}
