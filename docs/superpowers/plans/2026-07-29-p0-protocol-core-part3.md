# P0 协议内核 Implementation Plan · Part 3（认证与 HTTP）

> 续 `2026-07-29-p0-protocol-core-part2.md`。执行前请先完成 Task 1–10。
> Global Constraints 沿用 Part 1，此处不再重复。

本篇覆盖 Task 11–13：会话与 Cookie、wbi 签名与 buvid、HTTP 客户端与房间接口。

---

### Task 11: 会话与 Cookie 解析

**Files:**
- Create: `server/internal/connector/bilibili/auth/session.go`
- Test: `server/internal/connector/bilibili/auth/session_test.go`

**Interfaces:**
- Consumes: 无（独立于前面的任务）
- Produces:
  - `auth.Session{Cookie, SESSDATA, CSRF, UID, BuVID3, BuVID4, BNut string}`
  - `auth.ParseSession(cookie string) (*Session, error)`
  - `auth.ErrMissingSESSDATA`、`auth.ErrMissingCSRF`
  - `(*Session).IsAnonymous() bool`
  - `(*Session).EnsureDeviceFields(buvid string)` — 补齐 buvid3/buvid4/b_nut 并重建 Cookie 串
  - `(*Session).CookieHeader() string`

**背景：** 原项目要求用户从浏览器 devtools 里复制**两样东西**——Cookie 字符串和发弹幕请求的原始 POST body（`browserData`），然后靠字符串裁剪去改写后者。新设计只需 Cookie，csrf 从 `bili_jct` 解析得到。

**风控相关：** Cookie 缺少 `buvid3` 会导致 `getDanmuInfo` 返回 -352。补齐 `buvid3`/`buvid4`/`b_nut` 可恢复（原项目 `bili_liveservice.cpp:369-441`）。

- [ ] **Step 1: 写失败测试**

创建 `server/internal/connector/bilibili/auth/session_test.go`：

```go
package auth

import (
	"errors"
	"strings"
	"testing"
)

const fullCookie = "buvid3=DE04FB9D-9A3C-09E7-3B1E-A0FBF55CE628infoc; " +
	"b_nut=1700000000; SESSDATA=abc%2Cdef%2Cghi; bili_jct=deadbeefcafe; " +
	"DedeUserID=20285041; DedeUserID__ckMd5=1234567890abcdef"

func TestParseSessionFull(t *testing.T) {
	s, err := ParseSession(fullCookie)
	if err != nil {
		t.Fatalf("ParseSession 失败: %v", err)
	}
	if s.SESSDATA != "abc%2Cdef%2Cghi" {
		t.Errorf("SESSDATA = %q", s.SESSDATA)
	}
	if s.CSRF != "deadbeefcafe" {
		t.Errorf("CSRF = %q", s.CSRF)
	}
	if s.UID != "20285041" {
		t.Errorf("UID = %q", s.UID)
	}
	if s.BuVID3 != "DE04FB9D-9A3C-09E7-3B1E-A0FBF55CE628infoc" {
		t.Errorf("BuVID3 = %q", s.BuVID3)
	}
	if s.BNut != "1700000000" {
		t.Errorf("BNut = %q", s.BNut)
	}
	if s.IsAnonymous() {
		t.Error("完整 Cookie 不应判定为匿名")
	}
}

func TestParseSessionToleratesSpacingAndTrailingSemicolon(t *testing.T) {
	s, err := ParseSession("  SESSDATA=xyz ;bili_jct=tok;  DedeUserID=42;  ")
	if err != nil {
		t.Fatalf("ParseSession 失败: %v", err)
	}
	if s.SESSDATA != "xyz" || s.CSRF != "tok" || s.UID != "42" {
		t.Errorf("解析结果错误: %+v", s)
	}
}

func TestParseSessionRejectsMissingSESSDATA(t *testing.T) {
	_, err := ParseSession("bili_jct=tok; DedeUserID=42")
	if !errors.Is(err, ErrMissingSESSDATA) {
		t.Errorf("err = %v, 期望 ErrMissingSESSDATA", err)
	}
}

func TestParseSessionRejectsMissingCSRF(t *testing.T) {
	_, err := ParseSession("SESSDATA=xyz; DedeUserID=42")
	if !errors.Is(err, ErrMissingCSRF) {
		t.Errorf("err = %v, 期望 ErrMissingCSRF", err)
	}
}

func TestParseSessionRejectsEmpty(t *testing.T) {
	if _, err := ParseSession("   "); err == nil {
		t.Error("空 Cookie 应当报错")
	}
}

func TestEnsureDeviceFieldsAddsMissing(t *testing.T) {
	s, err := ParseSession("SESSDATA=xyz; bili_jct=tok; DedeUserID=42")
	if err != nil {
		t.Fatalf("ParseSession 失败: %v", err)
	}
	if s.BuVID3 != "" {
		t.Fatalf("前置条件错误，BuVID3 应为空")
	}

	s.EnsureDeviceFields("NEW-BUVID-VALUE")

	if s.BuVID3 != "NEW-BUVID-VALUE" {
		t.Errorf("BuVID3 = %q", s.BuVID3)
	}
	if s.BuVID4 != "NEW-BUVID-VALUE" {
		t.Errorf("BuVID4 = %q，缺失时应与 buvid3 同值", s.BuVID4)
	}
	if s.BNut == "" {
		t.Error("BNut 应被填上当前时间戳")
	}

	h := s.CookieHeader()
	for _, want := range []string{"SESSDATA=xyz", "bili_jct=tok", "DedeUserID=42",
		"buvid3=NEW-BUVID-VALUE", "buvid4=NEW-BUVID-VALUE", "b_nut="} {
		if !strings.Contains(h, want) {
			t.Errorf("CookieHeader 缺少 %q，实际 %q", want, h)
		}
	}
}

func TestEnsureDeviceFieldsKeepsExisting(t *testing.T) {
	s, err := ParseSession(fullCookie)
	if err != nil {
		t.Fatalf("ParseSession 失败: %v", err)
	}
	orig := s.BuVID3

	s.EnsureDeviceFields("SHOULD-NOT-OVERWRITE")

	if s.BuVID3 != orig {
		t.Errorf("已有 buvid3 不应被覆盖，实际 %q", s.BuVID3)
	}
	if s.BNut != "1700000000" {
		t.Errorf("已有 b_nut 不应被覆盖，实际 %q", s.BNut)
	}
}

func TestCookieHeaderRoundTrips(t *testing.T) {
	s, err := ParseSession(fullCookie)
	if err != nil {
		t.Fatalf("ParseSession 失败: %v", err)
	}
	s2, err := ParseSession(s.CookieHeader())
	if err != nil {
		t.Fatalf("回环解析失败: %v", err)
	}
	if s2.SESSDATA != s.SESSDATA || s2.CSRF != s.CSRF || s2.UID != s.UID || s2.BuVID3 != s.BuVID3 {
		t.Errorf("回环后字段不一致:\n原始 %+v\n回环 %+v", s, s2)
	}
}
```

