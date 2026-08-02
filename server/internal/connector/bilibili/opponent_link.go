package bilibili

import (
	"context"
	"sync"
	"time"

	"github.com/MrZhongzq/MagicalDanmaku/server/internal/event"
)

// pkTeardownGraceLimit 是 disconnect 兜底等待清理完成的硬上限。正常情况
// 下清理在毫秒级完成；给上限是为了防一个慢角落：子连接可能卡在
// authenticate()（client.go）里等对面服务器的认证回复——那段代码用的是
// 固定 authTimeout 读超时，不接受 ctx，ctx 取消对它不生效，只能等读
// 超时自己触发。disconnect 常常同步跑在宿主 Client.Run 的 defer 里，
// 不能因为一个（可能异常/被风控）对手服务器迟迟不回包，就把宿主整个
// 退出流程拖住到无限久——超过这个上限就记录警告后放弃等待，让调用方
// 尽快拿回控制权；子连接自己的 goroutine 仍然会在各自的读超时触发后
// 正常退出，不是真正意义上的无限泄漏，只是不再同步阻塞在这里等它。
const pkTeardownGraceLimit = authTimeout + 5*time.Second

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
	mine     map[string]struct{}            // 我方观众 uid 集合
	opposite map[string]map[string]struct{} // 对面观众 uid 集合，按对手房间号分开

	// opposite 为什么按房间分开而不是拍平成一个集合：多人 PK 下每个
	// 对手是不同的房间，播种（seedAudiences）时是逐个对手房间调
	// ajax/msg 拿到的，这份「归属哪个对手」的信息只有在这一刻才完整；
	// 一旦拍平合并，事后没法从合并结果里反推出某个 uid 到底是哪个
	// 对手房间的观众，要重建就得重新打一遍接口。按房间分开保留成本
	// 几乎为零，实时更新那一路（trackOpposite）本来就知道事件来自
	// 哪个对手连接，也天然能对上号。
}

// pkRound 是一场 PK 期间全部对手连接共享的状态。每次 connect 都会创建
// 一个全新的 pkRound，disconnect/异常退出后旧的 round 不会被复用——这样
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
		opposite: make(map[string]map[string]struct{}),
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

