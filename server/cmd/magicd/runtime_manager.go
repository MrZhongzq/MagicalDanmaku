package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync"

	"github.com/MrZhongzq/MagicalDanmaku/server/internal/connector/bilibili"
	"github.com/MrZhongzq/MagicalDanmaku/server/internal/event"
	"github.com/MrZhongzq/MagicalDanmaku/server/internal/httpapi"
	"github.com/MrZhongzq/MagicalDanmaku/server/internal/logging"
	"github.com/MrZhongzq/MagicalDanmaku/server/internal/scheduler"
	"github.com/MrZhongzq/MagicalDanmaku/server/internal/store"
)

// apiRuntimeSink 是 runtimeManager 需要从 httpapi.Server 借用的最小能力
// 集合：登记/摘除单个绑定的运行期能力、把事件扇出给网页。
//
// 抽成接口而不是直接持有 *httpapi.Server，理由与 run.go 里的
// danmakuSender/loginChecker 一致：测试不必起一个真正的 httpapi.Server
// （要真实的 *store.Store、认证、路由）就能验证 runtimeManager 在
// 装配/拆除时有没有正确调用登记/摘除，也能验证「HTTP 关闭时
// （api 为 nil）runtimeManager 照常工作」这条路径。
type apiRuntimeSink interface {
	PutRuntime(bindingID int64, rt httpapi.BindingRuntime)
	RemoveRuntime(bindingID int64)
	Publish(bindingID int64, ev event.Event)
}

// 编译期核对 *httpapi.Server 确实满足这个接口——生产环境就是直接把它
// 传给 newRuntimeManager，接口方法签名对不上会在这里最先炸出来。
var _ apiRuntimeSink = (*httpapi.Server)(nil)

// liveBinding 是运行期正跑着的一个绑定的全部可拆除资源。
type liveBinding struct {
	room   *roomRuntime
	bot    *roomBot
	client *bilibili.Client

	// cancel 只取消这一个绑定派生出的子 ctx，不牵连宿主根 ctx 或
	// 其余绑定——这是「拆一个绑定不影响其余绑定」的关键：根 ctx 取消
	// （宿主整体关停）会级联取消全部子 ctx，反过来单独取消一个子 ctx
	// 对根 ctx 与兄弟绑定毫无影响。
	cancel context.CancelFunc

	// stopped 在事件扇出 + 连接两条 goroutine 都退出后关闭，供
	// StopBinding 等待——不等的话，cancel 之后立刻去关引擎，事件扇出
	// goroutine 可能还在调用 rt.Engine().Handle()，与引擎的 Close 并发。
	stopped chan struct{}
}

