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
	"time"

	"github.com/MrZhongzq/MagicalDanmaku/server/internal/account"
	"github.com/MrZhongzq/MagicalDanmaku/server/internal/connector/bilibili"
	"github.com/MrZhongzq/MagicalDanmaku/server/internal/connector/bilibili/api"
	"github.com/MrZhongzq/MagicalDanmaku/server/internal/connector/bilibili/auth"
	"github.com/MrZhongzq/MagicalDanmaku/server/internal/logging"
	"github.com/MrZhongzq/MagicalDanmaku/server/internal/ratelimit"
	"github.com/MrZhongzq/MagicalDanmaku/server/internal/rules"
	"github.com/MrZhongzq/MagicalDanmaku/server/internal/scheduler"
	"github.com/MrZhongzq/MagicalDanmaku/server/internal/store"
)

// defaultRateLimit 是未配置时的账号发送间隔。
const defaultRateLimit = 1500 * time.Millisecond

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
type roomBot struct {
	binding danmakuSender
	ctx     context.Context
}

var _ rules.BotAPI = (*roomBot)(nil)

func (b *roomBot) SendDanmaku(text string) error {
	return b.binding.SendDanmaku(b.ctx, text)
}

func (b *roomBot) Block(uid string, hours int) error {
	return b.binding.Block(b.ctx, uid, hours)
}

// accountRuntime 是一个账号的运行时资源。
//
// 同一账号连接多个直播间时共用一份：共享限流器（风控按账号算）
// 与一个 HTTP 客户端（wbi 签名只需刷新一次）。
type accountRuntime struct {
	acc *account.Account
	api *api.Client
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

	runtimes, err := buildAccounts(ctx, cfgs, log)
	if err != nil {
		activity.Close()
		return err
	}

	sched := scheduler.New(log)
	var wg sync.WaitGroup
	var engines []*rules.Engine

	// 清理放在 defer 里，而不是只写在正常关停路径的末尾。
	//
	// 装配循环里有多个早返回点（房间信息获取失败、规则非法、定时任务注册失败），
	// 而此时前面的绑定可能已经建好引擎并跑起来了。只在末尾清理的话，
	// 那些引擎的 Close() 永远不会被调用——后果不是日志被丢弃，而是
	// 那批日志行根本不会产生：Close() 才是结算未决合并窗口的地方，
	// 不调用它，攒着的欢迎语既不会发出去，也不会有对应的动作日志。
	//
	// 这里用闭包而不是直接 defer closeAll(engines, activity)：defer 的
	// 参数在语句执行时就求值，若直接传值会捕获此刻仍是 nil 的 engines，
	// 后续循环里的 append 不会反映到已经求过值的参数上。闭包捕获的是
	// 变量本身，调用时才读取，因此能看到循环结束时的最终切片。
	defer func() {
		closeAll(engines, activity)
	}()

	for _, c := range cfgs {
		rt := runtimes[c.AccountName]

		// 解析真实房间号：配置里可能填的是短号
		info, err := rt.api.RoomInfo(ctx, c.RoomID)
		if err != nil {
			return fmt.Errorf("账号 %q 获取直播间 %s 信息失败: %w", c.AccountName, c.RoomID, err)
		}

		// 限流统一由 account.Binding 负责，这里传空限流器，
		// 否则与 Binding 的等待叠加会让实际间隔翻倍。
		actions := bilibili.NewActions(rt.api, ratelimit.NewInterval(0))
		if c.MaxLength > 0 {
			actions.SetMaxLength(c.MaxLength)
		}
		binding := &account.Binding{
			Account: rt.acc,
			RoomID:  info.RoomID,
			Actions: actions,
		}

		engine, err := rules.NewEngine(rules.EngineOptions{
			Label:          binding.Label(),
			RoomID:         info.RoomID,
			Rules:          c.Rules,
			Bot:            &roomBot{binding: binding, ctx: ctx},
			Storage:        st.BindingStorage(c.BindingID),
			Activity:       activity.Sink(c.AccountID, c.BindingID, info.RoomID),
			CooldownGroups: c.CooldownGroups,
			Logger:         log,
		})
		if err != nil {
			return fmt.Errorf("%s 的规则非法: %w", binding.Label(), err)
		}
		engines = append(engines, engine)

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

		client := bilibili.NewClient(info.RoomID, rt.api, bilibili.WithLogger(log))

		wg.Add(2)
		go func(eng *rules.Engine, c *bilibili.Client) {
			defer wg.Done()
			for ev := range c.Events() {
				eng.Handle(ev)
			}
		}(engine, client)

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

	sched.Start()
	log.Info("机器人已启动", "绑定数", len(cfgs), "账号数", len(runtimes))
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

// closeAll 按「引擎优先」的顺序关闭资源。
//
// engine.Close() 会结算未决的合并窗口（比如攒着的欢迎语），这一步会
// 通过 Activity sink 产生业务日志；activity.Close() 才是把这些日志真正
// 冲刷出去的地方。顺序反了不会报错，只会让那批日志静默消失。
func closeAll(engines []*rules.Engine, activity *logging.ActivityWriter) {
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