// connect 为 members 里每一个非自己的房间各起一条弹幕连接。
//
// 只应该在「PK 接通」这一刻调用一次；如果上一场还没断开就再次调用，会
// 先把上一场断干净、等它完全清理完，再开始新的一场——不允许两场 PK 的
// 连接叠加，也不依赖调用方按规矩先调 disconnect 再调 connect。
//
// ctx 通常是宿主 Client.Run 自己的 ctx（或它的派生）：一旦这个 ctx 被
// 取消（不管是因为调用方主动收尾，还是宿主连接/进程整体退出），全部
// 对手连接都会跟着退出，不需要调用方额外做什么。
func (p *PkLink) connect(ctx context.Context, members []event.PkMember) {
	p.disconnect()

	opponents := filterOpponents(members, p.host.roomID)
	if len(opponents) == 0 {
		return
	}
	selfUID := selfMemberUID(members, p.host.roomID)

	linkCtx, cancel := context.WithCancel(ctx)
	round := &pkRound{
		cancel: cancel,
		events: make(chan event.Event, eventBufferSize),
		done:   make(chan struct{}),
	}

	p.mu.Lock()
	p.round = round
	p.mu.Unlock()

	// 必须在起任何连接、做任何阻塞工作（下面的播种 HTTP 调用）之前，
	// 就把这一轮登记到宿主 Client 上——宿主 Run 的 defer 兜底清理靠
	// host.pkLink 才能找到并断开这一轮。审查曾指出：如果登记发生在
	// seedAudiences 那 N+1 次 HTTP 调用（预算最长 opponentSnapshotBudget）
	// 之后，宿主若恰好在这个窗口内退出，defer 读到的 pkLink 还是 nil，
	// 这一轮连接会永久变成孤儿——探针复现过。registerPKLink 跟 Run 的
	// defer 共用同一把锁做「读 closed + 写 pkLink」，不管两个 goroutine
	// 谁先谁后都不会漏；如果它告知宿主已经关闭，说明这次 connect 本身
	// 就是「宿主已经退出后才发起」，直接自行收尾，不建立任何真实连接。
	if p.host.registerPKLink(p) {
		cancel()
		p.finishRound(round)
		return
	}

	// owner 用 round 自己的指针当归属令牌——finishRound 清钩子时会
	// 拿它跟 host 当前记着的 owner 比对，防止一次迟到的清理（比如
	// disconnect 因 pkTeardownGraceLimit 提前返回）把新一轮已经装上的
	// 钩子误摘掉（N-3 修复，见 client.go clearEventHookIfOwner）。
	p.host.setEventHook(round, p.observeMine)

	var wg sync.WaitGroup

	// 播种观众集合是同步的 N+1 次 HTTP 调用（预算最长
	// opponentSnapshotBudget，最坏几秒钟）。StartPK 的文档化调用方就是
	// 消费 Client.Events() 的那个事件循环，如果在这里同步等它跑完，
	// 主房间的事件会在 PK 接通、弹幕礼物最密集的那一刻被晾在 256
	// 缓冲的 c.events 里，超出缓冲就被 handleMessage 的 default 分支
	// 直接丢弃。观众集合本来就是「有多少算多少」的降级语义，不需要在
	// connect 返回前就绪，放进独立 goroutine、并入 wg 一起等它退出。
	wg.Add(1)
	go func() {
		defer wg.Done()
		p.seedAudiences(linkCtx, selfUID, opponents)
	}()

	for _, m := range opponents {
		wg.Add(1)
		go func(m event.PkMember) {
			defer wg.Done()
			p.runOpponent(linkCtx, m, round.events)
		}(m)
	}

	// 收尾协程：不管 linkCtx 是被 disconnect 显式取消，还是被调用方传入
	// 的父 ctx 间接取消（PK 异常结束、宿主 Run 退出走的都是这条路），
	// 都在这一个地方做最终清理——这是唯一能同时兜住「正常结束」之外
	// 全部退出路径的地方，disconnect 本身只负责触发取消、然后等这里
	// 跑完。
	go func() {
		<-linkCtx.Done()
		wg.Wait() // 等全部对手的 Client.Run、播种 goroutine 真正退出
		p.finishRound(round)
	}()
}

// finishRound 是一场 PK 的最终清理：摘观众集合钩子、关闭事件通道、
// 把 p.round 清空（如果它还指向这一轮——避免清掉后面新一轮已经装上的
// round）、最后关闭 done 通知 disconnect 已经断干净。不管是正常收尾
// 途中的收尾协程调用它，还是 connect 发现宿主已关闭时直接同步调用它，
// 都走这一份代码，不重复写两遍容易漏同步的清理逻辑。
//
// clearEventHookIfOwner(round) 而不是无条件摘钩子：如果 disconnect
// 因为 pkTeardownGraceLimit 提前放弃等待、随后新一轮已经装上了自己的
// 钩子，这次迟到的清理不能把新一轮的钩子也摘了。
func (p *PkLink) finishRound(round *pkRound) {
	p.host.clearEventHookIfOwner(round)
	close(round.events)

	p.mu.Lock()
	if p.round == round {
		p.round = nil
	}
	p.mu.Unlock()

	close(round.done) // 必须最后关，disconnect 靠它判断"已经断干净"
}