// runtimeManager 是绑定运行时的动态注册表：新增/启用一个绑定时按需
// 装配并登记，停用/删除时反向拆除——取代 P5-1 之前「启动时装配一次、
// 之后只读」的静态 map（run.go 里原来那个一次性 apiRuntimes + 一次性
// api.SetRuntime）。
//
// 复用 assembleOne/buildRoomRuntime：这两个函数本来就是"给定一份
// RunConfig 装配出可运行资源"的通用逻辑，进程启动时的批量装配
// （runRun 里的 assembleRuntimes）与这里的单个动态装配没有理由各写
// 一套；两者共同的"把装配好的资源起 goroutine、登记进注册表"收尾
// 逻辑收在 Adopt，同样只有一份。
//
// **锁的粒度是刻意选粗的**：mu 序列化全部 Start/Stop，不管操作的是不是
// 同一个绑定。细粒度（每个绑定各自一把锁）能让不同绑定的增删启停互不
// 阻塞，但换来的复杂度是这次任务担不起的——绑定的增删启停是人在
// WebUI 上点出来的操作，频率是「偶尔」，不是热路径；而 P4-4 已经在
// 「拆除路径的并发正确性」上出过两个 Critical（连接登记时序导致永久
// 泄漏、共享无锁 Session 导致进程 fatal），这次选择用一把粗锁把
// 「装配→登记→起 goroutine」或「取消→等 goroutine 退出→摘除」这类
// 复合操作变成显然正确的原子步骤，而不是再去抠细粒度锁的正确性。
type runtimeManager struct {
	mu sync.Mutex

	ctx      context.Context // run 的根 ctx；派生的绑定级子 ctx 在根 ctx 取消时自动级联取消
	st       *store.Store
	sched    *scheduler.Scheduler
	activity *logging.ActivityWriter
	api      apiRuntimeSink // 可能为 nil（HTTP 关闭，见 run.go 的 apiSink 说明）
	log      *slog.Logger
	wg       *sync.WaitGroup // run 的主 wg：绑定级 goroutine 记进这里，宿主关停靠它等干净

	accounts map[string]*accountRuntime // 跨绑定共享限流器；由 runRun 用 buildAccounts 建好的那份接手，不重建
	live     map[int64]*liveBinding

	// buildAccount 装配单个账号运行时，默认是 buildAccountRuntime（含一次
	// 真实的 RefreshNav 网络请求）。抽成字段而不是直接调用包级函数，
	// 只是为了让 ensureAccountRuntime 的"命中缓存但 Cookie 已经在数据库
	// 里被换新，需要重建"这条分支可以在单元测试里不打真实网络就验证——
	// 与 apiRuntimeSink 抽接口是同一个理由。生产环境里 newRuntimeManager
	// 把它设成 buildAccountRuntime，行为与抽出这个字段之前完全一致。
	buildAccount func(ctx context.Context, c store.RunConfig) (*accountRuntime, error)
}

// newRuntimeManager 创建运行时管理器。accounts 应该是 runRun 里
// buildAccounts 已经装配好的那份——同一账号的多个绑定必须共享同一个
// accountRuntime（也就共享同一个限流器），接手它而不是从空表重建，
// 是这份共享得以在「进程启动时就存在的绑定」与「运行期动态新增的
// 同账号绑定」之间延续下去的唯一原因。
func newRuntimeManager(
	ctx context.Context,
	st *store.Store,
	sched *scheduler.Scheduler,
	activity *logging.ActivityWriter,
	api apiRuntimeSink,
	log *slog.Logger,
	wg *sync.WaitGroup,
	accounts map[string]*accountRuntime,
) *runtimeManager {
	if accounts == nil {
		accounts = make(map[string]*accountRuntime)
	}
	return &runtimeManager{
		ctx:          ctx,
		st:           st,
		sched:        sched,
		activity:     activity,
		api:          api,
		log:          log,
		wg:           wg,
		accounts:     accounts,
		live:         make(map[int64]*liveBinding),
		buildAccount: buildAccountRuntime,
	}
}

var _ httpapi.BindingLifecycle = (*runtimeManager)(nil)
var _ httpapi.AccountRuntimeUpdater = (*runtimeManager)(nil)
var _ liveStatusNotifier = (*runtimeManager)(nil)

