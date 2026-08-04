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
	"sync/atomic"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/MrZhongzq/MagicalDanmaku/server/internal/account"
	"github.com/MrZhongzq/MagicalDanmaku/server/internal/connector"
	"github.com/MrZhongzq/MagicalDanmaku/server/internal/connector/bilibili/api"
	"github.com/MrZhongzq/MagicalDanmaku/server/internal/connector/bilibili/auth"
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
	mu         sync.Mutex
	sent       []string
	blocks     []blockRecord
	blacklists []string
	err        error
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

func (b *bindingStub) Blacklist(ctx context.Context, uid string) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.err != nil {
		return b.err
	}
	b.blacklists = append(b.blacklists, uid)
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

// TestRoomBotForwardsBlacklistToBinding 钉住「拉黑走独立路径」——
// roomBot.Blacklist 转发给 binding.Blacklist，不途经 Block。
func TestRoomBotForwardsBlacklistToBinding(t *testing.T) {
	bs := &bindingStub{}
	b := newRoomBot(bs, context.Background())

	if err := b.Blacklist("999"); err != nil {
		t.Fatalf("Blacklist 失败: %v", err)
	}
	if len(bs.blacklists) != 1 || bs.blacklists[0] != "999" {
		t.Errorf("blacklists = %v", bs.blacklists)
	}
	if len(bs.blocks) != 0 {
		t.Errorf("blocks = %v，拉黑不该顺带触发禁言", bs.blocks)
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
	if err := b.Blacklist("1"); err == nil {
		t.Error("底层错误应当上报")
	}
}

// fakeAccountActions 是 connector.Actions 的测试替身，供构造
// account.Binding 时注入——roomRuntime.Blacklist/Unblacklist/
// BlacklistStatus/Nickname 直接调用 rt.binding（*account.Binding），
// 不经过 roomBot，所以不能复用 bindingStub（那是 danmakuSender 的替身）。
type fakeAccountActions struct {
	mu           sync.Mutex
	blacklists   []string
	unblacklists []string
	attribute    int
	nickname     string
	err          error
	nicknameErr  error
}

func (f *fakeAccountActions) SendDanmaku(context.Context, connector.SendDanmakuRequest) error {
	return nil
}
func (f *fakeAccountActions) BlockUser(context.Context, connector.BlockRequest) error { return nil }
func (f *fakeAccountActions) UnblockUser(context.Context, string, string) error       { return nil }

func (f *fakeAccountActions) BlacklistUser(_ context.Context, uid string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.err != nil {
		return f.err
	}
	f.blacklists = append(f.blacklists, uid)
	return nil
}

func (f *fakeAccountActions) UnblacklistUser(_ context.Context, uid string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.err != nil {
		return f.err
	}
	f.unblacklists = append(f.unblacklists, uid)
	return nil
}

func (f *fakeAccountActions) RelationAttribute(context.Context, string) (int, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.attribute, f.err
}

func (f *fakeAccountActions) Nickname(context.Context, string) (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.nicknameErr != nil {
		return "", f.nicknameErr
	}
	return f.nickname, nil
}

// newTestRoomRuntimeForBlacklist 建一个只够 Blacklist/Unblacklist/
// BlacklistStatus/Nickname 用的最小 roomRuntime——这几个方法不碰
// st/sched/storage/engine，不需要数据库，比 newReloadTestStore 轻得多。
//
// 返回的 *logging.ActivityWriter 由调用方自己决定何时 Close()：
// 业务日志是异步落库的，测试要断言"确实记了几条"就必须先 Close()
// 把缓冲排空、等后台协程真正写完，不能在 Enqueue 之后立刻读——
// 那是"绕开真实时序"的假绿。
func newTestRoomRuntimeForBlacklist(t *testing.T, actions *fakeAccountActions, flush func(context.Context, []store.ActivityRow) error) (*roomRuntime, *logging.ActivityWriter) {
	t.Helper()
	activity := logging.NewActivityWriter(logging.ActivityWriterOptions{
		Flush:     flush,
		BatchSize: 1,
		Interval:  time.Hour,
	})

	binding := &account.Binding{
		Account: account.New("小号", nil, 0),
		RoomID:  "123",
		Actions: actions,
	}
	rt := &roomRuntime{
		binding: binding,
		sink:    activity.Sink(1, 1, "123"),
		label:   "小号@123",
		roomID:  "123",
		log:     slog.Default(),
	}
	return rt, activity
}

// TestRoomRuntimeBlacklistRecordsActivity 钉住「拉黑无论成败都要落业务
// 日志」——这是用户明确要求的（把操作失败写日志，是 recordManual 早就
// 定下的约定，拉黑不能是例外）。
func TestRoomRuntimeBlacklistRecordsActivity(t *testing.T) {
	var mu sync.Mutex
	var rows []store.ActivityRow
	collect := func(_ context.Context, batch []store.ActivityRow) error {
		mu.Lock()
		rows = append(rows, batch...)
		mu.Unlock()
		return nil
	}

	actions := &fakeAccountActions{}
	rt, activity := newTestRoomRuntimeForBlacklist(t, actions, collect)

	if err := rt.Blacklist(context.Background(), "10086"); err != nil {
		t.Fatalf("Blacklist 失败: %v", err)
	}
	if len(actions.blacklists) != 1 || actions.blacklists[0] != "10086" {
		t.Errorf("blacklists = %v", actions.blacklists)
	}

	// 让失败的那次也走一遍，两条都要落日志
	actions.err = errors.New("B 站返回风控")
	if err := rt.Blacklist(context.Background(), "10087"); err == nil {
		t.Fatal("失败的拉黑应当把错误传出去")
	}

	// 业务日志异步落库：必须先 Close() 排空缓冲、等后台协程真正写完，
	// 才能确定性地断言写了几条——不能在 Enqueue 之后立刻读 rows。
	activity.Close()

	mu.Lock()
	got := append([]store.ActivityRow{}, rows...)
	mu.Unlock()

	if len(got) != 2 {
		t.Fatalf("业务日志条数 = %d, 期望 2（成功一条失败一条）", len(got))
	}
	for i, r := range got {
		if r.ActionType != string(rules.ActionBlacklist) {
			t.Errorf("第 %d 条 ActionType = %q, 期望 %q", i+1, r.ActionType, rules.ActionBlacklist)
		}
	}
	if got[1].Detail == nil {
		t.Error("失败那条也应当带上事件详情")
	}
}

func TestRoomRuntimeUnblacklistForwards(t *testing.T) {
	actions := &fakeAccountActions{}
	rt, activity := newTestRoomRuntimeForBlacklist(t, actions, func(context.Context, []store.ActivityRow) error { return nil })
	t.Cleanup(activity.Close)

	if err := rt.Unblacklist(context.Background(), "10086"); err != nil {
		t.Fatalf("Unblacklist 失败: %v", err)
	}
	if len(actions.unblacklists) != 1 || actions.unblacklists[0] != "10086" {
		t.Errorf("unblacklists = %v", actions.unblacklists)
	}
}

// TestRoomRuntimeBlacklistAndUnblacklistLogDifferentActionTypes 钉住协调者
// 2026-08-04 的裁决：拉黑与取消拉黑在 activity_logs 里必须能区分方向——
// 两者都是「无论成败都要落日志」的不可逆/半不可逆对外操作，如果两条相反
// 的记录长得一模一样（此前都记 rules.ActionBlacklist），这份日志作为事后
// 审计依据就作废了。
//
// 只断言 ActionType 字段本身，不去动 detail 的 JSON 结构——协调者要求
// 「不要改变其它动作类型的现有日志形态」，这条测试如果被改成去检查
// detail 里多了什么字段，反而会引诱实现往 sink.go 的公共路径里加东西。
func TestRoomRuntimeBlacklistAndUnblacklistLogDifferentActionTypes(t *testing.T) {
	var mu sync.Mutex
	var rows []store.ActivityRow
	collect := func(_ context.Context, batch []store.ActivityRow) error {
		mu.Lock()
		rows = append(rows, batch...)
		mu.Unlock()
		return nil
	}

	actions := &fakeAccountActions{}
	rt, activity := newTestRoomRuntimeForBlacklist(t, actions, collect)

	if err := rt.Blacklist(context.Background(), "10086"); err != nil {
		t.Fatalf("Blacklist 失败: %v", err)
	}
	if err := rt.Unblacklist(context.Background(), "10086"); err != nil {
		t.Fatalf("Unblacklist 失败: %v", err)
	}

	// 业务日志异步落库：必须先 Close() 排空缓冲，见上面同类测试的注释。
	activity.Close()

	mu.Lock()
	got := append([]store.ActivityRow{}, rows...)
	mu.Unlock()

	if len(got) != 2 {
		t.Fatalf("业务日志条数 = %d, 期望 2（拉黑一条、取消拉黑一条）", len(got))
	}
	if got[0].ActionType != string(rules.ActionBlacklist) {
		t.Errorf("第 1 条（拉黑）ActionType = %q, 期望 %q", got[0].ActionType, rules.ActionBlacklist)
	}
	if got[1].ActionType != string(rules.ActionUnblacklist) {
		t.Errorf("第 2 条（取消拉黑）ActionType = %q, 期望 %q", got[1].ActionType, rules.ActionUnblacklist)
	}
	if got[0].ActionType == got[1].ActionType {
		t.Errorf("拉黑与取消拉黑的 ActionType 不该相同，实际都是 %q——日志分不清方向", got[0].ActionType)
	}
}

// TestRoomRuntimeBlacklistStatusReportsBlacklisted 钉住 attribute==128
// 的判据经过完整链路（api.IsBlacklisted）后仍然正确——这是「自检变异
// (c)」在 cmd/magicd 这一层的对应防线。
func TestRoomRuntimeBlacklistStatusReportsBlacklisted(t *testing.T) {
	actions := &fakeAccountActions{attribute: 128, nickname: "测试昵称"}
	rt, activity := newTestRoomRuntimeForBlacklist(t, actions, func(context.Context, []store.ActivityRow) error { return nil })
	t.Cleanup(activity.Close)

	blacklisted, name, err := rt.BlacklistStatus(context.Background(), "10086")
	if err != nil {
		t.Fatalf("BlacklistStatus 失败: %v", err)
	}
	if !blacklisted {
		t.Error("attribute=128 应判定为已拉黑")
	}
	if name != "测试昵称" {
		t.Errorf("nickname = %q", name)
	}
}

func TestRoomRuntimeBlacklistStatusReportsNotBlacklisted(t *testing.T) {
	actions := &fakeAccountActions{attribute: 0}
	rt, activity := newTestRoomRuntimeForBlacklist(t, actions, func(context.Context, []store.ActivityRow) error { return nil })
	t.Cleanup(activity.Close)

	blacklisted, _, err := rt.BlacklistStatus(context.Background(), "10086")
	if err != nil {
		t.Fatalf("BlacklistStatus 失败: %v", err)
	}
	if blacklisted {
		t.Error("attribute=0 不应判定为已拉黑")
	}
}

// TestRoomRuntimeBlacklistStatusToleratesNicknameFailure 昵称查询失败
// 不该拖累状态回读本身——两者失败模式独立，拉黑状态是主流程，昵称只是
// 锦上添花的自动回填。
func TestRoomRuntimeBlacklistStatusToleratesNicknameFailure(t *testing.T) {
	actions := &fakeAccountActions{attribute: 128, nicknameErr: errors.New("查询昵称失败")}
	rt, activity := newTestRoomRuntimeForBlacklist(t, actions, func(context.Context, []store.ActivityRow) error { return nil })
	t.Cleanup(activity.Close)

	blacklisted, name, err := rt.BlacklistStatus(context.Background(), "10086")
	if err != nil {
		t.Fatalf("BlacklistStatus 不该因为昵称查询失败而报错: %v", err)
	}
	if !blacklisted {
		t.Error("attribute=128 应判定为已拉黑")
	}
	if name != "" {
		t.Errorf("name = %q, 期望空串（昵称查询失败时留空）", name)
	}
}

// ---- 全批次终审项【2】：单个绑定规则非法不该让整个守护进程起不来 ----

// newAssembleTestAccount 建一个不会真的联网的 accountRuntime：api.Client
// 指向本地 httptest 服务器，只用于 RoomInfo（不需要 wbi 签名，room.go 里
// GetJSON 的 sign 参数是 false），所以不必先调 RefreshNav。
func newAssembleTestAccount(t *testing.T, name string, srv *httptest.Server) *accountRuntime {
	t.Helper()
	sess, err := auth.ParseSession("SESSDATA=" + name + "; bili_jct=y; DedeUserID=1")
	if err != nil {
		t.Fatalf("ParseSession 失败: %v", err)
	}
	c := api.New(sess, api.WithHTTPClient(srv.Client()))
	c.SetBaseURL("roomInfo", srv.URL)
	return &accountRuntime{acc: account.New(name, sess, time.Second), api: c}
}

// TestBuildRoomRuntimeSkipsBindingWithInvalidRules 验证 buildRoomRuntime
// 在规则非法（这里用 Suppress 指向不存在的规则名模拟）时返回 error，且不
// 返回任何部分构造好的资源——调用方据此判断"跳过这个绑定"而不是
// "带着半成品资源继续跑"。
//
// 不需要真实网络：buildRoomRuntime 本身不发任何请求，直播间信息由
// 调用方直接构造传入。
func TestBuildRoomRuntimeSkipsBindingWithInvalidRules(t *testing.T) {
	sess, err := auth.ParseSession("SESSDATA=x; bili_jct=y; DedeUserID=1")
	if err != nil {
		t.Fatalf("ParseSession 失败: %v", err)
	}
	acctRT := &accountRuntime{acc: account.New("小号", sess, time.Second), api: api.New(sess)}
	info := &api.RoomInfo{RoomID: "123", UID: "1", Title: "标题"}

	activity := logging.NewActivityWriter(logging.ActivityWriterOptions{
		Flush: func(context.Context, []store.ActivityRow) error { return nil },
	})
	t.Cleanup(activity.Close)

	cfg := store.RunConfig{
		AccountName: "小号", BindingID: 1, AccountID: 1, RoomID: "123",
		Rules: []rules.Rule{{
			Name:    "只欢迎舰长",
			Enabled: true,
			On:      []event.Type{event.TypeUserEnter},
			Do:      []rules.Action{{Type: rules.ActionDanmaku, Template: []string{"舰长你好"}}},
			// 压制一个不存在的规则名——这是 review 描述的两条现实可达
			// 路径之一：DELETE 一条规则后，另一条 Suppress 指向它的
			// 规则就会变成这样，且当时没有任何检查会拦下来。
			Suppress: []string{"不存在的规则"},
		}},
	}

	room, bot, engine, client, err := buildRoomRuntime(
		context.Background(), cfg, acctRT, info, activity, nil, scheduler.New(slog.Default()), slog.Default())
	if err == nil {
		t.Fatal("Suppress 指向不存在的规则名，buildRoomRuntime 应当报错，实际没有")
	}
	if room != nil || bot != nil || engine != nil || client != nil {
		t.Error("失败时不应返回任何部分构造好的资源")
	}
}

// TestAssembleRuntimesSkipsInvalidBindingButKeepsOthers 是本项修复的核心
// 测试：两个绑定一起装配，一个规则非法、一个合法。**RED（修复前）**：
// 旧代码在装配循环里对 NewEngine 的错误直接 `return err`，这个测试会在
// 第一个断言（err 不该非 nil）就失败，因为 assembleRuntimes 会把坏绑定
// 的错误整个冒泡出来，而不是跳过它。
//
// 用 httptest 模拟 RoomInfo（唯一需要的网络请求），不需要真实 B 站账号，
// 也不需要 wbi 签名（room.go 的 GetJSON 调用 sign=false）。
func TestAssembleRuntimesSkipsInvalidBindingButKeepsOthers(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		roomID := r.URL.Query().Get("room_id")
		fmt.Fprintf(w, `{"code":0,"data":{"room_id":%s,"uid":1,"title":"标题","live_status":0}}`, roomID)
	}))
	t.Cleanup(srv.Close)

	accounts := map[string]*accountRuntime{
		"坏账号": newAssembleTestAccount(t, "坏账号", srv),
		"好账号": newAssembleTestAccount(t, "好账号", srv),
	}

	cfgs := []store.RunConfig{
		{
			AccountName: "坏账号", BindingID: 1, AccountID: 1, RoomID: "111",
			Rules: []rules.Rule{{
				Name: "只欢迎舰长", Enabled: true,
				On:       []event.Type{event.TypeUserEnter},
				Do:       []rules.Action{{Type: rules.ActionDanmaku, Template: []string{"你好"}}},
				Suppress: []string{"不存在的规则"},
			}},
		},
		{
			AccountName: "好账号", BindingID: 2, AccountID: 2, RoomID: "222",
			Rules: []rules.Rule{{
				Name: "进场欢迎", Enabled: true,
				On: []event.Type{event.TypeUserEnter},
				Do: []rules.Action{{Type: rules.ActionDanmaku, Template: []string{"欢迎"}}},
			}},
		},
	}

	activity := logging.NewActivityWriter(logging.ActivityWriterOptions{
		Flush: func(context.Context, []store.ActivityRow) error { return nil },
	})
	t.Cleanup(activity.Close)

	assemblies, err := assembleRuntimes(
		context.Background(), cfgs, accounts, activity, nil, scheduler.New(slog.Default()), slog.Default())
	if err != nil {
		t.Fatalf("坏账号的规则非法不该让整体装配失败，实际报错: %v", err)
	}
	if len(assemblies) != 1 {
		t.Fatalf("应该只有 1 个绑定装配成功（坏账号应被跳过），实际 %d 个", len(assemblies))
	}
	if assemblies[0].cfg.BindingID != 2 {
		t.Errorf("装配成功的应是好账号（BindingID=2），实际 BindingID=%d", assemblies[0].cfg.BindingID)
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

	// "h_" 是 internal/httpapi 测试用的命名空间（schemaNameFor 里写死的
	// 前缀）。go test ./... 会并行跑这两个包，而 cleanup 是
	// DROP SCHEMA ... CASCADE——两个包各自建一个同名 schema 时，一个的
	// DROP 会把另一个正在用的表也删掉，表现是「随机报表不存在」。
	// 用 "m_" 前缀（magicd 命令本身）避开这个命名空间。
	const schema = "m_magicd_reload_test"
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

// ---- 账号登录态检测（任务 8） ----

// newLoginTestAPIClient 建一个指向假 nav 接口的 api.Client，供
// checkAccountLogin 的单元测试使用——真打 B 站接口的测试既慢又不可控，
// 与 internal/connector/bilibili/api 包里 newTestClient 是同样的手法。
func newLoginTestAPIClient(t *testing.T, h http.HandlerFunc) *api.Client {
	t.Helper()
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)

	sess, err := auth.ParseSession("SESSDATA=x; bili_jct=y; DedeUserID=1")
	if err != nil {
		t.Fatalf("ParseSession 失败: %v", err)
	}
	c := api.New(sess, api.WithHTTPClient(srv.Client()))
	c.SetBaseURL("nav", srv.URL)
	return c
}

