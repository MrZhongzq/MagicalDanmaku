package bilibili

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"

	"github.com/MrZhongzq/MagicalDanmaku/server/internal/connector/bilibili/api"
	"github.com/MrZhongzq/MagicalDanmaku/server/internal/connector/bilibili/auth"
	"github.com/MrZhongzq/MagicalDanmaku/server/internal/connector/bilibili/wire"
	"github.com/MrZhongzq/MagicalDanmaku/server/internal/event"
)

// ---------- 测试辅助：组装真实报文 ----------

// pkInfoRaw 构造一条 PK_INFO 的原始 JSON，字段形状对齐
// testdata/cmds/PK_INFO_basic.json（房间号/pk_id 可参数化，供不同测试
// 复用）。
func pkInfoRaw(hostRoom, oppRoom, pkID string) []byte {
	return []byte(fmt.Sprintf(`{
		"cmd":"PK_INFO",
		"data":{
			"members":[
				{"room_id":%s,"uid":11111111,"uname":"host-anchor","face":"h.jpg","votes":10,"is_winner":0},
				{"room_id":%s,"uid":22222222,"uname":"opp-anchor","face":"o.jpg","votes":20,"is_winner":1}
			],
			"pk_basic":{"pk_id":"%s","start_time":1700000000,"end_time":1700000300}
		}
	}`, hostRoom, oppRoom, pkID))
}

func pkInfoMsg(hostRoom, oppRoom, pkID string) []byte {
	return wire.Encode(wire.OpMessage, pkInfoRaw(hostRoom, oppRoom, pkID))
}

// pkBattleEndMsg 构造一条 PK_BATTLE_END 消息——battleCommands 里登记的
// 真实结束 CMD，PKPipeline 据此触发 EndPK。
func pkBattleEndMsg() []byte {
	return wire.Encode(wire.OpMessage, []byte(`{"cmd":"PK_BATTLE_END"}`))
}

func danmakuMsg(uid int, name, text string) []byte {
	body := fmt.Sprintf(`{"cmd":"DANMU_MSG","info":[[0,1,25,16777215,1700000000000],%q,`+
		`[%d,%q,0,0,0,10000,1,""],[],[10,0,0,0],["",""],0,0,null,null,0,0,[3]]}`, text, uid, name)
	return wire.Encode(wire.OpMessage, []byte(body))
}

// ---------- 测试辅助：能同时应答连接与快照六类接口的假客户端 ----------

// pkPipelineTestServer 把 PK_INFO 用得到的三类 HTTP 接口（danmu/room/
// audience）与 FetchOpponentSnapshots 用得到的三类接口（roomOnline/
// guardTotal/guardOnline）都装到一个 httptest.Server 上，按路径区分，
// 避免像 opponent_snapshot_test.go 那样纯靠查询参数区分时跟 room_id 之类
// 的公共参数打架。
type pkPipelineTestServer struct {
	srv *httptest.Server

	roomOnline  map[string]int64
	guardTotal  map[string]int64
	guardOnline map[string]int64

	// delay 让指定端点的响应人为变慢，用于验证「不阻塞事件循环」。
	delay map[string]time.Duration
}

