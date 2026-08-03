package bilibili

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"google.golang.org/protobuf/encoding/protowire"

	"github.com/MrZhongzq/MagicalDanmaku/server/internal/connector/bilibili/api"
	"github.com/MrZhongzq/MagicalDanmaku/server/internal/connector/bilibili/auth"
	"github.com/MrZhongzq/MagicalDanmaku/server/internal/connector/bilibili/wire"
	"github.com/MrZhongzq/MagicalDanmaku/server/internal/event"
)

// ---------- 通用测试辅助 ----------

// waitUntil 轮询 cond 直到为真或超时，超时则 Fatal。
func waitUntil(t *testing.T, timeout time.Duration, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	if !cond() {
		t.Fatalf("等待条件超时（%v）", timeout)
	}
}

// authRoomID 解出一个认证包里的 roomid 字段。
func authRoomID(t *testing.T, pkt []byte) string {
	t.Helper()
	var body struct {
		RoomID int64 `json:"roomid"`
	}
	if err := json.Unmarshal(pkt[wire.HeaderSize:], &body); err != nil {
		t.Fatalf("解析认证包失败: %v", err)
	}
	return strconv.FormatInt(body.RoomID, 10)
}

// waitForDistinctRooms 轮询直到假服务器已经观测到 rooms 里每一个房间号
// 各自的认证包，证明「这些不同房间各自建立了连接」。不能用单纯数
// fs.authCount() 是否达到某个总数替代：假服务器每 500ms 会主动断连，
// 如果对手其实压根没连上，光宿主自己反复重连也能把总数刷上去，凑够
// 数字并不能证明对手真的连上了——这是审查指出的一处弱断言。
func waitForDistinctRooms(t *testing.T, fs *multiRoomFakeServer, rooms ...string) {
	t.Helper()
	waitUntil(t, time.Second, func() bool {
		seen := make(map[string]bool, len(rooms))
		for _, r := range fs.authedRooms() {
			seen[r] = true
		}
		for _, want := range rooms {
			if !seen[want] {
				return false
			}
		}
		return true
	})
}

// goroutineBaseline 拍一张 goroutine 数快照。必须在 fs/httpSrv/Client 等
// 测试基础设施都已建好、宿主也已经连上之后再调用——httptest.Server 自己
// 的 accept 循环、http.Transport 的 keep-alive 连接读写协程只会在
// t.Cleanup（测试函数返回之后）才会被摘掉，如果 baseline 拍在这些基础
// 设施创建之前，比对时永远会把它们错判成「PK 泄漏的 goroutine」。
func goroutineBaseline(t *testing.T) int {
	t.Helper()
	runtime.GC()
	time.Sleep(20 * time.Millisecond)
	return runtime.NumGoroutine()
}

// assertSettles 等待 goroutine 数回落到 baseline 附近（允许少量测试框架
// 自身的抖动），超时未回落则判定为泄漏。
func assertSettles(t *testing.T, baseline int) {
	t.Helper()
	var last int
	waitUntilNoFatal(2*time.Second, func() bool {
		runtime.GC()
		last = runtime.NumGoroutine()
		return last <= baseline+2
	})
	if last > baseline+2 {
		t.Errorf("goroutine 数未回落: baseline=%d, 结束时=%d（疑似泄漏）", baseline, last)
	}
}

// waitUntilNoFatal 跟 waitUntil 类似，但不依赖 *testing.T，纯粹轮询到
// 条件成立或超时为止，调用方自行判断结果。
func waitUntilNoFatal(timeout time.Duration, cond func() bool) {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
}

// multiRoomFakeServer 是能同时服务多个「房间」的假弹幕服务器：每条连接
// 独立握手、独立记录认证包，可选地在认证成功后按 onConnected 推事件。
type multiRoomFakeServer struct {
	srv *httptest.Server

	mu       sync.Mutex
	authPkts [][]byte

	onConnected func(c *websocket.Conn, roomID string)
}

func newMultiRoomFakeServer(t *testing.T, onConnected func(*websocket.Conn, string)) *multiRoomFakeServer {
	t.Helper()
	fs := &multiRoomFakeServer{onConnected: onConnected}

	up := websocket.Upgrader{CheckOrigin: func(*http.Request) bool { return true }}
	fs.srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c, err := up.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer c.Close()

		_, msg, err := c.ReadMessage()
		if err != nil {
			return
		}
		fs.mu.Lock()
		fs.authPkts = append(fs.authPkts, append([]byte(nil), msg...))
		fs.mu.Unlock()

		var body struct {
			RoomID int64 `json:"roomid"`
		}
		json.Unmarshal(msg[wire.HeaderSize:], &body)
		roomID := strconv.FormatInt(body.RoomID, 10)

		c.WriteMessage(websocket.BinaryMessage, wire.Encode(wire.OpAuthReply, []byte(`{"code":0}`)))

		go func() {
			for {
				if _, _, err := c.ReadMessage(); err != nil {
					return
				}
			}
		}()

		if fs.onConnected != nil {
			fs.onConnected(c, roomID)
		}
		time.Sleep(500 * time.Millisecond)
	}))
	t.Cleanup(fs.srv.Close)
	return fs
}

