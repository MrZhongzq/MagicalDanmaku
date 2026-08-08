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
	"github.com/MrZhongzq/MagicalDanmaku/server/internal/ghost"
)

// stubActions 是 connector.Actions 的测试替身。
type stubActions struct {
	mu          sync.Mutex
	sent        []string
	blocks      []string
	unblocks    []string
	rooms       []string
	blacklists  []string
	unblacklist []string
	attribute   int
	nickname    string
	err         error
	// segments 非空时作为 SendDanmaku 的返回值，模拟超长文本被切分成
	// 多段发出；为空则按不切分处理。
	segments []string
}

func (s *stubActions) SendDanmaku(ctx context.Context, req connector.SendDanmakuRequest) ([]string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.err != nil {
		return nil, s.err
	}
	s.sent = append(s.sent, req.Text)
	s.rooms = append(s.rooms, req.RoomID)
	if len(s.segments) > 0 {
		return s.segments, nil
	}
	return []string{req.Text}, nil
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

func (s *stubActions) BlacklistUser(ctx context.Context, uid string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.err != nil {
		return s.err
	}
	s.blacklists = append(s.blacklists, uid)
	return nil
}

func (s *stubActions) UnblacklistUser(ctx context.Context, uid string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.err != nil {
		return s.err
	}
	s.unblacklist = append(s.unblacklist, uid)
	return nil
}

func (s *stubActions) RelationAttribute(ctx context.Context, uid string) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.err != nil {
		return 0, s.err
	}
	return s.attribute, nil
}

func (s *stubActions) Nickname(ctx context.Context, uid string) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.err != nil {
		return "", s.err
	}
	return s.nickname, nil
}

func testSession(t *testing.T) *auth.Session {
	t.Helper()
	sess, err := auth.ParseSession("SESSDATA=x; bili_jct=tok; DedeUserID=42")
	if err != nil {
		t.Fatalf("ParseSession 失败: %v", err)
	}
	return sess
}

func TestSendDanmakuRegistersEverySegmentForGhostDetection(t *testing.T) {
	d := ghost.New(10 * time.Second)
	var got []ghost.Report
	d.OnGhost = func(r ghost.Report) { got = append(got, r) }

	b := &Binding{
		Account: New("小号", testSession(t), 0),
		RoomID:  "1781257934",
		Actions: &stubActions{segments: []string{"前半段", "后半段"}},
		Ghost:   d,
	}

	if err := b.SendDanmaku(context.Background(), "前半段后半段"); err != nil {
		t.Fatalf("发送失败: %v", err)
	}

	// 一条回显都没来，两段都该被判成幽灵——报满两条就证明两段各自登记了。
	d.Sweep(time.Now().Add(11*time.Second), func(string) bool { return true })

	if len(got) != 2 {
		t.Fatalf("两段都该登记等回显，实际报了 %d 条: %+v", len(got), got)
	}
	if got[0].Text != "前半段" || got[1].Text != "后半段" {
		t.Errorf("登记的文本 = %q/%q，期望 前半段/后半段", got[0].Text, got[1].Text)
	}
	if got[0].RoomID != "1781257934" || got[0].UID != "42" {
		t.Errorf("登记的房间/账号 = %q/%q，期望 1781257934/42", got[0].RoomID, got[0].UID)
	}
}

func TestSendDanmakuProbeReportsEchoOutcome(t *testing.T) {
	d := ghost.New(10 * time.Second)
	b := &Binding{
		Account: New("小号", testSession(t), 0),
		RoomID:  "1781257934",
		Actions: &stubActions{},
		Ghost:   d,
	}

	ch, err := b.SendDanmakuProbe(context.Background(), "探针")
	if err != nil {
		t.Fatalf("探针发送失败: %v", err)
	}
	d.Observe("1781257934", "42", "探针")

	select {
	case got := <-ch:
		if got.Result != ghost.ResultOK {
			t.Errorf("Result = %q，期望 %q", got.Result, ghost.ResultOK)
		}
	default:
		t.Fatal("回显已到达，探针应当立刻拿到结果")
	}
}

func TestSendDanmakuProbeNeedsDetector(t *testing.T) {
	b := &Binding{
		Account: New("小号", testSession(t), 0),
		RoomID:  "1",
		Actions: &stubActions{},
	}
	if _, err := b.SendDanmakuProbe(context.Background(), "探针"); err == nil {
		t.Error("没有检测器时探针无从判定，应当报错而不是假装成功")
	}
}

