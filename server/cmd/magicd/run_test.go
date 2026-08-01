package main

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/MrZhongzq/MagicalDanmaku/server/internal/event"
	"github.com/MrZhongzq/MagicalDanmaku/server/internal/logging"
	"github.com/MrZhongzq/MagicalDanmaku/server/internal/rules"
	"github.com/MrZhongzq/MagicalDanmaku/server/internal/rules/spec"
	"github.com/MrZhongzq/MagicalDanmaku/server/internal/scheduler"
	"github.com/MrZhongzq/MagicalDanmaku/server/internal/store"
)

// bindingStub 记录 roomBot 转发给绑定的调用。
// 它实现 run.go 中的 danmakuSender 接口，因此无需引入 account 包。
type bindingStub struct {
	mu     sync.Mutex
	sent   []string
	blocks []blockRecord
	err    error
}

type blockRecord struct {
	uid   string
	hours int
}

func (b *bindingStub) SendDanmaku(ctx context.Context, text string) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	// 模拟真实的 Limiter.Wait(ctx)：ctx 已取消时立刻返回 context.Canceled，
	// 不管有没有配置 err。这正是 review 里描述的失败模式，也是
	// TestCloseAllUsesShutdownBudgetForPendingSends 要验证的关键行为。
	if err := ctx.Err(); err != nil {
		return err
	}
	if b.err != nil {
		return b.err
	}
	b.sent = append(b.sent, text)
	return nil
}

func (b *bindingStub) Block(ctx context.Context, uid string, hours int) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.err != nil {
		return b.err
	}
	b.blocks = append(b.blocks, blockRecord{uid, hours})
	return nil
}

func TestRoomBotForwardsToBinding(t *testing.T) {
	bs := &bindingStub{}
	b := newRoomBot(bs, context.Background())

	if err := b.SendDanmaku("你好"); err != nil {
		t.Fatalf("SendDanmaku 失败: %v", err)
	}
	if len(bs.sent) != 1 || bs.sent[0] != "你好" {
		t.Errorf("sent = %v", bs.sent)
	}

	if err := b.Block("999", 12); err != nil {
		t.Fatalf("Block 失败: %v", err)
	}
	if len(bs.blocks) != 1 || bs.blocks[0].uid != "999" || bs.blocks[0].hours != 12 {
		t.Errorf("blocks = %v", bs.blocks)
	}
}

func TestRoomBotPropagatesError(t *testing.T) {
	bs := &bindingStub{err: errors.New("发送失败")}
	b := newRoomBot(bs, context.Background())

	if err := b.SendDanmaku("x"); err == nil {
		t.Error("底层错误应当上报")
	}
	if err := b.Block("1", 1); err == nil {
		t.Error("底层错误应当上报")
	}
}

func contains(s, sub string) bool {
	return len(s) >= len(sub) && (func() bool {
		for i := 0; i+len(sub) <= len(s); i++ {
			if s[i:i+len(sub)] == sub {
				return true
			}
		}
		return false
	})()
}

func TestRunRequiresDatabase(t *testing.T) {
	t.Setenv("MAGICD_DATABASE_URL", "")
	err := runRun([]string{})
	if err == nil {
		t.Fatal("没有数据库连接串应报错")
	}
	if !contains(err.Error(), "MAGICD_DATABASE_URL") {
		t.Errorf("错误信息应提示怎么配置，实际: %v", err)
	}
}

func TestRunRejectsUnreachableDatabase(t *testing.T) {
	// 端口 1 上不会有 PostgreSQL
	err := runRun([]string{"-db", "postgres://x:y@127.0.0.1:1/z?sslmode=disable&connect_timeout=1"})
	if err == nil {
		t.Fatal("连不上数据库应报错")
	}
}

func TestRetentionDaysFromEnv(t *testing.T) {
	t.Setenv("MAGICD_LOG_RETENTION_DAYS", "7")
	if got := retentionDays(); got != 7 {
		t.Errorf("= %d, 期望 7", got)
	}
}

func TestRetentionDaysDefault(t *testing.T) {
	t.Setenv("MAGICD_LOG_RETENTION_DAYS", "")
	if got := retentionDays(); got != 30 {
		t.Errorf("默认应为 30，实际 %d", got)
	}
}

