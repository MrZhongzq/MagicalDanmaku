package rules

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"sync"
)

// defaultBlockHours 是未指定时长时的默认禁言小时数。
const defaultBlockHours = 1

// ExecutorOptions 配置动作执行器。
type ExecutorOptions struct {
	Bot               BotAPI
	Renderer          *Renderer
	Script            *Sandbox
	DefaultBlockHours int
	Activity          ActivitySink
	Logger            *slog.Logger
}

// Executor 执行规则的动作列表。
type Executor struct {
	bot        BotAPI
	renderer   *Renderer
	script     *Sandbox
	blockHours int
	activity   ActivitySink
	log        *slog.Logger

	// pickCursor 记住每个 danmaku 动作（PickSequential 模式）轮询到第几条。
	//
	// 键是「规则名#动作下标」——一条规则可以有多个 danmaku 动作，
	// 各自的模板列表独立，共用一个游标会让两个动作交替推进它，
	// 结果比随机还糟（详见 executor_test.go 的
	// TestSequentialCursorIsPerAction）。
	//
	// 热重载会重置它：Executor 属于 Engine，换引擎就是新的一份。
	// 这是可接受的——轮询是为了「文案别老重复」，重载后从头开始
	// 不影响这个目的，把游标持久化进 kv_store 要为一个纯展示效果
	// 引入写库开销，不值得。
	//
	// 加锁理由：目前 Executor 只会被 Engine.Handle 与 FireScheduled
	// 调用，两者共用同一把 Engine.mu，因此实际是串行调用的、本可以
	// 不加锁。但依赖调用方持锁是隐式契约——万一将来 Engine 改成
	// 并行分发（比如按房间分片处理），这里会静默出现数据竞争，
	// 只有 -race 才抓得到。所以仍然显式加锁，把正确性建在
	// Executor 自身而不是调用方的承诺上。
	cursorMu sync.Mutex
	cursor   map[string]int
}

// NewExecutor 创建动作执行器。
func NewExecutor(opts ExecutorOptions) *Executor {
	if opts.DefaultBlockHours <= 0 {
		opts.DefaultBlockHours = defaultBlockHours
	}
	if opts.Logger == nil {
		opts.Logger = slog.Default()
	}
	if opts.Activity == nil {
		opts.Activity = nopSink{}
	}
	return &Executor{
		bot:        opts.Bot,
		renderer:   opts.Renderer,
		script:     opts.Script,
		blockHours: opts.DefaultBlockHours,
		activity:   opts.Activity,
		log:        opts.Logger,
		cursor:     make(map[string]int),
	}
}

// Execute 按序执行规则的全部动作。
//
// 单个动作失败只记录日志并继续执行后续动作，最后返回聚合错误——
// 一条规则里前面的动作失败，不该让后面的动作也做不成。
func (e *Executor) Execute(ctx context.Context, r Rule, tr Trigger) error {
	var errs []error

	for i, a := range r.Do {
		err := e.runAction(ctx, r.Name, i, a, tr)
		// 成功与失败都上报：「为什么没发出去」正是要查的
		e.activity.RecordAction(r.Name, a, tr, err)
		if err != nil {
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

// runAction 执行单个动作。ruleName 仅用于日志；actionIdx 是该动作在
// r.Do 中的下标，用于给顺序轮询模式定位专属游标。
func (e *Executor) runAction(ctx context.Context, ruleName string, actionIdx int, a Action, tr Trigger) error {
	switch a.Type {
	case ActionDanmaku:
		return e.sendDanmaku(ctx, ruleName, actionIdx, a, tr)
	case ActionBlock:
		return e.blockUsers(ctx, ruleName, a, tr)
	case ActionScript:
		if e.script == nil {
			return errors.New("rules: 未配置脚本沙箱")
		}
		return e.script.RunAction(a.Script, tr.Vars)
	case ActionLog:
		e.log.Info("规则触发", "rule", ruleName, "type", tr.Type,
			"count", tr.Vars["count"], "vars", tr.Vars)
		return nil
	default:
		return fmt.Errorf("rules: 未知的动作类型 %q", a.Type)
	}
}

// sendDanmaku 渲染模板并发送弹幕。
func (e *Executor) sendDanmaku(ctx context.Context, ruleName string, actionIdx int, a Action, tr Trigger) error {
	if e.bot == nil {
		return errors.New("rules: 未配置机器人接口")
	}

	var text string
	var err error
	if a.Pick == PickSequential {
		idx := e.nextCursor(ruleName, actionIdx)
		text, err = e.renderer.RenderAt(a.Template, idx, tr.Vars)
	} else {
		text, err = e.renderer.Render(a.Template, tr.Vars)
	}
	if err != nil {
		return err
	}
	// 渲染结果为空时静默跳过：空弹幕发不出去，报错反而制造噪声。
	if strings.TrimSpace(text) == "" {
		return nil
	}

	// 账号级发送限流由 BotAPI 的实现负责（见 account.Binding），
	// 这里不再重复等待，否则实际间隔会翻倍。
	if err := e.bot.SendDanmaku(text); err != nil {
		return err
	}
	// 成功也记日志：运维时若只有出错才有输出，就无从知道机器人在做什么。
	e.log.Info("已发送弹幕", "rule", ruleName, "text", text)
	return nil
}

// nextCursor 取出「规则名#动作下标」对应游标的当前值并推进到下一个。
//
// 返回值直接喂给 Renderer.RenderAt，越界由它取模处理，这里不需要
// 关心 len(templates)——游标只管单调递增，不管模板列表有多长。
func (e *Executor) nextCursor(ruleName string, actionIdx int) int {
	key := fmt.Sprintf("%s#%d", ruleName, actionIdx)

	e.cursorMu.Lock()
	defer e.cursorMu.Unlock()
	idx := e.cursor[key]
	e.cursor[key] = idx + 1
	return idx
}

// blockUsers 禁言 Trigger 涉及的全部用户。
//
// 合并后的 Trigger 可能包含多个用户，逐个禁言。
func (e *Executor) blockUsers(ctx context.Context, ruleName string, a Action, tr Trigger) error {
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
		if err := e.bot.Block(uid, hours); err != nil {
			errs = append(errs, fmt.Errorf("禁言 %s 失败: %w", uid, err))
			continue
		}
		e.log.Info("已禁言用户", "rule", ruleName, "uid", uid, "hours", hours)
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
