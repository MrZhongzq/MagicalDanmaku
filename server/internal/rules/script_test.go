package rules

import (
	"errors"
	"strings"
	"sync"
	"testing"
	"time"
)

// fakeBot 是 BotAPI 的测试替身。
type fakeBot struct {
	mu       sync.Mutex
	danmakus []string
	blocks   []string
}

func (f *fakeBot) SendDanmaku(text string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.danmakus = append(f.danmakus, text)
	return nil
}

func (f *fakeBot) Block(uid string, hours int) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.blocks = append(f.blocks, uid)
	return nil
}

func (f *fakeBot) Blacklist(uid string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.blocks = append(f.blocks, uid)
	return nil
}

// memStore 是 Storage 的内存实现，测试用。
type memStore struct {
	mu sync.Mutex
	m  map[string]string
}

func newMemStore() *memStore { return &memStore{m: map[string]string{}} }

func (s *memStore) Get(k string) (string, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	v, ok := s.m[k]
	return v, ok
}

func (s *memStore) Set(k, v string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.m[k] = v
}

func newTestSandbox(bot BotAPI, st Storage) *Sandbox {
	return NewSandbox(SandboxOptions{Timeout: 200 * time.Millisecond, Bot: bot, Storage: st})
}

func TestSandboxEvalBool(t *testing.T) {
	sb := newTestSandbox(nil, nil)
	vars := map[string]any{
		"text": "点歌 晴天",
		"user": map[string]any{"guardLevel": 3, "username": "甲"},
	}

	cases := []struct {
		code string
		want bool
	}{
		{`event.user.guardLevel > 0`, true},
		{`event.user.guardLevel > 5`, false},
		{`event.text.indexOf("点歌") === 0`, true},
		{`event.text.length > 100`, false},
		{`event.user.username === "甲"`, true},
	}
	for _, tc := range cases {
		got, err := sb.EvalBool(tc.code, vars)
		if err != nil {
			t.Errorf("%s: 求值失败 %v", tc.code, err)
			continue
		}
		if got != tc.want {
			t.Errorf("%s = %v, 期望 %v", tc.code, got, tc.want)
		}
	}
}

func TestSandboxEvalBoolNonBooleanIsTruthy(t *testing.T) {
	sb := newTestSandbox(nil, nil)
	// JS 的真值语义：非空字符串为真，0 为假
	got, err := sb.EvalBool(`"非空"`, map[string]any{})
	if err != nil {
		t.Fatalf("求值失败: %v", err)
	}
	if !got {
		t.Error("非空字符串应为真")
	}
	got, _ = sb.EvalBool(`0`, map[string]any{})
	if got {
		t.Error("0 应为假")
	}
}

func TestSandboxTimeoutInterruptsInfiniteLoop(t *testing.T) {
	sb := NewSandbox(SandboxOptions{Timeout: 50 * time.Millisecond})

	start := time.Now()
	_, err := sb.EvalBool(`while(true){}`, map[string]any{})
	elapsed := time.Since(start)

	if !errors.Is(err, ErrScriptTimeout) {
		t.Errorf("err = %v, 期望 ErrScriptTimeout", err)
	}
	if elapsed > 2*time.Second {
		t.Errorf("死循环应在超时后被打断，实际耗时 %v", elapsed)
	}
}

func TestSandboxHasNoFileSystemAccess(t *testing.T) {
	sb := newTestSandbox(nil, nil)
	// 这些全局对象一律不得存在——沙箱安全的核心保证
	for _, name := range []string{"require", "process", "fs", "child_process", "fetch", "XMLHttpRequest", "eval_file"} {
		code := `typeof ` + name + ` === "undefined"`
		got, err := sb.EvalBool(code, map[string]any{})
		if err != nil {
			t.Errorf("%s: 求值失败 %v", name, err)
			continue
		}
		if !got {
			t.Errorf("全局对象 %q 不该存在于沙箱中", name)
		}
	}
}

func TestSandboxBotAPI(t *testing.T) {
	bot := &fakeBot{}
	sb := newTestSandbox(bot, nil)

	err := sb.RunAction(`bot.sendDanmaku("你好"); bot.block("123", 2)`, map[string]any{})
	if err != nil {
		t.Fatalf("RunAction 失败: %v", err)
	}
	if len(bot.danmakus) != 1 || bot.danmakus[0] != "你好" {
		t.Errorf("danmakus = %v", bot.danmakus)
	}
	if len(bot.blocks) != 1 || bot.blocks[0] != "123" {
		t.Errorf("blocks = %v", bot.blocks)
	}
}

func TestSandboxStorage(t *testing.T) {
	st := newMemStore()
	sb := newTestSandbox(nil, st)

	if err := sb.RunAction(`storage.set("计数", "1")`, map[string]any{}); err != nil {
		t.Fatalf("RunAction 失败: %v", err)
	}
	if v, ok := st.Get("计数"); !ok || v != "1" {
		t.Errorf("storage 未写入: %v %v", v, ok)
	}

	got, err := sb.EvalBool(`storage.get("计数") === "1"`, map[string]any{})
	if err != nil {
		t.Fatalf("求值失败: %v", err)
	}
	if !got {
		t.Error("storage.get 未取到写入的值")
	}
}

func TestSandboxStorageMissingKeyReturnsEmpty(t *testing.T) {
	sb := newTestSandbox(nil, newMemStore())
	got, err := sb.EvalBool(`storage.get("从未写过") === ""`, map[string]any{})
	if err != nil {
		t.Fatalf("求值失败: %v", err)
	}
	if !got {
		t.Error("未写过的键应返回空串而非抛异常")
	}
}

func TestSandboxSyntaxErrorReported(t *testing.T) {
	sb := newTestSandbox(nil, nil)
	_, err := sb.EvalBool(`这不是合法的 JS ((( `, map[string]any{})
	if err == nil {
		t.Fatal("语法错误应当报错")
	}
	if !strings.Contains(err.Error(), "脚本") {
		t.Errorf("错误信息应提及脚本，实际 %v", err)
	}
}

func TestSandboxRuntimeErrorReported(t *testing.T) {
	sb := newTestSandbox(nil, nil)
	_, err := sb.EvalBool(`null.foo`, map[string]any{})
	if err == nil {
		t.Error("运行时异常应当报错")
	}
}

func TestSandboxConcurrentUse(t *testing.T) {
	// goja.Runtime 非并发安全，Sandbox 必须自行隔离
	sb := newTestSandbox(nil, nil)
	var wg sync.WaitGroup
	errs := make(chan error, 50)

	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			vars := map[string]any{"n": n}
			ok, err := sb.EvalBool(`event.n >= 0`, vars)
			if err != nil {
				errs <- err
				return
			}
			if !ok {
				errs <- errors.New("结果错误")
			}
		}(i)
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		t.Errorf("并发执行出错: %v", err)
	}
}

func TestSandboxVarsIsolatedBetweenRuns(t *testing.T) {
	sb := newTestSandbox(nil, nil)
	// 前一次执行污染的全局变量不得泄漏到下一次
	if err := sb.RunAction(`globalThis.污染 = "脏数据"`, map[string]any{}); err != nil {
		t.Fatalf("RunAction 失败: %v", err)
	}
	got, err := sb.EvalBool(`typeof 污染 === "undefined"`, map[string]any{})
	if err != nil {
		t.Fatalf("求值失败: %v", err)
	}
	if !got {
		t.Error("上一次执行的全局变量泄漏到了下一次")
	}
}
