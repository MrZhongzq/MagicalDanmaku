package rules

import (
	"strings"

	"github.com/MrZhongzq/MagicalDanmaku/server/internal/event"
)

// VarsFromEvent 把事件展开为条件求值与模板渲染共用的取值表。
//
// 这是全项目唯一的字段展开处。条件里写 "user.guardLevel"、模板里写
// "{{.user.guardLevel}}"，两者指向同一份数据，杜绝两套字段名各自演化。
//
// 约定：值为零值的可选字段（如未佩戴勋章）不写入表中，
// 使 LookupPath 能区分「字段不存在」与「字段值为零」。
func VarsFromEvent(ev event.Event) map[string]any {
	v := map[string]any{
		"type":      string(ev.Type),
		"roomId":    ev.RoomID,
		"timestamp": ev.Timestamp.Unix(),
	}

	switch p := ev.Payload.(type) {
	case event.Danmaku:
		v["user"] = userVars(p.User)
		v["text"] = p.Text
		v["danmaku"] = map[string]any{
			"color":      p.Color,
			"isEmoticon": p.IsEmoticon,
			"replyToUid": p.ReplyToUID,
		}
	case event.SuperChat:
		v["user"] = userVars(p.User)
		v["text"] = p.Text
		v["superChat"] = map[string]any{
			"id": p.ID, "price": p.Price, "duration": int64(p.Duration),
		}
	case event.Gift:
		v["user"] = userVars(p.User)
		gm := map[string]any{
			"id": p.GiftID, "name": p.GiftName, "count": p.Count,
			"coinType": p.CoinType, "totalCoin": p.TotalCoin, "action": p.Action,
			"price": p.Price,
			// isBlindBox 不走「零值不写入」的惯例——它必须在普通礼物上也
			// 明确写成 false，否则常规礼物答谢规则想用
			// {field:"gift.isBlindBox", op:"eq", value:false} 把盲盒排除
			// 掉时，路径缺失会让条件在普通礼物上也匹配不到，规则整个失效。
			"isBlindBox": p.BlindBox != nil,
		}
		if p.BlindBox != nil {
			gm["blindBox"] = map[string]any{
				"name": p.BlindBox.Name, "giftId": p.BlindBox.GiftID,
				"price": p.BlindBox.Price, "tipPrice": p.BlindBox.TipPrice,
			}
		}
		v["gift"] = gm
	case event.GiftCombo:
		v["user"] = userVars(p.User)
		v["gift"] = map[string]any{
			"id": p.GiftID, "name": p.GiftName, "count": p.Count,
			"totalCoin": p.TotalCoin, "comboId": p.ComboID,
		}
	case event.GuardBuy:
		v["user"] = userVars(p.User)
		v["guard"] = map[string]any{
			"level": int64(p.GuardLevel), "name": p.GuardName,
			"count": int64(p.Count), "price": p.Price, "isRenew": p.IsRenew,
		}
	case event.UserEnter:
		v["user"] = userVars(p.User)
	case event.UserFollow:
		v["user"] = userVars(p.User)
	case event.UserShare:
		v["user"] = userVars(p.User)
	case event.UserLike:
		v["user"] = userVars(p.User)
	case event.UserBlocked:
		v["user"] = userVars(p.User)
	case event.RoomChange:
		v["room"] = map[string]any{
			"title": p.Title, "areaName": p.AreaName, "parentAreaName": p.ParentAreaName,
		}
	case event.RoomStatsUpdate:
		stats := map[string]any{}
		if p.Fans != nil {
			stats["fans"] = *p.Fans
		}
		if p.FansClub != nil {
			stats["fansClub"] = *p.FansClub
		}
		if p.Watched != nil {
			stats["watched"] = *p.Watched
		}
		if p.LikeCount != nil {
			stats["likeCount"] = *p.LikeCount
		}
		v["stats"] = stats
	case event.OnlineRankUpdate:
		v["rank"] = map[string]any{"count": int64(p.Count)}
	case event.Battle:
		v["battle"] = map[string]any{"subCommand": p.SubCommand}
		v["pk"] = pkVars(p, ev.RoomID)
	case event.VisitFromOpponent:
		v["user"] = userVars(p.User)
		v["visit"] = visitVars(p.OpponentRoomID, p.MatchedBy)
	case event.VisitToOpponent:
		v["user"] = userVars(p.User)
		v["visit"] = visitVars(p.OpponentRoomID, p.MatchedBy)
	case event.Unknown:
		v["unknown"] = map[string]any{"command": p.Command}
	}
	return v
}

