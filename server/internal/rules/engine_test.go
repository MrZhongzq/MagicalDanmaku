package rules

import (
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/MrZhongzq/MagicalDanmaku/server/internal/event"
)

// recordBot 记录所有发出的动作。
type recordBot struct {
	mu       sync.Mutex
	danmakus []string
	blocks   []string
}

func (b *recordBot) SendDanmaku(text string) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.danmakus = append(b.danmakus, text)
	return nil
}

func (b *recordBot) Block(uid string, hours int) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.blocks = append(b.blocks, uid)
	return nil
}

func (b *recordBot) sent() []string {
	b.mu.Lock()
	defer b.mu.Unlock()
	out := make([]string, len(b.danmakus))
	copy(out, b.danmakus)
	return out
}

func (b *recordBot) blocked() []string {
	b.mu.Lock()
	defer b.mu.Unlock()
	out := make([]string, len(b.blocks))
	copy(out, b.blocks)
	return out
}

func newTestEngine(t *testing.T, rs []Rule, bot BotAPI) *Engine {
	t.Helper()
	e, err := NewEngine(EngineOptions{
		Label:         "测试号@1",
		RoomID:        "1",
		Rules:         rs,
		Bot:           bot,
		ScriptTimeout: 200 * time.Millisecond,
	})
	if err != nil {
		t.Fatalf("NewEngine 失败: %v", err)
	}
	t.Cleanup(e.Close)
	return e
}

func mkDanmaku(uid, name, text string, guard int) event.Event {
	return event.Event{
		Type: event.TypeDanmaku, RoomID: "1", Timestamp: time.Unix(1700000000, 0),
		Payload: event.Danmaku{
			User: event.User{UID: uid, Username: name, GuardLevel: guard},
			Text: text,
		},
	}
}

func mkEnter(uid, name string, guard int) event.Event {
	return event.Event{
		Type: event.TypeUserEnter, RoomID: "1", Timestamp: time.Unix(1700000000, 0),
		Payload: event.UserEnter{User: event.User{UID: uid, Username: name, GuardLevel: guard}},
	}
}

func TestEngineSimpleRule(t *testing.T) {
	bot := &recordBot{}
	e := newTestEngine(t, []Rule{{
		Name: "复读", Enabled: true, On: []event.Type{event.TypeDanmaku},
		Do: []Action{{Type: ActionDanmaku, Template: []string{"收到：{{.text}}"}}},
	}}, bot)

	e.Handle(mkDanmaku("1", "甲", "你好", 0))

	if got := bot.sent(); len(got) != 1 || got[0] != "收到：你好" {
		t.Errorf("= %v", got)
	}
}

func TestEngineConditionFilters(t *testing.T) {
	bot := &recordBot{}
	e := newTestEngine(t, []Rule{{
		Name: "仅舰长", Enabled: true, On: []event.Type{event.TypeDanmaku},
		When: &Condition{Field: "user.guardLevel", Op: "gt", Value: 0},
		Do:   []Action{{Type: ActionDanmaku, Template: []string{"舰长你好"}}},
	}}, bot)

	e.Handle(mkDanmaku("1", "甲", "你好", 0)) // 非舰长
	e.Handle(mkDanmaku("2", "乙", "你好", 3)) // 舰长

	if got := bot.sent(); len(got) != 1 {
		t.Errorf("只应响应舰长，实际 %v", got)
	}
}

func TestEngineAggregatesEnters(t *testing.T) {
	bot := &recordBot{}
	e := newTestEngine(t, []Rule{{
		Name: "进场欢迎", Enabled: true, On: []event.Type{event.TypeUserEnter},
		Aggregate: &AggregateSpec{Window: 60 * time.Millisecond, By: AggregateByType},
		Do:        []Action{{Type: ActionDanmaku, Template: []string{`欢迎 {{join .users "、"}}`}}},
	}}, bot)

	e.Handle(mkEnter("1", "甲", 0))
	e.Handle(mkEnter("2", "乙", 0))
	e.Handle(mkEnter("3", "丙", 0))

	if got := bot.sent(); len(got) != 0 {
		t.Errorf("窗口未到期不应发送，实际 %v", got)
	}
	time.Sleep(150 * time.Millisecond)

	got := bot.sent()
	if len(got) != 1 {
		t.Fatalf("应合并为 1 条，实际 %v", got)
	}
	if got[0] != "欢迎 甲、乙、丙" {
		t.Errorf("= %q", got[0])
	}
}

