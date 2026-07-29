# P0 协议内核 Implementation Plan · Part 4（连接、动作与 CLI）

> 续 `2026-07-29-p0-protocol-core-part3.md`。执行前请先完成 Task 1–13。
> Global Constraints 沿用 Part 1，此处不再重复。

本篇覆盖 Task 14–16，完成 P0 的最后交付：WebSocket 连接状态机、动作执行、
扫码登录与 `magicd probe` CLI。

---

### Task 14: Connector 接口与 WebSocket 连接状态机

**Files:**
- Create: `server/internal/connector/connector.go`
- Create: `server/internal/connector/bilibili/client.go`
- Test: `server/internal/connector/bilibili/client_test.go`
- Modify: `server/go.mod`（新增 `gorilla/websocket` 依赖）

**Interfaces:**
- Consumes: Task 2/3 的 `wire.Encode`、`wire.Split`、`wire.Frame`、`Op*`；Task 4 的 `cmdmap.Map`、`cmdmap.Context`；Task 11 的 `auth.Session`；Task 13 的 `api.Client`、`api.DanmuInfo`、`api.IsRiskControl`
- Produces:
  - `connector.State` 与常量 `StateIdle`、`StateResolving`、`StateConnecting`、`StateConnected`、`StateReconnecting`、`StateRiskControlled`、`StateClosed`
  - `connector.Connector` 接口：`Run(ctx) error`、`Events() <-chan event.Event`、`State() State`
  - `bilibili.Client`，`bilibili.NewClient(roomID string, apiClient *api.Client, opts ...ClientOption) *Client`
  - `bilibili.WithDialer(*websocket.Dialer) ClientOption`、`WithHeartbeatInterval(time.Duration) ClientOption`、`WithBackoff(min, max time.Duration) ClientOption`、`WithLogger(*slog.Logger) ClientOption`、`WithDialURLOverride(string) ClientOption`（仅测试用）

**关键行为**（每条都对应设计文档中的一条硬要求）：

| 行为 | 要求 |
|---|---|
| 心跳 | 每 30 秒发 `OpHeartbeat`，body 固定为字面量 `[object Object]`（B 站前端历史遗留，服务端至今兼容） |
| 认证 | 连上后立刻发 `OpAuth`，等 `OpAuthReply` 且 `code==0` 才算连接成功 |
| 重连退避 | 3 秒起指数增长，上限 60 秒，带随机抖动 |
| host 轮换 | 每次重连换用 `host_list` 的下一个 |
| 风控 | 遇 -352 先补齐设备字段重试一次；仍失败则进入 `StateRiskControlled` 并采用 5 分钟起步的长退避，**禁止无限快速重试** |

- [ ] **Step 1: 定义 Connector 接口**

创建 `server/internal/connector/connector.go`：

```go
// Package connector 定义直播平台接入的抽象。
package connector

import (
	"context"

	"github.com/MrZhongzq/MagicalDanmaku/server/internal/event"
)

// State 是连接状态。
type State string

// 全部连接状态。
const (
	StateIdle           State = "idle"            // 未开始
	StateResolving      State = "resolving"       // 正在获取房间与长连接信息
	StateConnecting     State = "connecting"      // 正在建立 WebSocket 并认证
	StateConnected      State = "connected"       // 已连接
	StateReconnecting   State = "reconnecting"    // 断线重连中
	StateRiskControlled State = "risk_controlled" // 触发风控，长退避中
	StateClosed         State = "closed"          // 已关闭
)

// Connector 是平台接入的唯一抽象点，一个实例对应一个直播间的事件流。
//
// 事件流是房间级的，与账号身份无关；需要账号身份的写操作定义在 Actions 中。
type Connector interface {
	// Run 阻塞运行直到 ctx 取消，内部自行处理重连。
	Run(ctx context.Context) error
	// Events 返回归一化事件流。Run 结束后该通道会被关闭。
	Events() <-chan event.Event
	// State 返回当前连接状态。
	State() State
}
```

- [ ] **Step 2: 写失败测试**

创建 `server/internal/connector/bilibili/client_test.go`：

