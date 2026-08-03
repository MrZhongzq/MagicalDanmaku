package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/MrZhongzq/MagicalDanmaku/server/internal/account"
	"github.com/MrZhongzq/MagicalDanmaku/server/internal/connector"
	"github.com/MrZhongzq/MagicalDanmaku/server/internal/connector/bilibili/api"
	"github.com/MrZhongzq/MagicalDanmaku/server/internal/connector/bilibili/auth"
	"github.com/MrZhongzq/MagicalDanmaku/server/internal/event"
	"github.com/MrZhongzq/MagicalDanmaku/server/internal/httpapi"
	"github.com/MrZhongzq/MagicalDanmaku/server/internal/logging"
	"github.com/MrZhongzq/MagicalDanmaku/server/internal/rules"
	"github.com/MrZhongzq/MagicalDanmaku/server/internal/rules/spec"
	"github.com/MrZhongzq/MagicalDanmaku/server/internal/scheduler"
	"github.com/MrZhongzq/MagicalDanmaku/server/internal/store"
)

// ---- P5-1：绑定的增删启停在运行期生效——runtimeManager 的装配/拆除 ----
//
// 这组测试用的是真实的 buildRoomRuntime/bilibili.Client，而不是
// httpapi 包那种"只验证 handler 有没有调用接口"的假实现——runtimeManager
// 本身就是"真的建连接、真的起 goroutine、真的能被拆干净"这件事的实现，
// 糊弄它自己的测试没有意义。
//
// 网络全部指向本地假服务器，不发任何真实请求：
//   - nav / roomInfo 落在同一个 httptest.Server 上，用有没有 room_id
//     查询参数区分；
//   - danmuInfo 故意指向一个必然连接被拒的地址（127.0.0.1:1），
//     让 bilibili.Client.Run 的重连循环快速失败、进入退避等待——
//     这样才有一个真实在跑、能被 ctx 取消终止的 goroutine 可供测试
//     拆除路径，同时不需要搭一整套 WebSocket 握手假服务器（那是
//     internal/connector/bilibili 包自己测协议细节的地方，这里只关心
//     生命周期，不关心协议）。

// fakeAPISink 记录被调用的绑定 ID，不依赖真实的 httpapi.Server——
// 与 httpapi 包自己的 fakeLifecycle 是同一个理由：测试装配/拆除逻辑
// 不需要真的认证、真的路由。
type fakeAPISink struct {
	mu        sync.Mutex
	put       []int64
	removed   []int64
	published int
}

func (f *fakeAPISink) PutRuntime(id int64, _ httpapi.BindingRuntime) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.put = append(f.put, id)
}

func (f *fakeAPISink) RemoveRuntime(id int64) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.removed = append(f.removed, id)
}

func (f *fakeAPISink) Publish(int64, event.Event) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.published++
}

func (f *fakeAPISink) snapshot() (put, removed []int64) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]int64{}, f.put...), append([]int64{}, f.removed...)
}

// newRuntimeManagerTestStore 建一个独立 schema 的真实存储。runtimeManager
// 直接依赖 *store.Store 的 LoadRunConfig，绕不开真库——与
// newReloadTestStore 同一个理由，只是换一个独立的 schema 名以免与
// 其余测试文件的固定 schema 互相 DROP。
func newRuntimeManagerTestStore(t *testing.T) *store.Store {
	t.Helper()
	dsn := os.Getenv("MAGICD_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("未设置 MAGICD_TEST_DATABASE_URL，跳过需要真实数据库的测试。\n" +
			"本地起库：docker compose -f docker-compose.dev.yml up -d\n" +
			"然后：export MAGICD_TEST_DATABASE_URL='postgres://magicd:magicd@localhost:5433/magicd?sslmode=disable'")
	}

	const schema = "m_runtime_manager_test"
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
	return st
}

