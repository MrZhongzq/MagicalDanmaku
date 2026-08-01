package rules

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"math/rand"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/MrZhongzq/MagicalDanmaku/server/internal/event"
)

// failingBot 让指定次数的发送失败，用于测试错误隔离。
type failingBot struct {
	mu       sync.Mutex
	danmakus []string
	blocks   []blockCall
	failNext bool
}

type blockCall struct {
	uid   string
	hours int
}

func (f *failingBot) SendDanmaku(text string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.failNext {
		f.failNext = false
		return errors.New("模拟发送失败")
	}
	f.danmakus = append(f.danmakus, text)
	return nil
}

func (f *failingBot) Block(uid string, hours int) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.blocks = append(f.blocks, blockCall{uid, hours})
	return nil
}

func (f *failingBot) sent() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := make([]string, len(f.danmakus))
	copy(out, f.danmakus)
	return out
}

func newTestExecutor(bot BotAPI) *Executor {
	return NewExecutor(ExecutorOptions{
		Bot:               bot,
		Renderer:          NewRenderer(rand.New(rand.NewSource(1))),
		Script:            NewSandbox(SandboxOptions{Timeout: 200 * time.Millisecond, Bot: bot}),
		DefaultBlockHours: 1,
	})
}

func enterTrigger(uid, name string) Trigger {
	return PassthroughTrigger(event.Event{
		Type: event.TypeUserEnter, RoomID: "1", Timestamp: time.Unix(1700000000, 0),
		Payload: event.UserEnter{User: event.User{UID: uid, Username: name}},
	})
}

// multiTrigger 构造一个合并触发的 Trigger，模拟 Aggregator.mergeBuckets
// 产出的 count（实际类型是 int，见 aggregate.go）。
func multiTrigger(count int, users ...string) Trigger {
	tr := enterTrigger("1", users[0])
	tr.Vars["count"] = count
	tr.Vars["users"] = users
	return tr
}

func TestExecuteDanmakuAction(t *testing.T) {
	bot := &failingBot{}
	ex := newTestExecutor(bot)

	r := Rule{Name: "欢迎", Do: []Action{
		{Type: ActionDanmaku, Template: []string{"欢迎 {{.user.username}}"}},
	}}
	if err := ex.Execute(context.Background(), r, enterTrigger("1", "甲")); err != nil {
		t.Fatalf("Execute 失败: %v", err)
	}

	sent := bot.sent()
	if len(sent) != 1 || sent[0] != "欢迎 甲" {
		t.Errorf("发送内容 = %v", sent)
	}
}

func TestExecuteDanmakuUsesMergedUsers(t *testing.T) {
	bot := &failingBot{}
	ex := newTestExecutor(bot)

	tr := enterTrigger("1", "甲")
	tr.Vars["users"] = []string{"甲", "乙", "丙"}
	tr.Vars["count"] = 3

	r := Rule{Name: "欢迎", Do: []Action{
		{Type: ActionDanmaku, Template: []string{`欢迎 {{join .users "、"}} 回家`}},
	}}
	if err := ex.Execute(context.Background(), r, tr); err != nil {
		t.Fatalf("Execute 失败: %v", err)
	}
	if got := bot.sent(); len(got) != 1 || got[0] != "欢迎 甲、乙、丙 回家" {
		t.Errorf("= %v", got)
	}
}

// 行为 1：count == 1 用 Template。
func TestSendDanmakuUsesTemplateForSingleCount(t *testing.T) {
	bot := &failingBot{}
	ex := newTestExecutor(bot)

	r := Rule{Name: "欢迎", Do: []Action{
		{Type: ActionDanmaku,
			Template:      []string{"欢迎 {{.user.username}} 回家"},
			TemplateMulti: []string{"欢迎 {{join .users \"、\"}} 回家"}},
	}}
	tr := multiTrigger(1, "甲")
	if err := ex.Execute(context.Background(), r, tr); err != nil {
		t.Fatalf("Execute 失败: %v", err)
	}
	if got := bot.sent(); len(got) != 1 || got[0] != "欢迎 甲 回家" {
		t.Errorf("count=1 应使用 Template，实际 = %v", got)
	}
}