// ensureAccountRuntime 取或建 c.AccountName 对应的账号运行时。
//
// 调用者必须持有 rm.mu——与 StartBinding 整体持锁是同一个理由：
// 「查表未命中 → 建账号 → 登记」是复合操作，不加锁的话两个并发请求
// 可能各自建出一份，其中一份的限流器再也没人共享。
//
// **Important-1 的修复**：命中缓存时额外比一次 Cookie。运行时缓存装配
// 于「上一次这个账号被装配」那一刻，而账号掉线后用户重新扫码只会走
// account_handler.go 的 saveScannedAccount → store.UpdateAccountCookie
// 写库，不会通知任何还在跑的 runtimeManager——此前 Cookie 换了缓存也
// 不失效，用户按 P5-1 教的「停用再启用」这个操作重启这个绑定时，
// StartBinding 仍然会在这里命中缓存，把绑定接到装配那一刻的死会话上，
// 界面上却因为 P5-2 的立即探测显示登录态正常，看起来无法解释。
//
// c 来自 StartBinding 每次现查的 LoadRunConfig，Cookie 字段永远是数据库
// 里的最新值，因此这里的比较不会有陈旧读的问题。
func (rm *runtimeManager) ensureAccountRuntime(ctx context.Context, c store.RunConfig) (*accountRuntime, error) {
	if acctRT, ok := rm.accounts[c.AccountName]; ok {
		if acctRT.cookie == c.Cookie {
			return acctRT, nil
		}
		rm.log.Info("账号 Cookie 已更新，重建运行时", "name", c.AccountName)
		rebuilt, err := rm.buildAccount(ctx, c)
		if err != nil {
			return nil, err
		}
		// 限流器必须是原来那一份，不能跟着重建换掉：风控按账号累计节奏
		// 计算，同一账号可能有多个绑定共享它，重建时若换成一份全新的
		// （从零开始计时的）限流器，等于让这个账号的发送节奏计时器无声
		// 重置，也会让其余仍持有旧 accountRuntime 指针的绑定与这里新建的
		// 这份各算各的，共享限流器的语义就破了。
		rebuilt.acc.Limiter = acctRT.acc.Limiter
		rm.accounts[c.AccountName] = rebuilt
		return rebuilt, nil
	}
	acctRT, err := rm.buildAccount(ctx, c)
	if err != nil {
		return nil, err
	}
	rm.accounts[c.AccountName] = acctRT
	rm.log.Info("已载入账号", "name", c.AccountName, "uid", acctRT.acc.Session.UID)
	return acctRT, nil
}

// StartBinding 为 bindingID 建立连接、装配规则引擎、注册定时任务，
// 登记进运行时注册表。已经在跑的绑定重复调用是幂等的。
//
// 实现 httpapi.BindingLifecycle：绑定的增删启停 handler 在数据库状态
// 改对之后调用这个方法，把「让改动在运行期生效」这件事交出去——这正是
// P5-1 要修的问题：过去只改库，不重启进程不生效。
func (rm *runtimeManager) StartBinding(ctx context.Context, bindingID int64) error {
	rm.mu.Lock()
	defer rm.mu.Unlock()

	if _, ok := rm.live[bindingID]; ok {
		return nil // 已经在跑，幂等——重复点启用按钮不该报错
	}

	// 用 LoadRunConfig 现拉一份最新的启用绑定列表，而不是另开一个只查
	// 单个绑定的存储方法——LoadRunConfig 本来就是"拿到装配一个绑定所需
	// 全部信息"的唯一入口（规则、冷却组、账号 Cookie 都在这一次查询里
	// 拼好了），绑定规模是几个到几十个，多拉整表不值得为这一条路径
	// 另写一套查询。
	cfgs, err := rm.st.LoadRunConfig(ctx)
	if err != nil {
		return fmt.Errorf("读取配置失败: %w", err)
	}
	var cfg *store.RunConfig
	for i := range cfgs {
		if cfgs[i].BindingID == bindingID {
			cfg = &cfgs[i]
			break
		}
	}
	if cfg == nil {
		// 调用者（handler）刚把这个绑定标成启用，这里却查不到，只可能是
		// 绑定在这两步之间又被别的请求删掉/停用了，或者调用者传了个
		// 根本不存在的 ID——不管哪种，这不是 StartBinding 能处理的
		return fmt.Errorf("绑定 %d 不在当前启用列表里", bindingID)
	}

	acctRT, err := rm.ensureAccountRuntime(ctx, *cfg)
	if err != nil {
		return err
	}

	asm, err := assembleOne(ctx, *cfg, acctRT, rm.activity, rm.st, rm.sched, rm.log)
	if err != nil {
		return err
	}

	rm.adoptLocked(ctx, asm)
	return nil
}