// newFakeBilibiliInfoServer 起一个假 HTTP 服务器，同时充当 nav、roomInfo
// 与 sendMsg 三个接口：sendMsg 是 POST（表单体），nav/roomInfo 是 GET，
// 二者再按有没有 room_id 查询参数区分——用这几条规则区分，不用建三个
// 端口。
//
// 处理 POST（sendMsg）是 Critical-1 回归测试需要的：验证 roomBot 的
// 发送 ctx 有没有被正确收敛到绑定级 bindCtx，最直接的办法就是真的走一遍
// SendDanmaku，而不是只读它私有字段——但这必须发生在一个可控的假接口
// 上，不能碰真实 B 站。
func newFakeBilibiliInfoServer(t *testing.T) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			w.Write([]byte(`{"code":0,"message":"0","data":{}}`))
			return
		}
		if roomID := r.URL.Query().Get("room_id"); roomID != "" {
			fmt.Fprintf(w, `{"code":0,"data":{"room_id":%s,"uid":1,"title":"标题","live_status":0}}`, roomID)
			return
		}
		w.Write([]byte(`{"code":0,"data":{"wbi_img":{
			"img_url":"https://i0.hdslb.com/bfs/wbi/0123456789abcdef0123456789abcdef.png",
			"sub_url":"https://i0.hdslb.com/bfs/wbi/fedcba9876543210fedcba9876543210.png"
		}}}`))
	}))
	t.Cleanup(srv.Close)
	return srv
}

// runtimeManagerTestBinding 建「用户 + 账号 + 绑定」，账号 Cookie 是随意
// 构造的合法格式（RefreshNav/RoomInfo 都走假服务器，Cookie 内容本身
// 不重要），绑定按需带一批规则。返回 bindingID 与该绑定的 label
// （"账号名@房间号"，与 roomRuntime.label 的格式一致，供测试断言定时
// 任务前缀）。
func runtimeManagerTestBinding(
	t *testing.T, st *store.Store, ownerName, accountName, roomID string, ruleSpecs []spec.Rule,
) (bindingID int64, label string) {
	t.Helper()
	ctx := context.Background()

	owner, err := st.GetUserByName(ctx, ownerName)
	if err != nil {
		owner, err = st.CreateUser(ctx, ownerName, "密码123456", false)
		if err != nil {
			t.Fatalf("建用户报错: %v", err)
		}
	}
	acc, err := st.CreateAccount(ctx, store.AccountInput{
		Name: accountName, Cookie: "SESSDATA=x; bili_jct=y; DedeUserID=1",
		OwnerID: owner.ID, RateLimit: time.Second, MaxLength: 40,
	})
	if err != nil {
		t.Fatalf("建账号报错: %v", err)
	}
	b, err := st.UpsertBinding(ctx, acc.ID, roomID)
	if err != nil {
		t.Fatalf("建绑定报错: %v", err)
	}
	if len(ruleSpecs) > 0 {
		if err := st.ReplaceRules(ctx, b.ID, ruleSpecs); err != nil {
			t.Fatalf("写规则报错: %v", err)
		}
	}
	return b.ID, accountName + "@" + roomID
}

// newTestRuntimeManager 建一个指向假 B 站接口的 runtimeManager：
// nav/roomInfo 走 infoSrv，danmuInfo 故意指向必然连接被拒的地址，
// 让连接 goroutine 快速失败并进入退避等待——足够验证生命周期管理，
// 不需要真实握手成功。
func newTestRuntimeManager(t *testing.T, ctx context.Context, st *store.Store, wg *sync.WaitGroup, api apiRuntimeSink) *runtimeManager {
	t.Helper()
	sched := scheduler.New(slog.Default())
	activity := logging.NewActivityWriter(logging.ActivityWriterOptions{
		Flush: func(context.Context, []store.ActivityRow) error { return nil },
	})
	t.Cleanup(activity.Close)

	rm := newRuntimeManager(ctx, st, sched, activity, api, slog.Default(), wg, nil)
	return rm
}

