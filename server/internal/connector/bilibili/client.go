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

	events          chan event.Event
	closeEventsOnce sync.Once

	mu          sync.RWMutex
	state       connector.State
	hostIndex   int
	deviceFixed bool // 是否已因风控补齐过设备字段
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
	buvid := ""
	if sess := c.api.Session(); sess != nil {
		uid, _ = strconv.ParseInt(sess.UID, 10, 64)
		buvid = sess.BuVID3
	}
	roomID, _ := strconv.ParseInt(c.roomID, 10, 64)

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
