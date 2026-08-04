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
	// BlockUser 禁言用户。这是房间级操作（房管权限），与账号级的
	// BlacklistUser 是两条不同的能力——分开是 P5-6 的明确要求，见
	// BlacklistUser 的注释。
	BlockUser(ctx context.Context, req BlockRequest) error
	// UnblockUser 解除禁言。
	UnblockUser(ctx context.Context, roomID, uid string) error

	// BlacklistUser 把 uid 加入当前账号的黑名单（账号级操作，与直播间
	// 无关）。**不是禁言的一个时长档位**——原 C++ 项目与本项目早期版本
	// 都把"拉黑"实现成"禁言 720 小时"，用户明确纠正过两次：拉黑是账号
	// 操作，账号拉黑和直播间禁言是完全不同的两件事。
	BlacklistUser(ctx context.Context, uid string) error
	// UnblacklistUser 把 uid 移出当前账号的黑名单。
	UnblacklistUser(ctx context.Context, uid string) error
	// RelationAttribute 查询当前账号与 uid 的关系属性值，调用方用
	// bilibili/api.IsBlacklisted 判断是否已拉黑——这是"白捡"的状态回读
	// 接口，让界面能显示真实状态而不是"发了请求所以大概成功了"。
	RelationAttribute(ctx context.Context, uid string) (int, error)
	// Nickname 查询 uid 的昵称，用于免手填自动回填；失败时调用方应当
	// 把昵称留空处理，不能让这一步的失败拖累拉黑/禁言名单本身。
	Nickname(ctx context.Context, uid string) (string, error)
}