func TestCheckAccountLoginValid(t *testing.T) {
	c := newLoginTestAPIClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"code":0,"message":"0","data":{}}`))
	})

	state, err := checkAccountLogin(context.Background(), c)
	if err != nil {
		t.Fatalf("不应报错: %v", err)
	}
	if state != store.LoginStateValid {
		t.Errorf("state = %q, 期望 %q", state, store.LoginStateValid)
	}
}

// nav 接口在未登录时返回 code=-101，这是 client.go 注释里明确写出的、
// 唯一确认代表登录态失效的业务码。
func TestCheckAccountLoginInvalid(t *testing.T) {
	c := newLoginTestAPIClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"code":-101,"message":"账号未登录"}`))
	})

	state, err := checkAccountLogin(context.Background(), c)
	if err != nil {
		t.Fatalf("code=-101 应被识别为登录失效而非探测失败，实际报错: %v", err)
	}
	if state != store.LoginStateInvalid {
		t.Errorf("state = %q, 期望 %q", state, store.LoginStateInvalid)
	}
}

// 核心行为：探测本身失败（这里模拟成 HTTP 层错误，代表网络不通等情形）
// 绝不能被当作登录失效处理，否则用户会在断网时看到「账号已掉线」。
func TestCheckAccountLoginDetectionFailureIsNotInvalid(t *testing.T) {
	c := newLoginTestAPIClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	})

	state, err := checkAccountLogin(context.Background(), c)
	if err == nil {
		t.Fatal("探测失败应返回非 nil 的 err，供调用方与「登录失效」区分")
	}
	if state == store.LoginStateInvalid {
		t.Error("探测失败被误判为登录失效——网络/服务端错误不等于账号掉线")
	}
	if state != store.LoginStateUnknown {
		t.Errorf("state = %q, 期望 %q", state, store.LoginStateUnknown)
	}
}

