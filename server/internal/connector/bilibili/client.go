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

	// defaultOpponentSnapshotBudget 是 FetchOpponentSnapshots 的整体超时预算。
	// 调用点是「PK 接通的一瞬间」，这是整场 PK 里最在乎播报延迟的时刻，
	// 拖住它比拿不全数据更糟；同时短时间内密集打同一批接口正是 -352
	// 风控的触发条件（见 riskControlCode），预算越松风控风险越高。
	// 5 秒是权衡后的取值：单个 GetJSON 请求本身有 15s 超时兜底，5 秒足够
	// 网络正常时三个接口（含 GuardOnline 的少量翻页）跑完，也足够在网络
	// 变慢时尽早放弃、把没拿到的字段降级为未知，而不是让 PK 播报干等。
	defaultOpponentSnapshotBudget = 5 * time.Second
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

// WithOpponentSnapshotBudget 设置 FetchOpponentSnapshots 的整体超时预算，
// 主要供测试用小预算触发降级路径；生产环境一般用默认值即可。
func WithOpponentSnapshotBudget(d time.Duration) ClientOption {
	return func(c *Client) { c.opponentSnapshotBudget = d }
}

// Client 是 B 站直播间的事件流连接器，实现 connector.Connector。
type Client struct {
	roomID string
	api    *api.Client
	dialer *websocket.Dialer
	log    *slog.Logger

	heartbeatInterval      time.Duration
	backoffMin             time.Duration
	backoffMax             time.Duration
	dialURLOverride        string
	opponentSnapshotBudget time.Duration

	events          chan event.Event
	closeEventsOnce sync.Once

	mu          sync.RWMutex
	state       connector.State
	hostIndex   int
	deviceFixed bool // 是否已因风控补齐过设备字段

	onEvent func(event.Event) // PK 期间的同步观察钩子，见 setEventHook
	// onEventOwner 是当前钩子的归属令牌，见 clearEventHookIfOwner。
	// 类型写成具体的 *pkRound 而不是 any：目前唯一的调用方就是
	// PkLink，用具体类型让编译器挡住误传，也避免 any 比较在不可比较
	// 类型上 panic 的理论风险（这里其实用不到那个风险，但没有理由
	// 为了一个从来只有一种调用方的场景放宽成 any）。
	onEventOwner *pkRound
	pkLink       *PkLink // 当前进行中的 PK 对面连接管理器，无 PK 时为 nil
	closed       bool    // Run 的 defer 是否已经跑过；见 registerPKLink

	// pkMu 序列化 StartPK/EndPK 之间的调用（包括 Run 退出时兜底触发的
	// 那次 EndPK）。文档化的调用方是单一事件循环，正常不会有并发调用，
	// 但审查指出：(1) Run 的 defer 本身就会在另一个 goroutine 上调
	// EndPK，并发是架构固有的，不是使用者违约；(2) 仓库里 StartPK 目前
	// 只有测试在调，没有任何东西真正强制"只从一个 goroutine 调用"这条
	// 假设。不用 mu 本身做这把锁，是因为 registerPKLink 需要在
	// StartPK 已经持有序列化锁的情况下再拿 mu 读 closed/写 pkLink，
	// 两把锁必须是不同的两个对象，否则同一 goroutine 重入会自锁死。
	pkMu sync.Mutex
}

// 确保 Client 满足 Connector 接口。
var _ connector.Connector = (*Client)(nil)

