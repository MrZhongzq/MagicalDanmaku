package main

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/MrZhongzq/MagicalDanmaku/server/internal/connector/bilibili/api"
	"github.com/MrZhongzq/MagicalDanmaku/server/internal/store"
)

// fakeLiveStatusNotifier 记录被通知过的 (bindingID, state) 组合，验证
// 探测完成（心跳循环或立即检测）之后有没有把结果同步给运行时的事件
// 分发循环——不依赖真正的 runtimeManager（那需要账号运行时、调度器等
// 一整套装配），理由与本文件其余假实现一致。
type fakeLiveStatusNotifier struct {
	mu       sync.Mutex
	notified []liveStatusNotification
}

type liveStatusNotification struct {
	bindingID int64
	state     string
}

func (f *fakeLiveStatusNotifier) UpdateLiveStatus(bindingID int64, state string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.notified = append(f.notified, liveStatusNotification{bindingID, state})
}

func (f *fakeLiveStatusNotifier) snapshot() []liveStatusNotification {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]liveStatusNotification{}, f.notified...)
}

// ---- resolveRoomState：探测结果 -> 三态的映射（P5-2 任务 1b） ----

// TestResolveRoomStateOnProbeFailureIsUnknown 是本任务最核心的一条测试：
// 探测本身失败（网络错误、超时、风控）必须映射成 unknown，绝不能是
// offline——那是把"拿不到"伪装成"确认没开播"，会让用户看着一个像是
// 正常结果的错误答案。
func TestResolveRoomStateOnProbeFailureIsUnknown(t *testing.T) {
	state, uid, name := resolveRoomState(nil, errors.New("网络错误：连接超时"))
	if state != store.RoomLiveUnknown {
		t.Errorf("state = %q, 期望 %q", state, store.RoomLiveUnknown)
	}
	if uid != "" || name != "" {
		t.Errorf("探测失败不该带出任何主播身份，实际 uid=%q name=%q", uid, name)
	}
}

func TestResolveRoomStateLivingWhenAPISaysLiving(t *testing.T) {
	status := &api.RoomStatus{LiveStatus: api.LiveStatusLiving, AnchorUID: "1", AnchorName: "主播"}
	state, uid, name := resolveRoomState(status, nil)
	if state != store.RoomLiveLiving {
		t.Errorf("state = %q, 期望 %q", state, store.RoomLiveLiving)
	}
	if uid != "1" || name != "主播" {
		t.Errorf("主播身份应原样带出，实际 uid=%q name=%q", uid, name)
	}
}

// TestResolveRoomStateOfflineWhenAPISaysNotLiving 覆盖未开播（0）与
// 轮播中（2）两种"不算开播"的取值，两者都该映射成 offline——与
// api.RoomInfo.IsLiving 的既有语义保持一致。
func TestResolveRoomStateOfflineWhenAPISaysNotLiving(t *testing.T) {
	for _, liveStatus := range []int{api.LiveStatusOffline, api.LiveStatusRound} {
		status := &api.RoomStatus{LiveStatus: liveStatus}
		state, _, _ := resolveRoomState(status, nil)
		if state != store.RoomLiveOffline {
			t.Errorf("live_status=%d 时 state = %q, 期望 %q", liveStatus, state, store.RoomLiveOffline)
		}
	}
}

// ---- roomStatusCheckOnce/roomStatusCheckLoop 编排逻辑 ----

// newRoomStatusCheckTestStore 建一个独立 schema 的真实存储，供本文件
// 的编排逻辑测试使用——手法与 run_test.go 里 newLoginCheckTestStore
// 完全一致，只是换一个 schema 名，避免两组测试的 DROP SCHEMA 互相打架。
func newRoomStatusCheckTestStore(t *testing.T) *store.Store {
	t.Helper()
	dsn := envOrSkip(t)

	const schema = "m_magicd_room_status_check_test"
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

// envOrSkip 取测试数据库连接串，未设置则跳过——与
// run_test.go/internal/store/testhelp_test.go 的判断一致。
func envOrSkip(t *testing.T) string {
	t.Helper()
	dsn := os.Getenv("MAGICD_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("未设置 MAGICD_TEST_DATABASE_URL，跳过需要真实数据库的测试。\n" +
			"本地起库：docker compose -f docker-compose.dev.yml up -d\n" +
			"然后：export MAGICD_TEST_DATABASE_URL='postgres://magicd:magicd@localhost:5433/magicd?sslmode=disable'")
	}
	return dsn
}

