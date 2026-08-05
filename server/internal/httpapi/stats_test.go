package httpapi_test

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/MrZhongzq/MagicalDanmaku/server/internal/perm"
	"github.com/MrZhongzq/MagicalDanmaku/server/internal/store"
)

func TestQueryStatsByDay(t *testing.T) {
	srv, st := newTestServer(t)
	c := loginAs(t, srv, st, "张三", false)
	bid := mustBindingFor(t, st, "张三", "小号", "123")
	grantEventRead(t, st, "张三", "小号", "123")

	seedActivity(t, st, bid,
		store.ActivityRow{Kind: store.ActivityEvent, EventType: "danmaku", OccurredAt: seedTime},
		store.ActivityRow{Kind: store.ActivityEvent, EventType: "danmaku", OccurredAt: seedTime.Add(time.Minute)},
		// BlindBox 显式写成 null（不是省略这个键）——对齐 event.Gift 真实
		// json.Marshal 的输出形状（没有 json tag、没有 omitempty）。
		// 省略键在 SQL 侧用 ->> 判断时结果一样，但复审指出「测试数据
		// 形状偏离生产序列化路径」本身就是 store/stats.go 那条 ->/->>
		// bug 长期没被测出来的根因，这里顺手改成生产真实形状，不留同一
		// 类隐患。
		store.ActivityRow{Kind: store.ActivityEvent, EventType: "gift",
			Detail: []byte(`{"GiftName":"辣条","Count":1,"BlindBox":null}`), OccurredAt: seedTime},
		store.ActivityRow{Kind: store.ActivityEvent, EventType: "user_enter", OccurredAt: seedTime},
		store.ActivityRow{Kind: store.ActivityEvent, EventType: "guard_buy", OccurredAt: seedTime},
	)

	resp := jsonRequest(t, c, "GET", srv.URL+"/api/bindings/"+itoa(bid)+"/stats?by=day", "")
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("状态码 = %d", resp.StatusCode)
	}

	var got []map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&got); err != nil {
		t.Fatalf("解析报错: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("bucket 数 = %d, 期望 1: %+v", len(got), got)
	}
	row := got[0]
	if row["bucket"] != "2026-07-31" {
		t.Errorf("bucket = %v", row["bucket"])
	}
	if row["danmakuCount"].(float64) != 2 {
		t.Errorf("danmakuCount = %v, 期望 2", row["danmakuCount"])
	}
	if row["enterCount"].(float64) != 1 {
		t.Errorf("enterCount = %v, 期望 1", row["enterCount"])
	}
	if row["giftCount"].(float64) != 1 {
		t.Errorf("giftCount = %v, 期望 1", row["giftCount"])
	}
	if row["giftKinds"].(float64) != 1 {
		t.Errorf("giftKinds = %v, 期望 1", row["giftKinds"])
	}
	if row["guardCount"].(float64) != 1 {
		t.Errorf("guardCount = %v, 期望 1", row["guardCount"])
	}
	if row["blindBoxProfit"].(float64) != 0 {
		t.Errorf("blindBoxProfit = %v, 期望 0（这批样本没有盲盒礼物）", row["blindBoxProfit"])
	}
}

// TestQueryStatsByDayBlindBoxProfit 验证盲盒盈亏字段经完整 HTTP 层
// 下发——统计页盲盒盈亏卡片直接读的就是这个字段（悬空清单第 7/15 条）。
func TestQueryStatsByDayBlindBoxProfit(t *testing.T) {
	srv, st := newTestServer(t)
	c := loginAs(t, srv, st, "张三", false)
	bid := mustBindingFor(t, st, "张三", "小号", "123")
	grantEventRead(t, st, "张三", "小号", "123")

	seedActivity(t, st, bid,
		store.ActivityRow{Kind: store.ActivityEvent, EventType: "gift",
			Detail: []byte(`{"GiftName":"星光铃铛","Count":1,"Price":5200,"TotalCoin":5000,` +
				`"BlindBox":{"Name":"幸运盲盒","Price":5000,"TipPrice":5200}}`),
			OccurredAt: seedTime},
	)

	resp := jsonRequest(t, c, "GET", srv.URL+"/api/bindings/"+itoa(bid)+"/stats?by=day", "")
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("状态码 = %d", resp.StatusCode)
	}

	var got []map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&got); err != nil {
		t.Fatalf("解析报错: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("bucket 数 = %d, 期望 1: %+v", len(got), got)
	}
	if got[0]["blindBoxProfit"].(float64) != 200 {
		t.Errorf("blindBoxProfit = %v, 期望 200（5200*1-5000）", got[0]["blindBoxProfit"])
	}
}