// Adopt 起 asm 的事件扇出/连接 goroutine，登记进 rm.live 与 httpapi 的
// 运行时注册表。供 runRun 在进程启动时对 assembleRuntimes 装配出的每个
// 绑定调用——与 StartBinding 内部走的是同一段收尾逻辑（adoptLocked），
// 不重复实现。
func (rm *runtimeManager) Adopt(ctx context.Context, asm roomAssembly) {
	rm.mu.Lock()
	defer rm.mu.Unlock()
	rm.adoptLocked(ctx, asm)
}

// adoptLocked 是 Adopt/StartBinding 共同的收尾逻辑，调用者必须持有 rm.mu。
func (rm *runtimeManager) adoptLocked(ctx context.Context, asm roomAssembly) {
	bindingID := asm.cfg.BindingID
	bindCtx, cancel := context.WithCancel(rm.ctx)
	lb := &liveBinding{
		room: asm.room, bot: asm.bot, client: asm.client,
		cancel: cancel, stopped: make(chan struct{}),
	}

	// 把发送 ctx 收敛到 bindCtx——**这是 Critical-1 的修复**。
	//
	// asm.bot 装配时（assembleOne → buildRoomRuntime → newRoomBot）用的是
	// 调用方传进来的 ctx：StartBinding 场景下那是 HTTP handler 的
	// r.Context()，net/http 会在 ServeHTTP 返回时把它 cancel 掉；启动时
	// 批量装配场景下那是 run 的根 ctx，凑巧和 bindCtx 的父 ctx 是同一个，
	// 所以这条洞只在运行期动态启动（P5-1 新增的路径）才会命中。
	//
	// 不换的话：请求一结束，bot.ctx 就是一个已取消的 ctx，
	// account.Binding.SendDanmaku/Block 的第一步 Limiter.Wait(ctx) 立刻
	// 返回 context.Canceled——绑定连得上、收得到事件，但一条弹幕也发不
	// 出去，日志里却没有任何报错（连接 goroutine 用的是从 rm.ctx 派生的
	// bindCtx，不受影响，照常打「已配置绑定」「已连接直播间」）。
	//
	// bindCtx 派生自 rm.ctx，生命周期恰好等于这个绑定：随 rm.ctx（宿主
	// 整体关停）级联取消，也会在 StopBinding→teardownLocked 里被显式
	// cancel。必须在起 goroutine 之前做这一步——事件扇出 goroutine 一
	// 收到事件就可能触发规则动作，若那时 bot.ctx 还是旧的，第一条弹幕
	// 就会撞上这个问题。
	asm.bot.setCtx(bindCtx)

	var local sync.WaitGroup
	local.Add(2)
	rm.wg.Add(2)
	go func(rt *roomRuntime, c *bilibili.Client, bID int64) {
		defer rm.wg.Done()
		defer local.Done()
		// 消费 PKPipeline 合流后的通道，不直接消费 c.Events()：这一步
		// 把 StartPK/EndPK/FetchOpponentSnapshots/ClassifyVisit 接进了
		// 实时事件流（P4-4 Task 7）——用法与直接消费 c.Events() 时完全
		// 一致，只是多了 PK 期间合成的快照事件与串门信号事件。
		for ev := range bilibili.NewPKPipeline(c).Run(bindCtx) {
			// 未开播时不处理高能榜/进房事件（P6 任务 4）。rt.LiveOffline()
			// 每次都要重新读，理由与下面 rt.Engine() 的注释一致：状态会被
			// 心跳/立即探测异步更新，缓存一次就等于把某一刻的快照捕获死。
			if shouldSkipOfflineEvent(rt.LiveOffline(), ev.Type) {
				continue
			}
			// rt.Engine() 每次都要重新取：热重载会把 rt.engine 换成
			// 新引擎，若在循环外缓存一次就等于把旧引擎闭包捕获死了，
			// 重载之后事件会继续打在已经 Close 掉的旧引擎上。
			rt.Engine().Handle(ev)
			if rm.api != nil {
				// 扇出给网页。必须在 Handle 之后：机器人的响应也是通过
				// 弹幕事件回流的，顺序颠倒会让因果看起来是反的。
				rm.api.Publish(bID, ev)
			}
		}
	}(asm.room, asm.client, bindingID)

	go func(label string, c *bilibili.Client) {
		defer rm.wg.Done()
		defer local.Done()
		// 单个绑定的连接出错不影响其他绑定，也不做账号切换
		if err := c.Run(bindCtx); err != nil && !errors.Is(err, context.Canceled) {
			rm.log.Error("绑定连接退出", "binding", label, "err", err)
		}
	}(asm.room.label, asm.client)

	go func() { local.Wait(); close(lb.stopped) }()

	rm.live[bindingID] = lb

	if rm.api != nil {
		rm.api.PutRuntime(bindingID, asm.room)
	}
}