// pkVars 把 Battle.Members 展开成 PK 播报用得到的模板变量。
//
// 「对面」的判定按既定裁决在规则层做——拿本绑定的房间号（ev.RoomID）
// 跟每个 member.RoomID 比对，绝不用 init_info/match_info，那两个字段
// 的真实语义是发起方/被匹配方（Task 4 的教训，opponent_link.go 的
// filterOpponents 是同一份裁决在连接器层的写法，这里照抄）。
//
// pkId/opponents 对任何 Battle 事件都写入，哪怕是空值/空列表——只有
// PK_INFO 与 PKPipeline 合成的快照事件（bilibili.PKOpponentSnapshotSubCommand）
// 才会真的带 Members，其余 PK_BATTLE_* CMD 上二者就是零值/空列表，
// 不是「不存在」，模板里判断「是否在 PK 中」应该用 pk.pkId 是否非空，
// 不该指望这两个字段缺失。opponent（单数，取第一个对手）是多人 PK
// 之外最常见场景（1v1）的便利视图，模板写
// {{.pk.opponent.uname}} 比 {{index .pk.opponents 0}} 自然得多。
func pkVars(b event.Battle, selfRoomID string) map[string]any {
	opponents := make([]map[string]any, 0, len(b.Members))
	for _, m := range b.Members {
		if m.RoomID == selfRoomID {
			continue
		}
		opponents = append(opponents, pkOpponentVars(m))
	}

	pk := map[string]any{
		"pkId":      b.PkID,
		"opponents": opponents,
	}
	// opponent 只在至少有一个对手时才写入，对齐「可选字段不写零值」
	// 的惯例——LookupPath 才能区分「还没进 PK / 没有对手」与
	// 「对手的某个字段恰好是零值」。
	if len(opponents) > 0 {
		pk["opponent"] = opponents[0]
	}
	// endTime 同样只在非零时才写入：只有 PK_INFO（mapPkInfo）和
	// PKPipeline 合成的快照事件带 pk_basic.end_time，其余 PK_BATTLE_*
	// CMD 上是零值，不是「真的是 0」。终审 Important-3：这个字段早在
	// PK_INFO 阶段就已经解析进 event.Battle.EndTime，但此前全仓没有
	// 任何地方读它——PKPipeline 现在用它驱动 PK_BATTLE_END 丢失时的
	// 超时兜底（watchEndTimeFallback，见 pk_pipeline.go），这里顺带把
	// 它暴露成模板变量，规则作者也能用它做「PK 快结束了」之类的判断。
	if b.EndTime != 0 {
		pk["endTime"] = b.EndTime
	}
	return pk
}

// pkOpponentVars 展开单个对手成员。Online/GuardTotal/GuardOnline 三个
// 指针字段为 nil 时不写入——它们是 PK 接通瞬间的一次性快照
// （FetchOpponentSnapshots），「快照还没就绪/接口失败」与「人数真的是
// 0」必须在这里保持可区分，不能把 nil 塌缩成 0 写进去。
func pkOpponentVars(m event.PkMember) map[string]any {
	o := map[string]any{
		"roomId":   m.RoomID,
		"uid":      m.UID,
		"uname":    m.Username,
		"votes":    m.Votes,
		"isWinner": m.IsWinner,
	}
	if m.Online != nil {
		o["online"] = *m.Online
	}
	if m.GuardTotal != nil {
		o["guardTotal"] = *m.GuardTotal
	}
	if m.GuardOnline != nil {
		o["guardOnline"] = *m.GuardOnline
	}
	return o
}

// visitVars 展开串门信号事件共有的部分（VisitFromOpponent/
// VisitToOpponent 除了 User 之外的字段形状完全相同）。
func visitVars(opponentRoomID string, matchedBy event.VisitMatchedBy) map[string]any {
	return map[string]any{
		"opponentRoomId": opponentRoomID,
		"matchedBy":      string(matchedBy),
	}
}