func TestSendDanmakuProbeSurfacesSendFailure(t *testing.T) {
	d := ghost.New(10 * time.Second)
	b := &Binding{
		Account: New("小号", testSession(t), 0),
		RoomID:  "1",
		Actions: &stubActions{err: &api.APIError{Code: 1003, Message: "你已经在本房间被禁言"}},
		Ghost:   d,
	}

	_, err := b.SendDanmakuProbe(context.Background(), "探针")
	if err == nil {
		t.Fatal("发送这步就失败时应当报错——那不是幽灵，是明确拒绝")
	}
	if !strings.Contains(err.Error(), "禁言") {
		t.Errorf("错误应当带上 B 站给的原因，实际 %v", err)
	}
}

func TestSendDanmakuWorksWithoutGhostDetector(t *testing.T) {
	// 绝大多数调用方（含既有测试）不配检测器，这条路必须照常能发。
	b := &Binding{
		Account: New("小号", testSession(t), 0),
		RoomID:  "1",
		Actions: &stubActions{},
	}
	if err := b.SendDanmaku(context.Background(), "喵"); err != nil {
		t.Fatalf("未配检测器时发送失败: %v", err)
	}
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

// TestBindingBlacklist 照着 Unblock 的用例写：断言 BlacklistUser 被调用
// （账号级，不带房间号），且共享的限流器确实被等待过。
func TestBindingBlacklist(t *testing.T) {
	acc := New("账号", testSession(t), 60*time.Millisecond)
	st := &stubActions{}
	b := &Binding{Account: acc, RoomID: "1706666491", Actions: st}

	ctx := context.Background()
	if err := b.Blacklist(ctx, "999"); err != nil {
		t.Fatalf("Blacklist 失败: %v", err)
	}
	start := time.Now()
	if err := b.Blacklist(ctx, "999"); err != nil {
		t.Fatalf("Blacklist 失败: %v", err)
	}
	if d := time.Since(start); d < 40*time.Millisecond {
		t.Errorf("第二次 Blacklist 应等待限流器，实际间隔 %v", d)
	}
	if len(st.blacklists) != 2 || st.blacklists[0] != "999" {
		t.Errorf("blacklists = %v", st.blacklists)
	}
}

func TestBindingUnblacklist(t *testing.T) {
	acc := New("账号", testSession(t), 0)
	st := &stubActions{}
	b := &Binding{Account: acc, RoomID: "1706666491", Actions: st}

	if err := b.Unblacklist(context.Background(), "999"); err != nil {
		t.Fatalf("Unblacklist 失败: %v", err)
	}
	if len(st.unblacklist) != 1 || st.unblacklist[0] != "999" {
		t.Errorf("unblacklist = %v", st.unblacklist)
	}
}

func TestBindingRelationAttribute(t *testing.T) {
	acc := New("账号", testSession(t), 0)
	st := &stubActions{attribute: 128}
	b := &Binding{Account: acc, RoomID: "甲", Actions: st}

	attr, err := b.RelationAttribute(context.Background(), "999")
	if err != nil {
		t.Fatalf("RelationAttribute 失败: %v", err)
	}
	if attr != 128 {
		t.Errorf("attribute = %d, 期望 128", attr)
	}
}

func TestBindingNickname(t *testing.T) {
	acc := New("账号", testSession(t), 0)
	st := &stubActions{nickname: "小明"}
	b := &Binding{Account: acc, RoomID: "甲", Actions: st}

	name, err := b.Nickname(context.Background(), "999")
	if err != nil {
		t.Fatalf("Nickname 失败: %v", err)
	}
	if name != "小明" {
		t.Errorf("name = %q", name)
	}
}

func TestBindingBlacklistWithoutAccountFails(t *testing.T) {
	b := &Binding{RoomID: "甲", Actions: &stubActions{}}
	if err := b.Blacklist(context.Background(), "999"); !errors.Is(err, ErrNoAccount) {
		t.Errorf("err = %v, 期望 ErrNoAccount", err)
	}
}

// TestBindingBlacklistReportsErrorWithLabel 与 Block/Unblock 一致：错误信息
// 要能定位到是哪个账号出的问题（拉黑没有房间号，但账号名仍然要在）。
func TestBindingBlacklistReportsErrorWithLabel(t *testing.T) {
	st := &stubActions{err: &api.APIError{Code: -101, Message: "账号未登录"}}
	b := &Binding{
		Account: New("失效账号", testSession(t), 0),
		RoomID:  "甲",
		Actions: st,
	}

	err := b.Blacklist(context.Background(), "999")
	if err == nil {
		t.Fatal("账号失效应当报错")
	}
	if !strings.Contains(err.Error(), "失效账号") {
		t.Errorf("错误信息应含账号名，实际 %v", err)
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