```go
package bilibili

import (
	"context"
	"encoding/binary"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gorilla/websocket"

	"github.com/MrZhongzq/MagicalDanmaku/server/internal/connector"
	"github.com/MrZhongzq/MagicalDanmaku/server/internal/connector/bilibili/api"
	"github.com/MrZhongzq/MagicalDanmaku/server/internal/connector/bilibili/auth"
	"github.com/MrZhongzq/MagicalDanmaku/server/internal/connector/bilibili/wire"
	"github.com/MrZhongzq/MagicalDanmaku/server/internal/event"
)

// fakeServer 是一个可编程的假弹幕服务器。
type fakeServer struct {
	srv *httptest.Server

	mu       sync.Mutex
	authPkts [][]byte // 收到的认证包
	beats    int      // 收到的心跳数
	conns    int      // 累计连接次数

	// onConnected 在认证完成后调用，用于推送测试消息。
	onConnected func(c *websocket.Conn)
}

func newFakeServer(t *testing.T, onConnected func(*websocket.Conn)) *fakeServer {
	t.Helper()
	fs := &fakeServer{onConnected: onConnected}

	up := websocket.Upgrader{CheckOrigin: func(*http.Request) bool { return true }}
	fs.srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c, err := up.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer c.Close()

		fs.mu.Lock()
		fs.conns++
		fs.mu.Unlock()

		// 读认证包
		_, msg, err := c.ReadMessage()
		if err != nil {
			return
		}
		fs.mu.Lock()
		fs.authPkts = append(fs.authPkts, append([]byte(nil), msg...))
		fs.mu.Unlock()

		// 回认证成功
		c.WriteMessage(websocket.BinaryMessage,
			wire.Encode(wire.OpAuthReply, []byte(`{"code":0}`)))

		// 后台统计心跳
		go func() {
			for {
				_, m, err := c.ReadMessage()
				if err != nil {
					return
				}
				if h, err := wire.DecodeHeader(m); err == nil && h.Operation == wire.OpHeartbeat {
					fs.mu.Lock()
					fs.beats++
					fs.mu.Unlock()
				}
			}
		}()

		if fs.onConnected != nil {
			fs.onConnected(c)
		}
		// 保持连接一小段时间
		time.Sleep(300 * time.Millisecond)
	}))
	t.Cleanup(fs.srv.Close)
	return fs
}

func (fs *fakeServer) wsURL() string {
	return "ws" + strings.TrimPrefix(fs.srv.URL, "http")
}

func (fs *fakeServer) stats() (conns, beats int, auths [][]byte) {
	fs.mu.Lock()
	defer fs.mu.Unlock()
	return fs.conns, fs.beats, fs.authPkts
}

// newTestClient 构造一个指向假服务器的 Client。
func newTestClient(t *testing.T, fs *fakeServer, opts ...ClientOption) *Client {
	t.Helper()

	sess, err := auth.ParseSession("SESSDATA=x; bili_jct=tok; DedeUserID=42; buvid3=BV3")
	if err != nil {
		t.Fatalf("ParseSession 失败: %v", err)
	}

	// 假的 HTTP 接口：房间信息与弹幕信息都指向 httptest。
	httpSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.Contains(r.URL.Path, "danmu"):
			json.NewEncoder(w).Encode(map[string]any{
				"code": 0,
				"data": map[string]any{
					"token": "tok-abc",
					"host_list": []map[string]any{
						{"host": "unused", "wss_port": 443, "port": 2243, "ws_port": 2244},
					},
				},
			})
		default:
			json.NewEncoder(w).Encode(map[string]any{
				"code": 0,
				"data": map[string]any{"room_id": 21452505, "uid": 1, "live_status": 1},
			})
		}
	}))
	t.Cleanup(httpSrv.Close)

	ac := api.New(sess, api.WithHTTPClient(httpSrv.Client()))
	ac.SetBaseURL("roomInfo", httpSrv.URL+"/room")
	ac.SetBaseURL("danmuInfo", httpSrv.URL+"/danmu")
	ac.Signer().SetMixinKey("abcdefghijklmnopqrstuvwxyz012345")

	all := append([]ClientOption{
		WithDialURLOverride(fs.wsURL()),
		WithHeartbeatInterval(50 * time.Millisecond),
		WithBackoff(10*time.Millisecond, 40*time.Millisecond),
	}, opts...)

	return NewClient("21452505", ac, all...)
}

func TestClientSendsAuthPacket(t *testing.T) {
	fs := newFakeServer(t, nil)
	c := newTestClient(t, fs)

	ctx, cancel := context.WithTimeout(context.Background(), 400*time.Millisecond)
	defer cancel()

	go c.Run(ctx)
	// 等认证包抵达
	deadline := time.Now().Add(300 * time.Millisecond)
	for time.Now().Before(deadline) {
		if _, _, auths := fs.stats(); len(auths) > 0 {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}

	_, _, auths := fs.stats()
	if len(auths) == 0 {
		t.Fatal("未收到认证包")
	}

	h, err := wire.DecodeHeader(auths[0])
	if err != nil {
		t.Fatalf("认证包头解析失败: %v", err)
	}
	if h.Operation != wire.OpAuth {
		t.Errorf("Operation = %d, 期望 %d", h.Operation, wire.OpAuth)
	}

	var body struct {
		UID      int64  `json:"uid"`
		RoomID   int64  `json:"roomid"`
		ProtoVer int    `json:"protover"`
		Platform string `json:"platform"`
		Type     int    `json:"type"`
		Key      string `json:"key"`
		BuVID    string `json:"buvid"`
	}
	if err := json.Unmarshal(auths[0][wire.HeaderSize:], &body); err != nil {
		t.Fatalf("认证包体解析失败: %v", err)
	}
	if body.UID != 42 {
		t.Errorf("uid = %d, 期望 42", body.UID)
	}
	if body.RoomID != 21452505 {
		t.Errorf("roomid = %d", body.RoomID)
	}
	if body.ProtoVer != 3 {
		t.Errorf("protover = %d, 期望 3（声明支持 brotli）", body.ProtoVer)
	}
	if body.Key != "tok-abc" {
		t.Errorf("key = %q", body.Key)
	}
	if body.BuVID != "BV3" {
		t.Errorf("buvid = %q", body.BuVID)
	}
	if body.Platform != "web" {
		t.Errorf("platform = %q", body.Platform)
	}
}

func TestClientEmitsEvents(t *testing.T) {
	fs := newFakeServer(t, func(c *websocket.Conn) {
		danmaku := `{"cmd":"DANMU_MSG","info":[[0,1,25,16777215,1700000000000],"测试弹幕",[123,"测试用户",0,0,0,10000,1,""],[],[10,0,0,0],["",""],0,0,null,null,0,0,[3]]}`
		c.WriteMessage(websocket.BinaryMessage, wire.Encode(wire.OpMessage, []byte(danmaku)))
	})
	c := newTestClient(t, fs)

	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()
	go c.Run(ctx)

	select {
	case ev, ok := <-c.Events():
		if !ok {
			t.Fatal("事件通道被提前关闭")
		}
		if ev.Type != event.TypeDanmaku {
			t.Fatalf("Type = %s, 期望 %s", ev.Type, event.TypeDanmaku)
		}
		d := ev.Payload.(event.Danmaku)
		if d.Text != "测试弹幕" {
			t.Errorf("Text = %q", d.Text)
		}
		if d.User.Username != "测试用户" {
			t.Errorf("Username = %q", d.User.Username)
		}
		if ev.RoomID != "21452505" {
			t.Errorf("RoomID = %q", ev.RoomID)
		}
	case <-time.After(500 * time.Millisecond):
		t.Fatal("超时未收到事件")
	}
}

func TestClientSendsHeartbeat(t *testing.T) {
	fs := newFakeServer(t, nil)
	c := newTestClient(t, fs, WithHeartbeatInterval(30*time.Millisecond))

	ctx, cancel := context.WithTimeout(context.Background(), 400*time.Millisecond)
	defer cancel()
	go c.Run(ctx)

	time.Sleep(250 * time.Millisecond)
	_, beats, _ := fs.stats()
	if beats < 2 {
		t.Errorf("心跳数 = %d, 期望至少 2", beats)
	}
}

func TestHeartbeatPayloadIsLegacyLiteral(t *testing.T) {
	// B 站前端把 JS 对象隐式转成了字符串，服务端至今兼容这个历史遗留值。
	pkt := buildHeartbeat()
	if got := string(pkt[wire.HeaderSize:]); got != "[object Object]" {
		t.Errorf("心跳包体 = %q, 期望 %q", got, "[object Object]")
	}
	h, err := wire.DecodeHeader(pkt)
	if err != nil {
		t.Fatalf("包头解析失败: %v", err)
	}
	if h.Operation != wire.OpHeartbeat {
		t.Errorf("Operation = %d", h.Operation)
	}
}

func TestClientReconnectsAndRotatesHost(t *testing.T) {
	fs := newFakeServer(t, func(c *websocket.Conn) {
		// 立刻关闭，迫使客户端重连
		c.Close()
	})
	c := newTestClient(t, fs)

	ctx, cancel := context.WithTimeout(context.Background(), 400*time.Millisecond)
	defer cancel()
	go c.Run(ctx)

	time.Sleep(300 * time.Millisecond)
	conns, _, _ := fs.stats()
	if conns < 2 {
		t.Errorf("连接次数 = %d, 期望至少 2（应自动重连）", conns)
	}
}

func TestClientStateTransitions(t *testing.T) {
	fs := newFakeServer(t, nil)
	c := newTestClient(t, fs)

	if got := c.State(); got != connector.StateIdle {
		t.Errorf("初始状态 = %s, 期望 %s", got, connector.StateIdle)
	}

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { c.Run(ctx); close(done) }()

	deadline := time.Now().Add(400 * time.Millisecond)
	for time.Now().Before(deadline) {
		if c.State() == connector.StateConnected {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if got := c.State(); got != connector.StateConnected {
		t.Errorf("认证后状态 = %s, 期望 %s", got, connector.StateConnected)
	}

	cancel()
	<-done
	if got := c.State(); got != connector.StateClosed {
		t.Errorf("退出后状态 = %s, 期望 %s", got, connector.StateClosed)
	}
	if _, ok := <-c.Events(); ok {
		t.Error("Run 结束后事件通道应被关闭")
	}
}

func TestClientIgnoresHeartbeatReply(t *testing.T) {
	fs := newFakeServer(t, func(c *websocket.Conn) {
		body := make([]byte, 4)
		binary.BigEndian.PutUint32(body, 1234)
		c.WriteMessage(websocket.BinaryMessage, wire.Encode(wire.OpHeartbeatReply, body))
		danmaku := `{"cmd":"PREPARING","roomid":"1"}`
		c.WriteMessage(websocket.BinaryMessage, wire.Encode(wire.OpMessage, []byte(danmaku)))
	})
	c := newTestClient(t, fs)

	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()
	go c.Run(ctx)

	select {
	case ev := <-c.Events():
		// 心跳回复不应产生事件，第一个事件必须是 PREPARING
		if ev.Type != event.TypeLiveStop {
			t.Errorf("Type = %s, 期望 %s（心跳回复不得产生事件）", ev.Type, event.TypeLiveStop)
		}
	case <-time.After(500 * time.Millisecond):
		t.Fatal("超时未收到事件")
	}
}
```

- [ ] **Step 3: 运行测试确认失败**

```bash
cd server && go get github.com/gorilla/websocket@v1.5.3 && go test ./internal/connector/bilibili/ -v
```

Expected: 编译失败，`undefined: NewClient`。

- [ ] **Step 4: 实现**

创建 `server/internal/connector/bilibili/client.go`：

