package connector

import "context"

// SendDanmakuRequest 是一次发弹幕请求。
type SendDanmakuRequest struct {
	RoomID     string // 目标直播间号
	Text       string // 弹幕正文
	ReplyToUID string // @ 回复的目标 UID，可为空
}

// BlockRequest 是一次禁言请求。
type BlockRequest struct {
	RoomID string
	UID    string
	Hours  int // 禁言时长，单位小时
}

// Actions 是需要账号身份的写操作集合。
//
// 与 Connector 分离是因为：事件流是房间级的，与身份无关；
// 而写操作是账号级的，且需要支持多账号轮换发言。
type Actions interface {
	// SendDanmaku 发送弹幕。文本超长时会自动切分为多条依次发送。
	SendDanmaku(ctx context.Context, req SendDanmakuRequest) error
	// BlockUser 禁言用户。
	BlockUser(ctx context.Context, req BlockRequest) error
	// UnblockUser 解除禁言。
	UnblockUser(ctx context.Context, roomID, uid string) error
}
