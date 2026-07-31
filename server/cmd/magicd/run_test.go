package main

import (
	"context"
	"errors"
	"os"
	"path/filepath"
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

func TestRunRejectsMissingConfig(t *testing.T) {
	err := runRun([]string{"-c", "/不存在的路径/config.yaml"})
	if err == nil {
		t.Error("配置文件不存在应当报错")
	}
}

func TestRunRejectsEmptyConfigFlag(t *testing.T) {
	if err := runRun([]string{}); err == nil {
		t.Error("未指定 -c 应当报错")
	}
}

func TestRunRejectsMissingCookieFile(t *testing.T) {
	dir := t.TempDir()
	cfg := filepath.Join(dir, "config.yaml")
	content := `
accounts:
  - name: 测试号
    cookieFile: /不存在的/cookie.txt
    rooms:
      - id: "1"
        rules:
          - name: 规则
            on: [danmaku]
            do: [{type: log}]
`
	if err := os.WriteFile(cfg, []byte(content), 0o600); err != nil {
		t.Fatalf("写入配置失败: %v", err)
	}

	err := runRun([]string{"-c", cfg})
	if err == nil {
		t.Fatal("Cookie 文件不存在应当报错")
	}
	// 错误信息要指出是哪个账号
	if !contains(err.Error(), "测试号") {
		t.Errorf("错误信息应含账号名，实际: %v", err)
	}
}

func TestRunRejectsInvalidCookie(t *testing.T) {
	dir := t.TempDir()
	cookie := filepath.Join(dir, "cookie.txt")
	if err := os.WriteFile(cookie, []byte("这不是合法的 Cookie"), 0o600); err != nil {
		t.Fatalf("写入 Cookie 失败: %v", err)
	}

	cfg := filepath.Join(dir, "config.yaml")
	content := `
accounts:
  - name: 测试号
    cookieFile: ` + cookie + `
    rooms:
      - id: "1"
        rules:
          - name: 规则
            on: [danmaku]
            do: [{type: log}]
`
	if err := os.WriteFile(cfg, []byte(content), 0o600); err != nil {
		t.Fatalf("写入配置失败: %v", err)
	}

	if err := runRun([]string{"-c", cfg}); err == nil {
		t.Error("非法 Cookie 应当报错")
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
