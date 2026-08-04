package main

import (
	"context"
	"log/slog"

	"github.com/MrZhongzq/MagicalDanmaku/server/internal/httpapi"
	"github.com/MrZhongzq/MagicalDanmaku/server/internal/store"
)

// 本文件实现 httpapi.LoginProbe 与 httpapi.RoomStatusProbe——扫码成功后
// 立即探测一次登录态、新增绑定后立即探测一次直播间状态（P5-2 任务 2）。
// httpapi 自己不知道怎么打 B 站接口，两个探测器都复用已有的
// loginChecker/roomStatusChecker 类型（心跳循环也用它们），保证"立即
// 检测一次"与"后台定期心跳"对同一次探测结果的判定逻辑完全一致，不会
// 出现"立即检测说有效，10 分钟后的心跳又判定失效"这种自相矛盾。

// accountLoginProbe 是 httpapi.LoginProbe 的生产实现。
type accountLoginProbe struct {
	st    *store.Store
	check loginChecker
	log   *slog.Logger
}

var _ httpapi.LoginProbe = (*accountLoginProbe)(nil)

// ProbeNow 探测 accountName 对应账号的登录态并写库。
//
// 账号在探测前已被删掉（正常竞态：扫码轮询与删账号并发）不算错误，
// 只记日志、安静返回——道理与 loginCheckOnce 遍历账号快照时的处理
// 一致。
func (p *accountLoginProbe) ProbeNow(ctx context.Context, accountName string) {
	acc, err := p.st.GetAccountByName(ctx, accountName)
	if err != nil {
		p.log.Warn("登录态立即检测: 查账号失败", "account", accountName, "err", err)
		return
	}

	state, err := p.check(ctx, acc.Cookie)
	if err != nil {
		p.log.Warn("登录态立即检测失败", "account", accountName, "err", err)
	}
	if uerr := p.st.UpdateAccountLoginState(ctx, accountName, state); uerr != nil {
		p.log.Error("写入登录态失败", "account", accountName, "err", uerr)
	}
}

// bindingRoomStatusProbe 是 httpapi.RoomStatusProbe 的生产实现。
type bindingRoomStatusProbe struct {
	st    *store.Store
	check roomStatusChecker
	// notify 把探测结果同步给运行时的事件分发循环（P6 任务 4），与心跳
	// 循环（roomStatusCheckOnce）共用同一份 liveStatusNotifier 实现——
	// 两处都要能在探测完成后让"未开播时不处理高能榜/进房事件"这件事
	// 立即生效，不必等下一轮 60 秒心跳。可能为 nil：测试通常不关心
	// 这一步。
	notify liveStatusNotifier
	log    *slog.Logger
}

var _ httpapi.RoomStatusProbe = (*bindingRoomStatusProbe)(nil)

// ProbeNow 探测 bindingID 对应的直播间并写库。
//
// 绑定在探测前已被删掉（正常竞态）不算错误，只记日志、安静返回。
func (p *bindingRoomStatusProbe) ProbeNow(ctx context.Context, bindingID int64) {
	b, err := p.st.GetBindingByID(ctx, bindingID)
	if err != nil {
		p.log.Warn("直播间状态立即检测: 查绑定失败", "bindingID", bindingID, "err", err)
		return
	}
	acc, err := p.st.GetAccountByName(ctx, b.AccountName)
	if err != nil {
		p.log.Warn("直播间状态立即检测: 查账号失败", "binding", b.Label(), "err", err)
		return
	}

	status, err := p.check(ctx, acc.Cookie, b.RoomID)
	if err != nil {
		p.log.Warn("直播间状态立即检测失败", "binding", b.Label(), "err", err)
	}
	// 复用与心跳循环完全相同的一份映射（resolveRoomState），确保"立即
	// 检测"与"60 秒心跳"对同一个探测结果的判定不会走岔。
	state, uid, name := resolveRoomState(status, err)
	if uerr := p.st.UpdateBindingRoomStatus(ctx, bindingID, state, uid, name); uerr != nil {
		p.log.Error("写入直播间状态失败", "binding", b.Label(), "err", uerr)
	}
	notifyLiveStatus(p.notify, bindingID, state)
}