// by 不传时默认按天分组
func TestQueryStatsDefaultsToDay(t *testing.T) {
	srv, st := newTestServer(t)
	c := loginAs(t, srv, st, "张三", false)
	bid := mustBindingFor(t, st, "张三", "小号", "123")
	grantEventRead(t, st, "张三", "小号", "123")

	seedActivity(t, st, bid,
		store.ActivityRow{Kind: store.ActivityEvent, EventType: "danmaku", OccurredAt: seedTime},
	)

	resp := jsonRequest(t, c, "GET", srv.URL+"/api/bindings/"+itoa(bid)+"/stats", "")
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("状态码 = %d", resp.StatusCode)
	}
	var got []map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&got); err != nil {
		t.Fatalf("解析报错: %v", err)
	}
	if len(got) != 1 || got[0]["bucket"] != "2026-07-31" {
		t.Errorf("默认应按天分组: %+v", got)
	}
}

func TestQueryStatsBySession(t *testing.T) {
	srv, st := newTestServer(t)
	c := loginAs(t, srv, st, "张三", false)
	bid := mustBindingFor(t, st, "张三", "小号", "123")
	grantEventRead(t, st, "张三", "小号", "123")

	start := seedTime
	stop := start.Add(2 * time.Hour)
	seedActivity(t, st, bid,
		store.ActivityRow{Kind: store.ActivityEvent, EventType: "live_start", OccurredAt: start},
		store.ActivityRow{Kind: store.ActivityEvent, EventType: "danmaku", OccurredAt: start.Add(time.Minute)},
		store.ActivityRow{Kind: store.ActivityEvent, EventType: "live_stop", OccurredAt: stop},
	)

	resp := jsonRequest(t, c, "GET", srv.URL+"/api/bindings/"+itoa(bid)+"/stats?by=session", "")
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("状态码 = %d", resp.StatusCode)
	}
	var got []map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&got); err != nil {
		t.Fatalf("解析报错: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("场次数 = %d, 期望 1: %+v", len(got), got)
	}
	if got[0]["danmakuCount"].(float64) != 1 {
		t.Errorf("danmakuCount = %v, 期望 1", got[0]["danmakuCount"])
	}
	wantSeconds := float64(2 * time.Hour / time.Second)
	if got[0]["liveSeconds"].(float64) != wantSeconds {
		t.Errorf("liveSeconds = %v, 期望 %v", got[0]["liveSeconds"], wantSeconds)
	}
}

// 边界 1：只有 live_start 没有 live_stop——还在直播中，或漏了下播事件。
// 用查询区间的 until 兜底结束时间，不静默丢弃这一场。
func TestQueryStatsBySessionOngoingWithoutStop(t *testing.T) {
	srv, st := newTestServer(t)
	c := loginAs(t, srv, st, "张三", false)
	bid := mustBindingFor(t, st, "张三", "小号", "123")
	grantEventRead(t, st, "张三", "小号", "123")

	start := seedTime
	until := start.Add(90 * time.Minute)
	seedActivity(t, st, bid,
		store.ActivityRow{Kind: store.ActivityEvent, EventType: "live_start", OccurredAt: start},
		store.ActivityRow{Kind: store.ActivityEvent, EventType: "danmaku", OccurredAt: start.Add(time.Minute)},
	)

	resp := jsonRequest(t, c, "GET", srv.URL+"/api/bindings/"+itoa(bid)+
		"/stats?by=session&until="+until.Format(time.RFC3339), "")
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("状态码 = %d", resp.StatusCode)
	}
	var got []map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&got); err != nil {
		t.Fatalf("解析报错: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("只有开播没有下播的场次不该被丢弃，实际场次数 = %d: %+v", len(got), got)
	}
	wantSeconds := float64(until.Sub(start) / time.Second)
	if got[0]["liveSeconds"].(float64) != wantSeconds {
		t.Errorf("liveSeconds = %v, 期望用 until 兜底得到 %v", got[0]["liveSeconds"], wantSeconds)
	}
}