func newPKPipelineTestServer(t *testing.T) *pkPipelineTestServer {
	t.Helper()
	ts := &pkPipelineTestServer{
		roomOnline:  map[string]int64{},
		guardTotal:  map[string]int64{},
		guardOnline: map[string]int64{},
		delay:       map[string]time.Duration{},
	}
	ts.srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.Contains(r.URL.Path, "/danmu"):
			json.NewEncoder(w).Encode(map[string]any{
				"code": 0,
				"data": map[string]any{
					"token": "tok-abc",
					"host_list": []map[string]any{
						{"host": "unused", "wss_port": 443, "port": 2243, "ws_port": 2244},
					},
				},
			})
		case strings.Contains(r.URL.Path, "/audience"):
			// seedAudiences 必须本地兜底，否则会真的打到外网。
			w.Write([]byte(`{"code":0,"data":{"room":[]}}`))
		case strings.Contains(r.URL.Path, "/room-online"):
			if d := ts.delay["roomOnline"]; d > 0 {
				time.Sleep(d)
			}
			online := ts.roomOnline[r.URL.Query().Get("room_id")]
			w.Write([]byte(`{"code":0,"data":{"room_info":{"online":` + strconv.FormatInt(online, 10) + `}}}`))
		case strings.Contains(r.URL.Path, "/guard-total"):
			if d := ts.delay["guardTotal"]; d > 0 {
				time.Sleep(d)
			}
			total := ts.guardTotal[r.URL.Query().Get("roomid")]
			w.Write([]byte(`{"code":0,"data":{"info":{"num":` + strconv.FormatInt(total, 10) + `}}}`))
		case strings.Contains(r.URL.Path, "/guard-online"):
			if d := ts.delay["guardOnline"]; d > 0 {
				time.Sleep(d)
			}
			n := ts.guardOnline[r.URL.Query().Get("room_id")]
			items := "["
			for i := int64(0); i < n; i++ {
				if i > 0 {
					items += ","
				}
				items += fmt.Sprintf(`{"uid":%d,"guard_level":3}`, i+1)
			}
			items += "]"
			w.Write([]byte(`{"code":0,"data":{"item":` + items + `,"count":` + strconv.FormatInt(n, 10) + `}}`))
		default: // roomInfo
			json.NewEncoder(w).Encode(map[string]any{
				"code": 0,
				"data": map[string]any{"room_id": 21452505, "uid": 1, "live_status": 1},
			})
		}
	}))
	t.Cleanup(ts.srv.Close)
	return ts
}

// newPKPipelineTestClient 构造一台连着 fs（WS）与 ts（HTTP）的宿主
// Client，选项跟 newPKHostClient 一致地把心跳/退避调快，避免测试被拖慢。
func newPKPipelineTestClient(t *testing.T, fs *multiRoomFakeServer, ts *pkPipelineTestServer, roomID string) *Client {
	t.Helper()

	sess, err := auth.ParseSession("SESSDATA=x; bili_jct=tok; DedeUserID=42; buvid3=BV3")
	if err != nil {
		t.Fatalf("ParseSession 失败: %v", err)
	}

	ac := api.New(sess, api.WithHTTPClient(ts.srv.Client()))
	ac.SetBaseURL("roomInfo", ts.srv.URL+"/room")
	ac.SetBaseURL("danmuInfo", ts.srv.URL+"/danmu")
	ac.SetBaseURL("roomAudience", ts.srv.URL+"/audience")
	ac.SetBaseURL("roomOnline", ts.srv.URL+"/room-online")
	ac.SetBaseURL("guardTotal", ts.srv.URL+"/guard-total")
	ac.SetBaseURL("guardOnline", ts.srv.URL+"/guard-online")
	ac.Signer().SetMixinKey("abcdefghijklmnopqrstuvwxyz012345")

	return NewClient(roomID, ac,
		WithDialURLOverride(fs.wsURL()),
		WithHeartbeatInterval(50*time.Millisecond),
		WithBackoff(10*time.Millisecond, 30*time.Millisecond),
	)
}

// drainUntil 从 out 里收集事件直到 match 返回 true 或超时，返回命中的
// 那条事件。
func drainUntil(t *testing.T, out <-chan event.Event, timeout time.Duration, match func(event.Event) bool) event.Event {
	t.Helper()
	deadline := time.After(timeout)
	for {
		select {
		case ev, ok := <-out:
			if !ok {
				t.Fatal("通道已关闭，仍未等到期望的事件")
			}
			if match(ev) {
				return ev
			}
		case <-deadline:
			t.Fatal("等待期望事件超时")
		}
	}
}

// ---------- 核心安全性：不阻塞事件循环 ----------