func (fs *multiRoomFakeServer) wsURL() string {
	return "ws" + strings.TrimPrefix(fs.srv.URL, "http")
}

func (fs *multiRoomFakeServer) authedRooms() []string {
	fs.mu.Lock()
	defer fs.mu.Unlock()
	rooms := make([]string, 0, len(fs.authPkts))
	for _, p := range fs.authPkts {
		var body struct {
			RoomID int64 `json:"roomid"`
		}
		json.Unmarshal(p[wire.HeaderSize:], &body)
		rooms = append(rooms, strconv.FormatInt(body.RoomID, 10))
	}
	return rooms
}

func (fs *multiRoomFakeServer) authCount() int {
	fs.mu.Lock()
	defer fs.mu.Unlock()
	return len(fs.authPkts)
}

// newPKHostClient 构造一台可以正常握手 danmuInfo/roomInfo 的宿主 Client，
// 所有房间（宿主自己 + PkLink 起的对手连接）都拨向同一个假 WS 服务器
// ——这跟生产环境一致：同一个 api.Client、同一份鉴权信息，只是房间号
// 不同。
func newPKHostClient(t *testing.T, fs *multiRoomFakeServer, roomID string, opts ...ClientOption) *Client {
	t.Helper()

	sess, err := auth.ParseSession("SESSDATA=x; bili_jct=tok; DedeUserID=42; buvid3=BV3")
	if err != nil {
		t.Fatalf("ParseSession 失败: %v", err)
	}

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
		case strings.Contains(r.URL.Path, "audience"):
			// seedAudiences 在 connect 内部的独立 goroutine 里播种观众
			// 集合，必须本地兜底，否则会真的打到外网的 ajax/msg，把
			// 测试拖慢到预算超时（5s）。
			w.Write([]byte(`{"code":0,"data":{"room":[]}}`))
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
	ac.SetBaseURL("roomAudience", httpSrv.URL+"/audience")
	ac.Signer().SetMixinKey("abcdefghijklmnopqrstuvwxyz012345")

	all := append([]ClientOption{
		WithDialURLOverride(fs.wsURL()),
		WithHeartbeatInterval(50 * time.Millisecond),
		WithBackoff(10*time.Millisecond, 30*time.Millisecond),
	}, opts...)

	return NewClient(roomID, ac, all...)
}

// ---------- 多对手连接 ----------

func TestStartPKConnectsOneClientPerOpponent(t *testing.T) {
	fs := newMultiRoomFakeServer(t, nil)
	c := newPKHostClient(t, fs, "21452505")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go c.Run(ctx)

	members := []event.PkMember{
		{RoomID: "21452505", UID: "u-self"}, // 自己，不应另开连接
		{RoomID: "33333", UID: "u33333"},
		{RoomID: "44444", UID: "u44444"},
	}
	c.StartPK(ctx, members)

	// 宿主自己 + 2 个对手，各自都要建立连接。
	waitForDistinctRooms(t, fs, "21452505", "33333", "44444")

	c.EndPK()
}

// TestStartPKSupportsMoreThanTwoOpponents 证明「对手是复数」：三个对手都
// 应各自建一条连接，不写死两条。
func TestStartPKSupportsMoreThanTwoOpponents(t *testing.T) {
	fs := newMultiRoomFakeServer(t, nil)
	c := newPKHostClient(t, fs, "21452505")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go c.Run(ctx)

	members := []event.PkMember{
		{RoomID: "21452505", UID: "u-self"},
		{RoomID: "11111", UID: "u1"},
		{RoomID: "22222", UID: "u2"},
		{RoomID: "33333", UID: "u3"},
	}
	c.StartPK(ctx, members)

	// 宿主 + 三个对手，各自都要建立连接，不写死两条。
	waitForDistinctRooms(t, fs, "21452505", "11111", "22222", "33333")

	c.EndPK()
}

// ---------- 事件来源标记 ----------

// TestOpponentEventsCarryOpponentRoomID 验证「来源标记」这个硬性约束：
// 并进主事件流之后必须能分清「我房间有人进来」和「对面房间有人进来」。
// 这里不新造一个字段，直接用 Event.RoomID——每个对手连接自己是一个
// 独立 Client，天然只会给自己房间的事件盖上自己的 RoomID，PkLink 转发
// 时原样带过去，不会跟宿主自己的事件混淆。
func TestOpponentEventsCarryOpponentRoomID(t *testing.T) {
	danmakuFor := func(roomID string) []byte {
		body := `{"cmd":"DANMU_MSG","info":[[0,1,25,16777215,1700000000000],"来自` + roomID + `",` +
			`[1,"user` + roomID + `",0,0,0,10000,1,""],[],[10,0,0,0],["",""],0,0,null,null,0,0,[3]]}`
		return wire.Encode(wire.OpMessage, []byte(body))
	}

	fs := newMultiRoomFakeServer(t, func(c *websocket.Conn, roomID string) {
		c.WriteMessage(websocket.BinaryMessage, danmakuFor(roomID))
	})
	c := newPKHostClient(t, fs, "21452505")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go c.Run(ctx)

	members := []event.PkMember{
		{RoomID: "21452505", UID: "u-self"},
		{RoomID: "33333", UID: "u33333"},
	}
	link := c.StartPK(ctx, members)

	var hostEv, oppEv event.Event
	var gotHost, gotOpp bool
	deadline := time.After(2 * time.Second)
	for !gotHost || !gotOpp {
		select {
		case ev := <-c.Events():
			if ev.Type == event.TypeDanmaku {
				hostEv = ev
				gotHost = true
			}
		case ev := <-link.Events():
			if ev.Type == event.TypeDanmaku {
				oppEv = ev
				gotOpp = true
			}
		case <-deadline:
			t.Fatalf("超时：gotHost=%v gotOpp=%v", gotHost, gotOpp)
		}
	}

	if hostEv.RoomID != "21452505" {
		t.Errorf("宿主事件 RoomID = %q, 期望 21452505", hostEv.RoomID)
	}
	if oppEv.RoomID != "33333" {
		t.Errorf("对手事件 RoomID = %q, 期望 33333（必须能跟宿主自己的事件区分开）", oppEv.RoomID)
	}

	c.EndPK()
}