// disconnect 断开当前这一场 PK 的全部对手连接，等待清理真正完成
// （goroutine 退出、事件通道关闭、观众集合钩子摘除）才返回，但不会
// 无限等下去——见 pkTeardownGraceLimit 的注释：子连接可能卡在不接受
// ctx 的 authenticate() 读超时里，超过上限就放弃等待、记录警告后返回，
// 不能让一个慢/异常的对手连接把调用方（常常是宿主 Run 的 defer）
// 拖住无限久。
//
// 幂等：没有进行中的 PK、或者这场 PK 已经因为 ctx 被外部取消而自行清理
// 完毕，都是安全的空操作/快速返回——不要求调用方精确知道当前状态。
func (p *PkLink) disconnect() {
	p.mu.Lock()
	round := p.round
	p.mu.Unlock()
	if round == nil {
		return
	}
	round.cancel()
	select {
	case <-round.done:
	case <-time.After(pkTeardownGraceLimit):
		p.host.log.Warn("PK 连接清理超过等待上限，放弃同步等待，交由各自的读超时自行收尾",
			"limit", pkTeardownGraceLimit)
	}
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
		p.trackOpposite(m.RoomID, ev)
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

// selfMemberUID 从 members 里找出「自己」那一项的 UID——PK_INFO 里这个
// 值就是本房间主播的 uid，是 myAudience 该播种的值（简报原文「自己的
// upUid」）。
//
// 绝不能退回去用 sess.UID（登录账号）：工具很可能是助播/小号账号登录、
// 给别的主播的房间跑，这种情况下登录账号和本房间主播是两个人，混用
// 会把错误的人算进「自己」的观众集合——这不是一个边界风险，是一个
// 确定会用错的字段（审查发现的真实缺陷）。找不到自己（PK_INFO 数据
// 异常/不完整，理论上不应该发生）时返回空字符串，播种就只播对手那
// 一路，而不是悄悄退化成一个明知是错的值。
func selfMemberUID(members []event.PkMember, selfRoomID string) string {
	for _, m := range members {
		if m.RoomID == selfRoomID {
			return m.UID
		}
	}
	return ""
}

// ---------- 观众集合 ----------

// seedAudiences 播种两个观众集合，语义照抄原 C++ connectPkRoom/
// getRoomCurrentAudiences（bili_liveservice.cpp:3278-3365）：双方房间
// 各拉一次「近期弹幕发送者」uid 当观众集合的近似值，再补上自己主播、
// 对面主播本人。
//
// 这是在 connect 里的独立 goroutine 中调用的（Important-3 修复），不是
// 同步跑在对手连接建立之前——种子集合本来就是「有多少算多少」的降级
// 语义，不需要在对手连接真正建立前就绪，没必要为了这个去阻塞调用方。
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
func (p *PkLink) seedAudiences(ctx context.Context, selfUID string, opponents []event.PkMember) {
	if selfUID != "" {
		p.addMine(selfUID)
	}
	for _, m := range opponents {
		p.addOppositeRoom(m.RoomID, m.UID)
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
		p.addOppositeRoom(m.RoomID, uids...)
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
// 同步调用；roomID 就是这条事件所属的那个对手房间号，runOpponent 本来
// 就知道，天然能对上号，不需要从事件内容里反推。
func (p *PkLink) trackOpposite(roomID string, ev event.Event) {
	if uid := uidOf(ev); uid != "" {
		p.addOppositeRoom(roomID, uid)
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

func (p *PkLink) addOppositeRoom(roomID string, uids ...string) {
	p.audMu.Lock()
	defer p.audMu.Unlock()
	set := p.opposite[roomID]
	if set == nil {
		set = make(map[string]struct{})
		p.opposite[roomID] = set
	}
	for _, u := range uids {
		if u != "" {
			set[u] = struct{}{}
		}
	}
}

// Audiences 返回两个观众集合的快照副本，供调用方（P6 的偷塔/串门播报）
// 只读查询。mine 是自己房间的观众 uid 集合；opposite 按对手房间号分开
// （见 PkLink 类型注释里对这个结构的说明）。返回的都是副本，不是内部
// map 本身，避免调用方拿到的引用跟后续更新产生数据竞争。
func (p *PkLink) Audiences() (mine map[string]struct{}, opposite map[string]map[string]struct{}) {
	p.audMu.Lock()
	defer p.audMu.Unlock()

	oppCopy := make(map[string]map[string]struct{}, len(p.opposite))
	for room, set := range p.opposite {
		oppCopy[room] = cloneSet(set)
	}
	return cloneSet(p.mine), oppCopy
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
