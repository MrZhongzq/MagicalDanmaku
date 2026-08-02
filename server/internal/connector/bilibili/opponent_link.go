package bilibili

import (
	"context"
	"sync"

	"github.com/MrZhongzq/MagicalDanmaku/server/internal/event"
)

// PkLink 管理 PK 期间到对手房间的若干条弹幕连接。
//
// 核心思路是「不新增连接逻辑，只编排 Client」：Client 本来就是 per-room
// 的（NewClient + Run + Events()），重连/心跳/鉴权都已经在里面实现好了，
// 「连对面」就是给每个 member.RoomID != 自己房间号的对手各起一个 Client，
// 把它们的事件转发到一个统一通道，再由调用方跟宿主 Client.Events() 合流
// 消费。这里绝不重新实现任何 WebSocket 逻辑。
//
// 事件的「来源标记」就是 Event.RoomID：每个对手连接都是独立的 Client，
// 天然只会给自己房间的事件盖上自己的房间号，PkLink 转发时原样带过去。
// 没有另外发明一个 IsOpponent/Source 字段——多发明一个字段，就多一份
// 跟 RoomID 不一致的风险，而且 RoomID 本身已经能在多人 PK 下分清「是
// 哪一个对手」，比一个二元的布尔值更精确。
//
// 「对面连接失败绝不能影响主房间」是设计前提：每个对手都是完全独立的
// Client 实例，有自己的 goroutine、自己的重连退避；PkLink 只转发事件，
// 从不把子连接的 error 冒泡给宿主，对手连不上就是「link.Events() 没有
// 事件」，宿主自己的连接和事件流照常运作。
type PkLink struct {
	host *Client

	mu    sync.Mutex
	round *pkRound // 当前进行中的 PK；没有 PK 时为 nil

	audMu    sync.Mutex
	mine     map[string]struct{} // 我方观众 uid 集合
	opposite map[string]struct{} // 对面观众 uid 集合（多个对手合并为一个集合，语义照抄原项目的二元划分）
}

// pkRound 是一场 PK 期间全部对手连接共享的状态。每次 Connect 都会创建
// 一个全新的 pkRound，Disconnect/异常退出后旧的 round 不会被复用——这样
// 「PK 正常结束」和「PK 异常结束（ctx 被外部取消）」可以走同一套清理
// 代码，不需要在两个地方各写一份、容易漏同步。
type pkRound struct {
	cancel context.CancelFunc
	events chan event.Event
	done   chan struct{} // 清理彻底完成（goroutine 全部退出 + hook 摘除 + 通道关闭）后关闭
}

// closedEventsPlaceholder 是没有进行中 PK 时 Events() 返回的占位通道：
// 已经关闭，任何 range/receive 都会立刻返回零值——比返回 nil 通道更安全
// （nil 通道上 receive 会永久阻塞），调用方不需要每次都先判空。
var closedEventsPlaceholder = func() chan event.Event {
	ch := make(chan event.Event)
	close(ch)
	return ch
}()

// newPkLink 创建一个绑定到 host 的 PkLink。host 只用来读取连接参数
// （拨号器、退避、心跳、日志）和共享的 api.Client，不复用 host 自己的
// WebSocket 连接——对手连接是完全独立的 Client 实例。
func newPkLink(host *Client) *PkLink {
	return &PkLink{
		host:     host,
		mine:     make(map[string]struct{}),
		opposite: make(map[string]struct{}),
	}
}

// Events 返回对手房间事件流；每条事件的 RoomID 就是它所属对手房间的
// 房间号，见类型注释里对「来源标记」的说明。
func (p *PkLink) Events() <-chan event.Event {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.round == nil {
		return closedEventsPlaceholder
	}
	return p.round.events
}

