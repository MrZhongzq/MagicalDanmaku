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
		store.ActivityRow{Kind: store.ActivityEvent, EventType: "gift",
			Detail: []byte(`{"GiftName":"辣条"}`), OccurredAt: seedTime},
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
