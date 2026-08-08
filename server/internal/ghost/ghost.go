// Package ghost 检测幽灵弹幕：发送接口返回成功、但直播间广播里收不到
// 自己那条弹幕。
package ghost

import (
	"sync"
	"time"
)

// Report 是一条幽灵弹幕的判定结果。
type Report struct {
	RoomID string
	UID    string
	Text   string
	SentAt time.Time
	Waited time.Duration
}

// Mismatch 是「回显到了但文本对不上」的判定结果。这通常意味着弹幕被
// 关键词规则改写过，比原样吞掉更有诊断价值。
type Mismatch struct {
	RoomID string
	UID    string
	Sent   string
	Echo   string
	SentAt time.Time
}

// key 是 pending 的分桶键：同一账号在不同房间是两回事。
type key struct{ roomID, uid string }

// item 是一条等待回显的登记。
type item struct {
	text   string
	sentAt time.Time
}

// Detector 是检测器。
type Detector struct {
	window time.Duration

	mu      sync.Mutex
	pending map[key][]item

	// OnGhost 在判定出幽灵弹幕时回调。
	OnGhost func(Report)

	// OnMismatch 在回显文本与发送内容不一致时回调。
	OnMismatch func(Mismatch)
}

// New 构造一个等待窗口为 window 的检测器。
func New(window time.Duration) *Detector {
	return &Detector{window: window, pending: make(map[key][]item)}
}

// Sent 登记一条已成功发出的弹幕。分段发送时每段各登记一条。
func (d *Detector) Sent(roomID, uid, text string, at time.Time) {
	d.mu.Lock()
	defer d.mu.Unlock()
	k := key{roomID, uid}
	d.pending[k] = append(d.pending[k], item{text: text, sentAt: at})
}

// Observe 用收到的一条自己的弹幕销账。
//
// 同文本可能有多条未销账的登记（轮播三分钟一条，内容会重复），按 FIFO
// 销掉最早的那条：先发的先回显。
func (d *Detector) Observe(roomID, uid, text string) {
	d.mu.Lock()
	k := key{roomID, uid}
	items := d.pending[k]

	// 没有待销账的登记：这条不是本系统发的（同一账号在手机上发言也会
	// 广播回来），与检测无关。
	if len(items) == 0 {
		d.mu.Unlock()
		return
	}

	for i, it := range items {
		if it.text == text {
			d.removeLocked(k, i)
			d.mu.Unlock()
			return
		}
	}

	// 有登记在等，回来的却是一条内容对不上的：判定为改写，并销掉最早
	// 那条。这里可能误伤——账号在别处发言、恰好本系统也有一条在等，就
	// 会被算成改写。日志里带着两边原文，人能一眼看出来。
	oldest := items[0]
	d.removeLocked(k, 0)
	d.mu.Unlock()

	if d.OnMismatch != nil {
		d.OnMismatch(Mismatch{
			RoomID: roomID,
			UID:    uid,
			Sent:   oldest.text,
			Echo:   text,
			SentAt: oldest.sentAt,
		})
	}
}

// removeLocked 摘掉 k 桶里第 i 项，桶空则删键。调用方必须持锁。
func (d *Detector) removeLocked(k key, i int) {
	items := append(d.pending[k][:i], d.pending[k][i+1:]...)
	if len(items) == 0 {
		delete(d.pending, k)
		return
	}
	d.pending[k] = items
}

// Sweep 判定超过窗口仍未销账的登记项。
func (d *Detector) Sweep(now time.Time, online func(roomID string) bool) {
	d.mu.Lock()
	var reports []Report
	for k, items := range d.pending {
		// 断线期间收不到任何回显，此时判定全是误报。整桶顺延，不判定
		// 也不丢弃——连接恢复后仍然没有回显的，下一轮照样报出来。
		if online != nil && !online(k.roomID) {
			continue
		}
		var kept []item
		for _, it := range items {
			if now.Sub(it.sentAt) < d.window {
				kept = append(kept, it)
				continue
			}
			reports = append(reports, Report{
				RoomID: k.roomID,
				UID:    k.uid,
				Text:   it.text,
				SentAt: it.sentAt,
				Waited: now.Sub(it.sentAt),
			})
		}
		if len(kept) == 0 {
			delete(d.pending, k)
			continue
		}
		d.pending[k] = kept
	}
	d.mu.Unlock()

	// 回调放在锁外：回调里若再调本检测器的方法（比如探针回调中登记新的
	// 一条），持锁调用会直接死锁。
	for _, r := range reports {
		if d.OnGhost != nil {
			d.OnGhost(r)
		}
	}
}
