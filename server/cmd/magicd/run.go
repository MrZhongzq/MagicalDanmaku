package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"github.com/MrZhongzq/MagicalDanmaku/server/internal/account"
	"github.com/MrZhongzq/MagicalDanmaku/server/internal/connector"
	"github.com/MrZhongzq/MagicalDanmaku/server/internal/connector/bilibili"
	"github.com/MrZhongzq/MagicalDanmaku/server/internal/connector/bilibili/api"
	"github.com/MrZhongzq/MagicalDanmaku/server/internal/connector/bilibili/auth"
	"github.com/MrZhongzq/MagicalDanmaku/server/internal/event"
	"github.com/MrZhongzq/MagicalDanmaku/server/internal/httpapi"
	"github.com/MrZhongzq/MagicalDanmaku/server/internal/logging"
	"github.com/MrZhongzq/MagicalDanmaku/server/internal/ratelimit"
	"github.com/MrZhongzq/MagicalDanmaku/server/internal/rules"
	"github.com/MrZhongzq/MagicalDanmaku/server/internal/scheduler"
	"github.com/MrZhongzq/MagicalDanmaku/server/internal/store"
)

// defaultRateLimit 是未配置时的账号发送间隔。
const defaultRateLimit = 1500 * time.Millisecond

// httpAddr 读 HTTP 监听地址。
//
// 默认只监听本机：不因为用户忘了配防火墙就把管理界面暴露到公网。
// Docker 部署需显式设为 0.0.0.0:8080。设为 "off" 或空串则完全不起 HTTP 服务。
func httpAddr() string {
	v, ok := os.LookupEnv("MAGICD_HTTP_ADDR")
	if !ok {
		return "127.0.0.1:8080"
	}
	return v
}

// danmakuSender 是 roomBot 依赖的绑定能力，抽成接口便于测试。
type danmakuSender interface {
	SendDanmaku(ctx context.Context, text string) error
	Block(ctx context.Context, uid string, hours int) error
}

// roomBot 把「账号-直播间」绑定适配成 rules.BotAPI。
//
// BotAPI 的方法不带 ctx —— 它要被 goja 从 JS 里同步调用，签名必须简单。
// 因此把 ctx 存进结构体。这在 Go 里通常是反模式，但 roomBot 的生命周期
// 严格等于一次 run，不会泄漏。
//
// ctx 用 atomic.Pointer 而不是普通字段：关停时 runRun 会把它换成一个
// 有超时预算、脱离取消链的 ctx（见 closeAll），好让 engine.Close() 结算
// 未决合并窗口时真的能发出去，而不是立刻拿到 context.Canceled。这次
// 替换发生时，早返回路径上可能还有其他绑定的事件处理 goroutine在
// 并发调用 SendDanmaku/Block（它们还没被 wg.Wait() 等到），所以这里
// 需要的是原子替换，不是一次性赋值——但也不是互斥锁：正常运行期
// 只是多一次原子读，行为不变。
type roomBot struct {
	binding danmakuSender
	ctx     atomic.Pointer[context.Context]
}

var _ rules.BotAPI = (*roomBot)(nil)

// newRoomBot 创建一个绑定到给定 ctx 的 roomBot。
func newRoomBot(binding danmakuSender, ctx context.Context) *roomBot {
	b := &roomBot{binding: binding}
	b.ctx.Store(&ctx)
	return b
}

// setCtx 原子地替换发送用的 ctx。供关停时切到有预算的 ctx。
func (b *roomBot) setCtx(ctx context.Context) {
	b.ctx.Store(&ctx)
}

func (b *roomBot) SendDanmaku(text string) error {
	return b.binding.SendDanmaku(*b.ctx.Load(), text)
}

func (b *roomBot) Block(uid string, hours int) error {
	return b.binding.Block(*b.ctx.Load(), uid, hours)
}

// accountRuntime 是一个账号的运行时资源。
//
// 同一账号连接多个直播间时共用一份：共享限流器（风控按账号算）
// 与一个 HTTP 客户端（wbi 签名只需刷新一次）。
type accountRuntime struct {
	acc *account.Account
	api *api.Client
}