func TestEngineConditionAppliedBeforeAggregation(t *testing.T) {
	// 这是核心约定：先按单事件过滤，再合并
	bot := &recordBot{}
	e := newTestEngine(t, []Rule{{
		Name: "只欢迎舰长", Enabled: true, On: []event.Type{event.TypeUserEnter},
		When:      &Condition{Field: "user.guardLevel", Op: "gt", Value: 0},
		Aggregate: &AggregateSpec{Window: 60 * time.Millisecond, By: AggregateByType},
		Do:        []Action{{Type: ActionDanmaku, Template: []string{`欢迎 {{join .users "、"}}`}}},
	}}, bot)

	e.Handle(mkEnter("1", "普通甲", 0))
	e.Handle(mkEnter("2", "舰长乙", 3))
	e.Handle(mkEnter("3", "普通丙", 0))
	e.Handle(mkEnter("4", "舰长丁", 3))

	time.Sleep(150 * time.Millisecond)

	got := bot.sent()
	if len(got) != 1 {
		t.Fatalf("应产出 1 条，实际 %v", got)
	}
	if got[0] != "欢迎 舰长乙、舰长丁" {
		t.Errorf("= %q，非舰长不该进入合并结果", got[0])
	}
}

func TestEngineMergesDuplicateEnter(t *testing.T) {
	// P0 联调发现的真实问题：ENTRY_EFFECT 无昵称 + INTERACT_WORD_V2 完整
	bot := &recordBot{}
	e := newTestEngine(t, []Rule{{
		Name: "进场欢迎", Enabled: true, On: []event.Type{event.TypeUserEnter},
		Aggregate: &AggregateSpec{Window: 60 * time.Millisecond, By: AggregateByType},
		Do:        []Action{{Type: ActionDanmaku, Template: []string{`欢迎 {{join .users "、"}}`}}},
	}}, bot)

	e.Handle(mkEnter("1018633655", "", 3))       // ENTRY_EFFECT
	e.Handle(mkEnter("1018633655", "洛洛的小小小", 0)) // INTERACT_WORD_V2

	time.Sleep(150 * time.Millisecond)

	got := bot.sent()
	if len(got) != 1 {
		t.Fatalf("同一用户应只欢迎一次，实际 %v", got)
	}
	if got[0] != "欢迎 洛洛的小小小" {
		t.Errorf("= %q，应取非空昵称且不重复", got[0])
	}
}

func TestEngineCooldownBlocksRepeat(t *testing.T) {
	bot := &recordBot{}
	e := newTestEngine(t, []Rule{{
		Name: "有冷却", Enabled: true, On: []event.Type{event.TypeDanmaku},
		Cooldown: time.Hour,
		Do:       []Action{{Type: ActionDanmaku, Template: []string{"回复"}}},
	}}, bot)

	for i := 0; i < 5; i++ {
		e.Handle(mkDanmaku("1", "甲", "你好", 0))
	}
	if got := bot.sent(); len(got) != 1 {
		t.Errorf("冷却期内只应发一次，实际 %v", got)
	}
}

func TestEngineCooldownGroupShared(t *testing.T) {
	bot := &recordBot{}
	e, err := NewEngine(EngineOptions{
		Label:          "测试号@1",
		RoomID:         "1",
		CooldownGroups: map[string]time.Duration{"greeting": time.Hour},
		Bot:            bot,
		Rules: []Rule{
			{Name: "规则A", Enabled: true, On: []event.Type{event.TypeDanmaku},
				CooldownGroup: "greeting",
				Do:            []Action{{Type: ActionDanmaku, Template: []string{"A"}}}},
			{Name: "规则B", Enabled: true, On: []event.Type{event.TypeDanmaku},
				CooldownGroup: "greeting",
				Do:            []Action{{Type: ActionDanmaku, Template: []string{"B"}}}},
		},
	})
	if err != nil {
		t.Fatalf("NewEngine 失败: %v", err)
	}
	defer e.Close()

	e.Handle(mkDanmaku("1", "甲", "你好", 0))

	if got := bot.sent(); len(got) != 1 {
		t.Errorf("同组规则应共享节流，实际 %v", got)
	}
}