// userVars 展开用户信息。零值的可选字段不写入。
func userVars(u event.User) map[string]any {
	m := map[string]any{
		"uid":         u.UID,
		"username":    u.Username,
		"guardLevel":  u.GuardLevel,
		"userLevel":   u.UserLevel,
		"wealthLevel": u.WealthLevel,
		"isAdmin":     u.IsAdmin,
	}
	if u.AvatarURL != "" {
		m["avatarUrl"] = u.AvatarURL
	}
	if u.Medal != nil {
		m["medal"] = map[string]any{
			"name":       u.Medal.Name,
			"level":      u.Medal.Level,
			"roomId":     u.Medal.RoomID,
			"anchorUid":  u.Medal.AnchorUID,
			"guardLevel": u.Medal.GuardLevel,
			"isLighted":  u.Medal.IsLighted,
		}
	}
	return m
}

// Variable 是一条可用于条件与模板的变量。
// json tag 不能省：这个结构体会经 /api/meta/variables 直接吐给前端，
// 不带 tag 的话字段名是大写的 Path/Label/Optional，与其余 /api/meta/*
// 接口（metaItem 用的是 value/label）风格不一致，前端只能跟着迁就。
type Variable struct {
	Path     string `json:"path"`     // 点分路径，如 "user.medal.isLighted"，与 LookupPath 的参数同形
	Label    string `json:"label"`    // 中文说明，供前端下拉框展示
	Optional bool   `json:"optional"` // 可能不存在（如未佩戴粉丝牌时没有 medal.*），配条件时仍可选用
}

// commonVariables 是任意事件都会产出的公共字段。
//
// count/users/gifts 是例外：它们不是 VarsFromEvent 本身产出的，而是
// 合并窗口（见 aggregate.go 的 mergeBuckets/PassthroughTrigger）算完之后
// 才补进 Vars 的，所以标 Optional——用真实事件跑 VarsFromEvent 永远看
// 不到它们，但它们是配置聚合规则、写礼物答谢模板时用户真实用得到的路径。
//
// **这是 TestVariableCatalogMatchesVarsFromEvent 的天然盲区**：那条测试
// 只跟 VarsFromEvent 的实际产出对照，而聚合期变量本来就不在那边产出，
// 全靠这里人工标 Optional 混过去——加一个聚合期变量却忘了在这里补一条，
// 测试不会报红，只会在 /api/meta/variables 和条件构建器下拉里悄悄
// 少一项（gifts 就是这么漏掉的，直到全批次终审才补上）。新增聚合期
// 变量时记得回来加一行，并检查 mergeBuckets/PassthroughTrigger 两处
// 是否都填了它。
//
// blindBox.* 是这个盲区里最新的一批：它们只在 AggregateByBlindBox 分组
// 触发、mergeBuckets 结算出盈亏后才存在（PassthroughTrigger 不填——
// 盲盒盈亏统计的前提就是配了盲盒聚合，没配聚合的直通触发本来就不该有
// 这套字段），与 gifts 同一类聚合期变量，同样只靠这里人工登记来对
// /api/meta/variables 和条件构建器生效，检查方式见 vars_test.go 里的
// TestVariableCatalogHasBlindBoxFields。
var commonVariables = []Variable{
	{Path: "type", Label: "事件类型"},
	{Path: "roomId", Label: "直播间号"},
	{Path: "timestamp", Label: "事件时间戳（Unix 秒）"},
	{Path: "count", Label: "合并窗口内的事件数量（仅聚合规则触发时存在）", Optional: true},
	{Path: "users", Label: "合并窗口内涉及的用户昵称列表（仅聚合规则触发时存在）", Optional: true},
	{Path: "gifts", Label: "合并窗口内涉及的礼物名列表，去重（仅聚合规则触发时存在）", Optional: true},
	{Path: "blindBox.name", Label: "本轮盲盒名称（仅按盲盒聚合触发时存在）", Optional: true},
	{Path: "blindBox.count", Label: "本轮盲盒开出次数（仅按盲盒聚合触发时存在）", Optional: true},
	{Path: "blindBox.cost", Label: "本轮盲盒投入，1/100 电池（仅按盲盒聚合触发时存在）", Optional: true},
	{Path: "blindBox.gain", Label: "本轮盲盒产出，1/100 电池（仅按盲盒聚合触发时存在）", Optional: true},
	{Path: "blindBox.profit", Label: "本轮盲盒盈亏=产出-投入，1/100 电池，可为负（仅按盲盒聚合触发时存在）", Optional: true},
	{Path: "blindBox.costYuan", Label: "本轮盲盒投入（元，展示用字符串；仅按盲盒聚合触发时存在）", Optional: true},
	{Path: "blindBox.gainYuan", Label: "本轮盲盒产出（元，展示用字符串；仅按盲盒聚合触发时存在）", Optional: true},
	{Path: "blindBox.profitYuan", Label: "本轮盲盒盈亏（元，展示用字符串，可为负；仅按盲盒聚合触发时存在）", Optional: true},
	{Path: "blindBox.profitAbsYuan", Label: "本轮盲盒盈亏的绝对值（元，展示用字符串，恒不为负，专供播报模板" +
		"搭配「赚了/亏了」这类已经带方向的措辞、避免双重否定；仅按盲盒聚合触发时存在）", Optional: true},
}