// roomRuntime 是一个绑定的运行期能力，也是热重载的替换单元。
//
// 可替换的只有 engine 一个字段。连接（client）与它的两个 goroutine、
// 限流器、业务日志 Sink 都在重载中原样留着——用户改的只是规则。
type roomRuntime struct {
	engine atomic.Pointer[rules.Engine] // 唯一会被热替换的东西

	binding   *account.Binding
	client    *bilibili.Client
	bot       *roomBot
	sink      *logging.Sink
	storage   rules.Storage
	label     string
	roomID    string
	bindingID int64
	st        *store.Store
	sched     *scheduler.Scheduler
	log       *slog.Logger
}

var _ httpapi.BindingRuntime = (*roomRuntime)(nil)

// Engine 取当前引擎。事件扇出的 goroutine 每条事件都要调一次，
// 不能把返回值缓存起来——那就等于又把引擎捕获死了。
func (rt *roomRuntime) Engine() *rules.Engine { return rt.engine.Load() }

func (rt *roomRuntime) SendDanmaku(ctx context.Context, text string) error {
	err := rt.binding.SendDanmaku(ctx, text)
	rt.recordManual(rules.Action{Type: rules.ActionDanmaku, Template: []string{text}}, nil, err)
	return err
}

func (rt *roomRuntime) Block(ctx context.Context, uid string, hours int) error {
	err := rt.binding.Block(ctx, uid, hours)
	rt.recordManual(rules.Action{Type: rules.ActionBlock, Hours: hours},
		map[string]any{"user": map[string]any{"uid": uid}}, err)
	return err
}

func (rt *roomRuntime) Unblock(ctx context.Context, uid string) error {
	err := rt.binding.Unblock(ctx, uid)
	rt.recordManual(rules.Action{Type: rules.ActionBlock, Hours: 0},
		map[string]any{"user": map[string]any{"uid": uid}}, err)
	return err
}

// State 报告连接状态，供 /api/meta/runtime 展示——连接（client）与它的
// 重连逻辑不受热重载影响，这里只是如实转发。
func (rt *roomRuntime) State() connector.State { return rt.client.State() }

// recordManual 把手动操作记进业务日志，无论成败。
//
// 「把操作失败写日志」是明确要求：在不是房管的直播间点禁言，B 站会
// 回退操作失败，而 WebUI 的日志页读的就是 activity_logs——只写系统
// 日志的话操作者在界面上什么都看不见。
//
// 手动操作与规则驱动的动作走同一个 Sink，因此落在**同一条时间线**上，
// 这正是「两类日志」设计里业务日志的意义。
func (rt *roomRuntime) recordManual(a rules.Action, vars map[string]any, err error) {
	if vars == nil {
		vars = map[string]any{}
	}
	rt.sink.RecordAction("手动操作", a, rules.Trigger{
		Type: event.TypeManual,
		Vars: vars,
	}, err)
}

// Reload 用数据库里当前的配置重建这个绑定的规则引擎。
//
// 顺序是刻意的：**先构造、再切换、最后关旧的**。
//
//   - 构造失败就直接返回，旧引擎一点没动。保存了一份非法规则不该把
//     机器人搞停
//   - 切换在关旧引擎之前。反过来的话，两步之间到达的事件会打在已经
//     关闭的引擎上被丢掉
//   - 旧引擎关掉是为了结算未决的合并窗口，那里面攒着待发的欢迎语。
//     这时主 ctx 还活着（roomBot 用的就是它），补发能真的发出去
func (rt *roomRuntime) Reload(ctx context.Context) error {
	cfgs, err := rt.st.LoadRunConfig(ctx)
	if err != nil {
		return fmt.Errorf("读取配置失败: %w", err)
	}

	var cfg *store.RunConfig
	for i := range cfgs {
		if cfgs[i].BindingID == rt.bindingID {
			cfg = &cfgs[i]
			break
		}
	}
	if cfg == nil {
		// 绑定被停用或删掉了。这不是重载能解决的，让调用者知道
		return fmt.Errorf("绑定 %s 已不在启用列表里，请重启或重新启用", rt.label)
	}

	// 先构造。失败就到此为止，旧引擎继续跑
	next, err := rules.NewEngine(rules.EngineOptions{
		Label:          rt.label,
		RoomID:         rt.roomID,
		Rules:          cfg.Rules,
		Bot:            rt.bot,
		Storage:        rt.storage,
		Activity:       rt.sink,
		CooldownGroups: cfg.CooldownGroups,
		Logger:         rt.log,
	})
	if err != nil {
		return fmt.Errorf("规则非法: %w", err)
	}

	// 再切换。这一刻起新事件都进新引擎
	prev := rt.engine.Swap(next)

	// 定时规则整组换掉。RemoveByPrefix 就是为这一步加的——不移除的话，
	// 改了 cron 表达式的规则会有新旧两个条目**都指向新引擎**，
	// 同一条规则一天触发两次
	rt.sched.RemoveByPrefix(rt.label + "/")
	for _, r := range next.ScheduledRules() {
		name, eng := r.Name, next
		if err := rt.sched.Add(r.Schedule, rt.label+"/"+name, func() {
			eng.FireScheduled(name)
		}); err != nil {
			// 引擎已经换上去了，定时规则没挂上不该让整次重载显示为失败——
			// 弹幕规则是生效的。记日志，让用户从日志里看到这一条
			rt.log.Error("重载后注册定时规则失败",
				"binding", rt.label, "rule", name, "err", err)
		}
	}

	// 最后关旧的，结算它未决的合并窗口
	if prev != nil {
		prev.Close()
	}

	rt.log.Info("已热重载", "binding", rt.label, "rules", len(cfg.Rules))
	return nil
}

