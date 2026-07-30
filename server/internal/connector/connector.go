// Package connector 定义直播平台接入的抽象。
package connector

import (
	"context"

	"github.com/MrZhongzq/MagicalDanmaku/server/internal/event"
)

// State 是连接状态。
type State string

// 全部连接状态。
const (
	StateIdle           State = "idle"            // 未开始
	StateResolving      State = "resolving"       // 正在获取房间与长连接信息
	StateConnecting     State = "connecting"      // 正在建立 WebSocket 并认证
	StateConnected      State = "connected"       // 已连接
	StateReconnecting   State = "reconnecting"    // 断线重连中
	StateRiskControlled State = "risk_controlled" // 触发风控，长退避中
	StateClosed         State = "closed"          // 已关闭
)

// Connector 是平台接入的唯一抽象点，一个实例对应一个直播间的事件流。
//
// 事件流是房间级的，与账号身份无关；需要账号身份的写操作定义在 Actions 中。
type Connector interface {
	// Run 阻塞运行直到 ctx 取消，内部自行处理重连。
	Run(ctx context.Context) error
	// Events 返回归一化事件流。Run 结束后该通道会被关闭。
	Events() <-chan event.Event
	// State 返回当前连接状态。
	State() State
}
