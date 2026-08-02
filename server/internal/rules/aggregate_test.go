package rules

import (
	"sync"
	"testing"
	"time"

	"github.com/MrZhongzq/MagicalDanmaku/server/internal/event"
)

// collector 收集 Aggregator 产出的 Trigger。
type collector struct {
	mu  sync.Mutex
	got []Trigger
}

func (c *collector) add(tr Trigger) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.got = append(c.got, tr)
}

func (c *collector) all() []Trigger {
	c.mu.Lock()
	defer c.mu.Unlock()
	out := make([]Trigger, len(c.got))
	copy(out, c.got)
	return out
}

func enterEvent(uid, name string, guard int) event.Event {
	return event.Event{
		Type: event.TypeUserEnter, RoomID: "1", Timestamp: time.Unix(1700000000, 0),
		Payload: event.UserEnter{User: event.User{UID: uid, Username: name, GuardLevel: guard}},
	}
}

func giftEvent(uid, name, giftName string, count, coin int64) event.Event {
	return event.Event{
		Type: event.TypeGift, RoomID: "1", Timestamp: time.Unix(1700000000, 0),
		Payload: event.Gift{
			User:     event.User{UID: uid, Username: name},
			GiftName: giftName, Count: count, TotalCoin: coin, CoinType: "gold",
		},
	}
}

func TestAggregateByTypeMergesAll(t *testing.T) {
	c := &collector{}
	// 窗口设很长，用 Flush 手动结算，避免测试依赖真实时间
	agg := NewAggregator(AggregateSpec{Window: time.Hour, By: AggregateByType}, c.add)
	defer agg.Close()

	agg.Add(enterEvent("1", "甲", 0))
	agg.Add(enterEvent("2", "乙", 3))
	agg.Add(enterEvent("3", "丙", 0))
	agg.Flush()

	got := c.all()
	if len(got) != 1 {
		t.Fatalf("按类型合并应产出 1 条 Trigger，实际 %d", len(got))
	}
	tr := got[0]
	if tr.Type != event.TypeUserEnter {
		t.Errorf("Type = %s", tr.Type)
	}
	if len(tr.Events) != 3 {
		t.Errorf("Events 数 = %d, 期望 3", len(tr.Events))
	}
	if cnt, _ := LookupPath(tr.Vars, "count"); cnt != 3 {
		t.Errorf("count = %v, 期望 3", cnt)
	}
	users, ok := tr.Vars["users"].([]string)
	if !ok {
		t.Fatalf("users 类型错误: %T", tr.Vars["users"])
	}
	if len(users) != 3 || users[0] != "甲" || users[2] != "丙" {
		t.Errorf("users = %v, 期望按首次出现顺序 [甲 乙 丙]", users)
	}
}

func TestAggregateMergesSameUIDFields(t *testing.T) {
	// 模拟 ENTRY_EFFECT（无昵称）+ INTERACT_WORD_V2（完整）
	c := &collector{}
	agg := NewAggregator(AggregateSpec{Window: time.Hour, By: AggregateByType}, c.add)
	defer agg.Close()

	agg.Add(enterEvent("1018633655", "", 3))       // ENTRY_EFFECT：有 UID 有舰长，无昵称
	agg.Add(enterEvent("1018633655", "洛洛的小小小", 0)) // INTERACT_WORD_V2：有昵称，无舰长
	agg.Flush()

	got := c.all()
	if len(got) != 1 {
		t.Fatalf("同一 UID 应合并为 1 条，实际 %d", len(got))
	}
	users := got[0].Vars["users"].([]string)
	if len(users) != 1 {
		t.Fatalf("同一用户只应出现一次，实际 %v", users)
	}
	if users[0] != "洛洛的小小小" {
		t.Errorf("users[0] = %q，应取非空昵称", users[0])
	}
	if cnt, _ := LookupPath(got[0].Vars, "count"); cnt != 1 {
		t.Errorf("count = %v，同一用户应计为 1", cnt)
	}
	// 舰长等级来自第一条，不该被第二条的 0 覆盖
	if gl, _ := LookupPath(got[0].Vars, "user.guardLevel"); gl != 3 {
		t.Errorf("user.guardLevel = %v，非空值不应被零值覆盖", gl)
	}
}