// defaultRetentionDays 是业务日志的默认保留天数。
const defaultRetentionDays = 30

// runRun 从数据库加载配置并启动机器人。
func runRun(args []string) error {
	fs := flag.NewFlagSet("run", flag.ExitOnError)
	dsn := addDBFlag(fs)
	if err := fs.Parse(args); err != nil {
		return err
	}

	log := slog.Default()
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	st, err := openStoreChecked(ctx, *dsn)
	if err != nil {
		return err
	}
	defer st.Close()

	cfgs, err := st.LoadRunConfig(ctx)
	if err != nil {
		return err
	}
	if len(cfgs) == 0 {
		return fmt.Errorf("数据库里没有任何启用的绑定。\n" +
			"先 magicd login --save <账号名> --owner <用户名>，再 magicd binding add <账号名> <房间号>；\n" +
			"或者用 magicd import -c config.yaml --owner <用户名> 导入现成的配置")
	}

	// 业务日志：一个写入器，每个绑定分一个带归属 ID 的 Sink
	activity := logging.NewActivityWriter(logging.ActivityWriterOptions{
		Flush:  st.InsertActivity,
		Logger: log,
	})

	accounts, err := buildAccounts(ctx, cfgs, log)
	if err != nil {
		activity.Close()
		return err
	}

	// HTTP 管理界面与机器人同进程：实时事件流直接复用机器人已持有的
	// 事件通道。MAGICD_HTTP_ADDR 设为空串或 off 时完全不起 HTTP 服务，
	// 退化成纯机器人。
	var api *httpapi.Server
	apiRuntimes := make(map[int64]httpapi.BindingRuntime, len(cfgs))
	if addr := httpAddr(); addr != "" && addr != "off" {
		api = httpapi.New(st, httpapi.Options{
			Addr:         addr,
			SecureCookie: os.Getenv("MAGICD_HTTP_SECURE_COOKIE") == "1",
			Logger:       log,
		})
	}

	sched := scheduler.New(log)
	var wg sync.WaitGroup
	var roomRTs []*roomRuntime // 供关停时结算——必须取热重载后的当前引擎，见下方 defer
	var bots []*roomBot

	// 关停时给未决的合并窗口一个有限的发送预算。
	//
	// engine.Close() 会结算未决窗口并真的去发弹幕，但此时主 ctx 已被
	// Ctrl+C 取消，限流器会立刻返回 context.Canceled，那批攒着的欢迎语
	// 一条也发不出去（只会留下一堆 error 日志行）。用一个脱离取消链、
	// 但有超时上限的 ctx，让它们有机会发出去而又不会拖死退出。
	const shutdownSendBudget = 5 * time.Second

	// 清理放在 defer 里，而不是只写在正常关停路径的末尾。
	//
	// 装配循环里有多个早返回点（房间信息获取失败、规则非法、定时任务注册失败），
	// 而此时前面的绑定可能已经建好引擎并跑起来了。只在末尾清理的话，
	// 那些引擎的 Close() 永远不会被调用——后果不是日志被丢弃，而是
	// 那批日志行根本不会产生：Close() 才是结算未决合并窗口的地方，
	// 不调用它，攒着的欢迎语既不会发出去，也不会有对应的动作日志。
	//
	// 这里用闭包而不是直接 defer closeAll(engines, bots, ..., activity)：
	// defer 的参数在语句执行时就求值，若直接传值会捕获此刻仍是 nil 的
	// engines/bots，后续循环里的 append 不会反映到已经求过值的参数上。
	// 闭包捕获的是变量本身，调用时才读取，因此能看到循环结束时的
	// 最终切片。
	//
	// engines 列表在这里现取而不是在装配循环里固定下来：一个绑定可能
	// 在运行期间被热重载过，此时 roomRuntime.engine 里存的已经是新引擎，
	// 关停时要结算的是这个新引擎，而不是启动时那个早已被 Reload 关掉的旧引擎。
	defer func() {
		shutdownCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), shutdownSendBudget)
		defer cancel()
		engines := make([]*rules.Engine, len(roomRTs))
		for i, rt := range roomRTs {
			engines[i] = rt.Engine()
		}
		closeAll(engines, bots, shutdownCtx, activity)
	}()

	for _, c := range cfgs {
		acctRT := accounts[c.AccountName]

		// 解析真实房间号：配置里可能填的是短号
		info, err := acctRT.api.RoomInfo(ctx, c.RoomID)
		if err != nil {
			return fmt.Errorf("账号 %q 获取直播间 %s 信息失败: %w", c.AccountName, c.RoomID, err)
		}

		// 限流统一由 account.Binding 负责，这里传空限流器，
		// 否则与 Binding 的等待叠加会让实际间隔翻倍。
		actions := bilibili.NewActions(acctRT.api, ratelimit.NewInterval(0))
		if c.MaxLength > 0 {
			actions.SetMaxLength(c.MaxLength)
		}
		binding := &account.Binding{
			Account: acctRT.acc,
			RoomID:  info.RoomID,
			Actions: actions,
		}

		bot := newRoomBot(binding, ctx)
		sink := activity.Sink(c.AccountID, c.BindingID, info.RoomID)
		storage := st.BindingStorage(c.BindingID)
		engine, err := rules.NewEngine(rules.EngineOptions{
			Label:          binding.Label(),
			RoomID:         info.RoomID,
			Rules:          c.Rules,
			Bot:            bot,
			Storage:        storage,
			Activity:       sink,
			CooldownGroups: c.CooldownGroups,
			Logger:         log,
		})
		if err != nil {
			return fmt.Errorf("%s 的规则非法: %w", binding.Label(), err)
		}
		bots = append(bots, bot)

		client := bilibili.NewClient(info.RoomID, acctRT.api, bilibili.WithLogger(log))

		room := &roomRuntime{
			binding:   binding,
			client:    client,
			bot:       bot,
			sink:      sink,
			storage:   storage,
			label:     binding.Label(),
			roomID:    info.RoomID,
			bindingID: c.BindingID,
			st:        st,
			sched:     sched,
			log:       log,
		}
		room.engine.Store(engine)
		roomRTs = append(roomRTs, room)
		if api != nil {
			apiRuntimes[c.BindingID] = room
		}

		// 注册该绑定的定时规则
		for _, r := range engine.ScheduledRules() {
			name, eng := r.Name, engine
			if err := sched.Add(r.Schedule, binding.Label()+"/"+name, func() {
				eng.FireScheduled(name)
			}); err != nil {
				return err
			}
		}

		status := "未开播"
		if info.IsLiving() {
			status = "直播中"
		}
		enabled := 0
		for _, r := range c.Rules {
			if r.Enabled {
				enabled++
			}
		}
		log.Info("已配置绑定",
			"binding", binding.Label(),
			"title", info.Title,
			"status", status,
			"rules", len(c.Rules),
			"enabled", enabled)

		wg.Add(2)
		go func(rt *roomRuntime, c *bilibili.Client, bindingID int64) {
			defer wg.Done()
			for ev := range c.Events() {
				// rt.Engine() 每次都要重新取：热重载会把 rt.engine 换成
				// 新引擎，若在循环外缓存一次就等于把旧引擎闭包捕获死了，
				// 重载之后事件会继续打在已经 Close 掉的旧引擎上。
				rt.Engine().Handle(ev)
				if api != nil {
					// 扇出给网页。必须在 Handle 之后：机器人的响应也是通过
					// 弹幕事件回流的，顺序颠倒会让因果看起来是反的。
					api.Hub().Publish(bindingID, ev)
				}
			}
		}(room, client, c.BindingID)

		go func(label string, c *bilibili.Client) {
			defer wg.Done()
			// 单个绑定的连接出错不影响其他绑定，也不做账号切换
			if err := c.Run(ctx); err != nil && !errors.Is(err, context.Canceled) {
				log.Error("绑定连接退出", "binding", label, "err", err)
			}
		}(binding.Label(), client)
	}

	// 业务日志的定期清理
	if days := retentionDays(); days > 0 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			purgeLoop(ctx, st, days, log)
		}()
	}

	if api != nil {
		api.SetRuntime(apiRuntimes)
		if h, err := api.CurrentConfigHash(ctx); err == nil {
			api.SetConfigHash(h)
		} else {
			log.Warn("计算配置版本失败，界面将无法提示「有未重载的改动」", "err", err)
		}

		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := api.ListenAndServe(ctx); err != nil {
				log.Error("HTTP 服务异常退出", "err", err)
			}
		}()
	}

	sched.Start()
	log.Info("机器人已启动", "绑定数", len(cfgs), "账号数", len(accounts))
	fmt.Println("按 Ctrl+C 退出")

	<-ctx.Done()
	log.Info("正在退出...")

	sched.Stop()
	wg.Wait()
	log.Info("已退出")
	return nil
}

