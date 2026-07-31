package main

import (
	"context"
	"errors"
	"sync"
	"testing"
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
