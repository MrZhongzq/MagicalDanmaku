// Package guard 处理大航海（舰长/提督/总督）相关的运行期状态。
package guard

import (
	"sync"
	"time"
)

// Dedupe 把同一次上舰的多次广播收敛成一条。
//
// B 站对一次上舰会先后下发 GUARD_BUY 与 USER_TOAST_MSG 两个 CMD，两者
// 归一化后是内容相同的两条 guard_buy 事件（真机数据里相隔 0.3~0.6 秒）。
// 不去重的话，上舰答谢规则会被触发两次，观众会看到机器人连着感谢同一
// 个人两遍。
//
// 判定键刻意不含「是不是续费」：那个字段恰恰是靠消息来源推出来的，两条
// 广播上正好相反，拿它当键等于永远去不掉重。
type Dedupe struct {
	window time.Duration

	mu   sync.Mutex
	seen map[key]time.Time
}

type key struct {
	roomID string
	uid    string
	level  int
}

// NewDedupe 构造一个窗口为 window 的去重器。
func NewDedupe(window time.Duration) *Dedupe {
	return &Dedupe{window: window, seen: make(map[key]time.Time)}
}

// Accept 报告这条上舰事件该不该放行。
//
// 同一 (直播间, 用户, 大航海等级) 在窗口内只放行第一条。等级进入判定键，
// 是因为同一个人紧接着从舰长升提督是另一笔真实购买，不该被吞掉。
func (d *Dedupe) Accept(roomID, uid string, level int, at time.Time) bool {
	k := key{roomID: roomID, uid: uid, level: level}

	d.mu.Lock()
	defer d.mu.Unlock()

	if last, ok := d.seen[k]; ok && at.Sub(last) < d.window {
		return false
	}
	d.seen[k] = at
	d.sweepLocked(at)
	return true
}

// sweepLocked 清掉早已过窗口的条目，避免这张表随着直播间人数无限增长。
// 调用方必须持锁。
func (d *Dedupe) sweepLocked(now time.Time) {
	for k, ts := range d.seen {
		if now.Sub(ts) >= d.window {
			delete(d.seen, k)
		}
	}
}