// buildAccounts 载入全部账号并初始化各自的运行时资源。
//
// 同一账号连接多个直播间时只建一份，保证限流器被真正共享。
func buildAccounts(ctx context.Context, cfgs []store.RunConfig, log *slog.Logger) (map[string]*accountRuntime, error) {
	out := make(map[string]*accountRuntime)

	for _, c := range cfgs {
		if _, ok := out[c.AccountName]; ok {
			continue
		}

		sess, err := auth.ParseSession(c.Cookie)
		if err != nil {
			return nil, fmt.Errorf("账号 %q 的 Cookie 无效，请重新扫码登录（magicd login --save %s）: %w",
				c.AccountName, c.AccountName, err)
		}

		interval := c.RateLimit
		if interval <= 0 {
			interval = defaultRateLimit
		}

		apiClient := api.New(sess)
		// wbi 签名每个账号刷新一次即可，其全部直播间共用
		if err := apiClient.RefreshNav(ctx); err != nil {
			return nil, fmt.Errorf("账号 %q 初始化签名失败: %w", c.AccountName, err)
		}

		out[c.AccountName] = &accountRuntime{
			acc: account.New(c.AccountName, sess, interval),
			api: apiClient,
		}
		log.Info("已载入账号", "name", c.AccountName, "uid", sess.UID, "发送间隔", interval)
	}
	return out, nil
}

