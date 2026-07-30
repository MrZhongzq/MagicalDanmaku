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
