package httpapi

import (
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/MrZhongzq/MagicalDanmaku/server/internal/store"
)

// maxActivityLimit 是单次查询的硬上限。
//
// 不设上限就是全表扫：一个活跃房间一天几万行。客户端传再大也截到这里。
const maxActivityLimit = 500

// activityView 是一条业务日志对外的表示。
type activityView struct {
	ID         int64           `json:"id"`
	Kind       string          `json:"kind"`
	EventType  string          `json:"eventType"`
	ActionType string          `json:"actionType"`
	RuleName   string          `json:"ruleName"`
	UserUID    string          `json:"userUid"`
	UserName   string          `json:"userName"`
	Detail     json.RawMessage `json:"detail"`
	OccurredAt string          `json:"occurredAt"`
}

func toActivityView(rec *store.ActivityRecord) activityView {
	detail := rec.Detail
	if len(detail) == 0 {
		detail = json.RawMessage("null")
	}
	return activityView{
		ID:         rec.ID,
		Kind:       string(rec.Kind),
		EventType:  rec.EventType,
		ActionType: rec.ActionType,
		RuleName:   rec.RuleName,
		UserUID:    rec.UserUID,
		UserName:   rec.UserName,
		Detail:     detail,
		OccurredAt: rec.OccurredAt.Format(timeLayout),
	}
}

func (s *Server) handleQueryActivity(w http.ResponseWriter, r *http.Request) {
	b := bindingFrom(r.Context())
	q := store.ActivityQuery{BindingID: b.ID}

	params := r.URL.Query()
	if v := params.Get("kind"); v != "" {
		if v != string(store.ActivityEvent) && v != string(store.ActivityAction) {
			respondError(w, http.StatusUnprocessableEntity,
				"kind 只能是 event 或 action，实际 %q", v)
			return
		}
		q.Kind = store.ActivityKind(v)
	}
	// eventType 刻意不做白名单校验。事件类型是开放集合——P0 的 CMD
	// 注册表对未知消息有兜底，新的 B 站消息类型随时会出现在日志里。
	// 拿一份固定清单去校验，只会让新类型查不出来
	q.EventType = params.Get("eventType")

	for _, f := range []struct {
		name string
		dst  *time.Time
	}{{"since", &q.Since}, {"until", &q.Until}} {
		if v := params.Get(f.name); v != "" {
			t, err := time.Parse(time.RFC3339, v)
			if err != nil {
				respondError(w, http.StatusUnprocessableEntity,
					"%s 必须是 RFC3339 时间，例如 2026-07-31T20:00:00Z", f.name)
				return
			}
			*f.dst = t
		}
	}
	// 时间填反了要说出来。静默返回空在日志页里和「这段时间真的没有
	// 日志」长得一模一样，用户会往错的方向查
	if !q.Since.IsZero() && !q.Until.IsZero() && q.Since.After(q.Until) {
		respondError(w, http.StatusUnprocessableEntity,
			"since 不能晚于 until（since=%s, until=%s）",
			q.Since.Format(time.RFC3339), q.Until.Format(time.RFC3339))
		return
	}

	q.Limit = maxActivityLimit
	if v := params.Get("limit"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n <= 0 {
			respondError(w, http.StatusUnprocessableEntity, "limit 必须是正整数")
			return
		}
		if n > maxActivityLimit {
			n = maxActivityLimit
		}
		q.Limit = n
	}

	recs, err := s.store.QueryActivity(r.Context(), q)
	if err != nil {
		respondStoreError(w, err, "")
		return
	}
	out := make([]activityView, 0, len(recs))
	for i := range recs {
		out = append(out, toActivityView(&recs[i]))
	}
	respondJSON(w, http.StatusOK, out)
}
