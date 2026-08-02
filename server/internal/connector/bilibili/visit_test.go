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
// ClassifyVisit 的判定逻辑只依赖 p.round/p.mine/p.opposite/p.host.roomID
// 这几个字段，不涉及任何 goroutine/网络 I/O，所以这里不需要
// opponent_link_test.go 里那一整套假服务器，直接手工构造 PkLink 状态
// 更快、也更能聚焦在判定逻辑本身。

// newTestPkLinkWithRound 构造一个「PK 进行中」的 PkLink：host 绑定
// selfRoomID，round.opponentRoomIDs 是 opponentRoomIDs。
func newTestPkLinkWithRound(selfRoomID string, opponentRoomIDs ...string) *PkLink {
	host := &Client{roomID: selfRoomID}
	p := newPkLink(host)
	p.round = &pkRound{opponentRoomIDs: opponentRoomIDSet(pkMembersOf(opponentRoomIDs))}
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

func TestClassifyVisitFromOpponentByAudienceSet(t *testing.T) {
	p := newTestPkLinkWithRound("self", "opp")
	p.addOppositeRoom("opp", "u1") // u1 是 PK 期间追踪到的对面观众

	ev := danmakuFrom("self", "u1", nil) // 没有戴任何粉丝牌

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

func TestClassifyVisitToOpponentByAudienceSet(t *testing.T) {
	p := newTestPkLinkWithRound("self", "opp")
	p.addMine("u1") // u1 是我方观众

	ev := danmakuFrom("opp", "u1", nil) // 事件来自对面连接（RoomID=opp）

	got, ok := p.ClassifyVisit(ev)
	if !ok {
		t.Fatal("我方观众出现在对面事件流里，应该判定为方向 B 的串门")
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
// ——他本来就是对面的人。
func TestClassifyVisitToOpponentExcludesDualAudience(t *testing.T) {
	p := newTestPkLinkWithRound("self", "opp")
	p.addMine("u1")
	p.addOppositeRoom("opp", "u1") // 同一个人两边都在看

	ev := danmakuFrom("opp", "u1", nil)

	if _, ok := p.ClassifyVisit(ev); ok {
		t.Fatal("既是我方观众又是对面常驻观众，不应该判定为串门")
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
	p.addOppositeRoom("opp", "u1") // u1 只是对面的常驻观众，不是我方的

	ev := danmakuFrom("opp", "u1", nil)

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
// 细节：SEND_GIFT 的 medal_info.anchor_roomid 在真实报文里恒为 0
// （原 C++ 在 bili_livecmds.cpp:2907 的注释也确认了这一点：「!注意：
// 这个一直为0」），也就是说粉丝牌判据在送礼场景下天生失效，如果只
// 实现判据 1（粉丝牌）不实现判据 2（观众集合），这个真实存在的案例
// 会被漏判。样本里的 medal_info.anchor_roomid 特意保留原值 0，不用
// 编造的非零值掩盖这个真实缺陷。
func TestClassifyVisitGoldenSampleOpponentAnchorGiftsHostRoom(t *testing.T) {
	const hostRoomID = "20001" // 桃酥Su-- 的房间号（脱敏，非真实值）
	const oppRoomID = "30002"  // Q亦巧儿 的房间号（脱敏，非真实值）
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
	p.addOppositeRoom(oppRoomID, oppAnchorUID) // seedAudiences 会做的事：对手主播本人入对面观众集合

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