func TestEngineMultipleRulesAllFire(t *testing.T) {
	bot := &recordBot{}
	e := newTestEngine(t, []Rule{
		{Name: "规则A", Enabled: true, On: []event.Type{event.TypeDanmaku},
			Do: []Action{{Type: ActionDanmaku, Template: []string{"A"}}}},
		{Name: "规则B", Enabled: true, On: []event.Type{event.TypeDanmaku},
			Do: []Action{{Type: ActionDanmaku, Template: []string{"B"}}}},
	}, bot)

	e.Handle(mkDanmaku("1", "甲", "你好", 0))

	got := bot.sent()
	if len(got) != 2 || got[0] != "A" || got[1] != "B" {
		t.Errorf("两条规则都应触发且保持顺序，实际 %v", got)
	}
}

func TestEngineBlockAction(t *testing.T) {
	bot := &recordBot{}
	e := newTestEngine(t, []Rule{{
		Name: "关键词禁言", Enabled: true, On: []event.Type{event.TypeDanmaku},
		When: &Condition{Field: "text", Op: "contains", Value: "广告"},
		Do:   []Action{{Type: ActionBlock, Hours: 1}},
	}}, bot)

	e.Handle(mkDanmaku("999", "坏人", "这是广告", 0))

	if got := bot.blocked(); len(got) != 1 || got[0] != "999" {
		t.Errorf("= %v", got)
	}
}

func TestEngineScriptAction(t *testing.T) {
	bot := &recordBot{}
	e := newTestEngine(t, []Rule{{
		Name: "脚本", Enabled: true, On: []event.Type{event.TypeDanmaku},
		Do: []Action{{Type: ActionScript,
			Script: `if (event.text.length > 2) { bot.sendDanmaku("长弹幕：" + event.user.username) }`}},
	}}, bot)

	e.Handle(mkDanmaku("1", "甲", "很长的一条弹幕", 0))

	if got := bot.sent(); len(got) != 1 || got[0] != "长弹幕：甲" {
		t.Errorf("= %v", got)
	}
}

func TestEngineScriptCondition(t *testing.T) {
	bot := &recordBot{}
	e := newTestEngine(t, []Rule{{
		Name: "脚本条件", Enabled: true, On: []event.Type{event.TypeDanmaku},
		When: &Condition{Script: `event.text.length > 5`},
		Do:   []Action{{Type: ActionDanmaku, Template: []string{"命中"}}},
	}}, bot)

	e.Handle(mkDanmaku("1", "甲", "短", 0))
	e.Handle(mkDanmaku("1", "甲", "这是一条很长的弹幕", 0))

	if got := bot.sent(); len(got) != 1 {
		t.Errorf("只有长弹幕应命中，实际 %v", got)
	}
}

func TestEngineStorageAcrossRuns(t *testing.T) {
	bot := &recordBot{}
	e := newTestEngine(t, []Rule{{
		Name: "计数", Enabled: true, On: []event.Type{event.TypeDanmaku},
		Do: []Action{{Type: ActionScript, Script: `
			var n = parseInt(storage.get("计数") || "0") + 1;
			storage.set("计数", String(n));
			bot.sendDanmaku("第 " + n + " 条");
		`}},
	}}, bot)

	e.Handle(mkDanmaku("1", "甲", "a", 0))
	e.Handle(mkDanmaku("1", "甲", "b", 0))
	e.Handle(mkDanmaku("1", "甲", "c", 0))

	got := bot.sent()
	if len(got) != 3 || got[0] != "第 1 条" || got[2] != "第 3 条" {
		t.Errorf("storage 应跨次保持，实际 %v", got)
	}
}

