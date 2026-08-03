package rules

import (
	"testing"
	"time"

	"github.com/MrZhongzq/MagicalDanmaku/server/internal/event"
)

func danmakuEvent() event.Event {
	return event.Event{
		Type:      event.TypeDanmaku,
		RoomID:    "1706666491",
		Timestamp: time.Unix(1753920000, 0),
		Payload: event.Danmaku{
			User: event.User{
				UID: "12345678", Username: "路人甲",
				GuardLevel: 3, UserLevel: 18, WealthLevel: 7, IsAdmin: true,
				Medal: &event.Medal{Name: "真yu中", Level: 24, RoomID: "999"},
			},
			Text:  "主播晚上好",
			Color: "#ffffff",
		},
	}
}

func TestVarsFromDanmaku(t *testing.T) {
	v := VarsFromEvent(danmakuEvent())

	cases := map[string]any{
		"type":             "danmaku",
		"roomId":           "1706666491",
		"text":             "主播晚上好",
		"user.uid":         "12345678",
		"user.username":    "路人甲",
		"user.guardLevel":  3,
		"user.userLevel":   18,
		"user.wealthLevel": 7,
		"user.isAdmin":     true,
		"user.medal.name":  "真yu中",
		"user.medal.level": 24,
	}
	for path, want := range cases {
		got, ok := LookupPath(v, path)
		if !ok {
			t.Errorf("路径 %q 不存在", path)
			continue
		}
		if got != want {
			t.Errorf("%s = %v (%T), 期望 %v (%T)", path, got, got, want, want)
		}
	}
}

func TestVarsMissingMedalIsAbsent(t *testing.T) {
	ev := danmakuEvent()
	d := ev.Payload.(event.Danmaku)
	d.User.Medal = nil
	ev.Payload = d

	v := VarsFromEvent(ev)
	if _, ok := LookupPath(v, "user.medal.name"); ok {
		t.Error("未佩戴勋章时 user.medal.name 不应存在")
	}
	// 但 user.username 仍应存在
	if _, ok := LookupPath(v, "user.username"); !ok {
		t.Error("user.username 应当存在")
	}
}

func TestVarsFromGift(t *testing.T) {
	ev := event.Event{
		Type: event.TypeGift, RoomID: "1", Timestamp: time.Unix(1700000000, 0),
		Payload: event.Gift{
			User:     event.User{UID: "9", Username: "土豪"},
			GiftID:   31531,
			GiftName: "小花花",
			Count:    10,
			CoinType: "gold", TotalCoin: 10000, Action: "投喂",
		},
	}
	v := VarsFromEvent(ev)
	cases := map[string]any{
		"gift.name":      "小花花",
		"gift.count":     int64(10),
		"gift.coinType":  "gold",
		"gift.totalCoin": int64(10000),
		"user.username":  "土豪",
	}
	for path, want := range cases {
		got, _ := LookupPath(v, path)
		if got != want {
			t.Errorf("%s = %v (%T), 期望 %v (%T)", path, got, got, want, want)
		}
	}
}

// TestVarsFromGiftBlindBox 钉住盲盒礼物暴露的判据与附加字段：
// gift.isBlindBox 供常规礼物答谢规则用 when 条件排除盲盒
// （{field: "gift.isBlindBox", op: "eq", value: false}），
// gift.blindBox.* 供「只答谢心动盲盒」这类条件使用。
func TestVarsFromGiftBlindBox(t *testing.T) {
	ev := event.Event{
		Type: event.TypeGift, RoomID: "1", Timestamp: time.Unix(1700000000, 0),
		Payload: event.Gift{
			User:      event.User{UID: "9", Username: "土豪"},
			GiftName:  "星光铃铛",
			Count:     1,
			CoinType:  "gold",
			TotalCoin: 5000,
			Price:     5200,
			BlindBox:  &event.BlindBox{Name: "幸运盲盒", GiftID: 35206, Price: 5000, TipPrice: 5200},
		},
	}
	v := VarsFromEvent(ev)
	cases := map[string]any{
		"gift.price":             int64(5200),
		"gift.isBlindBox":        true,
		"gift.blindBox.name":     "幸运盲盒",
		"gift.blindBox.giftId":   int64(35206),
		"gift.blindBox.price":    int64(5000),
		"gift.blindBox.tipPrice": int64(5200),
	}
	for path, want := range cases {
		got, ok := LookupPath(v, path)
		if !ok {
			t.Errorf("路径 %q 不存在", path)
			continue
		}
		if got != want {
			t.Errorf("%s = %v (%T), 期望 %v (%T)", path, got, got, want, want)
		}
	}
}