// Connect 为 members 里每一个非自己的房间各起一条弹幕连接。
//
// 只应该在「PK 接通」这一刻调用一次；如果上一场还没断开就再次调用，会
// 先把上一场断干净、等它完全清理完，再开始新的一场——不允许两场 PK 的
// 连接叠加，也不依赖调用方按规矩先调 Disconnect 再调 Connect。
//
// ctx 通常是宿主 Client.Run 自己的 ctx（或它的派生）：一旦这个 ctx 被
// 取消（不管是因为调用方主动收尾，还是宿主连接/进程整体退出），全部
// 对手连接都会跟着退出，不需要调用方额外做什么。
func (p *PkLink) Connect(ctx context.Context, members []event.PkMember) {
	p.Disconnect()

	opponents := filterOpponents(members, p.host.roomID)
	if len(opponents) == 0 {
		return
	}

	linkCtx, cancel := context.WithCancel(ctx)
	round := &pkRound{
		cancel: cancel,
		events: make(chan event.Event, eventBufferSize),
		done:   make(chan struct{}),
	}

	p.mu.Lock()
	p.round = round
	p.mu.Unlock()

	p.seedAudiences(linkCtx, opponents)
	p.host.setEventHook(p.observeMine)

	var wg sync.WaitGroup
	for _, m := range opponents {
		wg.Add(1)
		go func(m event.PkMember) {
			defer wg.Done()
			p.runOpponent(linkCtx, m, round.events)
		}(m)
	}

	// 收尾协程：不管 linkCtx 是被 Disconnect 显式取消，还是被调用方传入
	// 的父 ctx 间接取消（PK 异常结束、宿主 Run 退出走的都是这条路），
	// 都在这一个地方做最终清理——这是唯一能同时兜住「正常结束」之外
	// 全部退出路径的地方，Disconnect 本身只负责触发取消、然后等这里
	// 跑完。
	go func() {
		<-linkCtx.Done()
		wg.Wait() // 等全部对手的 Client.Run 真正返回，而不是刚发出取消信号

		p.host.setEventHook(nil)
		close(round.events)

		p.mu.Lock()
		if p.round == round {
			p.round = nil
		}
		p.mu.Unlock()

		close(round.done) // 必须最后关，Disconnect 靠它判断"已经断干净"
	}()
}

// Disconnect 断开当前这一场 PK 的全部对手连接，阻塞到全部清理真正完成
// （goroutine 退出、事件通道关闭、观众集合钩子摘除）才返回。
//
// 幂等：没有进行中的 PK、或者这场 PK 已经因为 ctx 被外部取消而自行清理
// 完毕，都是安全的空操作/快速返回——不要求调用方精确知道当前状态。
func (p *PkLink) Disconnect() {
	p.mu.Lock()
	round := p.round
	p.mu.Unlock()
	if round == nil {
		return
	}
	round.cancel()
	<-round.done
}

// runOpponent 是单个对手连接的完整生命周期：起一个绑定该对手房间的
// Client、把它的事件转发到 out、持续更新 oppositeAudience，直到 ctx
// 取消。
//
// child.Run 的返回值不会被抛给调用方——对手房间下播/连不上/被风控只会
// 让这一路自己按 Client 既有的退避策略反复重试，宿主房间完全不受影响；
// ctx 取消导致的返回是预期中的正常退出，不当错误处理。
func (p *PkLink) runOpponent(ctx context.Context, m event.PkMember, out chan<- event.Event) {
	child := NewClient(m.RoomID, p.host.api, p.childOptions()...)

	runDone := make(chan struct{})
	go func() {
		defer close(runDone)
		if err := child.Run(ctx); err != nil && ctx.Err() == nil {
			p.host.log.Warn("PK 对面连接异常结束", "room", m.RoomID, "err", err)
		}
	}()

	for ev := range child.Events() {
		p.trackOpposite(ev)
		select {
		case out <- ev:
		case <-ctx.Done():
		default:
			// 消费者跟不上时丢弃最新事件而非阻塞转发，跟 Client.handleMessage
			// 是同一套「宁可丢事件也不阻塞」的原则。
			p.host.log.Warn("PK 对面事件通道已满，丢弃事件", "room", m.RoomID, "type", ev.Type)
		}
	}
	<-runDone
}

// childOptions 让对手连接沿用宿主的连接参数（拨号器、心跳、退避、
// 日志）——行为应该跟宿主完全一致，测试里替换的假拨号地址、缩短的退避
// 时间也应该同样作用在对手连接上，不需要调用方重复配置一遍。
func (p *PkLink) childOptions() []ClientOption {
	h := p.host
	return []ClientOption{
		WithDialer(h.dialer),
		WithDialURLOverride(h.dialURLOverride),
		WithHeartbeatInterval(h.heartbeatInterval),
		WithBackoff(h.backoffMin, h.backoffMax),
		WithLogger(h.log),
	}
}

// filterOpponents 判断「谁是自己」只能靠房间号比对——绝不能用
// init_info/match_info，那两个字段的真实语义是发起方/被匹配方，跟
// 自己/对面是两回事，混用会在主播主动发起 PK 时把自己错认成对面
// （opponent_snapshot.go 踩过的同一个坑，这里照抄它的正确写法）。
func filterOpponents(members []event.PkMember, selfRoomID string) []event.PkMember {
	out := make([]event.PkMember, 0, len(members))
	for _, m := range members {
		if m.RoomID == selfRoomID {
			continue
		}
		out = append(out, m)
	}
	return out
}

// ---------- 观众集合 ----------

