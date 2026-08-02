package bilibili

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/MrZhongzq/MagicalDanmaku/server/internal/connector/bilibili/cmdmap"
	"github.com/MrZhongzq/MagicalDanmaku/server/internal/event"
)

// ---------- 测试辅助：直接摆弄 PkLink 内部状态，跳过真实网络连接 ----------
//
// ClassifyVisit 的判定逻辑只依赖 p.round/p.mine/p.opposite/p.mineSeed/
// p.oppositeSeed/p.host.roomID 这几个字段，不涉及任何 goroutine/网络
// I/O，所以这里不需要 opponent_link_test.go 里那一整套假服务器，直接
// 手工构造 PkLink 状态更快、也更能聚焦在判定逻辑本身。
//
// 第二轮审查的 Critical-1 明确指出：凡是要验证 mine/opposite 实时集合
// 相关判据的测试，必须按真实管道的调用顺序摆弄状态（先
// trackOpposite/observeMine，再 ClassifyVisit），不能绕开顺序直接摆
// 数据——绕开顺序曾经让一个真实存在的缺陷（观众判据在真实管道下恒不
// 成立）被测试误判成"已覆盖"。下面涉及 mine/opposite 的测试都遵循这
// 个纪律；只跟 mineSeed/oppositeSeed（冻结快照，没有顺序敏感性）或
// 粉丝勋章判据相关的测试不受这条约束。

// newTestPkLinkWithRound 构造一个「PK 进行中」的 PkLink：host 绑定
// selfRoomID，round.opponentRoomIDs/opponentRoomIDsOrdered 都是
// opponentRoomIDs（保持调用方传入的顺序，供确定性测试用）。
func newTestPkLinkWithRound(selfRoomID string, opponentRoomIDs ...string) *PkLink {
	host := &Client{roomID: selfRoomID}
	p := newPkLink(host)
	members := pkMembersOf(opponentRoomIDs)
	p.round = &pkRound{
		opponentRoomIDs:        opponentRoomIDSet(members),
		opponentRoomIDsOrdered: opponentRoomIDsOrdered(members),
	}
	return p
}

// pkMembersOf 是测试专用的小工具：把房间号列表包装成 opponentRoomIDSet
// 能吃的 []event.PkMember（这里只关心 RoomID，UID 留空）。
func pkMembersOf(roomIDs []string) []event.PkMember {
	members := make([]event.PkMember, 0, len(roomIDs))
	for _, r := range roomIDs {
		members = append(members, event.PkMember{RoomID: r})
	}
	return members
}

func danmakuFrom(roomID, uid string, medal *event.Medal) event.Event {
	return event.Event{
		RoomID: roomID,
		Type:   event.TypeDanmaku,
		Payload: event.Danmaku{
			User: event.User{UID: uid, Medal: medal},
			Text: "hi",
		},
	}
}

// ---------- 硬性约束：PK 未开启时不产生事件 ----------

// TestClassifyVisitRequiresActivePK 验证「必须只在 PK 期间生效」这条
// 硬性约束：即使传进来的事件数据本该命中判据，round 为 nil（没有进行
// 中的 PK）时也必须原样放行，不产生任何串门信号。
func TestClassifyVisitRequiresActivePK(t *testing.T) {
	host := &Client{roomID: "self"}
	p := newPkLink(host) // 没有调用 connect，p.round 保持 nil

	p.addOppositeRoom("opp", "u1") // 即使观众集合已经有数据……
	ev := danmakuFrom("self", "u1", nil)

	if _, ok := p.ClassifyVisit(ev); ok {
		t.Fatal("PK 未开启（round 为 nil）时不应该产生串门信号")
	}
}

// ---------- 方向 A：对面的人跑来我方房间（欢迎） ----------

func TestClassifyVisitFromOpponentByFanMedal(t *testing.T) {
	p := newTestPkLinkWithRound("self", "opp")

	medal := &event.Medal{RoomID: "opp"}
	ev := danmakuFrom("self", "u1", medal)

	got, ok := p.ClassifyVisit(ev)
	if !ok {
		t.Fatal("戴着对手主播粉丝牌，应该判定为方向 A 的串门")
	}
	if got.Type != event.TypeVisitFromOpponent {
		t.Errorf("Type = %v, 期望 %v", got.Type, event.TypeVisitFromOpponent)
	}
	payload, ok := got.Payload.(event.VisitFromOpponent)
	if !ok {
		t.Fatalf("Payload 类型 = %T, 期望 event.VisitFromOpponent", got.Payload)
	}
	if payload.OpponentRoomID != "opp" {
		t.Errorf("OpponentRoomID = %q, 期望 %q", payload.OpponentRoomID, "opp")
	}
	if payload.MatchedBy != event.VisitMatchedByFanMedal {
		t.Errorf("MatchedBy = %q, 期望 %q", payload.MatchedBy, event.VisitMatchedByFanMedal)
	}
	if payload.User.UID != "u1" {
		t.Errorf("User.UID = %q, 期望 %q", payload.User.UID, "u1")
	}
}

