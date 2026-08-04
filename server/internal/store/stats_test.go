package store

import (
	"context"
	"testing"
	"time"
)

// statsFixedTime 是统计测试用的固定时刻，避免跨天/跨秒时断言随机失败。
var statsFixedTime = time.Date(2026, 7, 31, 10, 0, 0, 0, time.UTC)

func statsBucketFor(t *testing.T, buckets []StatsBucket, bucket string) StatsBucket {
	t.Helper()
	for _, b := range buckets {
		if b.Bucket == bucket {
			return b
		}
	}
	t.Fatalf("没有找到 bucket %q，实际 %+v", bucket, buckets)
	return StatsBucket{}
}

func TestQueryStatsByDayCountsBusinessEvents(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	accID := mustAccount(t, s, "小号")

	if err := s.InsertActivity(ctx, []ActivityRow{
		{AccountID: accID, Kind: ActivityEvent, EventType: "danmaku", OccurredAt: statsFixedTime},
		{AccountID: accID, Kind: ActivityEvent, EventType: "danmaku", OccurredAt: statsFixedTime.Add(time.Minute)},
		{AccountID: accID, Kind: ActivityEvent, EventType: "user_enter", OccurredAt: statsFixedTime},
		{AccountID: accID, Kind: ActivityEvent, EventType: "guard_buy", OccurredAt: statsFixedTime},
		{AccountID: accID, Kind: ActivityEvent, EventType: "gift",
			Detail: []byte(`{"GiftName":"小心心"}`), OccurredAt: statsFixedTime},
	}); err != nil {
		t.Fatalf("写入报错: %v", err)
	}

	got, err := s.QueryStatsByDay(ctx, StatsQuery{AccountID: accID})
	if err != nil {
		t.Fatalf("按天聚合报错: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("bucket 数 = %d, 期望 1: %+v", len(got), got)
	}
	b := got[0]
	if b.Bucket != "2026-07-31" {
		t.Errorf("bucket = %q, 期望 2026-07-31", b.Bucket)
	}
	if b.DanmakuCount != 2 || b.EnterCount != 1 || b.GiftCount != 1 || b.GuardCount != 1 {
		t.Errorf("计数不对: %+v", b)
	}
}

// giftKinds 是去重计数，不是礼物总数
func TestQueryStatsByDayGiftKindsIsDistinct(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	accID := mustAccount(t, s, "小号")

	if err := s.InsertActivity(ctx, []ActivityRow{
		{AccountID: accID, Kind: ActivityEvent, EventType: "gift",
			Detail: []byte(`{"GiftName":"小心心"}`), OccurredAt: statsFixedTime},
		{AccountID: accID, Kind: ActivityEvent, EventType: "gift",
			Detail: []byte(`{"GiftName":"小心心"}`), OccurredAt: statsFixedTime.Add(time.Minute)},
		{AccountID: accID, Kind: ActivityEvent, EventType: "gift",
			Detail: []byte(`{"GiftName":"辣条"}`), OccurredAt: statsFixedTime.Add(2 * time.Minute)},
	}); err != nil {
		t.Fatalf("写入报错: %v", err)
	}

	got, err := s.QueryStatsByDay(ctx, StatsQuery{AccountID: accID})
	if err != nil {
		t.Fatalf("按天聚合报错: %v", err)
	}
	b := statsBucketFor(t, got, "2026-07-31")
	if b.GiftCount != 3 {
		t.Errorf("GiftCount = %d, 期望 3（总条数）", b.GiftCount)
	}
	if b.GiftKinds != 2 {
		t.Errorf("GiftKinds = %d, 期望 2（去重后的礼物种类）", b.GiftKinds)
	}
}

// COMBO_SEND 与其对应的多条 SEND_GIFT 是重复计数关系（见 cmdmap/gift.go），
// GiftCount 只数 event_type=gift，不把 gift_combo 也加进去，否则同一波
// 礼物会被数两遍。
func TestQueryStatsByDayExcludesGiftCombo(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	accID := mustAccount(t, s, "小号")

	if err := s.InsertActivity(ctx, []ActivityRow{
		{AccountID: accID, Kind: ActivityEvent, EventType: "gift",
			Detail: []byte(`{"GiftName":"辣条"}`), OccurredAt: statsFixedTime},
		{AccountID: accID, Kind: ActivityEvent, EventType: "gift_combo",
			Detail: []byte(`{"GiftName":"辣条"}`), OccurredAt: statsFixedTime.Add(time.Second)},
	}); err != nil {
		t.Fatalf("写入报错: %v", err)
	}

	got, err := s.QueryStatsByDay(ctx, StatsQuery{AccountID: accID})
	if err != nil {
		t.Fatalf("按天聚合报错: %v", err)
	}
	b := statsBucketFor(t, got, "2026-07-31")
	if b.GiftCount != 1 {
		t.Errorf("GiftCount = %d, 期望 1（gift_combo 不应计入）", b.GiftCount)
	}
}

// TestQueryStatsByDaySumsBlindBoxProfitByBattery 验证盈亏是按每一条盲盒
// 送礼事件的原始电池数量（Price*Count - TotalCoin）累加，不是按礼物名
// 分组再取某一种——两条盲盒记录爆出的礼物名完全不同也要能正确累加。
//
// 普通礼物那一行的 detail **故意写成 `"BlindBox":null`（显式 JSON
// null），不是干脆省略这个键**——event.Gift 没有 json tag、没有
// omitempty，`json.Marshal` 对非盲盒礼物真实产出的就是这个形状（已用
// 一次性脚本核实过）。这个细节曾经真实掉过坑：SQL 侧一度写成
// `detail->'BlindBox' IS NOT NULL`（`->` 取 jsonb），PostgreSQL 对
// 「值是 JSON null」返回的 jsonb 值本身不是 SQL NULL，`IS NOT NULL` 对它
// 判真——如果测试 fixture 图省事省略这个键（等价于键缺失，`->` 对键
// 缺失能正确返回 SQL NULL），这个 bug 不会被任何测试测出来，是「测试
// 数据形状偏离真实生产序列化路径」的又一个真实案例。现在 SQL 已经改用
// `->>`（取文本，JSON null 与键缺失两种情况下都正确返回 SQL NULL），
// 这条测试的 fixture 也换成生产真实形状，两边对齐。
func TestQueryStatsByDaySumsBlindBoxProfitByBattery(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	accID := mustAccount(t, s, "小号")

	if err := s.InsertActivity(ctx, []ActivityRow{
		// 幸运盲盒：50 电池成本(5000)，爆出的礼物单价 52 电池(5200)*1 —— 赚 200
		{AccountID: accID, Kind: ActivityEvent, EventType: "gift",
			Detail: []byte(`{"GiftName":"星光铃铛","Count":1,"Price":5200,"TotalCoin":5000,` +
				`"BlindBox":{"Name":"幸运盲盒","Price":5000,"TipPrice":5200}}`),
			OccurredAt: statsFixedTime},
		// 心动盲盒：另一个礼物名，30 电池成本(3000)，爆出的单价 10 电池(1000)*1 —— 亏 2000
		{AccountID: accID, Kind: ActivityEvent, EventType: "gift",
			Detail: []byte(`{"GiftName":"棒棒糖","Count":1,"Price":1000,"TotalCoin":3000,` +
				`"BlindBox":{"Name":"心动盲盒","Price":3000,"TipPrice":1000}}`),
			OccurredAt: statsFixedTime.Add(time.Minute)},
		// 普通礼物（非盲盒）：Price*Count-TotalCoin = 100*100-9000 = 1000，
		// 刻意选一个非零值——如果 SQL 误判成盲盒，这一行会把 BlindBoxProfit
		// 的断言也一起带错（不只是 GiftCount/GiftKinds 两个断言），不依赖
		// 凑巧抵消成 0 才能抓住这个坑。
		{AccountID: accID, Kind: ActivityEvent, EventType: "gift",
			Detail:     []byte(`{"GiftName":"小心心","Count":100,"Price":100,"TotalCoin":9000,"BlindBox":null}`),
			OccurredAt: statsFixedTime.Add(2 * time.Minute)},
	}); err != nil {
		t.Fatalf("写入报错: %v", err)
	}

	got, err := s.QueryStatsByDay(ctx, StatsQuery{AccountID: accID})
	if err != nil {
		t.Fatalf("按天聚合报错: %v", err)
	}
	b := statsBucketFor(t, got, "2026-07-31")
	const want = 200 - 2000 // 两条盲盒记录按电池数量相加，与礼物名无关
	if b.BlindBoxProfit != want {
		t.Errorf("BlindBoxProfit = %d, 期望 %d（幸运赚 200、心动亏 2000，按电池数量分别累加，"+
			"普通礼物那一行——哪怕 detail 显式带 BlindBox:null——完全不应计入）",
			b.BlindBoxProfit, want)
	}

	// 计划文件硬性要求：礼物件数/种类不含盲盒（用户原话「盲盒类单独
	// 计算」）。3 条 gift 行里 2 条是盲盒，只有普通礼物那 1 条应该计入
	// GiftCount/GiftKinds；盲盒爆出的「星光铃铛」「棒棒糖」两个礼物名
	// 不该污染 GiftKinds。
	if b.GiftCount != 1 {
		t.Errorf("GiftCount = %d, 期望 1（3 条 gift 里 2 条是盲盒，礼物件数不含盲盒）", b.GiftCount)
	}
	if b.GiftKinds != 1 {
		t.Errorf("GiftKinds = %d, 期望 1（盲盒爆出的礼物名不该进礼物种类统计）", b.GiftKinds)
	}
}

// TestQueryStatsByDayBlindBoxProfitZeroWithoutBlindBoxGifts 验证没有任何
// 盲盒记录的分桶盈亏是真实的 0（SQL SUM 对空集合返回 NULL，必须
// COALESCE 成 0，否则 Scan 会因类型不匹配报错，调用方也拿不到一个能直接
// 展示的数字）。
func TestQueryStatsByDayBlindBoxProfitZeroWithoutBlindBoxGifts(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	accID := mustAccount(t, s, "小号")

	if err := s.InsertActivity(ctx, []ActivityRow{
		{AccountID: accID, Kind: ActivityEvent, EventType: "danmaku", OccurredAt: statsFixedTime},
	}); err != nil {
		t.Fatalf("写入报错: %v", err)
	}

	got, err := s.QueryStatsByDay(ctx, StatsQuery{AccountID: accID})
	if err != nil {
		t.Fatalf("按天聚合报错: %v", err)
	}
	b := statsBucketFor(t, got, "2026-07-31")
	if b.BlindBoxProfit != 0 {
		t.Errorf("BlindBoxProfit = %d, 期望 0", b.BlindBoxProfit)
	}
}

// RecordAction 把触发它的事件类型也写进 event_type 列（同一条时间线上
// 看因果），统计不能把这类 kind=action 的行当成业务事件数进去。
func TestQueryStatsByDayExcludesActionRows(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	accID := mustAccount(t, s, "小号")

	if err := s.InsertActivity(ctx, []ActivityRow{
		{AccountID: accID, Kind: ActivityEvent, EventType: "danmaku", OccurredAt: statsFixedTime},
		{AccountID: accID, Kind: ActivityAction, EventType: "danmaku",
			ActionType: "danmaku", RuleName: "关键词回复", OccurredAt: statsFixedTime.Add(time.Second)},
	}); err != nil {
		t.Fatalf("写入报错: %v", err)
	}

	got, err := s.QueryStatsByDay(ctx, StatsQuery{AccountID: accID})
	if err != nil {
		t.Fatalf("按天聚合报错: %v", err)
	}
	b := statsBucketFor(t, got, "2026-07-31")
	if b.DanmakuCount != 1 {
		t.Errorf("DanmakuCount = %d, 期望 1（action 行不该被算进去）", b.DanmakuCount)
	}
}

func TestQueryStatsByDayGroupsAcrossMultipleDays(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	accID := mustAccount(t, s, "小号")

	if err := s.InsertActivity(ctx, []ActivityRow{
		{AccountID: accID, Kind: ActivityEvent, EventType: "danmaku", OccurredAt: statsFixedTime},
		{AccountID: accID, Kind: ActivityEvent, EventType: "danmaku",
			OccurredAt: statsFixedTime.Add(24 * time.Hour)},
		{AccountID: accID, Kind: ActivityEvent, EventType: "danmaku",
			OccurredAt: statsFixedTime.Add(24 * time.Hour)},
	}); err != nil {
		t.Fatalf("写入报错: %v", err)
	}

	got, err := s.QueryStatsByDay(ctx, StatsQuery{AccountID: accID})
	if err != nil {
		t.Fatalf("按天聚合报错: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("bucket 数 = %d, 期望 2: %+v", len(got), got)
	}
	if statsBucketFor(t, got, "2026-07-31").DanmakuCount != 1 {
		t.Errorf("7-31 计数不对: %+v", got)
	}
	if statsBucketFor(t, got, "2026-08-01").DanmakuCount != 2 {
		t.Errorf("8-1 计数不对: %+v", got)
	}
}

// 这是本任务最重要的一条测试：证明聚合真的发生在 SQL 侧。
//
// QueryActivity 有 500 条硬上限（防止一天几万行的全表扫），如果统计
// 接口的实现不小心复用了它再在 Go 里累加，620 条弹幕会被截断成 500 条，
// 断言就会失败。真正的 SQL 侧 GROUP BY 没有这个上限。
func TestQueryStatsByDayCountsBeyondActivityQueryLimit(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	accID := mustAccount(t, s, "小号")

	const n = 620
	rows := make([]ActivityRow, n)
	for i := range rows {
		rows[i] = ActivityRow{
			AccountID: accID, Kind: ActivityEvent, EventType: "danmaku",
			OccurredAt: statsFixedTime.Add(time.Duration(i) * time.Second),
		}
	}
	if err := s.InsertActivity(ctx, rows); err != nil {
		t.Fatalf("写入报错: %v", err)
	}

	got, err := s.QueryStatsByDay(ctx, StatsQuery{AccountID: accID})
	if err != nil {
		t.Fatalf("按天聚合报错: %v", err)
	}
	b := statsBucketFor(t, got, "2026-07-31")
	if b.DanmakuCount != n {
		t.Errorf("DanmakuCount = %d, 期望恰好 %d（不是 500 或其他截断值）", b.DanmakuCount, n)
	}
}

// by=session：正常配对的一场直播，场次内的事件计数与时长都要对
func TestQueryStatsBySessionPairsStartAndStop(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	accID := mustAccount(t, s, "小号")
	b, err := s.UpsertBinding(ctx, accID, "123")
	if err != nil {
		t.Fatalf("创建绑定报错: %v", err)
	}

	start := statsFixedTime
	stop := start.Add(2 * time.Hour)
	if err := s.InsertActivity(ctx, []ActivityRow{
		{AccountID: accID, BindingID: &b.ID, Kind: ActivityEvent, EventType: "live_start", OccurredAt: start},
		{AccountID: accID, BindingID: &b.ID, Kind: ActivityEvent, EventType: "danmaku",
			OccurredAt: start.Add(time.Minute)},
		{AccountID: accID, BindingID: &b.ID, Kind: ActivityEvent, EventType: "danmaku",
			OccurredAt: start.Add(2 * time.Minute)},
		{AccountID: accID, BindingID: &b.ID, Kind: ActivityEvent, EventType: "live_stop", OccurredAt: stop},
	}); err != nil {
		t.Fatalf("写入报错: %v", err)
	}

	got, err := s.QueryStatsBySession(ctx, StatsQuery{AccountID: accID, BindingID: b.ID})
	if err != nil {
		t.Fatalf("按场次聚合报错: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("场次数 = %d, 期望 1: %+v", len(got), got)
	}
	sess := got[0]
	if sess.Bucket != start.Format(time.RFC3339) {
		t.Errorf("bucket = %q, 期望场次开始时刻 %q", sess.Bucket, start.Format(time.RFC3339))
	}
	if sess.DanmakuCount != 2 {
		t.Errorf("DanmakuCount = %d, 期望 2", sess.DanmakuCount)
	}
	if sess.LiveSeconds != int64(2*time.Hour/time.Second) {
		t.Errorf("LiveSeconds = %d, 期望 %d", sess.LiveSeconds, int64(2*time.Hour/time.Second))
	}
}

// TestQueryStatsBySessionSumsBlindBoxProfit 验证 by=session 走的是单行
// 聚合（aggregateEventCounts），跟 by=day 的 GROUP BY 是两条不同的 SQL
// 路径，盲盒盈亏的口径必须在两条路径上保持一致——不是只改了 GROUP BY
// 那一份就算数。
func TestQueryStatsBySessionSumsBlindBoxProfit(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	accID := mustAccount(t, s, "小号")
	b, err := s.UpsertBinding(ctx, accID, "123")
	if err != nil {
		t.Fatalf("创建绑定报错: %v", err)
	}

	start := statsFixedTime
	stop := start.Add(time.Hour)
	if err := s.InsertActivity(ctx, []ActivityRow{
		{AccountID: accID, BindingID: &b.ID, Kind: ActivityEvent, EventType: "live_start", OccurredAt: start},
		{AccountID: accID, BindingID: &b.ID, Kind: ActivityEvent, EventType: "gift",
			Detail: []byte(`{"GiftName":"星光铃铛","Count":2,"Price":5200,"TotalCoin":10000,` +
				`"BlindBox":{"Name":"幸运盲盒","Price":5000,"TipPrice":5200}}`),
			OccurredAt: start.Add(time.Minute)},
		{AccountID: accID, BindingID: &b.ID, Kind: ActivityEvent, EventType: "live_stop", OccurredAt: stop},
	}); err != nil {
		t.Fatalf("写入报错: %v", err)
	}

	got, err := s.QueryStatsBySession(ctx, StatsQuery{AccountID: accID, BindingID: b.ID})
	if err != nil {
		t.Fatalf("按场次聚合报错: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("场次数 = %d, 期望 1: %+v", len(got), got)
	}
	const want = 5200*2 - 10000 // 400
	if got[0].BlindBoxProfit != want {
		t.Errorf("BlindBoxProfit = %d, 期望 %d", got[0].BlindBoxProfit, want)
	}
}

// 边界 1：只有 live_start 没有 live_stop——还在直播中，或漏了下播事件。
// 场次的结束时间取查询窗口的 until（没给 until 时代码会退化到 now()，
// 但那样测试会因为跑测试的耗时而不确定，所以这里显式传 until）。
func TestQueryStatsBySessionOngoingWithoutStopUsesUntil(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	accID := mustAccount(t, s, "小号")
	b, err := s.UpsertBinding(ctx, accID, "123")
	if err != nil {
		t.Fatalf("创建绑定报错: %v", err)
	}

	start := statsFixedTime
	until := start.Add(90 * time.Minute)
	if err := s.InsertActivity(ctx, []ActivityRow{
		{AccountID: accID, BindingID: &b.ID, Kind: ActivityEvent, EventType: "live_start", OccurredAt: start},
		{AccountID: accID, BindingID: &b.ID, Kind: ActivityEvent, EventType: "danmaku",
			OccurredAt: start.Add(time.Minute)},
	}); err != nil {
		t.Fatalf("写入报错: %v", err)
	}

	got, err := s.QueryStatsBySession(ctx, StatsQuery{AccountID: accID, BindingID: b.ID, Until: until})
	if err != nil {
		t.Fatalf("按场次聚合报错: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("场次数 = %d, 期望 1（不能因为没有 stop 就被丢弃）: %+v", len(got), got)
	}
	sess := got[0]
	wantSeconds := int64(until.Sub(start) / time.Second)
	if sess.LiveSeconds != wantSeconds {
		t.Errorf("LiveSeconds = %d, 期望 %d（用 until 兜底结束时间）", sess.LiveSeconds, wantSeconds)
	}
	if sess.DanmakuCount != 1 {
		t.Errorf("DanmakuCount = %d, 期望 1", sess.DanmakuCount)
	}
}

// 边界 2：只有 live_stop 没有 live_start——查询区间从这场直播中间切开。
// 场次的开始时间取查询窗口的 since。
func TestQueryStatsBySessionStopWithoutStartUsesSince(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	accID := mustAccount(t, s, "小号")
	b, err := s.UpsertBinding(ctx, accID, "123")
	if err != nil {
		t.Fatalf("创建绑定报错: %v", err)
	}

	since := statsFixedTime
	stop := since.Add(45 * time.Minute)
	if err := s.InsertActivity(ctx, []ActivityRow{
		{AccountID: accID, BindingID: &b.ID, Kind: ActivityEvent, EventType: "danmaku",
			OccurredAt: since.Add(10 * time.Minute)},
		{AccountID: accID, BindingID: &b.ID, Kind: ActivityEvent, EventType: "live_stop", OccurredAt: stop},
	}); err != nil {
		t.Fatalf("写入报错: %v", err)
	}

	got, err := s.QueryStatsBySession(ctx, StatsQuery{AccountID: accID, BindingID: b.ID, Since: since})
	if err != nil {
		t.Fatalf("按场次聚合报错: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("场次数 = %d, 期望 1（不能因为没有 start 就被丢弃）: %+v", len(got), got)
	}
	sess := got[0]
	if sess.Bucket != since.Format(time.RFC3339) {
		t.Errorf("bucket = %q, 期望用 since 兜底的开始时刻 %q", sess.Bucket, since.Format(time.RFC3339))
	}
	wantSeconds := int64(stop.Sub(since) / time.Second)
	if sess.LiveSeconds != wantSeconds {
		t.Errorf("LiveSeconds = %d, 期望 %d（用 since 兜底开始时间）", sess.LiveSeconds, wantSeconds)
	}
	if sess.DanmakuCount != 1 {
		t.Errorf("DanmakuCount = %d, 期望 1", sess.DanmakuCount)
	}
}

// 多场直播要各自成一行，不能被揉在一起
func TestQueryStatsBySessionMultipleSessions(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	accID := mustAccount(t, s, "小号")
	b, err := s.UpsertBinding(ctx, accID, "123")
	if err != nil {
		t.Fatalf("创建绑定报错: %v", err)
	}

	s1start := statsFixedTime
	s1stop := s1start.Add(time.Hour)
	s2start := s1start.Add(24 * time.Hour)
	s2stop := s2start.Add(3 * time.Hour)

	if err := s.InsertActivity(ctx, []ActivityRow{
		{AccountID: accID, BindingID: &b.ID, Kind: ActivityEvent, EventType: "live_start", OccurredAt: s1start},
		{AccountID: accID, BindingID: &b.ID, Kind: ActivityEvent, EventType: "live_stop", OccurredAt: s1stop},
		{AccountID: accID, BindingID: &b.ID, Kind: ActivityEvent, EventType: "live_start", OccurredAt: s2start},
		{AccountID: accID, BindingID: &b.ID, Kind: ActivityEvent, EventType: "live_stop", OccurredAt: s2stop},
	}); err != nil {
		t.Fatalf("写入报错: %v", err)
	}

	got, err := s.QueryStatsBySession(ctx, StatsQuery{AccountID: accID, BindingID: b.ID})
	if err != nil {
		t.Fatalf("按场次聚合报错: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("场次数 = %d, 期望 2: %+v", len(got), got)
	}
	if got[0].LiveSeconds != int64(time.Hour/time.Second) {
		t.Errorf("第一场 LiveSeconds = %d, 期望 %d", got[0].LiveSeconds, int64(time.Hour/time.Second))
	}
	if got[1].LiveSeconds != int64(3*time.Hour/time.Second) {
		t.Errorf("第二场 LiveSeconds = %d, 期望 %d", got[1].LiveSeconds, int64(3*time.Hour/time.Second))
	}
}

// ---- P6 任务 5：今日电池到账（GiftCoins）——判据是"电池价值是不是 0" ----
//
// 判据经历过两次订正：
//  1. 白名单（只有 coin_type=gold 才算）→ 黑名单（只排除 coin_type=silver）
//     ——用户原话：同一个礼物名既可能免费也可能收费，免费的里面还要再分
//     是不是会进电池总榜，礼物身份本身不足以判定。
//  2. 电池价值取 TotalCoin → 取 Price*Count——用户原话（用
//     `server/blindbox.jsonl` 真实样本核对过）：TotalCoin 是送礼人的
//     花费（盲盒场景下是盲盒售价，跟爆出什么无关），Price 才是这条
//     礼物本身的价值、也就是主播实际到账。两者在非盲盒场景下恒等，只有
//     盲盒会分叉，用 TotalCoin 会把"送礼人花了多少"误当成"主播到账
//     多少"。且盲盒爆出的礼物同样是收入，不该像 gift_count/gift_kinds
//     那样把盲盒排除掉。

// TestQueryStatsByDaySumsGiftCoinsExcludesOnlySilver 验证：银瓜子礼物不
// 计入（即便 TotalCoin/Price 本身是正数），但除银瓜子之外的任何
// coin_type 都要计入——不是"只有恰好等于 gold 的才算"。这里 Price 与
// TotalCoin/Count 保持一致（非盲盒场景下协议不变量：总价=单价×数量），
// 盲盒场景下 Price 与 TotalCoin 分叉的情形见
// TestQueryStatsByDayGiftCoinsIncludesBlindBoxUsingPriceNotTotalCoin。
func TestQueryStatsByDaySumsGiftCoinsExcludesOnlySilver(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	accID := mustAccount(t, s, "小号")

	if err := s.InsertActivity(ctx, []ActivityRow{
		// 金瓜子礼物：辣条，500 电池到账
		{AccountID: accID, Kind: ActivityEvent, EventType: "gift",
			Detail:     []byte(`{"GiftName":"辣条","Count":1,"CoinType":"gold","Price":50000,"TotalCoin":50000,"BlindBox":null}`),
			OccurredAt: statsFixedTime},
		// 免费礼物：小心心，coin_type 是 silver，即便金额字段是正数
		// （面值单位不是电池），也绝不能计进电池到账——这是唯一的排除项。
		{AccountID: accID, Kind: ActivityEvent, EventType: "gift",
			Detail:     []byte(`{"GiftName":"小心心","Count":1,"CoinType":"silver","Price":100,"TotalCoin":100,"BlindBox":null}`),
			OccurredAt: statsFixedTime.Add(time.Minute)},
		// 既不是 gold 也不是 silver 的 coin_type：如果实现是"只有 gold
		// 才算"的白名单，这一条会被错误地排除掉；"排除法"（只排除 silver）
		// 则会正确把它计入 300 电池。
		{AccountID: accID, Kind: ActivityEvent, EventType: "gift",
			Detail:     []byte(`{"GiftName":"某种非金非银结算的礼物","Count":1,"CoinType":"other","Price":30000,"TotalCoin":30000,"BlindBox":null}`),
			OccurredAt: statsFixedTime.Add(2 * time.Minute)},
	}); err != nil {
		t.Fatalf("写入报错: %v", err)
	}

	got, err := s.QueryStatsByDay(ctx, StatsQuery{AccountID: accID})
	if err != nil {
		t.Fatalf("按天聚合报错: %v", err)
	}
	b := statsBucketFor(t, got, "2026-07-31")
	if b.GiftCoins != 80000 {
		t.Errorf("GiftCoins = %d, 期望 80000（50000+30000，只排除银瓜子那一条，"+
			"不是只有 coin_type=gold 才算）", b.GiftCoins)
	}
}

// TestQueryStatsByDayGiftCoinsMissingCoinTypeDefaultsToZero 覆盖 CoinType
// 字段整个缺失的边界（比如手写测试数据、或理论上的老数据缺口）：SQL 里
// `detail->>'CoinType' = 'silver'` 在键缺失时是 SQL NULL，NULL 参与
// `CASE WHEN` 判断时既不算真也不算假，会落到 ELSE 分支——这条测试确认
// 这个三值逻辑的推断是对的，缺失 CoinType 的礼物安全地记 0（不会被
// "排除法"误判成"确定不是银瓜子"从而把缺字段的电池价值也算进去）。
func TestQueryStatsByDayGiftCoinsMissingCoinTypeDefaultsToZero(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	accID := mustAccount(t, s, "小号")

	if err := s.InsertActivity(ctx, []ActivityRow{
		{AccountID: accID, Kind: ActivityEvent, EventType: "gift",
			Detail:     []byte(`{"GiftName":"老数据礼物","Count":1,"Price":9999,"TotalCoin":9999,"BlindBox":null}`),
			OccurredAt: statsFixedTime},
	}); err != nil {
		t.Fatalf("写入报错: %v", err)
	}

	got, err := s.QueryStatsByDay(ctx, StatsQuery{AccountID: accID})
	if err != nil {
		t.Fatalf("按天聚合报错: %v", err)
	}
	b := statsBucketFor(t, got, "2026-07-31")
	if b.GiftCoins != 0 {
		t.Errorf("GiftCoins = %d, 期望 0（CoinType 字段缺失时应该安全地记 0，"+
			"不该把缺字段的电池价值算进去）", b.GiftCoins)
	}
}

// TestQueryStatsByDayGiftCoinsIncludesBlindBoxUsingPriceNotTotalCoin 是
// 本任务最核心的一条测试：用户用 `server/blindbox.jsonl` 的真实样本核对
// 过，`price`（爆出礼物的价值，主播到账）与 `total_coin`（盲盒售价，
// 送礼人的花费）在盲盒场景下是两回事——幸运盲盒不管开出什么，
// TotalCoin 恒为 5000，但爆出星光铃铛时 Price 是 5200；心动盲盒
// TotalCoin 恒为 15000，但爆出棉花糖时 Price 只有 9000。
//
// 如果 GiftCoins 算错成 TotalCoin 之和，这里会得到 5000+15000=20000
// （送礼人花了多少）；用对 Price 之和则是 5200+9000=14200
// （主播实际到账多少）——两个数字都不是 0，也不接近，用近似值蒙混不
// 过去这条测试。
//
// 同时验证盲盒**要**计入 GiftCoins（与 gift_count/gift_kinds 排除盲盒
// 是两条不同的约束，不冲突，见 StatsBucket.GiftCoins 的注释）。
func TestQueryStatsByDayGiftCoinsIncludesBlindBoxUsingPriceNotTotalCoin(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	accID := mustAccount(t, s, "小号")

	if err := s.InsertActivity(ctx, []ActivityRow{
		// 幸运盲盒开出星光铃铛：售价（TotalCoin）5000，爆出礼物价值
		// （Price）5200——真实样本数值。
		{AccountID: accID, Kind: ActivityEvent, EventType: "gift",
			Detail: []byte(`{"GiftName":"星光铃铛","Count":1,"CoinType":"gold","Price":5200,"TotalCoin":5000,` +
				`"BlindBox":{"Name":"幸运盲盒","Price":5000,"TipPrice":5200}}`),
			OccurredAt: statsFixedTime},
		// 心动盲盒开出棉花糖：售价（TotalCoin）15000，爆出礼物价值
		// （Price）9000——真实样本数值。
		{AccountID: accID, Kind: ActivityEvent, EventType: "gift",
			Detail: []byte(`{"GiftName":"棉花糖","Count":1,"CoinType":"gold","Price":9000,"TotalCoin":15000,` +
				`"BlindBox":{"Name":"心动盲盒","Price":15000,"TipPrice":9000}}`),
			OccurredAt: statsFixedTime.Add(time.Minute)},
	}); err != nil {
		t.Fatalf("写入报错: %v", err)
	}

	got, err := s.QueryStatsByDay(ctx, StatsQuery{AccountID: accID})
	if err != nil {
		t.Fatalf("按天聚合报错: %v", err)
	}
	b := statsBucketFor(t, got, "2026-07-31")
	if b.GiftCoins != 14200 {
		t.Errorf("GiftCoins = %d, 期望 14200（5200+9000，用 Price 之和，"+
			"不是 TotalCoin 之和 20000；且盲盒要计入电池到账）", b.GiftCoins)
	}
}

// TestQueryStatsBySessionSumsGiftCoins 验证 by=session 走的单行聚合路径
// （aggregateEventCounts）与 by=day 的 GROUP BY 口径一致，不是只改了一份。
func TestQueryStatsBySessionSumsGiftCoins(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	accID := mustAccount(t, s, "小号")
	b, err := s.UpsertBinding(ctx, accID, "123")
	if err != nil {
		t.Fatalf("创建绑定报错: %v", err)
	}

	start := statsFixedTime
	stop := start.Add(time.Hour)
	if err := s.InsertActivity(ctx, []ActivityRow{
		{AccountID: accID, BindingID: &b.ID, Kind: ActivityEvent, EventType: "live_start", OccurredAt: start},
		{AccountID: accID, BindingID: &b.ID, Kind: ActivityEvent, EventType: "gift",
			Detail:     []byte(`{"GiftName":"辣条","Count":1,"CoinType":"gold","Price":50000,"TotalCoin":50000,"BlindBox":null}`),
			OccurredAt: start.Add(time.Minute)},
		{AccountID: accID, BindingID: &b.ID, Kind: ActivityEvent, EventType: "gift",
			Detail:     []byte(`{"GiftName":"小心心","Count":1,"CoinType":"silver","Price":100,"TotalCoin":100,"BlindBox":null}`),
			OccurredAt: start.Add(2 * time.Minute)},
		{AccountID: accID, BindingID: &b.ID, Kind: ActivityEvent, EventType: "live_stop", OccurredAt: stop},
	}); err != nil {
		t.Fatalf("写入报错: %v", err)
	}

	got, err := s.QueryStatsBySession(ctx, StatsQuery{AccountID: accID, BindingID: b.ID})
	if err != nil {
		t.Fatalf("按场次聚合报错: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("场次数 = %d, 期望 1: %+v", len(got), got)
	}
	if got[0].GiftCoins != 50000 {
		t.Errorf("GiftCoins = %d, 期望 50000", got[0].GiftCoins)
	}
}

// ---- P6 任务 5：礼物明细列表（按礼物名分组：数量 + 电池数加和） ----

// TestQueryGiftBreakdownGroupsByName 验证按礼物名分组求和：数量是各行
// Count 之和，电池数按"排除法"（只排除银瓜子）累加 Price*Count——不是
// "只有金瓜子才算"的白名单，也不是 TotalCoin，见文件头部关于 GiftCoins
// 判据的说明。这里全是非盲盒礼物，Price*Count 与 TotalCoin 恒等，
// 盲盒场景下两者分叉的情形见
// TestQueryStatsByDayGiftCoinsIncludesBlindBoxUsingPriceNotTotalCoin。
func TestQueryGiftBreakdownGroupsByName(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	accID := mustAccount(t, s, "小号")
	bind, err := s.UpsertBinding(ctx, accID, "123")
	if err != nil {
		t.Fatalf("创建绑定报错: %v", err)
	}

	if err := s.InsertActivity(ctx, []ActivityRow{
		{AccountID: accID, BindingID: &bind.ID, Kind: ActivityEvent, EventType: "gift",
			Detail:     []byte(`{"GiftName":"辣条","Count":1,"CoinType":"gold","Price":50000,"TotalCoin":50000,"BlindBox":null}`),
			OccurredAt: statsFixedTime},
		{AccountID: accID, BindingID: &bind.ID, Kind: ActivityEvent, EventType: "gift",
			Detail:     []byte(`{"GiftName":"辣条","Count":2,"CoinType":"gold","Price":50000,"TotalCoin":100000,"BlindBox":null}`),
			OccurredAt: statsFixedTime.Add(time.Minute)},
		{AccountID: accID, BindingID: &bind.ID, Kind: ActivityEvent, EventType: "gift",
			Detail:     []byte(`{"GiftName":"小心心","Count":3,"CoinType":"silver","Price":100,"TotalCoin":300,"BlindBox":null}`),
			OccurredAt: statsFixedTime.Add(2 * time.Minute)},
		// 既不是 gold 也不是 silver：应该照常计入电池数——证明这里是
		// "排除法"（只排除银瓜子），不是"只有恰好等于 gold 的才算"的白名单。
		{AccountID: accID, BindingID: &bind.ID, Kind: ActivityEvent, EventType: "gift",
			Detail:     []byte(`{"GiftName":"神秘礼物","Count":1,"CoinType":"other","Price":700,"TotalCoin":700,"BlindBox":null}`),
			OccurredAt: statsFixedTime.Add(3 * time.Minute)},
	}); err != nil {
		t.Fatalf("写入报错: %v", err)
	}

	got, err := s.QueryGiftBreakdown(ctx, bind.ID, time.Time{}, time.Time{})
	if err != nil {
		t.Fatalf("查询礼物明细报错: %v", err)
	}
	if len(got) != 3 {
		t.Fatalf("礼物种类数 = %d, 期望 3: %+v", len(got), got)
	}

	byName := map[string]GiftBreakdownRow{}
	for _, r := range got {
		byName[r.GiftName] = r
	}
	larou, ok := byName["辣条"]
	if !ok {
		t.Fatalf("没有找到「辣条」: %+v", got)
	}
	if larou.Count != 3 {
		t.Errorf("辣条 Count = %d, 期望 3（两行 1+2 求和）", larou.Count)
	}
	if larou.Coins != 150000 {
		t.Errorf("辣条 Coins = %d, 期望 150000（两行 50000+100000 求和）", larou.Coins)
	}

	heart, ok := byName["小心心"]
	if !ok {
		t.Fatalf("没有找到「小心心」: %+v", got)
	}
	if heart.Count != 3 {
		t.Errorf("小心心 Count = %d, 期望 3（数量照常统计，即便是免费礼物）", heart.Count)
	}
	if heart.Coins != 0 {
		t.Errorf("小心心 Coins = %d, 期望 0（银瓜子免费礼物不产生电池，即便 TotalCoin 是正数）", heart.Coins)
	}

	mystery, ok := byName["神秘礼物"]
	if !ok {
		t.Fatalf("没有找到「神秘礼物」: %+v", got)
	}
	if mystery.Coins != 700 {
		t.Errorf("神秘礼物 Coins = %d, 期望 700（coin_type 既非 gold 也非 silver，"+
			"「排除法」应该照常计入，不能被白名单误排除）", mystery.Coins)
	}
}

// TestQueryGiftBreakdownExcludesBlindBox 验证盲盒不混进礼物明细列表——
// P4-4 的硬性要求：盲盒继续单独算。
func TestQueryGiftBreakdownExcludesBlindBox(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	accID := mustAccount(t, s, "小号")
	bind, err := s.UpsertBinding(ctx, accID, "123")
	if err != nil {
		t.Fatalf("创建绑定报错: %v", err)
	}

	if err := s.InsertActivity(ctx, []ActivityRow{
		{AccountID: accID, BindingID: &bind.ID, Kind: ActivityEvent, EventType: "gift",
			Detail: []byte(`{"GiftName":"星光铃铛","Count":1,"CoinType":"gold","Price":5200,"TotalCoin":5000,` +
				`"BlindBox":{"Name":"幸运盲盒","Price":5000,"TipPrice":5200}}`),
			OccurredAt: statsFixedTime},
	}); err != nil {
		t.Fatalf("写入报错: %v", err)
	}

	got, err := s.QueryGiftBreakdown(ctx, bind.ID, time.Time{}, time.Time{})
	if err != nil {
		t.Fatalf("查询礼物明细报错: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("盲盒爆出的「星光铃铛」不该出现在礼物明细列表里，实际 %+v", got)
	}
}

// TestQueryGiftBreakdownRespectsTimeRange 验证 since/until 生效——
// "今日电池到账"卡片下方的明细列表要能按"今天"这个窗口过滤。
func TestQueryGiftBreakdownRespectsTimeRange(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	accID := mustAccount(t, s, "小号")
	bind, err := s.UpsertBinding(ctx, accID, "123")
	if err != nil {
		t.Fatalf("创建绑定报错: %v", err)
	}

	if err := s.InsertActivity(ctx, []ActivityRow{
		{AccountID: accID, BindingID: &bind.ID, Kind: ActivityEvent, EventType: "gift",
			Detail:     []byte(`{"GiftName":"昨天的礼物","Count":1,"CoinType":"gold","TotalCoin":100,"BlindBox":null}`),
			OccurredAt: statsFixedTime.Add(-24 * time.Hour)},
		{AccountID: accID, BindingID: &bind.ID, Kind: ActivityEvent, EventType: "gift",
			Detail:     []byte(`{"GiftName":"今天的礼物","Count":1,"CoinType":"gold","TotalCoin":200,"BlindBox":null}`),
			OccurredAt: statsFixedTime},
	}); err != nil {
		t.Fatalf("写入报错: %v", err)
	}

	dayStart := time.Date(2026, 7, 31, 0, 0, 0, 0, time.UTC)
	dayEnd := dayStart.Add(24 * time.Hour)
	got, err := s.QueryGiftBreakdown(ctx, bind.ID, dayStart, dayEnd)
	if err != nil {
		t.Fatalf("查询礼物明细报错: %v", err)
	}
	if len(got) != 1 || got[0].GiftName != "今天的礼物" {
		t.Errorf("按时间窗口过滤后 = %+v, 期望只有「今天的礼物」", got)
	}
}

// 按绑定隔离：不同绑定的开播事件不能串场
func TestQueryStatsBySessionIsolatedPerBinding(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	accID := mustAccount(t, s, "小号")
	b1, err := s.UpsertBinding(ctx, accID, "111")
	if err != nil {
		t.Fatalf("创建绑定报错: %v", err)
	}
	b2, err := s.UpsertBinding(ctx, accID, "222")
	if err != nil {
		t.Fatalf("创建绑定报错: %v", err)
	}

	if err := s.InsertActivity(ctx, []ActivityRow{
		{AccountID: accID, BindingID: &b1.ID, Kind: ActivityEvent, EventType: "live_start", OccurredAt: statsFixedTime},
		{AccountID: accID, BindingID: &b1.ID, Kind: ActivityEvent, EventType: "live_stop",
			OccurredAt: statsFixedTime.Add(time.Hour)},
		{AccountID: accID, BindingID: &b2.ID, Kind: ActivityEvent, EventType: "live_start", OccurredAt: statsFixedTime},
		{AccountID: accID, BindingID: &b2.ID, Kind: ActivityEvent, EventType: "live_stop",
			OccurredAt: statsFixedTime.Add(5 * time.Hour)},
	}); err != nil {
		t.Fatalf("写入报错: %v", err)
	}

	got, err := s.QueryStatsBySession(ctx, StatsQuery{AccountID: accID, BindingID: b1.ID})
	if err != nil {
		t.Fatalf("按场次聚合报错: %v", err)
	}
	if len(got) != 1 || got[0].LiveSeconds != int64(time.Hour/time.Second) {
		t.Errorf("绑定隔离失败: %+v", got)
	}
}

// ---- P8：真机 35 小时直播时长故障的回归测试 ----
//
// 真机数据（绑定 1，2026-08-04）：四条 live_start、零条 live_stop，其中
// 两条相差 13 微秒（同一批帧里 B 站重连重发了两次 LIVE 报文）：
//   02:55:12.512416
//   02:55:12.512429   ← 与上一条相差 13 微秒
//   03:29:53.816369
//   03:29:59.014680
//
// 旧的配对算法遇到连续 live_start（中间没有 live_stop）时，把前一次
// 开播单独收成一场"只有 Start"的场次；effectiveSessionBounds 对这种
// 场次一律拿 until/now 兜底结束时间。于是四场互相重叠、各自都伸到
// "现在"的场次被分别计入，加总成了 35 小时 9 分钟——一天最多只有 24
// 小时。

// TestQueryStatsByDayConsecutiveLiveStartsDoNotInflateLiveSeconds 是这个
// 缺陷本身的验收标准：断言修好之后算出来的当天直播时长不超过查询窗口
// 的长度，且恰好等于"第一条 live_start 到窗口结束"——连续 live_start
// 之间没有 live_stop 时，前一场的结束时刻应该是下一条 live_start 的
// 时刻，四段场次首尾相接、互不重叠，加总正好等于整段时间，不会因为
// 中间的"缝"被各自延伸到 until 而重复计入。
//
// until 特意取 t4 加整数小时：这样 until 与 t4 的秒内小数部分完全相同
// （until-t4 是整数秒），四段场次各自按秒截断后再相加，才能与
// "until-t1 整体截断"严格相等，不会因为四次独立截断各自丢一点小数、
// 累积出与预期差 1 秒的假失败——四条真机时间戳里前三段小数部分之和
// 是 0.502264 秒，不足 1 秒，不会发生截断进位，这条等式在算术上是成立的。
func TestQueryStatsByDayConsecutiveLiveStartsDoNotInflateLiveSeconds(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	accID := mustAccount(t, s, "小号")
	b, err := s.UpsertBinding(ctx, accID, "1")
	if err != nil {
		t.Fatalf("创建绑定报错: %v", err)
	}

	day := time.Date(2026, 8, 4, 0, 0, 0, 0, time.UTC)
	t1 := time.Date(2026, 8, 4, 2, 55, 12, 512416000, time.UTC)
	t2 := time.Date(2026, 8, 4, 2, 55, 12, 512429000, time.UTC) // 与 t1 相差 13 微秒
	t3 := time.Date(2026, 8, 4, 3, 29, 53, 816369000, time.UTC)
	t4 := time.Date(2026, 8, 4, 3, 29, 59, 14680000, time.UTC)
	until := t4.Add(18 * time.Hour) // 与 t4 秒内小数部分对齐，见上方注释

	if err := s.InsertActivity(ctx, []ActivityRow{
		{AccountID: accID, BindingID: &b.ID, Kind: ActivityEvent, EventType: "live_start", OccurredAt: t1},
		{AccountID: accID, BindingID: &b.ID, Kind: ActivityEvent, EventType: "live_start", OccurredAt: t2},
		{AccountID: accID, BindingID: &b.ID, Kind: ActivityEvent, EventType: "live_start", OccurredAt: t3},
		{AccountID: accID, BindingID: &b.ID, Kind: ActivityEvent, EventType: "live_start", OccurredAt: t4},
		// day 视图只给"窗口内有业务事件的那些天"生成 bucket（QueryStatsByDay
		// 里的注释），这里补一条弹幕让 2026-08-04 这天有 bucket 可以累加。
		{AccountID: accID, BindingID: &b.ID, Kind: ActivityEvent, EventType: "danmaku", OccurredAt: t1.Add(time.Minute)},
	}); err != nil {
		t.Fatalf("写入报错: %v", err)
	}

	got, err := s.QueryStatsByDay(ctx, StatsQuery{AccountID: accID, BindingID: b.ID, Since: day, Until: until})
	if err != nil {
		t.Fatalf("按天聚合报错: %v", err)
	}
	bucket := statsBucketFor(t, got, "2026-08-04")

	windowSeconds := int64(until.Sub(day) / time.Second)
	if bucket.LiveSeconds > windowSeconds {
		t.Fatalf("LiveSeconds = %d 超过了查询窗口长度 %d（一天最多 24 小时，这正是真机 35 小时 bug 的复现）",
			bucket.LiveSeconds, windowSeconds)
	}

	wantSeconds := int64(until.Sub(t1) / time.Second)
	if bucket.LiveSeconds != wantSeconds {
		t.Errorf("LiveSeconds = %d, 期望等于「第一条 live_start 到窗口结束」的 %d", bucket.LiveSeconds, wantSeconds)
	}
}

// TestQueryStatsByDayTwoCompletePairsSumCorrectly 是上面那条修复的对称
// 测试：真的开播两次、每次都正常配对（Start/End 都在），中间没有连续
// live_start 这种残缺情况，两段时长要老老实实相加——不能因为"连续
// live_start 用下一条 live_start 兜底 End"这条新规则而被误伤，这条新
// 规则只应该在"连续两个 live_start 之间没有 live_stop"时触发。
func TestQueryStatsByDayTwoCompletePairsSumCorrectly(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	accID := mustAccount(t, s, "小号")
	b, err := s.UpsertBinding(ctx, accID, "1")
	if err != nil {
		t.Fatalf("创建绑定报错: %v", err)
	}

	day := time.Date(2026, 8, 4, 0, 0, 0, 0, time.UTC)
	s1start := day.Add(2 * time.Hour)
	s1stop := s1start.Add(30 * time.Minute)
	s2start := s1stop.Add(time.Hour)
	s2stop := s2start.Add(45 * time.Minute)

	if err := s.InsertActivity(ctx, []ActivityRow{
		{AccountID: accID, BindingID: &b.ID, Kind: ActivityEvent, EventType: "live_start", OccurredAt: s1start},
		{AccountID: accID, BindingID: &b.ID, Kind: ActivityEvent, EventType: "live_stop", OccurredAt: s1stop},
		{AccountID: accID, BindingID: &b.ID, Kind: ActivityEvent, EventType: "live_start", OccurredAt: s2start},
		{AccountID: accID, BindingID: &b.ID, Kind: ActivityEvent, EventType: "live_stop", OccurredAt: s2stop},
		{AccountID: accID, BindingID: &b.ID, Kind: ActivityEvent, EventType: "danmaku", OccurredAt: s1start.Add(time.Minute)},
	}); err != nil {
		t.Fatalf("写入报错: %v", err)
	}

	got, err := s.QueryStatsByDay(ctx, StatsQuery{AccountID: accID, BindingID: b.ID})
	if err != nil {
		t.Fatalf("按天聚合报错: %v", err)
	}
	bucket := statsBucketFor(t, got, "2026-08-04")

	want := int64(30*time.Minute/time.Second) + int64(45*time.Minute/time.Second)
	if bucket.LiveSeconds != want {
		t.Errorf("LiveSeconds = %d, 期望两段时长相加 %d", bucket.LiveSeconds, want)
	}
}
