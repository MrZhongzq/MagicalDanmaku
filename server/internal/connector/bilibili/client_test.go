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

// TestAuthenticateRespectsCtxCancellation 证明 authenticate 阶段现在
// 会响应 ctx 取消，不会傻等固定的 authTimeout（10s）——这是 Important-4
// 根因修复的直接证据：假服务器故意读完认证包之后永远不回认证回复（模拟
// 对手卡在认证阶段：可能是异常、被风控、或者恶意），50ms 后取消 ctx，
// 断言 authenticate 在远小于 authTimeout 的时间内就返回错误，而不是要
// 等满 10 秒。
//
// 把这条测试的 ctx.Done() 分支临时注释掉（即还原成旧实现）重跑，会
// 看到 authenticate 老老实实等满 authTimeout 才返回——这就是复审所说
// 「pkTeardownGraceLimit 设得比 authTimeout 还大，形同虚设」的根因，
// 本次修复之后这条路径的真实耗时应该是毫秒级，不再是 10 秒级。
func TestAuthenticateRespectsCtxCancellation(t *testing.T) {
	up := websocket.Upgrader{CheckOrigin: func(*http.Request) bool { return true }}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := up.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer conn.Close()
		conn.ReadMessage() // 读认证包，读完之后故意永远不回认证回复
		time.Sleep(20 * time.Second)
	}))
	t.Cleanup(srv.Close)

	wsURL := "ws" + strings.TrimPrefix(srv.URL, "http")
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("拨号失败: %v", err)
	}
	defer conn.Close()

	c := NewClient("21452505", api.New(nil))

	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		time.Sleep(50 * time.Millisecond)
		cancel()
	}()

	start := time.Now()
	err = c.authenticate(ctx, conn, "tok")
	elapsed := time.Since(start)

	if err == nil {
		t.Fatal("ctx 取消后 authenticate 应该返回错误")
	}
	if elapsed > 2*time.Second {
		t.Fatalf("authenticate 耗时 %v，ctx 取消后应该在毫秒级返回，不该傻等 authTimeout（10s）", elapsed)
	}
}

// TestClearEventHookIfOwnerDoesNotClobberNewerHook 覆盖 N-3：
// finishRound 曾经无条件 setEventHook(nil)。如果 Disconnect 因为
// pkTeardownGraceLimit 提前放弃等待、随后新一轮 PK 已经装上了自己的
// 钩子，旧一轮的收尾协程这时候才姗姗来迟地跑到清钩子这一步——无条件
// 清除会把新一轮正在用的钩子也摘掉，导致新一轮的 myAudience 从此停止
// 更新。owner 令牌应该能防住这个：owner 已经不是当前钩子的主人时，
// 清理动作什么都不该做。
func TestClearEventHookIfOwnerDoesNotClobberNewerHook(t *testing.T) {
	c := NewClient("21452505", api.New(nil))

	// 用 new(int) 而不是 &struct{}{}：Go 对零大小分配的地址不保证唯一
	// （运行时可能把所有 &struct{}{} 都指向同一个地址），拿它们当"两个
	// 不同身份的令牌"本身就是错的，会让这条测试自己产生假阳性/假阴性。
	ownerA := new(int) // 模拟旧一轮的 *pkRound
	ownerB := new(int) // 模拟新一轮的 *pkRound

	var calledB bool
	c.setEventHook(ownerA, func(event.Event) {})
	// 模拟"新一轮已经装上自己的钩子"，此时旧一轮的收尾协程才迟到执行。
	c.setEventHook(ownerB, func(event.Event) { calledB = true })

	c.clearEventHookIfOwner(ownerA) // 旧一轮尝试清理，owner 已经对不上了

	c.mu.Lock()
	hook := c.onEvent
	c.mu.Unlock()
	if hook == nil {
		t.Fatal("owner 不匹配时清理动作不该清掉当前钩子，但钩子已经被清空")
	}
	hook(event.Event{})
	if !calledB {
		t.Error("当前钩子应该仍然是 ownerB 装的那个，但它没有被调用到")
	}

	// 新一轮自己清理，这次 owner 对得上，应该真的清掉。
	c.clearEventHookIfOwner(ownerB)
	c.mu.Lock()
	hook = c.onEvent
	c.mu.Unlock()
	if hook != nil {
		t.Error("owner 匹配时应该清掉当前钩子，但钩子仍然存在")
	}
}