// seedFakeAccount 直接往 rm.accounts 里塞一份指向假接口的 accountRuntime，
// 绕开 buildAccountRuntime 自己的构造过程。
//
// 不能先调 buildAccountRuntime 再事后覆盖 base URL：buildAccountRuntime
// 内部会用默认（真实 bilibili.com）地址立刻调一次 RefreshNav，等它
// 返回时已经发出了一次真实网络请求（或者在没有真实网络的 CI 环境里
// 直接超时失败）。这里手动重现 buildAccountRuntime 的构造步骤，只是
// 把 api.New 的 base URL 换成假服务器——步骤与 buildAccountRuntime
// 完全一致，只是不能复用它（复用不了的原因就是上面这一条，不是懒得
// 抽象）。ensureAccountRuntime 命中 rm.accounts 缓存就不会再重建，
// 因此这份手工装配的账号会一路带进真实的 StartBinding/assembleOne
// 装配路径。
func seedFakeAccount(t *testing.T, rm *runtimeManager, cfg store.RunConfig, infoSrv *httptest.Server) {
	t.Helper()
	sess, err := auth.ParseSession(cfg.Cookie)
	if err != nil {
		t.Fatalf("解析 Cookie 报错: %v", err)
	}
	apiClient := api.New(sess, api.WithHTTPClient(infoSrv.Client()))
	apiClient.SetBaseURL("nav", infoSrv.URL)
	apiClient.SetBaseURL("roomInfo", infoSrv.URL)
	// 127.0.0.1:1 上没有任何服务在监听，连接必然被立刻拒绝——
	// 不需要搭假 WebSocket 服务器就能让连接 goroutine 真实地跑起来、
	// 快速失败、进入可被 ctx 取消中断的退避等待。
	apiClient.SetBaseURL("danmuInfo", "http://127.0.0.1:1")
	// sendMsg 也指向假服务器：Critical-1 的回归测试要真的调用
	// SendDanmaku 走一遍完整发送路径，不能碰真实 B 站。
	apiClient.SetBaseURL("sendMsg", infoSrv.URL)
	if err := apiClient.RefreshNav(context.Background()); err != nil {
		t.Fatalf("RefreshNav 报错: %v", err)
	}

	interval := cfg.RateLimit
	if interval <= 0 {
		interval = defaultRateLimit
	}
	rm.accounts[cfg.AccountName] = &accountRuntime{
		acc:    account.New(cfg.AccountName, sess, interval),
		api:    apiClient,
		cookie: cfg.Cookie,
	}
}

// waitForState 轮询直到 s() 返回 want 或超时，用于确认连接 goroutine
// 真的在跑（状态会从 idle 变成 resolving/reconnecting），而不是固定
// sleep 再赌一把时序。
func waitForState(t *testing.T, timeout time.Duration, s func() connector.State, want connector.State) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for {
		if got := s(); got == want {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("等待状态变为 %q 超时，最后一次读到 %q", want, s())
		}
		time.Sleep(5 * time.Millisecond)
	}
}

// waitForNotState 轮询直到 s() 不再是 not 或超时。
func waitForNotState(t *testing.T, timeout time.Duration, s func() connector.State, not connector.State) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for {
		if got := s(); got != not {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("等待状态离开 %q 超时", not)
		}
		time.Sleep(5 * time.Millisecond)
	}
}

// TestStartBindingConnectsAndRegisters 验证 P5-1 的核心验收场景：
// StartBinding 之后不需要重启进程，绑定就真的建了连接（状态离开
// idle）、登记进了运行时注册表——这正是真机故障单里"加完绑定，日志
// 一条不动"要修的那个洞。
func TestStartBindingConnectsAndRegisters(t *testing.T) {
	st := newRuntimeManagerTestStore(t)
	infoSrv := newFakeBilibiliInfoServer(t)
	ctx := context.Background()

	bindingID, _ := runtimeManagerTestBinding(t, st, "张三", "小号", "123", nil)

	var wg sync.WaitGroup
	sink := &fakeAPISink{}
	rm := newTestRuntimeManager(t, ctx, st, &wg, sink)
	cfgs, err := st.LoadRunConfig(ctx)
	if err != nil {
		t.Fatalf("读取配置报错: %v", err)
	}
	seedFakeAccount(t, rm, cfgs[0], infoSrv)

	if err := rm.StartBinding(ctx, bindingID); err != nil {
		t.Fatalf("StartBinding 报错: %v", err)
	}
	t.Cleanup(func() { rm.StopBinding(ctx, bindingID) })

	lb, ok := rm.live[bindingID]
	if !ok {
		t.Fatal("StartBinding 之后 rm.live 里应该有这个绑定")
	}
	waitForNotState(t, 2*time.Second, lb.client.State, connector.StateIdle)

	put, _ := sink.snapshot()
	if len(put) != 1 || put[0] != bindingID {
		t.Errorf("PutRuntime 调用记录 = %v, 期望恰好 [%d]", put, bindingID)
	}

	// 幂等：已经在跑的绑定重复 StartBinding 不该报错，也不该建出第二份
	// 连接/登记第二次——否则第一份连接就成了悬挂的 goroutine，谁也管不到它。
	if err := rm.StartBinding(ctx, bindingID); err != nil {
		t.Fatalf("重复 StartBinding 不该报错: %v", err)
	}
	put, _ = sink.snapshot()
	if len(put) != 1 {
		t.Errorf("重复 StartBinding 不该重新登记，PutRuntime 调用记录 = %v", put)
	}
}

