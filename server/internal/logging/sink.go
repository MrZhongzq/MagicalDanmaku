package logging

import (
	"encoding/json"
	"time"

	"github.com/MrZhongzq/MagicalDanmaku/server/internal/event"
	"github.com/MrZhongzq/MagicalDanmaku/server/internal/rules"
	"github.com/MrZhongzq/MagicalDanmaku/server/internal/store"
)

// loggedEventTypes 是默认入库的事件类型。
//
// 排行榜（ONLINE_RANK_UPDATE）与房间统计（ROOM_STATS_UPDATE）每 8 秒
// 一条且没有分析价值，不记；未知事件同理——它们的用途是补映射，
// 那由 magicd dump 覆盖。
//
// live_start/live_stop 反过来必须记：二者频率极低（一天几次），但是
// 统计聚合接口（GET /api/bindings/{id}/stats）划分开播场次、计算直播
// 时长的唯一依据——没有它们，by=session 无从配对，日维度的 liveSeconds
// 也无从算起。历史数据里没有这两类事件（此前一直被排除在白名单外），
// 补上白名单不会回填过去——直播时长只能从这次改动生效之后的数据算起，
// 这一点在 stats.go 与统计接口的注释里也写了一遍。
var loggedEventTypes = map[event.Type]bool{
	event.TypeDanmaku:     true,
	event.TypeSuperChat:   true,
	event.TypeGift:        true,
	event.TypeGiftCombo:   true,
	event.TypeGuardBuy:    true,
	event.TypeUserEnter:   true,
	event.TypeUserFollow:  true,
	event.TypeUserShare:   true,
	event.TypeUserLike:    true,
	event.TypeUserBlocked: true,
	event.TypeLiveStart:   true,
	event.TypeLiveStop:    true,
}

// DefaultLoggedEventTypes 返回默认入库的事件类型集合的副本。
func DefaultLoggedEventTypes() map[event.Type]bool {
	out := make(map[event.Type]bool, len(loggedEventTypes))
	for k, v := range loggedEventTypes {
		out[k] = v
	}
	return out
}

// Sink 把某个绑定的事件与动作转成业务日志行，实现 rules.ActivitySink。
//
// 每个绑定一个 Sink，共用同一个 ActivityWriter：归属 ID 在这里附上，
// 批量写入的调度只有一份。
type Sink struct {
	w         *ActivityWriter
	accountID int64
	bindingID int64
	roomID    string
	types     map[event.Type]bool
	now       func() time.Time
}

// Sink 为某个绑定创建日志接收器。
func (w *ActivityWriter) Sink(accountID, bindingID int64, roomID string) *Sink {
	return &Sink{
		w:         w,
		accountID: accountID,
		bindingID: bindingID,
		roomID:    roomID,
		types:     loggedEventTypes,
		now:       time.Now,
	}
}

// SetLoggedTypes 覆盖默认的事件类型过滤。
func (s *Sink) SetLoggedTypes(types map[event.Type]bool) {
	s.types = types
}

// RecordEvent 记录一个收到的事件。噪声类型直接丢弃，不进队列。
func (s *Sink) RecordEvent(ev event.Event) {
	if !s.types[ev.Type] {
		return
	}

	vars := rules.VarsFromEvent(ev)
	uid, _ := rules.LookupPath(vars, "user.uid")
	name, _ := rules.LookupPath(vars, "user.username")

	// Payload 而非 Raw：Raw 是完整的 B 站 JSON，体量是 Payload 的数倍，
	// 而排障场景已经由 magicd dump 覆盖。
	detail, err := json.Marshal(ev.Payload)
	if err != nil {
		detail = nil
	}

	// 取局部变量的地址而非 &s.bindingID：行会在写入器缓冲里滞留一段
	// 时间才落库，共享同一个地址的话，将来若给 Sink 加了 mutator，
	// 已入队未落库的行会追溯性地观察到新值。今天没有 mutator，
	// 这个字段安全，但代价太小，不必等出问题才修。
	bid := s.bindingID

	s.w.Enqueue(store.ActivityRow{
		AccountID:  s.accountID,
		BindingID:  &bid,
		RoomID:     s.roomID,
		Kind:       store.ActivityEvent,
		EventType:  string(ev.Type),
		UserUID:    toStr(uid),
		UserName:   toStr(name),
		Detail:     detail,
		OccurredAt: s.now(),
	})
}

// RecordAction 记录一个执行过的动作。
//
// 不做类型筛选：机器人干了什么是这份日志的核心。
func (s *Sink) RecordAction(ruleName string, a rules.Action, tr rules.Trigger, err error) {
	uid, _ := rules.LookupPath(tr.Vars, "user.uid")
	name, _ := rules.LookupPath(tr.Vars, "user.username")

	detail := map[string]any{}
	if n, ok := tr.Vars["count"]; ok {
		detail["count"] = n
	}
	if us, ok := tr.Vars["users"]; ok {
		detail["users"] = us
	}
	if len(a.Template) > 0 {
		detail["template"] = a.Template
	}
	if a.Hours > 0 {
		detail["hours"] = a.Hours
	}
	if err != nil {
		// 「为什么没发出去」正是事后要查的
		detail["error"] = err.Error()
	}

	raw, mErr := json.Marshal(detail)
	if mErr != nil {
		raw = nil
	}

	bid := s.bindingID // 见 RecordEvent 里的说明：取局部变量地址，不共享 &s.bindingID

	s.w.Enqueue(store.ActivityRow{
		AccountID:  s.accountID,
		BindingID:  &bid,
		RoomID:     s.roomID,
		Kind:       store.ActivityAction,
		EventType:  string(tr.Type),
		ActionType: string(a.Type),
		RuleName:   ruleName,
		UserUID:    toStr(uid),
		UserName:   toStr(name),
		Detail:     raw,
		OccurredAt: s.now(),
	})
}

// toStr 把 Vars 里取出的任意值转成字符串，非字符串一律当空。
func toStr(v any) string {
	s, _ := v.(string)
	return s
}