// 只认 -101 代表未登录，别的业务码一律当探测失败，不猜测。
func TestCheckAccountLoginUnknownCodeIsDetectionFailure(t *testing.T) {
	c := newLoginTestAPIClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"code":-400,"message":"请求错误"}`))
	})

	state, err := checkAccountLogin(context.Background(), c)
	if err == nil {
		t.Fatal("未在文档中确认过的业务码应当算探测失败，而不是断定为登录失效")
	}
	if state != store.LoginStateUnknown {
		t.Errorf("state = %q, 期望 %q", state, store.LoginStateUnknown)
	}
}

// newLoginCheckTestStore 建一个独立 schema 的真实存储，供登录态检测
// 编排逻辑（loginCheckOnce/loginCheckLoop）的测试使用——它们直接依赖
// *store.Store 的 ListAccounts/UpdateAccountLoginState。
//
// 用固定但独立于 newReloadTestStore 的 schema 名，理由与其注释相同：
// 避免与 internal/httpapi 包并行测试时 DROP SCHEMA ... CASCADE 互相打架。
func newLoginCheckTestStore(t *testing.T) *store.Store {
	t.Helper()
	dsn := os.Getenv("MAGICD_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("未设置 MAGICD_TEST_DATABASE_URL，跳过需要真实数据库的测试。\n" +
			"本地起库：docker compose -f docker-compose.dev.yml up -d\n" +
			"然后：export MAGICD_TEST_DATABASE_URL='postgres://magicd:magicd@localhost:5433/magicd?sslmode=disable'")
	}

	const schema = "m_magicd_login_check_test"
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

// TestLoginCheckOnceWritesCheckerResults 验证 loginCheckOnce 把 loginChecker
// 的判定结果原样落到对应账号身上，不会张冠李戴。
func TestLoginCheckOnceWritesCheckerResults(t *testing.T) {
	st := newLoginCheckTestStore(t)
	ctx := context.Background()

	owner, err := st.CreateUser(ctx, "张三", "密码123456", false)
	if err != nil {
		t.Fatalf("建用户报错: %v", err)
	}
	valid, err := st.CreateAccount(ctx, store.AccountInput{Name: "有效号", Cookie: "SESSDATA=a", OwnerID: owner.ID})
	if err != nil {
		t.Fatalf("建账号报错: %v", err)
	}
	if _, err := st.CreateAccount(ctx, store.AccountInput{Name: "失效号", Cookie: "SESSDATA=b", OwnerID: owner.ID}); err != nil {
		t.Fatalf("建账号报错: %v", err)
	}

	check := func(_ context.Context, cookie string) (string, error) {
		if cookie == valid.Cookie {
			return store.LoginStateValid, nil
		}
		return store.LoginStateInvalid, nil
	}

	loginCheckOnce(ctx, st, check, slog.Default())

	got1, err := st.GetAccountByName(ctx, "有效号")
	if err != nil {
		t.Fatalf("查询报错: %v", err)
	}
	if got1.LoginState != store.LoginStateValid {
		t.Errorf("有效号 LoginState = %q, 期望 %q", got1.LoginState, store.LoginStateValid)
	}
	if got1.LoginCheckedAt == nil {
		t.Error("检测完成后应记录 LoginCheckedAt")
	}

	got2, err := st.GetAccountByName(ctx, "失效号")
	if err != nil {
		t.Fatalf("查询报错: %v", err)
	}
	if got2.LoginState != store.LoginStateInvalid {
		t.Errorf("失效号 LoginState = %q, 期望 %q", got2.LoginState, store.LoginStateInvalid)
	}
}

// TestLoginCheckOnceOneAccountFailureDoesNotBlockOthers 是本任务里最关键的
// 一条测试，钉住两件事：
//  1. 探测失败（loginChecker 返回 err）绝不能被写成 LoginStateInvalid——
//     网络不通不等于账号掉线；
//  2. 一个账号探测失败不能让循环提前退出，其余账号必须照常被检测到。
//
// 用变异测试验证过它的有效性：把 loginCheckOnce 里「err != nil 时仍按
// checker 返回的 state 写库」改成「err != nil 就强制写 LoginStateInvalid」，
// 本测试的第一个断言会失败（见任务报告）。
func TestLoginCheckOnceOneAccountFailureDoesNotBlockOthers(t *testing.T) {
	st := newLoginCheckTestStore(t)
	ctx := context.Background()

	owner, err := st.CreateUser(ctx, "张三", "密码123456", false)
	if err != nil {
		t.Fatalf("建用户报错: %v", err)
	}
	bad, err := st.CreateAccount(ctx, store.AccountInput{Name: "坏号", Cookie: "SESSDATA=bad", OwnerID: owner.ID})
	if err != nil {
		t.Fatalf("建账号报错: %v", err)
	}
	if _, err := st.CreateAccount(ctx, store.AccountInput{Name: "好号", Cookie: "SESSDATA=good", OwnerID: owner.ID}); err != nil {
		t.Fatalf("建账号报错: %v", err)
	}

	check := func(_ context.Context, cookie string) (string, error) {
		if cookie == bad.Cookie {
			return store.LoginStateUnknown, errors.New("网络错误：连接超时")
		}
		return store.LoginStateValid, nil
	}

	loginCheckOnce(ctx, st, check, slog.Default())

	gotBad, err := st.GetAccountByName(ctx, "坏号")
	if err != nil {
		t.Fatalf("查询报错: %v", err)
	}
	if gotBad.LoginState == store.LoginStateInvalid {
		t.Error("探测失败的账号被记成了登录失效——网络错误不等于账号掉线")
	}
	if gotBad.LoginState != store.LoginStateUnknown {
		t.Errorf("坏号 LoginState = %q, 期望 %q", gotBad.LoginState, store.LoginStateUnknown)
	}

	gotGood, err := st.GetAccountByName(ctx, "好号")
	if err != nil {
		t.Fatalf("查询报错: %v", err)
	}
	if gotGood.LoginState != store.LoginStateValid {
		t.Errorf("坏号探测失败不该影响好号：好号 LoginState = %q, 期望 %q（说明循环提前退出了）",
			gotGood.LoginState, store.LoginStateValid)
	}
}

// 全批次终审项【6b】：ctx 取消（关停中）时，loginCheckOnce 不该继续
// 对剩下的账号挨个探测——那必然失败（Warn）再必然写库失败（Error），
// 每个账号刷 2 行没有任何信息量的日志，纯粹是退出路径上的噪音。
//
// 手法：账号甲先建（ListAccounts 按 id 排序，先被处理），check 在处理
// 账号甲期间触发 cancel()，模拟"探测进行到一半，Ctrl+C 来了"。断言
// check 恰好被调用 1 次——若没有 ctx.Done() 检查，循环会继续跑到账号乙，
// calls 会是 2。
func TestLoginCheckOnceReturnsEarlyWhenContextCancelled(t *testing.T) {
	st := newLoginCheckTestStore(t)
	ctx := context.Background()

	owner, err := st.CreateUser(ctx, "张三", "密码123456", false)
	if err != nil {
		t.Fatalf("建用户报错: %v", err)
	}
	if _, err := st.CreateAccount(ctx, store.AccountInput{Name: "账号甲", Cookie: "SESSDATA=a", OwnerID: owner.ID}); err != nil {
		t.Fatalf("建账号报错: %v", err)
	}
	if _, err := st.CreateAccount(ctx, store.AccountInput{Name: "账号乙", Cookie: "SESSDATA=b", OwnerID: owner.ID}); err != nil {
		t.Fatalf("建账号报错: %v", err)
	}

	runCtx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var calls int32
	check := func(context.Context, string) (string, error) {
		if atomic.AddInt32(&calls, 1) == 1 {
			cancel() // 模拟处理账号甲期间关停发生
		}
		return store.LoginStateValid, nil
	}

	loginCheckOnce(runCtx, st, check, slog.Default())

	if got := atomic.LoadInt32(&calls); got != 1 {
		t.Errorf("check 被调用了 %d 次，期望恰好 1 次——ctx 在处理账号甲期间被取消后，"+
			"账号乙不该再被探测", got)
	}
}

// TestLoginCheckLoopRunsImmediatelyAndRespectsCancellation 验证
// loginCheckLoop 与 purgeLoop 同样的两条约束：启动时立刻做一次检测
// （不等第一个 10 分钟的 tick），以及 ctx 取消后能干净退出。
func TestLoginCheckLoopRunsImmediatelyAndRespectsCancellation(t *testing.T) {
	st := newLoginCheckTestStore(t)
	ctx := context.Background()

	owner, err := st.CreateUser(ctx, "张三", "密码123456", false)
	if err != nil {
		t.Fatalf("建用户报错: %v", err)
	}
	if _, err := st.CreateAccount(ctx, store.AccountInput{Name: "小号", Cookie: "SESSDATA=x", OwnerID: owner.ID}); err != nil {
		t.Fatalf("建账号报错: %v", err)
	}

	var calls int32
	check := func(context.Context, string) (string, error) {
		atomic.AddInt32(&calls, 1)
		return store.LoginStateValid, nil
	}

	runCtx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		loginCheckLoop(runCtx, st, check, slog.Default())
		close(done)
	}()

	// 轮询等第一次检测跑完，而不是固定 sleep：loginCheckInterval 是
	// 10 分钟量级，若这里等的是 tick 而不是启动时的立即检测，测试会
	// 一直卡到超时，能明确暴露「没有立刻检测」这个问题。
	deadline := time.Now().Add(3 * time.Second)
	for atomic.LoadInt32(&calls) == 0 {
		if time.Now().After(deadline) {
			t.Fatal("启动时应立刻做一次检测，而不是等第一个 10 分钟的 tick 才开始")
		}
		time.Sleep(10 * time.Millisecond)
	}

	cancel()
	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("ctx 取消后 loginCheckLoop 应尽快退出，而不是继续等下一个 tick")
	}
}

// 「一个绑定都没有」不能让守护进程退出——WebUI 是添加绑定的唯一图形入口，
// 退出就成了死锁（没绑定 → run 退出 → WebUI 起不来 → 加不了绑定）。
// Docker 部署下这个死锁表现为崩溃重启循环，是在真实机器上第一次部署时
// 撞到的；这条测试就是防止有人「顺手」把那个校验加回去。
func TestNoBindingsDoesNotKillDaemonWhileWebUIIsUp(t *testing.T) {
	for _, addr := range []string{"127.0.0.1:8080", "0.0.0.0:20992"} {
		if err := noBindingsFatal(addr); err != nil {
			t.Errorf("WebUI 开着(%s)时不该退出，实际报错: %v", addr, err)
		}
	}

	// WebUI 也关掉时就真的无事可做了，这时报错退出才是对的。
	for _, addr := range []string{"", "off"} {
		err := noBindingsFatal(addr)
		if err == nil {
			t.Errorf("WebUI 关闭(%q)且无绑定时应报错退出", addr)
			continue
		}
		if !contains(err.Error(), "MAGICD_HTTP_ADDR=off") {
			t.Errorf("错误信息应告诉用户怎么自救，实际: %v", err)
		}
	}
}