// TestStopBindingNormalTeardownIsClean 是「正常停用」路径：绑定正常
// 连接着（虽然示例里连不上真实 B 站，但连接 goroutine 是真实在跑、
// 真实在退避重连的），调用 StopBinding 之后必须干净：
//   - 连接 goroutine 真的退出了（client.State() 变成 closed，这是
//     bilibili.Client.Run 的收尾 defer 里设的，只有 goroutine 真的
//     返回了才会看到）——不是"从表里摘掉就假装没事"；
//   - 定时任务被摘干净（自检 (a)：去掉 RemoveByPrefix 这一步应该被
//     这条断言抓住，见任务报告里手工验证的记录）；
//   - 从 httpapi 的运行时注册表摘除。
func TestStopBindingNormalTeardownIsClean(t *testing.T) {
	st := newRuntimeManagerTestStore(t)
	infoSrv := newFakeBilibiliInfoServer(t)
	ctx := context.Background()

	bindingID, label := runtimeManagerTestBinding(t, st, "张三", "小号", "123", []spec.Rule{
		{Name: "定时问候", Schedule: "* * * * * *", Do: []spec.Action{{Type: "danmaku", Template: []string{"大家好"}}}},
	})

	var wg sync.WaitGroup
	sink := &fakeAPISink{}
	rm := newTestRuntimeManager(t, ctx, st, &wg, sink)
	cfgs, err := st.LoadRunConfig(ctx)
	if err != nil {
		t.Fatalf("读取配置报错: %v", err)
	}
	seedFakeAccount(t, rm, cfgs[0], infoSrv)

	if err := rm.StartBinding(ctx, bindingID); err != nil {
		t.Fatalf("StartBinding 报错: %v", err)
	}
	lb := rm.live[bindingID]
	waitForNotState(t, 2*time.Second, lb.client.State, connector.StateIdle)

	rm.StopBinding(ctx, bindingID)

	if got := lb.client.State(); got != connector.StateClosed {
		t.Errorf("StopBinding 返回后连接状态 = %q, 期望 %q——"+
			"说明连接 goroutine 没有真的退出，是悬挂的", got, connector.StateClosed)
	}

	if _, ok := rm.live[bindingID]; ok {
		t.Error("StopBinding 之后 rm.live 不该再有这个绑定")
	}

	// 自检 (a)：定时任务要摘干净。RemoveByPrefix 若返回非 0，说明
	// teardownLocked 里那一步没生效（或压根没调），残留的条目会在
	// 未来这个绑定重新启用时造成"同一条规则触发两次"（cron 表达式变了
	// 的话）或指向一个已经 Close 掉的旧引擎。
	if n := rm.sched.RemoveByPrefix(label + "/"); n != 0 {
		t.Errorf("StopBinding 之后仍残留 %d 条定时任务，前缀 %q——应该已经被摘干净", n, label+"/")
	}

	_, removed := sink.snapshot()
	if len(removed) != 1 || removed[0] != bindingID {
		t.Errorf("RemoveRuntime 调用记录 = %v, 期望恰好 [%d]", removed, bindingID)
	}
}

