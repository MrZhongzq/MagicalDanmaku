package httpapi

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/MrZhongzq/MagicalDanmaku/server/internal/event"
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

// sseKeepalive 是心跳间隔，防中间代理掐断空闲连接。
const sseKeepalive = 30 * time.Second

// sseWriteTimeout 是单次写的上限。
//
// 没有它，一个卡住的客户端（TCP 接收窗口收敛到 0）会让 Fprintf
// **永远**阻塞——而 select 里的 r.Context().Done() 根本轮不到，
// 因为我们就卡在写里面。goroutine、socket、Hub 订阅者三样都收不回来。
//
// 服务器的 WriteTimeout 又必须是 0（见 server.go：不为 0 的话所有
// SSE 长连接都会被定时掐断），所以 per-write deadline 是唯一的兜底。
// fix-wave-1 给两个 ResponseWriter 包装器加 Unwrap() 就是为了这个。
const sseWriteTimeout = 10 * time.Second

// handleStream 用 SSE 推送该绑定的实时事件。
//
// 用 SSE 而非 WebSocket：需求是单向的，浏览器原生 EventSource 自带
// 断线重连，不需要维护握手协议。
func (s *Server) handleStream(w http.ResponseWriter, r *http.Request) {
	b := bindingFrom(r.Context())

	flusher, ok := w.(http.Flusher)
	if !ok {
		respondError(w, http.StatusInternalServerError, "服务器不支持流式响应")
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	// 关掉 nginx 之类反向代理的缓冲，否则事件会被攒着不发
	w.Header().Set("X-Accel-Buffering", "no")

	// rc 走 Unwrap 链下钻到真正的 ResponseWriter，用来给每次写设截止时间
	rc := http.NewResponseController(w)

	// write 是这个处理器唯一的写出口。返回 false 表示连接废了，该收摊。
	//
	// 每次写之前都重设一次截止时间——不是设一次就够。设一次的话时钟
	// 从那一刻开始一直走，一条完全正常的长连接会在 sseWriteTimeout
	// 之后被自己掐断。
	//
	// SetWriteDeadline 在不支持的底层上返回 ErrNotSupported，那种情况
	// 下没有兜底可用，但也不该因此拒绝服务，所以只忽略这一种错误。
	write := func(format string, args ...any) bool {
		if err := rc.SetWriteDeadline(time.Now().Add(sseWriteTimeout)); err != nil &&
			!errors.Is(err, http.ErrNotSupported) {
			return false
		}
		if _, err := fmt.Fprintf(w, format, args...); err != nil {
			return false
		}
		flusher.Flush()
		return true
	}

	w.WriteHeader(http.StatusOK)
	flusher.Flush()

	ch, cancel := s.hub.Subscribe(b.ID)
	defer cancel() // 客户端断开必须退订，否则每次刷新页面都泄漏一个订阅者

	ticker := time.NewTicker(sseKeepalive)
	defer ticker.Stop()

	for {
		select {
		case <-r.Context().Done():
			return

		case ev, ok := <-ch:
			if !ok {
				return
			}
			payload, err := json.Marshal(streamEventView(ev))
			if err != nil {
				s.log.Warn("序列化事件失败", "err", err)
				continue
			}
			// ev.Type 全是 event 包里的常量（未识别的 CMD 归为 TypeUnknown），
			// 不含换行，不会把 SSE 的帧结构撑破
			if !write("event: %s\ndata: %s\n\n", ev.Type, payload) {
				return
			}

		case <-ticker.C:
			// SSE 注释行，客户端会忽略它，但能让中间代理知道连接还活着
			if !write(": keepalive\n\n") {
				return
			}
		}
	}
}

// streamEventView 是推给浏览器的事件形状。
//
// **不含 Raw**：那是完整的 B 站原始 JSON，体量是 Payload 的数倍，
// 网页不需要它，推过去只是浪费带宽。
func streamEventView(ev event.Event) map[string]any {
	return map[string]any{
		"id":        ev.ID,
		"type":      string(ev.Type),
		"roomId":    ev.RoomID,
		"timestamp": ev.Timestamp.Format(timeLayout),
		"payload":   ev.Payload,
	}
}