func TestRetentionDaysZeroMeansNoPurge(t *testing.T) {
	t.Setenv("MAGICD_LOG_RETENTION_DAYS", "0")
	if got := retentionDays(); got != 0 {
		t.Errorf("0 表示不清理，实际 %d", got)
	}
}

func TestRetentionDaysIgnoresGarbage(t *testing.T) {
	// 环境变量写错就退回默认值，不该让机器人起不来
	t.Setenv("MAGICD_LOG_RETENTION_DAYS", "三十天")
	if got := retentionDays(); got != 30 {
		t.Errorf("非法值应退回默认 30，实际 %d", got)
	}
}

// TestCloseAllSettlesEngineWindowsBeforeFlushingWriter 验证 closeAll 的
// 关闭顺序：引擎必须先结算未决的合并窗口，写入器才能把结算产生的动作
// 日志冲刷出去。
//
// 用一个窗口设为 1 小时（必然不会自然到期）的聚合规则模拟「攒着的欢迎语」：
// 只有 engine.Close() 被调用，这条动作日志才会产生；只有此后
// activity.Close() 被调用，它才会被冲刷进 Flush。若 closeAll 的顺序反了
// （或者压根没关引擎），这条日志就不会出现在 flushed 里——这正是
// review 指出的、早返回路径曾经会触发的那个问题。
func TestCloseAllSettlesEngineWindowsBeforeFlushingWriter(t *testing.T) {
	var mu sync.Mutex
	var flushed []store.ActivityRow

	activity := logging.NewActivityWriter(logging.ActivityWriterOptions{
		Flush: func(_ context.Context, rows []store.ActivityRow) error {
			mu.Lock()
			defer mu.Unlock()
			flushed = append(flushed, rows...)
			return nil
		},
	})

	bot := newRoomBot(&bindingStub{}, context.Background())
	eng, err := rules.NewEngine(rules.EngineOptions{
		RoomID:   "123",
		Bot:      bot,
		Activity: activity.Sink(1, 1, "123"),
		Rules: []rules.Rule{{
			Name:      "进场欢迎",
			Enabled:   true,
			On:        []event.Type{event.TypeUserEnter},
			Aggregate: &rules.AggregateSpec{Window: time.Hour, By: rules.AggregateByType},
			Do:        []rules.Action{{Type: rules.ActionDanmaku, Template: []string{"欢迎"}}},
		}},
	})
	if err != nil {
		t.Fatalf("创建引擎失败: %v", err)
	}

	eng.Handle(event.Event{
		Type:    event.TypeUserEnter,
		Payload: event.UserEnter{User: event.User{UID: "1", Username: "观众"}},
	})

	closeAll([]*rules.Engine{eng}, []*roomBot{bot}, context.Background(), activity)

	mu.Lock()
	defer mu.Unlock()
	for _, r := range flushed {
		if r.Kind == store.ActivityAction {
			return // 找到了结算产生的动作日志，顺序正确
		}
	}
	t.Error("closeAll 应先让引擎结算未决窗口，再让写入器冲刷——" +
		"未在 flushed 里找到窗口结算产生的动作日志")
}

