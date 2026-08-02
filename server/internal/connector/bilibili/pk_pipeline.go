package bilibili

import (
	"context"
	"encoding/json"
	"sync"
	"time"

	"github.com/MrZhongzq/MagicalDanmaku/server/internal/event"
)

// PKOpponentSnapshotSubCommand 标记一条 Go 自己合成的 Battle 事件：PK 接通
// 那一瞬间抓取的对面快照（人数/大航海）已经就绪。**不是 B 站真实下发的
// CMD 名**——event.Battle.SubCommand 的文档写着「原始 CMD 名，P0 只归一化
// 不解释」，这是唯一的例外。规则作者可以拿它当 when 条件精确定位「PK
// 接通的这一瞬间」，不必在 PK_BATTLE_* 系列几十次状态流转上都触发一遍
// 播报（PK_INFO/PK_BATTLE_PRE/PK_BATTLE_PROCESS/... 一场 PK 里会来很多次）。
const PKOpponentSnapshotSubCommand = "PK_OPPONENT_SNAPSHOT"

// pkBattleEndSubCommand 是 B 站下发的 PK 结束 CMD，battle.go 的
// battleCommands 里就有这一项。PKPipeline 用它触发 EndPK——不使用
// pk_basic.battle_type（视频/普通 PK 相关的字段，本项目从未解析过），
// PK_BATTLE_END 这个 CMD 名本身已经是最直接、最不需要解释的结束信号。
const pkBattleEndSubCommand = "PK_BATTLE_END"

// synthesizedRaw 是合成事件的占位 Raw。event.Event.Raw 的约定是「永不为
// nil」，但这条事件不是从真实报文解析出来的，没有原始字节可放，用一个
// 合法的空 JSON 对象占位，不违反约定又不假装有来源数据。
var synthesizedRaw = json.RawMessage(`{}`)

// PKPipeline 把已经就绪但一直没有生产调用点的几块能力
// （StartPK/EndPK/FetchOpponentSnapshots/ClassifyVisit）接进 Client 的
// 实时事件流。
//
// **为什么需要单独一层，而不是在消费 Client.Events() 的地方直接调用**：
// StartPK 本身不阻塞（只登记 + 起 goroutine），但 FetchOpponentSnapshots
// 有自己的 5s 预算、同步执行三个 HTTP 接口；如果调用方在消费
// Client.Events() 的同一个 goroutine 里顺手调用它，PK 接通瞬间——恰好是
// 弹幕礼物最密集的时刻——主房间事件会被晾在只有 256 缓冲的 c.events
// 里，超出缓冲就被 handleMessage 的 default 分支直接丢弃。PKPipeline
// 把「读 Client.Events()」和「触发网络调用」拆到不同 goroutine：前者
// （loop）只做零 I/O 的快速判断（pk_id 去重、ClassifyVisit 的 map
// 查询），后者（startPK/EndPK）才做真正阻塞的网络调用。
type PKPipeline struct {
	client *Client
	out    chan event.Event

	// wg 跟踪全部还在跑的 startPK goroutine——loop 退出（Client.Events()
	// 关闭）之后必须先等它们都返回才能 close(out)，否则一个还在
	// FetchOpponentSnapshots/消费 link.Events() 的 startPK goroutine
	// 迟到的 forward() 调用会往已关闭的通道发送，直接 panic（测试
	// TestPKPipelineDoesNotBlockMainEventsDuringSnapshotFetch 复现过
	// 这个真实存在的关闭时序竞争）。Add 只在 loop 自己的 goroutine里
	// （handleBattle）调用，且都发生在 loop 退出、走到 Wait 之前，满足
	// sync.WaitGroup「Add 必须先于对应的 Wait」这条使用约束。
	wg sync.WaitGroup

	mu         sync.Mutex
	activePkID string  // 当前已触发 StartPK 的 pk_id，用于按 pk_id 去重
	link       *PkLink // 当前 PK 的对面连接管理器，供 ClassifyVisit 判方向 A 用；无 PK 时为 nil
}

// NewPKPipeline 创建一个绑定到 c 的 PKPipeline。
func NewPKPipeline(c *Client) *PKPipeline {
	return &PKPipeline{
		client: c,
		out:    make(chan event.Event, eventBufferSize),
	}
}