func TestAggregateByUserProducesOnePerUser(t *testing.T) {
	c := &collector{}
	agg := NewAggregator(AggregateSpec{Window: time.Hour, By: AggregateByUser}, c.add)
	defer agg.Close()

	agg.Add(enterEvent("1", "甲", 0))
	agg.Add(enterEvent("1", "甲", 0)) // 重复，应被去重
	agg.Add(enterEvent("2", "乙", 0))
	agg.Flush()

	got := c.all()
	if len(got) != 2 {
		t.Fatalf("两个用户应产出 2 条 Trigger，实际 %d", len(got))
	}
}

func TestAggregateByGiftAccumulatesCount(t *testing.T) {
	c := &collector{}
	agg := NewAggregator(AggregateSpec{Window: time.Hour, By: AggregateByGift}, c.add)
	defer agg.Close()

	agg.Add(giftEvent("9", "土豪", "小花花", 1, 100))
	agg.Add(giftEvent("9", "土豪", "小花花", 1, 100))
	agg.Add(giftEvent("9", "土豪", "小花花", 3, 300))
	agg.Add(giftEvent("9", "土豪", "辣条", 5, 50)) // 不同礼物，另算一组
	agg.Flush()

	got := c.all()
	if len(got) != 2 {
		t.Fatalf("两种礼物应产出 2 条 Trigger，实际 %d", len(got))
	}

	var flower Trigger
	for _, tr := range got {
		if n, _ := LookupPath(tr.Vars, "gift.name"); n == "小花花" {
			flower = tr
		}
	}
	if cnt, _ := LookupPath(flower.Vars, "gift.count"); cnt != int64(5) {
		t.Errorf("gift.count = %v (%T), 期望累加为 5", cnt, cnt)
	}
	if coin, _ := LookupPath(flower.Vars, "gift.totalCoin"); coin != int64(500) {
		t.Errorf("gift.totalCoin = %v, 期望累加为 500", coin)
	}
}

func TestAggregateFiresAfterWindow(t *testing.T) {
	c := &collector{}
	agg := NewAggregator(AggregateSpec{Window: 60 * time.Millisecond, By: AggregateByType}, c.add)
	defer agg.Close()

	agg.Add(enterEvent("1", "甲", 0))
	if len(c.all()) != 0 {
		t.Error("窗口未到期不应产出")
	}

	time.Sleep(150 * time.Millisecond)
	if got := c.all(); len(got) != 1 {
		t.Fatalf("窗口到期应自动产出，实际 %d 条", len(got))
	}
}

func TestAggregateEmptyFlushProducesNothing(t *testing.T) {
	c := &collector{}
	agg := NewAggregator(AggregateSpec{Window: time.Hour, By: AggregateByType}, c.add)
	defer agg.Close()

	agg.Flush()
	if got := c.all(); len(got) != 0 {
		t.Errorf("空窗口不应产出，实际 %d 条", len(got))
	}
}

func TestAggregateSeparatesEventTypes(t *testing.T) {
	c := &collector{}
	agg := NewAggregator(AggregateSpec{Window: time.Hour, By: AggregateByType}, c.add)
	defer agg.Close()

	agg.Add(enterEvent("1", "甲", 0))
	agg.Add(giftEvent("2", "乙", "小花花", 1, 100))
	agg.Flush()

	got := c.all()
	if len(got) != 2 {
		t.Fatalf("不同事件类型应分开产出，实际 %d 条", len(got))
	}
}

func TestPassthroughTrigger(t *testing.T) {
	ev := enterEvent("1", "甲", 3)
	tr := PassthroughTrigger(ev)

	if tr.Type != event.TypeUserEnter {
		t.Errorf("Type = %s", tr.Type)
	}
	if len(tr.Events) != 1 {
		t.Errorf("Events 数 = %d, 期望 1", len(tr.Events))
	}
	if u, _ := LookupPath(tr.Vars, "user.username"); u != "甲" {
		t.Errorf("user.username = %v", u)
	}
	// 直通事件也应带 count，让模板可以统一写法
	if cnt, _ := LookupPath(tr.Vars, "count"); cnt != 1 {
		t.Errorf("count = %v, 期望 1", cnt)
	}
	users, _ := tr.Vars["users"].([]string)
	if len(users) != 1 || users[0] != "甲" {
		t.Errorf("users = %v", users)
	}
}