// userVariables 是 userVars 展开产出的字段，路径不带前缀——
// 使用处按事件类型套上 "user." 前缀（见 withPrefix）。
//
// 这份清单必须和 userVars 函数的实现同步：那边加一个字段，这边就要
// 加一条，否则要么用户配不出用到新字段的条件，要么清单里有一条
// VarsFromEvent 从不产出的死路径。TestVariableCatalogMatchesVarsFromEvent
// 会在两者漂开时报红。
var userVariables = []Variable{
	{Path: "uid", Label: "用户 UID"},
	{Path: "username", Label: "用户昵称"},
	{Path: "guardLevel", Label: "大航海等级（0 无/1 总督/2 提督/3 舰长）"},
	{Path: "userLevel", Label: "用户等级（UL）"},
	{Path: "wealthLevel", Label: "荣耀等级"},
	{Path: "isAdmin", Label: "是否房管"},
	{Path: "avatarUrl", Label: "头像地址", Optional: true},
	{Path: "medal.name", Label: "粉丝勋章名称", Optional: true},
	{Path: "medal.level", Label: "粉丝勋章等级", Optional: true},
	{Path: "medal.roomId", Label: "粉丝勋章所属直播间号", Optional: true},
	{Path: "medal.anchorUid", Label: "粉丝勋章所属主播 UID", Optional: true},
	{Path: "medal.guardLevel", Label: "粉丝勋章对应的大航海等级", Optional: true},
	{Path: "medal.isLighted", Label: "粉丝勋章是否点亮", Optional: true},
}

// withPrefix 给一组变量的 Path 统一加前缀，返回新切片（不改原切片）。
func withPrefix(prefix string, vars []Variable) []Variable {
	out := make([]Variable, len(vars))
	for i, v := range vars {
		out[i] = Variable{Path: prefix + "." + v.Path, Label: v.Label, Optional: v.Optional}
	}
	return out
}