- [ ] **Step 2: 运行测试确认失败**

```bash
cd server && go test ./internal/connector/bilibili/auth/ -v
```

Expected: 编译失败，`undefined: ParseSession`。

- [ ] **Step 3: 实现**

创建 `server/internal/connector/bilibili/auth/session.go`：

```go
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
```

- [ ] **Step 4: 运行测试确认通过**

```bash
cd server && go test ./internal/connector/bilibili/auth/ -v
```

Expected: 八个测试全部 PASS。

- [ ] **Step 5: 提交**

```bash
git add server/internal/connector/bilibili/auth/
git commit -m "feat: 实现 Cookie 解析与会话管理"
```

---

### Task 12: wbi 签名

**Files:**
- Create: `server/internal/connector/bilibili/auth/wbi.go`
- Test: `server/internal/connector/bilibili/auth/wbi_test.go`

**Interfaces:**
- Consumes: 无
- Produces:
  - `auth.MixinKeyEncTab` — 64 元素置换表常量
  - `auth.DeriveMixinKey(imgURL, subURL string) (string, error)`
  - `auth.Signer{}`（含 `mixinKey` 与刷新日期，并发安全）
  - `auth.NewSigner() *Signer`
  - `(*Signer).SetMixinKey(key string)`
  - `(*Signer).NeedsRefresh() bool`
  - `(*Signer).Sign(params url.Values) (url.Values, error)` — 返回含 `wts` 与 `w_rid` 的新集合
  - `auth.ErrNoMixinKey`

**算法**（源自原项目 `bili_liveservice.cpp:269-333`，并补齐原实现遗漏的编码步骤）：

1. `GET https://api.bilibili.com/x/web-interface/nav` → `data.wbi_img.img_url` 与 `sub_url`
2. 从两个 URL 各提取 32 位十六进制文件名，拼成 64 字符
3. 按 `MixinKeyEncTab` 重排，取前 32 位得 `mixinKey`
4. 签名时：**过滤参数值中的 `!'()*`**，按 key 字典序排序，URL 编码为 query，追加 `wts=<unix秒>`，对 `<query>+<mixinKey>` 取 MD5 小写十六进制即 `w_rid`

> 原 C++ 实现跳过了第 4 步的字符过滤与 URL 编码，参数含中文或特殊字符时会签名失败。本实现按标准算法补齐。

- [ ] **Step 1: 写失败测试**

创建 `server/internal/connector/bilibili/auth/wbi_test.go`：