func mustRoomStatusAccountAndBinding(t *testing.T, st *store.Store, ownerID int64, accName, roomID string) *store.Binding {
	t.Helper()
	ctx := context.Background()
	acc, err := st.CreateAccount(ctx, store.AccountInput{Name: accName, Cookie: "SESSDATA=" + accName, OwnerID: ownerID})
	if err != nil {
		t.Fatalf("建账号报错: %v", err)
	}
	b, err := st.UpsertBinding(ctx, acc.ID, roomID)
	if err != nil {
		t.Fatalf("建绑定报错: %v", err)
	}
	return b
}

// TestRoomStatusCheckOnceWritesCheckerResults 验证 roomStatusCheckOnce
// 把 roomStatusChecker 的判定结果原样落到对应绑定身上，不会张冠李戴，
// 且主播 UID/昵称一并写入。
func TestRoomStatusCheckOnceWritesCheckerResults(t *testing.T) {
	st := newRoomStatusCheckTestStore(t)
	ctx := context.Background()

	owner, err := st.CreateUser(ctx, "张三", "密码123456", false)
	if err != nil {
		t.Fatalf("建用户报错: %v", err)
	}
	living := mustRoomStatusAccountAndBinding(t, st, owner.ID, "开播号", "111")
	offline := mustRoomStatusAccountAndBinding(t, st, owner.ID, "下播号", "222")

	check := func(_ context.Context, _ string, roomID string) (*api.RoomStatus, error) {
		if roomID == "111" {
			return &api.RoomStatus{LiveStatus: api.LiveStatusLiving, AnchorUID: "9001", AnchorName: "开播主播"}, nil
		}
		return &api.RoomStatus{LiveStatus: api.LiveStatusOffline, AnchorUID: "9002", AnchorName: "下播主播"}, nil
	}

	roomStatusCheckOnce(ctx, st, check, nil, slog.Default())

	got1, err := st.GetBindingByID(ctx, living.ID)
	if err != nil {
		t.Fatalf("查询报错: %v", err)
	}
	if got1.LiveStatus != store.RoomLiveLiving {
		t.Errorf("开播号 LiveStatus = %q, 期望 %q", got1.LiveStatus, store.RoomLiveLiving)
	}
	if got1.AnchorUID != "9001" || got1.AnchorName != "开播主播" {
		t.Errorf("开播号主播身份 = uid=%q name=%q", got1.AnchorUID, got1.AnchorName)
	}
	if got1.LiveCheckedAt == nil {
		t.Error("检测完成后应记录 LiveCheckedAt")
	}

	got2, err := st.GetBindingByID(ctx, offline.ID)
	if err != nil {
		t.Fatalf("查询报错: %v", err)
	}
	if got2.LiveStatus != store.RoomLiveOffline {
		t.Errorf("下播号 LiveStatus = %q, 期望 %q", got2.LiveStatus, store.RoomLiveOffline)
	}
}