// TestCloseAllUsesShutdownBudgetForPendingSends 验证 review 项 C 的修复：
// 关停时未决合并窗口结算产生的弹幕，必须真的发得出去，而不是因为
// roomBot 还在用已取消的主 ctx 而立刻拿到 context.Canceled。
//
// bindingStub.SendDanmaku 会在 ctx 已取消时立刻返回 context.Canceled
// （模拟真实的 Limiter.Wait(ctx) 行为）。这里给 roomBot 一个已取消的 ctx，
// 但 closeAll 收到的 shutdownCtx 是有效的——只有 closeAll 真的把 roomBot
// 的发送 ctx 换成 shutdownCtx，窗口结算时的弹幕才能发出去。
func TestCloseAllUsesShutdownBudgetForPendingSends(t *testing.T) {
	var mu sync.Mutex
	var flushed []store.ActivityRow

	activity := logging.NewActivityWriter(logging.ActivityWriterOptions{
		Flush: func(_ context.Context, rows []store.ActivityRow) error {
			mu.Lock()
			defer mu.Unlock()
			flushed = append(flushed, rows...)
			return nil
		},
	})

	stub := &bindingStub{}
	canceledCtx, cancel := context.WithCancel(context.Background())
	cancel() // 模拟 Ctrl+C 已经发生：roomBot 目前持有的就是这个 ctx

	bot := newRoomBot(stub, canceledCtx)
	eng, err := rules.NewEngine(rules.EngineOptions{
		RoomID:   "123",
		Bot:      bot,
		Activity: activity.Sink(1, 1, "123"),
		Rules: []rules.Rule{{
			Name:      "进场欢迎",
			Enabled:   true,
			On:        []event.Type{event.TypeUserEnter},
			Aggregate: &rules.AggregateSpec{Window: time.Hour, By: rules.AggregateByType},
			Do:        []rules.Action{{Type: rules.ActionDanmaku, Template: []string{"欢迎"}}},
		}},
	})
	if err != nil {
		t.Fatalf("创建引擎失败: %v", err)
	}

	eng.Handle(event.Event{
		Type:    event.TypeUserEnter,
		Payload: event.UserEnter{User: event.User{UID: "1", Username: "观众"}},
	})

	// shutdownCtx 未取消：closeAll 必须把它换给 bot，结算才能发出去
	closeAll([]*rules.Engine{eng}, []*roomBot{bot}, context.Background(), activity)

	stub.mu.Lock()
	sent := len(stub.sent)
	stub.mu.Unlock()
	if sent != 1 {
		t.Fatalf("关停时未决窗口的弹幕应发得出去，实际发送 %d 条", sent)
	}

	mu.Lock()
	defer mu.Unlock()
	for _, r := range flushed {
		if r.Kind == store.ActivityAction && r.Detail != nil &&
			!contains(string(r.Detail), "error") {
			return // 找到了不带 error 的动作日志：发送真的成功了
		}
	}
	t.Error("动作日志应不带 error（发送应当成功），实际 flushed 里没有找到")
}

func TestHTTPAddrDefault(t *testing.T) {
	t.Setenv("MAGICD_HTTP_ADDR", "")
	// 显式设为空串表示关闭，与「未设置」不同
	if got := httpAddr(); got != "" {
		t.Errorf("显式空串应表示关闭，实际 %q", got)
	}
}

func TestHTTPAddrUnsetUsesLocalhost(t *testing.T) {
	os.Unsetenv("MAGICD_HTTP_ADDR")
	if got := httpAddr(); got != "127.0.0.1:8080" {
		t.Errorf("默认应只监听本机，实际 %q", got)
	}
}

func TestHTTPAddrExplicit(t *testing.T) {
	t.Setenv("MAGICD_HTTP_ADDR", "0.0.0.0:9000")
	if got := httpAddr(); got != "0.0.0.0:9000" {
		t.Errorf("= %q", got)
	}
}