```go
package auth

import (
	"crypto/md5"
	"encoding/hex"
	"errors"
	"net/url"
	"strconv"
	"testing"
	"time"
)

func TestMixinKeyEncTabShape(t *testing.T) {
	if len(MixinKeyEncTab) != 64 {
		t.Fatalf("置换表长度 = %d, 期望 64", len(MixinKeyEncTab))
	}
	seen := make(map[int]bool, 64)
	for i, v := range MixinKeyEncTab {
		if v < 0 || v > 63 {
			t.Errorf("下标 %d 处的值 %d 越界", i, v)
		}
		if seen[v] {
			t.Errorf("值 %d 重复出现", v)
		}
		seen[v] = true
	}
	if len(seen) != 64 {
		t.Errorf("置换表应覆盖 0..63，实际只有 %d 个不同值", len(seen))
	}
}

func TestDeriveMixinKey(t *testing.T) {
	// 构造 64 个可区分的字符：前 32 位为 '0'..'9''a'..'v'，后 32 位为大写
	imgName := "0123456789abcdefghijklmnopqrstuv"
	subName := "0123456789ABCDEFGHIJKLMNOPQRSTUV"
	imgURL := "https://i0.hdslb.com/bfs/wbi/" + imgName + ".png"
	subURL := "https://i0.hdslb.com/bfs/wbi/" + subName + ".png"

	got, err := DeriveMixinKey(imgURL, subURL)
	if err != nil {
		t.Fatalf("DeriveMixinKey 失败: %v", err)
	}
	if len(got) != 32 {
		t.Fatalf("mixinKey 长度 = %d, 期望 32", len(got))
	}

	concat := imgName + subName
	want := make([]byte, 0, 32)
	for i := 0; i < 32; i++ {
		want = append(want, concat[MixinKeyEncTab[i]])
	}
	if got != string(want) {
		t.Errorf("mixinKey = %q, 期望 %q", got, string(want))
	}
}

func TestDeriveMixinKeyRejectsBadURL(t *testing.T) {
	if _, err := DeriveMixinKey("https://example.com/notahash.png", "https://example.com/x.png"); err == nil {
		t.Error("非法 URL 应当报错")
	}
}

func TestSignProducesCorrectWRid(t *testing.T) {
	s := NewSigner()
	const key = "abcdefghijklmnopqrstuvwxyz012345"
	s.SetMixinKey(key)

	in := url.Values{}
	in.Set("id", "21452505")
	in.Set("type", "0")

	out, err := s.Sign(in)
	if err != nil {
		t.Fatalf("Sign 失败: %v", err)
	}

	wts := out.Get("wts")
	if wts == "" {
		t.Fatal("缺少 wts")
	}
	if n, err := strconv.ParseInt(wts, 10, 64); err != nil {
		t.Fatalf("wts 不是整数: %v", err)
	} else if d := time.Since(time.Unix(n, 0)); d < 0 || d > time.Minute {
		t.Errorf("wts 偏离当前时间过多: %v", d)
	}

	// 手工复算期望值
	expect := url.Values{}
	expect.Set("id", "21452505")
	expect.Set("type", "0")
	expect.Set("wts", wts)
	sum := md5.Sum([]byte(expect.Encode() + key))
	want := hex.EncodeToString(sum[:])

	if got := out.Get("w_rid"); got != want {
		t.Errorf("w_rid = %q, 期望 %q", got, want)
	}
}

func TestSignFiltersForbiddenChars(t *testing.T) {
	s := NewSigner()
	s.SetMixinKey("abcdefghijklmnopqrstuvwxyz012345")

	in := url.Values{}
	in.Set("name", "a!b'c(d)e*f")

	out, err := s.Sign(in)
	if err != nil {
		t.Fatalf("Sign 失败: %v", err)
	}
	if got := out.Get("name"); got != "abcdef" {
		t.Errorf("name = %q, 期望 abcdef（应剔除 !'()* ）", got)
	}
}

func TestSignDoesNotMutateInput(t *testing.T) {
	s := NewSigner()
	s.SetMixinKey("abcdefghijklmnopqrstuvwxyz012345")

	in := url.Values{}
	in.Set("id", "1")

	if _, err := s.Sign(in); err != nil {
		t.Fatalf("Sign 失败: %v", err)
	}
	if in.Get("wts") != "" || in.Get("w_rid") != "" {
		t.Errorf("Sign 不得修改入参，实际 %v", in)
	}
}

func TestSignWithoutKeyFails(t *testing.T) {
	s := NewSigner()
	if _, err := s.Sign(url.Values{}); !errors.Is(err, ErrNoMixinKey) {
		t.Errorf("err = %v, 期望 ErrNoMixinKey", err)
	}
}

func TestNeedsRefresh(t *testing.T) {
	s := NewSigner()
	if !s.NeedsRefresh() {
		t.Error("未设置 mixinKey 时应需要刷新")
	}
	s.SetMixinKey("abcdefghijklmnopqrstuvwxyz012345")
	if s.NeedsRefresh() {
		t.Error("刚设置后不应需要刷新")
	}
}
```

- [ ] **Step 2: 运行测试确认失败**

```bash
cd server && go test ./internal/connector/bilibili/auth/ -run 'TestMixin|TestDerive|TestSign|TestNeedsRefresh' -v
```

Expected: 编译失败，`undefined: MixinKeyEncTab`。

- [ ] **Step 3: 实现**

创建 `server/internal/connector/bilibili/auth/wbi.go`：

```go
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
```

- [ ] **Step 4: 运行测试确认通过**

```bash
cd server && go test ./internal/connector/bilibili/auth/ -v
```

Expected: 全部 PASS（含 Task 11 的测试）。

- [ ] **Step 5: 提交**