// TestPKPipelineDoesNotBlockMainEventsDuringSnapshotFetch 是简报里点名
// 的最高风险场景：FetchOpponentSnapshots 同步执行 HTTP 请求，若被安排在
// 消费 Client.Events() 的同一个 goroutine 上，PK 接通瞬间的主房间事件会
// 被晾住，超出 256 缓冲就被丢弃。这里故意把 roomOnline 接口拖慢到 1.5s，
// 断言 PK_INFO 之后紧跟着的一串弹幕仍然能在远小于 1.5s 的时间内经由
// PKPipeline 合流通道收到——如果编排逻辑不小心同步调用了
// FetchOpponentSnapshots，这批弹幕会被那 1.5s 的慢请求卡住，断言超时失败。
func TestPKPipelineDoesNotBlockMainEventsDuringSnapshotFetch(t *testing.T) {
	const hostRoom = "21452505"
	const oppRoom = "33333"
	const burstCount = 20

	ts := newPKPipelineTestServer(t)
	ts.delay["roomOnline"] = 1500 * time.Millisecond

	fs := newMultiRoomFakeServer(t, func(c *websocket.Conn, roomID string) {
		if roomID != hostRoom {
			return
		}
		c.WriteMessage(websocket.BinaryMessage, pkInfoMsg(hostRoom, oppRoom, "pk-slow-1"))
		for i := 0; i < burstCount; i++ {
			c.WriteMessage(websocket.BinaryMessage, danmakuMsg(9000+i, fmt.Sprintf("u%d", i), "hi"))
		}
	})

	c := newPKPipelineTestClient(t, fs, ts, hostRoom)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go c.Run(ctx)

	out := NewPKPipeline(c).Run(ctx)

	danmakuSeen := 0
	deadline := time.After(700 * time.Millisecond) // 远小于 1.5s 的慢请求
	for danmakuSeen < burstCount {
		select {
		case ev := <-out:
			if ev.Type == event.TypeDanmaku {
				danmakuSeen++
			}
		case <-deadline:
			t.Fatalf("700ms 内只收到 %d/%d 条弹幕——怀疑 PK 编排的网络调用同步阻塞了事件循环",
				danmakuSeen, burstCount)
		}
	}
}

// ---------- 按 pk_id 去重 ----------

// TestPKPipelineDedupesSamePkID 验证同一场 PK（同一个 pk_id）重复收到
// PK_INFO 不应该重复触发 StartPK/FetchOpponentSnapshots——只应该看到
// 一条合成的快照事件，而不是两条。
func TestPKPipelineDedupesSamePkID(t *testing.T) {
	const hostRoom = "21452505"
	const oppRoom = "33333"

	ts := newPKPipelineTestServer(t)
	ts.roomOnline[oppRoom] = 42

	fs := newMultiRoomFakeServer(t, func(c *websocket.Conn, roomID string) {
		if roomID != hostRoom {
			return
		}
		// 同一个 pk_id 连发两次——真实场景里 PK_INFO 确实可能因为
		// 客户端重连/服务端补发而重复到达。
		c.WriteMessage(websocket.BinaryMessage, pkInfoMsg(hostRoom, oppRoom, "pk-dup"))
		c.WriteMessage(websocket.BinaryMessage, pkInfoMsg(hostRoom, oppRoom, "pk-dup"))
	})

	c := newPKPipelineTestClient(t, fs, ts, hostRoom)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go c.Run(ctx)

	out := NewPKPipeline(c).Run(ctx)

	isSnapshot := func(ev event.Event) bool {
		b, ok := ev.Payload.(event.Battle)
		return ok && b.SubCommand == PKOpponentSnapshotSubCommand
	}
	first := drainUntil(t, out, 2*time.Second, isSnapshot)
	if b := first.Payload.(event.Battle); b.PkID != "pk-dup" {
		t.Fatalf("PkID = %q, 期望 pk-dup", b.PkID)
	}

	// 再等一小段时间，确认不会有第二条快照事件跟上来。
	select {
	case ev := <-out:
		if isSnapshot(ev) {
			t.Fatal("同一个 pk_id 重复到达的 PK_INFO 不应该触发第二次 StartPK/快照")
		}
	case <-time.After(300 * time.Millisecond):
	}
}

// ---------- 快照数据正确合并进合成事件 ----------