// NewClient 创建一个直播间连接器。
func NewClient(roomID string, apiClient *api.Client, opts ...ClientOption) *Client {
	c := &Client{
		roomID:                 roomID,
		api:                    apiClient,
		dialer:                 websocket.DefaultDialer,
		log:                    slog.Default(),
		heartbeatInterval:      defaultHeartbeatInterval,
		backoffMin:             defaultBackoffMin,
		backoffMax:             defaultBackoffMax,
		opponentSnapshotBudget: defaultOpponentSnapshotBudget,
		events:                 make(chan event.Event, eventBufferSize),
		state:                  connector.StateIdle,
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

		// 兜底清理：不管调用方有没有记得在 PK 结束时调用 EndPK，只要
		// 宿主连接的 Run 退出（无论是正常收尾、异常掉线还是进程整体
		// 退出），就不能再留下悬挂的对面连接——这正是本任务要堵住的
		// 泄漏风险，不能只靠注释保证。
		//
		// closed 先在 c.mu 下单独置位：StartPK 内部的 PkLink.connect
		// 会在起任何连接前调用 registerPKLink 原子地「读 closed +
		// 写 pkLink」；只要这个标志先置位，任何在此之后才走到
		// registerPKLink 的 StartPK 调用都会看到 closed=true 并自己
		// 放弃，不会真的建立连接。
		//
		// 随后的 c.EndPK() 走 pkMu 序列化：如果这一刻恰好有别的
		// goroutine 正在执行 StartPK（架构上这是并发的常态，不是使用者
		// 违约），这里会等它那次调用完整跑完（很快，因为 connect 的
		// 同步部分只是登记 + 起 goroutine，不再包含阻塞的播种 HTTP
		// 调用）再继续——这就保证了「Run 已经跑完 defer、pkLink 还没来
		// 得及登记」这个曾经真实存在的悬挂窗口（审查者用探针复现过）
		// 彻底消失，两个 goroutine 无论谁先谁后都不会漏。
		c.mu.Lock()
		c.closed = true
		c.mu.Unlock()

		c.EndPK()
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

	if err := c.authenticate(ctx, conn, di.Token); err != nil {
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
//
// ctx 取消时会立即关闭连接，不会傻等固定的 authTimeout（10s）——这跟
// pump() 里对收包阶段的处理是同一个道理，只是认证阶段之前没有补上。
// 复审指出的缺口：对手连接卡在认证阶段（对面不回认证包，可能是异常/
// 恶意/网络抖动）时，PkLink.disconnect 靠 ctx 取消让子连接尽快退出，
// 如果 authenticate 不感知 ctx，这个「尽快」在认证阶段就落空了，
// 只能傻等 10 秒——pkTeardownGraceLimit 那道兜底上限本来是为了防止
// 这类情况把宿主退出流程拖住，但如果这里从根上就不会卡 10 秒，那道
// 上限就真的只是"理论上限"，不是"日常要付的代价"。
func (c *Client) authenticate(ctx context.Context, conn *websocket.Conn, token string) error {
	uid := int64(0)
	buvid := ""
	if sess := c.api.Session(); sess != nil {
		uid, _ = strconv.ParseInt(sess.UID, 10, 64)
		// PK 场景下多个 Client 共享同一个 *Session，BuVID3 可能被另一个
		// goroutine 的 EnsureDeviceFields 并发写，必须走线程安全的
		// 访问方法，不能直接读字段（Critical-2 修复）。
		buvid = sess.BuVID3()
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

	// ctx 取消时立即关闭连接。必须装在 WriteMessage 之前：仓库里没有
	// 任何地方对这条连接设置过 SetWriteDeadline，如果对端 TCP 接收
	// 窗口已经填满（异常/恶意对手常见的表现），WriteMessage 本身就会
	// 无限阻塞，跟下面的读超时无关——这才是 pkTeardownGraceLimit 真正
	// 要兜的场景，早前把 watcher 只挡在读之前是不完整的。watcherDone
	// 保证 authenticate 正常返回时这个 goroutine 也会跟着退出，不会
	// 泄漏。
	watcherDone := make(chan struct{})
	defer close(watcherDone)
	go func() {
		select {
		case <-ctx.Done():
			conn.Close()
		case <-watcherDone:
		}
	}()

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

	// PK 期间 PkLink 会挂一个观察钩子来同步维护观众集合；只读一次快照，
	// 不在每条事件上都重新加锁。钩子不改变下面的投递路径，纯旁路观察。
	c.mu.RLock()
	hook := c.onEvent
	c.mu.RUnlock()

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
			if hook != nil {
				hook(ev)
			}
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

// setEventHook 设置一个同步事件观察钩子，目前只给 PkLink 用来维护 PK
// 观众集合。钩子不改变事件投递路径（该丢的还是丢、该发的还是发），
// 只是在事件产生的同一时刻做一次旁路观察。owner 是这次设置的归属
// 令牌（PkLink 传自己那一轮的 *pkRound），配合 clearEventHookIfOwner
// 使用，防止过期的清理动作摘掉新一轮已经装上的钩子。
func (c *Client) setEventHook(owner *pkRound, fn func(event.Event)) {
	c.mu.Lock()
	c.onEvent = fn
	c.onEventOwner = owner
	c.mu.Unlock()
}

// clearEventHookIfOwner 仅当当前钩子仍然属于 owner 时才摘除。
//
// 复审指出的坑：finishRound 曾经无条件 setEventHook(nil)。如果
// disconnect 因为 pkTeardownGraceLimit 提前放弃等待而返回，随后新一轮
// StartPK 装上了自己的钩子，旧一轮的收尾协程这时候才姗姗来迟地执行到
// 这一步，无条件清除会把新一轮正在用的钩子也摘掉，导致新一轮的
// myAudience 从此停止更新。用 owner 令牌判断"这把钩子还是不是我挂的"，
// 不是我挂的就什么都不做。
func (c *Client) clearEventHookIfOwner(owner *pkRound) {
	c.mu.Lock()
	if c.onEventOwner == owner {
		c.onEvent = nil
		c.onEventOwner = nil
	}
	c.mu.Unlock()
}

// StartPK 为 PK 的每一个对手房间各起一条弹幕连接，事件经返回的 PkLink
// 统一交付。这不是 c 自身连接的一部分：对面下播/连不上/风控都只影响
// PkLink 内部，c 的事件流和状态机完全不受影响。
//
// 调用方通常在观测到 PK_INFO（event.Battle.Members 非空）时调用一次；
// 即使调用方忘了在 PK 结束时调用 EndPK、甚至宿主 Run 已经退出，这次
// 调用也不会产生悬挂连接——具体怎么保证的，见 PkLink.connect 里
// registerPKLink 那一段注释。
//
// 用 pkMu 跟 EndPK（含 Run 退出时兜底触发的那次）互斥：如果没有这把
// 锁，两个几乎同时发生的 StartPK 调用会各自往 c.pkLink 写一次，后写
// 的会静默覆盖先写的——先注册的那个 PkLink 从此失去引用，将来任何一次
// EndPK 都不会再找到它，变成悬挂连接。文档化的调用约定是单一事件循环
// 串行调用，正常不会触发这一路，但审查明确要求现在就补上，不留成
// "调用方自觉遵守"。
func (c *Client) StartPK(ctx context.Context, members []event.PkMember) *PkLink {
	c.pkMu.Lock()
	defer c.pkMu.Unlock()

	c.endPKLocked() // 防御性收尾：不允许两场 PK 的连接叠加

	link := newPkLink(c)
	link.connect(ctx, members)
	return link
}

// EndPK 断开当前 PK 期间连到对面房间的全部连接（PK 正常结束时调用）。
// 幂等：没有进行中的 PK 时是安全的空操作。
func (c *Client) EndPK() {
	c.pkMu.Lock()
	defer c.pkMu.Unlock()
	c.endPKLocked()
}

// endPKLocked 是 EndPK 的核心逻辑，调用方必须已经持有 c.pkMu。
func (c *Client) endPKLocked() {
	c.mu.Lock()
	link := c.pkLink
	c.pkLink = nil
	c.mu.Unlock()

	if link != nil {
		link.disconnect()
	}
}

// PKLink 返回当前进行中的 PK 连接管理器，没有 PK 时为 nil。
func (c *Client) PKLink() *PkLink {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.pkLink
}

// registerPKLink 把 link 原子地登记为当前 PK，同时返回宿主此刻是否
// 已经关闭（Run 的 defer 已经跑完）。
//
// 这是堵住「先连接/播种、后登记，宿主退出兜底失效」这个窗口的关键：
// 调用方（PkLink.connect）必须在起任何连接、做任何阻塞工作之前就调用
// 这个方法。因为它跟 Run 的 defer 共用同一把 c.mu 做「读 closed +
// 写 pkLink」这个组合操作，两个 goroutine 不管谁先谁后都不会漏——
// 要么这里先跑完，Run 的 defer 稍后一定能读到刚登记的 link 并断开它；
// 要么 Run 的 defer 先跑完（closed 已经是 true），这里会如实告知调用方
// 「宿主已经关闭」，调用方应该立刻自行收尾、不再建立任何真实连接。
func (c *Client) registerPKLink(link *PkLink) (hostClosed bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closed {
		return true
	}
	c.pkLink = link
	return false
}