```bash
git add server/internal/connector/bilibili/auth/
git commit -m "feat: 实现 wbi 签名算法"
```

---

### Task 13: HTTP 客户端与房间接口

**Files:**
- Create: `server/internal/connector/bilibili/api/client.go`
- Create: `server/internal/connector/bilibili/api/room.go`
- Test: `server/internal/connector/bilibili/api/client_test.go`
- Test: `server/internal/connector/bilibili/api/room_test.go`

**Interfaces:**
- Consumes: Task 11 的 `auth.Session`；Task 12 的 `auth.Signer`、`auth.DeriveMixinKey`
- Produces:
  - `api.Client{}`，`api.New(sess *auth.Session, opts ...Option) *Client`
  - `api.WithHTTPClient(*http.Client) Option`、`api.WithBaseURL(name, url string) Option`
  - `(*Client).Session() *auth.Session`
  - `(*Client).GetJSON(ctx, rawURL string, params url.Values, signed bool, out any) error`
  - `(*Client).PostForm(ctx, rawURL string, form url.Values, out any) error`
  - `api.APIError{Code int, Message string}`，实现 `error`；`api.IsRiskControl(err) bool`
  - `(*Client).RefreshNav(ctx) error` — 拉取 nav 并设置 mixinKey
  - `(*Client).FetchBuVID(ctx) (string, error)`
  - `(*Client).RoomInfo(ctx, roomID string) (*RoomInfo, error)`
  - `api.RoomInfo{RoomID, ShortID, UID, Title, LiveStatus, AreaID, AreaName, ParentAreaID, ParentAreaName, Attention, Online, LiveStartTime}`
  - `(*Client).DanmuInfo(ctx, roomID string) (*DanmuInfo, error)`
  - `api.DanmuInfo{Token string, Hosts []Host}`，`api.Host{Host string, WssPort, WsPort, Port int}`，`(Host).WSSURL() string`

**风控码：** `-352` 表示风控。`IsRiskControl` 供上层判断是否需要补齐设备字段后重试。

- [ ] **Step 1: 写客户端失败测试**

创建 `server/internal/connector/bilibili/api/client_test.go`：

```go
package api

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/MrZhongzq/MagicalDanmaku/server/internal/connector/bilibili/auth"
)

const testCookie = "SESSDATA=xyz; bili_jct=tok123; DedeUserID=42"

func newTestClient(t *testing.T, h http.HandlerFunc) (*Client, *httptest.Server) {
	t.Helper()
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)

	sess, err := auth.ParseSession(testCookie)
	if err != nil {
		t.Fatalf("ParseSession 失败: %v", err)
	}
	return New(sess, WithHTTPClient(srv.Client())), srv
}

func TestGetJSONSendsCookieAndHeaders(t *testing.T) {
	var gotCookie, gotUA, gotReferer string
	c, srv := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		gotCookie = r.Header.Get("Cookie")
		gotUA = r.Header.Get("User-Agent")
		gotReferer = r.Header.Get("Referer")
		w.Write([]byte(`{"code":0,"message":"0","data":{"ok":true}}`))
	})

	var out struct {
		OK bool `json:"ok"`
	}
	if err := c.GetJSON(context.Background(), srv.URL, nil, false, &out); err != nil {
		t.Fatalf("GetJSON 失败: %v", err)
	}
	if !out.OK {
		t.Error("data 未被解出")
	}
	if !strings.Contains(gotCookie, "SESSDATA=xyz") {
		t.Errorf("Cookie = %q", gotCookie)
	}
	if gotUA == "" {
		t.Error("必须携带 User-Agent，否则易触发风控")
	}
	if !strings.Contains(gotReferer, "bilibili.com") {
		t.Errorf("Referer = %q", gotReferer)
	}
}

func TestGetJSONReturnsAPIError(t *testing.T) {
	c, srv := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"code":-101,"message":"账号未登录"}`))
	})

	err := c.GetJSON(context.Background(), srv.URL, nil, false, nil)
	if err == nil {
		t.Fatal("应当返回错误")
	}
	var apiErr *APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("错误类型 = %T, 期望 *APIError", err)
	}
	if apiErr.Code != -101 {
		t.Errorf("Code = %d", apiErr.Code)
	}
	if !strings.Contains(apiErr.Error(), "账号未登录") {
		t.Errorf("错误信息应含服务端消息，实际 %q", apiErr.Error())
	}
}

func TestIsRiskControl(t *testing.T) {
	if !IsRiskControl(&APIError{Code: -352}) {
		t.Error("-352 应判定为风控")
	}
	if IsRiskControl(&APIError{Code: -101}) {
		t.Error("-101 不应判定为风控")
	}
	if IsRiskControl(errors.New("其他错误")) {
		t.Error("非 APIError 不应判定为风控")
	}
}