// TestPKPipelineEmitsSnapshotEventWithOpponentData 验证 PK 接通后合流
// 通道会收到一条 SubCommand=PKOpponentSnapshotSubCommand 的 Battle
// 事件，对手成员的 Online/GuardTotal/GuardOnline 被正确填上，自己那一项
// 保持 nil（从未为自己抓过这份数据）。
func TestPKPipelineEmitsSnapshotEventWithOpponentData(t *testing.T) {
	const hostRoom = "21452505"
	const oppRoom = "33333"

	ts := newPKPipelineTestServer(t)
	ts.roomOnline[oppRoom] = 12345
	ts.guardTotal[oppRoom] = 67
	ts.guardOnline[oppRoom] = 3

	fs := newMultiRoomFakeServer(t, func(c *websocket.Conn, roomID string) {
		if roomID == hostRoom {
			c.WriteMessage(websocket.BinaryMessage, pkInfoMsg(hostRoom, oppRoom, "pk-snap"))
		}
	})

	c := newPKPipelineTestClient(t, fs, ts, hostRoom)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go c.Run(ctx)

	out := NewPKPipeline(c).Run(ctx)

	ev := drainUntil(t, out, 2*time.Second, func(ev event.Event) bool {
		b, ok := ev.Payload.(event.Battle)
		return ok && b.SubCommand == PKOpponentSnapshotSubCommand
	})

	b := ev.Payload.(event.Battle)
	if b.PkID != "pk-snap" {
		t.Errorf("PkID = %q, 期望 pk-snap", b.PkID)
	}
	if len(b.Members) != 2 {
		t.Fatalf("Members 数量 = %d, 期望 2", len(b.Members))
	}

	var self, opp *event.PkMember
	for i := range b.Members {
		m := &b.Members[i]
		switch m.RoomID {
		case hostRoom:
			self = m
		case oppRoom:
			opp = m
		}
	}
	if self == nil || opp == nil {
		t.Fatalf("未能按房间号找全双方成员: %+v", b.Members)
	}
	if self.Online != nil || self.GuardTotal != nil || self.GuardOnline != nil {
		t.Errorf("自己这一项应保持 nil（从未为自己抓过快照），实际 online=%v guardTotal=%v guardOnline=%v",
			self.Online, self.GuardTotal, self.GuardOnline)
	}
	if opp.Online == nil || *opp.Online != 12345 {
		t.Errorf("对面 Online = %v, 期望 12345", opp.Online)
	}
	if opp.GuardTotal == nil || *opp.GuardTotal != 67 {
		t.Errorf("对面 GuardTotal = %v, 期望 67", opp.GuardTotal)
	}
	if opp.GuardOnline == nil || *opp.GuardOnline != 3 {
		t.Errorf("对面 GuardOnline = %v, 期望 3", opp.GuardOnline)
	}
}

// ---------- 降级：拿不到 ≠ 真的是 0 ----------