// TestVarsFromGiftNotBlindBoxIsBlindBoxFalse 钉住 gift.isBlindBox 在非
// 盲盒礼物上也必须存在且为 false——它不能走「零值不写入」的惯例，
// 否则 when 条件 {field:"gift.isBlindBox", op:"eq", value:false} 在
// 普通礼物上会因路径缺失而永远匹配不到，常规礼物答谢规则就排除不掉
// 盲盒之外的东西（本该总是命中却总是不命中）。
func TestVarsFromGiftNotBlindBoxIsBlindBoxFalse(t *testing.T) {
	ev := event.Event{
		Type: event.TypeGift, RoomID: "1", Timestamp: time.Unix(1700000000, 0),
		Payload: event.Gift{
			User:      event.User{UID: "9", Username: "土豪"},
			GiftName:  "小花花",
			Count:     1,
			TotalCoin: 100,
		},
	}
	v := VarsFromEvent(ev)
	isBlind, ok := LookupPath(v, "gift.isBlindBox")
	if !ok {
		t.Fatal("gift.isBlindBox 应始终存在，即使不是盲盒")
	}
	if isBlind != false {
		t.Errorf("gift.isBlindBox = %v，非盲盒礼物应为 false", isBlind)
	}
	if _, ok := LookupPath(v, "gift.blindBox.name"); ok {
		t.Error("非盲盒礼物不应存在 gift.blindBox.name")
	}
}

func TestVarsFromGuardBuy(t *testing.T) {
	ev := event.Event{
		Type: event.TypeGuardBuy, RoomID: "1", Timestamp: time.Unix(1700000000, 0),
		Payload: event.GuardBuy{
			User:       event.User{UID: "9", Username: "新舰长"},
			GuardLevel: 3, GuardName: "舰长", Count: 1, Price: 198000, IsRenew: false,
		},
	}
	v := VarsFromEvent(ev)
	if got, _ := LookupPath(v, "guard.name"); got != "舰长" {
		t.Errorf("guard.name = %v", got)
	}
	if got, _ := LookupPath(v, "guard.isRenew"); got != false {
		t.Errorf("guard.isRenew = %v", got)
	}
}

func TestVarsFromSuperChat(t *testing.T) {
	ev := event.Event{
		Type: event.TypeSuperChat, RoomID: "1", Timestamp: time.Unix(1700000000, 0),
		Payload: event.SuperChat{
			User: event.User{UID: "9", Username: "SC用户"},
			Text: "加油", Price: 30, Duration: 60,
		},
	}
	v := VarsFromEvent(ev)
	if got, _ := LookupPath(v, "text"); got != "加油" {
		t.Errorf("text = %v", got)
	}
	if got, _ := LookupPath(v, "superChat.price"); got != int64(30) {
		t.Errorf("superChat.price = %v", got)
	}
}

func TestVarsFromUserEnter(t *testing.T) {
	ev := event.Event{
		Type: event.TypeUserEnter, RoomID: "1", Timestamp: time.Unix(1700000000, 0),
		Payload: event.UserEnter{User: event.User{UID: "9", Username: "进场用户", GuardLevel: 3}},
	}
	v := VarsFromEvent(ev)
	if got, _ := LookupPath(v, "user.username"); got != "进场用户" {
		t.Errorf("user.username = %v", got)
	}
	if got, _ := LookupPath(v, "user.guardLevel"); got != 3 {
		t.Errorf("user.guardLevel = %v", got)
	}
}

