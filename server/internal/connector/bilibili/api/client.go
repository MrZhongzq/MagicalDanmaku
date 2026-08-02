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
	// 以下三个是 PK 场景专用：查「对面」直播间的人数与大航海。
	// 接口地址与字段路径取自原 C++ 项目 bili_liveservice.cpp 里真实调用过的
	// 参数（行号见 task-5-api-research.md），不是从通用知识里猜的。
	"roomOnline":  "https://api.live.bilibili.com/xlive/web-room/v1/index/getInfoByRoom",
	"guardTotal":  "https://api.live.bilibili.com/xlive/app-room/v2/guardTab/topListNew",
	"guardOnline": "https://api.live.bilibili.com/xlive/app-room/v2/guardTab/topList",
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

// URLFor 返回命名接口的地址。
func (c *Client) URLFor(name string) string { return c.baseURLs[name] }

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
	err := c.GetJSON(ctx, c.URLFor("nav"), nil, false, &data)
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
	if err := c.GetJSON(ctx, c.URLFor("spi"), nil, false, &data); err != nil {
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