// ---------- 对面连接失败不影响主房间 ----------

// TestOpponentConnectFailureDoesNotAffectMainRoom 模拟对面房间已下播/
// 连不上（danmuInfo 接口对特定房间号永远失败）：宿主自己的连接与事件流
// 必须完全不受影响，PkLink 那一侧则应该「没有对面事件」而不是让宿主
// 也跟着出问题。
func TestOpponentConnectFailureDoesNotAffectMainRoom(t *testing.T) {
	sess, err := auth.ParseSession("SESSDATA=x; bili_jct=tok; DedeUserID=42; buvid3=BV3")
	if err != nil {
		t.Fatalf("ParseSession 失败: %v", err)
	}

	httpSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.Contains(r.URL.Path, "danmu"):
			// 对手房间 99999 的 danmuInfo 永远失败（模拟下播/风控/连不上）。
			if r.URL.Query().Get("id") == "99999" {
				json.NewEncoder(w).Encode(map[string]any{"code": -400, "message": "房间不存在"})
				return
			}
			json.NewEncoder(w).Encode(map[string]any{
				"code": 0,
				"data": map[string]any{
					"token": "tok-abc",
					"host_list": []map[string]any{
						{"host": "unused", "wss_port": 443, "port": 2243, "ws_port": 2244},
					},
				},
			})
		case strings.Contains(r.URL.Path, "audience"):
			// 必须本地兜底，否则播种观众集合会真的打到外网的 ajax/msg。
			w.Write([]byte(`{"code":0,"data":{"room":[]}}`))
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
	ac.SetBaseURL("roomAudience", httpSrv.URL+"/audience")
	ac.Signer().SetMixinKey("abcdefghijklmnopqrstuvwxyz012345")

	danmaku := wire.Encode(wire.OpMessage, []byte(
		`{"cmd":"DANMU_MSG","info":[[0,1,25,16777215,1700000000000],"主房间弹幕",`+
			`[1,"host",0,0,0,10000,1,""],[],[10,0,0,0],["",""],0,0,null,null,0,0,[3]]}`))
	fs := newMultiRoomFakeServer(t, func(c *websocket.Conn, roomID string) {
		if roomID == "21452505" {
			c.WriteMessage(websocket.BinaryMessage, danmaku)
		}
	})

	c := NewClient("21452505", ac,
		WithDialURLOverride(fs.wsURL()),
		WithHeartbeatInterval(50*time.Millisecond),
		WithBackoff(10*time.Millisecond, 30*time.Millisecond),
	)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go c.Run(ctx)

	members := []event.PkMember{
		{RoomID: "21452505", UID: "u-self"},
		{RoomID: "99999", UID: "u-bad"}, // 连不上的对手
	}
	link := c.StartPK(ctx, members)

	// 宿主自己的事件流必须正常收到事件，完全不受对手连不上影响。
	select {
	case ev := <-c.Events():
		if ev.Type != event.TypeDanmaku || ev.RoomID != "21452505" {
			t.Errorf("宿主事件异常: type=%s room=%s", ev.Type, ev.RoomID)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("宿主房间应正常收到事件，不该被对手连接失败拖累")
	}

	// 对面连不上，link.Events() 在短时间内不应该出现任何事件。
	select {
	case ev := <-link.Events():
		t.Errorf("对面连不上时不应产生事件，实际收到: %+v", ev)
	case <-time.After(200 * time.Millisecond):
	}

	c.EndPK()
}

// ---------- 四条清理路径：不留悬挂连接/goroutine ----------

func startPKMembers(selfRoom string, opponents ...string) []event.PkMember {
	members := []event.PkMember{{RoomID: selfRoom, UID: "u-self"}}
	for _, r := range opponents {
		members = append(members, event.PkMember{RoomID: r, UID: "u-" + r})
	}
	return members
}

// TestPKEndCleanlyDisconnectsAllOpponents 覆盖路径①：PK 正常结束——
// 显式调用 EndPK，之后不应残留任何对手连接对应的 goroutine。
func TestPKEndCleanlyDisconnectsAllOpponents(t *testing.T) {
	fs := newMultiRoomFakeServer(t, nil)
	c := newPKHostClient(t, fs, "21452505")

	ctx, cancel := context.WithCancel(context.Background())
	go c.Run(ctx)
	waitUntil(t, time.Second, func() bool { return fs.authCount() >= 1 })

	// baseline 必须在宿主自己稳定连接之后才拍：httptest 服务器的 accept
	// 循环、http.Transport 的长连接读写协程这时候已经跑起来了，不应该被
	// 后面的比对误判成 PK 泄漏。
	base := goroutineBaseline(t)

	c.StartPK(ctx, startPKMembers("21452505", "33333", "44444"))
	waitForDistinctRooms(t, fs, "21452505", "33333", "44444")

	c.EndPK()
	cancel()

	// 排空事件通道，确保 Run 的 goroutine 能顺利退出。
	for range c.Events() {
	}

	assertSettles(t, base)
}

// TestPKAbnormalCtxCancelStillCleansUp 覆盖路径②：PK 异常结束——不经过
// EndPK，而是直接取消传给 StartPK 的那个 ctx（例如上层因为出错主动砍掉
// 这一场 PK 的上下文）。清理必须照样发生，不能只在 EndPK 里写。
func TestPKAbnormalCtxCancelStillCleansUp(t *testing.T) {
	fs := newMultiRoomFakeServer(t, nil)
	c := newPKHostClient(t, fs, "21452505")

	hostCtx, hostCancel := context.WithCancel(context.Background())
	go c.Run(hostCtx)
	waitUntil(t, time.Second, func() bool { return fs.authCount() >= 1 })

	base := goroutineBaseline(t)

	pkCtx, pkCancel := context.WithCancel(context.Background())
	c.StartPK(pkCtx, startPKMembers("21452505", "33333", "44444"))
	waitForDistinctRooms(t, fs, "21452505", "33333", "44444")

	// 异常结束：直接砍 PK 自己的 ctx，不调用 EndPK。
	pkCancel()

	// disconnect 语义上应该能等到清理完成；这里通过 EndPK 再调用一次
	// 验证其幂等（此时清理可能已经由 ctx 取消触发完成）。
	c.EndPK()

	hostCancel()
	for range c.Events() {
	}

	assertSettles(t, base)
}

// TestHostRunExitClosesPKLinkEvenWithoutEndPK 覆盖路径③：主连接退出——
// 调用方忘了调用 EndPK，宿主 Client.Run 自己退出时必须兜底清理，
// 不能把对手连接留在半空中。
//
// 关键细节：StartPK 故意传 context.Background()，跟宿主 Run 自己的 ctx
// 完全独立——如果传同一个 ctx，取消它会同时经由 ctx 传播关掉 PK 连接，
// 那样测的其实是 ctx 传播这条路（路径②已经覆盖），而不是 Run 的 defer
// 里那句 c.EndPK() 兜底清理本身。这里必须让 PkLink 的存活跟 Run 的 ctx
// 彻底脱钩，才能真正证明「调用方忘了 EndPK、也没把 ctx 挂在一起」时，
// Run 退出仍然会把 PK 连接收干净。
func TestHostRunExitClosesPKLinkEvenWithoutEndPK(t *testing.T) {
	fs := newMultiRoomFakeServer(t, nil)
	c := newPKHostClient(t, fs, "21452505")

	ctx, cancel := context.WithCancel(context.Background())
	runDone := make(chan struct{})
	go func() { c.Run(ctx); close(runDone) }()
	waitUntil(t, time.Second, func() bool { return fs.authCount() >= 1 })

	base := goroutineBaseline(t)

	link := c.StartPK(context.Background(), startPKMembers("21452505", "33333", "44444"))
	waitForDistinctRooms(t, fs, "21452505", "33333", "44444")

	// 故意不调用 EndPK、也不去碰 PK 自己的 ctx，直接让宿主退出
	// （模拟异常/主连接被上层关闭，调用方完全没顾上 PK 那一摊）。
	cancel()
	<-runDone

	// Run 退出后，PkLink 的事件通道也应该被关闭（不再有事件可能到来）。
	select {
	case _, ok := <-link.Events():
		if ok {
			t.Error("Run 退出后 link.Events() 仍在产出事件")
		}
	case <-time.After(time.Second):
		t.Fatal("Run 退出后 link.Events() 应该已关闭，而不是继续阻塞")
	}

	assertSettles(t, base)
}

// TestNoLeakAcrossRepeatedPKRounds 覆盖路径④：进程退出前反复经历多场
// PK（贴近真实运行：一个直播间一晚可能打好几场乱斗），每一场都必须
// 干净收尾，不能慢慢攒 goroutine，最后随进程退出统一验证。
func TestNoLeakAcrossRepeatedPKRounds(t *testing.T) {
	fs := newMultiRoomFakeServer(t, nil)
	c := newPKHostClient(t, fs, "21452505")

	ctx, cancel := context.WithCancel(context.Background())
	go c.Run(ctx)
	waitUntil(t, time.Second, func() bool { return fs.authCount() >= 1 })

	base := goroutineBaseline(t)

	for i := 0; i < 5; i++ {
		c.StartPK(ctx, startPKMembers("21452505", "33333", "44444"))
		// 故意只是给连接一点建立的时间，不逐轮验证握手是否成功——
		// fs.authedRooms()/authCount() 是跨全部历史连接累积的，一旦
		// 第一轮真的连上过，后续轮次即使连接失败也会在累积视图里
		// 显得"看起来连上了"，据此做逐轮断言只会是自欺欺人。这里要
		// 测的是反复 Start/End 不积累 goroutine 泄漏，不是逐轮连通性，
		// 连通性已经由前面几个测试单独验证过。
		time.Sleep(30 * time.Millisecond)
		c.EndPK()
	}

	cancel()
	for range c.Events() {
	}

	assertSettles(t, base)
}

// ---------- Critical-1 回归：注册必须先于阻塞工作 ----------

// TestStartPKDuringHostShutdownDoesNotLeak 测的是纵深防御，不是生产
// 路径上唯一承重的那道防线——这一点必须先说清楚，否则容易被后人误读。
//
// 【第三轮复审订正】真正扛住"宿主退出时 PK 连接变成孤儿"的，是
// registerPKLink 里的 closed 检查，不是它在 connect 内部被调用的
// 先后位置：
//
//   - Run 的 defer 先在 c.mu 下把 closed 置成 true，再走 EndPK；
//     registerPKLink 在同一把 c.mu 下把"读 closed + 写 pkLink"做成
//     一次原子操作。两种加锁顺序穷尽了全部交错可能，结果都安全——
//     这跟登记发生在 seedAudiences 之前还是之后完全无关。
//   - 生产路径上这道防线甚至连"被绕过"的机会都没有：StartPK 全程
//     持有 pkMu，EndPK（含 Run defer 触发的那次）也要先抢到 pkMu
//     才能碰 pkLink，两者根本不可能在 connect() 内部产生交错。也就
//     是说，在真实调用路径下，connect 内部"先播种再登记"还是"先
//     登记再播种"对防泄漏是零贡献——它的价值在于 Important-3（不
//     阻塞调用方的事件循环），不在于防泄漏。
//
// 本测试之所以还能观察到"登记顺序"的影响，是因为它**故意绕开了
// pkMu**、直接调 link.connect（connect 已经不导出，包外没有任何
// 途径能这样调用）。第二轮曾经错误地下结论说"单独去掉 closed 检查
// 也不会让测试变红，所以早登记本身就是充分条件"——那是样本太少
// （几次而非几百次）得出的假结论：复审用约 200 次重跑，仅去掉 closed
// 检查（保留早登记）复现出了约 1% 的红；只有同时"播种挪回同步 + 登记
// 挪到阻塞工作之后 + 去掉 closed 检查"才能稳定 100% 复现。换句话说，
// 早登记只是把交错窗口从"几秒（N+1 次 HTTP）"压缩到"几微秒"，从未
// 真正消除它——不能把"压缩窗口"和"关闭窗口"混为一谈。
//
// closed 检查这道真正承重的防线，有确定性覆盖，但覆盖它的是**另一条**
// 测试：TestStartPKAfterHostAlreadyClosedIsNoop（去掉 closed 检查后
// 5/5 必定失败，见该测试注释）。这条测试留着，是为了在纵深防御的
// 第二层（早登记缩小窗口）上也有验证，不是因为它测的是生产路径的
// 唯一防线。
//
// 测试写法本身（供理解代码用，不再是"防线在哪"的证据）：pkCtx 独立于
// 宿主 ctx（否则 ctx 传播会自己把连接收干净，掩盖登记时机的影响）；
// roomAudience 接口人为拖慢到 200ms，把交错窗口拉宽到跟调度时机无关；
// link.connect(pkCtx, ...) 和 cancel()（只取消宿主自己的 ctx）从两个
// 独立 goroutine 同时发起，绕开 pkMu 的序列化。
func TestStartPKDuringHostShutdownDoesNotLeak(t *testing.T) {
	const slowSeedDelay = 200 * time.Millisecond

	sess, err := auth.ParseSession("SESSDATA=x; bili_jct=tok; DedeUserID=42; buvid3=BV3")
	if err != nil {
		t.Fatalf("ParseSession 失败: %v", err)
	}

	httpSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.Contains(r.URL.Path, "danmu"):
			json.NewEncoder(w).Encode(map[string]any{
				"code": 0,
				"data": map[string]any{
					"token":     "tok-abc",
					"host_list": []map[string]any{{"host": "unused", "wss_port": 443}},
				},
			})
		case strings.Contains(r.URL.Path, "audience"):
			// 故意拖慢，把"同步播种"的窗口拉宽到远超调度抖动的量级。
			time.Sleep(slowSeedDelay)
			w.Write([]byte(`{"code":0,"data":{"room":[]}}`))
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
	ac.SetBaseURL("roomAudience", httpSrv.URL+"/audience")
	ac.Signer().SetMixinKey("abcdefghijklmnopqrstuvwxyz012345")

	fs := newMultiRoomFakeServer(t, nil)
	c := NewClient("21452505", ac,
		WithDialURLOverride(fs.wsURL()),
		WithHeartbeatInterval(50*time.Millisecond),
		WithBackoff(10*time.Millisecond, 30*time.Millisecond),
	)

	ctx, cancel := context.WithCancel(context.Background()) // 宿主 Run 自己的 ctx
	runDone := make(chan struct{})
	go func() { c.Run(ctx); close(runDone) }()
	waitUntil(t, time.Second, func() bool { return fs.authCount() >= 1 })

	base := goroutineBaseline(t)

	// pkCtx 独立于宿主 ctx——这是让测试真正有效的关键，见函数注释第 3 点。
	pkCtx := context.Background()
	link := newPkLink(c)
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		link.connect(pkCtx, startPKMembers("21452505", "33333", "44444"))
	}()
	go func() {
		defer wg.Done()
		cancel() // 只取消宿主自己的 ctx，不碰 pkCtx
	}()
	wg.Wait() // 建立 happens-before：下面读 link 是安全的

	select {
	case <-runDone:
	case <-time.After(2 * time.Second):
		t.Fatal("Run 迟迟不退出（怀疑被慢速播种请求拖住，正确实现不应该发生）")
	}

	// pkCtx 从未被取消，per-opponent 连接不会因为 ctx 传播自己退出——
	// 唯一能让这场 PK 被清理掉的机制就是 Run 的 defer 里那次 EndPK
	// 找到 registerPKLink 登记的 pkLink 并 disconnect。
	select {
	case _, ok := <-link.Events():
		if ok {
			t.Error("Run 退出后 link.Events() 仍在产出事件")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Run 退出后 link.Events() 应该已关闭，而不是继续阻塞——" +
			"说明 connect 与宿主退出发生竞态时，这一轮 PK 变成了孤儿")
	}

	if l := c.PKLink(); l != nil {
		t.Error("宿主退出后 c.PKLink() 应该为 nil")
	}

	assertSettles(t, base)
}