// TestVarsFromBattlePkVarsFilterSelfAndExposeOpponent 验证「对面」的
// 判定按既定裁决在规则层做——拿 ev.RoomID（本绑定房间号）跟每个
// member.RoomID 比对：自己那一项必须被过滤掉，不出现在 pk.opponents
// 里；对手的身份字段（uname/votes/isWinner）要能取到。
func TestVarsFromBattlePkVarsFilterSelfAndExposeOpponent(t *testing.T) {
	ev := event.Event{
		Type: event.TypeBattle, RoomID: "self-room", Timestamp: time.Unix(1700000000, 0),
		Payload: event.Battle{
			PkID: "pk-42",
			Members: []event.PkMember{
				{RoomID: "self-room", UID: "self-uid", Username: "自己"},
				{RoomID: "opp-room", UID: "opp-uid", Username: "对面主播", Votes: 65, IsWinner: true},
			},
		},
	}
	v := VarsFromEvent(ev)

	if got, _ := LookupPath(v, "pk.pkId"); got != "pk-42" {
		t.Errorf("pk.pkId = %v, 期望 pk-42", got)
	}
	if got, _ := LookupPath(v, "pk.opponent.roomId"); got != "opp-room" {
		t.Errorf("pk.opponent.roomId = %v, 期望 opp-room（自己应被过滤，剩下的是对面）", got)
	}
	if got, _ := LookupPath(v, "pk.opponent.uname"); got != "对面主播" {
		t.Errorf("pk.opponent.uname = %v", got)
	}
	if got, _ := LookupPath(v, "pk.opponent.votes"); got != int64(65) {
		t.Errorf("pk.opponent.votes = %v", got)
	}
	if got, _ := LookupPath(v, "pk.opponent.isWinner"); got != true {
		t.Errorf("pk.opponent.isWinner = %v", got)
	}

	opponents, ok := LookupPath(v, "pk.opponents")
	if !ok {
		t.Fatal("pk.opponents 应该存在")
	}
	list, ok := opponents.([]map[string]any)
	if !ok || len(list) != 1 {
		t.Fatalf("pk.opponents 应该恰好 1 个（自己被过滤），实际 %+v", opponents)
	}
}

// TestVarsFromBattlePkOpponentSnapshotFieldsDistinguishUnknownFromZero
// 是硬性约束的直接回归测试：「拿不到」（接口失败/还没抓到快照）和
// 「真的是 0」必须在 pk.opponent.online/guardTotal/guardOnline 这几个
// 变量上保持可区分——LookupPath 返回 (nil,false) 表示「不存在」，不能
// 把 nil 指针塌缩成写进 map 的整数 0，那样规则作者写
// {{.pk.opponent.online}} 会看到一个「看起来正常」的 0，而实际是「接口
// 没拿到」。同一个事件里刻意让一个对手已知为 0、另一个未知，两者必须
// 表现不同。
func TestVarsFromBattlePkOpponentSnapshotFieldsDistinguishUnknownFromZero(t *testing.T) {
	zero := int64(0)
	ev := event.Event{
		Type: event.TypeBattle, RoomID: "self-room", Timestamp: time.Unix(1700000000, 0),
		Payload: event.Battle{
			PkID: "pk-1",
			Members: []event.PkMember{
				{RoomID: "self-room", UID: "self-uid"},
				// 对手的快照三个字段全部未知（nil）——接口失败或快照还没就绪。
				{RoomID: "opp-unknown", UID: "u-unknown"},
			},
		},
	}
	v := VarsFromEvent(ev)
	for _, path := range []string{"pk.opponent.online", "pk.opponent.guardTotal", "pk.opponent.guardOnline"} {
		if _, ok := LookupPath(v, path); ok {
			t.Errorf("%s 应该不存在（未知），不该被塌缩成 0", path)
		}
	}

	// 换一个真的是 0 的场景：字段必须存在且等于 0，不能因为「不写零值」
	// 的惯例被误伤——这里的 0 是真实数据，不是「不存在」。
	evKnownZero := event.Event{
		Type: event.TypeBattle, RoomID: "self-room", Timestamp: time.Unix(1700000000, 0),
		Payload: event.Battle{
			PkID: "pk-2",
			Members: []event.PkMember{
				{RoomID: "self-room", UID: "self-uid"},
				{RoomID: "opp-zero", UID: "u-zero", Online: &zero, GuardTotal: &zero, GuardOnline: &zero},
			},
		},
	}
	v2 := VarsFromEvent(evKnownZero)
	for _, path := range []string{"pk.opponent.online", "pk.opponent.guardTotal", "pk.opponent.guardOnline"} {
		got, ok := LookupPath(v2, path)
		if !ok {
			t.Errorf("%s 应该存在（真实值是 0，不是未知）", path)
			continue
		}
		if got != int64(0) {
			t.Errorf("%s = %v, 期望 int64(0)", path, got)
		}
	}
}

