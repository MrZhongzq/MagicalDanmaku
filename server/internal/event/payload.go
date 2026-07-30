package event

// Payload 是所有具体事件载荷的标记接口。
type Payload interface {
	isPayload()
}

// Danmaku 是一条弹幕。
type Danmaku struct {
	User        User
	Text        string // 弹幕正文
	Color       string // 十六进制颜色，形如 "#ffffff"
	IsEmoticon  bool   // 是否为表情弹幕
	ReplyToUID  string // @ 回复的目标 UID，无则为空
	ReplyToName string // @ 回复的目标昵称，无则为空
}

// Gift 是一次送礼。
type Gift struct {
	User      User
	GiftID    int64
	GiftName  string
	Count     int64
	CoinType  string // "gold" 金瓜子 / "silver" 银瓜子
	TotalCoin int64  // 总价值，单位瓜子
	Action    string // 动作描述，如「投喂」
}

// GiftCombo 是礼物连击汇总。
type GiftCombo struct {
	User      User
	GiftID    int64
	GiftName  string
	Count     int64
	ComboID   string
	TotalCoin int64
}

// GuardBuy 是一次上舰或续费。
type GuardBuy struct {
	User       User
	GuardLevel int    // 见 Guard* 常量
	GuardName  string // 如「舰长」
	Count      int    // 购买月数
	Price      int64  // 单位金瓜子
	IsRenew    bool   // true 为续费，false 为新购
}

// SuperChat 是一条醒目留言。
type SuperChat struct {
	User     User
	ID       int64
	Text     string
	Price    int64 // 单位元
	Duration int   // 展示秒数
}

// SuperChatDelete 表示若干醒目留言被删除。
type SuperChatDelete struct {
	IDs []int64
}

// UserEnter 表示用户进入直播间。
type UserEnter struct{ User User }

// UserFollow 表示用户关注了主播。
type UserFollow struct{ User User }

// UserShare 表示用户分享了直播间。
type UserShare struct{ User User }

// UserLike 表示用户点赞。
type UserLike struct{ User User }

// LiveStart 表示开播。
type LiveStart struct{}

// LiveStop 表示下播。
type LiveStop struct{}

// RoomChange 表示房间标题或分区变更。
type RoomChange struct {
	Title          string
	AreaID         string
	AreaName       string
	ParentAreaID   string
	ParentAreaName string
}

// UserBlocked 表示有用户被禁言。
type UserBlocked struct {
	User         User
	OperatorName string // 操作者昵称，可能为空
}

// RankUser 是高能榜上的一位用户。
type RankUser struct {
	User  User
	Rank  int
	Score string
}

// OnlineRankUpdate 是高能榜变化。
type OnlineRankUpdate struct {
	Count int        // 高能榜总人数，未知时为 -1
	Top   []RankUser // 榜单前若干名，可能为空
}

// RoomStatsUpdate 是房间统计数据变化。
// 指针字段为 nil 表示本次事件未携带该数据。
type RoomStatsUpdate struct {
	Fans      *int64 // 粉丝数
	FansClub  *int64 // 粉丝团人数
	Watched   *int64 // 累计看过人数
	LikeCount *int64 // 点赞数
}

// Battle 是 PK 大乱斗相关事件，P0 只归一化不解释。
type Battle struct {
	SubCommand string // 原始 CMD 名，如 "PK_BATTLE_START_NEW"
}

// Unknown 是未识别的 CMD。
type Unknown struct {
	Command string // 原始 CMD 名
}

func (Danmaku) isPayload()          {}
func (Gift) isPayload()             {}
func (GiftCombo) isPayload()        {}
func (GuardBuy) isPayload()         {}
func (SuperChat) isPayload()        {}
func (SuperChatDelete) isPayload()  {}
func (UserEnter) isPayload()        {}
func (UserFollow) isPayload()       {}
func (UserShare) isPayload()        {}
func (UserLike) isPayload()         {}
func (LiveStart) isPayload()        {}
func (LiveStop) isPayload()         {}
func (RoomChange) isPayload()       {}
func (UserBlocked) isPayload()      {}
func (OnlineRankUpdate) isPayload() {}
func (RoomStatsUpdate) isPayload()  {}
func (Battle) isPayload()           {}
func (Unknown) isPayload()          {}