// VariableCatalog 返回按事件类型分组的变量清单，供前端条件构建器/模板
// 编辑器渲染下拉框用。
//
// **它与 VarsFromEvent 必须一起改。** 前端的条件构建器靠它渲染下拉，
// 清单漏了某个路径，用户就配不出用到那个路径的条件；清单里有而
// VarsFromEvent 不产出，用户会配出永远不匹配且不报错的条件。
// TestVariableCatalogMatchesVarsFromEvent 用真实事件跑 VarsFromEvent，
// 把实际产出的键路径与这里声明的逐条对照，两边漂开就会红。
//
// common 是任意事件类型都有的字段（type/roomId/timestamp，以及只在
// 合并窗口聚合后才有的 count/users），不在 byEvent 的各分组里重复。
// byEvent 只覆盖 VarsFromEvent 的 switch 里实际有分支、会产出额外字段
// 的事件类型；像 live_start/live_stop/super_chat_delete/manual 这些
// switch 里没有分支的类型不出现在 byEvent 里——它们除了公共字段之外
// 没有别的变量可选。
func VariableCatalog() (common []Variable, byEvent map[event.Type][]Variable) {
	user := withPrefix("user", userVariables)

	byEvent = map[event.Type][]Variable{
		event.TypeDanmaku: append(append([]Variable{}, user...), []Variable{
			{Path: "text", Label: "弹幕正文"},
			{Path: "danmaku.color", Label: "弹幕颜色（十六进制，如 #ffffff）"},
			{Path: "danmaku.isEmoticon", Label: "是否为表情弹幕"},
			{Path: "danmaku.replyToUid", Label: "被 @ 回复的用户 UID"},
		}...),
		event.TypeSuperChat: append(append([]Variable{}, user...), []Variable{
			{Path: "text", Label: "醒目留言正文"},
			{Path: "superChat.id", Label: "醒目留言 ID"},
			{Path: "superChat.price", Label: "价格（元）"},
			{Path: "superChat.duration", Label: "展示秒数"},
		}...),
		event.TypeGift: append(append([]Variable{}, user...), []Variable{
			{Path: "gift.id", Label: "礼物 ID"},
			{Path: "gift.name", Label: "礼物名称"},
			{Path: "gift.count", Label: "礼物数量"},
			{Path: "gift.coinType", Label: "瓜子类型（gold 金瓜子 / silver 银瓜子）"},
			{Path: "gift.totalCoin", Label: "总价值（瓜子）"},
			{Path: "gift.action", Label: "动作描述（如「投喂」）"},
			{Path: "gift.price", Label: "单价，1/100 电池；盲盒场景下为爆出礼物的价值"},
			{Path: "gift.isBlindBox", Label: "是否为盲盒礼物"},
			{Path: "gift.blindBox.name", Label: "盲盒名称（如「幸运盲盒」），仅盲盒礼物存在", Optional: true},
			{Path: "gift.blindBox.giftId", Label: "盲盒自身的礼物 ID，仅盲盒礼物存在", Optional: true},
			{Path: "gift.blindBox.price", Label: "盲盒售价（单个），1/100 电池，仅盲盒礼物存在", Optional: true},
			{Path: "gift.blindBox.tipPrice", Label: "爆出礼物的价值，1/100 电池，仅盲盒礼物存在", Optional: true},
		}...),
		event.TypeGiftCombo: append(append([]Variable{}, user...), []Variable{
			{Path: "gift.id", Label: "礼物 ID"},
			{Path: "gift.name", Label: "礼物名称"},
			{Path: "gift.count", Label: "礼物数量（连击汇总）"},
			{Path: "gift.totalCoin", Label: "总价值（瓜子）"},
			{Path: "gift.comboId", Label: "连击 ID"},
		}...),
		event.TypeGuardBuy: append(append([]Variable{}, user...), []Variable{
			{Path: "guard.level", Label: "大航海等级（1 总督/2 提督/3 舰长）"},
			{Path: "guard.name", Label: "大航海名称（如「舰长」）"},
			{Path: "guard.count", Label: "购买月数"},
			{Path: "guard.price", Label: "单价（金瓜子）"},
			{Path: "guard.isRenew", Label: "是否为续费（false 为新购）"},
		}...),
		event.TypeUserEnter:   append([]Variable{}, user...),
		event.TypeUserFollow:  append([]Variable{}, user...),
		event.TypeUserShare:   append([]Variable{}, user...),
		event.TypeUserLike:    append([]Variable{}, user...),
		event.TypeUserBlocked: append([]Variable{}, user...),
		event.TypeRoomChange: {
			{Path: "room.title", Label: "直播间标题"},
			{Path: "room.areaName", Label: "分区名称"},
			{Path: "room.parentAreaName", Label: "父分区名称"},
		},
		event.TypeRoomStatsUpdate: {
			{Path: "stats.fans", Label: "粉丝数", Optional: true},
			{Path: "stats.fansClub", Label: "粉丝团人数", Optional: true},
			{Path: "stats.watched", Label: "累计看过人数", Optional: true},
			{Path: "stats.likeCount", Label: "点赞数", Optional: true},
		},
		event.TypeOnlineRankUpdate: {
			{Path: "rank.count", Label: "高能榜总人数（未知时为 -1）"},
		},
		event.TypeBattle: {
			{Path: "battle.subCommand", Label: "PK 原始 CMD 名（P0 只归一化不解释；" +
				"PKPipeline 合成的对面快照就绪事件固定为 PK_OPPONENT_SNAPSHOT，" +
				"可用作 when 条件精确定位「PK 接通的这一瞬间」，避免每个 PK_BATTLE_* " +
				"子状态都触发一遍播报）"},
			{Path: "pk.pkId", Label: "这场 PK 的 ID，来自 pk_basic.pk_id；不在 PK 中或未携带明细的 CMD 上为空"},
			{Path: "pk.endTime", Label: "这场 PK 预计结束时间（Unix 秒，来自 pk_basic.end_time）；" +
				"未携带明细的 CMD 上不存在，不要跟 0 混为一谈", Optional: true},
			{Path: "pk.opponents", Label: "全部对手列表（多人 PK 可能不止一个），" +
				"按本绑定房间号与 member.roomId 比对筛出——不是发起方/被匹配方"},
			{Path: "pk.opponent.roomId", Label: "对手（取第一个）直播间号", Optional: true},
			{Path: "pk.opponent.uid", Label: "对手主播 UID", Optional: true},
			{Path: "pk.opponent.uname", Label: "对手主播昵称", Optional: true},
			{Path: "pk.opponent.votes", Label: "对手当前票数", Optional: true},
			{Path: "pk.opponent.isWinner", Label: "对手是否是当前领先方", Optional: true},
			{Path: "pk.opponent.online", Label: "对手直播间人数（PK 接通瞬间的一次性快照，" +
				"接口失败或快照还未就绪时不存在，不要跟 0 混为一谈）", Optional: true},
			{Path: "pk.opponent.guardTotal", Label: "对手大航海总数（同上，一次性快照）", Optional: true},
			{Path: "pk.opponent.guardOnline", Label: "对手大航海在线数（同上，一次性快照）", Optional: true},
		},
		event.TypeVisitFromOpponent: append(append([]Variable{}, user...), []Variable{
			{Path: "visit.opponentRoomId", Label: "来访者所属的对手直播间号"},
			{Path: "visit.matchedBy", Label: "判定依据：fan_medal 粉丝勋章 / audience 观众集合" +
				"（PK 期间累计，只增不减） / energy_rank 对面高能榜过去 10 秒滚动窗口（P5-5 7c 新增，会过期）"},
		}...),
		event.TypeVisitToOpponent: append(append([]Variable{}, user...), []Variable{
			{Path: "visit.opponentRoomId", Label: "我方观众跑去的那个对手直播间号"},
			{Path: "visit.matchedBy", Label: "判定依据：fan_medal 粉丝勋章 / audience 观众集合"},
		}...),
		event.TypeUnknown: {
			{Path: "unknown.command", Label: "未识别事件的原始 CMD 名"},
		},
	}
	return commonVariables, byEvent
}

