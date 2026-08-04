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

// ---- P6 任务 5：今日电池到账（GiftCoins）——只有金瓜子才算 ----
//
// B 站礼物有 coin_type（gold/silver）之分，免费礼物（小花花、人气票等）
// 是 silver，不产生真实电池，绝不能计进"电池到账"。**不能用
// TotalCoin > 0 这种近似判据**——某些免费礼物的 total_coin 本身就不是
// 0（比如承载着人气值），用金额是否为正来猜测coin_type 会在这类礼物上
// 判错，必须老老实实读 coin_type 字段。

// TestQueryStatsByDaySumsGiftCoinsOnlyForGoldCoinGifts 是这条修复最核心
// 的一条测试：金瓜子礼物计入 GiftCoins，银瓜子（免费）礼物不计入，
// 即便它的 TotalCoin 字段本身是正数。
func TestQueryStatsByDaySumsGiftCoinsOnlyForGoldCoinGifts(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	accID := mustAccount(t, s, "小号")

	if err := s.InsertActivity(ctx, []ActivityRow{
		// 金瓜子礼物：辣条，500 电池到账
		{AccountID: accID, Kind: ActivityEvent, EventType: "gift",
			Detail:     []byte(`{"GiftName":"辣条","Count":1,"CoinType":"gold","TotalCoin":50000,"BlindBox":null}`),
			OccurredAt: statsFixedTime},
		// 免费礼物：小心心，coin_type 是 silver，即便 TotalCoin 是正数
		// （承载人气值，不是真实电池），也绝不能计进电池到账。
		{AccountID: accID, Kind: ActivityEvent, EventType: "gift",
			Detail:     []byte(`{"GiftName":"小心心","Count":1,"CoinType":"silver","TotalCoin":100,"BlindBox":null}`),
			OccurredAt: statsFixedTime.Add(time.Minute)},
	}); err != nil {
		t.Fatalf("写入报错: %v", err)
	}

	got, err := s.QueryStatsByDay(ctx, StatsQuery{AccountID: accID})
	if err != nil {
		t.Fatalf("按天聚合报错: %v", err)
	}
	b := statsBucketFor(t, got, "2026-07-31")
	if b.GiftCoins != 50000 {
		t.Errorf("GiftCoins = %d, 期望 50000（只算金瓜子礼物，免费礼物不算，"+
			"即便它的 TotalCoin 本身是正数）", b.GiftCoins)
	}
}

// TestQueryStatsByDayGiftCoinsExcludesBlindBox 验证盲盒礼物不计入
// GiftCoins——盲盒继续单独算（BlindBoxProfit），不混进常规礼物统计，
// 这是 P4-4 的硬性要求，电池到账这个新字段也不能例外。
func TestQueryStatsByDayGiftCoinsExcludesBlindBox(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	accID := mustAccount(t, s, "小号")

	if err := s.InsertActivity(ctx, []ActivityRow{
		{AccountID: accID, Kind: ActivityEvent, EventType: "gift",
			Detail: []byte(`{"GiftName":"星光铃铛","Count":1,"CoinType":"gold","Price":5200,"TotalCoin":5000,` +
				`"BlindBox":{"Name":"幸运盲盒","Price":5000,"TipPrice":5200}}`),
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
		t.Errorf("GiftCoins = %d, 期望 0（盲盒礼物不计入常规电池到账，单独看盲盒盈亏）", b.GiftCoins)
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
			Detail:     []byte(`{"GiftName":"辣条","Count":1,"CoinType":"gold","TotalCoin":50000,"BlindBox":null}`),
			OccurredAt: start.Add(time.Minute)},
		{AccountID: accID, BindingID: &b.ID, Kind: ActivityEvent, EventType: "gift",
			Detail:     []byte(`{"GiftName":"小心心","Count":1,"CoinType":"silver","TotalCoin":100,"BlindBox":null}`),
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
// Count 之和，电池数只累加金瓜子礼物的 TotalCoin。
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
			Detail:     []byte(`{"GiftName":"辣条","Count":1,"CoinType":"gold","TotalCoin":50000,"BlindBox":null}`),
			OccurredAt: statsFixedTime},
		{AccountID: accID, BindingID: &bind.ID, Kind: ActivityEvent, EventType: "gift",
			Detail:     []byte(`{"GiftName":"辣条","Count":2,"CoinType":"gold","TotalCoin":100000,"BlindBox":null}`),
			OccurredAt: statsFixedTime.Add(time.Minute)},
		{AccountID: accID, BindingID: &bind.ID, Kind: ActivityEvent, EventType: "gift",
			Detail:     []byte(`{"GiftName":"小心心","Count":3,"CoinType":"silver","TotalCoin":300,"BlindBox":null}`),
			OccurredAt: statsFixedTime.Add(2 * time.Minute)},
	}); err != nil {
		t.Fatalf("写入报错: %v", err)
	}

	got, err := s.QueryGiftBreakdown(ctx, bind.ID, time.Time{}, time.Time{})
	if err != nil {
		t.Fatalf("查询礼物明细报错: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("礼物种类数 = %d, 期望 2: %+v", len(got), got)
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