// TestClassifyVisitFromOpponentByAudienceSet 是第三轮审查 New-2 的核心
// 回归测试：必须按真实管道顺序摆数据，而且必须让命中路径本身真的经过
// `observeMine`，否则测不出 mineSeed 被误写成 mine 这个回退（第二轮
// 审查特别警告过的踩坑写法）。
//
// 真实管道里 u1 先在对面房间发过言（runOpponent 转发前先调用
// trackOpposite），然后跑来我方房间发言——而 client.go 的事件钩子
// observeMine 对*任何*到达消费者之前的本房间事件都会同步执行
// （handleMessage 无条件调用 hook(ev)，不管这个人是不是"真的算我方的"），
// 所以到 ClassifyVisit 被调用的这一刻，u1 必然已经被 observeMine 写进了
// 实时 mine——如果排除条件读的是 mine 而不是 mineSeed，这里会被误判成
// "他是我方老观众"从而排除掉、判定失败。上一版测试没有调用 observeMine，
// 所以查 mine 和查 mineSeed 在测试里看起来行为一样（复审变异 mineSeed→mine
// 后这条测试依然是绿的，抓不到），这一版补上这一步，让它真正对这个
// 回退敏感。
func TestClassifyVisitFromOpponentByAudienceSet(t *testing.T) {
	p := newTestPkLinkWithRound("self", "opp")
	p.trackOpposite("opp", danmakuFrom("opp", "u1", nil)) // u1 先在对面说过话

	ev := danmakuFrom("self", "u1", nil) // 没有戴任何粉丝牌，现在跑来我方
	p.observeMine(ev)                    // 真实管道：client.go 的钩子会先同步跑一遍

	got, ok := p.ClassifyVisit(ev)
	if !ok {
		t.Fatal("出现在 oppositeAudience 里，应该判定为方向 A 的串门")
	}
	if got.Type != event.TypeVisitFromOpponent {
		t.Errorf("Type = %v, 期望 %v", got.Type, event.TypeVisitFromOpponent)
	}
	payload := got.Payload.(event.VisitFromOpponent)
	if payload.MatchedBy != event.VisitMatchedByAudience {
		t.Errorf("MatchedBy = %q, 期望 %q", payload.MatchedBy, event.VisitMatchedByAudience)
	}
	if payload.OpponentRoomID != "opp" {
		t.Errorf("OpponentRoomID = %q, 期望 %q", payload.OpponentRoomID, "opp")
	}
}

// TestClassifyVisitFromOpponentIgnoresOwnRegularWhoVisitedAndReturned
// 是第二轮审查 Important-2 的回归测试：我方一个 PK 前就已经是常驻观众
// 的人（种进 mineSeed），中途去对面串了个门（真实管道下 trackOpposite
// 会把他计入对面观众的实时集合），回到我方房间发言——不应该被误判成
// 「对面来的客人」。裁决：这条按冻结快照修（mineSeed 排除），不是简报
// 与实现冲突，是上游 Task 6a 引入实时追踪导致的语义漂移。
func TestClassifyVisitFromOpponentIgnoresOwnRegularWhoVisitedAndReturned(t *testing.T) {
	p := newTestPkLinkWithRound("self", "opp")
	p.seedMine("u1") // u1 是 PK 前就已经是我方的常驻观众（冻结快照）

	// 真实管道：他中途去对面说了句话，runOpponent 转发给消费者之前
	// 先调用 trackOpposite，把他计入对面观众的实时集合。
	p.trackOpposite("opp", danmakuFrom("opp", "u1", nil))

	// 现在他回到我方房间发言——真实管道下 client.go 的事件钩子
	// observeMine 会在消费者拿到事件之前先跑一遍，把他也写进实时
	// mine（这个动作本身不该影响判定，因为排除用的是 mineSeed）。
	hostEv := danmakuFrom("self", "u1", nil)
	p.observeMine(hostEv)

	if got, ok := p.ClassifyVisit(hostEv); ok {
		t.Fatalf("我方常驻观众回到我方房间，不应该被判定成「对面来的客人」，实际判定为 %+v", got)
	}
}

