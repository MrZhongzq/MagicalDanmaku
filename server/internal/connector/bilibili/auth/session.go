// Package auth 负责 B 站账号会话的解析、签名与登录。
package auth

import (
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"
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
	BuVID3   string // 设备指纹，缺失会触发 -352 风控
	BuVID4   string
	BNut     string // buvid 的生成时间戳

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
	s.BuVID3 = s.pairs["buvid3"]
	s.BuVID4 = s.pairs["buvid4"]
	s.BNut = s.pairs["b_nut"]

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
func (s *Session) EnsureDeviceFields(buvid string) {
	if buvid == "" {
		return
	}
	if s.BuVID3 == "" {
		s.BuVID3 = buvid
		s.set("buvid3", buvid)
	}
	if s.BuVID4 == "" {
		// buvid4 的真实算法未公开；实测复用 buvid3 即可通过校验。
		s.BuVID4 = s.BuVID3
		s.set("buvid4", s.BuVID3)
	}
	if s.BNut == "" {
		s.BNut = strconv.FormatInt(time.Now().Unix(), 10)
		s.set("b_nut", s.BNut)
	}
}

// set 写入键值并维护顺序。
func (s *Session) set(k, v string) {
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
	if s == nil || len(s.pairs) == 0 {
		return ""
	}
	keys := s.order
	if len(keys) != len(s.pairs) {
		// 顺序信息不完整时退化为字典序，保证输出确定。
		keys = keys[:0]
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