// TestStartBindingRegistersScheduledRule 独立验证"定时任务真的注册过"，
// 与上一条测试互补：上一条测试的断言只能证明"停用之后没有残留"，
// 但如果 StartBinding 那一步压根没注册成功，停用之后自然也查不到
// 残留——两个"没有"会被误判为"干净"。这里在停用之前先确认存在。
func TestStartBindingRegistersScheduledRule(t *testing.T) {
	st := newRuntimeManagerTestStore(t)
	infoSrv := newFakeBilibiliInfoServer(t)
	ctx := context.Background()

	bindingID, label := runtimeManagerTestBinding(t, st, "张三", "小号", "123", []spec.Rule{
		{Name: "定时问候", Schedule: "* * * * * *", Do: []spec.Action{{Type: "danmaku", Template: []string{"大家好"}}}},
	})

	var wg sync.WaitGroup
	rm := newTestRuntimeManager(t, ctx, st, &wg, nil)
	cfgs, err := st.LoadRunConfig(ctx)
	if err != nil {
		t.Fatalf("读取配置报错: %v", err)
	}
	seedFakeAccount(t, rm, cfgs[0], infoSrv)

	if err := rm.StartBinding(ctx, bindingID); err != nil {
		t.Fatalf("StartBinding 报错: %v", err)
	}
	t.Cleanup(func() { rm.StopBinding(ctx, bindingID) })

	if n := rm.sched.RemoveByPrefix(label + "/"); n != 1 {
		t.Errorf("StartBinding 应该注册 1 条定时任务，实际摘到 %d 条", n)
	}
}

// TestStopBindingIsIdempotent 是「异常停用」的一种：重复停用（比如
// 用户手快点了两次停用按钮，或者删除绑定时它已经被前一次请求停用过）
// 不该 panic、不该重复摘除、不该 hang。
func TestStopBindingIsIdempotent(t *testing.T) {
	st := newRuntimeManagerTestStore(t)
	infoSrv := newFakeBilibiliInfoServer(t)
	ctx := context.Background()

	bindingID, _ := runtimeManagerTestBinding(t, st, "张三", "小号", "123", nil)

	var wg sync.WaitGroup
	sink := &fakeAPISink{}
	rm := newTestRuntimeManager(t, ctx, st, &wg, sink)
	cfgs, err := st.LoadRunConfig(ctx)
	if err != nil {
		t.Fatalf("读取配置报错: %v", err)
	}
	seedFakeAccount(t, rm, cfgs[0], infoSrv)

	if err := rm.StartBinding(ctx, bindingID); err != nil {
		t.Fatalf("StartBinding 报错: %v", err)
	}
	lb := rm.live[bindingID]
	waitForNotState(t, 2*time.Second, lb.client.State, connector.StateIdle)

	rm.StopBinding(ctx, bindingID)
	rm.StopBinding(ctx, bindingID) // 第二次：不该 panic，也不该再拆一遍

	_, removed := sink.snapshot()
	if len(removed) != 1 {
		t.Errorf("RemoveRuntime 调用记录 = %v, 期望恰好 1 次（第二次 Stop 应是空操作）", removed)
	}
}

// TestStopBindingOnNeverStartedBindingIsNoop 覆盖「从未成功启动过的
// 绑定被停用」——例如账号 Cookie 失效导致 StartBinding 失败，DB 里
// enabled 仍然被改成了 true/false，随后 handler 仍会调 StopBinding：
// 不该 panic，也不该碰注册表。
func TestStopBindingOnNeverStartedBindingIsNoop(t *testing.T) {
	st := newRuntimeManagerTestStore(t)
	ctx := context.Background()

	var wg sync.WaitGroup
	sink := &fakeAPISink{}
	rm := newTestRuntimeManager(t, ctx, st, &wg, sink)

	rm.StopBinding(ctx, 999999) // 从未 Start 过的 ID

	_, removed := sink.snapshot()
	if len(removed) != 0 {
		t.Errorf("从未启动过的绑定不该触发 RemoveRuntime，实际 = %v", removed)
	}
}