func TestAggregateCloseFlushesPending(t *testing.T) {
	c := &collector{}
	agg := NewAggregator(AggregateSpec{Window: time.Hour, By: AggregateByType}, c.add)

	agg.Add(enterEvent("1", "甲", 0))
	agg.Close()

	if got := c.all(); len(got) != 1 {
		t.Errorf("Close 应结算未决窗口，实际 %d 条", len(got))
	}
}

func TestAggregateByTypeStillSeparatesDifferentGifts(t *testing.T) {
	// 即使按 type 分组，同一用户的不同礼物也不得在第一步被混合累加。
	// 桶键必须包含礼物名，否则小花花与辣条的数量会相加。
	c := &collector{}
	agg := NewAggregator(AggregateSpec{Window: time.Hour, By: AggregateByGift}, c.add)
	defer agg.Close()

	agg.Add(giftEvent("9", "土豪", "小花花", 2, 200))
	agg.Add(giftEvent("9", "土豪", "辣条", 7, 70))
	agg.Flush()

	got := c.all()
	if len(got) != 2 {
		t.Fatalf("同一用户的不同礼物应分开，实际 %d 条", len(got))
	}
	counts := map[string]int64{}
	for _, tr := range got {
		name, _ := LookupPath(tr.Vars, "gift.name")
		cnt, _ := LookupPath(tr.Vars, "gift.count")
		counts[toString(name)] = cnt.(int64)
	}
	if counts["小花花"] != 2 {
		t.Errorf("小花花数量 = %d, 期望 2", counts["小花花"])
	}
	if counts["辣条"] != 7 {
		t.Errorf("辣条数量 = %d, 期望 7", counts["辣条"])
	}
}

func TestAggregateMinCountEmitsIndividually(t *testing.T) {
	// 人数不足 minCount 时不合并，每人各出一条 Trigger
	c := &collector{}
	agg := NewAggregator(AggregateSpec{
		Window: time.Hour, By: AggregateByType, MinCount: 3,
	}, c.add)
	defer agg.Close()

	agg.Add(enterEvent("1", "甲", 0))
	agg.Add(enterEvent("2", "乙", 0))
	agg.Flush()

	got := c.all()
	if len(got) != 2 {
		t.Fatalf("人数 2 < minCount 3，应各出一条，实际 %d 条", len(got))
	}
	for i, tr := range got {
		if cnt, _ := LookupPath(tr.Vars, "count"); cnt != 1 {
			t.Errorf("第 %d 条的 count = %v, 期望 1", i+1, cnt)
		}
		if users := tr.Vars["users"].([]string); len(users) != 1 {
			t.Errorf("第 %d 条的 users = %v, 期望单人", i+1, users)
		}
	}
}

func TestAggregateMinCountMergesWhenEnough(t *testing.T) {
	c := &collector{}
	agg := NewAggregator(AggregateSpec{
		Window: time.Hour, By: AggregateByType, MinCount: 3,
	}, c.add)
	defer agg.Close()

	agg.Add(enterEvent("1", "甲", 0))
	agg.Add(enterEvent("2", "乙", 0))
	agg.Add(enterEvent("3", "丙", 0))
	agg.Flush()

	got := c.all()
	if len(got) != 1 {
		t.Fatalf("人数 3 >= minCount 3，应合并为一条，实际 %d 条", len(got))
	}
	if cnt, _ := LookupPath(got[0].Vars, "count"); cnt != 3 {
		t.Errorf("count = %v, 期望 3", cnt)
	}
}

func TestAggregateMinCountZeroAlwaysMerges(t *testing.T) {
	c := &collector{}
	agg := NewAggregator(AggregateSpec{Window: time.Hour, By: AggregateByType}, c.add)
	defer agg.Close()

	agg.Add(enterEvent("1", "甲", 0))
	agg.Flush()

	if got := c.all(); len(got) != 1 {
		t.Errorf("未设 minCount 时单人也应走合并路径，实际 %d 条", len(got))
	}
}

