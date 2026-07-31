package rules

import (
	"context"
	"errors"
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