// TestPKPipelineDegradesSnapshotFieldOnHTTPFailure 是自检要求的变异 (b)：
// 让 roomOnline 接口失败，断言 Online 字段降级为 nil（而不是被塞成 0），
// 且 PK 播报（合成事件）依然照常发出、其余字段不受影响——外部 HTTP
// 拿不到数据不能让整条播报都不发。
func TestPKPipelineDegradesSnapshotFieldOnHTTPFailure(t *testing.T) {
	const hostRoom = "21452505"
	const oppRoom = "33333"

	ts := newPKPipelineTestServer(t)
	ts.guardOnline[oppRoom] = 2 // guardOnline 正常返回，证明只有失败的那个字段被降级

	// newPKPipelineTestServer 本身没有开「让某个端点失败」的口子，这里
	// 单独起一个只服务 roomOnline、且恒定返回业务错误码的服务器，
	// api.Client 的 roomOnline 基址单独指过去，其余基址仍指向 ts。
	fail := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "/room-online") {
			w.Write([]byte(`{"code":-352,"message":"风控"}`))
			return
		}
		http.NotFound(w, r)
	}))
	t.Cleanup(fail.Close)

	fs := newMultiRoomFakeServer(t, func(c *websocket.Conn, roomID string) {
		if roomID == hostRoom {
			c.WriteMessage(websocket.BinaryMessage, pkInfoMsg(hostRoom, oppRoom, "pk-degrade"))
		}
	})

	sess, err := auth.ParseSession("SESSDATA=x; bili_jct=tok; DedeUserID=42; buvid3=BV3")
	if err != nil {
		t.Fatalf("ParseSession 失败: %v", err)
	}
	ac := api.New(sess, api.WithHTTPClient(ts.srv.Client()))
	ac.SetBaseURL("roomInfo", ts.srv.URL+"/room")
	ac.SetBaseURL("danmuInfo", ts.srv.URL+"/danmu")
	ac.SetBaseURL("roomAudience", ts.srv.URL+"/audience")
	ac.SetBaseURL("roomOnline", fail.URL+"/room-online") // 唯独这个指向会失败的服务器
	ac.SetBaseURL("guardTotal", ts.srv.URL+"/guard-total")
	ac.SetBaseURL("guardOnline", ts.srv.URL+"/guard-online")
	ac.Signer().SetMixinKey("abcdefghijklmnopqrstuvwxyz012345")

	c := NewClient(hostRoom, ac,
		WithDialURLOverride(fs.wsURL()),
		WithHeartbeatInterval(50*time.Millisecond),
		WithBackoff(10*time.Millisecond, 30*time.Millisecond),
	)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go c.Run(ctx)

	out := NewPKPipeline(c).Run(ctx)

	ev := drainUntil(t, out, 2*time.Second, func(ev event.Event) bool {
		b, ok := ev.Payload.(event.Battle)
		return ok && b.SubCommand == PKOpponentSnapshotSubCommand
	})
	b := ev.Payload.(event.Battle)

	var opp *event.PkMember
	for i := range b.Members {
		if b.Members[i].RoomID == oppRoom {
			opp = &b.Members[i]
		}
	}
	if opp == nil {
		t.Fatal("未找到对面成员")
	}
	if opp.Online != nil {
		t.Errorf("Online 应降级为 nil（roomOnline 接口失败），实际 %v——"+
			"「拿不到」不能被显示成「真的是 0」", *opp.Online)
	}
	if opp.GuardOnline == nil || *opp.GuardOnline != 2 {
		t.Errorf("GuardOnline 不该受 roomOnline 失败影响，期望 2，实际 %v", opp.GuardOnline)
	}
}

// danmakuMsgWithMedal 跟 danmakuMsg 一样，但带一个指向 medalRoomID 的
// 粉丝勋章——用来触发 ClassifyVisit 的判据 1（粉丝牌），不依赖
// mineSeed/oppositeSeed 播种是否已经完成，避免测试跟「PK 编排是异步的」
// 这一事实产生时序竞争。
func danmakuMsgWithMedal(uid int, name, text, medalRoomID string) []byte {
	body := fmt.Sprintf(`{"cmd":"DANMU_MSG","info":[[0,1,25,16777215,1700000000000],%q,`+
		`[%d,%q,0,0,0,10000,1,""],[10,"M","A",%q,0,0,0,0,0,0,0,0,"0"],[10,0,0,0],["",""],0,0,null,null,0,0,[3]]}`,
		text, uid, name, medalRoomID)
	return wire.Encode(wire.OpMessage, []byte(body))
}

// ---------- 两个串门方向都要能通过合流通道观察到 ----------

