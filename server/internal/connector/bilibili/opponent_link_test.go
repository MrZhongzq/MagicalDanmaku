package bilibili

import (
	"context"
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

// goroutineBaseline 在测试主体开始前拍一张 goroutine 数快照；测试结束时
// 用 assertSettles 跟这张快照比对，证明没有悬挂 goroutine。用
// runtime.GC() + 轮询等待，避免 GC/调度抖动导致的偶发误报。
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
			// PkLink.Connect 会同步播种观众集合，必须本地兜底，否则会
			// 真的打到外网的 ajax/msg，把测试拖慢到预算超时（5s）。
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

	// 宿主自己 1 条 + 2 个对手各 1 条 = 3 条连接。
	waitUntil(t, time.Second, func() bool { return fs.authCount() >= 3 })

	rooms := fs.authedRooms()
	seen := map[string]bool{}
	for _, r := range rooms {
		seen[r] = true
	}
	for _, want := range []string{"21452505", "33333", "44444"} {
		if !seen[want] {
			t.Errorf("未观测到房间 %s 的连接，已连接: %v", want, rooms)
		}
	}

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

	// 宿主 1 条 + 三个对手各 1 条 = 4 条连接。
	waitUntil(t, time.Second, func() bool { return fs.authCount() >= 4 })

	rooms := map[string]bool{}
	for _, r := range fs.authedRooms() {
		rooms[r] = true
	}
	for _, want := range []string{"11111", "22222", "33333"} {
		if !rooms[want] {
			t.Errorf("对手房间 %s 应各自建一条连接", want)
		}
	}

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
	waitUntil(t, time.Second, func() bool { return fs.authCount() >= 3 })

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
	waitUntil(t, time.Second, func() bool { return fs.authCount() >= 3 })

	// 异常结束：直接砍 PK 自己的 ctx，不调用 EndPK。
	pkCancel()

	// Disconnect 语义上应该能等到清理完成；这里通过 EndPK 再调用一次
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
	waitUntil(t, time.Second, func() bool { return fs.authCount() >= 3 })

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
		waitUntil(t, time.Second, func() bool { return true }) // 让连接有机会建立
		time.Sleep(30 * time.Millisecond)
		c.EndPK()
	}

	cancel()
	for range c.Events() {
	}

	assertSettles(t, base)
}

// ---------- 观众集合 ----------

// TestPKSeedsAudienceSetsFromRecentDanmakuAndSelves 验证播种逻辑：
// 双方房间各自的近期弹幕发送者 + 自己主播 + 对面主播都应该进各自的集合。
func TestPKSeedsAudienceSetsFromRecentDanmakuAndSelves(t *testing.T) {
	sess, err := auth.ParseSession("SESSDATA=x; bili_jct=tok; DedeUserID=1; buvid3=BV3")
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
		{RoomID: "21452505", UID: "u-self"},
		{RoomID: "33333", UID: "u-opp-anchor"},
	})

	waitUntil(t, time.Second, func() bool {
		mine, _ := link.Audiences()
		return len(mine) > 0
	})

	mine, opposite := link.Audiences()
	for _, want := range []string{"100", "101", "1"} { // 100/101 来自近期弹幕，1 是自己主播 uid
		if _, ok := mine[want]; !ok {
			t.Errorf("myAudience 缺少 %q, 实际 %v", want, mine)
		}
	}
	for _, want := range []string{"200", "u-opp-anchor"} { // 200 来自对面近期弹幕，u-opp-anchor 是对面主播
		if _, ok := opposite[want]; !ok {
			t.Errorf("oppositeAudience 缺少 %q, 实际 %v", want, opposite)
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
		_, hasOpp := opposite["7002"]
		return hasMine && hasOpp
	})

	c.EndPK()
}