```go
// Package bilibili 实现 B 站直播间的事件流接入。
package bilibili

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"math/rand"
	"strconv"
	"sync"
	"time"

	"github.com/gorilla/websocket"

	"github.com/MrZhongzq/MagicalDanmaku/server/internal/connector"
	"github.com/MrZhongzq/MagicalDanmaku/server/internal/connector/bilibili/api"
	"github.com/MrZhongzq/MagicalDanmaku/server/internal/connector/bilibili/cmdmap"
	"github.com/MrZhongzq/MagicalDanmaku/server/internal/connector/bilibili/wire"
	"github.com/MrZhongzq/MagicalDanmaku/server/internal/event"
)

// 默认参数。
const (
	defaultHeartbeatInterval = 30 * time.Second
	defaultBackoffMin        = 3 * time.Second
	defaultBackoffMax        = 60 * time.Second
	riskControlBackoff       = 5 * time.Minute
	eventBufferSize          = 256
	authTimeout              = 10 * time.Second
)

// heartbeatBody 是心跳包的固定内容。
//
// 这是 B 站前端的历史遗留 bug：JS 对象被隐式转成了字符串。
// 服务端至今兼容该值，必须原样发送，不能改成合法 JSON。
const heartbeatBody = "[object Object]"

// buildHeartbeat 构造心跳包。
func buildHeartbeat() []byte {
	return wire.Encode(wire.OpHeartbeat, []byte(heartbeatBody))
}

// ClientOption 配置 Client。
type ClientOption func(*Client)

// WithHeartbeatInterval 设置心跳间隔。
func WithHeartbeatInterval(d time.Duration) ClientOption {
	return func(c *Client) { c.heartbeatInterval = d }
}

// WithBackoff 设置重连退避的下限与上限。
func WithBackoff(min, max time.Duration) ClientOption {
	return func(c *Client) { c.backoffMin, c.backoffMax = min, max }
}

// WithDialer 替换 WebSocket 拨号器。
func WithDialer(d *websocket.Dialer) ClientOption {
	return func(c *Client) { c.dialer = d }
}

// WithDialURLOverride 强制所有连接都拨向指定地址，仅供测试使用。
func WithDialURLOverride(u string) ClientOption {
	return func(c *Client) { c.dialURLOverride = u }
}

// WithLogger 替换日志器。
func WithLogger(l *slog.Logger) ClientOption {
	return func(c *Client) { c.log = l }
}

// Client 是 B 站直播间的事件流连接器，实现 connector.Connector。
type Client struct {
	roomID string
	api    *api.Client
	dialer *websocket.Dialer
	log    *slog.Logger

	heartbeatInterval time.Duration
	backoffMin        time.Duration
	backoffMax        time.Duration
	dialURLOverride   string

	events chan event.Event

	mu           sync.RWMutex
	state        connector.State
	hostIndex    int
	deviceFixed  bool // 是否已因风控补齐过设备字段
	closeEventsOnce sync.Once
}

// 确保 Client 满足 Connector 接口。
var _ connector.Connector = (*Client)(nil)

// NewClient 创建一个直播间连接器。
func NewClient(roomID string, apiClient *api.Client, opts ...ClientOption) *Client {
	c := &Client{
		roomID:            roomID,
		api:               apiClient,
		dialer:            websocket.DefaultDialer,
		log:               slog.Default(),
		heartbeatInterval: defaultHeartbeatInterval,
		backoffMin:        defaultBackoffMin,
		backoffMax:        defaultBackoffMax,
		events:            make(chan event.Event, eventBufferSize),
		state:             connector.StateIdle,
	}
	for _, o := range opts {
		o(c)
	}
	return c
}

// Events 返回归一化事件流。
func (c *Client) Events() <-chan event.Event { return c.events }

// State 返回当前连接状态。
func (c *Client) State() connector.State {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.state
}

func (c *Client) setState(s connector.State) {
	c.mu.Lock()
	c.state = s
	c.mu.Unlock()
}

// Run 阻塞运行直到 ctx 取消，内部自行处理重连。
func (c *Client) Run(ctx context.Context) error {
	defer func() {
		c.setState(connector.StateClosed)
		c.closeEventsOnce.Do(func() { close(c.events) })
	}()

	backoff := c.backoffMin
	for {
		if ctx.Err() != nil {
			return ctx.Err()
		}

		established, err := c.connectOnce(ctx)
		if ctx.Err() != nil {
			return ctx.Err()
		}

		// 曾成功建立连接说明网络与账号都正常，
		// 本次断开应视为独立事件，退避从头开始计。
		if established {
			backoff = c.backoffMin
		}

		wait := backoff
		if api.IsRiskControl(err) {
			c.setState(connector.StateRiskControlled)
			wait = riskControlBackoff
			c.log.Warn("触发风控，进入长退避", "room", c.roomID, "wait", wait, "err", err)
		} else {
			c.setState(connector.StateReconnecting)
			c.log.Info("连接断开，准备重连", "room", c.roomID, "wait", wait, "err", err)
			// 指数退避，上限封顶
			backoff *= 2
			if backoff > c.backoffMax {
				backoff = c.backoffMax
			}
		}

		// 轮换到下一个 host
		c.mu.Lock()
		c.hostIndex++
		c.mu.Unlock()

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(withJitter(wait)):
		}
	}
}

// withJitter 给等待时长叠加 ±20% 的抖动，避免多房间同时重连。
func withJitter(d time.Duration) time.Duration {
	if d <= 0 {
		return d
	}
	delta := float64(d) * 0.2
	return time.Duration(float64(d) - delta + rand.Float64()*2*delta)
}

// connectOnce 完成一次「取信息 → 连接 → 认证 → 收包」的完整生命周期，
// 直到连接断开或 ctx 取消才返回。
//
// 返回的 established 表示本次是否成功完成过认证，调用方据此决定
// 是否重置退避间隔。
func (c *Client) connectOnce(ctx context.Context) (established bool, err error) {
	c.setState(connector.StateResolving)

	if c.api.Signer().NeedsRefresh() {
		if err := c.api.RefreshNav(ctx); err != nil {
			return false, fmt.Errorf("刷新 wbi 密钥失败: %w", err)
		}
	}

	di, err := c.fetchDanmuInfo(ctx)
	if err != nil {
		return false, err
	}

	c.mu.RLock()
	idx := c.hostIndex % len(di.Hosts)
	c.mu.RUnlock()

	dialURL := di.Hosts[idx].WSSURL()
	if c.dialURLOverride != "" {
		dialURL = c.dialURLOverride
	}

	c.setState(connector.StateConnecting)
	conn, _, err := c.dialer.DialContext(ctx, dialURL, nil)
	if err != nil {
		return false, fmt.Errorf("连接弹幕服务器失败 %s: %w", dialURL, err)
	}
	defer conn.Close()

	if err := c.authenticate(conn, di.Token); err != nil {
		return false, err
	}

	c.setState(connector.StateConnected)
	c.log.Info("已连接直播间", "room", c.roomID, "host", dialURL)

	return true, c.pump(ctx, conn)
}

// fetchDanmuInfo 获取长连接信息，遇风控时补齐设备字段后重试一次。
func (c *Client) fetchDanmuInfo(ctx context.Context) (*api.DanmuInfo, error) {
	di, err := c.api.DanmuInfo(ctx, c.roomID)
	if err == nil {
		return di, nil
	}
	if !api.IsRiskControl(err) {
		return nil, err
	}

	c.mu.Lock()
	alreadyFixed := c.deviceFixed
	c.deviceFixed = true
	c.mu.Unlock()

	if alreadyFixed {
		return nil, err // 已经补过一次仍失败，不再重试
	}

	sess := c.api.Session()
	if sess == nil {
		return nil, err
	}
	buvid, ferr := c.api.FetchBuVID(ctx)
	if ferr != nil {
		return nil, err // 返回原始的风控错误
	}
	sess.EnsureDeviceFields(buvid)
	c.log.Info("已补齐设备指纹字段，重试获取长连接信息", "room", c.roomID)

	return c.api.DanmuInfo(ctx, c.roomID)
}

// authenticate 发送认证包并等待认证回复。
func (c *Client) authenticate(conn *websocket.Conn, token string) error {
	uid := int64(0)
	if sess := c.api.Session(); sess != nil {
		uid, _ = strconv.ParseInt(sess.UID, 10, 64)
	}
	roomID, _ := strconv.ParseInt(c.roomID, 10, 64)

	buvid := ""
	if sess := c.api.Session(); sess != nil {
		buvid = sess.BuVID3
	}

	body, err := json.Marshal(map[string]any{
		"uid":      uid,
		"roomid":   roomID,
		"protover": 3, // 声明支持 brotli
		"platform": "web",
		"type":     2,
		"key":      token,
		"buvid":    buvid,
	})
	if err != nil {
		return fmt.Errorf("构造认证包失败: %w", err)
	}

	if err := conn.WriteMessage(websocket.BinaryMessage, wire.Encode(wire.OpAuth, body)); err != nil {
		return fmt.Errorf("发送认证包失败: %w", err)
	}

	// 等待认证回复
	if err := conn.SetReadDeadline(time.Now().Add(authTimeout)); err != nil {
		return fmt.Errorf("设置读超时失败: %w", err)
	}
	_, msg, err := conn.ReadMessage()
	if err != nil {
		return fmt.Errorf("读取认证回复失败: %w", err)
	}
	if err := conn.SetReadDeadline(time.Time{}); err != nil {
		return fmt.Errorf("清除读超时失败: %w", err)
	}

	frames, err := wire.Split(msg)
	if err != nil {
		return fmt.Errorf("解析认证回复失败: %w", err)
	}
	for _, f := range frames {
		if f.Operation != wire.OpAuthReply {
			continue
		}
		var reply struct {
			Code int `json:"code"`
		}
		if err := json.Unmarshal(f.Body, &reply); err != nil {
			return fmt.Errorf("解析认证回复体失败: %w", err)
		}
		if reply.Code != 0 {
			return fmt.Errorf("认证被拒绝，code=%d", reply.Code)
		}
		return nil
	}
	return fmt.Errorf("未收到认证回复包")
}

// pump 启动心跳并持续收包，直到连接断开或 ctx 取消。
func (c *Client) pump(ctx context.Context, conn *websocket.Conn) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	// 心跳协程
	go func() {
		t := time.NewTicker(c.heartbeatInterval)
		defer t.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-t.C:
				if err := conn.WriteMessage(websocket.BinaryMessage, buildHeartbeat()); err != nil {
					cancel()
					return
				}
			}
		}
	}()

	// ctx 取消时打断阻塞中的 ReadMessage
	go func() {
		<-ctx.Done()
		conn.Close()
	}()

	for {
		_, msg, err := conn.ReadMessage()
		if err != nil {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			return fmt.Errorf("读取消息失败: %w", err)
		}
		c.handleMessage(ctx, msg)
	}
}

// handleMessage 解析一个 WebSocket 帧并投递其中的全部事件。
func (c *Client) handleMessage(ctx context.Context, msg []byte) {
	frames, err := wire.Split(msg)
	if err != nil {
		c.log.Warn("解析数据包失败", "room", c.roomID, "err", err)
		return
	}

	mapCtx := cmdmap.Context{RoomID: c.roomID, ReceivedAt: time.Now()}
	for _, f := range frames {
		// 心跳回复与认证回复不产生业务事件。
		if f.Operation != wire.OpMessage {
			continue
		}
		evs, err := cmdmap.Map(mapCtx, f.Body)
		if err != nil {
			c.log.Warn("映射 CMD 失败", "room", c.roomID, "err", err)
			continue
		}
		for _, ev := range evs {
			select {
			case c.events <- ev:
			case <-ctx.Done():
				return
			default:
				// 消费者跟不上时丢弃最新事件而非阻塞收包，
				// 避免拖垮整条连接。
				c.log.Warn("事件通道已满，丢弃事件", "room", c.roomID, "type", ev.Type)
			}
		}
	}
}
```

- [ ] **Step 5: 运行测试确认通过**

```bash
cd server && go mod tidy && go test ./internal/connector/... -v
```

Expected: 七个测试全部 PASS。

- [ ] **Step 6: 运行竞态检测**

```bash
cd server && go test ./internal/connector/bilibili/ -race
```

Expected: PASS，无 DATA RACE 报告。

- [ ] **Step 7: 提交**

```bash
git add server/
git commit -m "feat: 实现 WebSocket 连接状态机与事件流"
```

---

### Task 15: 限流器与动作执行

