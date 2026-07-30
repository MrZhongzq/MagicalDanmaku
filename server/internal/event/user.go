package event

// 大航海等级。
const (
	GuardNone     = 0 // 非舰队
	GuardGovernor = 1 // 总督
	GuardAdmiral  = 2 // 提督
	GuardCaptain  = 3 // 舰长
)

// Medal 是用户佩戴的粉丝勋章。
type Medal struct {
	Name       string // 勋章名
	Level      int    // 勋章等级
	AnchorUID  string // 勋章所属主播 UID
	AnchorName string // 勋章所属主播昵称
	RoomID     string // 勋章所属直播间号
	GuardLevel int    // 该勋章对应的大航海等级
	IsLighted  bool   // 勋章是否点亮
}

// User 是所有事件共用的用户信息。
// 抽成值对象以避免每个载荷重复十几个字段。
type User struct {
	UID         string // 用户 UID
	Username    string // 昵称
	AvatarURL   string // 头像地址，可能为空
	GuardLevel  int    // 本房间大航海等级，见 Guard* 常量
	UserLevel   int    // UL 等级
	WealthLevel int    // 荣耀等级
	Medal       *Medal // 佩戴的勋章，未佩戴时为 nil
	IsAdmin     bool   // 是否房管
}