// TestHostShutdownCancelsLiveBindingsWithoutLeak 是「宿主关停时仍有
// 绑定在跑」这条路径：不经过 StopBinding（用户没有主动停用任何绑定），
// 而是像 runRun 里 signal.NotifyContext 那样直接取消根 ctx，模拟
// Ctrl+C——两个绑定的连接 goroutine 必须能在有限时间内退出（根 ctx
// 取消要能级联取消全部绑定级子 ctx），不能因为进程还没来得及调用
// StopBinding 就永远挂着。
//
// 这条测试还顺带验证 runRun 里那个 shutdown defer 的等价逻辑：
// wg.Wait() 完成之后，用 rm.liveRoomRuntimes() 取到的引擎调用 closeAll，
// 未决的合并窗口应该被结算（复用 run_test.go 已有的 bindingStub）。
func TestHostShutdownCancelsLiveBindingsWithoutLeak(t *testing.T) {
	st := newRuntimeManagerTestStore(t)
	infoSrv := newFakeBilibiliInfoServer(t)

	rootCtx, cancel := context.WithCancel(context.Background())
	defer cancel()

	bindingA, _ := runtimeManagerTestBinding(t, st, "张三", "小号A", "111", nil)
	bindingB, _ := runtimeManagerTestBinding(t, st, "张三", "小号B", "222", nil)

	var wg sync.WaitGroup
	sink := &fakeAPISink{}
	rm := newTestRuntimeManager(t, rootCtx, st, &wg, sink)
	cfgs, err := st.LoadRunConfig(context.Background())
	if err != nil {
		t.Fatalf("读取配置报错: %v", err)
	}
	for _, c := range cfgs {
		seedFakeAccount(t, rm, c, infoSrv)
	}

	for _, id := range []int64{bindingA, bindingB} {
		if err := rm.StartBinding(rootCtx, id); err != nil {
			t.Fatalf("StartBinding(%d) 报错: %v", id, err)
		}
	}

	lbA, lbB := rm.live[bindingA], rm.live[bindingB]
	waitForNotState(t, 2*time.Second, lbA.client.State, connector.StateIdle)
	waitForNotState(t, 2*time.Second, lbB.client.State, connector.StateIdle)

	// 模拟 Ctrl+C：直接取消根 ctx，不经过 StopBinding。
	cancel()

	done := make(chan struct{})
	go func() { wg.Wait(); close(done) }()
	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("根 ctx 取消后，绑定级 goroutine 没有在 3 秒内退出——" +
			"说明绑定级子 ctx 没有真正级联根 ctx 的取消，是泄漏")
	}

	waitForState(t, time.Second, lbA.client.State, connector.StateClosed)
	waitForState(t, time.Second, lbB.client.State, connector.StateClosed)

	// runRun 的 shutdown defer 等价逻辑：用当前仍登记着的引擎调 closeAll，
	// 结算未决的合并窗口。这里两个绑定都没有配置合并规则，主要断言
	// closeAll 本身不 panic、能正常跑完；合并窗口结算的行为已经由
	// run_test.go 的 TestCloseAllSettlesEngineWindowsBeforeFlushingWriter/
	// TestCloseAllUsesShutdownBudgetForPendingSends 钉死，不在这里重复。
	rts, bots := rm.liveRoomRuntimes()
	if len(rts) != 2 {
		t.Fatalf("liveRoomRuntimes 应该还能看到 2 个绑定，实际 %d 个", len(rts))
	}
	shutdownCtx, cancelShutdown := context.WithTimeout(context.Background(), shutdownSendBudget)
	defer cancelShutdown()
	engines := make([]*rules.Engine, len(rts))
	for i, rt := range rts {
		engines[i] = rt.Engine()
	}
	closeAll(engines, bots, shutdownCtx, rm.activity) // 不该 panic
}

// ---- 审查回合修复的回归测试 ----
//
// 下面两条测试专门覆盖审查指出的"现有 6 条测试全部用 context.Background()
// 调 StartBinding，永不取消，前提不成立"这个假绿形态：它们都会传一个
// 会被主动取消的 ctx，复现"HTTP 请求结束后 r.Context() 被 net/http
// cancel 掉"这个真实场景，而不是像上面的测试那样让 ctx 从头到尾存活。