// 边界 2：只有 live_stop 没有 live_start——查询区间从这场直播中间切开。
// 用查询区间的 since 兜底开始时间，不静默丢弃这一场。
func TestQueryStatsBySessionStopWithoutStart(t *testing.T) {
	srv, st := newTestServer(t)
	c := loginAs(t, srv, st, "张三", false)
	bid := mustBindingFor(t, st, "张三", "小号", "123")
	grantEventRead(t, st, "张三", "小号", "123")

	since := seedTime
	stop := since.Add(45 * time.Minute)
	seedActivity(t, st, bid,
		store.ActivityRow{Kind: store.ActivityEvent, EventType: "danmaku", OccurredAt: since.Add(10 * time.Minute)},
		store.ActivityRow{Kind: store.ActivityEvent, EventType: "live_stop", OccurredAt: stop},
	)

	resp := jsonRequest(t, c, "GET", srv.URL+"/api/bindings/"+itoa(bid)+
		"/stats?by=session&since="+since.Format(time.RFC3339), "")
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("状态码 = %d", resp.StatusCode)
	}
	var got []map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&got); err != nil {
		t.Fatalf("解析报错: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("只有下播没有开播的场次不该被丢弃，实际场次数 = %d: %+v", len(got), got)
	}
	wantSeconds := float64(stop.Sub(since) / time.Second)
	if got[0]["liveSeconds"].(float64) != wantSeconds {
		t.Errorf("liveSeconds = %v, 期望用 since 兜底得到 %v", got[0]["liveSeconds"], wantSeconds)
	}
	if got[0]["danmakuCount"].(float64) != 1 {
		t.Errorf("danmakuCount = %v, 期望 1", got[0]["danmakuCount"])
	}
}

func TestQueryStatsRejectsBadBy(t *testing.T) {
	srv, st := newTestServer(t)
	c := loginAs(t, srv, st, "张三", false)
	bid := mustBindingFor(t, st, "张三", "小号", "123")
	grantEventRead(t, st, "张三", "小号", "123")

	resp := jsonRequest(t, c, "GET", srv.URL+"/api/bindings/"+itoa(bid)+"/stats?by=周", "")
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusUnprocessableEntity {
		t.Errorf("状态码 = %d, 期望 422", resp.StatusCode)
	}
}

// since 晚于 until 要报错，不能静默返回空——校验与 /activity 保持一致
func TestQueryStatsRejectsInvertedRange(t *testing.T) {
	srv, st := newTestServer(t)
	c := loginAs(t, srv, st, "张三", false)
	bid := mustBindingFor(t, st, "张三", "小号", "123")
	grantEventRead(t, st, "张三", "小号", "123")

	resp := jsonRequest(t, c, "GET",
		srv.URL+"/api/bindings/"+itoa(bid)+
			"/stats?since=2026-08-01T00:00:00Z&until=2026-07-01T00:00:00Z", "")
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusUnprocessableEntity {
		t.Errorf("状态码 = %d, 期望 422", resp.StatusCode)
	}
}

func TestQueryStatsRejectsBadTimeFormat(t *testing.T) {
	srv, st := newTestServer(t)
	c := loginAs(t, srv, st, "张三", false)
	bid := mustBindingFor(t, st, "张三", "小号", "123")
	grantEventRead(t, st, "张三", "小号", "123")

	resp := jsonRequest(t, c, "GET",
		srv.URL+"/api/bindings/"+itoa(bid)+"/stats?since=不是时间", "")
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusUnprocessableEntity {
		t.Errorf("状态码 = %d, 期望 422", resp.StatusCode)
	}
}