// TestVarsFromBattlePkEndTimeExposed 是终审 Important-3 的回归测试：
// pk_basic.end_time 早在 PK_INFO 阶段就解析进了 event.Battle.EndTime
// （cmdmap/battle.go 的 mapPkInfo），但在这次修复之前全仓没有任何地方
// 读它，模板变量里也没有对应的 pk.endTime。这里验证非零时正确暴露、
// 零值（其余 PK_BATTLE_* CMD 上恒为零值，不是「真的是 0」）时不存在，
// 跟 pk.opponent.online 等字段同一套「拿不到不能塌缩成 0」的约定。
func TestVarsFromBattlePkEndTimeExposed(t *testing.T) {
	ev := event.Event{
		Type: event.TypeBattle, RoomID: "self-room", Timestamp: time.Unix(1700000000, 0),
		Payload: event.Battle{PkID: "pk-1", EndTime: 1700003600},
	}
	v := VarsFromEvent(ev)
	got, ok := LookupPath(v, "pk.endTime")
	if !ok {
		t.Fatal("pk.endTime 应该存在（EndTime 非零）")
	}
	if got != int64(1700003600) {
		t.Errorf("pk.endTime = %v, 期望 int64(1700003600)", got)
	}
}

// TestVarsFromBattlePkEndTimeAbsentWhenZero 验证零值 EndTime（大多数
// PK_BATTLE_* CMD 上都是这样，只有 PK_INFO/合成的快照事件真的带这个
// 字段）不会被写成 pk.endTime=0——那样会让模板作者误以为「PK 0 秒后
// 结束」这种看起来正常实则错误的数字。
func TestVarsFromBattlePkEndTimeAbsentWhenZero(t *testing.T) {
	ev := event.Event{
		Type: event.TypeBattle, RoomID: "self-room", Timestamp: time.Unix(1700000000, 0),
		Payload: event.Battle{SubCommand: "PK_BATTLE_PROCESS"},
	}
	v := VarsFromEvent(ev)
	if _, ok := LookupPath(v, "pk.endTime"); ok {
		t.Error("EndTime 为零值时 pk.endTime 不应该存在")
	}
}

func TestLookupPathMissingReturnsFalse(t *testing.T) {
	v := VarsFromEvent(danmakuEvent())
	for _, p := range []string{"不存在", "user.不存在", "text.深一层", ""} {
		if got, ok := LookupPath(v, p); ok {
			t.Errorf("路径 %q 不应存在，却返回 %v", p, got)
		}
	}
}

func TestMergeVarsKeepsNonEmpty(t *testing.T) {
	// 模拟 ENTRY_EFFECT（无昵称）与 INTERACT_WORD_V2（完整）的合并
	sparse := map[string]any{
		"type": "user_enter",
		"user": map[string]any{"uid": "123", "username": "", "guardLevel": 3},
	}
	full := map[string]any{
		"type": "user_enter",
		"user": map[string]any{"uid": "123", "username": "完整昵称", "guardLevel": 0},
	}

	MergeVars(sparse, full)

	u := sparse["user"].(map[string]any)
	if u["username"] != "完整昵称" {
		t.Errorf("空值应被非空值覆盖，实际 %v", u["username"])
	}
	if u["guardLevel"] != 3 {
		t.Errorf("非空值不应被空值覆盖，实际 %v", u["guardLevel"])
	}
}

func TestMergeVarsAddsMissingKeys(t *testing.T) {
	dst := map[string]any{"a": 1}
	MergeVars(dst, map[string]any{"b": 2})
	if dst["b"] != 2 {
		t.Errorf("缺失的键应被补上，实际 %v", dst)
	}
}

// flattenVarPaths 把 VarsFromEvent 的输出展开成一组叶子节点的点分路径，
// 供与 VariableCatalog() 声明的清单逐条比对。
func flattenVarPaths(v map[string]any) map[string]bool {
	out := map[string]bool{}
	var walk func(prefix string, m map[string]any)
	walk = func(prefix string, m map[string]any) {
		for k, val := range m {
			p := k
			if prefix != "" {
				p = prefix + "." + k
			}
			if sub, ok := val.(map[string]any); ok {
				walk(p, sub)
			} else {
				out[p] = true
			}
		}
	}
	walk("", v)
	return out
}

