package rules

import (
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/dop251/goja"
)

// ErrScriptTimeout 表示脚本执行超时被强制中断。
var ErrScriptTimeout = errors.New("rules: 脚本执行超时")

// defaultScriptTimeout 是脚本执行的默认硬超时。
const defaultScriptTimeout = 200 * time.Millisecond

// BotAPI 是注入脚本的机器人能力。
type BotAPI interface {
	SendDanmaku(text string) error
	Block(uid string, hours int) error
}

// Storage 是注入脚本的房间级键值存储。
type Storage interface {
	Get(key string) (string, bool)
	Set(key, value string)
}

// SandboxOptions 配置沙箱。
type SandboxOptions struct {
	Timeout time.Duration // 单次执行硬超时，0 表示用默认值
	Bot     BotAPI        // 可为 nil，此时脚本调用 bot.* 会抛异常
	Storage Storage       // 可为 nil，此时脚本调用 storage.* 会抛异常
	Logger  *slog.Logger
}

// Sandbox 是 goja 脚本沙箱，实现 ScriptRunner。
//
// 安全模型：goja 默认不提供 require、文件系统、process、网络，
// 这是天然白名单——不注入就没有。本沙箱刻意不注入网络访问：
// 在多用户场景下，那等同把服务器变成任意请求代理。
//
// goja.Runtime 非并发安全，故用池管理，每次执行独占一个实例。
type Sandbox struct {
	timeout time.Duration
	bot     BotAPI
	storage Storage
	log     *slog.Logger

	pool sync.Pool
}

var _ ScriptRunner = (*Sandbox)(nil)

// NewSandbox 创建沙箱。
func NewSandbox(opts SandboxOptions) *Sandbox {
	if opts.Timeout <= 0 {
		opts.Timeout = defaultScriptTimeout
	}
	if opts.Logger == nil {
		opts.Logger = slog.Default()
	}

	s := &Sandbox{
		timeout: opts.Timeout,
		bot:     opts.Bot,
		storage: opts.Storage,
		log:     opts.Logger,
	}
	s.pool.New = func() any { return s.newRuntime() }
	return s
}

// newRuntime 创建一个注入好 API 的 Runtime。
func (s *Sandbox) newRuntime() *goja.Runtime {
	vm := goja.New()

	bot := vm.NewObject()
	_ = bot.Set("sendDanmaku", func(text string) {
		if s.bot == nil {
			panic(vm.ToValue("bot 能力未启用"))
		}
		if err := s.bot.SendDanmaku(text); err != nil {
			panic(vm.ToValue("发送弹幕失败: " + err.Error()))
		}
	})
	_ = bot.Set("block", func(uid string, hours int) {
		if s.bot == nil {
			panic(vm.ToValue("bot 能力未启用"))
		}
		if err := s.bot.Block(uid, hours); err != nil {
			panic(vm.ToValue("禁言失败: " + err.Error()))
		}
	})
	_ = vm.Set("bot", bot)

	storage := vm.NewObject()
	_ = storage.Set("get", func(k string) string {
		if s.storage == nil {
			panic(vm.ToValue("storage 能力未启用"))
		}
		v, _ := s.storage.Get(k) // 缺失返回空串，不抛异常
		return v
	})
	_ = storage.Set("set", func(k, v string) {
		if s.storage == nil {
			panic(vm.ToValue("storage 能力未启用"))
		}
		s.storage.Set(k, v)
	})
	_ = vm.Set("storage", storage)

	console := vm.NewObject()
	logFn := func(args ...any) { s.log.Info("脚本日志", "args", args) }
	_ = console.Set("log", logFn)
	_ = console.Set("info", logFn)
	_ = console.Set("warn", func(args ...any) { s.log.Warn("脚本日志", "args", args) })
	_ = console.Set("error", func(args ...any) { s.log.Error("脚本日志", "args", args) })
	_ = vm.Set("console", console)

	return vm
}

// EvalBool 求值一段 JS 表达式，按 JS 真值语义返回布尔结果。
func (s *Sandbox) EvalBool(code string, vars map[string]any) (bool, error) {
	v, err := s.run(code, vars)
	if err != nil {
		return false, err
	}
	return v.ToBoolean(), nil
}

// RunAction 执行一段 JS 语句，忽略返回值。
func (s *Sandbox) RunAction(code string, vars map[string]any) error {
	_, err := s.run(code, vars)
	return err
}

// run 在受控环境中执行脚本。
func (s *Sandbox) run(code string, vars map[string]any) (goja.Value, error) {
	vm := s.pool.Get().(*goja.Runtime)
	defer func() {
		s.cleanup(vm)
		s.pool.Put(vm)
	}()

	if vars == nil {
		vars = map[string]any{}
	}
	if err := vm.Set("event", vars); err != nil {
		return nil, fmt.Errorf("rules: 注入事件数据失败: %w", err)
	}

	// 超时守卫：到期强制中断，防死循环拖垮房间。
	timer := time.AfterFunc(s.timeout, func() {
		vm.Interrupt(ErrScriptTimeout)
	})
	defer timer.Stop()

	v, err := vm.RunString(code)
	// 无论成功与否都要清除中断标志，否则该 Runtime 会永久不可用。
	vm.ClearInterrupt()

	if err != nil {
		var ie *goja.InterruptedError
		if errors.As(err, &ie) {
			return nil, ErrScriptTimeout
		}
		return nil, fmt.Errorf("rules: 脚本执行失败: %w", err)
	}
	return v, nil
}

// cleanup 清除本次执行注入的变量与脚本产生的全局污染。
//
// Runtime 会被复用，必须保证上一次执行的全局变量不泄漏到下一次。
func (s *Sandbox) cleanup(vm *goja.Runtime) {
	// 删除脚本自行创建的全局变量，保留注入的 API。
	protected := map[string]bool{
		"bot": true, "storage": true, "console": true,
		"globalThis": true, "undefined": true, "NaN": true, "Infinity": true,
	}
	global := vm.GlobalObject()
	for _, k := range global.Keys() {
		if !protected[k] {
			// 内置构造器（Object、Array 等）不可删除，Delete 会静默失败，无害。
			_ = global.Delete(k)
		}
	}
}