// seedAudiences 在对手连接建立前先播种两个观众集合，语义照抄原 C++
// connectPkRoom/getRoomCurrentAudiences（bili_liveservice.cpp:3278-3365）：
// 双方房间各拉一次「近期弹幕发送者」uid 当观众集合的近似值，再补上
// 自己主播、对面主播本人。
//
// 三个 HTTP 调用都是「有多少算多少」的降级，不让任何一个失败拖住整场
// PK 或让种子集合直接报错——PK 接通瞬间正是这类接口最容易超时/限流的
// 时候。用 host.opponentSnapshotBudget 兜底整体预算：这和
// FetchOpponentSnapshots 是同一个「PK 接通瞬间」场景，复用同一份预算
// 语义，不重新发明一个新的超时策略。
//
// 原 C++ 还有第三路种子来源——本地已缓存的历史弹幕发送者（roomDanmakus）
// 并入 myAudience。当前 Go 重写里还没有对应的本地弹幕历史缓存组件，
// 这里不为这一个功能造一个新的缓存子系统；等未来哪个组件持有这份历史
// 数据了，再把它接进来。
func (p *PkLink) seedAudiences(ctx context.Context, opponents []event.PkMember) {
	if sess := p.host.api.Session(); sess != nil && sess.UID != "" {
		p.addMine(sess.UID)
	}
	for _, m := range opponents {
		p.addOpposite(m.UID)
	}

	budget := p.host.opponentSnapshotBudget
	if budget <= 0 {
		budget = defaultOpponentSnapshotBudget
	}
	seedCtx, cancel := context.WithTimeout(ctx, budget)
	defer cancel()

	if uids, err := p.host.api.RoomRecentDanmakuUIDs(seedCtx, p.host.roomID); err != nil {
		p.host.log.Warn("播种己方观众集合失败，降级为仅有主播本人", "room", p.host.roomID, "err", err)
	} else {
		p.addMine(uids...)
	}

	for _, m := range opponents {
		uids, err := p.host.api.RoomRecentDanmakuUIDs(seedCtx, m.RoomID)
		if err != nil {
			p.host.log.Warn("播种对面观众集合失败，降级为仅有对面主播本人", "room", m.RoomID, "err", err)
			continue
		}
		p.addOpposite(uids...)
	}
}

// observeMine 挂在宿主 Client 上的事件钩子（见 client.go setEventHook），
// PK 期间每来一条本房间事件就同步喂一次，让 myAudience 跟原 C++ 语义
// 一样「在哪个房间说话/互动，就进哪个集合」。
func (p *PkLink) observeMine(ev event.Event) {
	if uid := uidOf(ev); uid != "" {
		p.addMine(uid)
	}
}

// trackOpposite 是对面房间那一侧的持续更新，在 runOpponent 转发事件时
// 同步调用。
func (p *PkLink) trackOpposite(ev event.Event) {
	if uid := uidOf(ev); uid != "" {
		p.addOpposite(uid)
	}
}

func (p *PkLink) addMine(uids ...string) {
	p.audMu.Lock()
	defer p.audMu.Unlock()
	for _, u := range uids {
		if u != "" {
			p.mine[u] = struct{}{}
		}
	}
}

func (p *PkLink) addOpposite(uids ...string) {
	p.audMu.Lock()
	defer p.audMu.Unlock()
	for _, u := range uids {
		if u != "" {
			p.opposite[u] = struct{}{}
		}
	}
}

// Audiences 返回两个观众集合的快照副本，供调用方（P6 的偷塔/串门播报）
// 只读查询。返回副本而不是内部 map 本身，避免调用方拿到的引用跟后续
// 更新产生数据竞争。
func (p *PkLink) Audiences() (mine, opposite map[string]struct{}) {
	p.audMu.Lock()
	defer p.audMu.Unlock()
	return cloneSet(p.mine), cloneSet(p.opposite)
}

func cloneSet(m map[string]struct{}) map[string]struct{} {
	out := make(map[string]struct{}, len(m))
	for k := range m {
		out[k] = struct{}{}
	}
	return out
}

// uidOf 从事件载荷里取「谁在互动」的 uid，覆盖带 User 字段的常见事件
// 类型。覆盖不到的类型（如 SuperChatDelete、RoomChange、Battle 本身）
// 本来就没有「单个用户在互动」这个概念，返回空字符串，调用方自然跳过。
func uidOf(ev event.Event) string {
	switch p := ev.Payload.(type) {
	case event.Danmaku:
		return p.User.UID
	case event.Gift:
		return p.User.UID
	case event.GiftCombo:
		return p.User.UID
	case event.GuardBuy:
		return p.User.UID
	case event.SuperChat:
		return p.User.UID
	case event.UserEnter:
		return p.User.UID
	case event.UserFollow:
		return p.User.UID
	case event.UserShare:
		return p.User.UID
	case event.UserLike:
		return p.User.UID
	case event.UserBlocked:
		return p.User.UID
	default:
		return ""
	}
}