func TestGetJSONSignsWhenRequested(t *testing.T) {
	var gotQuery url.Values
	c, srv := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		gotQuery = r.URL.Query()
		w.Write([]byte(`{"code":0,"data":{}}`))
	})
	c.Signer().SetMixinKey("abcdefghijklmnopqrstuvwxyz012345")

	params := url.Values{}
	params.Set("id", "123")
	if err := c.GetJSON(context.Background(), srv.URL, params, true, nil); err != nil {
		t.Fatalf("GetJSON 失败: %v", err)
	}
	if gotQuery.Get("w_rid") == "" {
		t.Error("签名请求应带 w_rid")
	}
	if gotQuery.Get("wts") == "" {
		t.Error("签名请求应带 wts")
	}
	if gotQuery.Get("id") != "123" {
		t.Errorf("原参数丢失: %v", gotQuery)
	}
}

func TestPostFormAddsCSRF(t *testing.T) {
	var gotForm url.Values
	c, srv := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		r.ParseForm()
		gotForm = r.PostForm
		w.Write([]byte(`{"code":0,"data":{}}`))
	})

	form := url.Values{}
	form.Set("msg", "你好")
	if err := c.PostForm(context.Background(), srv.URL, form, nil); err != nil {
		t.Fatalf("PostForm 失败: %v", err)
	}
	if gotForm.Get("csrf") != "tok123" {
		t.Errorf("csrf = %q", gotForm.Get("csrf"))
	}
	if gotForm.Get("csrf_token") != "tok123" {
		t.Errorf("csrf_token = %q", gotForm.Get("csrf_token"))
	}
	if gotForm.Get("msg") != "你好" {
		t.Errorf("msg = %q", gotForm.Get("msg"))
	}
}

func TestRefreshNavSetsMixinKey(t *testing.T) {
	c, srv := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"code":0,"data":{"wbi_img":{
			"img_url":"https://i0.hdslb.com/bfs/wbi/0123456789abcdef0123456789abcdef.png",
			"sub_url":"https://i0.hdslb.com/bfs/wbi/fedcba9876543210fedcba9876543210.png"
		}}}`))
	})
	c.SetBaseURL("nav", srv.URL)

	if !c.Signer().NeedsRefresh() {
		t.Fatal("前置条件错误：新客户端应需要刷新")
	}
	if err := c.RefreshNav(context.Background()); err != nil {
		t.Fatalf("RefreshNav 失败: %v", err)
	}
	if c.Signer().NeedsRefresh() {
		t.Error("刷新后不应再需要刷新")
	}
}
```

- [ ] **Step 2: 运行测试确认失败**

```bash
cd server && go test ./internal/connector/bilibili/api/ -v
```

Expected: 编译失败，`undefined: New`。

- [ ] **Step 3: 实现客户端**

创建 `server/internal/connector/bilibili/api/client.go`：

