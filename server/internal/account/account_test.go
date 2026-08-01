package account

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/MrZhongzq/MagicalDanmaku/server/internal/connector"
	"github.com/MrZhongzq/MagicalDanmaku/server/internal/connector/bilibili/api"
	"github.com/MrZhongzq/MagicalDanmaku/server/internal/connector/bilibili/auth"
)

// stubActions 是 connector.Actions 的测试替身。
type stubActions struct {
	mu       sync.Mutex
	sent     []string
	blocks   []string
	unblocks []string
	rooms    []string
	err      error
}

func (s *stubActions) SendDanmaku(ctx context.Context, req connector.SendDanmakuRequest) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.err != nil {
		return s.err
	}
	s.sent = append(s.sent, req.Text)
	s.rooms = append(s.rooms, req.RoomID)
	return nil
}

func (s *stubActions) BlockUser(ctx context.Context, req connector.BlockRequest) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.err != nil {
		return s.err
	}
	s.blocks = append(s.blocks, req.UID)
	s.rooms = append(s.rooms, req.RoomID)
	return nil
}

func (s *stubActions) UnblockUser(ctx context.Context, roomID, uid string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.err != nil {
		return s.err
	}
	s.unblocks = append(s.unblocks, uid)
	s.rooms = append(s.rooms, roomID)
	return nil
}

func testSession(t *testing.T) *auth.Session {
	t.Helper()
	sess, err := auth.ParseSession("SESSDATA=x; bili_jct=tok; DedeUserID=42")
	if err != nil {
		t.Fatalf("ParseSession 失败: %v", err)
	}
	return sess
}

func TestAccountSharesLimiterAcrossRooms(t *testing.T) {
	// 风控按账号算：同一账号的不同房间必须共用限流器
	acc := New("主播号", testSession(t), 60*time.Millisecond)

	a := &Binding{Account: acc, RoomID: "甲", Actions: &stubActions{}}
	b := &Binding{Account: acc, RoomID: "乙", Actions: &stubActions{}}

	ctx := context.Background()
	if err := a.SendDanmaku(ctx, "第一条"); err != nil {
		t.Fatalf("发送失败: %v", err)
	}
	start := time.Now()
	if err := b.SendDanmaku(ctx, "第二条"); err != nil {
		t.Fatalf("发送失败: %v", err)
	}
	if d := time.Since(start); d < 40*time.Millisecond {
		t.Errorf("同账号的另一个房间应受同一限流器约束，实际间隔 %v", d)
	}
}

func TestDifferentAccountsDoNotShareLimiter(t *testing.T) {
	a := New("账号A", testSession(t), 5*time.Second)
	b := New("账号B", testSession(t), 5*time.Second)

	ba := &Binding{Account: a, RoomID: "甲", Actions: &stubActions{}}
	bb := &Binding{Account: b, RoomID: "甲", Actions: &stubActions{}}

	ctx := context.Background()
	if err := ba.SendDanmaku(ctx, "A 发的"); err != nil {
		t.Fatalf("发送失败: %v", err)
	}
	start := time.Now()
	if err := bb.SendDanmaku(ctx, "B 发的"); err != nil {
		t.Fatalf("发送失败: %v", err)
	}
	if d := time.Since(start); d > 500*time.Millisecond {
		t.Errorf("不同账号不应互相拖慢，实际等待 %v", d)
	}
}

func TestBindingPassesRoomID(t *testing.T) {
	st := &stubActions{}
	b := &Binding{
		Account: New("账号", testSession(t), 0),
		RoomID:  "1706666491",
		Actions: st,
	}

	ctx := context.Background()
	if err := b.SendDanmaku(ctx, "你好"); err != nil {
		t.Fatalf("SendDanmaku 失败: %v", err)
	}
	if err := b.Block(ctx, "999", 12); err != nil {
		t.Fatalf("Block 失败: %v", err)
	}

	if len(st.rooms) != 2 {
		t.Fatalf("调用数 = %d", len(st.rooms))
	}
	for i, r := range st.rooms {
		if r != "1706666491" {
			t.Errorf("第 %d 次调用的 roomID = %q", i+1, r)
		}
	}
	if len(st.sent) != 1 || st.sent[0] != "你好" {
		t.Errorf("sent = %v", st.sent)
	}
	if len(st.blocks) != 1 || st.blocks[0] != "999" {
		t.Errorf("blocks = %v", st.blocks)
	}
}

