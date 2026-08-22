// Package guard 处理大航海（舰长/提督/总督）相关的运行期状态。
package guard

import (
	"sync"
	"time"

	"github.com/MrZhongzq/MagicalDanmaku/server/internal/event"
)

// Settled 是一条等够了、可以送进规则引擎的上舰事件。
//
// Owner 是调用方自己的归属标识（本项目里是 bindingID），Merger 原样带
// 回来——缓冲是全部绑定共用一个，交还的时候得知道这条属于谁。
type Settled struct {
	Owner int64
	Event event.Event
}

// Merger 把同一次上舰的多次广播收敛成一条，并且**留下信息最全的那条**。
//
// B 站对一次上舰会先后下发 GUARD_BUY 与 USER_TOAST_MSG，归一化后是内容
// 相近的两条 guard_buy 事件（真机数据里相隔 0.3~0.6 秒）。不收敛的话，
// 上舰答谢会被触发两次。
//
// 但光收敛还不够：**两条广播携带的信息不对等**。只有 USER_TOAST_MSG 带
// op_type（1 开通 / 2 续费），GUARD_BUY 在两种情况下逐字段一致，连 price
// 都同样报 198000（2026-08-22 真机对照，续费实际扣的是 168000）。先到先得
// 恰好留下没有信息的那条，于是续费一律被说成「开通」。
//
// 所以这里的规则是「等更全的那条，但不无限等」：
//
//   - 带 op_type 的（RenewKnown）立刻放行，并顶掉缓冲里那条
//   - 不带的先押在缓冲里，最多等 wait
//   - 窗口内没等到更全的，就把押着的那条发出去——不能因为等一条可能永远
//     不来的消息，把答谢整个吞掉
//
// 真机时序下，放行时刻就是 USER_TOAST_MSG 到达的那一刻（约 0.5 秒），
// 并不比原来的先到先得慢多少。
//
// 判定键是 (直播间, 用户, 大航海等级)，刻意不含「是不是续费」：那正是
// 两条广播不一致的字段，拿它当键等于永远收敛不掉。等级要进键，是因为
// 同一个人紧接着从舰长升提督是另一笔真实购买。
//
// 本类型不起 goroutine、不看时钟：时间由调用方传进来，推进靠调用方定时
// 调 Sweep。理由与 internal/ghost 一致——可测试，且生命周期归调用方管。
type Merger struct {
	// wait 是「等更全那条」的最长时间。
	wait time.Duration
	// suppress 是同一次上舰放行之后，继续压掉它其余广播的时长。
	// 与 wait 分开：等待要短（拖慢答谢），压制要长（迟到的重复广播
	// 仍然得压住）。
	suppress time.Duration

	mu      sync.Mutex
	pending map[key]held
	emitted map[key]time.Time
}

type key struct {
	roomID string
	uid    string
	level  int
}

type held struct {
	owner    int64
	ev       event.Event
	deadline time.Time
}

// NewMerger 构造收敛器。wait 是等更全那条的最长时间，suppress 是放行后
// 压制同一次上舰其余广播的时长。
func NewMerger(wait, suppress time.Duration) *Merger {
	return &Merger{
		wait:     wait,
		suppress: suppress,
		pending:  make(map[key]held),
		emitted:  make(map[key]time.Time),
	}
}

// Offer 收下一条事件，返回**现在就该送进引擎**的那条。
//
// 非上舰事件原样立刻放行，调用方不必在外面先判类型。
func (m *Merger) Offer(owner int64, ev event.Event, at time.Time) (event.Event, bool) {
	if ev.Type != event.TypeGuardBuy {
		return ev, true
	}
	g, ok := ev.Payload.(event.GuardBuy)
	if !ok || g.User.UID == "" {
		// 载荷不是预期的形状，或者连是谁都不知道——没有收敛的依据，
		// 放行。宁可偶尔多答谢一次，也不要因为解析口径变化把答谢吞掉。
		return ev, true
	}

	k := key{roomID: ev.RoomID, uid: g.User.UID, level: g.GuardLevel}

	m.mu.Lock()
	defer m.mu.Unlock()

	if last, ok := m.emitted[k]; ok && at.Sub(last) < m.suppress {
		// 这一次上舰已经发过了，这是它的另一条广播。
		return event.Event{}, false
	}

	if g.RenewKnown {
		delete(m.pending, k)
		m.emitted[k] = at
		m.sweepMapsLocked(at)
		return ev, true
	}

	if _, ok := m.pending[k]; ok {
		// 已经押着一条同样没有 op_type 的，保留先到的那条。
		return event.Event{}, false
	}
	m.pending[k] = held{owner: owner, ev: ev, deadline: at.Add(m.wait)}
	m.sweepMapsLocked(at)
	return event.Event{}, false
}

// Sweep 交出已经等够、没等到更全那条的事件。调用方定时驱动。
func (m *Merger) Sweep(now time.Time) []Settled {
	m.mu.Lock()
	defer m.mu.Unlock()

	var out []Settled
	for k, h := range m.pending {
		if now.Before(h.deadline) {
			continue
		}
		delete(m.pending, k)
		m.emitted[k] = now
		out = append(out, Settled{Owner: h.owner, Event: h.ev})
	}
	m.sweepMapsLocked(now)
	return out
}

// sweepMapsLocked 清掉早已过窗口的已发记录，避免这张表随着直播间人数
// 无限增长。调用方必须持锁。
//
// pending 不在这里清：它自己带 deadline，由 Sweep 负责交出去，在这里
// 顺手删掉等于把一次该发的答谢悄悄吃掉。
func (m *Merger) sweepMapsLocked(now time.Time) {
	for k, ts := range m.emitted {
		if now.Sub(ts) >= m.suppress {
			delete(m.emitted, k)
		}
	}
}