// TestStartPKAfterHostAlreadyClosedIsNoop 覆盖 Critical-1 的另一半，
// 也是 registerPKLink 里 closed 检查——生产路径上真正唯一承重的防线
// ——的确定性覆盖（跟 TestStartPKDuringHostShutdownDoesNotLeak 分工：
// 那条测的是纵深防御第二层，早登记缩小交错窗口；这条测的是这道检查
// 本身，去掉它 5/5 必定失败，见「第三轮复审订正」的变异测试记录）：
// Run 已经完全退出（defer 已经跑完，closed 已经是 true）之后才调用
// StartPK——这次调用不该建立任何真实连接，也不该产生悬挂的 goroutine。
func TestStartPKAfterHostAlreadyClosedIsNoop(t *testing.T) {
	fs := newMultiRoomFakeServer(t, nil)
	c := newPKHostClient(t, fs, "21452505")

	ctx, cancel := context.WithCancel(context.Background())
	runDone := make(chan struct{})
	go func() { c.Run(ctx); close(runDone) }()
	waitUntil(t, time.Second, func() bool { return fs.authCount() >= 1 })

	cancel()
	<-runDone

	base := goroutineBaseline(t)
	before := fs.authCount()

	link := c.StartPK(context.Background(), startPKMembers("21452505", "33333", "44444"))

	time.Sleep(100 * time.Millisecond)
	if got := fs.authCount(); got != before {
		t.Errorf("宿主已退出后 StartPK 不应该建立任何新连接，连接次数从 %d 变成了 %d", before, got)
	}

	select {
	case _, ok := <-link.Events():
		if ok {
			t.Error("宿主已关闭后 StartPK 返回的 link 不应该产出事件")
		}
	default:
		t.Error("宿主已关闭后 link.Events() 应该已经关闭（能立刻读到零值），而不是继续阻塞")
	}

	if l := c.PKLink(); l != nil {
		t.Error("宿主已关闭时 c.PKLink() 应该为 nil，不应该登记这次徒劳的 StartPK")
	}

	assertSettles(t, base)
}