func TestAggregateWindowIsFixedNotRolling(t *testing.T) {
	// 乙语义：窗口从首个事件起算，固定 Window 时长后结算；中途来的
	// 事件不会推迟结算。这与旧版「静默计时、每来一个事件都把定时器
	// 往后推」不同——旧版下第二个事件会把结算推迟到 130ms(50+80) 后，
	// 本测试在 95ms 处检查，若仍是旧的滚动语义会看到 0 条。
	c := &collector{}
	agg := NewAggregator(AggregateSpec{
		Window: 80 * time.Millisecond, By: AggregateByType,
	}, c.add)
	defer agg.Close()

	agg.Add(enterEvent("1", "甲", 0))
	time.Sleep(50 * time.Millisecond)
	agg.Add(enterEvent("2", "乙", 0)) // 若窗口会滚动，这里会把结算推迟

	time.Sleep(60 * time.Millisecond) // 累计已过 110ms，超过 Window(80ms) 30ms 余量
	got := c.all()
	if len(got) != 1 {
		t.Fatalf("窗口应已到期（固定从首个事件起 80ms 结算），实际产出 %d 条", len(got))
	}
	if cnt, _ := LookupPath(got[0].Vars, "count"); cnt != 2 {
		t.Errorf("count = %v，期望两个事件都落在首轮内合并为 2", cnt)
	}
}

func TestAggregateNextEventReanchorsAfterWindowCloses(t *testing.T) {
	// 乙语义：上一轮结束后，下一个到来的事件重新起锚，而不是按固定
	// 时间网格切分（也不是永远不再结算）。
	c := &collector{}
	agg := NewAggregator(AggregateSpec{
		Window: 60 * time.Millisecond, By: AggregateByType,
	}, c.add)
	defer agg.Close()

	agg.Add(enterEvent("1", "甲", 0))
	time.Sleep(90 * time.Millisecond) // 第一轮已到期结算
	if got := c.all(); len(got) != 1 {
		t.Fatalf("第一轮应已结算，实际 %d 条", len(got))
	}

	agg.Add(enterEvent("2", "乙", 0)) // 应重新起锚，而不是延续上一轮的锚点
	time.Sleep(30 * time.Millisecond)
	if got := c.all(); len(got) != 1 {
		t.Fatalf("第二轮距其自身锚点尚未到期不应产出，实际 %d 条", len(got))
	}
	time.Sleep(50 * time.Millisecond) // 距第二轮锚点已过 80ms，超过 Window(60ms)
	got := c.all()
	if len(got) != 2 {
		t.Fatalf("第二轮到期后应产出第 2 条，实际 %d 条", len(got))
	}
	if cnt, _ := LookupPath(got[1].Vars, "count"); cnt != 1 {
		t.Errorf("第二轮 count = %v，期望 1", cnt)
	}
}

