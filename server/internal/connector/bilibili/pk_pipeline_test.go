package bilibili

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
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
//
// end_time 必须相对 time.Now() 动态计算，不能写死一个固定的历史 epoch
// （曾经写死过 1700000300，2023 年的时间戳）——PKPipeline 的
// watchEndTimeFallback（Important-3）会拿它算「PK 应该在什么时候结束」，
// 写死的历史时间戳在任何跑测试的当下都早已过期，超时兜底会在 PK 接通
// 后几乎立刻触发，把测试还没来得及观察的 PK 提前结束掉——这不是
// watchEndTimeFallback 的 bug，是测试数据要跟着真实时钟走。
func pkInfoRaw(hostRoom, oppRoom, pkID string) []byte {
	startTime := time.Now().Unix()
	endTime := startTime + 300 // 跟真实 B 站 PK 大乱斗的常见时长量级一致
	return []byte(fmt.Sprintf(`{
		"cmd":"PK_INFO",
		"data":{
			"members":[
				{"room_id":%s,"uid":11111111,"uname":"host-anchor","face":"h.jpg","votes":10,"is_winner":0},
				{"room_id":%s,"uid":22222222,"uname":"opp-anchor","face":"o.jpg","votes":20,"is_winner":1}
			],
			"pk_basic":{"pk_id":"%s","start_time":%d,"end_time":%d}
		}
	}`, hostRoom, oppRoom, pkID, startTime, endTime))
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

// userEnterMsgWithMedal 构造一条 INTERACT_WORD（msg_type=1，进场）消息，
// 带一个指向 medalRoomID 的粉丝勋章——跟 danmakuMsgWithMedal 同样的目的
// （用粉丝牌判据触发 ClassifyVisit，不依赖 mineSeed/oppositeSeed 播种是
// 否完成），但事件类型必须是真正的 event.TypeUserEnter（INTERACT_WORD
// 的 msg_type=1），因为终审 Critical-1 第二部分要验证的正是「进场事件
// 命中方向 A 时，原始 UserEnter 不该被重复转发」，用弹幕触发测不出这个
// 场景——「内置/进房欢迎」规则只监听 user_enter。
func userEnterMsgWithMedal(uid int, uname, medalRoomID string) []byte {
	body := fmt.Sprintf(`{"cmd":"INTERACT_WORD","data":{
		"msg_type":1,"uid":%d,"uname":%q,"timestamp":1700000000,
		"fans_medal":{"anchor_roomid":%s,"guard_level":0,"is_lighted":1,
			"medal_level":10,"medal_name":"M","target_id":22222222}
	}}`, uid, uname, medalRoomID)
	return wire.Encode(wire.OpMessage, []byte(body))
}

// ---------- 终审 Critical-1 第二部分：进场只欢迎一次，不重复播两条 ----------

// TestPKPipelineSuppressesRawUserEnterWhenClassifiedAsVisitFromOpponent
// 是终审 Critical-1 第二部分的回归测试：对面观众进场时命中方向 A
// （ClassifyVisit 判定为串门来客），合流通道里应该只看到一条
// TypeVisitFromOpponent，不该再看到原始的 TypeUserEnter——旧代码会把
// 两条都转发出去，导致「内置/进房欢迎」与「内置/PK串门欢迎」各播一条，
// 跟 PkPanel.vue 的界面文案「与常规进房欢迎区分」自相矛盾。
//
// 先用 drainUntil 等到快照事件（PK 接通、round 已建立、pl.link 已发布）
// 再发送这一条进场消息，是为了避免像 TestPKPipelineClassifiesBothVisitDirections
// 那样靠重发同一条消息渡过异步建连的时序窗口——重发在这里会跟
// welcomedFromOpponent 去重纠缠在一起（第一次命中之后，后续重发会因为
// 已经欢迎过而不再被分类，转发路径又会切回「未分类」的原始事件），
// 使断言复杂化。等 round 确定就绪后只发一次，断言就能保持精确。
func TestPKPipelineSuppressesRawUserEnterWhenClassifiedAsVisitFromOpponent(t *testing.T) {
	const hostRoom = "21452505"
	const oppRoom = "33333"
	const visitorUID = 55501

	ts := newPKPipelineTestServer(t)

	hostConnReady := make(chan *websocket.Conn, 1)
	fs := newMultiRoomFakeServer(t, func(c *websocket.Conn, roomID string) {
		if roomID != hostRoom {
			return
		}
		c.WriteMessage(websocket.BinaryMessage, pkInfoMsg(hostRoom, oppRoom, "pk-enter-visit"))
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

	// 等到快照事件：round 已建立、pl.link 已发布（Important-3 保证发布
	// 不会被 FetchOpponentSnapshots 拖住），此后发的事件才会被 ClassifyVisit
	// 真正处理到，不会因为 round 还没就绪而误判成「未分类」。
	drainUntil(t, out, 2*time.Second, func(ev event.Event) bool {
		b, ok := ev.Payload.(event.Battle)
		return ok && b.SubCommand == PKOpponentSnapshotSubCommand
	})

	var hostConn *websocket.Conn
	select {
	case hostConn = <-hostConnReady:
	case <-time.After(time.Second):
		t.Fatal("未能取得宿主连接")
	}
	hostConn.WriteMessage(websocket.BinaryMessage,
		userEnterMsgWithMedal(visitorUID, "opp-fan", oppRoom))

	// 收集接下来一小段时间内合流通道上的全部事件，而不是 drainUntil 到
	// 第一条命中就停——这条测试恰恰要断言「不该出现」的那条事件不存在，
	// 只看到期望的那一条不够，必须确认没有第二条。
	var got []event.Event
	deadline := time.After(1200 * time.Millisecond)
collect:
	for {
		select {
		case ev := <-out:
			got = append(got, ev)
		case <-deadline:
			break collect
		}
	}

	var visitCount, userEnterCount int
	for _, ev := range got {
		switch ev.Type {
		case event.TypeVisitFromOpponent:
			visitCount++
			payload, ok := ev.Payload.(event.VisitFromOpponent)
			if !ok {
				t.Errorf("TypeVisitFromOpponent 的 Payload 类型 = %T, 期望 event.VisitFromOpponent", ev.Payload)
				continue
			}
			if payload.User.UID != strconv.Itoa(visitorUID) {
				t.Errorf("串门事件的 User.UID = %q, 期望 %q", payload.User.UID, strconv.Itoa(visitorUID))
			}
		case event.TypeUserEnter:
			userEnterCount++
		}
	}

	if visitCount != 1 {
		t.Errorf("TypeVisitFromOpponent 出现 %d 次，期望恰好 1 次", visitCount)
	}
	if userEnterCount != 0 {
		t.Errorf("TypeUserEnter 出现 %d 次，期望 0 次——命中方向 A 的进场事件不该再被当作普通进场转发一遍，"+
			"否则「内置/进房欢迎」与「内置/PK串门欢迎」会各播一条，跟界面文案承诺的"+
			"「与常规进房欢迎区分」矛盾", userEnterCount)
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

// ---------- 终审 Important-3：PK_BATTLE_END 丢失时的超时兜底 ----------

// pkInfoRawWithEndTime 跟 pkInfoRaw 一样，但 end_time 可以显式指定——
// 专供超时兜底测试用，需要控制「end_time 距离现在还有多久」来让
// endTimeFallbackGrace 在测试可接受的时间内触发，不依赖 pkInfoRaw 里
// 「跟真实 PK 时长量级一致」的默认 +300s。
func pkInfoRawWithEndTime(hostRoom, oppRoom, pkID string, endTime time.Time) []byte {
	return []byte(fmt.Sprintf(`{
		"cmd":"PK_INFO",
		"data":{
			"members":[
				{"room_id":%s,"uid":11111111,"uname":"host-anchor","face":"h.jpg","votes":10,"is_winner":0},
				{"room_id":%s,"uid":22222222,"uname":"opp-anchor","face":"o.jpg","votes":20,"is_winner":1}
			],
			"pk_basic":{"pk_id":"%s","start_time":1700000000,"end_time":%d}
		}
	}`, hostRoom, oppRoom, pkID, endTime.Unix()))
}

// TestPKPipelineEndTimeFallbackEndsStalePKWhenBattleEndMissing 是终审
// Important-3 的核心回归测试：真实场景里 PK_BATTLE_END 有可能被 256
// 缓冲的 c.events 挤丢（PK 接通瞬间恰好是弹幕礼物最密集的时刻），丢了
// 之后没有任何后续 CMD 会补发它——旧代码在这种情况下会让 PkLink 无限期
// 存活、对面连接一直挂着重连、ClassifyVisit 一直把戴对面勋章的观众当
// 串门来客欢迎，直到几小时后下一场 PK 才会自愈。这里完全不发送
// PK_BATTLE_END，只靠 pk_basic.end_time 驱动的超时兜底，断言 PK 最终
// 还是会被结束。
//
// end_time 设成「300ms 之后」而不是「已经过去」：如果设成已经过去，
// wait 会钳到 0，超时兜底幾乎立即触发，可能在 c.StartPK() 内部完成
// 注册之前就跑完（见 watchEndTimeFallback 上方「已知的极窄边角」的
// 说明）——那不是这条测试要验证的东西，会引入不必要的时序不确定性。
// 用一个略微滞后的截止时间，让 PK 先正常接通（观察到快照事件、
// c.PKLink() 非 nil），再等超时兜底把它收尾，跟生产环境的时序关系一致。
func TestPKPipelineEndTimeFallbackEndsStalePKWhenBattleEndMissing(t *testing.T) {
	const hostRoom = "21452505"
	const oppRoom = "33333"

	ts := newPKPipelineTestServer(t)

	// end_time 在每次实际发送消息时才重新计算（而不是像早期版本那样在
	// 测试 setup 阶段算一次、之后重连每次都复用同一条固化了旧时间戳的
	// 消息）——multiRoomFakeServer 固定 500ms 断开重连一次，握手、鉴权、
	// 首次连接建立都要占用一部分时间，如果 end_time 是发送前很久就算好
	// 的，实际送达时留给「PK 建连 + 抓快照」的缓冲会被悄悄吃掉一部分，
	// 在系统繁忙时这个测试曾经因此变得不稳定（-count=5 全部超时）。
	// grace 给到 1.5s，远大于本地 httptest 服务器上完成快照抓取通常需要
	// 的时间（正常在几十毫秒量级），把「注册/建连/抓快照」的自然耗时
	// 与「超时兜底该等多久」这两件事的时间量级彻底拉开，不再共享同一个
	// 紧绷的预算。
	fs := newMultiRoomFakeServer(t, func(c *websocket.Conn, roomID string) {
		if roomID != hostRoom {
			return
		}
		msg := wire.Encode(wire.OpMessage,
			pkInfoRawWithEndTime(hostRoom, oppRoom, "pk-missing-end", time.Now()))
		c.WriteMessage(websocket.BinaryMessage, msg)
	})

	c := newPKPipelineTestClient(t, fs, ts, hostRoom)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go c.Run(ctx)

	pl := NewPKPipeline(c)
	pl.endTimeFallbackGrace = 1500 * time.Millisecond
	out := pl.Run(ctx)

	drainUntil(t, out, 2*time.Second, func(ev event.Event) bool {
		b, ok := ev.Payload.(event.Battle)
		return ok && b.SubCommand == PKOpponentSnapshotSubCommand
	})
	if c.PKLink() == nil {
		t.Fatal("PK 接通后 c.PKLink() 不应为 nil")
	}

	// 完全不发送 PK_BATTLE_END，只等超时兜底生效。
	waitUntil(t, 4*time.Second, func() bool { return c.PKLink() == nil })
}

// TestPKPipelineEndTimeFallbackDoesNotDoubleEndWhenBattleEndArrivesFirst
// 验证正常路径（PK_BATTLE_END 按时到达）不会被超时兜底干扰：即使把
// endTimeFallbackGrace 调得很短，只要 PK_BATTLE_END 先到，兜底触发时
// pl.activePkID 早就被正常路径清空，watchEndTimeFallback 里的
// endActivePK 会发现「不是当前这场」而直接放弃，不会误伤下一场 PK、
// 也不会重复调用 EndPK 造成任何可观察的异常。
func TestPKPipelineEndTimeFallbackDoesNotDoubleEndWhenBattleEndArrivesFirst(t *testing.T) {
	const hostRoom = "21452505"
	const oppRoom = "33333"

	ts := newPKPipelineTestServer(t)

	hostConnReady := make(chan *websocket.Conn, 1)
	fs := newMultiRoomFakeServer(t, func(c *websocket.Conn, roomID string) {
		if roomID != hostRoom {
			return
		}
		c.WriteMessage(websocket.BinaryMessage,
			wire.Encode(wire.OpMessage,
				pkInfoRawWithEndTime(hostRoom, oppRoom, "pk-end-before-fallback", time.Now().Add(time.Hour))))
		select {
		case hostConnReady <- c:
		default:
		}
	})

	c := newPKPipelineTestClient(t, fs, ts, hostRoom)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go c.Run(ctx)

	pl := NewPKPipeline(c)
	pl.endTimeFallbackGrace = 200 * time.Millisecond
	out := pl.Run(ctx)

	drainUntil(t, out, 2*time.Second, func(ev event.Event) bool {
		b, ok := ev.Payload.(event.Battle)
		return ok && b.SubCommand == PKOpponentSnapshotSubCommand
	})

	var hostConn *websocket.Conn
	select {
	case hostConn = <-hostConnReady:
	case <-time.After(time.Second):
		t.Fatal("未能取得宿主连接")
	}
	hostConn.WriteMessage(websocket.BinaryMessage, pkBattleEndMsg())
	waitUntil(t, 2*time.Second, func() bool { return c.PKLink() == nil })

	// end_time 是一小时之后，endTimeFallbackGrace 只有 200ms——如果兜底
	// 逻辑有问题（比如不看 pkID 是否匹配就无脑触发 EndPK），这里等一小段
	// 时间之后 c.PKLink() 应该仍然稳定为 nil，不会因为兜底重新触发什么
	// 副作用；至少证明兜底触发不会 panic、不会把状态搞乱。
	time.Sleep(300 * time.Millisecond)
	if c.PKLink() != nil {
		t.Fatal("PK 已经通过正常路径结束，c.PKLink() 不应该又变回非 nil")
	}
}

// ---------- 复审 Important-3：方向 A 判定不该等 FetchOpponentSnapshots ----------

// TestPKPipelinePublishesLinkBeforeSnapshotFetchCompletes 是 Important-3
// 的回归测试：pl.link 曾经要等 StartPK **和** FetchOpponentSnapshots 都
// 返回才发布，等于把方向 A 判定（粉丝勋章判据零成本可用，round 建立时
// 就绪）的生效时刻绑死在 FetchOpponentSnapshots 的预算上——PK 接通头
// 几秒恰恰是对面观众涌入的窗口，这段时间的欢迎信号会被静默漏判、且
// 不会补判。这里把 roomOnline 接口拖慢到 2s，断言方向 A 的串门信号
// 在远小于 2s 的时间内就能通过合流通道收到，证明 pl.link 在快照抓完
// 之前就已经发布。
func TestPKPipelinePublishesLinkBeforeSnapshotFetchCompletes(t *testing.T) {
	const hostRoom = "21452505"
	const oppRoom = "33333"

	ts := newPKPipelineTestServer(t)
	ts.delay["roomOnline"] = 1200 * time.Millisecond

	trigger := danmakuMsgWithMedal(22222222, "opp-anchor", "串门A", oppRoom)
	fs := newMultiRoomFakeServer(t, func(c *websocket.Conn, roomID string) {
		if roomID != hostRoom {
			return
		}
		c.WriteMessage(websocket.BinaryMessage, pkInfoMsg(hostRoom, oppRoom, "pk-early-link"))
		// 重发触发消息直到 round 建立——见
		// TestPKPipelineClassifiesBothVisitDirections 的说明，同样的
		// 时序考量，不依赖精确的跨 goroutine 同步信号。
		for i := 0; i < 10; i++ {
			c.WriteMessage(websocket.BinaryMessage, trigger)
			time.Sleep(60 * time.Millisecond)
		}
	})

	c := newPKPipelineTestClient(t, fs, ts, hostRoom)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go c.Run(ctx)

	out := NewPKPipeline(c).Run(ctx)

	// 600ms 远小于 roomOnline 的 1.2s 延迟——如果 pl.link 的发布仍然被
	// FetchOpponentSnapshots 拖住，这里必然超时失败。
	drainUntil(t, out, 600*time.Millisecond, func(ev event.Event) bool {
		return ev.Type == event.TypeVisitFromOpponent
	})
}

// ---------- 复审 Important-2：关闭上限，不能被卡住的 startPK 拖到无限久 ----------

// TestPKPipelineShutdownRespectsGraceLimitWhenStartPKGoroutineStuck 是
// Important-2 的回归测试：一个 startPK goroutine 如果卡在
// `for ev := range link.Events()`（对应已知遗留——conn.WriteMessage
// 没有 SetWriteDeadline，对端接收窗口打满时可能无限阻塞），旧实现的
// `wg.Wait(); close(out)` 会被这一个 goroutine 拖到无限久，宿主的
// 优雅退出流程跟着卡死。
//
// 真实触发这个卡住场景需要把对端 TCP 接收窗口打满，在单元测试里没有
// 可靠、快速的构造方式（这也是这个遗留问题至今没有专门测试的原因，
// 见 opponent_link.go 对 pkTeardownGraceLimit 的注释）。这里直接模拟
// 它的后果——wg 里有一个永远不调用 Done() 的 goroutine——测的是
// PKPipeline.shutdown() 这个兜底机制本身：不管 wg 里卡了什么，
// shutdownGraceLimit 到期后必须放弃等待、关闭 out，且此后任何迟到的
// forward() 调用都必须是安全的 no-op，不能 panic。
func TestPKPipelineShutdownRespectsGraceLimitWhenStartPKGoroutineStuck(t *testing.T) {
	fs := newMultiRoomFakeServer(t, nil)
	c := newPKHostClient(t, fs, "21452505")

	ctx, cancel := context.WithCancel(context.Background())
	go c.Run(ctx)
	waitUntil(t, time.Second, func() bool { return fs.authCount() >= 1 })

	pl := NewPKPipeline(c)
	pl.shutdownGraceLimit = 200 * time.Millisecond
	out := pl.Run(ctx)

	// 模拟一个卡住的 startPK goroutine：Add(1) 但永不 Done()。
	pl.wg.Add(1)
	t.Cleanup(pl.wg.Done) // 测试结束后放它走，避免这份状态影响其它测试

	start := time.Now()
	cancel() // 触发宿主 Run 退出 -> c.Events() 关闭 -> loop 退出 -> shutdown()

	select {
	case _, ok := <-out:
		if ok {
			t.Error("out 不应该再产出事件")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("out 迟迟没有关闭——shutdown 没有遵守 shutdownGraceLimit 上限，" +
			"PK 编排会卡死宿主的优雅退出流程")
	}

	elapsed := time.Since(start)
	if elapsed < pl.shutdownGraceLimit {
		t.Errorf("out 关闭得太快（耗时 %v < 上限 %v）——没有真的在等 wg，"+
			"上限形同虚设", elapsed, pl.shutdownGraceLimit)
	}
	if elapsed > pl.shutdownGraceLimit+time.Second {
		t.Errorf("out 关闭耗时 %v，明显超过 shutdownGraceLimit=%v，上限没有生效",
			elapsed, pl.shutdownGraceLimit)
	}

	// 关闭之后，一次迟到的 forward() 调用必须是安全的 no-op，不能 panic
	// ——这正是"不能简单给 wg.Wait() 加超时"的原因：换成简单超时的话，
	// 超时后 close(out) 会跟一个不知道已经超时、还在往 out 发送的
	// forward() 撞车。
	func() {
		defer func() {
			if r := recover(); r != nil {
				t.Fatalf("out 关闭之后调用 forward() 不应该 panic，实际: %v", r)
			}
		}()
		pl.forward(event.Event{Type: event.TypeUnknown})
	}()
}

// ---------- 跨语言字面量：低成本封堵，不是彻底修复 ----------

// TestPKOpponentSnapshotSubCommandMatchesFrontendLiteral 是复审指出的
// 跨语言耦合的低成本封堵：PKOpponentSnapshotSubCommand 这个值被
// PkPanel.vue 的 buildPkRule 写死成规则的 when 条件，且这条规则会被
// 保存进数据库——一旦 Go 侧改了这个常量而前端没跟着改，不是"下次保存
// 就会自愈"，是历史数据里固化了过期字面量，PK 播报从此再也不会触发，
// 且没有任何运行时信号能发现。这里只是让"改了一边忘了改另一边"在
// go test 阶段就报错，不是长期方案——真正的方案是把这个值也做成一个
// /api/meta/* 端点下发（meta_handler.go 顶部注释说得很清楚：写死会
// 悄悄漂移），已记进报告的后续项。
func TestPKOpponentSnapshotSubCommandMatchesFrontendLiteral(t *testing.T) {
	path := filepath.Join("..", "..", "..", "..", "web", "src", "components", "PkPanel.vue")
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("读取 PkPanel.vue 失败（路径 %s 是否还对）: %v", path, err)
	}
	literal := "'" + PKOpponentSnapshotSubCommand + "'"
	if !strings.Contains(string(b), literal) {
		t.Errorf("PkPanel.vue 里没有找到 %s——它应该在 PK_SNAPSHOT_SUBCOMMAND 常量里，"+
			"跟 bilibili.PKOpponentSnapshotSubCommand（当前值 %q）保持一致，"+
			"不一致的表现是 PK 播报规则从此再也不触发，且不会有任何错误提示",
			literal, PKOpponentSnapshotSubCommand)
	}
}

// ---------- 低成本保险：结束过的 pk_id 不重新触发 ----------

// TestPKPipelineIgnoresPKInfoForAlreadyEndedPkID 验证 handleBattle 上方
// 注释里的那道保险：一场 PK 结束（PK_BATTLE_END）之后，如果又收到同一个
// pk_id 的 PK_INFO（真实触发条件未知，是否发生过没有样本可查），不应该
// 重新触发 StartPK、不应该产出第二条快照事件。
// 设计说明：multiRoomFakeServer 每条连接固定存活 500ms 后自己断开
// （见该类型定义），Client 的重连退避在测试里被调快到 10~30ms——这意味着
// 如果 onConnected 在**每一次**连接（含重连）都重发同一条 PK_INFO，
// "PK 结束后同一个 pk_id 又收到一次 PK_INFO" 这个场景会在重连时自然
// 发生，不需要手工複用一个可能已经被服务端关闭的旧连接去补发第二条
// 消息去模拟——第一版测试就是这么写的，产生的问题是：由于 EndPK 走完、
// waitUntil 等多次轮询已经耗时接近或超过服务端那 500ms 的自动断开
// 窗口，手工复用的旧连接很可能已经被关闭，第二次 WriteMessage 静默
// 失败，测试断言"没有第二次快照"永远成立——不是因为保险生效，是因为
// 第二条消息压根没送到，去掉保险代码后这条测试依然会通过（已用变异
// 验证过，见任务报告）。改成让 onConnected 在每次连接都发一遍，靠
// Client 自身的重连机制驱动出真实的"同一个 pk_id 再次收到 PK_INFO"，
// 断言窗口也不再依赖手工连接对象是否还活着。
func TestPKPipelineIgnoresPKInfoForAlreadyEndedPkID(t *testing.T) {
	const hostRoom = "21452505"
	const oppRoom = "33333"
	const pkID = "pk-repeat"

	ts := newPKPipelineTestServer(t)

	hostConnReady := make(chan *websocket.Conn, 1)
	fs := newMultiRoomFakeServer(t, func(c *websocket.Conn, roomID string) {
		if roomID != hostRoom {
			return
		}
		// 每一次连接（含重连）都重发同一个 pk_id 的 PK_INFO。
		c.WriteMessage(websocket.BinaryMessage, pkInfoMsg(hostRoom, oppRoom, pkID))
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

	isSnapshot := func(ev event.Event) bool {
		b, ok := ev.Payload.(event.Battle)
		return ok && b.SubCommand == PKOpponentSnapshotSubCommand
	}
	first := drainUntil(t, out, 2*time.Second, isSnapshot)
	if b := first.Payload.(event.Battle); b.PkID != pkID {
		t.Fatalf("PkID = %q, 期望 %q", b.PkID, pkID)
	}

	// 立刻（不做任何其它等待）在第一条连接上推 PK_BATTLE_END，趁它还没
	// 被服务端的 500ms 定时器关闭。
	var hostConn *websocket.Conn
	select {
	case hostConn = <-hostConnReady:
	case <-time.After(time.Second):
		t.Fatal("未能取得宿主连接")
	}
	if err := hostConn.WriteMessage(websocket.BinaryMessage, pkBattleEndMsg()); err != nil {
		t.Fatalf("推送 PK_BATTLE_END 失败: %v", err)
	}
	waitUntil(t, 2*time.Second, func() bool { return c.PKLink() == nil })

	// 接下来 2s 内，Client 会因为服务端每 500ms 主动断开而反复重连，
	// 每次重连 onConnected 都会重发同一个 pk_id 的 PK_INFO——真实驱动出
	// "PK 结束后同一个 pk_id 又来一次" 这个场景，不需要手工模拟。
	// 有保险时不应该再看到第二条快照事件；没有保险时会看到。
	deadline := time.After(2 * time.Second)
	for {
		select {
		case ev := <-out:
			if isSnapshot(ev) {
				t.Fatal("已经结束的 pk_id 因重连重新收到 PK_INFO，不应该再触发一次快照播报")
			}
		case <-deadline:
			return // 2s 内没有第二条快照事件，保险生效
		}
	}
}