// newReloadTestStore 建一个独立 schema 的真实存储，供 roomRuntime.Reload
// 的测试使用——Reload 直接依赖 *store.Store 的 LoadRunConfig，绕不开真库。
func newReloadTestStore(t *testing.T) (*store.Store, int64, int64) {
	t.Helper()
	dsn := os.Getenv("MAGICD_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("未设置 MAGICD_TEST_DATABASE_URL，跳过需要真实数据库的测试。\n" +
			"本地起库：docker compose -f docker-compose.dev.yml up -d\n" +
			"然后：export MAGICD_TEST_DATABASE_URL='postgres://magicd:magicd@localhost:5433/magicd?sslmode=disable'")
	}

	const schema = "h_magicd_reload_test"
	ctx := context.Background()

	admin, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatalf("连接数据库失败: %v", err)
	}
	if _, err := admin.Exec(ctx, `DROP SCHEMA IF EXISTS `+schema+` CASCADE`); err != nil {
		admin.Close()
		t.Fatalf("清理旧 schema 失败: %v", err)
	}
	if _, err := admin.Exec(ctx, `CREATE SCHEMA `+schema); err != nil {
		admin.Close()
		t.Fatalf("创建 schema 失败: %v", err)
	}

	var st *store.Store
	t.Cleanup(func() {
		if st != nil {
			st.Close()
		}
		if _, err := admin.Exec(context.Background(),
			`DROP SCHEMA IF EXISTS `+schema+` CASCADE`); err != nil {
			t.Logf("清理 schema 失败: %v", err)
		}
		admin.Close()
	})

	st, err = store.OpenWithSchema(ctx, dsn, schema)
	if err != nil {
		t.Fatalf("打开存储失败: %v", err)
	}
	if err := st.Migrate(ctx); err != nil {
		t.Fatalf("迁移失败: %v", err)
	}

	owner, err := st.CreateUser(ctx, "张三", "密码123456", false)
	if err != nil {
		t.Fatalf("建用户报错: %v", err)
	}
	acc, err := st.CreateAccount(ctx, store.AccountInput{
		Name: "小号", Cookie: "SESSDATA=x", OwnerID: owner.ID,
		RateLimit: time.Second, MaxLength: 40,
	})
	if err != nil {
		t.Fatalf("建账号报错: %v", err)
	}
	b, err := st.UpsertBinding(ctx, acc.ID, "123")
	if err != nil {
		t.Fatalf("建绑定报错: %v", err)
	}
	return st, acc.ID, b.ID
}

// TestRoomRuntimeReloadSwapsEngine 证明 Reload 真的把引擎换成了新的——
// 而不是仅仅报告成功却仍在用旧规则处理事件。
//
// 手法：初始引擎（不经过数据库，直接构造）挂一条命中就发「旧规则响应」
// 的规则；数据库里存的是另一条命中就发「新规则响应」的规则。Reload
// 之后再喂一个弹幕事件，若 rt.engine 真的被换掉，bindingStub 收到的
// 应该是「新规则响应」；若 Swap 被跳过，会依然是「旧规则响应」。
func TestRoomRuntimeReloadSwapsEngine(t *testing.T) {
	st, _, bindingID := newReloadTestStore(t)
	ctx := context.Background()

	// 数据库里存新规则，Reload 会从这里读到
	if err := st.ReplaceRules(ctx, bindingID, []spec.Rule{{
		Name: "规则",
		On:   []string{"danmaku"},
		Do:   []spec.Action{{Type: "danmaku", Template: []string{"新规则响应"}}},
	}}); err != nil {
		t.Fatalf("写规则报错: %v", err)
	}

	stub := &bindingStub{}
	bot := newRoomBot(stub, ctx)

	activity := logging.NewActivityWriter(logging.ActivityWriterOptions{
		Flush: func(context.Context, []store.ActivityRow) error { return nil },
	})
	t.Cleanup(activity.Close)

	oldEngine, err := rules.NewEngine(rules.EngineOptions{
		Label:  "小号@123",
		RoomID: "123",
		Bot:    bot,
		Rules: []rules.Rule{{
			Name:    "旧规则",
			Enabled: true,
			On:      []event.Type{event.TypeDanmaku},
			Do:      []rules.Action{{Type: rules.ActionDanmaku, Template: []string{"旧规则响应"}}},
		}},
	})
	if err != nil {
		t.Fatalf("创建初始引擎报错: %v", err)
	}

	rt := &roomRuntime{
		bot:       bot,
		sink:      activity.Sink(1, bindingID, "123"),
		storage:   rules.NewMemStorage(),
		label:     "小号@123",
		roomID:    "123",
		bindingID: bindingID,
		st:        st,
		sched:     scheduler.New(slog.Default()),
		log:       slog.Default(),
	}
	rt.engine.Store(oldEngine)

	if err := rt.Reload(ctx); err != nil {
		t.Fatalf("Reload 报错: %v", err)
	}

	rt.Engine().Handle(event.Event{
		Type:    event.TypeDanmaku,
		Payload: event.Danmaku{User: event.User{UID: "1", Username: "观众"}, Text: "你好"},
	})

	stub.mu.Lock()
	sent := append([]string{}, stub.sent...)
	stub.mu.Unlock()

	if len(sent) != 1 || sent[0] != "新规则响应" {
		t.Fatalf("Reload 后应由新引擎处理事件，实际发送 = %v（期望 [\"新规则响应\"]，"+
			"若看到 \"旧规则响应\" 说明引擎没有真的被换掉）", sent)
	}
}