func TestQueryStatsRequiresEventRead(t *testing.T) {
	srv, st := newTestServer(t)
	loginAs(t, srv, st, "张三", false)
	bid := mustBindingFor(t, st, "张三", "小号", "123")

	li := loginAs(t, srv, st, "李四", false)
	if err := st.Grant(context.Background(), "李四", "小号", "123",
		[]perm.Permission{perm.RuleWrite}); err != nil {
		t.Fatalf("授权报错: %v", err)
	}

	resp := jsonRequest(t, li, "GET", srv.URL+"/api/bindings/"+itoa(bid)+"/stats", "")
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusForbidden {
		t.Errorf("状态码 = %d, 期望 403", resp.StatusCode)
	}
}

// 这是本任务最重要的一条测试：证明聚合真的发生在 SQL 侧，而不是把
// /activity 的 500 条上限拉过来在 Go 里数。塞 620 条弹幕，断言
// danmakuCount 是真实条数——如果实现偷懒复用了 QueryActivity，这里
// 会得到 500 或其他截断值。
func TestQueryStatsByDayCountsBeyondActivityLimit(t *testing.T) {
	srv, st := newTestServer(t)
	c := loginAs(t, srv, st, "张三", false)
	bid := mustBindingFor(t, st, "张三", "小号", "123")
	grantEventRead(t, st, "张三", "小号", "123")

	const n = 620
	rows := make([]store.ActivityRow, n)
	for i := range rows {
		rows[i] = store.ActivityRow{Kind: store.ActivityEvent, EventType: "danmaku",
			OccurredAt: seedTime.Add(time.Duration(i) * time.Second)}
	}
	seedActivity(t, st, bid, rows...)

	resp := jsonRequest(t, c, "GET", srv.URL+"/api/bindings/"+itoa(bid)+"/stats?by=day", "")
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("状态码 = %d", resp.StatusCode)
	}
	var got []map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&got); err != nil {
		t.Fatalf("解析报错: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("bucket 数 = %d, 期望 1: %+v", len(got), got)
	}
	if got[0]["danmakuCount"].(float64) != float64(n) {
		t.Errorf("danmakuCount = %v, 期望恰好 %d（不是 500 或其他截断值）", got[0]["danmakuCount"], n)
	}
}

// ---- P6 任务 5：今日电池到账（giftCoins）+ 礼物明细列表 ----

// TestQueryStatsByDayGiftCoins 验证 giftCoins 字段经完整 HTTP 层下发，
// 且银瓜子礼物（coin_type=silver）即便金额字段是正数也不该计入——判据
// 是"电池价值（Price*Count）是不是 0"，银瓜子是唯一确定恒为 0 的情形，
// 其余 SQL 层已经用"排除法"（不是"只有 gold 才算"）钉住，见
// store/stats_test.go；Price 与 TotalCoin 在盲盒场景下会分叉的细节也在
// store 层测试覆盖，这里只验证 HTTP 层字段下发。
func TestQueryStatsByDayGiftCoins(t *testing.T) {
	srv, st := newTestServer(t)
	c := loginAs(t, srv, st, "张三", false)
	bid := mustBindingFor(t, st, "张三", "小号", "123")
	grantEventRead(t, st, "张三", "小号", "123")

	seedActivity(t, st, bid,
		store.ActivityRow{Kind: store.ActivityEvent, EventType: "gift",
			Detail:     []byte(`{"GiftName":"辣条","Count":1,"CoinType":"gold","Price":50000,"TotalCoin":50000,"BlindBox":null}`),
			OccurredAt: seedTime},
		store.ActivityRow{Kind: store.ActivityEvent, EventType: "gift",
			Detail:     []byte(`{"GiftName":"小心心","Count":1,"CoinType":"silver","Price":100,"TotalCoin":100,"BlindBox":null}`),
			OccurredAt: seedTime.Add(time.Minute)},
	)

	resp := jsonRequest(t, c, "GET", srv.URL+"/api/bindings/"+itoa(bid)+"/stats?by=day", "")
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("状态码 = %d", resp.StatusCode)
	}

	var got []map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&got); err != nil {
		t.Fatalf("解析报错: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("bucket 数 = %d, 期望 1: %+v", len(got), got)
	}
	if got[0]["giftCoins"].(float64) != 50000 {
		t.Errorf("giftCoins = %v, 期望 50000（免费礼物不计入，即便金额字段是正数）", got[0]["giftCoins"])
	}
}