// Run 启动编排 goroutine，返回合流后的事件通道：宿主自己的事件原样
// 转发，加上 PK 期间产出的串门信号事件（TypeVisitFromOpponent/
// TypeVisitToOpponent）与对面快照就绪时合成的 Battle 事件
// （SubCommand=PKOpponentSnapshotSubCommand）。
//
// ctx 应该跟传给 c.Run 的是同一个（或它的派生）：不需要另外监听 ctx
// 取消，c.Events() 在宿主 Run 退出时会被关闭，这里的 loop 循环自然
// 结束、close(out)，调用方按「range 到通道关闭」处理收尾即可，跟今天
// 直接消费 c.Events() 的写法完全一致，只是换了个通道源。
func (pl *PKPipeline) Run(ctx context.Context) <-chan event.Event {
	go pl.loop(ctx)
	return pl.out
}

// loop 是唯一读 Client.Events() 的地方，只做零 I/O 的快速判断，任何可能
// 阻塞的网络调用都必须 go 出去，不能同步跑在这里——见类型注释。
func (pl *PKPipeline) loop(ctx context.Context) {
	defer func() {
		// 先等全部 startPK goroutine 真正退出，再关闭 out——见 wg 字段
		// 注释，这是防止「迟到的 forward() 写已关闭通道」panic 的关键
		// 顺序，不能颠倒。
		pl.wg.Wait()
		close(pl.out)
	}()
	for ev := range pl.client.Events() {
		pl.forward(ev)

		if b, ok := ev.Payload.(event.Battle); ok {
			pl.handleBattle(ctx, b)
		}

		// ev 来自 Client.Events()，RoomID 恒等于宿主自己的房间号，
		// ClassifyVisit 内部据此只会走方向 A（对面的人跑来我方）。
		// 方向 B 在 startPK 里消费 link.Events() 时单独判。
		if link := pl.currentLink(); link != nil {
			if visitEv, ok := link.ClassifyVisit(ev); ok {
				pl.forward(visitEv)
			}
		}
	}
}

func (pl *PKPipeline) currentLink() *PkLink {
	pl.mu.Lock()
	defer pl.mu.Unlock()
	return pl.link
}

// forward 把事件塞进合流通道，通道满了就丢弃——跟 Client.handleMessage/
// PkLink.runOpponent 是同一套「宁可丢事件也不阻塞」的策略。这里的
// 消费方通常是规则引擎 + SSE 扇出，一旦它一时跟不上，绝不能反过来拖住
// 这条编排 goroutine，否则会间接拖住上面对 Client.Events() 的消费。
func (pl *PKPipeline) forward(ev event.Event) {
	select {
	case pl.out <- ev:
	default:
		pl.client.log.Warn("PK 编排事件通道已满，丢弃事件", "room", pl.client.roomID, "type", ev.Type)
	}
}

// handleBattle 检测「PK 接通」与「PK 结束」两个时刻，触发对应的编排
// 动作。**两个分支都只做零 I/O 的判断 + 起一个新 goroutine**，真正的
// 网络调用（StartPK/FetchOpponentSnapshots/EndPK）全部丢给那个新
// goroutine，绝不同步跑在这里——这是 Important-3（上游 Task 6a 因此被
// 打回过一次）反复强调的红线。
func (pl *PKPipeline) handleBattle(ctx context.Context, b event.Battle) {
	// PK_INFO 是唯一携带 Members 明细的 CMD（见 cmdmap/battle.go 的
	// mapPkInfo），只有它才可能是「PK 接通」这一刻；其余 PK_BATTLE_* CMD
	// 的 Members 恒为空，天然被这个条件挡在外面，不需要额外按 SubCommand
	// 判断是不是 PK_INFO。
	if len(b.Members) > 0 && b.PkID != "" {
		pl.mu.Lock()
		isNew := pl.activePkID != b.PkID
		if isNew {
			pl.activePkID = b.PkID
		}
		pl.mu.Unlock()

		if isNew {
			pl.wg.Add(1)
			go pl.startPK(ctx, b)
		}
	}

	if b.SubCommand == pkBattleEndSubCommand {
		pl.mu.Lock()
		hadActive := pl.activePkID != ""
		pl.activePkID = ""
		pl.link = nil
		pl.mu.Unlock()

		if hadActive {
			// c.EndPK() 内部按 c.pkLink（不是我们这里的本地缓存）操作，
			// 即使 startPK 那个 goroutine 还没来得及把 link 登记到
			// pl.link，也不影响它找到真正需要断开的连接。
			go pl.client.EndPK()
		}
	}
}