```go
// Package api 封装 B 站直播相关的 HTTP 接口调用。
package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/MrZhongzq/MagicalDanmaku/server/internal/connector/bilibili/auth"
)

// DefaultUserAgent 是默认的浏览器标识。
// 不带 UA 的请求极易触发风控，因此这是必需项而非可选项。
const DefaultUserAgent = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 " +
	"(KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36"

// 默认接口地址。测试中可通过 SetBaseURL 替换。
var defaultBaseURLs = map[string]string{
	"nav":       "https://api.bilibili.com/x/web-interface/nav",
	"spi":       "https://api.bilibili.com/x/frontend/finger/spi",
	"roomInfo":  "https://api.live.bilibili.com/room/v1/Room/get_info",
	"danmuInfo": "https://api.live.bilibili.com/xlive/web-room/v1/index/getDanmuInfo",
	"sendMsg":   "https://api.live.bilibili.com/msg/send",
	"addBlock":  "https://api.live.bilibili.com/xlive/web-ucenter/v1/banned/AddSilentUser",
	"delBlock":  "https://api.live.bilibili.com/xlive/web-ucenter/v1/banned/DelSilentUser",
}

// riskControlCode 是 B 站的风控错误码。
const riskControlCode = -352

// APIError 是 B 站接口返回的业务错误（HTTP 200 但 code 非 0）。
type APIError struct {
	Code    int
	Message string
}

func (e *APIError) Error() string {
	return fmt.Sprintf("bilibili 接口错误 code=%d: %s", e.Code, e.Message)
}

// IsRiskControl 判断错误是否为 -352 风控。
func IsRiskControl(err error) bool {
	var apiErr *APIError
	return errors.As(err, &apiErr) && apiErr.Code == riskControlCode
}

// Option 配置 Client。
type Option func(*Client)

// WithHTTPClient 替换底层 HTTP 客户端。
func WithHTTPClient(hc *http.Client) Option {
	return func(c *Client) { c.hc = hc }
}

// WithUserAgent 替换 User-Agent。
func WithUserAgent(ua string) Option {
	return func(c *Client) { c.userAgent = ua }
}

// WithBaseURL 覆盖某个命名接口的地址，主要供测试使用。
func WithBaseURL(name, rawURL string) Option {
	return func(c *Client) { c.baseURLs[name] = rawURL }
}

// Client 是带会话与签名能力的 B 站 HTTP 客户端。
type Client struct {
	hc        *http.Client
	sess      *auth.Session
	signer    *auth.Signer
	userAgent string
	baseURLs  map[string]string
}

// New 创建客户端。sess 可为 nil，表示匿名访问。
func New(sess *auth.Session, opts ...Option) *Client {
	c := &Client{
		hc:        &http.Client{Timeout: 15 * time.Second},
		sess:      sess,
		signer:    auth.NewSigner(),
		userAgent: DefaultUserAgent,
		baseURLs:  make(map[string]string, len(defaultBaseURLs)),
	}
	for k, v := range defaultBaseURLs {
		c.baseURLs[k] = v
	}
	for _, o := range opts {
		o(c)
	}
	return c
}

// Session 返回当前会话。
func (c *Client) Session() *auth.Session { return c.sess }

// Signer 返回签名器。
func (c *Client) Signer() *auth.Signer { return c.signer }

// SetBaseURL 覆盖某个命名接口的地址。
func (c *Client) SetBaseURL(name, rawURL string) { c.baseURLs[name] = rawURL }

// urlFor 返回命名接口的地址。
func (c *Client) urlFor(name string) string { return c.baseURLs[name] }

// envelope 是 B 站接口的统一响应外壳。
type envelope struct {
	Code    int             `json:"code"`
	Message string          `json:"message"`
	Data    json.RawMessage `json:"data"`
}

// decodeEnvelope 校验业务码并把 data 解到 out。out 为 nil 时只做校验。
func decodeEnvelope(body []byte, out any) error {
	var env envelope
	if err := json.Unmarshal(body, &env); err != nil {
		return fmt.Errorf("api: 响应不是合法 JSON: %w", err)
	}
	if env.Code != 0 {
		return &APIError{Code: env.Code, Message: env.Message}
	}
	if out == nil || len(env.Data) == 0 {
		return nil
	}
	if err := json.Unmarshal(env.Data, out); err != nil {
		return fmt.Errorf("api: 解析 data 失败: %w", err)
	}
	return nil
}

// setCommonHeaders 填充风控相关的必备请求头。
func (c *Client) setCommonHeaders(req *http.Request) {
	req.Header.Set("User-Agent", c.userAgent)
	req.Header.Set("Referer", "https://live.bilibili.com/")
	req.Header.Set("Origin", "https://live.bilibili.com")
	if c.sess != nil {
		if ck := c.sess.CookieHeader(); ck != "" {
			req.Header.Set("Cookie", ck)
		}
	}
}

// do 执行请求并返回响应体。
func (c *Client) do(req *http.Request) ([]byte, error) {
	resp, err := c.hc.Do(req)
	if err != nil {
		return nil, fmt.Errorf("api: 请求失败: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	if err != nil {
		return nil, fmt.Errorf("api: 读取响应失败: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("api: HTTP %d: %s", resp.StatusCode, truncate(string(body), 200))
	}
	return body, nil
}

// GetJSON 发起 GET 请求。signed 为 true 时对参数做 wbi 签名。
func (c *Client) GetJSON(ctx context.Context, rawURL string, params url.Values, signed bool, out any) error {
	if params == nil {
		params = url.Values{}
	}
	if signed {
		s, err := c.signer.Sign(params)
		if err != nil {
			return err
		}
		params = s
	}

	full := rawURL
	if q := params.Encode(); q != "" {
		sep := "?"
		if strings.Contains(rawURL, "?") {
			sep = "&"
		}
		full = rawURL + sep + q
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, full, nil)
	if err != nil {
		return fmt.Errorf("api: 构造请求失败: %w", err)
	}
	c.setCommonHeaders(req)

	body, err := c.do(req)
	if err != nil {
		return err
	}
	return decodeEnvelope(body, out)
}

// PostForm 发起表单 POST 请求，自动补上 csrf 字段。
func (c *Client) PostForm(ctx context.Context, rawURL string, form url.Values, out any) error {
	if form == nil {
		form = url.Values{}
	}
	// 复制一份，避免修改调用方的集合。
	body := make(url.Values, len(form)+2)
	for k, vs := range form {
		for _, v := range vs {
			body.Add(k, v)
		}
	}
	if c.sess != nil && c.sess.CSRF != "" {
		body.Set("csrf", c.sess.CSRF)
		body.Set("csrf_token", c.sess.CSRF)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, rawURL, strings.NewReader(body.Encode()))
	if err != nil {
		return fmt.Errorf("api: 构造请求失败: %w", err)
	}
	c.setCommonHeaders(req)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	respBody, err := c.do(req)
	if err != nil {
		return err
	}
	return decodeEnvelope(respBody, out)
}

// RefreshNav 拉取 nav 接口并推导 wbi mixinKey。
func (c *Client) RefreshNav(ctx context.Context) error {
	var data struct {
		WbiImg struct {
			ImgURL string `json:"img_url"`
			SubURL string `json:"sub_url"`
		} `json:"wbi_img"`
	}
	// nav 在未登录时返回 code=-101，但 wbi_img 仍然有效，因此忽略该错误。
	err := c.GetJSON(ctx, c.urlFor("nav"), nil, false, &data)
	var apiErr *APIError
	if err != nil && !(errors.As(err, &apiErr) && data.WbiImg.ImgURL != "") {
		return err
	}

	key, err := auth.DeriveMixinKey(data.WbiImg.ImgURL, data.WbiImg.SubURL)
	if err != nil {
		return err
	}
	c.signer.SetMixinKey(key)
	return nil
}

// FetchBuVID 从设备指纹接口获取 buvid3。
func (c *Client) FetchBuVID(ctx context.Context) (string, error) {
	var data struct {
		B3 string `json:"b_3"`
		B4 string `json:"b_4"`
	}
	if err := c.GetJSON(ctx, c.urlFor("spi"), nil, false, &data); err != nil {
		return "", err
	}
	if data.B3 == "" {
		return "", errors.New("api: 设备指纹接口未返回 b_3")
	}
	return data.B3, nil
}

// truncate 截断过长字符串，用于错误信息。
func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}
```

