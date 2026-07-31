package rules

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
)

// defaultBlockHours 是未指定时长时的默认禁言小时数。
const defaultBlockHours = 1

// ExecutorOptions 配置动作执行器。
type ExecutorOptions struct {
	Bot               BotAPI
	Renderer          *Renderer
	Script            *Sandbox
	Cooldown          *Cooldown
	DefaultBlockHours int
	Logger            *slog.Logger
}

// Executor 执行规则的动作列表。
type Executor struct {
	bot        BotAPI
	renderer   *Renderer
	script     *Sandbox
	cooldown   *Cooldown
	blockHours int
	log        *slog.Logger
}

// NewExecutor 创建动作执行器。
func NewExecutor(opts ExecutorOptions) *Executor {
	if opts.DefaultBlockHours <= 0 {
		opts.DefaultBlockHours = defaultBlockHours
	}
	if opts.Logger == nil {
		opts.Logger = slog.Default()
	}
	return &Executor{
		bot:        opts.Bot,
		renderer:   opts.Renderer,
		script:     opts.Script,
		cooldown:   opts.Cooldown,
		blockHours: opts.DefaultBlockHours,
		log:        opts.Logger,
	}
}

// Execute 按序执行规则的全部动作。
//
// 单个动作失败只记录日志并继续执行后续动作，最后返回聚合错误——
// 一条规则里前面的动作失败，不该让后面的动作也做不成。
func (e *Executor) Execute(ctx context.Context, r Rule, tr Trigger) error {
	var errs []error

	for i, a := range r.Do {
		if err := e.runAction(ctx, a, tr); err != nil {
			e.log.Warn("动作执行失败",
				"rule", r.Name, "action", i+1, "type", a.Type, "err", err)
			errs = append(errs, fmt.Errorf("第 %d 个动作(%s): %w", i+1, a.Type, err))
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("规则 %q 有 %d 个动作失败: %w", r.Name, len(errs), errors.Join(errs...))
	}
	return nil
}

// runAction 执行单个动作。
func (e *Executor) runAction(ctx context.Context, a Action, tr Trigger) error {
	switch a.Type {
	case ActionDanmaku:
		return e.sendDanmaku(ctx, a, tr)
	case ActionBlock:
		return e.blockUsers(ctx, a, tr)
	case ActionScript:
		if e.script == nil {
			return errors.New("rules: 未配置脚本沙箱")
		}
		return e.script.RunAction(a.Script, tr.Vars)
	case ActionLog:
		e.log.Info("规则触发", "type", tr.Type, "count", tr.Vars["count"], "vars", tr.Vars)
		return nil
	default:
		return fmt.Errorf("rules: 未知的动作类型 %q", a.Type)
	}
}

// sendDanmaku 渲染模板并发送弹幕。
func (e *Executor) sendDanmaku(ctx context.Context, a Action, tr Trigger) error {
	if e.bot == nil {
		return errors.New("rules: 未配置机器人接口")
	}

	text, err := e.renderer.Render(a.Template, tr.Vars)
	if err != nil {
		return err
	}
	// 渲染结果为空时静默跳过：空弹幕发不出去，报错反而制造噪声。
	if strings.TrimSpace(text) == "" {
		return nil
	}

	// 全局限流在真正发送前等待
	if e.cooldown != nil {
		if err := e.cooldown.WaitGlobal(ctx); err != nil {
			return err
		}
	}
	return e.bot.SendDanmaku(text)
}

// blockUsers 禁言 Trigger 涉及的全部用户。
//
// 合并后的 Trigger 可能包含多个用户，逐个禁言。
func (e *Executor) blockUsers(ctx context.Context, a Action, tr Trigger) error {
	if e.bot == nil {
		return errors.New("rules: 未配置机器人接口")
	}

	hours := a.Hours
	if hours <= 0 {
		hours = e.blockHours
	}

	uids := uidsOf(tr)
	if len(uids) == 0 {
		return errors.New("rules: 事件中没有可禁言的用户")
	}

	var errs []error
	for _, uid := range uids {
		if e.cooldown != nil {
			if err := e.cooldown.WaitGlobal(ctx); err != nil {
				return err
			}
		}
		if err := e.bot.Block(uid, hours); err != nil {
			errs = append(errs, fmt.Errorf("禁言 %s 失败: %w", uid, err))
		}
	}
	return errors.Join(errs...)
}

// uidsOf 提取 Trigger 涉及的全部用户 UID，去重且保持顺序。
func uidsOf(tr Trigger) []string {
	seen := make(map[string]bool, len(tr.Events))
	out := make([]string, 0, len(tr.Events))

	for _, ev := range tr.Events {
		if uid := uidOf(ev); uid != "" && !seen[uid] {
			seen[uid] = true
			out = append(out, uid)
		}
	}
	// 事件列表为空时回退到 Vars（定时任务等场景）
	if len(out) == 0 {
		if uid, ok := LookupPath(tr.Vars, "user.uid"); ok {
			if s := toString(uid); s != "" {
				out = append(out, s)
			}
		}
	}
	return out
}