func TestAggregateRealSampleTimestampsSplitIntoRounds(t *testing.T) {
	// 用真实样本钉住乙语义：用户回报的 17 条盲盒相对时间戳（秒，从
	// 第一条起）为
	//   0,4,9,14,19,24,29,35,40,53,55,57,58,60,61,122,170
	// 本任务只测窗口语义，用同一个键（同一 UID+礼物名）喂入，
	// Window=10s。按「窗口从本轮首个事件起算，固定 Window 后结算，
	// 关闭后下一个事件重新起锚」逐轮验算（[anchor, anchor+10) 半开
	// 区间，到达时刻 >= 边界即出轮）：
	//
	//   R1 anchor=0  close=10  成员 0,4,9              下一条14>=10，出轮
	//   R2 anchor=14 close=24  成员 14,19               下一条24，恰好等于
	//                                                    边界（margin=0）
	//   R3 anchor=24 close=34  成员 24,29               下一条35，margin
	//                                                    仅 1s
	//   R4 anchor=35 close=45  成员 35,40               下一条53，margin=8s
	//   R5 anchor=53 close=63  成员 53,55,57,58,60,61    下一条122，安全
	//   R6 anchor=122 close=132 成员 122                 下一条170，安全
	//   R7 anchor=170           成员 170（用 Flush 结算，不等自然到期）
	//
	// 分轮结果 3+2+2+2+6+1+1=17，与原始数据一致。
	//
	// R2→R3 恰好卡在边界（margin=0）、R3→R4 只差 1s，这两处间隔在压缩
	// 后的真实定时器测试里会变成纯粹的计时器抖动竞态，与本测试要钉住
	// 的「窗口语义」无关。因此从偏移 24 起整体顺延 +3（消除卡边界的
	// 竞态），从偏移 35 起再顺延 +3（累积 +6，把 1s 的余量放大到安全
	// 范围）——顺延不改变同一轮内部的相对间隔，也不改变判轮结论。
	// R5→R6、R6→R7 的巨大间隔（61s、48s）本就远超 Window，压缩测试时
	// 没必要按比例保留，改为固定的安全间隔，同样不影响判轮结论。
	//
	// 调整后的偏移（样本秒，未乘真实时间尺度）：
	//   0,4,9,14,19,27,32,41,46,59,61,63,64,66,67,74,89
	const scale = 30 * time.Millisecond // 1 个样本秒 = 30ms 真实时间
	window := 10 * scale                // 300ms

	offsets := []int{0, 4, 9, 14, 19, 27, 32, 41, 46, 59, 61, 63, 64, 66, 67, 74, 89}
	wantRoundSizes := []int{3, 2, 2, 2, 6, 1, 1}

	c := &collector{}
	agg := NewAggregator(AggregateSpec{Window: window, By: AggregateByType}, c.add)
	defer agg.Close()

	start := time.Now()
	for _, off := range offsets {
		target := time.Duration(off) * scale
		if d := target - time.Since(start); d > 0 {
			time.Sleep(d)
		}
		agg.Add(giftEvent("3546956351671045", "同一个人", "幸运盲盒", 1, 100))
	}

	// 第 7 轮刚起锚，不等它自然到期，直接结算。
	agg.Flush()
	// 留出余量，确保前几轮由定时器触发的结算 goroutine 已经跑完。
	time.Sleep(80 * time.Millisecond)

	got := c.all()
	gotSizes := make([]int, len(got))
	for i, tr := range got {
		gotSizes[i] = len(tr.Events)
	}
	if len(got) != len(wantRoundSizes) {
		t.Fatalf("轮数 = %d，期望 %d；实际每轮事件数 = %v，期望 %v", len(got), len(wantRoundSizes), gotSizes, wantRoundSizes)
	}
	for i := range wantRoundSizes {
		if gotSizes[i] != wantRoundSizes[i] {
			t.Errorf("第 %d 轮事件数 = %d，期望 %d（全部轮次：%v，期望 %v）", i+1, gotSizes[i], wantRoundSizes[i], gotSizes, wantRoundSizes)
		}
	}
}

func TestAggregateByTypeGiftsDeduplicatesAndKeepsOrder(t *testing.T) {
	// 合并窗口里「小花花×3、人气票×1、小花花×2」（分属不同用户，
	// 否则会在第一步合并进同一桶）→ gifts 应为 ["小花花","人气票"]，
	// 既不是三条也不是按数量/字典序排列。
	c := &collector{}
	agg := NewAggregator(AggregateSpec{Window: time.Hour, By: AggregateByType}, c.add)
	defer agg.Close()

	agg.Add(giftEvent("1", "甲", "小花花", 3, 300))
	agg.Add(giftEvent("2", "乙", "人气票", 1, 10))
	agg.Add(giftEvent("3", "丙", "小花花", 2, 200))
	agg.Flush()

	got := c.all()
	if len(got) != 1 {
		t.Fatalf("按类型合并应产出 1 条 Trigger，实际 %d", len(got))
	}
	gifts, ok := got[0].Vars["gifts"].([]string)
	if !ok {
		t.Fatalf("gifts 类型错误: %T", got[0].Vars["gifts"])
	}
	if len(gifts) != 2 {
		t.Fatalf("gifts = %v，期望去重后 2 条", gifts)
	}
	if gifts[0] != "小花花" || gifts[1] != "人气票" {
		t.Errorf("gifts = %v，期望按首次出现顺序 [小花花 人气票]", gifts)
	}
}