// TestClassifyVisitFromOpponentMisclassifiesOwnRegularWhenSeedIncomplete
// 是第三轮审查 New-1 的记录性测试：钉住一个已知、经过裁决接受的局限，
// 不是要修的缺陷。
//
// 场景：u1 事实上是 PK 前就已经是我方的常驻观众，但因为播种
// mineSeed 的那一路（RoomRecentDanmakuUIDs 拉近期弹幕发送者）要么还
// 没跑完（connect 返回到 seedAudiences 那个独立 goroutine 真正播种
// 完成之间有一个窗口）、要么请求失败（seedAudiences 对失败只 Warn +
// continue，此后整场 PK 种子集合都不会再补全，是永久性的，不是暂时
// 的）——总之 u1 没能进 mineSeed。他中途去对面串了个门、又回到我方
// 房间发言，跟一个真实的对面观众在协议层完全没有区别，会被误判成
// TypeVisitFromOpponent。
//
// 裁决（选项 b）：不加"种子未就绪就不判"的门——"未就绪"（暂时）和
// "HTTP 失败后永久不完整"是两种性质不同的状态，一道门只能盖住前者，
// 而且会让串门判定在 PK 刚开始的一段时间内整体失效，代价不比现在的
// 误判小。保留当前行为，用这条测试把它钉成明确记录在案的已知局限，
// 不是没人知道的隐藏缺陷。完整的权衡记录见 opponent_link.go
// seedAudiences 函数上方的注释和 task-6b-report.md 的 New-1 一节。
func TestClassifyVisitFromOpponentMisclassifiesOwnRegularWhenSeedIncomplete(t *testing.T) {
	p := newTestPkLinkWithRound("self", "opp")
	// 故意不调用 p.seedMine("u1")：模拟播种 u1 这个老观众的那次 HTTP
	// 请求还没跑完/已经失败，mineSeed 里没有他。

	p.trackOpposite("opp", danmakuFrom("opp", "u1", nil)) // 他中途去对面说了句话

	ev := danmakuFrom("self", "u1", nil) // 现在他回到我方房间
	p.observeMine(ev)

	got, ok := p.ClassifyVisit(ev)
	if !ok {
		t.Fatal("已知局限的记录性断言失败：本该复现「种子不完整时被误判」这个已知行为，" +
			"实际却没有命中——如果这里变成了 false，说明底层判定逻辑变了，" +
			"请回读 New-1 的权衡记录，确认是否需要同步更新这条测试和相关注释")
	}
	if got.Type != event.TypeVisitFromOpponent {
		t.Errorf("Type = %v, 期望 %v（误判成了「对面来的客人」，这正是这条测试要钉住的已知局限）",
			got.Type, event.TypeVisitFromOpponent)
	}
}

// TestClassifyVisitFromOpponentDeterministicOpponentRoomIDAcrossMultipleOpponents
// 是第二轮审查 Minor-3 的回归测试：多人 PK 下同一个人同时出现在两个
// 对手房间的观众集合里时，OpponentRoomID 必须是确定的（取 PK_INFO 原始
// 顺序里排在前面的那个），不能因为遍历 map 顺序随机而在多次调用间
// 摇摆——重复跑几十次，结果必须每次都一样。
func TestClassifyVisitFromOpponentDeterministicOpponentRoomIDAcrossMultipleOpponents(t *testing.T) {
	p := newTestPkLinkWithRound("self", "oppA", "oppB") // 顺序：oppA 排第一
	p.trackOpposite("oppA", danmakuFrom("oppA", "u1", nil))
	p.trackOpposite("oppB", danmakuFrom("oppB", "u1", nil))

	ev := danmakuFrom("self", "u1", nil)
	for i := 0; i < 30; i++ {
		got, ok := p.ClassifyVisit(ev)
		if !ok {
			t.Fatalf("第 %d 次调用应该命中", i)
		}
		payload := got.Payload.(event.VisitFromOpponent)
		if payload.OpponentRoomID != "oppA" {
			t.Fatalf("第 %d 次调用 OpponentRoomID = %q, 期望恒为 %q（PK_INFO 原始顺序里排第一的对手）",
				i, payload.OpponentRoomID, "oppA")
		}
	}
}

