package httpapi

import (
	"net/http"

	"github.com/MrZhongzq/MagicalDanmaku/server/internal/store"
)

// statsView 是一行聚合统计对外的表示。
//
// 字段名与 store.StatsBucket 一一对应，只是换成 camelCase 与其余
// *View 保持一致。
type statsView struct {
	Bucket       string `json:"bucket"`
	DanmakuCount int64  `json:"danmakuCount"`
	EnterCount   int64  `json:"enterCount"`
	GiftCount    int64  `json:"giftCount"`
	GiftKinds    int64  `json:"giftKinds"`
	GuardCount   int64  `json:"guardCount"`
	LiveSeconds  int64  `json:"liveSeconds"`
}

func toStatsView(b store.StatsBucket) statsView {
	return statsView{
		Bucket:       b.Bucket,
		DanmakuCount: b.DanmakuCount,
		EnterCount:   b.EnterCount,
		GiftCount:    b.GiftCount,
		GiftKinds:    b.GiftKinds,
		GuardCount:   b.GuardCount,
		LiveSeconds:  b.LiveSeconds,
	}
}

// handleQueryStats 聚合业务日志，供 WebUI 统计页的卡片使用。
//
// 存在的理由：/activity 最多吐 500 条原始行，一个活跃房间一天几万条，
// 在 500 条上算出的数字是错的还不报错——只会显示一个看起来合理的小
// 数字。这里全部改成 SQL 侧 GROUP BY，见 store/stats.go 的说明。
//
// **liveSeconds 只从这次改动之后的数据算起**：live_start/live_stop
// 此前不在 logging/sink.go 的入库白名单里，历史数据里没有这两类事件，
// 老日子的场次算不出时长——不是 bug，是这批改动之前就已经存在的数据
// 缺口。WebUI 接这个接口时（任务 10）要把这一点提示给用户，不能让
// 一片 0 秒看起来像是接口坏了。
func (s *Server) handleQueryStats(w http.ResponseWriter, r *http.Request) {
	b := bindingFrom(r.Context())
	params := r.URL.Query()

	by := params.Get("by")
	if by == "" {
		by = string(store.StatsByDay)
	}
	if by != string(store.StatsByDay) && by != string(store.StatsBySession) {
		respondError(w, http.StatusUnprocessableEntity,
			"by 只能是 day 或 session，实际 %q", by)
		return
	}

	// since/until 的解析与校验复用 /activity 那份（activity_handler.go
	// 的 parseActivityTimeRange）：RFC3339，since 晚于 until 要 422，
	// 不能静默返回空——那样在统计卡片上会长得和「这段时间真的没数据」
	// 一模一样。这里此前自己重写了一遍逐字符相同的解析逻辑，是第三份
	// 口径，容易走岔，改成共用。
	since, until, ok := parseActivityTimeRange(w, params)
	if !ok {
		return
	}
	q := store.StatsQuery{BindingID: b.ID, Since: since, Until: until}

	var buckets []store.StatsBucket
	var err error
	switch store.StatsBy(by) {
	case store.StatsByDay:
		buckets, err = s.store.QueryStatsByDay(r.Context(), q)
	case store.StatsBySession:
		buckets, err = s.store.QueryStatsBySession(r.Context(), q)
	}
	if err != nil {
		respondStoreError(w, err, "")
		return
	}

	out := make([]statsView, 0, len(buckets))
	for _, bkt := range buckets {
		out = append(out, toStatsView(bkt))
	}
	respondJSON(w, http.StatusOK, out)
}
