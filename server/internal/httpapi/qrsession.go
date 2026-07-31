package httpapi

import (
	"sync"
	"time"
)

// qrPending 是一次待确认的扫码登录。
type qrPending struct {
	AccountName string // 扫码成功后要建或要更新的账号名
	UserID      int64  // 发起扫码的系统用户，新账号归属他
	ExpiresAt   time.Time
}

// qrSessions 是待确认扫码的内存表。
//
// 不入库：待确认的扫码是纯瞬态的，二维码本身就 3 分钟失效，
// 进程重启后重新扫一次即可，没必要为它加一张表。
type qrSessions struct {
	ttl time.Duration
	mu  sync.Mutex
	m   map[string]qrPending
}

func newQRSessions(ttl time.Duration) *qrSessions {
	return &qrSessions{ttl: ttl, m: make(map[string]qrPending)}
}

func (q *qrSessions) put(key string, p qrPending) {
	p.ExpiresAt = time.Now().Add(q.ttl)
	q.mu.Lock()
	q.m[key] = p
	q.mu.Unlock()
}

// take 读一条待确认扫码。**不删除**：轮询会被调用很多次，每次都要拿得到。
func (q *qrSessions) take(key string) (qrPending, bool) {
	q.mu.Lock()
	defer q.mu.Unlock()
	p, ok := q.m[key]
	if !ok || time.Now().After(p.ExpiresAt) {
		return qrPending{}, false
	}
	return p, true
}

func (q *qrSessions) delete(key string) {
	q.mu.Lock()
	delete(q.m, key)
	q.mu.Unlock()
}

// purgeExpired 清理过期项，返回清理条数。
func (q *qrSessions) purgeExpired() int {
	now := time.Now()
	q.mu.Lock()
	defer q.mu.Unlock()
	n := 0
	for k, p := range q.m {
		if now.After(p.ExpiresAt) {
			delete(q.m, k)
			n++
		}
	}
	return n
}
