// Package scheduler 提供基于 cron 表达式的定时任务调度。
package scheduler

import (
	"fmt"
	"log/slog"
	"strings"
	"sync"

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

	// ids 记住任务名到 cron 条目的对应，好让任务能被移除。
	// robfig/cron 只认 EntryID，不认名字。
	mu  sync.Mutex
	ids map[string]cron.EntryID
}

// New 创建调度器。
func New(log *slog.Logger) *Scheduler {
	if log == nil {
		log = slog.Default()
	}
	return &Scheduler{
		cron: cron.New(cron.WithParser(parser)),
		log:  log,
		ids:  make(map[string]cron.EntryID),
	}
}

// Add 注册一个定时任务。
//
// **同名任务会先被移除再注册。** 热重载时同一条规则可能改了 cron
// 表达式，留着旧条目会让两个条目都指向新引擎，同一条规则在新旧
// 两个时刻各触发一次——多发一条弹幕，而且不报任何错。
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

	s.mu.Lock()
	defer s.mu.Unlock()

	if old, ok := s.ids[name]; ok {
		s.cron.Remove(old)
		delete(s.ids, name)
	}

	id, err := s.cron.AddFunc(spec, wrapped)
	if err != nil {
		return fmt.Errorf("scheduler: 注册任务 %q 失败: %w", name, err)
	}
	s.ids[name] = id
	s.log.Info("已注册定时任务", "task", name, "schedule", spec)
	return nil
}

// RemoveByPrefix 移除名字带此前缀的全部任务，返回移除了几个。
//
// 热重载以绑定为单位重建定时规则，而任务名本来就是
// 「绑定标签/规则名」，前缀天然可用。
//
// cron.Remove 只保证「不再启动新的一轮」——已经在跑的那一轮会跑完。
// 这可以接受：调用方随后会 Close 掉旧引擎，而 Engine.FireScheduled
// 开头就查 closed 并直接返回（rules/engine.go:155），跑完的那一轮
// 不会产生任何动作。
func (s *Scheduler) RemoveByPrefix(prefix string) int {
	s.mu.Lock()
	defer s.mu.Unlock()

	n := 0
	for name, id := range s.ids {
		if strings.HasPrefix(name, prefix) {
			s.cron.Remove(id)
			delete(s.ids, name)
			n++
		}
	}
	if n > 0 {
		s.log.Info("已移除定时任务", "prefix", prefix, "count", n)
	}
	return n
}

// Start 开始调度，非阻塞。
func (s *Scheduler) Start() { s.cron.Start() }

// Stop 停止调度并等待运行中的任务结束。
func (s *Scheduler) Stop() {
	ctx := s.cron.Stop()
	<-ctx.Done()
}
