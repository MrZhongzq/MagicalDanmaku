package rules

import (
	"context"
	"log/slog"
	"math/rand"
	"sync"
	"time"

	"github.com/MrZhongzq/MagicalDanmaku/server/internal/event"
)

// EngineOptions 配置规则引擎。
type EngineOptions struct {
	Label          string // 用于日志，形如 "小号@1706666491"
	RoomID         string // 用于定时触发的 Vars
	Rules          []Rule
	Bot            BotAPI
	Storage        Storage // 可为 nil，此时使用内存存储
	CooldownGroups map[string]time.Duration
	ScriptTimeout  time.Duration
	Logger         *slog.Logger
}

// Engine 是单个「账号-直播间」绑定的规则处理流水线。
//
//	事件 → 按单事件求值条件 → 命中的规则各自合并（或直通）
//	     → 冷却检查 → 动作执行
//
// 条件在合并之前按单个事件求值。若先合并再判断，「只欢迎舰长」这类
// 规则会因为合并后的 Vars 只保留首个用户的等级而误伤非舰长。代价是
// 条件中无法引用 count/users 这类聚合属性——这是有意的取舍。
//
// 账号级的发送限流不在这里，由 BotAPI 的实现负责（见 account.Binding）：
// 同一账号的多个绑定各有一个 Engine，但必须共享同一个限流器。
type Engine struct {
	label    string
	roomID   string
	matcher  *Matcher
	executor *Executor
	cooldown *Cooldown
	log      *slog.Logger

	// 每条配置了合并的规则各有一个 Aggregator
	aggregators map[string]*Aggregator
	byName      map[string]Rule

	// 串行化处理，避免规则之间产生竞态
	mu     sync.Mutex
	closed bool
}

// NewEngine 创建引擎。规则在此完成校验，非法配置直接报错而非带病运行。
func NewEngine(opts EngineOptions) (*Engine, error) {
	if opts.Logger == nil {
		opts.Logger = slog.Default()
	}
	if opts.Storage == nil {
		opts.Storage = NewMemStorage()
	}
	if opts.Label == "" {
		opts.Label = opts.RoomID
	}

	for _, r := range opts.Rules {
		if err := r.Validate(); err != nil {
			return nil, err
		}
	}

	sandbox := NewSandbox(SandboxOptions{
		Timeout: opts.ScriptTimeout,
		Bot:     opts.Bot,
		Storage: opts.Storage,
		Logger:  opts.Logger,
	})

	cd := NewCooldown(time.Now)
	for g, d := range opts.CooldownGroups {
		cd.SetGroupInterval(g, d)
	}

	e := &Engine{
		label:       opts.Label,
		roomID:      opts.RoomID,
		cooldown:    cd,
		log:         opts.Logger,
		aggregators: make(map[string]*Aggregator),
		byName:      make(map[string]Rule, len(opts.Rules)),
	}

	e.matcher = NewMatcher(opts.Rules, NewEvaluator(sandbox), opts.Logger)
	e.executor = NewExecutor(ExecutorOptions{
		Bot:      opts.Bot,
		Renderer: NewRenderer(rand.New(rand.NewSource(time.Now().UnixNano()))),
		Script:   sandbox,
		Logger:   opts.Logger,
	})

	// 为配置了合并的规则各建一个 Aggregator。
	// 窗口到期后走冷却检查与动作执行。
	for _, r := range opts.Rules {
		e.byName[r.Name] = r
		if r.Aggregate == nil {
			continue
		}
		rule := r // 捕获副本
		e.aggregators[r.Name] = NewAggregator(*r.Aggregate, func(tr Trigger) {
			e.fire(rule, tr)
		})
	}
	return e, nil
}

// Handle 处理一个事件。
//
// 同一绑定内串行处理：规则之间共享冷却状态与存储，串行可避免竞态，
// 也让规则的触发顺序可预测。
func (e *Engine) Handle(ev event.Event) {
	e.mu.Lock()
	defer e.mu.Unlock()

	if e.closed {
		return
	}

	// 条件按单个事件求值
	tr := PassthroughTrigger(ev)
	matched := e.matcher.Match(tr)

	for _, r := range matched {
		if agg, ok := e.aggregators[r.Name]; ok {
			agg.Add(ev) // 进入窗口，到期后再触发
			continue
		}
		e.fireLocked(r, tr)
	}
}

// FireScheduled 触发一条定时规则。
func (e *Engine) FireScheduled(name string) {
	e.mu.Lock()
	defer e.mu.Unlock()

	if e.closed {
		return
	}
	r, ok := e.byName[name]
	if !ok || !r.Enabled || r.Schedule == "" {
		return
	}

	tr := Trigger{
		Type:   TypeScheduled,
		Events: nil,
		Vars: map[string]any{
			"type":      string(TypeScheduled),
			"roomId":    e.roomID,
			"timestamp": time.Now().Unix(),
			"count":     1,
			"users":     []string{},
		},
	}
	e.fireLocked(r, tr)
}

// ScheduledRules 返回全部启用的定时规则。
func (e *Engine) ScheduledRules() []Rule {
	return e.matcher.ScheduledRules()
}

// Label 返回本引擎所属绑定的标识，用于日志。
func (e *Engine) Label() string { return e.label }

// fire 是 Aggregator 窗口到期时的回调入口。
//
// 这里刻意不检查 closed：Aggregator 只会在自己存活期间或自己的
// Close() 过程中回调，而 Engine.Close() 正是要让这些未决事件结算完毕。
// 若在此拦截，Close 时缓冲区里的欢迎语就会被静默丢弃。
func (e *Engine) fire(r Rule, tr Trigger) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.fireLocked(r, tr)
}

// fireLocked 执行冷却检查与动作。调用者需持有锁。
func (e *Engine) fireLocked(r Rule, tr Trigger) {
	if !e.cooldown.Allow(r) {
		return
	}
	if err := e.executor.Execute(context.Background(), r, tr); err != nil {
		// 单条规则出错不影响其他规则
		e.log.Warn("规则执行出错", "binding", e.label, "rule", r.Name, "err", err)
	}
}

// Close 停止接收新事件，并结算全部未决的合并窗口。
//
// closed 只拦截新事件入口（Handle / FireScheduled），不拦截 Aggregator
// 的回调——后者正是 Close 要触发的收尾动作。Aggregator.Close() 返回后
// 不会再有回调，因此无需额外的关闭标志。
func (e *Engine) Close() {
	e.mu.Lock()
	if e.closed {
		e.mu.Unlock()
		return
	}
	e.closed = true // 不再接收新事件
	aggs := make([]*Aggregator, 0, len(e.aggregators))
	for _, a := range e.aggregators {
		aggs = append(aggs, a)
	}
	e.mu.Unlock()

	// 必须在释放锁之后调用：Aggregator.Close() 会同步触发 fire()，
	// 而 fire() 需要获取同一把锁。
	for _, a := range aggs {
		a.Close()
	}
}
