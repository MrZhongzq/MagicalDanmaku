// Package account 定义账号与「账号-直播间」绑定。
//
// 账号不是可互换的资源，而是各有职责的参与者：主播号可能只做统计与
// 房管而不发言，小号负责欢迎答谢。因此本包不提供轮换或 fallback——
// 指定账号失效就报错，由使用者处理。
package account

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/MrZhongzq/MagicalDanmaku/server/internal/connector"
	"github.com/MrZhongzq/MagicalDanmaku/server/internal/connector/bilibili/auth"
	"github.com/MrZhongzq/MagicalDanmaku/server/internal/ratelimit"
)

// ErrNoAccount 表示绑定缺少账号。
var ErrNoAccount = errors.New("account: 绑定未指定账号")

// Account 是一个已登录账号，可连接多个直播间。
//
// Limiter 挂在账号上而非绑定上：B 站的风控按账号计算，同一账号的全部
// 直播间必须共享发送节奏，不同账号之间则完全独立。
type Account struct {
	Name    string
	Session *auth.Session
	Limiter ratelimit.Limiter
}

// New 创建账号。interval 为该账号全部直播间共享的最小发送间隔。
func New(name string, sess *auth.Session, interval time.Duration) *Account {
	return &Account{
		Name:    name,
		Session: sess,
		Limiter: ratelimit.NewInterval(interval),
	}
}

// Binding 是「账号-直播间」组合，P2 的运行单元。
//
// 同一直播间被两个账号连接时是两个独立 Binding，各自有独立的连接、
// 规则集与冷却状态，互不知道对方存在。
type Binding struct {
	Account *Account
	RoomID  string
	Actions connector.Actions
}

// Label 返回用于日志的标识，形如 "主播号@1706666491"。
func (b *Binding) Label() string {
	name := "(未指定账号)"
	if b.Account != nil {
		name = b.Account.Name
	}
	return name + "@" + b.RoomID
}

// SendDanmaku 以本绑定的账号身份，向本绑定的直播间发送弹幕。
func (b *Binding) SendDanmaku(ctx context.Context, text string) error {
	if b.Account == nil {
		return ErrNoAccount
	}
	if err := b.Account.Limiter.Wait(ctx); err != nil {
		return err
	}
	err := b.Actions.SendDanmaku(ctx, connector.SendDanmakuRequest{
		RoomID: b.RoomID,
		Text:   text,
	})
	if err != nil {
		return fmt.Errorf("%s 发送弹幕失败: %w", b.Label(), err)
	}
	return nil
}

// Block 以本绑定的账号身份，在本绑定的直播间禁言用户。
func (b *Binding) Block(ctx context.Context, uid string, hours int) error {
	if b.Account == nil {
		return ErrNoAccount
	}
	if err := b.Account.Limiter.Wait(ctx); err != nil {
		return err
	}
	err := b.Actions.BlockUser(ctx, connector.BlockRequest{
		RoomID: b.RoomID,
		UID:    uid,
		Hours:  hours,
	})
	if err != nil {
		return fmt.Errorf("%s 禁言 %s 失败: %w", b.Label(), uid, err)
	}
	return nil
}

// Unblock 以本绑定的账号身份，在本绑定的直播间解除禁言。
func (b *Binding) Unblock(ctx context.Context, uid string) error {
	if b.Account == nil {
		return ErrNoAccount
	}
	if err := b.Account.Limiter.Wait(ctx); err != nil {
		return err
	}
	if err := b.Actions.UnblockUser(ctx, b.RoomID, uid); err != nil {
		return fmt.Errorf("%s 解除禁言 %s 失败: %w", b.Label(), uid, err)
	}
	return nil
}
