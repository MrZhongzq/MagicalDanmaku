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
	// 关闭）之后本该等它们都返回才 close(out)，否则一个还没退出的
	// startPK goroutine 迟到的 forward() 调用会往已关闭的通道发送，直接
	// panic（测试 TestPKPipelineDoesNotBlockMainEventsDuringSnapshotFetch
	// 复现过这个真实存在的关闭时序竞争）。Add 只在 loop 自己的
	// goroutine 里（handleBattle）调用，且都发生在 loop 退出、走到
	// shutdown 之前，满足 sync.WaitGroup「Add 必须先于对应的 Wait」这条
	// 使用约束。
	//
	// 【复审 Important-2 订正】"先等 wg 归零再 close" 这句话本身是有
	// 漏洞的——一个 startPK goroutine 可能卡在
	// `for ev := range link.Events()` 里，而这个通道只在
	// PkLink.finishRound 时关闭，finishRound 依赖 disconnect() 的清理
	// goroutine，disconnect() 本身受 pkTeardownGraceLimit（15s）兜底，
	// 但 wg.Wait() 完全绕开了这个上限——如果 disconnect 因为已知遗留
	// （conn.WriteMessage 没有 SetWriteDeadline）卡住不止 15s，
	// wg.Wait() 会无限等下去，宿主的优雅退出流程被整个拖死。
	// shutdownGraceLimit + closed/closeMu 就是补这个洞：等待有上限，
	// 超时后强制关闭 out，且 forward() 从此变成安全的 no-op（不会再
	// 往已关闭的通道发送），两个目标不再互斥。
	wg sync.WaitGroup

	// shutdownGraceLimit 是 loop 退出后等待全部 startPK goroutine 收尾
	// 的上限，语义与 opponent_link.go 的 pkTeardownGraceLimit 完全一致
	// （防止一个卡住的连接把宿主拖到无限久）。默认等于
	// pkTeardownGraceLimit，测试可以调小它，避免真的等 15s。
	shutdownGraceLimit time.Duration

	// closeMu/closed 保护 out 的关闭：forward() 在 RLock 下检查 closed，
	// shutdown() 在 Lock 下设置 closed 并 close(out)——两者互斥，
	// 保证不会有 forward() 在 close(out) 之后仍然往 out 发送。
	closeMu sync.RWMutex
	closed  bool

	mu            sync.Mutex
	activePkID    string  // 当前已触发 StartPK 的 pk_id，用于按 pk_id 去重
	lastEndedPkID string  // 最近一次触发过 EndPK 的 pk_id，见 handleBattle 上方的注释
	link          *PkLink // 当前 PK 的对面连接管理器，供 ClassifyVisit 判方向 A 用；无 PK 时为 nil

	// endTimeFallbackGrace 是 watchEndTimeFallback 的可覆盖预算，默认
	// 等于 pkEndTimeFallbackGrace（30s）。字段化而不是直接用包级常量，
	// 跟 shutdownGraceLimit 是同一个理由：测试需要把 30s 缩短到毫秒级，
	// 不然没法在合理时间内验证「超时兜底真的会触发」。
	endTimeFallbackGrace time.Duration
}