- [ ] **Step 4: 运行客户端测试确认通过**

```bash
cd server && go test ./internal/connector/bilibili/api/ -v
```

Expected: 六个测试全部 PASS。

- [ ] **Step 5: 写房间接口失败测试**

创建 `server/internal/connector/bilibili/api/room_test.go`：

```go
package api

import (
	"context"
	"net/http"
	"testing"
)

func TestRoomInfo(t *testing.T) {
	c, srv := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if got := r.URL.Query().Get("room_id"); got != "21452505" {
			t.Errorf("room_id = %q", got)
		}
		w.Write([]byte(`{"code":0,"data":{
			"room_id":21452505,"short_id":123,"uid":20285041,
			"title":"今天也在唱歌","live_status":1,
			"area_id":21,"area_name":"视频唱见",
			"parent_area_id":1,"parent_area_name":"娱乐",
			"attention":12345,"online":678,
			"live_time":"2026-07-29 19:00:00"
		}}`))
	})
	c.SetBaseURL("roomInfo", srv.URL)

	info, err := c.RoomInfo(context.Background(), "21452505")
	if err != nil {
		t.Fatalf("RoomInfo 失败: %v", err)
	}
	if info.RoomID != "21452505" {
		t.Errorf("RoomID = %q", info.RoomID)
	}
	if info.UID != "20285041" {
		t.Errorf("UID = %q", info.UID)
	}
	if info.Title != "今天也在唱歌" {
		t.Errorf("Title = %q", info.Title)
	}
	if info.LiveStatus != 1 {
		t.Errorf("LiveStatus = %d", info.LiveStatus)
	}
	if !info.IsLiving() {
		t.Error("live_status=1 时 IsLiving 应为 true")
	}
	if info.AreaName != "视频唱见" || info.ParentAreaName != "娱乐" {
		t.Errorf("分区 = %q/%q", info.ParentAreaName, info.AreaName)
	}
	if info.Attention != 12345 {
		t.Errorf("Attention = %d", info.Attention)
	}
}

func TestDanmuInfo(t *testing.T) {
	var sawSignature bool
	c, srv := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		sawSignature = r.URL.Query().Get("w_rid") != ""
		w.Write([]byte(`{"code":0,"data":{
			"token":"tok-abc",
			"host_list":[
				{"host":"broadcastlv.chat.bilibili.com","port":2243,"wss_port":443,"ws_port":2244},
				{"host":"hw-bj-live-comet-01.chat.bilibili.com","port":2243,"wss_port":443,"ws_port":2244}
			]
		}}`))
	})
	c.SetBaseURL("danmuInfo", srv.URL)
	c.Signer().SetMixinKey("abcdefghijklmnopqrstuvwxyz012345")

	di, err := c.DanmuInfo(context.Background(), "21452505")
	if err != nil {
		t.Fatalf("DanmuInfo 失败: %v", err)
	}
	if !sawSignature {
		t.Error("getDanmuInfo 必须使用 wbi 签名")
	}
	if di.Token != "tok-abc" {
		t.Errorf("Token = %q", di.Token)
	}
	if len(di.Hosts) != 2 {
		t.Fatalf("Hosts 数量 = %d, 期望 2", len(di.Hosts))
	}
	want := "wss://broadcastlv.chat.bilibili.com:443/sub"
	if got := di.Hosts[0].WSSURL(); got != want {
		t.Errorf("WSSURL = %q, 期望 %q", got, want)
	}
}

func TestDanmuInfoRiskControl(t *testing.T) {
	c, srv := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"code":-352,"message":"风控校验失败"}`))
	})
	c.SetBaseURL("danmuInfo", srv.URL)
	c.Signer().SetMixinKey("abcdefghijklmnopqrstuvwxyz012345")

	_, err := c.DanmuInfo(context.Background(), "21452505")
	if err == nil {
		t.Fatal("应当返回错误")
	}
	if !IsRiskControl(err) {
		t.Errorf("应判定为风控，实际 %v", err)
	}
}
```

- [ ] **Step 6: 运行测试确认失败**

```bash
cd server && go test ./internal/connector/bilibili/api/ -run 'TestRoomInfo|TestDanmuInfo' -v
```

Expected: 编译失败，`c.RoomInfo undefined`。

- [ ] **Step 7: 实现房间接口**

创建 `server/internal/connector/bilibili/api/room.go`：

```go
package api