// StopBinding 反向拆除 bindingID 的运行时：停连接、注销定时任务、结算
// 引擎未决的合并窗口、从注册表摘除。已经不在跑的绑定重复调用是幂等的。
//
// 实现 httpapi.BindingLifecycle。ctx 参数刻意未使用：拆除过程中唯一的
// 网络活动是结算未决合并窗口时补发弹幕，而这必须用一个脱离调用方 ctx
// 取消链、有自己超时预算的独立 ctx（teardownLocked 里的 flushCtx）——
// 调用方通常是一次 HTTP 请求，若改用它的 ctx，请求一结束（或客户端
// 提前断开）这次补发就会被腰斩，跟 run.go 里 closeAll 不能直接用主 ctx
// 是同一个道理。
func (rm *runtimeManager) StopBinding(_ context.Context, bindingID int64) {
	rm.mu.Lock()
	defer rm.mu.Unlock()

	lb, ok := rm.live[bindingID]
	if !ok {
		return // 已经不在跑，幂等——重复点停用按钮、或删除一个已停用的绑定都不该出错
	}
	delete(rm.live, bindingID)
	rm.teardownLocked(lb)
}

// teardownLocked 拆一个绑定的全部运行时资源。调用者必须持有 rm.mu——
// 理由见 runtimeManager 类型注释：这是复合操作，不是这次要引入细粒度
// 锁的地方。
//
// 顺序是刻意的，与 run.go 里 closeAll／roomRuntime.Reload 的收尾原则
// 一致：
//
//  1. 先切断这个绑定自己的连接（只取消它派生出的子 ctx），并等两条
//     goroutine 真正退出——不等的话，下面 Close 引擎时，事件扇出
//     goroutine 可能还在并发调用 rt.Engine().Handle()。
//  2. 注销这个绑定的全部定时任务。RemoveByPrefix 就是为这一步加的：
//     不摘的话，将来这个绑定重新启用，旧条目还残留在调度器里、且仍然
//     指向已经被下一步 Close 掉的旧引擎——同一条规则一天触发两次，
//     或者静默地什么都不触发，取决于运气。roomRuntime.Reload 的注释
//     写过一模一样的坑，拆除时同样要摘干净。
//  3. 结算引擎未决的合并窗口（比如攒着的欢迎语）。这一步会通过
//     roomBot 发送，而第 1 步取消的子 ctx 此刻已经失效——若引擎结算时
//     仍用那个 ctx 发送会立刻拿到 context.Canceled，一条都发不出去，
//     所以先把 bot 的发送 ctx 换成一个脱离取消链、有超时预算的独立
//     ctx（与 run.go 里 closeAll 的 shutdownCtx 是同一个道理）。
//  4. logging.Sink 本身不持有 goroutine 或需要关闭的资源（它只是共享
//     ActivityWriter 的一个薄视图，见 internal/logging/sink.go），
//     拆除之后没有任何代码路径会再调用它，天然停止产生日志，无需
//     显式关闭。
//  5. 从 httpapi 的运行时注册表摘除——此后针对这个绑定的即时动作/
//     重载请求会如实收到「未在运行」，而不是命中一个已经拆除掉的
//     运行时。
func (rm *runtimeManager) teardownLocked(lb *liveBinding) {
	lb.cancel()
	<-lb.stopped

	lb.room.sched.RemoveByPrefix(lb.room.label + "/")

	flushCtx, cancel := context.WithTimeout(context.WithoutCancel(rm.ctx), shutdownSendBudget)
	defer cancel()
	lb.bot.setCtx(flushCtx)
	lb.room.Engine().Close()

	if rm.api != nil {
		rm.api.RemoveRuntime(lb.room.bindingID)
	}
}