func TestEngineRuleErrorIsolation(t *testing.T) {
	bot := &recordBot{}
	e := newTestEngine(t, []Rule{
		{Name: "坏脚本", Enabled: true, On: []event.Type{event.TypeDanmaku},
			Do: []Action{{Type: ActionScript, Script: `null.foo`}}},
		{Name: "正常规则", Enabled: true, On: []event.Type{event.TypeDanmaku},
			Do: []Action{{Type: ActionDanmaku, Template: []string{"正常"}}}},
	}, bot)

	e.Handle(mkDanmaku("1", "甲", "你好", 0))

	if got := bot.sent(); len(got) != 1 || got[0] != "正常" {
		t.Errorf("单条规则出错不应影响其他规则，实际 %v", got)
	}
}

func TestEngineIgnoresUnmatchedEventType(t *testing.T) {
	bot := &recordBot{}
	e := newTestEngine(t, []Rule{{
		Name: "只管礼物", Enabled: true, On: []event.Type{event.TypeGift},
		Do: []Action{{Type: ActionDanmaku, Template: []string{"谢谢"}}},
	}}, bot)

	e.Handle(mkDanmaku("1", "甲", "你好", 0))

	if got := bot.sent(); len(got) != 0 {
		t.Errorf("不匹配的事件类型不应触发，实际 %v", got)
	}
}

func TestEngineFireScheduled(t *testing.T) {
	bot := &recordBot{}
	e := newTestEngine(t, []Rule{{
		Name: "定时广告", Enabled: true, Schedule: "0 */5 * * * *",
		Do: []Action{{Type: ActionDanmaku, Template: []string{"关注主播不迷路"}}},
	}}, bot)

	if names := e.ScheduledRules(); len(names) != 1 || names[0].Name != "定时广告" {
		t.Fatalf("ScheduledRules = %v", names)
	}

	e.FireScheduled("定时广告")

	if got := bot.sent(); len(got) != 1 || got[0] != "关注主播不迷路" {
		t.Errorf("= %v", got)
	}
}

func TestEngineFireScheduledUnknownIsNoop(t *testing.T) {
	bot := &recordBot{}
	e := newTestEngine(t, nil, bot)
	e.FireScheduled("不存在的规则") // 不得 panic
	if got := bot.sent(); len(got) != 0 {
		t.Errorf("= %v", got)
	}
}

func TestEngineCloseFlushesPendingAggregates(t *testing.T) {
	bot := &recordBot{}
	e, err := NewEngine(EngineOptions{
		Label: "测试号@1", RoomID: "1", Bot: bot,
		Rules: []Rule{{
			Name: "进场欢迎", Enabled: true, On: []event.Type{event.TypeUserEnter},
			Aggregate: &AggregateSpec{Window: time.Hour, By: AggregateByType},
			Do:        []Action{{Type: ActionDanmaku, Template: []string{`欢迎 {{join .users "、"}}`}}},
		}},
	})
	if err != nil {
		t.Fatalf("NewEngine 失败: %v", err)
	}

	e.Handle(mkEnter("1", "甲", 0))
	e.Close()

	if got := bot.sent(); len(got) != 1 {
		t.Errorf("Close 应结算未决窗口，实际 %v", got)
	}
}

func TestEngineHandleAfterCloseIsNoop(t *testing.T) {
	bot := &recordBot{}
	e := newTestEngine(t, []Rule{{
		Name: "复读", Enabled: true, On: []event.Type{event.TypeDanmaku},
		Do: []Action{{Type: ActionDanmaku, Template: []string{"回复"}}},
	}}, bot)

	e.Close()
	e.Handle(mkDanmaku("1", "甲", "你好", 0))

	if got := bot.sent(); len(got) != 0 {
		t.Errorf("关闭后不应再处理新事件，实际 %v", got)
	}
}