**Files:**
- Create: `server/internal/ratelimit/limiter.go`
- Create: `server/internal/connector/actions.go`
- Create: `server/internal/connector/bilibili/action.go`
- Test: `server/internal/ratelimit/limiter_test.go`
- Test: `server/internal/connector/bilibili/action_test.go`

**Interfaces:**
- Consumes: Task 13 的 `api.Client`、`api.APIError`
- Produces:
  - `ratelimit.Limiter` 接口：`Wait(ctx) error`
  - `ratelimit.NewInterval(d time.Duration) Limiter`
  - `connector.SendDanmakuRequest{RoomID, Text, ReplyToUID string}`
  - `connector.BlockRequest{RoomID, UID string, Hours int}`
  - `connector.Actions` 接口
  - `bilibili.Actions`，`bilibili.NewActions(*api.Client, ratelimit.Limiter) *Actions`
  - `bilibili.SplitLongText(text string, maxLen int) []string`
  - `bilibili.IsFatal(err error) bool` — 判断错误是否不应重试

**返回码处理**（源自设计文档 6.4）：

| 返回码 | 含义 | 处理 |
|---|---|---|
| `0` | 成功 | — |
| `10030` | 发送频率过快 | 可重试 |
| `-101` | 未登录 | **不重试** |
| `-111` | csrf 失效 | **不重试** |
| `1003` | 已被禁言 | **不重试** |

- [ ] **Step 1: 写限流器失败测试**

创建 `server/internal/ratelimit/limiter_test.go`：

```go
package ratelimit

import (
	"context"
	"testing"
	"time"
)

func TestIntervalLimiterFirstCallIsImmediate(t *testing.T) {
	l := NewInterval(100 * time.Millisecond)
	start := time.Now()
	if err := l.Wait(context.Background()); err != nil {
		t.Fatalf("Wait 失败: %v", err)
	}
	if d := time.Since(start); d > 20*time.Millisecond {
		t.Errorf("首次调用应立即返回，实际耗时 %v", d)
	}
}

func TestIntervalLimiterEnforcesGap(t *testing.T) {
	l := NewInterval(80 * time.Millisecond)
	ctx := context.Background()

	if err := l.Wait(ctx); err != nil {
		t.Fatalf("Wait 失败: %v", err)
	}
	start := time.Now()
	if err := l.Wait(ctx); err != nil {
		t.Fatalf("Wait 失败: %v", err)
	}
	if d := time.Since(start); d < 60*time.Millisecond {
		t.Errorf("第二次调用应等待约 80ms，实际 %v", d)
	}
}

func TestIntervalLimiterRespectsContext(t *testing.T) {
	l := NewInterval(5 * time.Second)
	ctx := context.Background()
	if err := l.Wait(ctx); err != nil {
		t.Fatalf("Wait 失败: %v", err)
	}

	ctx2, cancel := context.WithTimeout(ctx, 50*time.Millisecond)
	defer cancel()
	if err := l.Wait(ctx2); err == nil {
		t.Error("ctx 超时后 Wait 应返回错误")
	}
}

func TestIntervalLimiterZeroIsNoop(t *testing.T) {
	l := NewInterval(0)
	ctx := context.Background()
	start := time.Now()
	for i := 0; i < 5; i++ {
		if err := l.Wait(ctx); err != nil {
			t.Fatalf("Wait 失败: %v", err)
		}
	}
	if d := time.Since(start); d > 20*time.Millisecond {
		t.Errorf("间隔为 0 时不应等待，实际 %v", d)
	}
}
```

- [ ] **Step 2: 运行测试确认失败**

```bash
cd server && go test ./internal/ratelimit/ -v
```

Expected: 编译失败，`undefined: NewInterval`。

- [ ] **Step 3: 实现限流器**

创建 `server/internal/ratelimit/limiter.go`：

```go
// Package ratelimit 提供动作发送的限流机制。
//
// 本包只提供机制，不定策略：冷却通道、优先级、去重等业务策略
// 属于规则引擎（P2）的职责。
package ratelimit

import (
	"context"
	"sync"
	"time"
)

// Limiter 控制动作的发送节奏。
type Limiter interface {
	// Wait 阻塞到允许发送为止；ctx 取消时返回其错误。
	Wait(ctx context.Context) error
}

// intervalLimiter 保证相邻两次放行之间至少间隔 d。
type intervalLimiter struct {
	mu   sync.Mutex
	d    time.Duration
	next time.Time
}

// NewInterval 创建一个最小间隔限流器。d 为 0 或负数时不做限制。
func NewInterval(d time.Duration) Limiter {
	return &intervalLimiter{d: d}
}

func (l *intervalLimiter) Wait(ctx context.Context) error {
	if l.d <= 0 {
		return ctx.Err()
	}

	l.mu.Lock()
	now := time.Now()
	wait := time.Duration(0)
	if now.Before(l.next) {
		wait = l.next.Sub(now)
		l.next = l.next.Add(l.d)
	} else {
		l.next = now.Add(l.d)
	}
	l.mu.Unlock()

	if wait <= 0 {
		return ctx.Err()
	}
	t := time.NewTimer(wait)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-t.C:
		return nil
	}
}
```

- [ ] **Step 4: 运行测试确认通过**

```bash
cd server && go test ./internal/ratelimit/ -v
```

Expected: 四个测试全部 PASS。

- [ ] **Step 5: 定义 Actions 接口**

创建 `server/internal/connector/actions.go`：

```go
package connector

import "context"

// SendDanmakuRequest 是一次发弹幕请求。
type SendDanmakuRequest struct {
	RoomID     string // 目标直播间号
	Text       string // 弹幕正文
	ReplyToUID string // @ 回复的目标 UID，可为空
}

// BlockRequest 是一次禁言请求。
type BlockRequest struct {
	RoomID string
	UID    string
	Hours  int // 禁言时长，单位小时
}

// Actions 是需要账号身份的写操作集合。
//
// 与 Connector 分离是因为：事件流是房间级的，与身份无关；
// 而写操作是账号级的，且需要支持多账号轮换发言。
type Actions interface {
	// SendDanmaku 发送弹幕。文本超长时会自动切分为多条依次发送。
	SendDanmaku(ctx context.Context, req SendDanmakuRequest) error
	// BlockUser 禁言用户。
	BlockUser(ctx context.Context, req BlockRequest) error
	// UnblockUser 解除禁言。
	UnblockUser(ctx context.Context, roomID, uid string) error
}
```

- [ ] **Step 6: 写动作执行失败测试**

创建 `server/internal/connector/bilibili/action_test.go`：