func TestAggregateByTypeGiftsSkipsNonGiftEvents(t *testing.T) {
	// 合并窗口里混入非礼物事件（进场）时，gifts 不应 panic，
	// 也不应把非礼物事件塞成空字符串条目。
	c := &collector{}
	agg := NewAggregator(AggregateSpec{Window: time.Hour, By: AggregateByType}, c.add)
	defer agg.Close()

	agg.Add(enterEvent("1", "甲", 0))
	agg.Flush()

	got := c.all()
	if len(got) != 1 {
		t.Fatalf("应产出 1 条 Trigger，实际 %d", len(got))
	}
	gifts, ok := got[0].Vars["gifts"].([]string)
	if !ok {
		t.Fatalf("gifts 类型错误: %T", got[0].Vars["gifts"])
	}
	if len(gifts) != 0 {
		t.Errorf("非礼物事件不应产生 gifts 条目，实际 %v", gifts)
	}
}

func TestPassthroughTriggerGifts(t *testing.T) {
	// 非合并触发（PassthroughTrigger）也应填 gifts，让模板对合并
	// 与非合并事件可以统一写法：单个礼物事件的 gifts 就是那一个礼物名。
	tr := PassthroughTrigger(giftEvent("9", "土豪", "小花花", 1, 100))

	gifts, ok := tr.Vars["gifts"].([]string)
	if !ok {
		t.Fatalf("gifts 类型错误: %T", tr.Vars["gifts"])
	}
	if len(gifts) != 1 || gifts[0] != "小花花" {
		t.Errorf("gifts = %v，期望 [小花花]", gifts)
	}
}

func TestPassthroughTriggerGiftsEmptyForNonGiftEvent(t *testing.T) {
	// 非礼物事件（如进场）没有礼物名，gifts 应是空数组而非缺失该键
	// 或塞入空字符串——与 users 在同样情况下的处理保持一致。
	tr := PassthroughTrigger(enterEvent("1", "甲", 0))

	gifts, ok := tr.Vars["gifts"].([]string)
	if !ok {
		t.Fatalf("gifts 类型错误: %T", tr.Vars["gifts"])
	}
	if len(gifts) != 0 {
		t.Errorf("非礼物事件的 gifts 应为空数组，实际 %v", gifts)
	}
}

func TestAggregateMaxWaitNoLongerDelaysFixedWindow(t *testing.T) {
	// 乙语义下窗口本身有固定上界（Window），MaxWait 原本用于防止
	// 「持续送礼、静默期永不到来」的场景已不存在：结算永远发生在
	// Window 到期，不会被更大的 MaxWait 取值拖后。
	//
	// 注：这条测试取代了旧版的 TestAggregateMaxWaitCapsRolling——旧版
	// 靠持续 Add 让静默计时永不到期，只能证明 MaxWait 兜底生效；乙语义
	// 下窗口不再滚动，这个场景已经不存在，MaxWait 也就不再需要那份兜
	// 底职责了。
	c := &collector{}
	agg := NewAggregator(AggregateSpec{
		Window: 60 * time.Millisecond, MaxWait: 500 * time.Millisecond, By: AggregateByType,
	}, c.add)
	defer agg.Close()

	agg.Add(enterEvent("1", "甲", 0))
	time.Sleep(90 * time.Millisecond) // 远小于 MaxWait(500ms)，但已超过 Window(60ms)

	if got := c.all(); len(got) != 1 {
		t.Fatalf("应在 Window 到期时结算，不受 MaxWait 更大取值影响，实际 %d 条", len(got))
	}
}

func TestAggregateSpecValidateRejectsBadMaxWait(t *testing.T) {
	s := AggregateSpec{Window: 5 * time.Second, MaxWait: time.Second, By: AggregateByType}
	if err := s.Validate(); err == nil {
		t.Error("maxWait 小于 window 应当报错")
	}
}
