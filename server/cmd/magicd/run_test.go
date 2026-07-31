package main

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/MrZhongzq/MagicalDanmaku/server/internal/event"
	"github.com/MrZhongzq/MagicalDanmaku/server/internal/logging"
	"github.com/MrZhongzq/MagicalDanmaku/server/internal/rules"
	"github.com/MrZhongzq/MagicalDanmaku/server/internal/store"
)

// bindingStub 记录 roomBot 转发给绑定的调用。
// 它实现 run.go 中的 danmakuSender 接口，因此无需引入 account 包。
type bindingStub struct {
	mu     sync.Mutex
	sent   []string
	blocks []blockRecord
	err    error
}

type blockRecord struct {
	uid   string
	hours int
}

func (b *bindingStub) SendDanmaku(ctx context.Context, text string) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.err != nil {
		return b.err
	}
	b.sent = append(b.sent, text)
	return nil
}

func (b *bindingStub) Block(ctx context.Context, uid string, hours int) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.err != nil {
		return b.err
	}
	b.blocks = append(b.blocks, blockRecord{uid, hours})
	return nil
}

func TestRoomBotForwardsToBinding(t *testing.T) {
	bs := &bindingStub{}
	b := &roomBot{binding: bs, ctx: context.Background()}

	if err := b.SendDanmaku("你好"); err != nil {
		t.Fatalf("SendDanmaku 失败: %v", err)
	}
	if len(bs.sent) != 1 || bs.sent[0] != "你好" {
		t.Errorf("sent = %v", bs.sent)
	}

	if err := b.Block("999", 12); err != nil {
		t.Fatalf("Block 失败: %v", err)
	}
	if len(bs.blocks) != 1 || bs.blocks[0].uid != "999" || bs.blocks[0].hours != 12 {
		t.Errorf("blocks = %v", bs.blocks)
	}
}

func TestRoomBotPropagatesError(t *testing.T) {
	bs := &bindingStub{err: errors.New("发送失败")}
	b := &roomBot{binding: bs, ctx: context.Background()}

	if err := b.SendDanmaku("x"); err == nil {
		t.Error("底层错误应当上报")
	}
	if err := b.Block("1", 1); err == nil {
		t.Error("底层错误应当上报")
	}
}

func contains(s, sub string) bool {
	return len(s) >= len(sub) && (func() bool {
		for i := 0; i+len(sub) <= len(s); i++ {
			if s[i:i+len(sub)] == sub {
				return true
			}
		}
		return false
	})()
}

func TestRunRequiresDatabase(t *testing.T) {
	t.Setenv("MAGICD_DATABASE_URL", "")
	err := runRun([]string{})
	if err == nil {
		t.Fatal("没有数据库连接串应报错")
	}
	if !contains(err.Error(), "MAGICD_DATABASE_URL") {
		t.Errorf("错误信息应提示怎么配置，实际: %v", err)
	}
}

func TestRunRejectsUnreachableDatabase(t *testing.T) {
	// 端口 1 上不会有 PostgreSQL
	err := runRun([]string{"-db", "postgres://x:y@127.0.0.1:1/z?sslmode=disable&connect_timeout=1"})
	if err == nil {
		t.Fatal("连不上数据库应报错")
	}
}

func TestRetentionDaysFromEnv(t *testing.T) {
	t.Setenv("MAGICD_LOG_RETENTION_DAYS", "7")
	if got := retentionDays(); got != 7 {
		t.Errorf("= %d, 期望 7", got)
	}
}

func TestRetentionDaysDefault(t *testing.T) {
	t.Setenv("MAGICD_LOG_RETENTION_DAYS", "")
	if got := retentionDays(); got != 30 {
		t.Errorf("默认应为 30，实际 %d", got)
	}
}

func TestRetentionDaysZeroMeansNoPurge(t *testing.T) {
	t.Setenv("MAGICD_LOG_RETENTION_DAYS", "0")
	if got := retentionDays(); got != 0 {
		t.Errorf("0 表示不清理，实际 %d", got)
	}
}

func TestRetentionDaysIgnoresGarbage(t *testing.T) {
	// 环境变量写错就退回默认值，不该让机器人起不来
	t.Setenv("MAGICD_LOG_RETENTION_DAYS", "三十天")
	if got := retentionDays(); got != 30 {
		t.Errorf("非法值应退回默认 30，实际 %d", got)
	}
}

// TestCloseAllSettlesEngineWindowsBeforeFlushingWriter 验证 closeAll 的
// 关闭顺序：引擎必须先结算未决的合并窗口，写入器才能把结算产生的动作
// 日志冲刷出去。
//
// 用一个窗口设为 1 小时（必然不会自然到期）的聚合规则模拟「攒着的欢迎语」：
// 只有 engine.Close() 被调用，这条动作日志才会产生；只有此后
// activity.Close() 被调用，它才会被冲刷进 Flush。若 closeAll 的顺序反了
// （或者压根没关引擎），这条日志就不会出现在 flushed 里——这正是
// review 指出的、早返回路径曾经会触发的那个问题。
func TestCloseAllSettlesEngineWindowsBeforeFlushingWriter(t *testing.T) {
	var mu sync.Mutex
	var flushed []store.ActivityRow

	activity := logging.NewActivityWriter(logging.ActivityWriterOptions{
		Flush: func(_ context.Context, rows []store.ActivityRow) error {
			mu.Lock()
			defer mu.Unlock()
			flushed = append(flushed, rows...)
			return nil
		},
	})

	eng, err := rules.NewEngine(rules.EngineOptions{
		RoomID:   "123",
		Bot:      &roomBot{binding: &bindingStub{}, ctx: context.Background()},
		Activity: activity.Sink(1, 1, "123"),
		Rules: []rules.Rule{{
			Name:      "进场欢迎",
			Enabled:   true,
			On:        []event.Type{event.TypeUserEnter},
			Aggregate: &rules.AggregateSpec{Window: time.Hour, By: rules.AggregateByType},
			Do:        []rules.Action{{Type: rules.ActionDanmaku, Template: []string{"欢迎"}}},
		}},
	})
	if err != nil {
		t.Fatalf("创建引擎失败: %v", err)
	}

	eng.Handle(event.Event{
		Type:    event.TypeUserEnter,
		Payload: event.UserEnter{User: event.User{UID: "1", Username: "观众"}},
	})

	closeAll([]*rules.Engine{eng}, activity)

	mu.Lock()
	defer mu.Unlock()
	for _, r := range flushed {
		if r.Kind == store.ActivityAction {
			return // 找到了结算产生的动作日志，顺序正确
		}
	}
	t.Error("closeAll 应先让引擎结算未决窗口，再让写入器冲刷——" +
		"未在 flushed 里找到窗口结算产生的动作日志")
}