```go
package bilibili

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/MrZhongzq/MagicalDanmaku/server/internal/connector"
	"github.com/MrZhongzq/MagicalDanmaku/server/internal/connector/bilibili/api"
	"github.com/MrZhongzq/MagicalDanmaku/server/internal/connector/bilibili/auth"
	"github.com/MrZhongzq/MagicalDanmaku/server/internal/ratelimit"
)

// newTestActions 构造指向假服务器的 Actions。
func newTestActions(t *testing.T, body string, record *[]url.Values) *Actions {
	t.Helper()
	var mu sync.Mutex

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r.ParseForm()
		if record != nil {
			mu.Lock()
			*record = append(*record, r.PostForm)
			mu.Unlock()
		}
		w.Write([]byte(body))
	}))
	t.Cleanup(srv.Close)

	sess, err := auth.ParseSession("SESSDATA=x; bili_jct=tok; DedeUserID=42")
	if err != nil {
		t.Fatalf("ParseSession 失败: %v", err)
	}
	ac := api.New(sess, api.WithHTTPClient(srv.Client()))
	for _, n := range []string{"sendMsg", "addBlock", "delBlock"} {
		ac.SetBaseURL(n, srv.URL)
	}
	return NewActions(ac, ratelimit.NewInterval(0))
}

func TestSendDanmaku(t *testing.T) {
	var forms []url.Values
	a := newTestActions(t, `{"code":0,"data":{}}`, &forms)

	err := a.SendDanmaku(context.Background(), connector.SendDanmakuRequest{
		RoomID: "21452505", Text: "你好世界",
	})
	if err != nil {
		t.Fatalf("SendDanmaku 失败: %v", err)
	}
	if len(forms) != 1 {
		t.Fatalf("请求次数 = %d, 期望 1", len(forms))
	}

	f := forms[0]
	if f.Get("msg") != "你好世界" {
		t.Errorf("msg = %q", f.Get("msg"))
	}
	if f.Get("roomid") != "21452505" {
		t.Errorf("roomid = %q", f.Get("roomid"))
	}
	if f.Get("csrf") != "tok" {
		t.Errorf("csrf = %q", f.Get("csrf"))
	}
	if f.Get("color") == "" || f.Get("fontsize") == "" || f.Get("mode") == "" {
		t.Errorf("缺少必需的样式参数: %v", f)
	}
	if f.Get("rnd") == "" {
		t.Error("缺少 rnd 参数")
	}
}

func TestSendDanmakuWithReply(t *testing.T) {
	var forms []url.Values
	a := newTestActions(t, `{"code":0,"data":{}}`, &forms)

	err := a.SendDanmaku(context.Background(), connector.SendDanmakuRequest{
		RoomID: "21452505", Text: "收到", ReplyToUID: "12345",
	})
	if err != nil {
		t.Fatalf("SendDanmaku 失败: %v", err)
	}
	if got := forms[0].Get("reply_mid"); got != "12345" {
		t.Errorf("reply_mid = %q", got)
	}
}

func TestSendDanmakuSplitsLongText(t *testing.T) {
	var forms []url.Values
	a := newTestActions(t, `{"code":0,"data":{}}`, &forms)
	a.SetMaxLength(5)

	err := a.SendDanmaku(context.Background(), connector.SendDanmakuRequest{
		RoomID: "1", Text: "一二三四五六七八九十",
	})
	if err != nil {
		t.Fatalf("SendDanmaku 失败: %v", err)
	}
	if len(forms) != 2 {
		t.Fatalf("请求次数 = %d, 期望 2", len(forms))
	}
	if forms[0].Get("msg") != "一二三四五" {
		t.Errorf("第一条 = %q", forms[0].Get("msg"))
	}
	if forms[1].Get("msg") != "六七八九十" {
		t.Errorf("第二条 = %q", forms[1].Get("msg"))
	}
}

func TestSendDanmakuRejectsEmpty(t *testing.T) {
	a := newTestActions(t, `{"code":0,"data":{}}`, nil)
	if err := a.SendDanmaku(context.Background(), connector.SendDanmakuRequest{RoomID: "1"}); err == nil {
		t.Error("空文本应当报错")
	}
}

func TestBlockUser(t *testing.T) {
	var forms []url.Values
	a := newTestActions(t, `{"code":0,"data":{}}`, &forms)

	err := a.BlockUser(context.Background(), connector.BlockRequest{
		RoomID: "21452505", UID: "999", Hours: 12,
	})
	if err != nil {
		t.Fatalf("BlockUser 失败: %v", err)
	}
	f := forms[0]
	if f.Get("tuid") != "999" {
		t.Errorf("tuid = %q", f.Get("tuid"))
	}
	if f.Get("hour") != "12" {
		t.Errorf("hour = %q", f.Get("hour"))
	}
	if f.Get("room_id") != "21452505" {
		t.Errorf("room_id = %q", f.Get("room_id"))
	}
}

func TestUnblockUser(t *testing.T) {
	var forms []url.Values
	a := newTestActions(t, `{"code":0,"data":{}}`, &forms)

	if err := a.UnblockUser(context.Background(), "21452505", "888"); err != nil {
		t.Fatalf("UnblockUser 失败: %v", err)
	}
	if got := forms[0].Get("tuid"); got != "888" {
		t.Errorf("tuid = %q", got)
	}
}

func TestSendDanmakuSurfacesAPIError(t *testing.T) {
	a := newTestActions(t, `{"code":1003,"message":"您已被禁言"}`, nil)
	err := a.SendDanmaku(context.Background(), connector.SendDanmakuRequest{RoomID: "1", Text: "x"})
	if err == nil {
		t.Fatal("应当返回错误")
	}
	if !strings.Contains(err.Error(), "禁言") {
		t.Errorf("错误应含服务端消息，实际 %v", err)
	}
	if !IsFatal(err) {
		t.Error("1003 应判定为不可重试")
	}
}

func TestIsFatal(t *testing.T) {
	cases := []struct {
		code  int
		fatal bool
	}{
		{-101, true},  // 未登录
		{-111, true},  // csrf 失效
		{1003, true},  // 已被禁言
		{10030, false}, // 发送过快，可重试
		{-500, false},  // 未知错误，允许重试
	}
	for _, tc := range cases {
		if got := IsFatal(&api.APIError{Code: tc.code}); got != tc.fatal {
			t.Errorf("IsFatal(code=%d) = %v, 期望 %v", tc.code, got, tc.fatal)
		}
	}
}

func TestSplitLongText(t *testing.T) {
	cases := []struct {
		in     string
		maxLen int
		want   []string
	}{
		{"", 5, nil},
		{"短", 5, []string{"短"}},
		{"一二三四五", 5, []string{"一二三四五"}},
		{"一二三四五六", 5, []string{"一二三四五", "六"}},
		{"abcdefghij", 4, []string{"abcd", "efgh", "ij"}},
	}
	for _, tc := range cases {
		got := SplitLongText(tc.in, tc.maxLen)
		if len(got) != len(tc.want) {
			t.Errorf("SplitLongText(%q, %d) 段数 = %d, 期望 %d", tc.in, tc.maxLen, len(got), len(tc.want))
			continue
		}
		for i := range got {
			if got[i] != tc.want[i] {
				t.Errorf("SplitLongText(%q, %d)[%d] = %q, 期望 %q", tc.in, tc.maxLen, i, got[i], tc.want[i])
			}
		}
	}
}

func TestActionsRespectRateLimiter(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"code":0,"data":{}}`))
	}))
	t.Cleanup(srv.Close)

	sess, err := auth.ParseSession("SESSDATA=x; bili_jct=tok; DedeUserID=42")
	if err != nil {
		t.Fatalf("ParseSession 失败: %v", err)
	}
	ac := api.New(sess, api.WithHTTPClient(srv.Client()))
	ac.SetBaseURL("sendMsg", srv.URL)
	a := NewActions(ac, ratelimit.NewInterval(60*time.Millisecond))

	ctx := context.Background()
	req := connector.SendDanmakuRequest{RoomID: "1", Text: "x"}
	if err := a.SendDanmaku(ctx, req); err != nil {
		t.Fatalf("SendDanmaku 失败: %v", err)
	}
	start := time.Now()
	if err := a.SendDanmaku(ctx, req); err != nil {
		t.Fatalf("SendDanmaku 失败: %v", err)
	}
	if d := time.Since(start); d < 40*time.Millisecond {
		t.Errorf("第二次发送应受限流约束，实际间隔 %v", d)
	}
}
```

- [ ] **Step 7: 运行测试确认失败**

```bash
cd server && go test ./internal/connector/bilibili/ -run 'TestSend|TestBlock|TestUnblock|TestIsFatal|TestSplitLong|TestActions' -v
```

Expected: 编译失败，`undefined: NewActions`。

- [ ] **Step 8: 实现动作执行**

创建 `server/internal/connector/bilibili/action.go`：

```go
package bilibili

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"strconv"
	"sync"
	"time"

	"github.com/MrZhongzq/MagicalDanmaku/server/internal/connector"
	"github.com/MrZhongzq/MagicalDanmaku/server/internal/connector/bilibili/api"
	"github.com/MrZhongzq/MagicalDanmaku/server/internal/ratelimit"
)

// defaultMaxDanmakuLength 是普通账号的单条弹幕字数上限。
// 高等级或舰长账号可达 30/40，由上层通过 SetMaxLength 调整。
const defaultMaxDanmakuLength = 20

// fatalCodes 是不应重试的返回码。
var fatalCodes = map[int]bool{
	-101: true, // 账号未登录
	-111: true, // csrf 校验失败
	1003: true, // 已被禁言
}

// IsFatal 判断错误是否不可重试。
// 未知错误一律视为可重试，交由上层的退避策略处理。
func IsFatal(err error) bool {
	var apiErr *api.APIError
	if !errors.As(err, &apiErr) {
		return false
	}
	return fatalCodes[apiErr.Code]
}

// Actions 实现 connector.Actions。
type Actions struct {
	api     *api.Client
	limiter ratelimit.Limiter

	mu        sync.RWMutex
	maxLength int
}

var _ connector.Actions = (*Actions)(nil)

// NewActions 创建动作执行器。limiter 为 nil 时使用 1.5 秒的保守默认值。
func NewActions(c *api.Client, limiter ratelimit.Limiter) *Actions {
	if limiter == nil {
		limiter = ratelimit.NewInterval(1500 * time.Millisecond)
	}
	return &Actions{api: c, limiter: limiter, maxLength: defaultMaxDanmakuLength}
}

// SetMaxLength 设置单条弹幕的字数上限。
func (a *Actions) SetMaxLength(n int) {
	if n <= 0 {
		return
	}
	a.mu.Lock()
	a.maxLength = n
	a.mu.Unlock()
}

// SendDanmaku 发送弹幕，超长文本自动切分为多条依次发送。
func (a *Actions) SendDanmaku(ctx context.Context, req connector.SendDanmakuRequest) error {
	if req.Text == "" {
		return errors.New("bilibili: 弹幕内容不能为空")
	}
	if req.RoomID == "" {
		return errors.New("bilibili: 未指定直播间号")
	}

	a.mu.RLock()
	maxLen := a.maxLength
	a.mu.RUnlock()

	for i, part := range SplitLongText(req.Text, maxLen) {
		if err := a.limiter.Wait(ctx); err != nil {
			return err
		}
		if err := a.sendOne(ctx, req.RoomID, part, req.ReplyToUID); err != nil {
			return fmt.Errorf("发送第 %d 段弹幕失败: %w", i+1, err)
		}
	}
	return nil
}

// sendOne 发送单条弹幕。
func (a *Actions) sendOne(ctx context.Context, roomID, text, replyMID string) error {
	form := url.Values{}
	form.Set("bubble", "0")
	form.Set("msg", text)
	form.Set("color", "16777215")
	form.Set("mode", "1")
	form.Set("fontsize", "25")
	form.Set("rnd", strconv.FormatInt(time.Now().Unix(), 10))
	form.Set("roomid", roomID)
	if replyMID != "" {
		form.Set("reply_mid", replyMID)
	}
	return a.api.PostForm(ctx, a.api.URLFor("sendMsg"), form, nil)
}