// TestVariableCatalogCommonMatchesVarsFromEvent 校验公共变量组：
// type/roomId/timestamp 是 VarsFromEvent 对任何事件都必产出的字段，
// 必须出现在 VariableCatalog() 的公共分组里；count/users/gifts 只在
// 合并之后才存在（见 aggregate.go），VarsFromEvent 本身不产出，必须
// 标记为 Optional，否则这条对照测试会把它们误判为「清单声称存在但
// 实际没有」。
//
// 这三个是聚合期变量，TestVariableCatalogMatchesVarsFromEvent（跟
// VarsFromEvent 的实际产出逐事件对照）天然看不到它们——全靠人工在
// commonVariables 里标 Optional 混过去。新增聚合期变量时，这条测试
// 的下方循环也要记得加一行，否则清单漏了字段不会有任何测试报红
// （gifts 就是这么漏掉的，见 commonVariables 定义处的注释）。
func TestVariableCatalogCommonMatchesVarsFromEvent(t *testing.T) {
	common, _ := VariableCatalog()
	commonPaths := make(map[string]bool, len(common))
	optional := make(map[string]bool, len(common))
	for _, v := range common {
		commonPaths[v.Path] = true
		if v.Optional {
			optional[v.Path] = true
		}
	}

	actual := flattenVarPaths(VarsFromEvent(danmakuEvent()))
	for _, p := range []string{"type", "roomId", "timestamp"} {
		if !commonPaths[p] {
			t.Errorf("VarsFromEvent 总是产出 %q，但公共清单未声明", p)
		}
		if !actual[p] {
			t.Errorf("VarsFromEvent 未产出公共字段 %q", p)
		}
	}
	for _, p := range []string{
		"count", "users", "gifts",
		"blindBox.name", "blindBox.count", "blindBox.cost", "blindBox.gain",
		"blindBox.profit", "blindBox.costYuan", "blindBox.gainYuan", "blindBox.profitYuan",
		"blindBox.profitAbsYuan",
	} {
		if commonPaths[p] && !optional[p] {
			t.Errorf("公共变量 %q 只在合并窗口聚合后才存在，VarsFromEvent 本身不产出，"+
				"必须标记 Optional", p)
		}
	}
}

// TestVariableCatalogHasBlindBoxFields 钉住 blindBox.* 确实在公共变量
// 清单里——它和 gifts 是同一类聚合期变量（见 commonVariables 定义处
// 的注释）：mergeBuckets 结算时才填，VarsFromEvent 本身永远不产出，
// 全靠这里人工登记；漏了不会有 TestVariableCatalogMatchesVarsFromEvent
// 报红，只会在 /api/meta/variables 和条件构建器下拉里悄悄少几项。
func TestVariableCatalogHasBlindBoxFields(t *testing.T) {
	common, _ := VariableCatalog()
	want := []string{
		"blindBox.name", "blindBox.count", "blindBox.cost", "blindBox.gain",
		"blindBox.profit", "blindBox.costYuan", "blindBox.gainYuan", "blindBox.profitYuan",
		"blindBox.profitAbsYuan",
	}
	got := map[string]bool{}
	optional := map[string]bool{}
	for _, v := range common {
		got[v.Path] = true
		if v.Optional {
			optional[v.Path] = true
		}
	}
	for _, p := range want {
		if !got[p] {
			t.Errorf("公共变量清单里没有 %q——mergeBuckets 早就在填这个变量了，清单没跟上", p)
			continue
		}
		if !optional[p] {
			t.Errorf("%q 是聚合期变量，未按盲盒聚合触发时不存在，必须标记 Optional", p)
		}
	}
}

// TestVariableCatalogHasGifts 钉住 gifts 确实在公共变量清单里——它是
// aggregate.go 的 mergeBuckets/PassthroughTrigger 早就在填的聚合期变量，
// 但公共变量清单里漏了这一条，导致 /api/meta/variables 不下发它、
// 条件构建器与模板变量提示的下拉里也没有，直到全批次终审才发现补上。
func TestVariableCatalogHasGifts(t *testing.T) {
	common, _ := VariableCatalog()
	for _, v := range common {
		if v.Path == "gifts" {
			if !v.Optional {
				t.Error("gifts 是聚合期变量，未合并触发时不存在，必须标记 Optional")
			}
			return
		}
	}
	t.Error(`公共变量清单里没有 "gifts"——mergeBuckets/PassthroughTrigger 早就在填这个 ` +
		`变量了，清单没跟上，导致 /api/meta/variables 不下发它`)
}