func TestEngineRejectsInvalidRule(t *testing.T) {
	_, err := NewEngine(EngineOptions{
		Label: "测试号@1", RoomID: "1", Bot: &recordBot{},
		Rules: []Rule{{Name: "无动作", Enabled: true, On: []event.Type{event.TypeDanmaku}}},
	})
	if err == nil {
		t.Error("非法规则应在构造时报错，不允许带病运行")
	}
}

// 边界一：甲压制乙。命中同一次触发时，先执行的甲把乙标记为压制，
// 乙在这次分发里被完全跳过。
func TestEngineSuppressSkipsOtherRuleSameTrigger(t *testing.T) {
	bot := &recordBot{}
	e := newTestEngine(t, []Rule{
		{Name: "甲", Enabled: true, On: []event.Type{event.TypeUserEnter},
			Suppress: []string{"乙"},
			Do:       []Action{{Type: ActionDanmaku, Template: []string{"甲欢迎"}}}},
		{Name: "乙", Enabled: true, On: []event.Type{event.TypeUserEnter},
			Do: []Action{{Type: ActionDanmaku, Template: []string{"乙欢迎"}}}},
	}, bot)

	e.Handle(mkEnter("1", "舰长", 3))

	if got := bot.sent(); len(got) != 1 || got[0] != "甲欢迎" {
		t.Errorf("甲压制乙后，本次触发只应执行甲，实际 %v", got)
	}
}

// 边界二：压制只对同一次触发生效，不是全局开关。甲这次不命中（非舰长），
// 乙就该正常执行——上一次触发里甲对乙的压制不能延续到这一次。
func TestEngineSuppressOnlyAppliesToSameTrigger(t *testing.T) {
	bot := &recordBot{}
	e := newTestEngine(t, []Rule{
		{Name: "甲", Enabled: true, On: []event.Type{event.TypeUserEnter},
			When:     &Condition{Field: "user.guardLevel", Op: "gt", Value: 0},
			Suppress: []string{"乙"},
			Do:       []Action{{Type: ActionDanmaku, Template: []string{"甲欢迎"}}}},
		{Name: "乙", Enabled: true, On: []event.Type{event.TypeUserEnter},
			Do: []Action{{Type: ActionDanmaku, Template: []string{"乙欢迎"}}}},
	}, bot)

	e.Handle(mkEnter("1", "舰长", 3)) // 甲命中 -> 压制乙，乙这次不执行
	e.Handle(mkEnter("2", "普通", 0)) // 甲这次不命中（非舰长）-> 乙应正常执行

	got := bot.sent()
	if len(got) != 2 || got[0] != "甲欢迎" || got[1] != "乙欢迎" {
		t.Errorf("压制只应对同一次触发生效，实际 %v", got)
	}
}

// 边界三：循环压制（甲压乙、乙压甲）不会死循环。按 position 顺序处理：
// 甲先执行，把乙标记为压制；乙被跳过，乙对甲的压制永远不会生效。
// 谁在前谁生效，这取决于规则在 Rules 切片中的顺序（position）。
func TestEngineSuppressCyclicDoesNotDeadlock(t *testing.T) {
	bot := &recordBot{}
	e := newTestEngine(t, []Rule{
		{Name: "甲", Enabled: true, On: []event.Type{event.TypeUserEnter},
			Suppress: []string{"乙"},
			Do:       []Action{{Type: ActionDanmaku, Template: []string{"甲欢迎"}}}},
		{Name: "乙", Enabled: true, On: []event.Type{event.TypeUserEnter},
			Suppress: []string{"甲"},
			Do:       []Action{{Type: ActionDanmaku, Template: []string{"乙欢迎"}}}},
	}, bot)

	e.Handle(mkEnter("1", "舰长", 3))

	if got := bot.sent(); len(got) != 1 || got[0] != "甲欢迎" {
		t.Errorf("甲在前应生效并压制乙，乙不应执行，实际 %v", got)
	}
}