// TestConcurrentStartPKDoesNotOrphanEarlierLink 覆盖驳回后的 concern-4：
// 两个几乎同时发生的 StartPK 调用不该有一个静默失去引用变成孤儿。
// 没有序列化的话，两次调用各自往 c.pkLink 写一次，后写的会覆盖先写的，
// 先注册的那个 PkLink 从此再也不会被任何一次 EndPK 找到；pkMu 序列化
// StartPK 之间的调用后，不管谁先谁后，落败的那一个都会在赢家的
// StartPK 内部（endPKLocked）被干净地 EndPK 掉，不是被覆盖后放任不管。
func TestConcurrentStartPKDoesNotOrphanEarlierLink(t *testing.T) {
	fs := newMultiRoomFakeServer(t, nil)
	c := newPKHostClient(t, fs, "21452505")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go c.Run(ctx)
	waitUntil(t, time.Second, func() bool { return fs.authCount() >= 1 })

	base := goroutineBaseline(t)

	var wg sync.WaitGroup
	links := make([]*PkLink, 2)
	wg.Add(2)
	go func() {
		defer wg.Done()
		links[0] = c.StartPK(ctx, startPKMembers("21452505", "33333"))
	}()
	go func() {
		defer wg.Done()
		links[1] = c.StartPK(ctx, startPKMembers("21452505", "44444"))
	}()
	wg.Wait()

	current := c.PKLink()
	if current != links[0] && current != links[1] {
		t.Fatal("c.PKLink() 既不是 links[0] 也不是 links[1]")
	}
	orphan := links[0]
	if current == links[0] {
		orphan = links[1]
	}

	select {
	case _, ok := <-orphan.Events():
		if ok {
			t.Error("落败的那个 PkLink 应该已经被 EndPK 掉，Events() 不该再产出事件")
		}
	case <-time.After(time.Second):
		t.Fatal("落败的那个 PkLink 应该已经关闭，而不是继续阻塞——说明它变成了孤儿")
	}

	c.EndPK()
	cancel()
	for range c.Events() {
	}

	assertSettles(t, base)
}