// TestHandleGiftBreakdownGroupsByName 验证礼物明细列表接口：按礼物名
// 分组，数量与电池数（银瓜子记 0，其余按 Price*Count 计）各自求和。
func TestHandleGiftBreakdownGroupsByName(t *testing.T) {
	srv, st := newTestServer(t)
	c := loginAs(t, srv, st, "张三", false)
	bid := mustBindingFor(t, st, "张三", "小号", "123")
	grantEventRead(t, st, "张三", "小号", "123")

	seedActivity(t, st, bid,
		store.ActivityRow{Kind: store.ActivityEvent, EventType: "gift",
			Detail:     []byte(`{"GiftName":"辣条","Count":1,"CoinType":"gold","Price":50000,"TotalCoin":50000,"BlindBox":null}`),
			OccurredAt: seedTime},
		store.ActivityRow{Kind: store.ActivityEvent, EventType: "gift",
			Detail:     []byte(`{"GiftName":"辣条","Count":2,"CoinType":"gold","Price":50000,"TotalCoin":100000,"BlindBox":null}`),
			OccurredAt: seedTime.Add(time.Minute)},
		store.ActivityRow{Kind: store.ActivityEvent, EventType: "gift",
			Detail:     []byte(`{"GiftName":"小心心","Count":3,"CoinType":"silver","Price":100,"TotalCoin":300,"BlindBox":null}`),
			OccurredAt: seedTime.Add(2 * time.Minute)},
		// 盲盒**要**出现在明细列表里，并且带 blindBox:true 标记——真机反馈
		// 推翻了此前「明细完全排除盲盒」的行为：「当日电池到账」含盲盒，
		// 明细不含，界面上 423 电池只能加出 103，差额没有出处。P4-4 的
		// 「盲盒单独算」现在落在这一位标记上（「礼物数量/种类」两张卡片
		// 仍然不含盲盒），不再靠整行消失来实现。
		store.ActivityRow{Kind: store.ActivityEvent, EventType: "gift",
			Detail: []byte(`{"GiftName":"星光铃铛","Count":1,"CoinType":"gold","Price":5200,"TotalCoin":5000,` +
				`"BlindBox":{"Name":"幸运盲盒","Price":5000,"TipPrice":5200}}`),
			OccurredAt: seedTime.Add(3 * time.Minute)},
	)

	resp := jsonRequest(t, c, "GET", srv.URL+"/api/bindings/"+itoa(bid)+"/gifts", "")
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("状态码 = %d", resp.StatusCode)
	}

	var got []map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&got); err != nil {
		t.Fatalf("解析报错: %v", err)
	}
	if len(got) != 3 {
		t.Fatalf("明细行数 = %d, 期望 3（辣条/小心心/盲盒爆出的星光铃铛）: %+v", len(got), got)
	}

	byName := map[string]map[string]any{}
	for _, row := range got {
		byName[row["giftName"].(string)] = row
	}

	// 盲盒行必须带 blindBox:true 经完整 HTTP 层下发——前端靠这一位画「来源」
	// 列并组行键，字段一旦漏掉，同名礼物的两行会撞 row-key。
	bell, ok := byName["星光铃铛"]
	if !ok {
		t.Fatalf("盲盒爆出的「星光铃铛」没有出现在明细里: %+v", got)
	}
	if bell["blindBox"] != true {
		t.Errorf("星光铃铛 blindBox = %v, 期望 true", bell["blindBox"])
	}
	if bell["coins"].(float64) != 5200 {
		t.Errorf("星光铃铛 coins = %v, 期望 5200（Price*Count，不是盲盒售价 5000）", bell["coins"])
	}

	larou, ok := byName["辣条"]
	if !ok {
		t.Fatalf("没有找到「辣条」: %+v", got)
	}
	if larou["count"].(float64) != 3 {
		t.Errorf("辣条 count = %v, 期望 3", larou["count"])
	}
	if larou["coins"].(float64) != 150000 {
		t.Errorf("辣条 coins = %v, 期望 150000", larou["coins"])
	}

	heart, ok := byName["小心心"]
	if !ok {
		t.Fatalf("没有找到「小心心」: %+v", got)
	}
	if heart["count"].(float64) != 3 {
		t.Errorf("小心心 count = %v, 期望 3", heart["count"])
	}
	if heart["coins"].(float64) != 0 {
		t.Errorf("小心心 coins = %v, 期望 0（免费礼物不产生电池）", heart["coins"])
	}
	if larou["blindBox"] != false || heart["blindBox"] != false {
		t.Errorf("常规礼物的 blindBox 应为 false: 辣条=%v 小心心=%v",
			larou["blindBox"], heart["blindBox"])
	}
}

