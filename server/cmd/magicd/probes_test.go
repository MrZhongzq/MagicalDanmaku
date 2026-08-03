package main

import (
	"context"
	"errors"
	"log/slog"
	"testing"

	"github.com/MrZhongzq/MagicalDanmaku/server/internal/connector/bilibili/api"
	"github.com/MrZhongzq/MagicalDanmaku/server/internal/store"
)

// ---- accountLoginProbe：httpapi.LoginProbe 的生产实现 ----

// TestAccountLoginProbeWritesLoginState 验证 ProbeNow 探测成功时把
// loginChecker 的判定结果写进对应账号——这是扫码成功后立即检测一次
// 用到的实现，与后台心跳循环共用同一个 loginChecker 类型，只是触发
// 时机不同。
func TestAccountLoginProbeWritesLoginState(t *testing.T) {
	st := newLoginCheckTestStore(t)
	ctx := context.Background()
	owner, err := st.CreateUser(ctx, "张三", "密码123456", false)
	if err != nil {
		t.Fatalf("建用户报错: %v", err)
	}
	if _, err := st.CreateAccount(ctx, store.AccountInput{Name: "小号", Cookie: "SESSDATA=x", OwnerID: owner.ID}); err != nil {
		t.Fatalf("建账号报错: %v", err)
	}

	check := func(context.Context, string) (string, error) { return store.LoginStateValid, nil }
	probe := &accountLoginProbe{st: st, check: check, log: slog.Default()}

	probe.ProbeNow(ctx, "小号")

	got, err := st.GetAccountByName(ctx, "小号")
	if err != nil {
		t.Fatalf("查询报错: %v", err)
	}
	if got.LoginState != store.LoginStateValid {
		t.Errorf("LoginState = %q, 期望 %q", got.LoginState, store.LoginStateValid)
	}
	if got.LoginCheckedAt == nil {
		t.Error("应记录 LoginCheckedAt")
	}
}

// TestAccountLoginProbeDetectionFailureIsNotInvalid 验证探测失败时写的
// 是 unknown，不是 invalid——与心跳循环的判断必须一致，否则扫码那一刻
// 网络抖动会被立即检测误报成「登录已失效」。
func TestAccountLoginProbeDetectionFailureIsNotInvalid(t *testing.T) {
	st := newLoginCheckTestStore(t)
	ctx := context.Background()
	owner, err := st.CreateUser(ctx, "张三", "密码123456", false)
	if err != nil {
		t.Fatalf("建用户报错: %v", err)
	}
	if _, err := st.CreateAccount(ctx, store.AccountInput{Name: "小号", Cookie: "SESSDATA=x", OwnerID: owner.ID}); err != nil {
		t.Fatalf("建账号报错: %v", err)
	}

	check := func(context.Context, string) (string, error) {
		return store.LoginStateUnknown, errors.New("网络错误")
	}
	probe := &accountLoginProbe{st: st, check: check, log: slog.Default()}
	probe.ProbeNow(ctx, "小号")

	got, err := st.GetAccountByName(ctx, "小号")
	if err != nil {
		t.Fatalf("查询报错: %v", err)
	}
	if got.LoginState == store.LoginStateInvalid {
		t.Error("探测失败被误判为登录失效")
	}
	if got.LoginState != store.LoginStateUnknown {
		t.Errorf("LoginState = %q, 期望 %q", got.LoginState, store.LoginStateUnknown)
	}
}

// TestAccountLoginProbeUnknownAccountDoesNotPanic 验证账号在探测前被删掉
// （正常竞态）不会 panic，只是安静地什么都不做。
func TestAccountLoginProbeUnknownAccountDoesNotPanic(t *testing.T) {
	st := newLoginCheckTestStore(t)
	check := func(context.Context, string) (string, error) { return store.LoginStateValid, nil }
	probe := &accountLoginProbe{st: st, check: check, log: slog.Default()}
	probe.ProbeNow(context.Background(), "不存在的账号")
}

// ---- bindingRoomStatusProbe：httpapi.RoomStatusProbe 的生产实现 ----

// TestBindingRoomStatusProbeWritesRoomStatus 验证 ProbeNow 探测成功时
// 把开播状态与主播身份写进对应绑定。
func TestBindingRoomStatusProbeWritesRoomStatus(t *testing.T) {
	st := newRoomStatusCheckTestStore(t)
	ctx := context.Background()
	owner, err := st.CreateUser(ctx, "张三", "密码123456", false)
	if err != nil {
		t.Fatalf("建用户报错: %v", err)
	}
	b := mustRoomStatusAccountAndBinding(t, st, owner.ID, "小号", "111")

	check := func(context.Context, string, string) (*api.RoomStatus, error) {
		return &api.RoomStatus{LiveStatus: api.LiveStatusLiving, AnchorUID: "9001", AnchorName: "主播"}, nil
	}
	probe := &bindingRoomStatusProbe{st: st, check: check, log: slog.Default()}
	probe.ProbeNow(ctx, b.ID)

	got, err := st.GetBindingByID(ctx, b.ID)
	if err != nil {
		t.Fatalf("查询报错: %v", err)
	}
	if got.LiveStatus != store.RoomLiveLiving {
		t.Errorf("LiveStatus = %q, 期望 %q", got.LiveStatus, store.RoomLiveLiving)
	}
	if got.AnchorUID != "9001" || got.AnchorName != "主播" {
		t.Errorf("主播身份 = uid=%q name=%q", got.AnchorUID, got.AnchorName)
	}
}

// TestBindingRoomStatusProbeDetectionFailureIsUnknownNotOffline 是本任务
// 自检项 (a) 在这个实现上的落点：探测失败必须写 unknown，不能写
// offline——绝不能因为接口失败就显示"未开播"。
func TestBindingRoomStatusProbeDetectionFailureIsUnknownNotOffline(t *testing.T) {
	st := newRoomStatusCheckTestStore(t)
	ctx := context.Background()
	owner, err := st.CreateUser(ctx, "张三", "密码123456", false)
	if err != nil {
		t.Fatalf("建用户报错: %v", err)
	}
	b := mustRoomStatusAccountAndBinding(t, st, owner.ID, "小号", "111")

	check := func(context.Context, string, string) (*api.RoomStatus, error) {
		return nil, errors.New("api: 风控校验失败")
	}
	probe := &bindingRoomStatusProbe{st: st, check: check, log: slog.Default()}
	probe.ProbeNow(ctx, b.ID)

	got, err := st.GetBindingByID(ctx, b.ID)
	if err != nil {
		t.Fatalf("查询报错: %v", err)
	}
	if got.LiveStatus == store.RoomLiveOffline {
		t.Error("探测失败被误判为「未开播」——这是把拿不到伪装成没开播")
	}
	if got.LiveStatus != store.RoomLiveUnknown {
		t.Errorf("LiveStatus = %q, 期望 %q", got.LiveStatus, store.RoomLiveUnknown)
	}
}

// TestBindingRoomStatusProbeUnknownBindingDoesNotPanic 验证绑定在探测前
// 被删掉（正常竞态）不会 panic。
func TestBindingRoomStatusProbeUnknownBindingDoesNotPanic(t *testing.T) {
	st := newRoomStatusCheckTestStore(t)
	check := func(context.Context, string, string) (*api.RoomStatus, error) {
		return &api.RoomStatus{}, nil
	}
	probe := &bindingRoomStatusProbe{st: st, check: check, log: slog.Default()}
	probe.ProbeNow(context.Background(), 999999)
}