// ---------- 观众集合 ----------

// TestPKSeedsAudienceSetsFromRecentDanmakuAndSelves 验证播种逻辑：
// 双方房间各自的近期弹幕发送者 + 自己主播 + 对面主播都应该进各自的集合。
//
// DedeUserID（登录账号 uid，999999）故意跟 PK_INFO 里「自己」这一项的
// UID（本房间主播 uid，"room-anchor-uid"）设成不同的值——工具很可能是
// 助播/小号账号登录、给别的主播的房间跑，这两个 uid 本来就该是两个人。
// myAudience 播种的必须是后者，不是登录账号（审查发现的真实缺陷）。
func TestPKSeedsAudienceSetsFromRecentDanmakuAndSelves(t *testing.T) {
	sess, err := auth.ParseSession("SESSDATA=x; bili_jct=tok; DedeUserID=999999; buvid3=BV3")
	if err != nil {
		t.Fatalf("ParseSession 失败: %v", err)
	}

	httpSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.Contains(r.URL.Path, "danmu"):
			json.NewEncoder(w).Encode(map[string]any{
				"code": 0,
				"data": map[string]any{
					"token":     "tok-abc",
					"host_list": []map[string]any{{"host": "unused", "wss_port": 443}},
				},
			})
		case strings.Contains(r.URL.Path, "audience"):
			roomID := r.URL.Query().Get("roomid")
			var uids string
			if roomID == "21452505" {
				uids = `[{"uid":100},{"uid":"101"}]`
			} else {
				uids = `[{"uid":200}]`
			}
			w.Write([]byte(`{"code":0,"data":{"room":` + uids + `}}`))
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
	ac.SetBaseURL("roomAudience", httpSrv.URL+"/audience")
	ac.Signer().SetMixinKey("abcdefghijklmnopqrstuvwxyz012345")

	fs := newMultiRoomFakeServer(t, nil)
	c := NewClient("21452505", ac,
		WithDialURLOverride(fs.wsURL()),
		WithHeartbeatInterval(50*time.Millisecond),
		WithBackoff(10*time.Millisecond, 30*time.Millisecond),
	)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go c.Run(ctx)

	link := c.StartPK(ctx, []event.PkMember{
		{RoomID: "21452505", UID: "room-anchor-uid"}, // 自己：本房间主播 uid，不是登录账号
		{RoomID: "33333", UID: "u-opp-anchor"},
	})

	// 不能等 len(mine) > 0——seedAudiences 第一件事就是同步
	// seedMine(selfUID)，这个条件在 HTTP 请求发出去之前就已经成立，
	// 等到的只是「自己主播已经播种」，不是「近期弹幕发送者真的播种
	// 完了」。必须等真正来自 HTTP 的种子（"100"）落地，等一个必然
	// 立刻成立的条件等于没等（复审实测过 -count=100 有 3~10% 失败率）。
	waitUntil(t, time.Second, func() bool {
		mine, _ := link.Audiences()
		_, ok := mine["100"]
		return ok
	})

	mine, opposite := link.Audiences()
	for _, want := range []string{"100", "101", "room-anchor-uid"} { // 100/101 来自近期弹幕，room-anchor-uid 是本房间主播
		if _, ok := mine[want]; !ok {
			t.Errorf("myAudience 缺少 %q, 实际 %v", want, mine)
		}
	}
	if _, ok := mine["999999"]; ok {
		t.Error("myAudience 不应该包含登录账号 uid（999999）——播种的必须是 PK_INFO 里自己房间主播的 uid，不是登录账号")
	}

	oppRoom, ok := opposite["33333"]
	if !ok {
		t.Fatalf("opposite 应该有房间 33333 这个键，实际 %v", opposite)
	}
	for _, want := range []string{"200", "u-opp-anchor"} { // 200 来自对面近期弹幕，u-opp-anchor 是对面主播
		if _, ok := oppRoom[want]; !ok {
			t.Errorf("房间 33333 的 oppositeAudience 缺少 %q, 实际 %v", want, oppRoom)
		}
	}

	c.EndPK()
}