// LookupPath 按点分路径取值，如 "user.medal.level"。
// 路径不存在时返回 (nil, false)。
func LookupPath(vars map[string]any, path string) (any, bool) {
	if path == "" {
		return nil, false
	}
	parts := strings.Split(path, ".")

	var cur any = vars
	for _, p := range parts {
		m, ok := cur.(map[string]any)
		if !ok {
			return nil, false
		}
		cur, ok = m[p]
		if !ok {
			return nil, false
		}
	}
	return cur, true
}

// MergeVars 把 src 逐字段合并进 dst。
//
// 合并规则：dst 中缺失的键直接补上；已存在的键，只有当 dst 的值为
// 零值而 src 非零值时才覆盖。嵌套的 map 递归合并。
//
// 这条规则解决了 P0 联调发现的进场重复问题：ENTRY_EFFECT 只有 UID
// 没有昵称，INTERACT_WORD_V2 信息完整，两者合并后得到完整记录。
func MergeVars(dst, src map[string]any) {
	for k, sv := range src {
		dv, exists := dst[k]
		if !exists {
			dst[k] = sv
			continue
		}
		// 嵌套 map 递归合并
		if dm, ok := dv.(map[string]any); ok {
			if sm, ok := sv.(map[string]any); ok {
				MergeVars(dm, sm)
				continue
			}
		}
		if isZeroValue(dv) && !isZeroValue(sv) {
			dst[k] = sv
		}
	}
}

// isZeroValue 判断是否为「空」值：空串、0、false、nil。
func isZeroValue(v any) bool {
	switch t := v.(type) {
	case nil:
		return true
	case string:
		return t == ""
	case bool:
		return !t
	case int:
		return t == 0
	case int64:
		return t == 0
	case float64:
		return t == 0
	default:
		return false
	}
}