// TestStartBindingSendDanmakuOutlivesCallerContext 是 Critical-1
// （运行期启动的绑定连得上、收得到事件、一条弹幕也发不出去）的回归测试。
//
// 复现链路：net/http 在 ServeHTTP 返回时会 cancel 掉 r.Context()——
// StartBinding(r.Context(), bindingID) 用的正是这个 ctx。若 adoptLocked
// 没有把 roomBot 的发送 ctx 收敛到绑定级的 bindCtx（派生自 rm.ctx，
// 生命周期独立于任何一次 HTTP 请求），请求一结束，bot 的发送 ctx 就是
// 一个已取消的 ctx：account.Binding.SendDanmaku 第一步
// b.Account.Limiter.Wait(ctx) 立刻返回 context.Canceled——绑定连得上、
// 收得到事件，但一条弹幕也发不出去，且连接 goroutine（用从 rm.ctx 派生
// 的 bindCtx）完全不受影响，日志照常打「已配置绑定」「已连接直播间」，
// 从日志看不出任何异常。
//
// 变异自检：把 adoptLocked 里 `asm.bot.setCtx(bindCtx)` 这一行删掉，
// 这条测试必须由绿转红——且必须复现出与审查者手工测试完全一致的
// context.Canceled。
func TestStartBindingSendDanmakuOutlivesCallerContext(t *testing.T) {
	st := newRuntimeManagerTestStore(t)
	infoSrv := newFakeBilibiliInfoServer(t)
	rootCtx := context.Background()

	bindingID, _ := runtimeManagerTestBinding(t, st, "张三", "小号", "123", nil)

	var wg sync.WaitGroup
	sink := &fakeAPISink{}
	rm := newTestRuntimeManager(t, rootCtx, st, &wg, sink)
	cfgs, err := st.LoadRunConfig(rootCtx)
	if err != nil {
		t.Fatalf("读取配置报错: %v", err)
	}
	seedFakeAccount(t, rm, cfgs[0], infoSrv)

	// 模拟一次 HTTP 请求：handler 用请求的 ctx 调 StartBinding，
	// ServeHTTP 一返回，net/http 就会 cancel 掉这个 ctx——这里手动模拟
	// 那一刻。
	reqCtx, cancelReq := context.WithCancel(context.Background())
	if err := rm.StartBinding(reqCtx, bindingID); err != nil {
		t.Fatalf("StartBinding 报错: %v", err)
	}
	t.Cleanup(func() { rm.StopBinding(rootCtx, bindingID) })
	cancelReq() // 模拟 ServeHTTP 返回、请求 ctx 被取消

	lb, ok := rm.live[bindingID]
	if !ok {
		t.Fatal("StartBinding 之后 rm.live 里应该有这个绑定")
	}

	// 直接调用 roomBot.SendDanmaku——这正是规则引擎触发弹幕动作时的路径
	// （rules.BotAPI.SendDanmaku(text)，内部用 *b.ctx.Load()）。第一步是
	// 账号级限流器的 Wait(ctx)：这个账号的限流器是全新的（本测试第一次
	// 用它），Wait 会立即返回 ctx.Err()，不需要真的等待，也不会触碰网络——
	// 若这里已经是 context.Canceled，后面根本不会走到发 HTTP 请求这步。
	if err := lb.bot.SendDanmaku("测试弹幕"); errors.Is(err, context.Canceled) {
		t.Fatalf("SendDanmaku 在 HTTP 请求结束后返回 context.Canceled：%v\n"+
			"说明 roomBot 的发送 ctx 仍然是调用方传入的 r.Context()，而不是"+
			"绑定级的 bindCtx——绑定连得上、收得到事件，但一条弹幕也发不出去", err)
	}
}