// TestPKAudienceSetsUpdateLiveFromBothRooms 验证「连上之后两个集合要
// 随事件持续更新」：本房间来一条弹幕就该进 myAudience，对面房间来一条
// 就该进 oppositeAudience。
func TestPKAudienceSetsUpdateLiveFromBothRooms(t *testing.T) {
	hostDanmaku := wire.Encode(wire.OpMessage, []byte(
		`{"cmd":"DANMU_MSG","info":[[0,1,25,16777215,1700000000000],"h",`+
			`[7001,"host-talker",0,0,0,10000,1,""],[],[10,0,0,0],["",""],0,0,null,null,0,0,[3]]}`))
	oppDanmaku := wire.Encode(wire.OpMessage, []byte(
		`{"cmd":"DANMU_MSG","info":[[0,1,25,16777215,1700000000000],"o",`+
			`[7002,"opp-talker",0,0,0,10000,1,""],[],[10,0,0,0],["",""],0,0,null,null,0,0,[3]]}`))

	fs := newMultiRoomFakeServer(t, func(c *websocket.Conn, roomID string) {
		if roomID == "21452505" {
			c.WriteMessage(websocket.BinaryMessage, hostDanmaku)
		} else {
			c.WriteMessage(websocket.BinaryMessage, oppDanmaku)
		}
	})
	c := newPKHostClient(t, fs, "21452505")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go c.Run(ctx)

	link := c.StartPK(ctx, []event.PkMember{
		{RoomID: "21452505", UID: "u-self"},
		{RoomID: "33333", UID: "u-opp-anchor"},
	})

	// 排空两个通道，让宿主自己的事件也真的流经 handleMessage 的钩子。
	go func() {
		for range c.Events() {
		}
	}()
	go func() {
		for range link.Events() {
		}
	}()

	waitUntil(t, 2*time.Second, func() bool {
		mine, opposite := link.Audiences()
		_, hasMine := mine["7001"]
		oppRoom := opposite["33333"]
		_, hasOpp := oppRoom["7002"]
		return hasMine && hasOpp
	})

	c.EndPK()
}