// TestRoomRuntimeReloadFlushesPendingAggregateWindow 验证 Reload 收尾时
// 旧引擎的 Close() 真的结算了未决的合并窗口——那里面攒着待发的欢迎语。
//
// 这条不能用 cmd/magicd 里已有的 TestCloseAllUsesShutdownBudgetForPendingSends
// 代替：那条测的是关停路径，先调用了 roomBot.setCtx(shutdownCtx) 换上
// 一个有超时预算、脱离取消链的 ctx，前提是「主 ctx 已被取消」。Reload
// 路径完全不调 setCtx，依赖的是另一个前提——「主 ctx 仍然存活」（Reload
// 由 HTTP 请求触发时，进程根本没有在关停）。两条路径的前提不同，一条
// 测试不能替另一条作证。
//
// 手法：给旧引擎挂一条窗口长达 1 小时（必然不会自然到期）的聚合规则，
// 喂一个进场事件让它进入待决窗口；然后调用 Reload。若 Reload 收尾时
// 真的 Close 了旧引擎，Aggregator.Close() 会同步结算并通过 bot 把
// 欢迎语发出去；若收尾时机不对或压根没关旧引擎，bot 什么也收不到。
func TestRoomRuntimeReloadFlushesPendingAggregateWindow(t *testing.T) {
	st, _, bindingID := newReloadTestStore(t)
	ctx := context.Background()

	// Reload 要能读到合法配置，规则内容与本测试验证的行为无关
	if err := st.ReplaceRules(ctx, bindingID, []spec.Rule{{
		Name: "规则",
		On:   []string{"danmaku"},
		Do:   []spec.Action{{Type: "danmaku", Template: []string{"响应"}}},
	}}); err != nil {
		t.Fatalf("写规则报错: %v", err)
	}

	stub := &bindingStub{}
	bot := newRoomBot(stub, ctx)
	activity := logging.NewActivityWriter(logging.ActivityWriterOptions{
		Flush: func(context.Context, []store.ActivityRow) error { return nil },
	})
	t.Cleanup(activity.Close)

	oldEngine, err := rules.NewEngine(rules.EngineOptions{
		Label: "小号@123", RoomID: "123", Bot: bot,
		Rules: []rules.Rule{{
			Name:      "进场欢迎",
			Enabled:   true,
			On:        []event.Type{event.TypeUserEnter},
			Aggregate: &rules.AggregateSpec{Window: time.Hour, By: rules.AggregateByType},
			Do:        []rules.Action{{Type: rules.ActionDanmaku, Template: []string{"欢迎"}}},
		}},
	})
	if err != nil {
		t.Fatalf("创建旧引擎报错: %v", err)
	}

	rt := &roomRuntime{
		bot: bot, sink: activity.Sink(1, bindingID, "123"), storage: rules.NewMemStorage(),
		label: "小号@123", roomID: "123", bindingID: bindingID,
		st: st, sched: scheduler.New(slog.Default()), log: slog.Default(),
	}
	rt.engine.Store(oldEngine)

	// 进场事件进入待决窗口，1 小时内不会自然结算
	rt.Engine().Handle(event.Event{
		Type:    event.TypeUserEnter,
		Payload: event.UserEnter{User: event.User{UID: "1", Username: "观众"}},
	})

	stub.mu.Lock()
	before := len(stub.sent)
	stub.mu.Unlock()
	if before != 0 {
		t.Fatalf("窗口结算前不该有发送，实际 = %d 条", before)
	}

	if err := rt.Reload(ctx); err != nil {
		t.Fatalf("Reload 报错: %v", err)
	}

	stub.mu.Lock()
	sent := append([]string{}, stub.sent...)
	stub.mu.Unlock()

	if len(sent) != 1 || sent[0] != "欢迎" {
		t.Fatalf("Reload 应结算旧引擎未决的合并窗口并把欢迎语发出去，实际发送 = %v", sent)
	}
}