// NewPKPipeline 创建一个绑定到 c 的 PKPipeline。
func NewPKPipeline(c *Client) *PKPipeline {
	return &PKPipeline{
		client:               c,
		out:                  make(chan event.Event, eventBufferSize),
		shutdownGraceLimit:   pkTeardownGraceLimit,
		endTimeFallbackGrace: pkEndTimeFallbackGrace,
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
	defer pl.shutdown()
	for ev := range pl.client.Events() {
		// ev 来自 Client.Events()，RoomID 恒等于宿主自己的房间号，
		// ClassifyVisit 内部据此只会走方向 A（对面的人跑来我方）。
		// 方向 B 在 startPK 里消费 link.Events() 时单独判。
		visitEv, isVisit := event.Event{}, false
		if link := pl.currentLink(); link != nil {
			visitEv, isVisit = link.ClassifyVisit(ev)
		}

		// 【终审 Critical-1 第二部分】一个人「进场」这个动作只应该被
		// 欢迎一次，不该同时收到「内置/进房欢迎」（on: user_enter）与
		// 「内置/PK串门欢迎」（on: pk_visit_from_opponent）各一条——这两条
		// 规则各自独立处理各自的事件，spec.Rule.Suppress 救不了：它是
		// 单次 Engine.Handle 内的局部状态，这里是两条独立事件、两次
		// Handle 调用（见 visit.go 顶部对这个场景的说明）。
		//
		// 选择「只转发串门欢迎、压下这条原始 UserEnter」而不是「两条都
		// 发」或「文案上打太极」：PkPanel.vue 的界面文案原话是「对面观众
		// 串门时用单独欢迎语（与常规进房欢迎区分）」——用户看到的承诺是
		// 只有一条欢迎语，不是「同一次进场收到两条不同语气的欢迎」。
		//
		// 权衡（刻意的，不是遗漏）：这条被压下的 UserEnter 不会进
		// activity_logs（logging/sink.go 的 loggedEventTypes 白名单只认
		// TypeUserEnter，串门信号类型不在表里），这一次「对面观众进场」
		// 的业务日志行会缺失。范围很窄——只影响「PK 期间、这个人这一轮
		// 唯一一次被判定为串门来客的那一条 UserEnter」（下一次同一个人
		// 再进场，welcomedFromOpponent 已经记过，ClassifyVisit 不会再
		// 命中，UserEnter 正常转发正常入库）；这个人在 PK 期间如果还有
		// 弹幕/送礼等其它互动，那些事件走各自的类型正常记录，不受影响。
		// 换来的是不会给直播间刷两条欢迎语，符合界面文案的承诺。
		suppressRawUserEnter := isVisit && ev.Type == event.TypeUserEnter
		if suppressRawUserEnter {
			pl.forward(visitEv)
		} else {
			pl.forward(ev)
			if isVisit {
				pl.forward(visitEv)
			}
		}

		if b, ok := ev.Payload.(event.Battle); ok {
			pl.handleBattle(ctx, b)
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
//
// closeMu 的读锁保护「检查 closed + 发送」这一整段：与 shutdown() 的
// 写锁互斥，保证一旦 shutdown() 关闭了 out，此后任何一次 forward()
// 调用都会在 select 之前就看到 closed=true 并直接返回，绝不会执行到
// `pl.out <- ev`——这是 Important-2 复审指出的坑（先前的实现允许一个
// 卡住的 startPK goroutine 无限期拖住 shutdown，而如果换成简单加超时
// 又会在超时后 close 时撞上这个迟到的发送、panic）。
func (pl *PKPipeline) forward(ev event.Event) {
	pl.closeMu.RLock()
	defer pl.closeMu.RUnlock()
	if pl.closed {
		return
	}
	select {
	case pl.out <- ev:
	default:
		pl.client.log.Warn("PK 编排事件通道已满，丢弃事件", "room", pl.client.roomID, "type", ev.Type)
	}
}

// shutdown 是 loop 退出后的收尾：等全部 startPK goroutine 收尾，但不会
// 无限等——超过 shutdownGraceLimit 就放弃等待、记录警告，直接关闭
// out。兜的是跟 PkLink.disconnect() 对 pkTeardownGraceLimit 同一类
// 风险（一个卡住的对面连接，根因是 conn.WriteMessage 从未设过
// SetWriteDeadline），补的是 wg.Wait() 本身没有上限这个洞——但**两个
// 15s 窗口是并列关系，不是嵌套关系，语义不完全对称**（第二轮复审
// Minor-B 订正）：`startPK` 在真正卡住的场景下会先卡在
// `link.Events()`（等 `disconnect()` 的收尾），而 `disconnect()` 自己
// 那 15s 走完、真的把 `round.done` 关掉之后，`wg.Wait()` 才会解除——
// 也就是说合法的最坏情况下，`disconnect()` 用满自己的 15s 才让
// `startPK` 退出，`shutdown()` 这边的计时几乎是从同一时刻起算的，
// 两个窗口几乎同时到期，`shutdown()` 的 15s 很可能也几乎被用满，
// 从而打出一条「超过等待上限」的**误报**警告（此时 `startPK` 其实
// 刚刚正常收尾，不是真的卡死）。这条警告本身不代表功能损坏——超时
// 路径无论是不是误报都只是"放弃同步等待、转为 no-op 安全关闭"，不会
// panic 也不会漏数据——只是排查时不能把这条日志直接当成"真的卡住了"
// 的证据，需要结合有没有紧跟着收到真实的连接异常一起看。
//
// 用 closeMu 的写锁 + closed 标志保证「设置 closed、关闭 out」这个组合
// 操作与 forward() 的「检查 closed、发送」互斥，不管超时与否，
// close(out) 之后不会再有任何一次 forward() 执行到发送语句——
// 这正是不能简单给 wg.Wait() 加超时的原因：那样超时后 close 依然会跟
// 一个不知道已经超时、还在往 out 发送的 forward() 撞车。
//
// 【已知遗留，复审 Minor-C，可忽略】超时分支放弃等待之后，上面起的
// `go func(){ pl.wg.Wait(); close(done) }()` 并不会跟着退出，会一直
// 活到 wg 真正归零那一刻才结束——理论上是一次 goroutine 泄漏，但每个
// PKPipeline 生命周期内至多发生一次，且只出现在宿主退出这条路径上，
// 进程本身很快也会退出，代价可以忽略，不值得为它引入额外的取消机制。
func (pl *PKPipeline) shutdown() {
	done := make(chan struct{})
	go func() {
		pl.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(pl.shutdownGraceLimit):
		pl.client.log.Warn("PK 编排收尾超过等待上限，放弃同步等待，交由各自的连接自行收尾"+
			"（注意：这条警告可能是与 pkTeardownGraceLimit 同时到期导致的误报，不一定代表真的卡死）",
			"room", pl.client.roomID, "limit", pl.shutdownGraceLimit)
	}

	pl.closeMu.Lock()
	pl.closed = true
	close(pl.out)
	pl.closeMu.Unlock()
}

// handleBattle 检测「PK 接通」与「PK 结束」两个时刻，触发对应的编排
// 动作。**两个分支都只做零 I/O 的判断 + 起一个新 goroutine**，真正的
// 网络调用（StartPK/FetchOpponentSnapshots/EndPK）全部丢给那个新
// goroutine，绝不同步跑在这里——这是 Important-3（上游 Task 6a 因此被
// 打回过一次）反复强调的红线。
//
// **lastEndedPkID 是复审建议的低成本保险，不是已确认问题的修复**：
// B 站是否会在同一场 PK 结束（PK_BATTLE_END）之后，为惩罚阶段/长连接
// 重连等场景重推同一个 pk_id 的 PK_INFO——没有真实样本可查，无法确认，
// 复审明确说了「不要凭猜去改」判定逻辑本身。这里只加一道几乎零成本的
// 保险：记住最近一次真正结束（触发过 EndPK）的 pk_id，如果紧接着又
// 收到同一个 pk_id 的 PK_INFO，不重新触发 StartPK/二次快照播报——B 站
// 的 pk_id 是这一场 PK 唯一的会话标识，不该被两场不同的 PK 复用，所以
// 这道保险不会误伤真正的新 PK。只记一个值（不是一整份历史），至今唯一
// 已知的重复来源就是「同一个 pk_id 在结束后又收到一次」，没有理由为
// 假设中的其它模式预先造一个更复杂的结构。
func (pl *PKPipeline) handleBattle(ctx context.Context, b event.Battle) {
	// PK_INFO 是唯一携带 Members 明细的 CMD（见 cmdmap/battle.go 的
	// mapPkInfo），只有它才可能是「PK 接通」这一刻；其余 PK_BATTLE_* CMD
	// 的 Members 恒为空，天然被这个条件挡在外面，不需要额外按 SubCommand
	// 判断是不是 PK_INFO。
	if len(b.Members) > 0 && b.PkID != "" {
		pl.mu.Lock()
		alreadyEnded := b.PkID == pl.lastEndedPkID
		isNew := !alreadyEnded && pl.activePkID != b.PkID
		if isNew {
			pl.activePkID = b.PkID
		}
		pl.mu.Unlock()

		if isNew {
			pl.wg.Add(1)
			go pl.startPK(ctx, b)

			// 【终审 Important-3】PK_BATTLE_END 超时兜底：只在这里、
			// 判定为「新一场 PK」时启动一次，与 startPK 用同一个 isNew
			// 门槛，保证一场 PK 至多一个兜底计时器，不会重复触发也不会
			// 误伤下一场新 PK。b.EndTime 为 0 时（理论上 PK_INFO 不应该
			// 缺这个字段，但协议层从不假设报文一定完整）没有依据可算
			// 截止时间，跳过，不是「不兜底」而是「没有数据兜底」。
			if b.EndTime != 0 {
				go pl.watchEndTimeFallback(ctx, b)
			}
		}
	}

	if b.SubCommand == pkBattleEndSubCommand {
		// 真实的 PK_BATTLE_END 报文不带 pk_id 可用——mapBattle（battle.go）
		// 只归一化 SubCommand，从不解析 pk_basic，只有 PK_INFO 走的
		// mapPkInfo 才解析；requirePkID 传空字符串，表示「不要求匹配，
		// 直接结束当前活跃的这一场」，语义与去掉这次重构之前的旧代码
		// （`hadActive := pl.activePkID != ""`）完全一致。
		pl.endActivePK("")
	}
}

// endActivePK 是「结束当前这场 PK」的编排动作本身：清掉去重状态、清掉
// 供 ClassifyVisit 用的 link 引用，真正断开对面连接的 c.EndPK() 丢进
// 独立 goroutine（网络/等待操作，不能同步跑在调用方所在的 goroutine
// 里）。CMD 驱动的正常结束（handleBattle 的 pkBattleEndSubCommand 分支）
// 与超时兜底（watchEndTimeFallback）共用这一份逻辑，不重复写两遍容易
// 漏同步。
//
// requirePkID 为空字符串时无条件结束当前活跃的这一场——CMD 驱动的正常
// 结束路径用这个，因为真实的 PK_BATTLE_END 报文压根不携带 pk_id（见
// 上面调用处的说明），没有办法要求匹配，也没必要：能收到这条 CMD 就
// 说明这一场确实结束了。requirePkID 非空时，只有它与 pl.activePkID
// 相等才会真的触发——超时兜底路径（watchEndTimeFallback）传入它启动时
// 记住的 pk_id，如果这场 PK 已经被真正的 PK_BATTLE_END 结束、或被一场
// 新 PK 顶替，pl.activePkID 早就不是这个值了，直接放弃，不会误伤。
func (pl *PKPipeline) endActivePK(requirePkID string) bool {
	pl.mu.Lock()
	hadActive := pl.activePkID != "" && (requirePkID == "" || pl.activePkID == requirePkID)
	if hadActive {
		pl.lastEndedPkID = pl.activePkID
		pl.activePkID = ""
		pl.link = nil
	}
	pl.mu.Unlock()

	if hadActive {
		// c.EndPK() 内部按 c.pkLink（不是我们这里的本地缓存）操作，
		// 即使 startPK 那个 goroutine 还没来得及把 link 登记到
		// pl.link，也不影响它找到真正需要断开的连接。
		go pl.client.EndPK()
	}
	return hadActive
}

// pkEndTimeFallbackGrace 是超时兜底相对 pk_basic.end_time 额外多等的
// 缓冲——end_time 是 B 站预告的（大概率）战斗结束时间，不是「收尾完成
// 时间」，网络抖动、结算阶段的延迟都可能让真正的 PK_BATTLE_END 比它晚
// 到几秒到几十秒；给一个宽松但有限的缓冲，避免一场其实还没真正结束的
// PK 被误判成「CMD 丢了」。跟 pkTeardownGraceLimit/shutdownGraceLimit
// 不是同一类窗口（那两个兜的是「清理动作本身卡住」），这里兜的是
// 「PK_BATTLE_END 这条 CMD 本身从未到达」。
const pkEndTimeFallbackGrace = 30 * time.Second

// watchEndTimeFallback 是 PK_BATTLE_END 丢失时的兜底。
//
// c.events 只有 eventBufferSize（256）缓冲，PK 接通瞬间恰好是弹幕礼物
// 最密集、最容易把某一条 CMD 挤丢的时刻——Client.handleMessage 满溢时
// 走 default 分支直接丢弃，不重试不告警。如果丢的正好是
// PK_BATTLE_END，后果不是「少一次收尾」：没有任何后续 CMD 会补发它，
// pl.link/c.pkLink/pkRound 全部保持存活，对面那条 WebSocket 一直挂着、
// 持续重连，ClassifyVisit 会一直把戴对面勋章的人当串门来客欢迎，直到
// 下一场 PK 接通时 StartPK 内部的防御性收尾（c.endPKLocked，"不允许两场
// PK 的连接叠加"）才会自愈——中间可能隔几小时。
//
// pk_basic.end_time 早在 PK_INFO 阶段就已经拿到（event.Battle.EndTime，
// cmdmap/battle.go 的 mapPkInfo），用它做一个几乎零成本的超时兜底：
// 真正的 PK_BATTLE_END 到来时 pl.activePkID 已经被清空/被新一场顶替，
// endActivePK 传入的 pkID 参数会跟当前的 activePkID 对不上，兜底直接
// 放弃，不会误伤已经正常结束或已经是下一场的 PK。
//
// 【已知的极窄边角，不修】如果 end_time 在 PK_INFO 抵达时就已经过期
// （wait 被钳到 0），这个 goroutine 会几乎立即调用 endActivePK/EndPK，
// 理论上可能跟同一时刻 handleBattle 并发起的 startPK 里那次
// c.StartPK() 竞争 c.pkMu：如果 EndPK 抢先拿到锁，此时 c.pkLink 还是
// nil（StartPK 还没来得及注册），EndPK 会是空操作，随后 StartPK 才把
// 真实连接注册上去——这个新连接不会被这次已经空跑过的 EndPK 收尾，
// 要等下一场 PK 的防御性收尾才会清理，退化成了这个兜底本来要解决的
// 那个问题。这个窗口在正常场景下不可达：end_time 通常是几分钟之后，
// 远大于 c.StartPK() 内部「注册 + 起 goroutine」这几行代码的耗时，只有
// 「end_time 送达时就已经过期」这种本身就不该发生的报文异常才会撞上。
func (pl *PKPipeline) watchEndTimeFallback(ctx context.Context, b event.Battle) {
	grace := pl.endTimeFallbackGrace
	if grace <= 0 {
		grace = pkEndTimeFallbackGrace
	}
	wait := time.Until(time.Unix(b.EndTime, 0).Add(grace))
	if wait < 0 {
		wait = 0
	}

	timer := time.NewTimer(wait)
	defer timer.Stop()

	select {
	case <-ctx.Done():
		return
	case <-timer.C:
	}

	if pl.endActivePK(b.PkID) {
		pl.client.log.Warn("PK 超过预告的 end_time 仍未收到 PK_BATTLE_END，触发超时兜底收尾",
			"room", pl.client.roomID, "pkId", b.PkID, "endTime", b.EndTime)
	}
}

// startPK 是「PK 接通」触发的编排动作，独立 goroutine 里跑，不阻塞
// loop 对 Client.Events() 的消费。
//
// 【复审 Important-3 订正】StartPK 与 FetchOpponentSnapshots 仍然并发
// 发起（FetchOpponentSnapshots 起在独立 goroutine，不等 StartPK），
// 但 **pl.link 的发布不再等 FetchOpponentSnapshots**——上一版用一个
// 公共 sync.WaitGroup 等两者都返回才发布 link，等于把方向 A 判定的
// 生效时刻绑死在 FetchOpponentSnapshots 的 5s 预算上。而
// pkRound.opponentRoomIDs（粉丝勋章判据的依据）在 StartPK 内部的
// connect() 同步阶段就已经就绪，StartPK 本身也不阻塞（只登记 + 起
// goroutine，见 client.go 对 StartPK 的注释）——没有理由让它陪着快照
// 一起等。现在 StartPK 一返回就发布 pl.link，让方向 A 判定在 PK 接通
// 的头几秒就生效，不再错过对面观众涌入的那个窗口；快照仍然并发抓，
// 抓完再合成播报事件。
func (pl *PKPipeline) startPK(ctx context.Context, b event.Battle) {
	defer pl.wg.Done()

	snapshotDone := make(chan []OpponentSnapshot, 1)
	go func() {
		snapshotDone <- pl.client.FetchOpponentSnapshots(ctx, b.Members)
	}()

	link := pl.client.StartPK(ctx, b.Members)

	pl.mu.Lock()
	stillCurrent := pl.activePkID == b.PkID
	if stillCurrent {
		pl.link = link
	}
	pl.mu.Unlock()

	if !stillCurrent {
		// 这场 PK 在 StartPK 跑的这段时间里已经结束（见到了
		// PK_BATTLE_END）或被新一场 PK_INFO 顶替——不管哪一种，
		// c.StartPK/c.EndPK 内部的 endPKLocked 都已经把这个 link
		// 断干净了（Client 层面的互斥保证，opponent_link_test.go 的
		// TestConcurrentStartPKDoesNotOrphanEarlierLink 已经覆盖过这条
		// 保证）。这里只是不要把一个已经过期、不再对应「当前」的 link
		// 挂上去当作分类依据，也不用再等快照、不用再消费它的事件——
		// 直接放弃。
		return
	}

	snapshots := <-snapshotDone

	// 快照抓完这段时间里，这场 PK 也可能已经结束/被顶替——同样的
	// stillCurrent 检查，避免把一份不再对应「当前」的快照当成当前播出去。
	pl.mu.Lock()
	stillCurrent = pl.activePkID == b.PkID
	pl.mu.Unlock()
	if stillCurrent {
		pl.forward(pl.buildSnapshotEvent(b, snapshots))
	}

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