// TestHandleGiftBreakdownRespectsTimeRange 验证 since/until 生效，与
// /stats 接口共用同一份时间范围解析（parseActivityTimeRange）。
func TestHandleGiftBreakdownRespectsTimeRange(t *testing.T) {
	srv, st := newTestServer(t)
	c := loginAs(t, srv, st, "张三", false)
	bid := mustBindingFor(t, st, "张三", "小号", "123")
	grantEventRead(t, st, "张三", "小号", "123")

	seedActivity(t, st, bid,
		store.ActivityRow{Kind: store.ActivityEvent, EventType: "gift",
			Detail:     []byte(`{"GiftName":"昨天的礼物","Count":1,"CoinType":"gold","TotalCoin":100,"BlindBox":null}`),
			OccurredAt: seedTime.Add(-24 * time.Hour)},
		store.ActivityRow{Kind: store.ActivityEvent, EventType: "gift",
			Detail:     []byte(`{"GiftName":"今天的礼物","Count":1,"CoinType":"gold","TotalCoin":200,"BlindBox":null}`),
			OccurredAt: seedTime},
	)

	since := seedTime.Add(-time.Hour).UTC().Format(time.RFC3339)
	until := seedTime.Add(time.Hour).UTC().Format(time.RFC3339)
	resp := jsonRequest(t, c, "GET",
		srv.URL+"/api/bindings/"+itoa(bid)+"/gifts?since="+since+"&until="+until, "")
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("状态码 = %d", resp.StatusCode)
	}

	var got []map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&got); err != nil {
		t.Fatalf("解析报错: %v", err)
	}
	if len(got) != 1 || got[0]["giftName"] != "今天的礼物" {
		t.Errorf("按时间窗口过滤后 = %+v, 期望只有「今天的礼物」", got)
	}
}

// TestHandleGiftBreakdownRequiresEventRead 验证权限守卫与 /stats 一致，
// 走 event:read——手法与 TestQueryStatsRequiresEventRead 完全一致：
// 授予一个不相干的权限点（rule:write），证明"有别的权限"不等于
// "有 event:read"。
func TestHandleGiftBreakdownRequiresEventRead(t *testing.T) {
	srv, st := newTestServer(t)
	loginAs(t, srv, st, "张三", false)
	bid := mustBindingFor(t, st, "张三", "小号", "123")

	li := loginAs(t, srv, st, "李四", false)
	if err := st.Grant(context.Background(), "李四", "小号", "123",
		[]perm.Permission{perm.RuleWrite}); err != nil {
		t.Fatalf("授权报错: %v", err)
	}

	resp := jsonRequest(t, li, "GET", srv.URL+"/api/bindings/"+itoa(bid)+"/gifts", "")
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusForbidden {
		t.Errorf("状态码 = %d, 期望 403", resp.StatusCode)
	}
}
