// Package auth 负责 B 站账号会话的解析、签名与登录。
package auth

import (
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

// 会话解析错误。
var (
	ErrMissingSESSDATA = errors.New("auth: Cookie 中缺少 SESSDATA")
	ErrMissingCSRF     = errors.New("auth: Cookie 中缺少 bili_jct（csrf token）")
	ErrEmptyCookie     = errors.New("auth: Cookie 为空")
)

// Session 是一个 B 站账号会话。
//
// 与原项目不同，这里只需要用户提供 Cookie 字符串：
// csrf 从 bili_jct 解析得到，无需再让用户复制请求体。
type Session struct {
	SESSDATA string // 身份凭证
	CSRF     string // 即 bili_jct，所有写操作必需
	UID      string // 即 DedeUserID，账号自身 UID

	// mu 保护下面三个设备指纹字段的后续写入（EnsureDeviceFields）以及
	// pairs/order。PK 场景下宿主 Client 和它为每个对手另起的 Client
	// 共享同一个 *Session（同一账号登录信息，即 N+1 个 Client 对应
	// 一个 Session）：-352 是账号级风控，宿主和全部对手连接会同时
	// 命中，于是多个 goroutine 可能同时调 EnsureDeviceFields 写这些
	// 字段，同时又有别的 goroutine 在每一次 HTTP 请求里调
	// CookieHeader() 遍历读 pairs/order——不加这把锁，Go 运行时对此的
	// 反应不是偶发脏读，而是 `fatal error: concurrent map read and
	// map write`：不可 recover，整个进程直接死。
	//
	// SESSDATA/CSRF/UID 只在 ParseSession 构造时写一次、之后只读，
	// 构造完成后才会被多个 Client 共享，不需要这把锁保护。
	mu     sync.RWMutex
	buVID3 string // 设备指纹，缺失会触发 -352 风控
	buVID4 string
	bNut   string // buvid 的生成时间戳

	// pairs 保留原始 Cookie 的全部键值，以便回写时不丢字段。
	pairs map[string]string
	// order 记录键的原始顺序，保证 CookieHeader 输出稳定。
	order []string
}

// ParseSession 从 Cookie 字符串解析出会话。
func ParseSession(cookie string) (*Session, error) {
	if strings.TrimSpace(cookie) == "" {
		return nil, ErrEmptyCookie
	}

	s := &Session{pairs: make(map[string]string)}
	for _, part := range strings.Split(cookie, ";") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		i := strings.IndexByte(part, '=')
		if i <= 0 {
			continue
		}
		k := strings.TrimSpace(part[:i])
		v := strings.TrimSpace(part[i+1:])
		if _, exists := s.pairs[k]; !exists {
			s.order = append(s.order, k)
		}
		s.pairs[k] = v
	}

	s.SESSDATA = s.pairs["SESSDATA"]
	s.CSRF = s.pairs["bili_jct"]
	s.UID = s.pairs["DedeUserID"]
	s.buVID3 = s.pairs["buvid3"]
	s.buVID4 = s.pairs["buvid4"]
	s.bNut = s.pairs["b_nut"]

	if s.SESSDATA == "" {
		return nil, ErrMissingSESSDATA
	}
	if s.CSRF == "" {
		return nil, ErrMissingCSRF
	}
	return s, nil
}

// IsAnonymous 判断是否为未登录会话。
func (s *Session) IsAnonymous() bool {
	return s == nil || s.SESSDATA == "" || s.UID == "" || s.UID == "0"
}

// EnsureDeviceFields 在缺失时补齐设备指纹字段。
//
// Cookie 缺少 buvid3 会导致 getDanmuInfo 返回 -352 风控错误，
// 补齐 buvid3/buvid4/b_nut 后重试即可恢复。已有的值不会被覆盖。
//
// 整个函数持锁执行：三个字段的读-判断-写必须是一个原子操作，不能只
// 锁 setLocked 那一小段——否则两个 goroutine 都读到「空」之后各自
// 判断「需要补」，虽然最终值一样不会写错，但 setLocked 内部对
// pairs/order 的并发读写依然会触发 map 的 fatal error。
func (s *Session) EnsureDeviceFields(buvid string) {
	if buvid == "" {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.buVID3 == "" {
		s.buVID3 = buvid
		s.setLocked("buvid3", buvid)
	}
	if s.buVID4 == "" {
		// buvid4 的真实算法未公开；实测复用 buvid3 即可通过校验。
		s.buVID4 = s.buVID3
		s.setLocked("buvid4", s.buVID3)
	}
	if s.bNut == "" {
		s.bNut = strconv.FormatInt(time.Now().Unix(), 10)
		s.setLocked("b_nut", s.bNut)
	}
}

// setLocked 写入键值并维护顺序。调用方必须已经持有 s.mu 的写锁——
// 这不是本方法自己加锁，是为了让 EnsureDeviceFields 能把「判断是否
// 需要补齐」跟「实际写入」合并成一次加锁，避免两次加锁之间出现别的
// goroutine 插进来的窗口。
func (s *Session) setLocked(k, v string) {
	if s.pairs == nil {
		s.pairs = make(map[string]string)
	}
	if _, exists := s.pairs[k]; !exists {
		s.order = append(s.order, k)
	}
	s.pairs[k] = v
}

// CookieHeader 生成用于 HTTP 请求的 Cookie 头。
func (s *Session) CookieHeader() string {
	if s == nil {
		return ""
	}
	s.mu.RLock()
	defer s.mu.RUnlock()

	if len(s.pairs) == 0 {
		return ""
	}
	keys := s.order
	if len(keys) != len(s.pairs) {
		// 顺序信息不完整时退化为字典序，保证输出确定。这里必须 make 一份
		// 全新的 slice，不能用 keys[:0] 复用 s.order 的底层数组再
		// append/sort——那是在只持有读锁的情况下写共享状态，并发调用
		// CookieHeader 会互相踩（复审指出的 N-4）。当前 len(order) ==
		// len(pairs) 恒成立，这个分支本来就不可达，但"不可达"不代表
		// "可以在锁语义上是错的"，加了 RWMutex 之后这条就是明确的
		// 违规写法，照样要修。
		keys = make([]string, 0, len(s.pairs))
		for k := range s.pairs {
			keys = append(keys, k)
		}
		sort.Strings(keys)
	}

	var b strings.Builder
	for _, k := range keys {
		v, ok := s.pairs[k]
		if !ok {
			continue
		}
		if b.Len() > 0 {
			b.WriteString("; ")
		}
		fmt.Fprintf(&b, "%s=%s", k, v)
	}
	return b.String()
}

// BuVID3 线程安全地返回当前的设备指纹 buvid3。
//
// PK 场景下宿主 Client 和它为每个对手另起的 Client 共享同一个
// *Session，EnsureDeviceFields 可能在另一个 goroutine 里并发写这个
// 字段，直接读裸字段是数据竞争——这是 client.go authenticate() 唯一
// 应该用的读取方式，不要绕开它直接访问字段（字段本身已经改成不导出，
// 包外也没法绕开）。
func (s *Session) BuVID3() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.buVID3
}

// BuVID4 线程安全地返回当前的设备指纹 buvid4，语义同 BuVID3。
func (s *Session) BuVID4() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.buVID4
}

// BNut 线程安全地返回当前的 buvid 生成时间戳，语义同 BuVID3。
func (s *Session) BNut() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.bNut
}