// 行为 2：count > 1 且 TemplateMulti 非空，用 TemplateMulti。
func TestSendDanmakuUsesTemplateMultiForMultiCount(t *testing.T) {
	bot := &failingBot{}
	ex := newTestExecutor(bot)

	r := Rule{Name: "欢迎", Do: []Action{
		{Type: ActionDanmaku,
			Template:      []string{"欢迎 {{.user.username}} 回家"},
			TemplateMulti: []string{"欢迎 {{join .users \"、\"}} 回家"}},
	}}
	tr := multiTrigger(3, "甲", "乙", "丙")
	if err := ex.Execute(context.Background(), r, tr); err != nil {
		t.Fatalf("Execute 失败: %v", err)
	}
	if got := bot.sent(); len(got) != 1 || got[0] != "欢迎 甲、乙、丙 回家" {
		t.Errorf("count>1 且 TemplateMulti 非空应使用 TemplateMulti，实际 = %v", got)
	}
}

// 行为 3：count > 1 但 TemplateMulti 为空，回落到 Template（兼容旧配置，
// 不是报错）。
func TestSendDanmakuFallsBackToTemplateWhenMultiEmpty(t *testing.T) {
	bot := &failingBot{}
	ex := newTestExecutor(bot)

	r := Rule{Name: "欢迎", Do: []Action{
		{Type: ActionDanmaku, Template: []string{"欢迎 {{join .users \"、\"}} 回家"}},
	}}
	tr := multiTrigger(3, "甲", "乙", "丙")
	if err := ex.Execute(context.Background(), r, tr); err != nil {
		t.Fatalf("Execute 失败: %v", err)
	}
	if got := bot.sent(); len(got) != 1 || got[0] != "欢迎 甲、乙、丙 回家" {
		t.Errorf("TemplateMulti 为空时应回落到 Template，实际 = %v", got)
	}
}

// 行为 4：count 缺失或类型不对时当 1 处理（走单人模板），不能 panic。
func TestSendDanmakuTreatsMissingOrBadCountAsSingle(t *testing.T) {
	r := Rule{Name: "欢迎", Do: []Action{
		{Type: ActionDanmaku,
			Template:      []string{"单人模板"},
			TemplateMulti: []string{"多人模板"}},
	}}

	t.Run("缺失", func(t *testing.T) {
		bot := &failingBot{}
		ex := newTestExecutor(bot)
		tr := enterTrigger("1", "甲")
		delete(tr.Vars, "count")
		if err := ex.Execute(context.Background(), r, tr); err != nil {
			t.Fatalf("Execute 失败: %v", err)
		}
		if got := bot.sent(); len(got) != 1 || got[0] != "单人模板" {
			t.Errorf("count 缺失应当 1 处理，实际 = %v", got)
		}
	})

	t.Run("类型不对_字符串", func(t *testing.T) {
		bot := &failingBot{}
		ex := newTestExecutor(bot)
		tr := enterTrigger("1", "甲")
		tr.Vars["count"] = "很多个"
		if err := ex.Execute(context.Background(), r, tr); err != nil {
			t.Fatalf("Execute 失败: %v", err)
		}
		if got := bot.sent(); len(got) != 1 || got[0] != "单人模板" {
			t.Errorf("count 类型不对应当 1 处理，实际 = %v", got)
		}
	})

	t.Run("类型不对_float64", func(t *testing.T) {
		// 例如经过一趟 JSON 反序列化后数字可能变成 float64
		bot := &failingBot{}
		ex := newTestExecutor(bot)
		tr := enterTrigger("1", "甲")
		tr.Vars["count"] = float64(5)
		if err := ex.Execute(context.Background(), r, tr); err != nil {
			t.Fatalf("Execute 失败: %v", err)
		}
		if got := bot.sent(); len(got) != 1 || got[0] != "单人模板" {
			t.Errorf("count 类型不对应当 1 处理，实际 = %v", got)
		}
	})
}

// 行为 5：TemplateMulti 与 Pick 组合时，轮询游标对两套模板是分开的。
//
// 共用一个游标的话，单人触发推进的游标会让多人模板跳着走（反之亦然）：
// 交替发生单人/多人触发，若游标共用，第二次多人触发本该拿 TemplateMulti
// 的第 1 条，却会被中间插入的单人触发偷偷推进到第 2 条。
func TestSequentialCursorSeparatesTemplateAndTemplateMulti(t *testing.T) {
	bot := &failingBot{}
	ex := newTestExecutor(bot)

	r := Rule{Name: "轮询欢迎", Do: []Action{
		{Type: ActionDanmaku, Pick: PickSequential,
			Template:      []string{"单1", "单2", "单3"},
			TemplateMulti: []string{"多1", "多2", "多3"}},
	}}

	// 交替：单、多、单、多、单、多
	seq := []Trigger{
		multiTrigger(1, "甲"),
		multiTrigger(2, "甲", "乙"),
		multiTrigger(1, "甲"),
		multiTrigger(2, "甲", "乙"),
		multiTrigger(1, "甲"),
		multiTrigger(2, "甲", "乙"),
	}
	for i, tr := range seq {
		if err := ex.Execute(context.Background(), r, tr); err != nil {
			t.Fatalf("第 %d 次 Execute 失败: %v", i, err)
		}
	}

	got := bot.sent()
	want := []string{"单1", "多1", "单2", "多2", "单3", "多3"}
	if len(got) != len(want) {
		t.Fatalf("发送次数 = %d, 期望 %d, 实际 %v", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("第 %d 条 = %q, 期望 %q（全部: %v）", i, got[i], want[i], got)
		}
	}
}