// TestClassifyVisitFromOpponentIgnoresUnrelatedMedal 验证粉丝牌判据不是
// 见到任何粉丝牌就算数——必须是这一轮 PK 里某个对手主播的勋章，戴着
// 跟这场 PK 毫不相干的第三方主播的勋章不该被误判成串门。
func TestClassifyVisitFromOpponentIgnoresUnrelatedMedal(t *testing.T) {
	p := newTestPkLinkWithRound("self", "opp")

	medal := &event.Medal{RoomID: "some-other-streamer"} // 跟这场 PK 无关
	ev := danmakuFrom("self", "u1", medal)

	if _, ok := p.ClassifyVisit(ev); ok {
		t.Fatal("戴着跟这场 PK 无关的第三方主播粉丝牌，不应该判定为串门")
	}
}

// TestClassifyVisitFromOpponentIgnoresNonAudienceStranger 验证「命中任一
// 判据」不等于「本房间事件全都算串门」——两个判据都不中的人是正常的
// 本房间观众，不该被误判。
func TestClassifyVisitFromOpponentIgnoresNonAudienceStranger(t *testing.T) {
	p := newTestPkLinkWithRound("self", "opp")
	ev := danmakuFrom("self", "stranger", nil)

	if _, ok := p.ClassifyVisit(ev); ok {
		t.Fatal("两个判据都没命中，不应该判定为串门")
	}
}

// ---------- 方向 B：我方观众跑去对面房间（警示） ----------

// TestClassifyVisitToOpponentByAudienceSet 是第二轮审查 Critical-1 的
// 核心回归测试：必须按真实管道的调用顺序摆数据——runOpponent 转发事件
// 给消费者之前会先同步调用 trackOpposite（opponent_link.go），也就是
// 说等 ClassifyVisit 被调用时，u1 因为「这条正在判定的事件本身」已经
// 被写进了 p.opposite 实时集合。旧实现（`!inOpposite` 查的是实时
// 集合）在这个真实顺序下会被自我污染、恒判定为「已经是对面的人」，
// 整条观众判据失效；改成查 p.oppositeSeed（冻结快照，trackOpposite
// 不会写它）之后才能在这个真实顺序下正确工作。
func TestClassifyVisitToOpponentByAudienceSet(t *testing.T) {
	p := newTestPkLinkWithRound("self", "opp")
	p.addMine("u1") // u1 是我方观众

	ev := danmakuFrom("opp", "u1", nil) // 事件来自对面连接（RoomID=opp）
	p.trackOpposite("opp", ev)          // 真实管道顺序：runOpponent 先 track 再转发

	got, ok := p.ClassifyVisit(ev)
	if !ok {
		t.Fatal("按真实管道顺序，我方观众出现在对面事件流里，应该判定为方向 B 的串门")
	}
	if got.Type != event.TypeVisitToOpponent {
		t.Errorf("Type = %v, 期望 %v", got.Type, event.TypeVisitToOpponent)
	}
	payload, ok := got.Payload.(event.VisitToOpponent)
	if !ok {
		t.Fatalf("Payload 类型 = %T, 期望 event.VisitToOpponent", got.Payload)
	}
	if payload.MatchedBy != event.VisitMatchedByAudience {
		t.Errorf("MatchedBy = %q, 期望 %q", payload.MatchedBy, event.VisitMatchedByAudience)
	}
	if payload.OpponentRoomID != "opp" {
		t.Errorf("OpponentRoomID = %q, 期望 %q", payload.OpponentRoomID, "opp")
	}
}

// TestClassifyVisitToOpponentExcludesDualAudience 验证
// !oppositeAudience.contains(uid) 那个否定条件不是多余的：对面的常驻
// 观众可能同时也在我方集合里（两边都看），这种人不算「跑去对面串门」
// ——他本来就是对面的人。「常驻」是 PK 前就已经确立的事实，所以用
// seedOppositeRoom（冻结快照）模拟，不是这条事件当场造成的实时归属
// （那种情况见 TestClassifyVisitToOpponentByAudienceSet）。
func TestClassifyVisitToOpponentExcludesDualAudience(t *testing.T) {
	p := newTestPkLinkWithRound("self", "opp")
	p.addMine("u1")
	p.seedOppositeRoom("opp", "u1") // PK 前就已经是对面的常驻观众

	ev := danmakuFrom("opp", "u1", nil)
	p.trackOpposite("opp", ev) // 真实管道顺序：这条事件也会经过 trackOpposite

	if _, ok := p.ClassifyVisit(ev); ok {
		t.Fatal("既是我方观众又是 PK 前就已经是对面的常驻观众，不应该判定为串门")
	}
}