// maxLengthSetter 是「按连接器设置单条弹幕字数上限」的可选能力，只有
// bilibili.Actions 实现它——字数上限是 B 站特有的协议限制，不写进
// connector.Actions 这个跨平台接口，未来接入的其他连接器不必被迫实现
// 一个对它们没有意义的方法。UpdateAccountRuntime 用类型断言取用这个
// 能力，取不到就跳过（虽然目前只有 bilibili 一种连接器，这条分支
// 理论上不会走到）。
type maxLengthSetter interface {
	SetMaxLength(n int)
}

// UpdateAccountRuntime 把 accountName 在数据库里的最新参数（发送间隔、
// 单条弹幕字数上限）同步给该账号当前正在跑的全部绑定，不需要重启进程
// 就能生效。
//
// 实现 httpapi.AccountRuntimeUpdater：handlePatchAccount 保存成功后调用
// 这个方法，把"运行时该跟着变"这件事交出去——修的是 P5-1 报告记录的
// 已知局限（SetMaxLength 只在 buildRoomRuntime 装配那一刻调用一次），
// 现在有了实际后果（用户把上限从 20 改成 40 保存后，运行中的绑定仍按
// 20 切，要重启才生效，界面上也没有任何提示）。
//
// 只改**已经在跑**的绑定，不碰数据库；下次绑定启动/重新启用时，
// buildRoomRuntime 装配这一步本来就会读到数据库里的最新值，两条路径
// 不冲突、不重复。
func (rm *runtimeManager) UpdateAccountRuntime(ctx context.Context, accountName string) {
	rm.mu.Lock()
	defer rm.mu.Unlock()

	acctRT, ok := rm.accounts[accountName]
	if !ok {
		// 这个账号眼下没有被任何运行时持有——从未启用过绑定，或者
		// 唯一的绑定已经停用。没有可同步的目标，不是错误。
		return
	}

	acc, err := rm.st.GetAccountByName(ctx, accountName)
	if err != nil {
		rm.log.Warn("同步账号运行参数失败：查账号出错", "account", accountName, "err", err)
		return
	}

	// 限流器必须原地改间隔，不能换成一份新的——它被这个账号名下全部
	// 绑定共享，换掉的话新旧两份各算各的，其余绑定的 accountRuntime
	// 指针还指着旧的那份，共享限流的语义就破了（与 ensureAccountRuntime
	// 重建账号运行时时"限流器必须是原来那一份"是同一个道理）。
	acctRT.acc.Limiter.SetInterval(acc.RateLimit)

	// 字数上限逐个绑定地改：每个绑定的 bilibili.Actions 是各自独立的
	// 实例（buildRoomRuntime 里 bilibili.NewActions 一个绑定建一份），
	// 不像限流器那样整个账号共享一份，必须遍历 rm.live 找到这个账号名下
	// 全部还在跑的绑定逐一设置。
	for _, lb := range rm.live {
		if lb.room.binding.Account != acctRT.acc {
			continue
		}
		if setter, ok := lb.room.binding.Actions.(maxLengthSetter); ok {
			setter.SetMaxLength(acc.MaxLength)
		}
	}
}