func TestExecuteBlockAction(t *testing.T) {
	bot := &failingBot{}
	ex := newTestExecutor(bot)

	r := Rule{Name: "禁言", Do: []Action{{Type: ActionBlock, Hours: 12}}}
	if err := ex.Execute(context.Background(), r, enterTrigger("999", "坏人")); err != nil {
		t.Fatalf("Execute 失败: %v", err)
	}

	if len(bot.blocks) != 1 {
		t.Fatalf("禁言次数 = %d", len(bot.blocks))
	}
	if bot.blocks[0].uid != "999" || bot.blocks[0].hours != 12 {
		t.Errorf("blocks[0] = %+v", bot.blocks[0])
	}
}

func TestExecuteBlockUsesDefaultHours(t *testing.T) {
	bot := &failingBot{}
	ex := newTestExecutor(bot)

	r := Rule{Name: "禁言", Do: []Action{{Type: ActionBlock}}} // Hours 未指定
	if err := ex.Execute(context.Background(), r, enterTrigger("999", "坏人")); err != nil {
		t.Fatalf("Execute 失败: %v", err)
	}
	if bot.blocks[0].hours != 1 {
		t.Errorf("hours = %d, 期望使用默认值 1", bot.blocks[0].hours)
	}
}

func TestExecuteBlockAppliesToAllMergedUsers(t *testing.T) {
	bot := &failingBot{}
	ex := newTestExecutor(bot)

	// 合并后的 Trigger 含多个事件，禁言应作用于全部参与者
	tr := Trigger{
		Type: event.TypeDanmaku,
		Events: []event.Event{
			{Type: event.TypeDanmaku, Payload: event.Danmaku{User: event.User{UID: "1"}}},
			{Type: event.TypeDanmaku, Payload: event.Danmaku{User: event.User{UID: "2"}}},
		},
		Vars: map[string]any{"user": map[string]any{"uid": "1"}},
	}
	r := Rule{Name: "禁言", Do: []Action{{Type: ActionBlock, Hours: 1}}}
	if err := ex.Execute(context.Background(), r, tr); err != nil {
		t.Fatalf("Execute 失败: %v", err)
	}
	if len(bot.blocks) != 2 {
		t.Errorf("应禁言 2 个用户，实际 %d", len(bot.blocks))
	}
}

func TestExecuteScriptAction(t *testing.T) {
	bot := &failingBot{}
	ex := newTestExecutor(bot)

	r := Rule{Name: "脚本", Do: []Action{
		{Type: ActionScript, Script: `bot.sendDanmaku("来自脚本: " + event.user.username)`},
	}}
	if err := ex.Execute(context.Background(), r, enterTrigger("1", "甲")); err != nil {
		t.Fatalf("Execute 失败: %v", err)
	}
	if got := bot.sent(); len(got) != 1 || got[0] != "来自脚本: 甲" {
		t.Errorf("= %v", got)
	}
}

func TestExecuteLogAction(t *testing.T) {
	bot := &failingBot{}
	ex := newTestExecutor(bot)

	r := Rule{Name: "只记日志", Do: []Action{{Type: ActionLog}}}
	if err := ex.Execute(context.Background(), r, enterTrigger("1", "甲")); err != nil {
		t.Fatalf("Execute 失败: %v", err)
	}
	if len(bot.sent()) != 0 {
		t.Error("log 动作不应发送弹幕")
	}
}