// TestRoomRuntimeConcurrentReloadAndEventFanoutIsRaceFree 用 -race 验证
// run.go 里事件扇出的 goroutine（每条事件都调 rt.Engine()，不缓存）与
// Reload 的 atomic.Pointer 替换是并发安全的，且多个 Reload 之间本身
// 也要互相安全——两个人同时按保存（或一个人双击）时，reloadMu 必须
// 把「构造 → Swap → 重建定时任务 → 关旧引擎」这一串复合操作串行化，
// 否则调度器里可能留下一个指向已被 Close 的旧引擎的条目，定时规则
// 从此静默停摆。
//
// 这不是行为断言（谁的 Reload 最终生效不保证，只要求不 panic、不
// deadlock、不留悬空状态），纯粹是给竞态检测器的靶子：一边不停地
// rt.Engine().Handle(ev)，一边起多个 goroutine **真正并发地**调用
// rt.Reload()，若哪处偷偷缓存了 *rules.Engine、漏了原子操作，或者
// reloadMu 没有把复合操作真正串行化，-race 会报出来。
func TestRoomRuntimeConcurrentReloadAndEventFanoutIsRaceFree(t *testing.T) {
	st, _, bindingID := newReloadTestStore(t)
	ctx := context.Background()

	if err := st.ReplaceRules(ctx, bindingID, []spec.Rule{{
		Name: "规则",
		On:   []string{"danmaku"},
		Do:   []spec.Action{{Type: "danmaku", Template: []string{"响应"}}},
	}}); err != nil {
		t.Fatalf("写规则报错: %v", err)
	}

	stub := &bindingStub{}
	bot := newRoomBot(stub, ctx)
	activity := logging.NewActivityWriter(logging.ActivityWriterOptions{
		Flush: func(context.Context, []store.ActivityRow) error { return nil },
	})
	t.Cleanup(activity.Close)

	initEngine, err := rules.NewEngine(rules.EngineOptions{
		Label: "小号@123", RoomID: "123", Bot: bot,
		Rules: []rules.Rule{{
			Name: "初始规则", Enabled: true,
			On: []event.Type{event.TypeDanmaku},
			Do: []rules.Action{{Type: rules.ActionDanmaku, Template: []string{"初始响应"}}},
		}},
	})
	if err != nil {
		t.Fatalf("创建初始引擎报错: %v", err)
	}

	rt := &roomRuntime{
		bot: bot, sink: activity.Sink(1, bindingID, "123"), storage: rules.NewMemStorage(),
		label: "小号@123", roomID: "123", bindingID: bindingID,
		st: st, sched: scheduler.New(slog.Default()), log: slog.Default(),
	}
	rt.engine.Store(initEngine)

	var wg sync.WaitGroup
	stop := make(chan struct{})

	// 模拟 run.go 的事件扇出：每条事件都重新取 rt.Engine()，不缓存
	wg.Add(1)
	go func() {
		defer wg.Done()
		ev := event.Event{
			Type:    event.TypeDanmaku,
			Payload: event.Danmaku{User: event.User{UID: "1", Username: "观众"}, Text: "你好"},
		}
		for {
			select {
			case <-stop:
				return
			default:
				rt.Engine().Handle(ev)
			}
		}
	}()

	// 多个 goroutine 真正并发地反复 Reload，模拟多人同时按保存
	const reloaders = 5
	const reloadsPerGoroutine = 10
	var reloadWG sync.WaitGroup
	errCh := make(chan error, reloaders*reloadsPerGoroutine)
	reloadWG.Add(reloaders)
	for i := 0; i < reloaders; i++ {
		go func() {
			defer reloadWG.Done()
			for j := 0; j < reloadsPerGoroutine; j++ {
				if err := rt.Reload(ctx); err != nil {
					errCh <- err
				}
			}
		}()
	}
	reloadWG.Wait()
	close(errCh)

	close(stop)
	wg.Wait()

	for err := range errCh {
		t.Errorf("并发 Reload 报错: %v", err)
	}
}