// ---------- P5-5 7c：对面高能榜滚动窗口，走真实管道 ----------

// onlineRankV3WireMessage 构造一条真实的 ONLINE_RANK_V3 wire 帧（protobuf
// 编码 + base64，跟 cmdmap/onlinerankv3_test.go 的构造方式一致，只是那边
// 的 pbVarint/pbString/pbMessage 是 cmdmap 包内 unexported 的测试辅助，
// 这里的字段号照抄 cmdmap/onlinerankv3.go 里的常量：field 1=uid（榜单项
// 内），field 3=榜单项（外层）。
func onlineRankV3WireMessage(uids ...string) []byte {
	var pb []byte
	for _, uid := range uids {
		n, err := strconv.ParseUint(uid, 10, 64)
		if err != nil {
			panic(err) // 测试构造数据，出错就是测试本身写错了
		}
		item := protowire.AppendTag(nil, 1, protowire.VarintType)
		item = protowire.AppendVarint(item, n)

		pb = protowire.AppendTag(pb, 3, protowire.BytesType)
		pb = protowire.AppendBytes(pb, item)
	}

	payload := map[string]any{
		"cmd":  "ONLINE_RANK_V3",
		"data": map[string]any{"pb": base64.StdEncoding.EncodeToString(pb)},
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		panic(err)
	}
	return wire.Encode(wire.OpMessage, raw)
}

// TestPKOppositeEnergyRankWindowUpdatesFromRealOnlineRankV3 是 P5-5 7c
// 新增第四套集合（oppositeEnergyRank）的管道级验证：不是直接摆内部状态
// （那部分已经在 visit_test.go 用 trackOppositeEnergyRank 覆盖过判定
// 逻辑本身），而是证明 runOpponent 真的把对手连接收到的真实
// ONLINE_RANK_V3 wire 帧接进了这套集合——从 wire 字节、经 child.Client
// 的真实握手/解包/cmdmap 归一化，到 runOpponent 循环里的
// trackOppositeEnergyRank 调用，一条不缺。
//
// 断言直接读 link.oppositeEnergyRank（同包白盒访问）而不是新增一个导出
// 的调试用 getter——Audiences() 是 P6 消费方要用的公开只读快照，这套
// 内部集合目前只服务于 visit.go 自己，没有必要为了这一条测试扩大公开
// 面。
func TestPKOppositeEnergyRankWindowUpdatesFromRealOnlineRankV3(t *testing.T) {
	rankMsg := onlineRankV3WireMessage("9001")

	fs := newMultiRoomFakeServer(t, func(c *websocket.Conn, roomID string) {
		if roomID != "21452505" { // 只推给对手连接，宿主连接不需要
			c.WriteMessage(websocket.BinaryMessage, rankMsg)
		}
	})
	c := newPKHostClient(t, fs, "21452505")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go c.Run(ctx)

	link := c.StartPK(ctx, []event.PkMember{
		{RoomID: "21452505", UID: "u-self"},
		{RoomID: "33333", UID: "u-opp-anchor"},
	})

	go func() {
		for range c.Events() {
		}
	}()
	go func() {
		for range link.Events() {
		}
	}()

	waitUntil(t, 2*time.Second, func() bool {
		link.audMu.Lock()
		defer link.audMu.Unlock()
		_, ok := link.oppositeEnergyRank["33333"]["9001"]
		return ok
	})

	c.EndPK()
}
