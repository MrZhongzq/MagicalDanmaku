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
	CoinType  string    // "gold" 金瓜子 / "silver" 银瓜子
	TotalCoin int64     // 总价值，单位瓜子
	Price     int64     // 单价，单位瓜子；盲盒场景下恒等于 BlindBox.TipPrice（爆出礼物的价值）
	Action    string    // 动作描述，如「投喂」
	BlindBox  *BlindBox // 盲盒附加信息，为 nil 表示这不是盲盒
}

// BlindBox 是盲盒礼物的附加信息。为 nil 表示这不是盲盒。
//
// **金额单位都是 1/100 电池**——B 站原始报文就是这个单位
// （幸运盲盒 50 电池，报文里是 5000）。存原始整数、只在展示层
// 除以 100，中间用浮点算钱会累积误差。
type BlindBox struct {
	Name     string // 盲盒名称，如「幸运盲盒」
	GiftID   int64  // 盲盒自身的礼物 ID
	Price    int64  // 盲盒售价（单个），1/100 电池；Gift.TotalCoin == Price * Gift.Count
	TipPrice int64  // 爆出礼物的价值，1/100 电池；恒等于 Gift.Price
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

// PkMember 是一场 PK 里的一方。
//
// 带 RoomID 是为了让规则层能拿 Event.RoomID 比对出「自己」和「对面」——
// 协议层不知道调用方绑定的是哪个房间，这个过滤故意不在这里做。
type PkMember struct {
	RoomID   string
	UID      string
	Username string
	Face     string
	Votes    int64
	IsWinner bool
}

// Battle 是 PK 大乱斗相关事件。
//
// Members 全量来自 PK_INFO.data.members[]，原样交出、一个不过滤；
// 其余 PK_BATTLE_* 系列 CMD 不携带参战方明细，只归一化 SubCommand，
// 解释权留给 P6（见 Raw）。
type Battle struct {
	SubCommand string     // 原始 CMD 名，如 "PK_BATTLE_START_NEW"
	PkID       string     // 这场 PK 的 ID，来自 pk_basic.pk_id
	Members    []PkMember // 参战方，可能多于两方；仅 PK_INFO 携带
	StartTime  int64      // 秒级时间戳，来自 pk_basic.start_time
	EndTime    int64      // 秒级时间戳，来自 pk_basic.end_time
}

// Unknown 是未识别的 CMD。
type Unknown struct {
	Command string // 原始 CMD 名
}

// VisitMatchedBy 标记「串门」判定命中的是哪一个判据，供规则层/排查
// 还原判断过程——同一个方向可能同时命中两个判据，这里记的是分支
// 命中优先级下实际生效的那一个，不是"两个都试了一遍"的日志。
type VisitMatchedBy string

const (
	VisitMatchedByFanMedal VisitMatchedBy = "fan_medal" // 戴着对方主播的粉丝勋章
	VisitMatchedByAudience VisitMatchedBy = "audience"  // 命中 PK 期间维护的观众集合
)

// VisitFromOpponent 表示 PK 期间对面房间的人（观众或主播本人）跑来了
// 本房间——方向 A，欢迎语气。触发它的原始事件（进房/弹幕/送礼……）
// 通过 Event.Raw 保留，OpponentRoomID 是此人所属的对手房间号（多人 PK
// 下可能有好几个对手，需要精确到具体是哪一个）。
type VisitFromOpponent struct {
	User           User
	OpponentRoomID string
	MatchedBy      VisitMatchedBy
}

// VisitToOpponent 表示 PK 期间本房间的观众跑去了对面房间——方向 B，
// 原 C++ 对应位置的注释是「自己这边过去送礼物，居心何在！」，是提示/
// 警示语气，不是欢迎，跟方向 A 语义相反。OpponentRoomID 是此人跑去的
// 那个对手房间号。
type VisitToOpponent struct {
	User           User
	OpponentRoomID string
	MatchedBy      VisitMatchedBy
}

func (Danmaku) isPayload()           {}
func (Gift) isPayload()              {}
func (GiftCombo) isPayload()         {}
func (GuardBuy) isPayload()          {}
func (SuperChat) isPayload()         {}
func (SuperChatDelete) isPayload()   {}
func (UserEnter) isPayload()         {}
func (UserFollow) isPayload()        {}
func (UserShare) isPayload()         {}
func (UserLike) isPayload()          {}
func (LiveStart) isPayload()         {}
func (LiveStop) isPayload()          {}
func (RoomChange) isPayload()        {}
func (UserBlocked) isPayload()       {}
func (OnlineRankUpdate) isPayload()  {}
func (RoomStatsUpdate) isPayload()   {}
func (Battle) isPayload()            {}
func (Unknown) isPayload()           {}
func (VisitFromOpponent) isPayload() {}
func (VisitToOpponent) isPayload()   {}
