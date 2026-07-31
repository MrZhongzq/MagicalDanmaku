package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"strings"
	"sync"
	"time"

	"github.com/MrZhongzq/MagicalDanmaku/server/internal/account"
	"github.com/MrZhongzq/MagicalDanmaku/server/internal/connector/bilibili"
	"github.com/MrZhongzq/MagicalDanmaku/server/internal/connector/bilibili/api"
	"github.com/MrZhongzq/MagicalDanmaku/server/internal/connector/bilibili/auth"
	"github.com/MrZhongzq/MagicalDanmaku/server/internal/ratelimit"
	"github.com/MrZhongzq/MagicalDanmaku/server/internal/rules"
	"github.com/MrZhongzq/MagicalDanmaku/server/internal/rules/config"
	"github.com/MrZhongzq/MagicalDanmaku/server/internal/scheduler"
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

// runRun 加载配置并启动机器人。
func runRun(args []string) error {
	fs := flag.NewFlagSet("run", flag.ExitOnError)
	cfgPath := fs.String("c", "", "配置文件路径（必填）")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *cfgPath == "" {
		return errors.New("必须通过 -c 指定配置文件")
	}

	cfg, err := config.Load(*cfgPath)
	if err != nil {
		return err
	}

	log := slog.Default()
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	runtimes, err := buildAccounts(cfg, log)
	if err != nil {
		return err
	}

	sched := scheduler.New(log)
	var wg sync.WaitGroup
	var engines []*rules.Engine

	for _, b := range cfg.Bindings() {
		rt := runtimes[b.AccountName]

		// 解析真实房间号：配置里可能填的是短号
		info, err := rt.api.RoomInfo(ctx, b.RoomID)
		if err != nil {
			return fmt.Errorf("账号 %q 获取直播间 %s 信息失败: %w", b.AccountName, b.RoomID, err)
		}

		// 限流统一由 account.Binding 负责，这里传空限流器，
		// 否则与 Binding 的等待叠加会让实际间隔翻倍。
		binding := &account.Binding{
			Account: rt.acc,
			RoomID:  info.RoomID,
			Actions: bilibili.NewActions(rt.api, ratelimit.NewInterval(0)),
		}

		engine, err := rules.NewEngine(rules.EngineOptions{
			Label:          binding.Label(),
			RoomID:         info.RoomID,
			Rules:          b.Rules,
			Bot:            &roomBot{binding: binding, ctx: ctx},
			CooldownGroups: b.CooldownGroups,
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
		log.Info("已配置绑定",
			"binding", binding.Label(),
			"title", info.Title,
			"status", status,
			"rules", len(b.Rules))

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

	sched.Start()
	log.Info("机器人已启动", "绑定数", len(cfg.Bindings()), "账号数", len(runtimes))
	fmt.Println("按 Ctrl+C 退出")

	<-ctx.Done()
	log.Info("正在退出...")

	sched.Stop()
	wg.Wait()
	for _, e := range engines {
		e.Close()
	}
	log.Info("已退出")
	return nil
}

// buildAccounts 载入全部账号并初始化各自的运行时资源。
//
// 同一账号在配置中出现多次（连接多个直播间）时只建一份，
// 保证限流器被真正共享。
func buildAccounts(cfg *config.Config, log *slog.Logger) (map[string]*accountRuntime, error) {
	out := make(map[string]*accountRuntime, len(cfg.Accounts))

	for _, a := range cfg.Accounts {
		data, err := os.ReadFile(a.CookieFile)
		if err != nil {
			return nil, fmt.Errorf("读取账号 %q 的 Cookie 文件失败: %w", a.Name, err)
		}
		sess, err := auth.ParseSession(strings.TrimSpace(string(data)))
		if err != nil {
			return nil, fmt.Errorf("账号 %q 的 Cookie 无效: %w", a.Name, err)
		}

		interval := time.Duration(a.RateLimit)
		if interval <= 0 {
			interval = defaultRateLimit
		}

		apiClient := api.New(sess)
		// wbi 签名每个账号刷新一次即可，其全部直播间共用
		if err := apiClient.RefreshNav(context.Background()); err != nil {
			return nil, fmt.Errorf("账号 %q 初始化签名失败: %w", a.Name, err)
		}

		out[a.Name] = &accountRuntime{
			acc: account.New(a.Name, sess, interval),
			api: apiClient,
		}
		log.Info("已载入账号",
			"name", a.Name, "uid", sess.UID,
			"直播间数", len(a.Rooms), "发送间隔", interval)
	}
	return out, nil
}