func TestBindingReportsErrorWithoutFallback(t *testing.T) {
	// 账号失效就报错，不切换到其他账号
	st := &stubActions{err: &api.APIError{Code: -101, Message: "账号未登录"}}
	b := &Binding{
		Account: New("失效账号", testSession(t), 0),
		RoomID:  "甲",
		Actions: st,
	}

	err := b.SendDanmaku(context.Background(), "消息")
	if err == nil {
		t.Fatal("账号失效应当报错")
	}
	// 错误信息要能定位到是哪个账号在哪个房间出的问题
	if !strings.Contains(err.Error(), "失效账号") {
		t.Errorf("错误信息应含账号名，实际 %v", err)
	}
	if !strings.Contains(err.Error(), "甲") {
		t.Errorf("错误信息应含房间号，实际 %v", err)
	}
}

func TestBindingLabel(t *testing.T) {
	b := &Binding{
		Account: New("主播号", testSession(t), 0),
		RoomID:  "1706666491",
	}
	if got := b.Label(); got != "主播号@1706666491" {
		t.Errorf("Label = %q", got)
	}
}

func TestBindingWithoutAccountFails(t *testing.T) {
	b := &Binding{RoomID: "甲", Actions: &stubActions{}}
	if err := b.SendDanmaku(context.Background(), "x"); !errors.Is(err, ErrNoAccount) {
		t.Errorf("err = %v, 期望 ErrNoAccount", err)
	}
}

// TestBindingUnblock 照着 Block 的用例写：断言 UnblockUser 被调用（含房间号、
// UID），且共享的限流器确实被等待过——第二次调用不应立刻返回。
func TestBindingUnblock(t *testing.T) {
	acc := New("账号", testSession(t), 60*time.Millisecond)
	st := &stubActions{}
	b := &Binding{Account: acc, RoomID: "1706666491", Actions: st}

	ctx := context.Background()
	if err := b.Unblock(ctx, "999"); err != nil {
		t.Fatalf("Unblock 失败: %v", err)
	}
	start := time.Now()
	if err := b.Unblock(ctx, "999"); err != nil {
		t.Fatalf("Unblock 失败: %v", err)
	}
	if d := time.Since(start); d < 40*time.Millisecond {
		t.Errorf("第二次 Unblock 应等待限流器，实际间隔 %v", d)
	}

	if len(st.unblocks) != 2 || st.unblocks[0] != "999" || st.unblocks[1] != "999" {
		t.Errorf("unblocks = %v", st.unblocks)
	}
	if len(st.rooms) != 2 || st.rooms[0] != "1706666491" || st.rooms[1] != "1706666491" {
		t.Errorf("rooms = %v", st.rooms)
	}
}

func TestBindingUnblockWithoutAccountFails(t *testing.T) {
	b := &Binding{RoomID: "甲", Actions: &stubActions{}}
	if err := b.Unblock(context.Background(), "999"); !errors.Is(err, ErrNoAccount) {
		t.Errorf("err = %v, 期望 ErrNoAccount", err)
	}
}

func TestBindingUnblockReportsErrorWithLabel(t *testing.T) {
	// 错误信息要能定位到是哪个账号在哪个房间出的问题，与 Block/SendDanmaku 一致
	st := &stubActions{err: &api.APIError{Code: -101, Message: "账号未登录"}}
	b := &Binding{
		Account: New("失效账号", testSession(t), 0),
		RoomID:  "甲",
		Actions: st,
	}

	err := b.Unblock(context.Background(), "999")
	if err == nil {
		t.Fatal("账号失效应当报错")
	}
	if !strings.Contains(err.Error(), "失效账号") {
		t.Errorf("错误信息应含账号名，实际 %v", err)
	}
	if !strings.Contains(err.Error(), "甲") {
		t.Errorf("错误信息应含房间号，实际 %v", err)
	}
}

func TestBindingRespectsContext(t *testing.T) {
	acc := New("账号", testSession(t), 5*time.Second)
	b := &Binding{Account: acc, RoomID: "甲", Actions: &stubActions{}}

	ctx := context.Background()
	if err := b.SendDanmaku(ctx, "第一条"); err != nil {
		t.Fatalf("发送失败: %v", err)
	}

	ctx2, cancel := context.WithTimeout(ctx, 50*time.Millisecond)
	defer cancel()
	if err := b.SendDanmaku(ctx2, "第二条"); err == nil {
		t.Error("ctx 超时后应返回错误")
	}
}

func TestBindingConcurrentSend(t *testing.T) {
	st := &stubActions{}
	b := &Binding{Account: New("账号", testSession(t), 0), RoomID: "甲", Actions: st}

	var wg sync.WaitGroup
	for i := 0; i < 30; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			b.SendDanmaku(context.Background(), "并发")
		}()
	}
	wg.Wait()

	st.mu.Lock()
	defer st.mu.Unlock()
	if len(st.sent) != 30 {
		t.Errorf("发送数 = %d, 期望 30", len(st.sent))
	}
}