// sequential 模式下单个动作按顺序轮流用模板。
func TestExecuteSequentialPickCyclesTemplates(t *testing.T) {
	bot := &failingBot{}
	ex := newTestExecutor(bot)

	r := Rule{Name: "轮询问好", Do: []Action{
		{Type: ActionDanmaku, Pick: PickSequential, Template: []string{"甲1", "甲2", "甲3"}},
	}}
	for i := 0; i < 4; i++ {
		if err := ex.Execute(context.Background(), r, enterTrigger("1", "x")); err != nil {
			t.Fatalf("第 %d 次 Execute 失败: %v", i, err)
		}
	}

	got := bot.sent()
	want := []string{"甲1", "甲2", "甲3", "甲1"}
	if len(got) != len(want) {
		t.Fatalf("发送次数 = %d, 期望 %d, 实际 %v", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("第 %d 次 = %q, 期望 %q（全部: %v）", i, got[i], want[i], got)
		}
	}
}

// 同一条规则的两个发弹幕动作各有各的游标，不互相打乱。
//
// 共用一个游标的话，两个动作会交替推进它——第一个动作拿甲、
// 第二个拿乙、下一轮第一个拿丙，每个动作看到的都是跳着的。
func TestSequentialCursorIsPerAction(t *testing.T) {
	bot := &failingBot{}
	ex := newTestExecutor(bot)

	r := Rule{Name: "双动作轮询", Do: []Action{
		{Type: ActionDanmaku, Pick: PickSequential, Template: []string{"甲1", "甲2", "甲3"}},
		{Type: ActionDanmaku, Pick: PickSequential, Template: []string{"乙1", "乙2", "乙3"}},
	}}

	for i := 0; i < 3; i++ {
		if err := ex.Execute(context.Background(), r, enterTrigger("1", "x")); err != nil {
			t.Fatalf("第 %d 次 Execute 失败: %v", i, err)
		}
	}

	got := bot.sent()
	want := []string{
		"甲1", "乙1",
		"甲2", "乙2",
		"甲3", "乙3",
	}
	if len(got) != len(want) {
		t.Fatalf("发送次数 = %d, 期望 %d, 实际 %v", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("第 %d 条 = %q, 期望 %q（全部: %v）", i, got[i], want[i], got)
		}
	}
}

func TestExecuteRunsAllActionsInOrder(t *testing.T) {
	bot := &failingBot{}
	ex := newTestExecutor(bot)

	r := Rule{Name: "多动作", Do: []Action{
		{Type: ActionDanmaku, Template: []string{"第一条"}},
		{Type: ActionDanmaku, Template: []string{"第二条"}},
	}}
	if err := ex.Execute(context.Background(), r, enterTrigger("1", "甲")); err != nil {
		t.Fatalf("Execute 失败: %v", err)
	}
	got := bot.sent()
	if len(got) != 2 || got[0] != "第一条" || got[1] != "第二条" {
		t.Errorf("= %v", got)
	}
}

func TestExecuteContinuesAfterActionFailure(t *testing.T) {
	bot := &failingBot{failNext: true}
	ex := newTestExecutor(bot)

	r := Rule{Name: "多动作", Do: []Action{
		{Type: ActionDanmaku, Template: []string{"会失败的"}},
		{Type: ActionDanmaku, Template: []string{"应当仍被执行"}},
	}}
	err := ex.Execute(context.Background(), r, enterTrigger("1", "甲"))
	if err == nil {
		t.Error("有动作失败时应返回错误")
	}

	got := bot.sent()
	if len(got) != 1 || got[0] != "应当仍被执行" {
		t.Errorf("单个动作失败不应中断后续动作，实际 %v", got)
	}
}

func TestExecuteBadTemplateDoesNotCrash(t *testing.T) {
	bot := &failingBot{}
	ex := newTestExecutor(bot)

	r := Rule{Name: "坏模板", Do: []Action{
		{Type: ActionDanmaku, Template: []string{"{{.未闭合"}},
		{Type: ActionDanmaku, Template: []string{"后续动作"}},
	}}
	err := ex.Execute(context.Background(), r, enterTrigger("1", "甲"))
	if err == nil {
		t.Error("模板错误应被上报")
	}
	if got := bot.sent(); len(got) != 1 || got[0] != "后续动作" {
		t.Errorf("模板错误不应中断后续动作，实际 %v", got)
	}
}

func TestExecuteEmptyRenderIsSkipped(t *testing.T) {
	bot := &failingBot{}
	ex := newTestExecutor(bot)

	// 模板渲染结果为空时不该发空弹幕
	r := Rule{Name: "空模板", Do: []Action{
		{Type: ActionDanmaku, Template: []string{"{{.不存在的字段}}"}},
	}}
	if err := ex.Execute(context.Background(), r, enterTrigger("1", "甲")); err != nil {
		t.Fatalf("Execute 失败: %v", err)
	}
	if len(bot.sent()) != 0 {
		t.Errorf("渲染为空时不应发送，实际 %v", bot.sent())
	}
}

