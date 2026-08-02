package bilibili

import "github.com/MrZhongzq/MagicalDanmaku/server/internal/event"

// ClassifyVisit 判定一条事件是否触发「串门」信号，命中则返回一个新构造
// 的信号事件。用户已裁决：两个方向都要做，且必须是可区分的独立事件
// （TypeVisitFromOpponent/TypeVisitToOpponent），不合成一个「串门」事件
// 配布尔字段区分方向——漏判一个布尔就会把警示播成欢迎，成本极低而
// 后果尴尬。
//
// 两个方向合并成一个入口，不需要调用方自己分流：Event.RoomID 本身已经
// 能区分「这条事件来自宿主自己的连接」还是「来自某个对手的连接」（同一
// 个约定见 opponent_link.go 顶部对「来源标记」的说明），据此就能唯一
// 确定该往哪个方向判：
//   - ev.RoomID == 宿主房间号 → 事件来自 Client.Events()，判方向 A
//     （对面的人跑来我方，欢迎语气）
//   - ev.RoomID 是当前这轮 PK 的某个对手房间号 → 事件来自
//     PkLink.Events()，判方向 B（我方观众跑去对面，警示语气）
//   - 其它房间号（不属于当前这轮 PK，理论上不该发生，调用方传错了
//     事件来源）：不产生信号，不瞎猜
//
// 硬性约束「必须只在 PK 期间生效」：PK 没开时 p.round 为 nil，直接返回
// false。这里特意查证过简报提到的原 C++ 前置条件 pkBattleType 是什么
// 语义，而不是照抄——它是 pk_basic.battle_type 字段（大乱斗开始/匹配
// 时写成 1 普通/2 视频，PK 结束或异常收尾时清零，见
// bili_liveservice.cpp 里三处 pkBattleType = data.value("battle_type")
// 和一处 pkBattleType = 0），toView 判定里 `pkBattleType &&` 用的只是它
// 「非零即真」这一个性质，等价于「PK 是否进行中」，不携带 toView 判定
// 用得上的其它语义（1/2 的区别只影响 pkVideo，跟串门判定无关）。这个
// Go 重写里「PK 是否进行中」本来就有更直接的信号——p.round 是否为
// nil（PkLink 每场 PK 独立一份，StartPK 时创建、EndPK/异常收尾时清空，
// 见 opponent_link.go）——不需要另外维护一个专门对应 battle_type 的
// 字段去复刻同一件事。
func (p *PkLink) ClassifyVisit(ev event.Event) (event.Event, bool) {
	p.mu.Lock()
	round := p.round
	p.mu.Unlock()
	if round == nil {
		return event.Event{}, false
	}

	if ev.RoomID == p.host.roomID {
		return p.classifyVisitFromOpponent(ev, round)
	}
	if _, isOpponent := round.opponentRoomIDs[ev.RoomID]; isOpponent {
		return p.classifyVisitToOpponent(ev, ev.RoomID)
	}
	return event.Event{}, false
}