// TestPKPipelineClassifiesBothVisitDirections 是自检要求的变异 (a)：
// 分别构造「对面主播来我方发言」（方向 A）与「我方观众去对面发言」
// （方向 B），断言合流通道里出现的 Type 分别是 TypeVisitFromOpponent/
// TypeVisitToOpponent 且互不相同——如果实现把两个方向的事件类型写反，
// 这里的断言会直接报错。
//
// 两个触发都用粉丝牌判据（不依赖 mineSeed/oppositeSeed 播种是否完成），
// 且各自重发若干次直到 PK 真正接通（StartPK 是异步触发的，round 建立
// 有一个不确定的短暂窗口）——重发在 round 未就绪时只是让 ClassifyVisit
// 返回 false，弹幕本身仍会正常投递，没有副作用。两个方向的事件用同一个
// 循环收集，不分两次顺序等待——顺序等待会在其中一个方向的事件先到达时
// 被当成「不匹配」丢弃，而对面只重发了有限次数，可能等不到第二次。
func TestPKPipelineClassifiesBothVisitDirections(t *testing.T) {
	const hostRoom = "21452505"
	const oppRoom = "33333"

	ts := newPKPipelineTestServer(t)

	fromOppTrigger := danmakuMsgWithMedal(22222222, "opp-anchor", "串门A", oppRoom)
	toOppTrigger := danmakuMsgWithMedal(90001, "host-fan", "串门B", hostRoom)

	fs := newMultiRoomFakeServer(t, func(c *websocket.Conn, roomID string) {
		switch roomID {
		case hostRoom:
			c.WriteMessage(websocket.BinaryMessage, pkInfoMsg(hostRoom, oppRoom, "pk-visit"))
			for i := 0; i < 15; i++ {
				c.WriteMessage(websocket.BinaryMessage, fromOppTrigger)
				time.Sleep(100 * time.Millisecond)
			}
		case oppRoom:
			for i := 0; i < 15; i++ {
				c.WriteMessage(websocket.BinaryMessage, toOppTrigger)
				time.Sleep(100 * time.Millisecond)
			}
		}
	})

	c := newPKPipelineTestClient(t, fs, ts, hostRoom)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go c.Run(ctx)

	out := NewPKPipeline(c).Run(ctx)

	var fromOpp, toOpp *event.Event
	deadline := time.After(4 * time.Second)
	for fromOpp == nil || toOpp == nil {
		select {
		case ev := <-out:
			switch ev.Type {
			case event.TypeVisitFromOpponent:
				if fromOpp == nil {
					e := ev
					fromOpp = &e
				}
			case event.TypeVisitToOpponent:
				if toOpp == nil {
					e := ev
					toOpp = &e
				}
			}
		case <-deadline:
			t.Fatalf("超时：fromOpp 命中=%v, toOpp 命中=%v", fromOpp != nil, toOpp != nil)
		}
	}

	if fromOpp.Type == toOpp.Type {
		t.Fatalf("两个方向的 Type 不该相同，实际都是 %v", fromOpp.Type)
	}
	if _, ok := fromOpp.Payload.(event.VisitFromOpponent); !ok {
		t.Errorf("方向 A 的 Payload 类型 = %T, 期望 event.VisitFromOpponent", fromOpp.Payload)
	}
	if _, ok := toOpp.Payload.(event.VisitToOpponent); !ok {
		t.Errorf("方向 B 的 Payload 类型 = %T, 期望 event.VisitToOpponent", toOpp.Payload)
	}
}

// ---------- PK_BATTLE_END 触发 EndPK ----------

// TestPKPipelineEndsOnBattleEndPushed 用支持「连接建立后按需推送」的假
// 服务器验证 PK_BATTLE_END 触发 EndPK：先接通 PK，再推送 PK_BATTLE_END，
// 断言 c.PKLink() 最终变回 nil。
func TestPKPipelineEndsOnBattleEndPushed(t *testing.T) {
	const hostRoom = "21452505"
	const oppRoom = "33333"

	ts := newPKPipelineTestServer(t)

	hostConnReady := make(chan *websocket.Conn, 1)
	fs := newMultiRoomFakeServer(t, func(c *websocket.Conn, roomID string) {
		if roomID != hostRoom {
			return
		}
		c.WriteMessage(websocket.BinaryMessage, pkInfoMsg(hostRoom, oppRoom, "pk-end-2"))
		select {
		case hostConnReady <- c:
		default:
		}
	})

	c := newPKPipelineTestClient(t, fs, ts, hostRoom)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go c.Run(ctx)

	out := NewPKPipeline(c).Run(ctx)

	drainUntil(t, out, 2*time.Second, func(ev event.Event) bool {
		b, ok := ev.Payload.(event.Battle)
		return ok && b.SubCommand == PKOpponentSnapshotSubCommand
	})
	if c.PKLink() == nil {
		t.Fatal("PK 接通后 c.PKLink() 不应为 nil")
	}

	var hostConn *websocket.Conn
	select {
	case hostConn = <-hostConnReady:
	case <-time.After(time.Second):
		t.Fatal("未能取得宿主连接")
	}
	hostConn.WriteMessage(websocket.BinaryMessage, pkBattleEndMsg())

	waitUntil(t, 2*time.Second, func() bool { return c.PKLink() == nil })
}