func TestExecuteScriptTimeoutIsReported(t *testing.T) {
	bot := &failingBot{}
	ex := NewExecutor(ExecutorOptions{
		Bot:      bot,
		Renderer: NewRenderer(rand.New(rand.NewSource(1))),
		Script:   NewSandbox(SandboxOptions{Timeout: 50 * time.Millisecond, Bot: bot}),
	})

	r := Rule{Name: "死循环", Do: []Action{{Type: ActionScript, Script: `while(true){}`}}}
	err := ex.Execute(context.Background(), r, enterTrigger("1", "甲"))
	if err == nil {
		t.Fatal("脚本超时应被上报")
	}
	if !strings.Contains(err.Error(), "超时") {
		t.Errorf("错误信息应提及超时，实际 %v", err)
	}
}

// capturingHandler 捕获 slog 输出，用于断言日志行为。
type capturingHandler struct {
	mu      sync.Mutex
	records []string
}

func (h *capturingHandler) Enabled(context.Context, slog.Level) bool { return true }

func (h *capturingHandler) Handle(_ context.Context, r slog.Record) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	line := r.Message
	r.Attrs(func(a slog.Attr) bool {
		line += " " + a.Key + "=" + fmt.Sprint(a.Value.Any())
		return true
	})
	h.records = append(h.records, line)
	return nil
}

func (h *capturingHandler) WithAttrs([]slog.Attr) slog.Handler { return h }
func (h *capturingHandler) WithGroup(string) slog.Handler      { return h }

func (h *capturingHandler) all() []string {
	h.mu.Lock()
	defer h.mu.Unlock()
	out := make([]string, len(h.records))
	copy(out, h.records)
	return out
}

func TestExecuteLogsSuccessfulSend(t *testing.T) {
	// 只有出错才记日志的话，运维时无从知道机器人在做什么
	h := &capturingHandler{}
	bot := &failingBot{}
	ex := NewExecutor(ExecutorOptions{
		Bot:      bot,
		Renderer: NewRenderer(rand.New(rand.NewSource(1))),
		Logger:   slog.New(h),
	})

	r := Rule{Name: "进场欢迎", Do: []Action{
		{Type: ActionDanmaku, Template: []string{"欢迎 {{.user.username}}"}},
	}}
	if err := ex.Execute(context.Background(), r, enterTrigger("1", "甲")); err != nil {
		t.Fatalf("Execute 失败: %v", err)
	}

	var found bool
	for _, line := range h.all() {
		if strings.Contains(line, "已发送弹幕") &&
			strings.Contains(line, "进场欢迎") &&
			strings.Contains(line, "欢迎 甲") {
			found = true
		}
	}
	if !found {
		t.Errorf("成功发送应记日志且含规则名与内容，实际日志:\n%v", h.all())
	}
}

func TestExecuteLogsSuccessfulBlock(t *testing.T) {
	h := &capturingHandler{}
	bot := &failingBot{}
	ex := NewExecutor(ExecutorOptions{
		Bot:      bot,
		Renderer: NewRenderer(rand.New(rand.NewSource(1))),
		Logger:   slog.New(h),
	})

	r := Rule{Name: "广告禁言", Do: []Action{{Type: ActionBlock, Hours: 2}}}
	if err := ex.Execute(context.Background(), r, enterTrigger("999", "坏人")); err != nil {
		t.Fatalf("Execute 失败: %v", err)
	}

	var found bool
	for _, line := range h.all() {
		if strings.Contains(line, "已禁言用户") && strings.Contains(line, "999") {
			found = true
		}
	}
	if !found {
		t.Errorf("成功禁言应记日志，实际日志:\n%v", h.all())
	}
}

func TestExecuteDoesNotLogSkippedEmptyRender(t *testing.T) {
	// 渲染为空时静默跳过，不该谎报「已发送」
	h := &capturingHandler{}
	ex := NewExecutor(ExecutorOptions{
		Bot:      &failingBot{},
		Renderer: NewRenderer(rand.New(rand.NewSource(1))),
		Logger:   slog.New(h),
	})

	r := Rule{Name: "空模板", Do: []Action{
		{Type: ActionDanmaku, Template: []string{"{{.不存在}}"}},
	}}
	if err := ex.Execute(context.Background(), r, enterTrigger("1", "甲")); err != nil {
		t.Fatalf("Execute 失败: %v", err)
	}

	for _, line := range h.all() {
		if strings.Contains(line, "已发送弹幕") {
			t.Errorf("未真正发送时不该记「已发送」，实际: %q", line)
		}
	}
}