// 反过来，乙在前时应是乙生效、压制甲——证明结果确由 position 决定，
// 不是巧合地总选中某个名字。
func TestEngineSuppressCyclicOrderDeterminesWinner(t *testing.T) {
	bot := &recordBot{}
	e := newTestEngine(t, []Rule{
		{Name: "乙", Enabled: true, On: []event.Type{event.TypeUserEnter},
			Suppress: []string{"甲"},
			Do:       []Action{{Type: ActionDanmaku, Template: []string{"乙欢迎"}}}},
		{Name: "甲", Enabled: true, On: []event.Type{event.TypeUserEnter},
			Suppress: []string{"乙"},
			Do:       []Action{{Type: ActionDanmaku, Template: []string{"甲欢迎"}}}},
	}, bot)

	e.Handle(mkEnter("1", "舰长", 3))

	if got := bot.sent(); len(got) != 1 || got[0] != "乙欢迎" {
		t.Errorf("乙在前应生效并压制甲，甲不应执行，实际 %v", got)
	}
}

// 边界四：被压制的规则若配了 aggregate，事件不应进入它的合并窗口。
// 喂入点（Aggregator.Add）与 fireLocked 在 Handle 里是同一处 if/else
// 分支，压制检查若只挡 fireLocked、不挡 agg.Add，被压制规则的窗口
// 仍会攒事件，窗口到期后还是会触发——这不是「不该再触发」的本意，
// 所以要求完全跳过：事件既不触发、也不进入合并窗口。
func TestEngineSuppressSkipsAggregateWindowEntirely(t *testing.T) {
	bot := &recordBot{}
	e := newTestEngine(t, []Rule{
		{Name: "甲", Enabled: true, On: []event.Type{event.TypeUserEnter},
			Suppress: []string{"乙"},
			Do:       []Action{{Type: ActionDanmaku, Template: []string{"甲欢迎"}}}},
		{Name: "乙", Enabled: true, On: []event.Type{event.TypeUserEnter},
			Aggregate: &AggregateSpec{Window: 60 * time.Millisecond, By: AggregateByType},
			Do:        []Action{{Type: ActionDanmaku, Template: []string{`乙欢迎 {{join .users "、"}}`}}}},
	}, bot)

	e.Handle(mkEnter("1", "舰长", 3)) // 甲命中并压制乙，事件不该进入乙的窗口

	time.Sleep(150 * time.Millisecond) // 等乙的窗口本该到期的时间

	if got := bot.sent(); len(got) != 1 || got[0] != "甲欢迎" {
		t.Errorf("乙被压制应完全跳过（含不入合并窗口），实际 %v", got)
	}
}

// 边界一（校验侧）：压制不存在的规则名要在构造时报错，而不是静默不生效——
// 写错规则名而查不出原因，比直接拒绝启动更难排查。
func TestNewEngineRejectsSuppressUnknownRuleName(t *testing.T) {
	_, err := NewEngine(EngineOptions{
		Label: "测试号@1", RoomID: "1", Bot: &recordBot{},
		Rules: []Rule{{
			Name: "甲", Enabled: true, On: []event.Type{event.TypeUserEnter},
			Suppress: []string{"不存在的规则"},
			Do:       []Action{{Type: ActionDanmaku, Template: []string{"欢迎"}}},
		}},
	})
	if err == nil {
		t.Fatal("压制不存在的规则名应在构造时报错")
	}
	if !strings.Contains(err.Error(), "甲") || !strings.Contains(err.Error(), "不存在的规则") {
		t.Errorf("错误信息应同时说明是哪条规则压制了哪个不存在的名字，实际: %v", err)
	}
}

func TestMemStorage(t *testing.T) {
	s := NewMemStorage()
	if _, ok := s.Get("空"); ok {
		t.Error("未写入的键应返回 false")
	}
	s.Set("键", "值")
	if v, ok := s.Get("键"); !ok || v != "值" {
		t.Errorf("Get = %q %v", v, ok)
	}
	s.Set("键", "新值")
	if v, _ := s.Get("键"); v != "新值" {
		t.Errorf("覆盖失败: %q", v)
	}
}

func TestMemStorageConcurrent(t *testing.T) {
	s := NewMemStorage()
	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			s.Set("键", "值")
			s.Get("键")
		}()
	}
	wg.Wait()
}