// TestClassifyVisitToOpponentByFanMedal 验证第二个判据：戴着我方主播的
// 粉丝牌，即使这个人从未出现在 myAudience 里也该算数（原 C++ 用 || 连接
// 两个判据，不要求同时成立）。
func TestClassifyVisitToOpponentByFanMedal(t *testing.T) {
	p := newTestPkLinkWithRound("self", "opp")
	// 故意不调用 p.addMine("u1")：myAudience 里没有这个人。

	medal := &event.Medal{RoomID: "self"} // 戴着我方主播的粉丝牌
	ev := danmakuFrom("opp", "u1", medal)
	p.trackOpposite("opp", ev) // 真实管道顺序，证明粉丝牌判据不受实时集合污染影响

	got, ok := p.ClassifyVisit(ev)
	if !ok {
		t.Fatal("戴着我方主播粉丝牌，即使不在 myAudience 里也该判定为串门")
	}
	payload := got.Payload.(event.VisitToOpponent)
	if payload.MatchedBy != event.VisitMatchedByFanMedal {
		t.Errorf("MatchedBy = %q, 期望 %q", payload.MatchedBy, event.VisitMatchedByFanMedal)
	}
}

func TestClassifyVisitToOpponentIgnoresOpponentsOwnAudience(t *testing.T) {
	p := newTestPkLinkWithRound("self", "opp")
	p.seedOppositeRoom("opp", "u1") // u1 是 PK 前就已经是对面的常驻观众，不是我方的

	ev := danmakuFrom("opp", "u1", nil)
	p.trackOpposite("opp", ev)

	if _, ok := p.ClassifyVisit(ev); ok {
		t.Fatal("对面自己的常驻观众不应该被判定为「我方观众跑来串门」")
	}
}

// ---------- 事件来源房间号不属于当前这轮 PK：不产生信号 ----------

func TestClassifyVisitFromUnrelatedRoomProducesNothing(t *testing.T) {
	p := newTestPkLinkWithRound("self", "opp")
	p.addMine("u1")

	ev := danmakuFrom("not-in-this-pk", "u1", nil)

	if _, ok := p.ClassifyVisit(ev); ok {
		t.Fatal("事件来自跟当前这轮 PK 无关的房间号，不应该产生串门信号")
	}
}

// ---------- 两个方向必须是可区分的独立事件类型 ----------