import (
	"context"
	"fmt"
	"net/url"
	"strconv"
)

// 直播状态取值。
const (
	LiveStatusOffline = 0 // 未开播
	LiveStatusLiving  = 1 // 直播中
	LiveStatusRound   = 2 // 轮播中
)

// RoomInfo 是直播间基础信息。
type RoomInfo struct {
	RoomID         string // 真实长房间号
	ShortID        string // 短号，可能为空
	UID            string // 主播 UID
	Title          string
	LiveStatus     int
	AreaID         string
	AreaName       string
	ParentAreaID   string
	ParentAreaName string
	Attention      int64  // 粉丝数
	Online         int64  // 在线人数
	LiveStartTime  string // 开播时间，形如 "2026-07-29 19:00:00"，未开播时为 "0000-00-00 00:00:00"
}

// IsLiving 判断是否正在直播。
func (r *RoomInfo) IsLiving() bool { return r.LiveStatus == LiveStatusLiving }

// RoomInfo 获取直播间基础信息。
//
// 传入短号也可调用，返回值中的 RoomID 是真实长号，
// 后续所有操作都应使用长号。
func (c *Client) RoomInfo(ctx context.Context, roomID string) (*RoomInfo, error) {
	params := url.Values{}
	params.Set("room_id", roomID)

	var data struct {
		RoomID         int64  `json:"room_id"`
		ShortID        int64  `json:"short_id"`
		UID            int64  `json:"uid"`
		Title          string `json:"title"`
		LiveStatus     int    `json:"live_status"`
		AreaID         int64  `json:"area_id"`
		AreaName       string `json:"area_name"`
		ParentAreaID   int64  `json:"parent_area_id"`
		ParentAreaName string `json:"parent_area_name"`
		Attention      int64  `json:"attention"`
		Online         int64  `json:"online"`
		LiveTime       string `json:"live_time"`
	}
	if err := c.GetJSON(ctx, c.urlFor("roomInfo"), params, false, &data); err != nil {
		return nil, fmt.Errorf("获取直播间信息失败: %w", err)
	}

	info := &RoomInfo{
		RoomID:         strconv.FormatInt(data.RoomID, 10),
		UID:            strconv.FormatInt(data.UID, 10),
		Title:          data.Title,
		LiveStatus:     data.LiveStatus,
		AreaID:         strconv.FormatInt(data.AreaID, 10),
		AreaName:       data.AreaName,
		ParentAreaID:   strconv.FormatInt(data.ParentAreaID, 10),
		ParentAreaName: data.ParentAreaName,
		Attention:      data.Attention,
		Online:         data.Online,
		LiveStartTime:  data.LiveTime,
	}
	if data.ShortID != 0 {
		info.ShortID = strconv.FormatInt(data.ShortID, 10)
	}
	return info, nil
}

// Host 是一个弹幕长连接服务器。
type Host struct {
	Host    string `json:"host"`
	Port    int    `json:"port"`
	WssPort int    `json:"wss_port"`
	WsPort  int    `json:"ws_port"`
}

// WSSURL 返回该服务器的 WebSocket 连接地址。
func (h Host) WSSURL() string {
	port := h.WssPort
	if port == 0 {
		port = 443
	}
	return fmt.Sprintf("wss://%s:%d/sub", h.Host, port)
}

// DanmuInfo 是建立弹幕长连接所需的认证信息。
type DanmuInfo struct {
	Token string // 认证包中的 key 字段
	Hosts []Host // 可用服务器列表，按优先级排序
}

// DanmuInfo 获取弹幕长连接的 token 与服务器列表。
//
// 该接口要求 wbi 签名；Cookie 缺少 buvid3 时会返回 -352 风控错误，
// 调用方应补齐设备字段后重试一次。
func (c *Client) DanmuInfo(ctx context.Context, roomID string) (*DanmuInfo, error) {
	params := url.Values{}
	params.Set("id", roomID)
	params.Set("type", "0")

	var data struct {
		Token    string `json:"token"`
		HostList []Host `json:"host_list"`
	}
	if err := c.GetJSON(ctx, c.urlFor("danmuInfo"), params, true, &data); err != nil {
		return nil, fmt.Errorf("获取弹幕服务器信息失败: %w", err)
	}
	if len(data.HostList) == 0 {
		return nil, fmt.Errorf("api: 弹幕服务器列表为空")
	}
	return &DanmuInfo{Token: data.Token, Hosts: data.HostList}, nil
}
```

- [ ] **Step 8: 运行全部测试确认通过**

```bash
cd server && go vet ./... && go test ./... 
```

Expected: 无 vet 输出，全部 PASS。

- [ ] **Step 9: 提交**

```bash
git add server/internal/connector/bilibili/api/
git commit -m "feat: 实现 B 站 HTTP 客户端与房间接口"
```

---

**下一步：** 继续阅读 `2026-07-29-p0-protocol-core-part4.md`，实现 WebSocket 连接状态机、动作执行与 CLI。
