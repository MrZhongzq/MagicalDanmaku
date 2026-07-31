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

func TestAggregateRollingWindowExtends(t *testing.T) {
	// 滚动：每来一个事件就重置静默计时，持续有人时不断延后结算
	c := &collector{}
	agg := NewAggregator(AggregateSpec{
		Window: 80 * time.Millisecond, By: AggregateByType,
	}, c.add)
	defer agg.Close()

	agg.Add(enterEvent("1", "甲", 0))
	time.Sleep(50 * time.Millisecond)
	agg.Add(enterEvent("2", "乙", 0)) // 重置计时
	time.Sleep(50 * time.Millisecond)
	agg.Add(enterEvent("3", "丙", 0)) // 再次重置

	// 此刻已过 100ms，若非滚动早该结算了
	if got := c.all(); len(got) != 0 {
		t.Fatalf("滚动窗口应被不断延后，实际已产出 %d 条", len(got))
	}

	time.Sleep(150 * time.Millisecond) // 静默超过 window
	got := c.all()
	if len(got) != 1 {
		t.Fatalf("静默后应结算，实际 %d 条", len(got))
	}
	if cnt, _ := LookupPath(got[0].Vars, "count"); cnt != 3 {
		t.Errorf("count = %v, 期望三人合并", cnt)
	}
}

func TestAggregateMaxWaitCapsRolling(t *testing.T) {
	// 持续有人进场时，maxWait 兜底强制结算，避免永不触发
	c := &collector{}
	agg := NewAggregator(AggregateSpec{
		Window:  100 * time.Millisecond,
		MaxWait: 150 * time.Millisecond,
		By:      AggregateByType,
	}, c.add)
	defer agg.Close()

	// 每 50ms 来一个人，静默窗口永远不会到期
	for i := 0; i < 6; i++ {
		agg.Add(enterEvent(string(rune('1'+i)), "用户", 0))
		time.Sleep(50 * time.Millisecond)
	}

	if got := c.all(); len(got) == 0 {
		t.Fatal("maxWait 应强制结算，实际一条未出")
	}
}

func TestAggregateSpecValidateRejectsBadMaxWait(t *testing.T) {
	s := AggregateSpec{Window: 5 * time.Second, MaxWait: time.Second, By: AggregateByType}
	if err := s.Validate(); err == nil {
		t.Error("maxWait 小于 window 应当报错")
	}
}
