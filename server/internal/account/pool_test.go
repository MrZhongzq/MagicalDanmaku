package account

import (
	"context"
	"errors"
	"sync"
	"testing"

	"github.com/MrZhongzq/MagicalDanmaku/server/internal/connector"
	"github.com/MrZhongzq/MagicalDanmaku/server/internal/connector/bilibili/api"
)

// stubActions 是 connector.Actions 的测试替身。
type stubActions struct {
	mu    sync.Mutex
	sent  []string
	err   error // 每次调用都返回该错误
	calls int
}

func (s *stubActions) SendDanmaku(ctx context.Context, req connector.SendDanmakuRequest) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.calls++
	if s.err != nil {
		return s.err
	}
	s.sent = append(s.sent, req.Text)
	return nil
}

func (s *stubActions) BlockUser(ctx context.Context, req connector.BlockRequest) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.calls++
	return s.err
}

func (s *stubActions) UnblockUser(ctx context.Context, roomID, uid string) error {
	return s.err
}

func (s *stubActions) count() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.calls
}

func (s *stubActions) sentCount() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.sent)
}

func TestPoolRoundRobin(t *testing.T) {
	a := &stubActions{}
	b := &stubActions{}
	p := New([]Account{{Name: "A", Actions: a}, {Name: "B", Actions: b}}, nil)

	ctx := context.Background()
	for i := 0; i < 4; i++ {
		if err := p.SendDanmaku(ctx, "1", "消息"); err != nil {
			t.Fatalf("第 %d 次发送失败: %v", i+1, err)
		}
	}

	if a.count() != 2 || b.count() != 2 {
		t.Errorf("应均匀轮询，A=%d B=%d", a.count(), b.count())
	}
}

func TestPoolSingleAccount(t *testing.T) {
	a := &stubActions{}
	p := New([]Account{{Name: "A", Actions: a}}, nil)

	for i := 0; i < 3; i++ {
		if err := p.SendDanmaku(context.Background(), "1", "消息"); err != nil {
			t.Fatalf("发送失败: %v", err)
		}
	}
	if a.count() != 3 {
		t.Errorf("单账号应全部承担，实际 %d", a.count())
	}
}

func TestPoolRemovesFatallyFailedAccount(t *testing.T) {
	bad := &stubActions{err: &api.APIError{Code: -101, Message: "账号未登录"}}
	good := &stubActions{}
	p := New([]Account{{Name: "坏账号", Actions: bad}, {Name: "好账号", Actions: good}}, nil)

	ctx := context.Background()
	// 第一次会打到坏账号，失败后应自动切到好账号并完成发送
	if err := p.SendDanmaku(ctx, "1", "消息"); err != nil {
		t.Fatalf("应自动切换到可用账号，实际失败: %v", err)
	}
	if good.sentCount() != 1 {
		t.Errorf("好账号应完成发送，实际发送数 %d", good.sentCount())
	}
	if p.Healthy() != 1 {
		t.Errorf("坏账号应被移出轮换，剩余健康账号 = %d", p.Healthy())
	}

	// 后续发送不应再尝试坏账号
	before := bad.count()
	for i := 0; i < 3; i++ {
		p.SendDanmaku(ctx, "1", "消息")
	}
	if bad.count() != before {
		t.Errorf("已失效账号不应再被调用，调用数从 %d 变为 %d", before, bad.count())
	}
}

func TestPoolKeepsAccountOnRetryableError(t *testing.T) {
	// 10030 发送过快是可重试错误，不该移出轮换
	a := &stubActions{err: &api.APIError{Code: 10030, Message: "发送过快"}}
	p := New([]Account{{Name: "A", Actions: a}}, nil)

	if err := p.SendDanmaku(context.Background(), "1", "消息"); err == nil {
		t.Error("发送应当失败")
	}
	if p.Healthy() != 1 {
		t.Errorf("可重试错误不应移出账号，健康账号 = %d", p.Healthy())
	}
}

func TestPoolAllAccountsFailed(t *testing.T) {
	a := &stubActions{err: &api.APIError{Code: -101}}
	b := &stubActions{err: &api.APIError{Code: -111}}
	p := New([]Account{{Name: "A", Actions: a}, {Name: "B", Actions: b}}, nil)

	err := p.SendDanmaku(context.Background(), "1", "消息")
	if err == nil {
		t.Fatal("全部账号失效时应返回错误，不得静默丢弃")
	}
	if p.Healthy() != 0 {
		t.Errorf("健康账号数 = %d, 期望 0", p.Healthy())
	}

	// 再次发送应立刻返回 ErrNoHealthyAccount
	err = p.SendDanmaku(context.Background(), "1", "消息")
	if !errors.Is(err, ErrNoHealthyAccount) {
		t.Errorf("err = %v, 期望 ErrNoHealthyAccount", err)
	}
}

func TestPoolEmptyIsError(t *testing.T) {
	p := New(nil, nil)
	if err := p.SendDanmaku(context.Background(), "1", "消息"); !errors.Is(err, ErrNoHealthyAccount) {
		t.Errorf("空账号池应返回 ErrNoHealthyAccount，实际 %v", err)
	}
}

func TestPoolBlock(t *testing.T) {
	a := &stubActions{}
	p := New([]Account{{Name: "A", Actions: a}}, nil)

	if err := p.Block(context.Background(), "1", "999", 12); err != nil {
		t.Fatalf("Block 失败: %v", err)
	}
	if a.count() != 1 {
		t.Errorf("调用数 = %d", a.count())
	}
}

func TestPoolConcurrentSend(t *testing.T) {
	a := &stubActions{}
	b := &stubActions{}
	p := New([]Account{{Name: "A", Actions: a}, {Name: "B", Actions: b}}, nil)

	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			p.SendDanmaku(context.Background(), "1", "并发消息")
		}()
	}
	wg.Wait()

	if total := a.count() + b.count(); total != 50 {
		t.Errorf("总调用数 = %d, 期望 50", total)
	}
}