// closeAll 按「切换发送 ctx → 引擎 → 业务日志」的顺序关闭资源。
//
// 先把每个 roomBot 的发送 ctx 换成 shutdownCtx：engine.Close() 结算未决
// 合并窗口时会调用 roomBot.SendDanmaku，若还在用已取消的主 ctx，攒着的
// 欢迎语会立刻拿到 context.Canceled，一条都发不出去。
//
// engine.Close() 会结算未决的合并窗口（比如攒着的欢迎语），这一步会
// 通过 Activity sink 产生业务日志；activity.Close() 才是把这些日志真正
// 冲刷出去的地方。顺序反了不会报错，只会让那批日志静默消失。
func closeAll(engines []*rules.Engine, bots []*roomBot, shutdownCtx context.Context, activity *logging.ActivityWriter) {
	for _, b := range bots {
		b.setCtx(shutdownCtx)
	}
	for _, e := range engines {
		e.Close()
	}
	activity.Close()
}

// retentionDays 读业务日志的保留天数。
//
// 环境变量写错就退回默认值：一个日志保留期的笔误，不该让机器人起不来。
func retentionDays() int {
	s := os.Getenv("MAGICD_LOG_RETENTION_DAYS")
	if s == "" {
		return defaultRetentionDays
	}
	n, err := strconv.Atoi(s)
	if err != nil || n < 0 {
		return defaultRetentionDays
	}
	return n
}

// purgeLoop 每小时清理一次超期的业务日志。
func purgeLoop(ctx context.Context, st *store.Store, days int, log *slog.Logger) {
	ticker := time.NewTicker(time.Hour)
	defer ticker.Stop()

	purge := func() {
		cutoff := time.Now().AddDate(0, 0, -days)
		n, err := st.PurgeActivityBefore(ctx, cutoff)
		if err != nil {
			log.Error("清理业务日志失败", "err", err)
			return
		}
		if n > 0 {
			log.Info("已清理超期业务日志", "条数", n, "保留天数", days)
		}
	}

	purge() // 启动时先清一次，不必等满一小时
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			purge()
		}
	}
}
