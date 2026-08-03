package main

import (
	"context"
	"log/slog"
	"time"

	"github.com/MrZhongzq/MagicalDanmaku/server/internal/connector/bilibili/api"
	"github.com/MrZhongzq/MagicalDanmaku/server/internal/connector/bilibili/auth"
	"github.com/MrZhongzq/MagicalDanmaku/server/internal/store"
)

// roomStatusCheckInterval 是直播间开播状态心跳的检测间隔。
//
// 与账号登录态检测（loginCheckInterval，10 分钟）是完全独立的两件事，
// 不要合并成同一个循环、同一个间隔：账号登录态分钟级都不会变
// （loginCheckInterval 的注释解释过原因），但开播状态是主播随时可能
// 切换的东西，10 分钟一次会让用户觉得界面"跟不上"——60 秒是本任务
// 明确要的心跳粒度。
const roomStatusCheckInterval = 60 * time.Second

// roomStatusChecker 探测单个直播间的开播状态与主播身份，抽成函数类型
// 便于测试注入假实现——理由与 loginChecker 完全一致：真打 B 站接口的
// 测试既慢又不可控，尤其还要验证"探测失败"这条分支。
type roomStatusChecker func(ctx context.Context, cookie, roomID string) (*api.RoomStatus, error)

// newAPIRoomStatusChecker 是生产环境用的 roomStatusChecker 实现：解析
// Cookie、建一个新的 API 客户端、探测直播间状态。
//
// 每一轮检测都新建一个 Client 而不是复用已有账号运行时——心跳覆盖的
// 是 bindings 表里的全部绑定（不止运行中的那些），与 newAPILoginChecker
// 的取舍是同一个道理。
func newAPIRoomStatusChecker() roomStatusChecker {
	return func(ctx context.Context, cookie, roomID string) (*api.RoomStatus, error) {
		sess, err := auth.ParseSession(cookie)
		if err != nil {
			return nil, err
		}
		return api.New(sess).RoomStatus(ctx, roomID)
	}
}

// resolveRoomState 把一次探测结果映射成 store 的三态之一。
//
// **这是本任务最需要守住的一条判断，只在这一处实现**：err != nil
// （探测本身失败——网络错误、超时、风控）必须映射成 RoomLiveUnknown，
// 绝不能是 RoomLiveOffline——那是把"拿不到"伪装成"确认没开播"，是一个
// 看起来正常、实则彻底错误的结论。心跳循环（roomStatusCheckOnce）与
// 立即检测（bindingRoomStatusProbe）共用这一份映射，不各写一套，
// 避免两处对"失败该映射成什么"的理解在将来悄悄分叉。
func resolveRoomState(status *api.RoomStatus, err error) (state, anchorUID, anchorName string) {
	if err != nil {
		return store.RoomLiveUnknown, "", ""
	}
	if status.IsLiving() {
		return store.RoomLiveLiving, status.AnchorUID, status.AnchorName
	}
	// 未开播与轮播中都算"没有在直播互动"，语义上与 api.RoomInfo.IsLiving
	// 保持一致，不额外区分轮播这第三种取值。
	return store.RoomLiveOffline, status.AnchorUID, status.AnchorName
}

// roomStatusCheckOnce 对 bindings 表里的每一条绑定各做一次直播间状态
// 探测并写库。
//
// 遍历全部绑定（不管是否启用）：与账号登录态检测覆盖全部账号是同一个
// 道理——WebUI 的绑定卡片本来就会展示全部绑定，用户在决定要不要重新
// 启用一个已停用的绑定之前，同样想知道那个房间现在是否在播。
//
// 探测串行进行，不开 goroutine 并发发出多个请求：与 loginCheckOnce
// 一致，是对 B 站的一种克制的风控姿态。
//
// 同账号的多个绑定共享同一次账号查询结果，避免同一个 Cookie 被
// GetAccountByName 重复查询——绑定规模是"账号数 × 房间数"，重复查询
// 的代价在这个规模下可忽略，但顺手做了就不必之后再补。
func roomStatusCheckOnce(ctx context.Context, st *store.Store, check roomStatusChecker, log *slog.Logger) {
	bindings, err := st.ListBindings(ctx)
	if err != nil {
		log.Error("直播间状态检测: 列出绑定失败", "err", err)
		return
	}

	accounts := make(map[string]*store.Account)
	for _, b := range bindings {
		// 关停时（ctx 已被取消）提前退出，理由与 loginCheckOnce 完全一致：
		// 不再对剩下的绑定做注定失败的探测，避免刷一堆没有信息量的日志。
		select {
		case <-ctx.Done():
			return
		default:
		}

		acc, ok := accounts[b.AccountName]
		if !ok {
			a, err := st.GetAccountByName(ctx, b.AccountName)
			if err != nil {
				log.Warn("直播间状态检测: 查账号失败", "binding", b.Label(), "err", err)
				if uerr := st.UpdateBindingRoomStatus(ctx, b.ID, store.RoomLiveUnknown, "", ""); uerr != nil {
					log.Error("写入直播间状态失败", "binding", b.Label(), "err", uerr)
				}
				continue
			}
			acc = a
			accounts[b.AccountName] = acc
		}

		status, checkErr := check(ctx, acc.Cookie, b.RoomID)
		if checkErr != nil {
			log.Warn("直播间状态探测失败", "binding", b.Label(), "err", checkErr)
		}
		state, uid, name := resolveRoomState(status, checkErr)
		if uerr := st.UpdateBindingRoomStatus(ctx, b.ID, state, uid, name); uerr != nil {
			log.Error("写入直播间状态失败", "binding", b.Label(), "err", uerr)
		}
	}
}

// roomStatusCheckLoop 定期对全部绑定做直播间状态探测，模式与
// loginCheckLoop/purgeLoop 一致：一个 goroutine + ticker + ctx 取消，
// 启动时立刻检测一次，不等第一个 tick。
func roomStatusCheckLoop(ctx context.Context, st *store.Store, check roomStatusChecker, log *slog.Logger) {
	ticker := time.NewTicker(roomStatusCheckInterval)
	defer ticker.Stop()

	run := func() { roomStatusCheckOnce(ctx, st, check, log) }

	run()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			run()
		}
	}
}