// BlockUser 禁言用户。
func (a *Actions) BlockUser(ctx context.Context, req connector.BlockRequest) error {
	if req.UID == "" || req.RoomID == "" {
		return errors.New("bilibili: 禁言请求缺少 UID 或直播间号")
	}
	hours := req.Hours
	if hours <= 0 {
		hours = 1
	}
	if err := a.limiter.Wait(ctx); err != nil {
		return err
	}

	form := url.Values{}
	form.Set("room_id", req.RoomID)
	form.Set("tuid", req.UID)
	form.Set("mobile_app", "web")
	form.Set("visit_id", "")
	form.Set("hour", strconv.Itoa(hours))
	return a.api.PostForm(ctx, a.api.URLFor("addBlock"), form, nil)
}

// UnblockUser 解除禁言。
func (a *Actions) UnblockUser(ctx context.Context, roomID, uid string) error {
	if uid == "" || roomID == "" {
		return errors.New("bilibili: 解禁请求缺少 UID 或直播间号")
	}
	if err := a.limiter.Wait(ctx); err != nil {
		return err
	}

	form := url.Values{}
	form.Set("roomid", roomID)
	form.Set("tuid", uid)
	form.Set("visit_id", "")
	return a.api.PostForm(ctx, a.api.URLFor("delBlock"), form, nil)
}