// UpdateLiveStatus 把探测到的直播间开播状态同步给该绑定当前运行时的
// 事件分发循环，供其决定要不要处理高能榜/进房事件（P6 任务 4）。
//
// 实现 cmd/magicd 内部的 liveStatusNotifier（roomstatus.go）：心跳循环
// （roomStatusCheckOnce，每 60 秒）与立即探测（bindingRoomStatusProbe.
// ProbeNow，新增绑定时）都会调用它，两处对同一个绑定的判断保证一致。
//
// **这里是唯一一处把"探测状态"翻译成"要不要掐事件"的地方**：只有
// state 恰好是 store.RoomLiveOffline（明确探测到未开播）才置位；
// RoomLiveLiving 与 RoomLiveUnknown 都会清掉这个标记——探测失败必须
// 退回"允许处理"这一侧，绝不能把"拿不到状态"当成"确认未开播"，那会让
// 一次网络抖动变成机器人对高能榜/进房事件整个哑掉，这条红线本项目
// 反复强调过。开播后自动恢复也是同一行代码的自然结果：下一轮心跳测到
// living，标记被清掉，不需要重启。
//
// 绑定当前不在跑（未启用，或探测发生在装配完成之前的极短窗口）时什么
// 都不做，不是错误——roomRuntime 新建时 liveOffline 的零值就是 false
// （允许），下一轮心跳很快会把真实状态同步过来。
func (rm *runtimeManager) UpdateLiveStatus(bindingID int64, state string) {
	rm.mu.Lock()
	lb, ok := rm.live[bindingID]
	rm.mu.Unlock()
	if !ok {
		return
	}
	lb.room.SetLiveOffline(state == store.RoomLiveOffline)
}

// shouldSkipOfflineEvent 判断一条事件在"确认未开播"时该不该被跳过。
//
// 只掐高能榜（TypeOnlineRankUpdate）与进房（TypeUserEnter）这两类——
// 用户原话只提到这两类，弹幕/礼物/上舰等其余事件即便主播下播了也可能
// 仍有意义（比如观众在下播后的互动区继续聊天、答谢仍要触发），不该
// 顺手一起掐掉。offline 为 false（未确认未开播，含"拿不到状态"与
// "确认在播"两种情况）时无条件不跳过——这条判断本身不重复"拿不到状态
// 不算没开播"这条红线，那条红线已经在 UpdateLiveStatus 里通过状态映射
// 实现，这里只管"确认未开播"之后该不该处理某一类事件。
func shouldSkipOfflineEvent(offline bool, evType event.Type) bool {
	if !offline {
		return false
	}
	switch evType {
	case event.TypeOnlineRankUpdate, event.TypeUserEnter:
		return true
	default:
		return false
	}
}

// liveRoomRuntimes 返回当前仍登记在 rm.live 里的全部 roomRuntime/roomBot
// 快照，供宿主整体关停时的收尾使用（run.go 里 runRun 的 defer）。
//
// 只读快照，不做任何拆除——宿主整体关停时，根 ctx 取消会自动级联取消
// 全部绑定的子 ctx，两条 goroutine 各自退出、runRun 的 wg.Wait() 能等到；
// 随后 runRun 的 defer 用这份快照统一调用 closeAll 结算全部还在跑的
// 引擎。已经被 StopBinding 单独拆除过的绑定不会出现在这里——它们的
// 引擎在各自的拆除路径里已经关过了，不需要也不应该被重复 Close。
func (rm *runtimeManager) liveRoomRuntimes() (rts []*roomRuntime, bots []*roomBot) {
	rm.mu.Lock()
	defer rm.mu.Unlock()
	rts = make([]*roomRuntime, 0, len(rm.live))
	bots = make([]*roomBot, 0, len(rm.live))
	for _, lb := range rm.live {
		rts = append(rts, lb.room)
		bots = append(bots, lb.bot)
	}
	return rts, bots
}