// TestVariableCatalogMatchesVarsFromEvent 是本任务真正的产出：用真实事件
// 逐个跑 VarsFromEvent，把实际产出的键路径与 VariableCatalog() 按事件
// 类型声明的清单对照。
//
// 两个方向都要查：
//   - 清单里有、VarsFromEvent 实际不产出、且没标 Optional → 清单撒谎，失败
//   - VarsFromEvent 实际产出、清单里没声明 → 清单漏了，失败（不管是否
//     标了 Optional，因为 Optional 只豁免「可能不存在」，不豁免「不存在
//     这条路径」）
//
// 为了让 Optional 字段（avatarUrl、medal.*、stats.*）也走到「实际产出」
// 这一侧被验证到，这里构造的事件尽量把所有字段都填满。
func TestVariableCatalogMatchesVarsFromEvent(t *testing.T) {
	_, byEvent := VariableCatalog()

	fullUser := event.User{
		UID: "1", Username: "全字段用户", AvatarURL: "http://example.com/a.png",
		GuardLevel: 3, UserLevel: 10, WealthLevel: 5, IsAdmin: true,
		Medal: &event.Medal{
			Name: "测试勋章", Level: 20, AnchorUID: "100", RoomID: "1",
			GuardLevel: 3, IsLighted: true,
		},
	}
	fans, fansClub, watched, likeCount := int64(100), int64(50), int64(1000), int64(10)

	events := map[event.Type]event.Event{
		event.TypeDanmaku: {
			Type: event.TypeDanmaku, RoomID: "1", Timestamp: time.Unix(1700000000, 0),
			Payload: event.Danmaku{
				User: fullUser, Text: "弹幕", Color: "#ffffff",
				IsEmoticon: true, ReplyToUID: "2",
			},
		},
		event.TypeSuperChat: {
			Type: event.TypeSuperChat, RoomID: "1", Timestamp: time.Unix(1700000000, 0),
			Payload: event.SuperChat{User: fullUser, ID: 1, Text: "SC", Price: 30, Duration: 60},
		},
		event.TypeGift: {
			Type: event.TypeGift, RoomID: "1", Timestamp: time.Unix(1700000000, 0),
			Payload: event.Gift{
				User: fullUser, GiftID: 1, GiftName: "礼物", Count: 1,
				CoinType: "gold", TotalCoin: 100, Action: "投喂", Price: 100,
				// BlindBox 填满，让 gift.blindBox.* 这组 Optional 字段也走到
				// 「实际产出」一侧被验证——不填的话这些路径在 catalog 里
				// 声明了 Optional 就永远不会被真正核对到，与 gifts 当年
				// 漏掉的路数一样：清单和实现悄悄漂开也不会有测试报红。
				BlindBox: &event.BlindBox{Name: "测试盲盒", GiftID: 999, Price: 5000, TipPrice: 5200},
			},
		},
		event.TypeGiftCombo: {
			Type: event.TypeGiftCombo, RoomID: "1", Timestamp: time.Unix(1700000000, 0),
			Payload: event.GiftCombo{
				User: fullUser, GiftID: 1, GiftName: "礼物", Count: 5,
				ComboID: "c1", TotalCoin: 500,
			},
		},
		event.TypeGuardBuy: {
			Type: event.TypeGuardBuy, RoomID: "1", Timestamp: time.Unix(1700000000, 0),
			Payload: event.GuardBuy{
				User: fullUser, GuardLevel: 3, GuardName: "舰长",
				Count: 1, Price: 198000, IsRenew: true,
			},
		},
		event.TypeUserEnter: {
			Type: event.TypeUserEnter, RoomID: "1", Timestamp: time.Unix(1700000000, 0),
			Payload: event.UserEnter{User: fullUser},
		},
		event.TypeUserFollow: {
			Type: event.TypeUserFollow, RoomID: "1", Timestamp: time.Unix(1700000000, 0),
			Payload: event.UserFollow{User: fullUser},
		},
		event.TypeUserShare: {
			Type: event.TypeUserShare, RoomID: "1", Timestamp: time.Unix(1700000000, 0),
			Payload: event.UserShare{User: fullUser},
		},
		event.TypeUserLike: {
			Type: event.TypeUserLike, RoomID: "1", Timestamp: time.Unix(1700000000, 0),
			Payload: event.UserLike{User: fullUser},
		},
		event.TypeUserBlocked: {
			Type: event.TypeUserBlocked, RoomID: "1", Timestamp: time.Unix(1700000000, 0),
			Payload: event.UserBlocked{User: fullUser, OperatorName: "房管"},
		},
		event.TypeRoomChange: {
			Type: event.TypeRoomChange, RoomID: "1", Timestamp: time.Unix(1700000000, 0),
			Payload: event.RoomChange{
				Title: "标题", AreaID: "1", AreaName: "分区",
				ParentAreaID: "2", ParentAreaName: "父分区",
			},
		},
		event.TypeRoomStatsUpdate: {
			Type: event.TypeRoomStatsUpdate, RoomID: "1", Timestamp: time.Unix(1700000000, 0),
			Payload: event.RoomStatsUpdate{
				Fans: &fans, FansClub: &fansClub, Watched: &watched, LikeCount: &likeCount,
			},
		},
		event.TypeOnlineRankUpdate: {
			Type: event.TypeOnlineRankUpdate, RoomID: "1", Timestamp: time.Unix(1700000000, 0),
			Payload: event.OnlineRankUpdate{Count: 50},
		},
		event.TypeBattle: {
			Type: event.TypeBattle, RoomID: "1", Timestamp: time.Unix(1700000000, 0),
			Payload: event.Battle{
				SubCommand: "PK_OPPONENT_SNAPSHOT",
				PkID:       "pk-1",
				// EndTime 填非零值，让 pk.endTime 这个 Optional 字段也走到
				// 「实际产出」一侧被验证——终审 Important-3 新增，同样的
				// 教训：不填的话 catalog 声明了这条路径也永远不会被真正
				// 核对到。
				EndTime: 1700003600,
				// 一个自己（RoomID="1"，必须被 pkVars 过滤掉）+ 一个对手，
				// 对手三个快照字段都填满，让 pk.opponent.online/guardTotal/
				// guardOnline 这组 Optional 字段也走到「实际产出」一侧被
				// 验证——不填的话即便 catalog 声明了这几条路径，也永远不会
				// 被真正核对到，跟 gifts/blindBox.* 当年漏掉的路数一样。
				Members: func() []event.PkMember {
					online, guardTotal, guardOnline := int64(100), int64(10), int64(3)
					return []event.PkMember{
						{RoomID: "1", UID: "self-uid", Username: "自己"},
						{
							RoomID: "2", UID: "opp-uid", Username: "对面主播",
							Votes: 65, IsWinner: true,
							Online: &online, GuardTotal: &guardTotal, GuardOnline: &guardOnline,
						},
					}
				}(),
			},
		},
		event.TypeVisitFromOpponent: {
			Type: event.TypeVisitFromOpponent, RoomID: "1", Timestamp: time.Unix(1700000000, 0),
			Payload: event.VisitFromOpponent{
				User: fullUser, OpponentRoomID: "2", MatchedBy: event.VisitMatchedByAudience,
			},
		},
		event.TypeVisitToOpponent: {
			Type: event.TypeVisitToOpponent, RoomID: "1", Timestamp: time.Unix(1700000000, 0),
			Payload: event.VisitToOpponent{
				User: fullUser, OpponentRoomID: "2", MatchedBy: event.VisitMatchedByFanMedal,
			},
		},
		event.TypeUnknown: {
			Type: event.TypeUnknown, RoomID: "1", Timestamp: time.Unix(1700000000, 0),
			Payload: event.Unknown{Command: "SOME_CMD"},
		},
	}

	for typ, ev := range events {
		t.Run(string(typ), func(t *testing.T) {
			actual := flattenVarPaths(VarsFromEvent(ev))
			// 公共字段单独由 TestVariableCatalogCommonMatchesVarsFromEvent 校验，
			// 这里只比对该事件类型特有的部分。
			delete(actual, "type")
			delete(actual, "roomId")
			delete(actual, "timestamp")

			catalog, ok := byEvent[typ]
			if !ok {
				t.Fatalf("VariableCatalog 未声明事件类型 %q，但 VarsFromEvent 为它产出了字段 %v",
					typ, actual)
			}
			catalogPaths := make(map[string]bool, len(catalog))
			for _, v := range catalog {
				catalogPaths[v.Path] = true
				if !v.Optional && !actual[v.Path] {
					t.Errorf("清单声称路径 %q 存在，但 VarsFromEvent 实际未产出", v.Path)
				}
			}
			for p := range actual {
				if !catalogPaths[p] {
					t.Errorf("VarsFromEvent 为事件类型 %q 产出了路径 %q，但 VariableCatalog 未声明",
						typ, p)
				}
			}
		})
	}
}