// SplitLongText 按字符数（而非字节数）把文本切成若干段。
// 空文本返回 nil。
func SplitLongText(text string, maxLen int) []string {
	if text == "" {
		return nil
	}
	if maxLen <= 0 {
		return []string{text}
	}

	runes := []rune(text)
	if len(runes) <= maxLen {
		return []string{text}
	}

	out := make([]string, 0, (len(runes)+maxLen-1)/maxLen)
	for i := 0; i < len(runes); i += maxLen {
		end := i + maxLen
		if end > len(runes) {
			end = len(runes)
		}
		out = append(out, string(runes[i:end]))
	}
	return out
}
```

- [ ] **Step 9: 为 api.Client 暴露 URLFor**

`action.go` 需要访问接口地址表。在 `server/internal/connector/bilibili/api/client.go` 中，把私有方法 `urlFor` 改为导出的 `URLFor`：

```go
// URLFor 返回命名接口的地址。
func (c *Client) URLFor(name string) string { return c.baseURLs[name] }
```

同时把 `client.go` 与 `room.go` 内部所有 `c.urlFor(` 的调用改为 `c.URLFor(`，并删除原来的私有 `urlFor` 方法。

```bash
cd server && grep -rn "urlFor" internal/connector/bilibili/api/
```

Expected: 无小写 `urlFor` 残留。

- [ ] **Step 10: 运行全部测试确认通过**

```bash
cd server && go vet ./... && go test ./... -race
```

Expected: 无 vet 输出，全部 PASS，无竞态报告。

- [ ] **Step 11: 提交**

```bash
git add server/
git commit -m "feat: 实现限流器与弹幕禁言动作执行"
```

---

### Task 16: 扫码登录与 probe CLI

**Files:**
- Create: `server/internal/connector/bilibili/auth/qrcode.go`
- Create: `server/cmd/magicd/main.go`
- Create: `server/cmd/magicd/probe.go`
- Create: `server/cmd/magicd/login.go`
- Create: `server/cmd/magicd/render.go`
- Test: `server/internal/connector/bilibili/auth/qrcode_test.go`
- Test: `server/cmd/magicd/render_test.go`

**Interfaces:**
- Consumes: Task 11–15 全部产出
- Produces:
  - `auth.QRCode{Key, URL string}`
  - `auth.QRLogin{}`，`auth.NewQRLogin(hc *http.Client) *QRLogin`
  - `(*QRLogin).Generate(ctx) (*QRCode, error)`
  - `(*QRLogin).Poll(ctx, key string) (*PollResult, error)`
  - `auth.PollResult{Status PollStatus, Cookie string}`，`PollStatus` 常量 `PollWaiting`、`PollScanned`、`PollExpired`、`PollSuccess`
  - `magicd` 可执行文件，含 `probe` 与 `login` 两个子命令

**关于二维码渲染：** 后端**不引入 QR 图像库**。`login` 子命令把登录 URL 打印出来，由用户自行用手机 B 站 App 扫描或粘贴。P4 阶段前端用 JS 库渲染即可。

**轮询状态码**（B 站 `qrcode/poll` 接口的 `data.code`）：`86101` 未扫描，`86090` 已扫描待确认，`86038` 二维码失效，`0` 登录成功。

- [ ] **Step 1: 写扫码登录失败测试**

创建 `server/internal/connector/bilibili/auth/qrcode_test.go`：

```go
package auth

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestQRGenerate(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"code":0,"data":{
			"url":"https://passport.bilibili.com/h5-app/passport/login/scan?navhide=1&qrcode_key=abc123",
			"qrcode_key":"abc123"
		}}`))
	}))
	defer srv.Close()

	l := NewQRLogin(srv.Client())
	l.SetGenerateURL(srv.URL)

	qr, err := l.Generate(context.Background())
	if err != nil {
		t.Fatalf("Generate 失败: %v", err)
	}
	if qr.Key != "abc123" {
		t.Errorf("Key = %q", qr.Key)
	}
	if !strings.Contains(qr.URL, "qrcode_key=abc123") {
		t.Errorf("URL = %q", qr.URL)
	}
}

func TestQRPollStatuses(t *testing.T) {
	cases := []struct {
		body string
		want PollStatus
	}{
		{`{"code":0,"data":{"code":86101,"message":"未扫码"}}`, PollWaiting},
		{`{"code":0,"data":{"code":86090,"message":"已扫码未确认"}}`, PollScanned},
		{`{"code":0,"data":{"code":86038,"message":"二维码已失效"}}`, PollExpired},
	}
	for _, tc := range cases {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Write([]byte(tc.body))
		}))
		l := NewQRLogin(srv.Client())
		l.SetPollURL(srv.URL)

		got, err := l.Poll(context.Background(), "k")
		srv.Close()
		if err != nil {
			t.Fatalf("Poll 失败: %v", err)
		}
		if got.Status != tc.want {
			t.Errorf("body=%s Status = %s, 期望 %s", tc.body, got.Status, tc.want)
		}
	}
}

func TestQRPollSuccessCollectsCookie(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := w.Header()
		h.Add("Set-Cookie", "SESSDATA=sess-value; Path=/; Domain=.bilibili.com; HttpOnly")
		h.Add("Set-Cookie", "bili_jct=jct-value; Path=/; Domain=.bilibili.com")
		h.Add("Set-Cookie", "DedeUserID=1234; Path=/; Domain=.bilibili.com")
		w.Write([]byte(`{"code":0,"data":{"code":0,"message":"","refresh_token":"rt"}}`))
	}))
	defer srv.Close()

	l := NewQRLogin(srv.Client())
	l.SetPollURL(srv.URL)

	got, err := l.Poll(context.Background(), "k")
	if err != nil {
		t.Fatalf("Poll 失败: %v", err)
	}
	if got.Status != PollSuccess {
		t.Fatalf("Status = %s, 期望 %s", got.Status, PollSuccess)
	}

	sess, err := ParseSession(got.Cookie)
	if err != nil {
		t.Fatalf("登录结果无法解析为会话: %v", err)
	}
	if sess.SESSDATA != "sess-value" || sess.CSRF != "jct-value" || sess.UID != "1234" {
		t.Errorf("会话字段错误: %+v", sess)
	}
}
```

- [ ] **Step 2: 运行测试确认失败**

```bash
cd server && go test ./internal/connector/bilibili/auth/ -run TestQR -v
```

Expected: 编译失败，`undefined: NewQRLogin`。

- [ ] **Step 3: 实现扫码登录**

创建 `server/internal/connector/bilibili/auth/qrcode.go`：

```go
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
		userAgent:   "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36",
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
```

- [ ] **Step 4: 运行测试确认通过**

```bash
cd server && go test ./internal/connector/bilibili/auth/ -v
```

Expected: 全部 PASS。

- [ ] **Step 5: 写事件渲染失败测试**

创建 `server/cmd/magicd/render_test.go`：

```go
package main

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/MrZhongzq/MagicalDanmaku/server/internal/event"
)

func baseEvent(t event.Type, p event.Payload) event.Event {
	return event.Event{
		ID:         "01ABC",
		RoomID:     "21452505",
		Platform:   event.PlatformBilibili,
		Type:       t,
		Timestamp:  time.Date(2026, 7, 29, 19, 23, 1, 0, time.Local),
		ReceivedAt: time.Date(2026, 7, 29, 19, 23, 1, 0, time.Local),
		Payload:    p,
		Raw:        json.RawMessage(`{}`),
	}
}

func TestRenderDanmaku(t *testing.T) {
	got := Render(baseEvent(event.TypeDanmaku, event.Danmaku{
		User: event.User{UID: "123", Username: "路人甲", UserLevel: 18, GuardLevel: event.GuardCaptain},
		Text: "主播晚上好",
	}))
	for _, want := range []string{"19:23:01", "DANMAKU", "路人甲", "123", "UL18", "舰长", "主播晚上好"} {
		if !strings.Contains(got, want) {
			t.Errorf("渲染结果缺少 %q，实际:\n%s", want, got)
		}
	}
}

func TestRenderGift(t *testing.T) {
	got := Render(baseEvent(event.TypeGift, event.Gift{
		User:     event.User{UID: "1", Username: "土豪"},
		GiftName: "小心心", Count: 3, CoinType: "silver",
	}))
	for _, want := range []string{"GIFT", "土豪", "小心心", "x3", "免费"} {
		if !strings.Contains(got, want) {
			t.Errorf("渲染结果缺少 %q，实际:\n%s", want, got)
		}
	}
}

func TestRenderGuardBuy(t *testing.T) {
	got := Render(baseEvent(event.TypeGuardBuy, event.GuardBuy{
		User: event.User{UID: "1", Username: "新舰长"}, GuardName: "舰长", Count: 1, Price: 198000,
	}))
	for _, want := range []string{"GUARD_BUY", "新舰长", "舰长", "x1"} {
		if !strings.Contains(got, want) {
			t.Errorf("渲染结果缺少 %q，实际:\n%s", want, got)
		}
	}
}

func TestRenderUnknownShowsCommand(t *testing.T) {
	got := Render(baseEvent(event.TypeUnknown, event.Unknown{Command: "LIVE_MULTI_VIEW_CHANGE"}))
	for _, want := range []string{"UNKNOWN", "LIVE_MULTI_VIEW_CHANGE", "raw"} {
		if !strings.Contains(got, want) {
			t.Errorf("渲染结果缺少 %q，实际:\n%s", want, got)
		}
	}
}

func TestRenderNeverPanics(t *testing.T) {
	payloads := []struct {
		t event.Type
		p event.Payload
	}{
		{event.TypeDanmaku, event.Danmaku{}},
		{event.TypeSuperChat, event.SuperChat{}},
		{event.TypeSuperChatDelete, event.SuperChatDelete{}},
		{event.TypeGift, event.Gift{}},
		{event.TypeGiftCombo, event.GiftCombo{}},
		{event.TypeGuardBuy, event.GuardBuy{}},
		{event.TypeUserEnter, event.UserEnter{}},
		{event.TypeUserFollow, event.UserFollow{}},
		{event.TypeUserShare, event.UserShare{}},
		{event.TypeUserLike, event.UserLike{}},
		{event.TypeLiveStart, event.LiveStart{}},
		{event.TypeLiveStop, event.LiveStop{}},
		{event.TypeRoomChange, event.RoomChange{}},
		{event.TypeUserBlocked, event.UserBlocked{}},
		{event.TypeOnlineRankUpdate, event.OnlineRankUpdate{}},
		{event.TypeRoomStatsUpdate, event.RoomStatsUpdate{}},
		{event.TypeBattle, event.Battle{}},
		{event.TypeUnknown, event.Unknown{}},
	}
	for _, tc := range payloads {
		if got := Render(baseEvent(tc.t, tc.p)); got == "" {
			t.Errorf("类型 %s 渲染为空串", tc.t)
		}
	}
}
```

- [ ] **Step 6: 运行测试确认失败**

```bash
cd server && go test ./cmd/magicd/ -v
```

Expected: 编译失败，`undefined: Render`。

- [ ] **Step 7: 实现事件渲染**

创建 `server/cmd/magicd/render.go`：

```go
package main

import (
	"fmt"
	"strings"

	"github.com/MrZhongzq/MagicalDanmaku/server/internal/event"
)

// Render 把一个事件渲染成一行可读文本。
func Render(ev event.Event) string {
	ts := ev.Timestamp.Format("15:04:05")
	typ := strings.ToUpper(string(ev.Type))
	return fmt.Sprintf("[%s] %-16s %s", ts, typ, describe(ev))
}

// describe 生成事件的正文描述。
func describe(ev event.Event) string {
	switch p := ev.Payload.(type) {
	case event.Danmaku:
		return fmt.Sprintf("%s: %s", userTag(p.User), p.Text)
	case event.SuperChat:
		return fmt.Sprintf("%s 醒目留言 ¥%d: %s", userTag(p.User), p.Price, p.Text)
	case event.SuperChatDelete:
		return fmt.Sprintf("删除了 %d 条醒目留言", len(p.IDs))
	case event.Gift:
		return fmt.Sprintf("%s 送出 %s x%d (%s)", userTag(p.User), p.GiftName, p.Count, coinLabel(p.CoinType, p.TotalCoin))
	case event.GiftCombo:
		return fmt.Sprintf("%s 连击 %s x%d", userTag(p.User), p.GiftName, p.Count)
	case event.GuardBuy:
		verb := "购买"
		if p.IsRenew {
			verb = "续费"
		}
		return fmt.Sprintf("%s %s %s x%d", userTag(p.User), verb, p.GuardName, p.Count)
	case event.UserEnter:
		return fmt.Sprintf("%s 进入直播间", userTag(p.User))
	case event.UserFollow:
		return fmt.Sprintf("%s 关注了主播", userTag(p.User))
	case event.UserShare:
		return fmt.Sprintf("%s 分享了直播间", userTag(p.User))
	case event.UserLike:
		return fmt.Sprintf("%s 点赞了", userTag(p.User))
	case event.LiveStart:
		return "主播开播了"
	case event.LiveStop:
		return "主播下播了"
	case event.RoomChange:
		return fmt.Sprintf("房间信息变更 标题=%q 分区=%s/%s", p.Title, p.ParentAreaName, p.AreaName)
	case event.UserBlocked:
		return fmt.Sprintf("%s 被禁言", userTag(p.User))
	case event.OnlineRankUpdate:
		if p.Count >= 0 {
			return fmt.Sprintf("高能榜人数 %d", p.Count)
		}
		return fmt.Sprintf("高能榜前 %d 名更新", len(p.Top))
	case event.RoomStatsUpdate:
		return statsText(p)
	case event.Battle:
		return fmt.Sprintf("大乱斗事件 %s", p.SubCommand)
	case event.Unknown:
		return fmt.Sprintf("cmd=%s (raw 已保留 %d 字节)", p.Command, len(ev.Raw))
	default:
		return fmt.Sprintf("未处理的载荷类型 %T", ev.Payload)
	}
}

// userTag 生成 "昵称(UID) UL等级 舰长" 形式的用户标签。
func userTag(u event.User) string {
	var b strings.Builder
	if u.Username != "" {
		b.WriteString(u.Username)
	} else {
		b.WriteString("(匿名)")
	}
	if u.UID != "" {
		fmt.Fprintf(&b, "(%s)", u.UID)
	}
	if u.UserLevel > 0 {
		fmt.Fprintf(&b, " UL%d", u.UserLevel)
	}
	switch u.GuardLevel {
	case event.GuardGovernor:
		b.WriteString(" 总督")
	case event.GuardAdmiral:
		b.WriteString(" 提督")
	case event.GuardCaptain:
		b.WriteString(" 舰长")
	}
	if u.Medal != nil {
		fmt.Fprintf(&b, " [%s%d]", u.Medal.Name, u.Medal.Level)
	}
	return b.String()
}

// coinLabel 描述礼物价值。
func coinLabel(coinType string, total int64) string {
	if coinType == "gold" {
		return fmt.Sprintf("¥%.1f", float64(total)/1000)
	}
	return "免费"
}

// statsText 描述房间统计变化。
func statsText(s event.RoomStatsUpdate) string {
	var parts []string
	if s.Fans != nil {
		parts = append(parts, fmt.Sprintf("粉丝 %d", *s.Fans))
	}
	if s.FansClub != nil {
		parts = append(parts, fmt.Sprintf("粉丝团 %d", *s.FansClub))
	}
	if s.Watched != nil {
		parts = append(parts, fmt.Sprintf("看过 %d", *s.Watched))
	}
	if s.LikeCount != nil {
		parts = append(parts, fmt.Sprintf("点赞 %d", *s.LikeCount))
	}
	if len(parts) == 0 {
		return "房间数据更新"
	}
	return strings.Join(parts, " ")
}
```

- [ ] **Step 8: 运行渲染测试确认通过**

```bash
cd server && go test ./cmd/magicd/ -v
```

Expected: 五个测试全部 PASS。

- [ ] **Step 9: 实现 CLI 入口**

创建 `server/cmd/magicd/main.go`：

```go
// Command magicd 是神奇弹幕的服务端可执行文件。
//
// P0 阶段提供两个子命令：
//   login  扫码登录，输出 Cookie
//   probe  连接直播间并打印实时事件流
package main

import (
	"fmt"
	"os"
)

const usage = `magicd —— 神奇弹幕服务端

用法:
  magicd login [-o cookie.txt]
        扫码登录，把 Cookie 写入文件（默认输出到标准输出）

  magicd probe -room <房间号> [-cookie-file cookie.txt] [-type <事件类型>]
        连接直播间并打印实时事件流

示例:
  magicd login -o cookie.txt
  magicd probe -room 21452505 -cookie-file cookie.txt
  magicd probe -room 21452505 -cookie-file cookie.txt -type danmaku,gift
`

func main() {
	if len(os.Args) < 2 {
		fmt.Fprint(os.Stderr, usage)
		os.Exit(2)
	}

	var err error
	switch os.Args[1] {
	case "login":
		err = runLogin(os.Args[2:])
	case "probe":
		err = runProbe(os.Args[2:])
	case "-h", "--help", "help":
		fmt.Print(usage)
		return
	default:
		fmt.Fprintf(os.Stderr, "未知的子命令: %s\n\n%s", os.Args[1], usage)
		os.Exit(2)
	}

	if err != nil {
		fmt.Fprintf(os.Stderr, "错误: %v\n", err)
		os.Exit(1)
	}
}
```

创建 `server/cmd/magicd/login.go`：

```go
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"time"

	"github.com/MrZhongzq/MagicalDanmaku/server/internal/connector/bilibili/auth"
)

// runLogin 执行扫码登录流程。
func runLogin(args []string) error {
	fs := flag.NewFlagSet("login", flag.ExitOnError)
	out := fs.String("o", "", "把 Cookie 写入指定文件；留空则打印到标准输出")
	if err := fs.Parse(args); err != nil {
		return err
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	l := auth.NewQRLogin(nil)
	qr, err := l.Generate(ctx)
	if err != nil {
		return err
	}

	fmt.Println("请用哔哩哔哩手机客户端扫描以下地址对应的二维码：")
	fmt.Println()
	fmt.Println("   " + qr.URL)
	fmt.Println()
	fmt.Println("（可把该地址粘贴到任意二维码生成器中显示，或直接在手机上打开）")
	fmt.Println("等待扫码中，按 Ctrl+C 取消...")

	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	last := auth.PollWaiting
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
		}

		res, err := l.Poll(ctx, qr.Key)
		if err != nil {
			return err
		}
		if res.Status != last {
			switch res.Status {
			case auth.PollScanned:
				fmt.Println("已扫码，请在手机上确认登录...")
			case auth.PollExpired:
				return fmt.Errorf("二维码已失效，请重新运行 login")
			}
			last = res.Status
		}
		if res.Status != auth.PollSuccess {
			continue
		}

		// 校验拿到的 Cookie 可用
		sess, err := auth.ParseSession(res.Cookie)
		if err != nil {
			return fmt.Errorf("登录成功但 Cookie 不完整: %w", err)
		}
		fmt.Printf("登录成功，UID=%s\n", sess.UID)

		if *out == "" {
			fmt.Println(res.Cookie)
			return nil
		}
		// 0600 权限：Cookie 等同于账号密码，不得让同机其他用户读到。
		if err := os.WriteFile(*out, []byte(res.Cookie), 0o600); err != nil {
			return fmt.Errorf("写入 %s 失败: %w", *out, err)
		}
		fmt.Printf("Cookie 已写入 %s\n", *out)
		return nil
	}
}
```

创建 `server/cmd/magicd/probe.go`：

```go
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"strings"

	"github.com/MrZhongzq/MagicalDanmaku/server/internal/connector/bilibili"
	"github.com/MrZhongzq/MagicalDanmaku/server/internal/connector/bilibili/api"
	"github.com/MrZhongzq/MagicalDanmaku/server/internal/connector/bilibili/auth"
	"github.com/MrZhongzq/MagicalDanmaku/server/internal/event"
)

// runProbe 连接直播间并打印事件流。
func runProbe(args []string) error {
	fs := flag.NewFlagSet("probe", flag.ExitOnError)
	room := fs.String("room", "", "直播间号（必填）")
	cookieFile := fs.String("cookie-file", "", "Cookie 文件路径；留空则匿名连接")
	typeFilter := fs.String("type", "", "只显示指定事件类型，逗号分隔，如 danmaku,gift")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *room == "" {
		return errors.New("必须通过 -room 指定直播间号")
	}

	var sess *auth.Session
	if *cookieFile != "" {
		b, err := os.ReadFile(*cookieFile)
		if err != nil {
			return fmt.Errorf("读取 Cookie 文件失败: %w", err)
		}
		sess, err = auth.ParseSession(strings.TrimSpace(string(b)))
		if err != nil {
			return err
		}
		fmt.Printf("已加载账号 UID=%s\n", sess.UID)
	} else {
		fmt.Println("未提供 Cookie，将以匿名身份连接（部分事件字段会缺失）")
	}

	allow := parseTypeFilter(*typeFilter)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	apiClient := api.New(sess)

	// 先解析真实房间号：用户可能填的是短号。
	if err := apiClient.RefreshNav(ctx); err != nil {
		return fmt.Errorf("初始化签名失败: %w", err)
	}
	info, err := apiClient.RoomInfo(ctx, *room)
	if err != nil {
		return err
	}
	status := "未开播"
	if info.IsLiving() {
		status = "直播中"
	}
	fmt.Printf("直播间 %s（%s）标题：%s  状态：%s\n\n",
		info.RoomID, info.ParentAreaName+"/"+info.AreaName, info.Title, status)

	c := bilibili.NewClient(info.RoomID, apiClient)

	go func() {
		for ev := range c.Events() {
			if allow != nil && !allow[ev.Type] {
				continue
			}
			fmt.Println(Render(ev))
		}
	}()

	err = c.Run(ctx)
	if errors.Is(err, context.Canceled) {
		fmt.Println("\n已断开连接")
		return nil
	}
	return err
}

// parseTypeFilter 解析 -type 参数，返回 nil 表示不过滤。
func parseTypeFilter(s string) map[event.Type]bool {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil
	}
	m := make(map[event.Type]bool)
	for _, part := range strings.Split(s, ",") {
		if part = strings.TrimSpace(part); part != "" {
			m[event.Type(part)] = true
		}
	}
	if len(m) == 0 {
		return nil
	}
	return m
}
```

- [ ] **Step 10: 编译并验证 CLI**

```bash
cd server && go build -o ../magicd.exe ./cmd/magicd && ../magicd.exe --help
```

Expected: 编译通过，打印用法说明。

- [ ] **Step 11: 运行全部测试与竞态检测**

```bash
cd server && go vet ./... && go test ./... -race
```

Expected: 无 vet 输出，全部 PASS，无竞态报告。

- [ ] **Step 12: 把构建产物加入 gitignore**

在仓库根目录的 `.gitignore` 末尾追加：

```
# Go 构建产物
/magicd
/magicd.exe
/server/magicd
/server/magicd.exe
```

- [ ] **Step 13: 提交**

```bash
git add server/ .gitignore
git commit -m "feat: 实现扫码登录与 probe 命令行工具"
```

---

## P0 验收

全部 16 个任务完成后，执行以下验收步骤：

- [ ] **验收 1: 静态检查与测试**

```bash
cd server && go vet ./... && go test ./... -race -count=1
```

Expected: 无 vet 输出；全部包 PASS；无竞态报告。

- [ ] **验收 2: 黄金样本覆盖率**

```bash
cd server && go test ./internal/connector/bilibili/cmdmap/ -run TestGoldenSamples -v
```

Expected: 输出「已校验 23 个黄金样本」，且无样本落入 Unknown。

- [ ] **验收 3: 真实直播间联调**

```bash
cd server && go build -o ../magicd.exe ./cmd/magicd
../magicd.exe login -o cookie.txt
../magicd.exe probe -room <一个正在直播的房间号> -cookie-file cookie.txt
```

Expected: 打印出与设计文档 1.1 节示例一致的实时事件流；Ctrl+C 可干净退出。

> 这是 P0 唯一需要接触真实 B 站接口的环节，属于人工验收而非自动化测试。

- [ ] **验收 4: 交叉编译预检**

P1 会正式搭建发布流水线，这里先确认没有平台相关的依赖：

```bash
cd server
GOOS=linux   GOARCH=amd64 go build -o /dev/null ./cmd/magicd
GOOS=linux   GOARCH=arm64 go build -o /dev/null ./cmd/magicd
GOOS=darwin  GOARCH=arm64 go build -o /dev/null ./cmd/magicd
GOOS=windows GOARCH=amd64 go build -o /dev/null ./cmd/magicd
```

Expected: 四个目标全部编译通过。若失败，说明引入了含 cgo 或平台特定代码的依赖，必须在进入 P1 前解决。

---

## 计划自查记录

**规格覆盖**：设计文档各节与任务的对应关系——

| 设计文档章节 | 覆盖任务 |
|---|---|
| 2. 归一化事件模型 | Task 1 |
| 2.6 修复非线程安全去重 | Task 14（`handleMessage` 无全局状态） |
| 2.6 修复字符串裁剪构造请求 | Task 15（`sendOne` 用结构化 `url.Values`） |
| 3. Connector 抽象 | Task 14（`Connector`）、Task 15（`Actions`） |
| 4.1 扫码登录 | Task 16 |
| 4.2 会话必需字段 | Task 11 |
| 4.3 buvid 获取 | Task 13（`FetchBuVID`）、Task 14（风控时补齐） |
| 4.4 wbi 签名 | Task 12 |
| 5.1–5.2 状态机与连接流程 | Task 14 |
| 5.3 包格式 | Task 2 |
| 5.3 多包切分 | Task 3 |
| 5.4 心跳 | Task 14 |
| 5.5 重连退避与 host 轮换 | Task 14 |
| 5.6 风控处理 | Task 14（`fetchDanmuInfo` + 长退避） |
| 6.1 三类动作 | Task 15 |
| 6.2 长弹幕切分 | Task 15（`SplitLongText`） |
| 6.3 限流机制 | Task 15（`ratelimit`） |
| 6.4 返回码处理 | Task 15（`IsFatal`） |
| 7.1 黄金样本回归 | Task 5 建立框架，Task 6–10 持续补充 |
| 7.2 包解析单测 | Task 2、Task 3 |
| 7.3 状态机单测 | Task 14 |
| 8. 仓库布局 | 全部任务的 Files 段 |

**已知偏差**（有意为之，均在对应任务中说明）：

1. `INTERACT_WORD_V2`（protobuf）P0 不解码，走 Unknown 兜底。v1 仍在下发，信息不丢失；解码留到 P2。
2. 按用户名反查 UID 的 `@昵称` 回复形式 P0 不支持，只支持 `@UID`。该功能依赖最近弹幕缓存，属 P2 职责。
3. 包名用 `cmdmap` 而非设计文档中的 `cmd`，避免与 Go 惯例的 `cmd/` 可执行目录混淆。
