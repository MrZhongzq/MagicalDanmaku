package auth

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// 默认的扫码登录接口地址。
const (
	defaultGenerateURL = "https://passport.bilibili.com/x/passport-login/web/qrcode/generate"
	defaultPollURL     = "https://passport.bilibili.com/x/passport-login/web/qrcode/poll"
)

// PollStatus 是扫码登录的轮询状态。
type PollStatus string

// 全部轮询状态。
const (
	PollWaiting PollStatus = "waiting" // 等待扫码
	PollScanned PollStatus = "scanned" // 已扫码，等待手机端确认
	PollExpired PollStatus = "expired" // 二维码已失效
	PollSuccess PollStatus = "success" // 登录成功
)

// B 站轮询接口的业务状态码。
const (
	codeWaiting = 86101
	codeScanned = 86090
	codeExpired = 86038
	codeSuccess = 0
)

// qrUserAgent 是扫码登录请求使用的浏览器标识。
const qrUserAgent = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 " +
	"(KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36"

// QRCode 是一个待扫描的登录二维码。
type QRCode struct {
	Key string // 轮询用的 qrcode_key
	URL string // 需要被编码进二维码的登录地址
}

// PollResult 是一次轮询的结果。
type PollResult struct {
	Status PollStatus
	Cookie string // 仅在 PollSuccess 时非空
}

// QRLogin 实现扫码登录流程。
type QRLogin struct {
	hc          *http.Client
	generateURL string
	pollURL     string
	userAgent   string
}

// NewQRLogin 创建扫码登录器。hc 为 nil 时使用带 15 秒超时的默认客户端。
func NewQRLogin(hc *http.Client) *QRLogin {
	if hc == nil {
		hc = &http.Client{Timeout: 15 * time.Second}
	}
	// 必须禁用自动跳转，否则登录成功时的 Set-Cookie 会丢失。
	hc.CheckRedirect = func(*http.Request, []*http.Request) error {
		return http.ErrUseLastResponse
	}
	return &QRLogin{
		hc:          hc,
		generateURL: defaultGenerateURL,
		pollURL:     defaultPollURL,
		userAgent:   qrUserAgent,
	}
}

// SetGenerateURL 覆盖生成接口地址，供测试使用。
func (l *QRLogin) SetGenerateURL(u string) { l.generateURL = u }

// SetPollURL 覆盖轮询接口地址，供测试使用。
func (l *QRLogin) SetPollURL(u string) { l.pollURL = u }

// Generate 申请一个登录二维码。
func (l *QRLogin) Generate(ctx context.Context) (*QRCode, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, l.generateURL, nil)
	if err != nil {
		return nil, fmt.Errorf("auth: 构造请求失败: %w", err)
	}
	req.Header.Set("User-Agent", l.userAgent)

	resp, err := l.hc.Do(req)
	if err != nil {
		return nil, fmt.Errorf("auth: 申请二维码失败: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, fmt.Errorf("auth: 读取响应失败: %w", err)
	}

	var env struct {
		Code int `json:"code"`
		Data struct {
			URL       string `json:"url"`
			QRCodeKey string `json:"qrcode_key"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &env); err != nil {
		return nil, fmt.Errorf("auth: 解析响应失败: %w", err)
	}
	if env.Code != 0 || env.Data.QRCodeKey == "" {
		return nil, fmt.Errorf("auth: 申请二维码被拒绝, code=%d", env.Code)
	}
	return &QRCode{Key: env.Data.QRCodeKey, URL: env.Data.URL}, nil
}

// Poll 轮询一次扫码状态。
func (l *QRLogin) Poll(ctx context.Context, key string) (*PollResult, error) {
	q := url.Values{}
	q.Set("qrcode_key", key)

	sep := "?"
	if strings.Contains(l.pollURL, "?") {
		sep = "&"
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, l.pollURL+sep+q.Encode(), nil)
	if err != nil {
		return nil, fmt.Errorf("auth: 构造请求失败: %w", err)
	}
	req.Header.Set("User-Agent", l.userAgent)

	resp, err := l.hc.Do(req)
	if err != nil {
		return nil, fmt.Errorf("auth: 轮询失败: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, fmt.Errorf("auth: 读取响应失败: %w", err)
	}

	var env struct {
		Code int `json:"code"`
		Data struct {
			Code    int    `json:"code"`
			Message string `json:"message"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &env); err != nil {
		return nil, fmt.Errorf("auth: 解析响应失败: %w", err)
	}

	switch env.Data.Code {
	case codeWaiting:
		return &PollResult{Status: PollWaiting}, nil
	case codeScanned:
		return &PollResult{Status: PollScanned}, nil
	case codeExpired:
		return &PollResult{Status: PollExpired}, nil
	case codeSuccess:
		return &PollResult{Status: PollSuccess, Cookie: collectCookies(resp)}, nil
	default:
		return nil, fmt.Errorf("auth: 未知的轮询状态 code=%d: %s", env.Data.Code, env.Data.Message)
	}
}

// collectCookies 把响应中的 Set-Cookie 合并成 Cookie 头格式。
func collectCookies(resp *http.Response) string {
	var b strings.Builder
	for _, ck := range resp.Cookies() {
		if ck.Name == "" || ck.Value == "" {
			continue
		}
		if b.Len() > 0 {
			b.WriteString("; ")
		}
		fmt.Fprintf(&b, "%s=%s", ck.Name, ck.Value)
	}
	return b.String()
}