// TestRoomStatusCheckOnceProbeFailureIsUnknownNotOffline 是本任务的自检项
// (a)：探测失败必须写成 unknown，不能写成 offline——否则用户会看到一个
// 看起来正常、实则彻底错误的"未开播"结论。同时验证一个绑定探测失败
// 不会阻塞其余绑定被正常检测到。
func TestRoomStatusCheckOnceProbeFailureIsUnknownNotOffline(t *testing.T) {
	st := newRoomStatusCheckTestStore(t)
	ctx := context.Background()

	owner, err := st.CreateUser(ctx, "张三", "密码123456", false)
	if err != nil {
		t.Fatalf("建用户报错: %v", err)
	}
	bad := mustRoomStatusAccountAndBinding(t, st, owner.ID, "坏号", "111")
	good := mustRoomStatusAccountAndBinding(t, st, owner.ID, "好号", "222")

	check := func(_ context.Context, _ string, roomID string) (*api.RoomStatus, error) {
		if roomID == "111" {
			return nil, errors.New("api: 风控校验失败")
		}
		return &api.RoomStatus{LiveStatus: api.LiveStatusLiving}, nil
	}

	roomStatusCheckOnce(ctx, st, check, nil, slog.Default())

	gotBad, err := st.GetBindingByID(ctx, bad.ID)
	if err != nil {
		t.Fatalf("查询报错: %v", err)
	}
	if gotBad.LiveStatus == store.RoomLiveOffline {
		t.Error("探测失败的绑定被记成了「未开播」——这是把拿不到伪装成没开播")
	}
	if gotBad.LiveStatus != store.RoomLiveUnknown {
		t.Errorf("坏号 LiveStatus = %q, 期望 %q", gotBad.LiveStatus, store.RoomLiveUnknown)
	}

	gotGood, err := st.GetBindingByID(ctx, good.ID)
	if err != nil {
		t.Fatalf("查询报错: %v", err)
	}
	if gotGood.LiveStatus != store.RoomLiveLiving {
		t.Errorf("坏号探测失败不该影响好号：好号 LiveStatus = %q, 期望 %q（说明循环提前退出了）",
			gotGood.LiveStatus, store.RoomLiveLiving)
	}
}

// TestRoomStatusCheckOnceReturnsEarlyWhenContextCancelled 与
// TestLoginCheckOnceReturnsEarlyWhenContextCancelled 是同一类测试：
// 关停期间不该对剩下的绑定继续做注定失败的探测。
func TestRoomStatusCheckOnceReturnsEarlyWhenContextCancelled(t *testing.T) {
	st := newRoomStatusCheckTestStore(t)
	ctx := context.Background()

	owner, err := st.CreateUser(ctx, "张三", "密码123456", false)
	if err != nil {
		t.Fatalf("建用户报错: %v", err)
	}
	mustRoomStatusAccountAndBinding(t, st, owner.ID, "账号甲", "111")
	mustRoomStatusAccountAndBinding(t, st, owner.ID, "账号乙", "222")

	runCtx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var calls int32
	check := func(context.Context, string, string) (*api.RoomStatus, error) {
		if atomic.AddInt32(&calls, 1) == 1 {
			cancel()
		}
		return &api.RoomStatus{LiveStatus: api.LiveStatusLiving}, nil
	}

	roomStatusCheckOnce(runCtx, st, check, nil, slog.Default())

	if got := atomic.LoadInt32(&calls); got != 1 {
		t.Errorf("check 被调用了 %d 次，期望恰好 1 次", got)
	}
}

// TestRoomStatusCheckLoopRunsImmediatelyAndRespectsCancellation 验证
// roomStatusCheckLoop 与 loginCheckLoop/purgeLoop 同样的两条约束：启动
// 时立刻检测一次（不等第一个 60 秒的 tick），以及 ctx 取消后能干净退出。
func TestRoomStatusCheckLoopRunsImmediatelyAndRespectsCancellation(t *testing.T) {
	st := newRoomStatusCheckTestStore(t)
	ctx := context.Background()

	owner, err := st.CreateUser(ctx, "张三", "密码123456", false)
	if err != nil {
		t.Fatalf("建用户报错: %v", err)
	}
	mustRoomStatusAccountAndBinding(t, st, owner.ID, "小号", "111")

	var calls int32
	check := func(context.Context, string, string) (*api.RoomStatus, error) {
		atomic.AddInt32(&calls, 1)
		return &api.RoomStatus{LiveStatus: api.LiveStatusLiving}, nil
	}

	runCtx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		roomStatusCheckLoop(runCtx, st, check, nil, slog.Default())
		close(done)
	}()

	deadline := time.Now().Add(3 * time.Second)
	for atomic.LoadInt32(&calls) == 0 {
		if time.Now().After(deadline) {
			t.Fatal("启动时应立刻做一次检测，而不是等第一个 60 秒的 tick 才开始")
		}
		time.Sleep(10 * time.Millisecond)
	}

	cancel()
	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("ctx 取消后 roomStatusCheckLoop 应尽快退出，而不是继续等下一个 tick")
	}
}