// TestVisitDirectionsAreDistinctTypes 是对「两个方向语义相反，必须可
// 区分」这条硬性约束最直接的断言：用对称的数据分别触发两个方向，
// 断言产出的 Type 不相等、且 Payload 类型也不同——防止未来有人图省事
// 把两个方向合并成一个事件配布尔字段。
func TestVisitDirectionsAreDistinctTypes(t *testing.T) {
	p := newTestPkLinkWithRound("self", "opp")
	p.addOppositeRoom("opp", "visitor") // 方向 A：对面的人来我方
	p.addMine("host-fan")               // 方向 B：我方的人去对面

	fromOpp, ok := p.ClassifyVisit(danmakuFrom("self", "visitor", nil))
	if !ok {
		t.Fatal("方向 A 应该命中")
	}
	toOpp, ok := p.ClassifyVisit(danmakuFrom("opp", "host-fan", nil))
	if !ok {
		t.Fatal("方向 B 应该命中")
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

// ---------- 没有可解析 User 的事件类型：不产生信号 ----------

func TestClassifyVisitIgnoresEventsWithoutUser(t *testing.T) {
	p := newTestPkLinkWithRound("self", "opp")
	ev := event.Event{RoomID: "self", Type: event.TypeRoomChange, Payload: event.RoomChange{Title: "t"}}

	if _, ok := p.ClassifyVisit(ev); ok {
		t.Fatal("没有 User 字段的事件类型不应该产生串门信号")
	}
}

// ---------- 黄金样本：blindbox.jsonl 第 446 行的真实案例 ----------

// TestClassifyVisitGoldenSampleOpponentAnchorGiftsHostRoom 复现
// server/blindbox.jsonl 第 446 行的真实案例（脱敏后落到
// testdata/cmds/SEND_GIFT_pk_visit_from_opponent.json）：PK 期间对面
// 主播本人给本房间送了一个「粉丝团灯牌」。
//
// 这条样本专门用来验证一个只看代码推不出来、必须拿真实数据核对的
// 细节：这一条真实样本里 SEND_GIFT 的 medal_info.anchor_roomid 是 0。
//
// 【第二轮审查订正】这里只能说「这一条真实样本实测是 0」，不能像上一版
// 那样引用原 C++ bili_livecmds.cpp:2907 的注释「!注意：这个一直为0」
// 来交叉印证——审查指出那条注释证明的是 C++ 自己读错了键名（它读的是
// 带下划线的 medalInfo.value("anchor_room_id")，真实报文里的键是不带
// 下划线的 anchor_roomid），C++ 那句"一直为0"只是「读一个不存在的键，
// 拿到 JSON 默认值 0」的必然结果，跟真实 anchor_roomid 字段本身是否
// 恒为 0 是两回事，不构成独立的第二个数据来源。这里的结论仅由这一条
// n=1 的真实样本支撑，不是「两个独立来源互相印证」。不管原因是不是
// B 站行为本身，这条真实样本证明的事实不变：粉丝牌判据在这次真实送礼
// 场景里没有命中，如果只实现判据 1（粉丝牌）不实现判据 2（观众集合），
// 这个真实存在的案例会被漏判。样本里的 medal_info.anchor_roomid 特意
// 保留原值 0，不用编造的非零值掩盖这一点。
func TestClassifyVisitGoldenSampleOpponentAnchorGiftsHostRoom(t *testing.T) {
	const hostRoomID = "20001" // 收礼房间号（脱敏，非真实值）
	const oppRoomID = "30002"  // 对面主播房间号（脱敏，非真实值）
	const oppAnchorUID = "22223333"

	raw, err := os.ReadFile(filepath.Join(
		"..", "..", "..", "testdata", "cmds", "SEND_GIFT_pk_visit_from_opponent.json"))
	if err != nil {
		t.Fatalf("读取黄金样本失败: %v", err)
	}

	evs, err := cmdmap.Map(cmdmap.Context{RoomID: hostRoomID, ReceivedAt: time.Now()}, raw)
	if err != nil {
		t.Fatalf("Map 返回错误: %v", err)
	}
	if len(evs) != 1 {
		t.Fatalf("期望 1 个事件，实际 %d 个", len(evs))
	}
	giftEvent := evs[0]

	gift, ok := giftEvent.Payload.(event.Gift)
	if !ok {
		t.Fatalf("Payload 类型 = %T, 期望 event.Gift", giftEvent.Payload)
	}
	// 先确认样本本身如实反映了「粉丝牌判据失效」这个真实 B 站行为，
	// 不是测试自己配置错了导致判据 1 没机会触发。
	if gift.User.Medal == nil || gift.User.Medal.RoomID != "0" {
		t.Fatalf("样本前提不成立：期望 medal.RoomID 是损坏的 \"0\"，实际 %+v", gift.User.Medal)
	}

	p := newTestPkLinkWithRound(hostRoomID, oppRoomID)
	p.seedOppositeRoom(oppRoomID, oppAnchorUID) // seedAudiences 会做的事：对手主播本人入对面观众集合

	got, ok := p.ClassifyVisit(giftEvent)
	if !ok {
		t.Fatal("真实黄金样本应该判定为方向 A 的串门（对面主播来我方送礼）")
	}
	if got.Type != event.TypeVisitFromOpponent {
		t.Errorf("Type = %v, 期望 %v", got.Type, event.TypeVisitFromOpponent)
	}
	payload, ok := got.Payload.(event.VisitFromOpponent)
	if !ok {
		t.Fatalf("Payload 类型 = %T, 期望 event.VisitFromOpponent", got.Payload)
	}
	if payload.MatchedBy != event.VisitMatchedByAudience {
		t.Errorf("MatchedBy = %q, 期望 %q（粉丝牌判据在这个真实样本里天生失效，必须靠观众集合兜底）",
			payload.MatchedBy, event.VisitMatchedByAudience)
	}
	if payload.OpponentRoomID != oppRoomID {
		t.Errorf("OpponentRoomID = %q, 期望 %q", payload.OpponentRoomID, oppRoomID)
	}
	if payload.User.UID != oppAnchorUID {
		t.Errorf("User.UID = %q, 期望 %q", payload.User.UID, oppAnchorUID)
	}
}