// TestStartBindingRebuildsAccountRuntimeWhenCookieChanges 是 Important-1
// （账号重新扫码后，运行时缓存永不失效）的回归测试。
//
// 复现完整故障场景：账号已经在跑 → 用户重新扫码续命，Cookie 换了
// （saveScannedAccount 只 UpdateAccountCookie 写库，不通知任何运行时）
// → 用户按 P5-1 教的手势把绑定停用再启用 → StartBinding 若命中
// rm.accounts 缓存且不比对 Cookie，就会把绑定接回装配那一刻的旧会话。
//
// 用一个注入的假 buildAccount（指向假服务器，不打真实网络）验证：
// Cookie 变了之后，ensureAccountRuntime 必须重建 accountRuntime（新
// Session 反映新 Cookie），同时不能把共享限流器换掉。
//
// 变异自检：把 ensureAccountRuntime 里的 Cookie 比对删掉（直接复用旧
// 缓存），这条测试必须由绿转红。
func TestStartBindingRebuildsAccountRuntimeWhenCookieChanges(t *testing.T) {
	st := newRuntimeManagerTestStore(t)
	infoSrv := newFakeBilibiliInfoServer(t)
	ctx := context.Background()

	bindingID, _ := runtimeManagerTestBinding(t, st, "张三", "小号", "123", nil)

	var wg sync.WaitGroup
	rm := newTestRuntimeManager(t, ctx, st, &wg, nil)
	cfgs, err := st.LoadRunConfig(ctx)
	if err != nil {
		t.Fatalf("读取配置报错: %v", err)
	}
	seedFakeAccount(t, rm, cfgs[0], infoSrv)
	oldAcctRT := rm.accounts[cfgs[0].AccountName]
	oldLimiter := oldAcctRT.acc.Limiter

	// 先真正启动一次绑定，再停用——完整复现"账号已经在跑"这个前提，
	// 而不是只摆弄 rm.accounts 这个内部缓存。
	if err := rm.StartBinding(ctx, bindingID); err != nil {
		t.Fatalf("StartBinding（首次）报错: %v", err)
	}
	waitForNotState(t, 2*time.Second, rm.live[bindingID].client.State, connector.StateIdle)
	rm.StopBinding(ctx, bindingID)

	// 注入一个不打真实网络的假装配函数：生产环境里 ensureAccountRuntime
	// 重建账号运行时会经过真实的 buildAccountRuntime（内含一次真实的
	// RefreshNav 网络请求），单元测试没有理由为了验证"该不该重建"这个
	// 决策逻辑本身而去打真实的 B 站接口——与 seedFakeAccount 不能直接
	// 复用 buildAccountRuntime 是同一个理由。
	rm.buildAccount = func(_ context.Context, c store.RunConfig) (*accountRuntime, error) {
		sess, err := auth.ParseSession(c.Cookie)
		if err != nil {
			return nil, err
		}
		apiClient := api.New(sess, api.WithHTTPClient(infoSrv.Client()))
		apiClient.SetBaseURL("nav", infoSrv.URL)
		apiClient.SetBaseURL("roomInfo", infoSrv.URL)
		apiClient.SetBaseURL("danmuInfo", "http://127.0.0.1:1")
		apiClient.SetBaseURL("sendMsg", infoSrv.URL)
		interval := c.RateLimit
		if interval <= 0 {
			interval = defaultRateLimit
		}
		return &accountRuntime{
			acc:    account.New(c.AccountName, sess, interval),
			api:    apiClient,
			cookie: c.Cookie,
		}, nil
	}

	// 模拟用户重新扫码续命：Cookie 换了，UID 也变了。
	const newUID = "999888777"
	newCookie := "SESSDATA=new-session-data; bili_jct=new-csrf; DedeUserID=" + newUID
	if err := st.UpdateAccountCookie(ctx, cfgs[0].AccountName, newCookie, newUID); err != nil {
		t.Fatalf("更新 Cookie 报错: %v", err)
	}

	// 用户手势：重新启用（P5-1 里唯一像样的"重启"手势）。
	if err := rm.StartBinding(ctx, bindingID); err != nil {
		t.Fatalf("StartBinding（重新启用）报错: %v", err)
	}
	t.Cleanup(func() { rm.StopBinding(ctx, bindingID) })

	got, ok := rm.accounts[cfgs[0].AccountName]
	if !ok {
		t.Fatal("账号运行时缓存应该还在")
	}
	if got.acc.Session.UID != newUID {
		t.Errorf("Cookie 更新后账号运行时的 Session.UID = %q, 期望 %q——"+
			"说明运行时缓存没有跟着新 Cookie 重建，机器人还在用重新扫码之前的死会话",
			got.acc.Session.UID, newUID)
	}
	if got.cookie != newCookie {
		t.Errorf("accountRuntime.cookie = %q, 期望更新为新 Cookie %q", got.cookie, newCookie)
	}
	if got.acc.Limiter != oldLimiter {
		t.Error("重建账号运行时后限流器不是原来那个实例——" +
			"限流是按账号累计节奏算的，重建不该把跨绑定共享的这一份换掉")
	}
}