// ---- P6 任务 4：探测完成后要通知运行时的事件分发循环 ----
//
// 光把状态写进数据库不够：未开播时不该继续处理高能榜/进房事件这件事
// 发生在事件分发的热路径上（cmd/magicd/runtime_manager.go 的 adoptLocked
// 循环），不会每条事件都去查一次数据库，得有一个内存态的"最近一次探测
// 结果"同步过去。roomStatusCheckOnce/roomStatusCheckLoop 是这份内存态
// 唯二两个写入来源之一（另一个是 bindingRoomStatusProbe.ProbeNow 立即
// 探测），必须真的调用 notify。

// TestRoomStatusCheckOnceNotifiesLiveStatusOfEachBinding 验证心跳循环把
// 每个绑定的探测结果都同步给了 notify，且 bindingID/state 与写库的结果
// 一一对应，不会张冠李戴。
func TestRoomStatusCheckOnceNotifiesLiveStatusOfEachBinding(t *testing.T) {
	st := newRoomStatusCheckTestStore(t)
	ctx := context.Background()

	owner, err := st.CreateUser(ctx, "张三", "密码123456", false)
	if err != nil {
		t.Fatalf("建用户报错: %v", err)
	}
	living := mustRoomStatusAccountAndBinding(t, st, owner.ID, "开播号", "111")
	offline := mustRoomStatusAccountAndBinding(t, st, owner.ID, "下播号", "222")

	check := func(_ context.Context, _ string, roomID string) (*api.RoomStatus, error) {
		if roomID == "111" {
			return &api.RoomStatus{LiveStatus: api.LiveStatusLiving}, nil
		}
		return &api.RoomStatus{LiveStatus: api.LiveStatusOffline}, nil
	}

	notify := &fakeLiveStatusNotifier{}
	roomStatusCheckOnce(ctx, st, check, notify, slog.Default())

	got := map[int64]string{}
	for _, n := range notify.snapshot() {
		got[n.bindingID] = n.state
	}
	if got[living.ID] != store.RoomLiveLiving {
		t.Errorf("开播号的通知状态 = %q, 期望 %q", got[living.ID], store.RoomLiveLiving)
	}
	if got[offline.ID] != store.RoomLiveOffline {
		t.Errorf("下播号的通知状态 = %q, 期望 %q", got[offline.ID], store.RoomLiveOffline)
	}
}

// TestRoomStatusCheckOnceNotifiesUnknownOnProbeFailure 是自检项 (a) 在
// 通知路径上的落点：探测失败必须通知 unknown，不能通知 offline——
// 不然写库那边是对的（unknown），内存态那边却错误地把事件分发循环
// 掐死了，两边不一致，而错误的那一边正是真正影响机器人行为的那一边。
func TestRoomStatusCheckOnceNotifiesUnknownOnProbeFailure(t *testing.T) {
	st := newRoomStatusCheckTestStore(t)
	ctx := context.Background()

	owner, err := st.CreateUser(ctx, "张三", "密码123456", false)
	if err != nil {
		t.Fatalf("建用户报错: %v", err)
	}
	bad := mustRoomStatusAccountAndBinding(t, st, owner.ID, "坏号", "111")

	check := func(context.Context, string, string) (*api.RoomStatus, error) {
		return nil, errors.New("api: 风控校验失败")
	}

	notify := &fakeLiveStatusNotifier{}
	roomStatusCheckOnce(ctx, st, check, notify, slog.Default())

	got := notify.snapshot()
	if len(got) != 1 || got[0].bindingID != bad.ID {
		t.Fatalf("通知记录 = %v, 期望恰好一条针对绑定 %d 的通知", got, bad.ID)
	}
	if got[0].state == store.RoomLiveOffline {
		t.Error("探测失败被通知成了「未开播」——这会让一次网络抖动掐掉高能榜/进房事件")
	}
	if got[0].state != store.RoomLiveUnknown {
		t.Errorf("通知状态 = %q, 期望 %q", got[0].state, store.RoomLiveUnknown)
	}
}
