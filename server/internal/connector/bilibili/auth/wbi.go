package auth

import (
	"crypto/md5"
	"encoding/hex"
	"errors"
	"fmt"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
)

// ErrNoMixinKey 表示尚未获取到 wbi 混淆密钥。
var ErrNoMixinKey = errors.New("auth: 尚未获取 wbi mixinKey，请先调用 nav 接口")

// MixinKeyEncTab 是 wbi 签名的字符重排表，由 B 站前端硬编码。
var MixinKeyEncTab = [64]int{
	46, 47, 18, 2, 53, 8, 23, 32, 15, 50, 10, 31, 58, 3, 45, 35,
	27, 43, 5, 49, 33, 9, 42, 19, 29, 28, 14, 39, 12, 38, 41, 13,
	37, 48, 7, 16, 24, 55, 40, 61, 26, 17, 0, 1, 60, 51, 30, 4,
	22, 25, 54, 21, 56, 59, 6, 63, 57, 62, 11, 36, 20, 34, 44, 52,
}

// wbiNameRe 从形如 .../ab12....ef.png 的地址中提取 32 位文件名。
var wbiNameRe = regexp.MustCompile(`/([0-9a-fA-F]{32})\.png`)

// forbiddenInValue 是签名前必须从参数值中剔除的字符。
const forbiddenInValue = "!'()*"

// DeriveMixinKey 根据 nav 接口返回的两个图片地址推导 mixinKey。
func DeriveMixinKey(imgURL, subURL string) (string, error) {
	img := wbiNameRe.FindStringSubmatch(imgURL)
	if img == nil {
		return "", fmt.Errorf("auth: 无法从 img_url 提取 wbi 文件名: %s", imgURL)
	}
	sub := wbiNameRe.FindStringSubmatch(subURL)
	if sub == nil {
		return "", fmt.Errorf("auth: 无法从 sub_url 提取 wbi 文件名: %s", subURL)
	}

	concat := img[1] + sub[1] // 恰好 64 字符
	out := make([]byte, 0, 32)
	for i := 0; i < 32; i++ {
		out = append(out, concat[MixinKeyEncTab[i]])
	}
	return string(out), nil
}

// Signer 持有 mixinKey 并负责为请求参数签名。并发安全。
type Signer struct {
	mu          sync.RWMutex
	mixinKey    string
	refreshedAt time.Time
}

// NewSigner 创建一个尚未初始化的签名器。
func NewSigner() *Signer { return &Signer{} }

// SetMixinKey 设置密钥并记录刷新时间。
func (s *Signer) SetMixinKey(key string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.mixinKey = key
	s.refreshedAt = time.Now()
}

// NeedsRefresh 判断是否需要重新拉取 mixinKey。
// B 站的 wbi 密钥按天轮换，因此跨自然日即失效。
func (s *Signer) NeedsRefresh() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.mixinKey == "" {
		return true
	}
	now := time.Now()
	y1, m1, d1 := s.refreshedAt.Date()
	y2, m2, d2 := now.Date()
	return y1 != y2 || m1 != m2 || d1 != d2
}

// Sign 返回一份追加了 wts 与 w_rid 的新参数集合，不修改入参。
func (s *Signer) Sign(params url.Values) (url.Values, error) {
	s.mu.RLock()
	key := s.mixinKey
	s.mu.RUnlock()

	if key == "" {
		return nil, ErrNoMixinKey
	}

	// 复制并过滤参数值中的保留字符。
	signed := make(url.Values, len(params)+2)
	for k, vs := range params {
		for _, v := range vs {
			signed.Add(k, stripForbidden(v))
		}
	}
	signed.Set("wts", strconv.FormatInt(time.Now().Unix(), 10))

	// url.Values.Encode 已按 key 字典序排序并做 URL 编码。
	sum := md5.Sum([]byte(signed.Encode() + key))
	signed.Set("w_rid", hex.EncodeToString(sum[:]))
	return signed, nil
}

// stripForbidden 剔除 wbi 签名不允许出现在参数值中的字符。
func stripForbidden(v string) string {
	if !strings.ContainsAny(v, forbiddenInValue) {
		return v
	}
	return strings.Map(func(r rune) rune {
		if strings.ContainsRune(forbiddenInValue, r) {
			return -1
		}
		return r
	}, v)
}