// startPK 是「PK 接通」触发的编排动作，独立 goroutine 里跑，不阻塞
// loop 对 Client.Events() 的消费。StartPK 与 FetchOpponentSnapshots
// 并发发起——两者各自的预算相互独立，并发执行只取两者耗时的较大值，
// 不会叠成简报警告过的「串起来最坏 10s」。
func (pl *PKPipeline) startPK(ctx context.Context, b event.Battle) {
	defer pl.wg.Done()

	var wg sync.WaitGroup
	var link *PkLink
	var snapshots []OpponentSnapshot

	wg.Add(2)
	go func() {
		defer wg.Done()
		link = pl.client.StartPK(ctx, b.Members)
	}()
	go func() {
		defer wg.Done()
		snapshots = pl.client.FetchOpponentSnapshots(ctx, b.Members)
	}()
	wg.Wait()

	pl.mu.Lock()
	stillCurrent := pl.activePkID == b.PkID
	if stillCurrent {
		pl.link = link
	}
	pl.mu.Unlock()

	if !stillCurrent {
		// 这场 PK 在 StartPK/FetchOpponentSnapshots 跑的这段时间里已经
		// 结束（见到了 PK_BATTLE_END）或被新一场 PK_INFO 顶替——不管
		// 哪一种，c.StartPK/c.EndPK 内部的 endPKLocked 都已经把这个 link
		// 断干净了（Client 层面的互斥保证，opponent_link_test.go 的
		// TestConcurrentStartPKDoesNotOrphanEarlierLink 已经覆盖过这条
		// 保证）。这里只是不要把一个已经过期、不再对应「当前」的 link
		// 挂上去当作分类依据，也不要把这份快照当成「当前」播出去——
		// 直接放弃，不需要再做任何清理。
		return
	}

	pl.forward(pl.buildSnapshotEvent(b, snapshots))

	// 消费对面房间的事件，跑串门判定方向 B（我方观众跑去对面）。这个
	// range 会在这场 PK 结束（EndPK/ctx 取消触发 finishRound 关闭
	// round.events）时自然退出，不需要额外的取消信号。
	for ev := range link.Events() {
		if visitEv, ok := link.ClassifyVisit(ev); ok {
			pl.forward(visitEv)
		}
	}
}

// buildSnapshotEvent 把 FetchOpponentSnapshots 的结果按 RoomID 合回
// b.Members，合成一条新的 Battle 事件。自己那一项、以及快照没抓到的
// 对手，三个指针字段保持 nil（未知），不会被误填成 0——「拿不到」和
// 「真的是 0」必须在数据结构上可区分，这里原样透传 OpponentSnapshot
// 已经做好的这份区分，不在这一步引入新的塌缩。
func (pl *PKPipeline) buildSnapshotEvent(b event.Battle, snapshots []OpponentSnapshot) event.Event {
	byRoom := make(map[string]OpponentSnapshot, len(snapshots))
	for _, s := range snapshots {
		byRoom[s.RoomID] = s
	}

	members := make([]event.PkMember, len(b.Members))
	copy(members, b.Members)
	for i, m := range members {
		if snap, ok := byRoom[m.RoomID]; ok {
			members[i].Online = snap.Online
			members[i].GuardTotal = snap.GuardTotal
			members[i].GuardOnline = snap.GuardOnline
		}
	}

	now := time.Now()
	return event.Event{
		ID:         event.NewID(),
		RoomID:     pl.client.roomID,
		Platform:   event.PlatformBilibili,
		Type:       event.TypeBattle,
		Timestamp:  now,
		ReceivedAt: now,
		Payload: event.Battle{
			SubCommand: PKOpponentSnapshotSubCommand,
			PkID:       b.PkID,
			Members:    members,
			StartTime:  b.StartTime,
			EndTime:    b.EndTime,
		},
		Raw: synthesizedRaw,
	}
}