// classifyVisitFromOpponent 是方向 A：这条本房间事件里的人，是不是从
// 对面跑过来的。这不是原 C++ 的逻辑（C++ 只有方向 B），是用户裁决要
// 新做的方向，两个判据都要试，命中任一即算：
//  1. 戴着这一轮 PK 里某个对手主播的粉丝牌——原 C++ 也用这个信号
//     （bili_livecmds.cpp:2960 附近，INTERACT_WORD 的
//     fans_medal.anchor_roomid），零成本，数据本来就在报文里。
//  2. 出现在这一轮 PK 里任意一个对手房间的 oppositeAudience 集合里
//     （Task 6a 维护，来自对面房间的实时事件流 + 播种）。
//
// 判据 1 用 round.opponentRoomIDs 而不是 p.opposite 的 key 判断「这个
// 勋章是不是这一轮 PK 的某个对手」，理由见 pkRound.opponentRoomIDs 的
// 字段注释：避免依赖 seedAudiences 那个异步 goroutine 是否已经播种
// 完成。
func (p *PkLink) classifyVisitFromOpponent(ev event.Event, round *pkRound) (event.Event, bool) {
	user, ok := userOf(ev)
	if !ok || user.UID == "" {
		return event.Event{}, false
	}

	if user.Medal != nil {
		if _, isOpponentMedal := round.opponentRoomIDs[user.Medal.RoomID]; isOpponentMedal {
			return p.newVisitEvent(ev, event.TypeVisitFromOpponent, event.VisitFromOpponent{
				User:           user,
				OpponentRoomID: user.Medal.RoomID,
				MatchedBy:      event.VisitMatchedByFanMedal,
			}), true
		}
	}

	p.audMu.Lock()
	defer p.audMu.Unlock()
	for roomID := range round.opponentRoomIDs {
		if _, seen := p.opposite[roomID][user.UID]; seen {
			return p.newVisitEvent(ev, event.TypeVisitFromOpponent, event.VisitFromOpponent{
				User:           user,
				OpponentRoomID: roomID,
				MatchedBy:      event.VisitMatchedByAudience,
			}), true
		}
	}
	return event.Event{}, false
}

// classifyVisitToOpponent 是方向 B，语义照抄原 C++ 的 toView
// （bili_livecmds.cpp:2934/2958，两处写法一致）：
//
//	（不在对面观众集合里 且 在我方观众集合里）或（戴着我方主播的粉丝牌）
//
// 判据顺序照抄原 C++ 子句的先后（先观众集合、后粉丝牌），MatchedBy
// 反映实际命中的是哪一个。
//
// !oppositeAudience.contains(uid) 这个否定条件不是多余的——对面的常驻
// 观众可能也在我方集合里（两边都看），那种人不算「串门」。原作者在这
// 里的注释是「自己这边过去送礼物，居心何在！」，是提示/警示语气，
// 不是欢迎，跟方向 A 语义相反，这也是为什么两个方向必须用不同 Type。
func (p *PkLink) classifyVisitToOpponent(ev event.Event, opponentRoomID string) (event.Event, bool) {
	user, ok := userOf(ev)
	if !ok || user.UID == "" {
		return event.Event{}, false
	}

	p.audMu.Lock()
	_, inOpposite := p.opposite[opponentRoomID][user.UID]
	_, inMine := p.mine[user.UID]
	p.audMu.Unlock()

	switch {
	case !inOpposite && inMine:
		return p.newVisitEvent(ev, event.TypeVisitToOpponent, event.VisitToOpponent{
			User:           user,
			OpponentRoomID: opponentRoomID,
			MatchedBy:      event.VisitMatchedByAudience,
		}), true
	case user.Medal != nil && user.Medal.RoomID == p.host.roomID:
		return p.newVisitEvent(ev, event.TypeVisitToOpponent, event.VisitToOpponent{
			User:           user,
			OpponentRoomID: opponentRoomID,
			MatchedBy:      event.VisitMatchedByFanMedal,
		}), true
	default:
		return event.Event{}, false
	}
}

// newVisitEvent 把命中判定的原始事件包装成一个新的串门信号事件。RoomID
// 统一写成宿主自己的房间号——不管方向 A/B，这个信号本来就是给「绑定
// 这个房间的这一侧」看的（欢迎谁来了/该提防谁走了），跟触发它的原始
// 事件记录在哪条物理连接上（宿主自己 or 某个对手）无关；对手房间号
// 已经在 payload 的 OpponentRoomID 里，不需要在 Event.RoomID 上重复。
// Raw 复用原始事件的，排查时还能回看触发这次判定的原始 CMD 内容。
func (p *PkLink) newVisitEvent(ev event.Event, t event.Type, payload event.Payload) event.Event {
	return event.Event{
		ID:         event.NewID(),
		RoomID:     p.host.roomID,
		Platform:   ev.Platform,
		Type:       t,
		Timestamp:  ev.Timestamp,
		ReceivedAt: ev.ReceivedAt,
		Payload:    payload,
		Raw:        ev.Raw,
	}
}
