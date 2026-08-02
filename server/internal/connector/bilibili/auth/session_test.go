package auth

import (
	"errors"
	"strings"
	"sync"
	"testing"
)

const fullCookie = "buvid3=DE04FB9D-9A3C-09E7-3B1E-A0FBF55CE628infoc; " +
	"b_nut=1700000000; SESSDATA=abc%2Cdef%2Cghi; bili_jct=deadbeefcafe; " +
	"DedeUserID=20285041; DedeUserID__ckMd5=1234567890abcdef"

func TestParseSessionFull(t *testing.T) {
	s, err := ParseSession(fullCookie)
	if err != nil {
		t.Fatalf("ParseSession 失败: %v", err)
	}
	if s.SESSDATA != "abc%2Cdef%2Cghi" {
		t.Errorf("SESSDATA = %q", s.SESSDATA)
	}
	if s.CSRF != "deadbeefcafe" {
		t.Errorf("CSRF = %q", s.CSRF)
	}
	if s.UID != "20285041" {
		t.Errorf("UID = %q", s.UID)
	}
	if s.BuVID3() != "DE04FB9D-9A3C-09E7-3B1E-A0FBF55CE628infoc" {
		t.Errorf("BuVID3 = %q", s.BuVID3())
	}
	if s.BNut() != "1700000000" {
		t.Errorf("BNut = %q", s.BNut())
	}
	if s.IsAnonymous() {
		t.Error("完整 Cookie 不应判定为匿名")
	}
}

func TestParseSessionToleratesSpacingAndTrailingSemicolon(t *testing.T) {
	s, err := ParseSession("  SESSDATA=xyz ;bili_jct=tok;  DedeUserID=42;  ")
	if err != nil {
		t.Fatalf("ParseSession 失败: %v", err)
	}
	if s.SESSDATA != "xyz" || s.CSRF != "tok" || s.UID != "42" {
		t.Errorf("解析结果错误: %+v", s)
	}
}

func TestParseSessionRejectsMissingSESSDATA(t *testing.T) {
	_, err := ParseSession("bili_jct=tok; DedeUserID=42")
	if !errors.Is(err, ErrMissingSESSDATA) {
		t.Errorf("err = %v, 期望 ErrMissingSESSDATA", err)
	}
}

func TestParseSessionRejectsMissingCSRF(t *testing.T) {
	_, err := ParseSession("SESSDATA=xyz; DedeUserID=42")
	if !errors.Is(err, ErrMissingCSRF) {
		t.Errorf("err = %v, 期望 ErrMissingCSRF", err)
	}
}

func TestParseSessionRejectsEmpty(t *testing.T) {
	if _, err := ParseSession("   "); err == nil {
		t.Error("空 Cookie 应当报错")
	}
}

func TestEnsureDeviceFieldsAddsMissing(t *testing.T) {
	s, err := ParseSession("SESSDATA=xyz; bili_jct=tok; DedeUserID=42")
	if err != nil {
		t.Fatalf("ParseSession 失败: %v", err)
	}
	if s.BuVID3() != "" {
		t.Fatalf("前置条件错误，BuVID3 应为空")
	}

	s.EnsureDeviceFields("NEW-BUVID-VALUE")

	if s.BuVID3() != "NEW-BUVID-VALUE" {
		t.Errorf("BuVID3 = %q", s.BuVID3())
	}
	if s.BuVID4() != "NEW-BUVID-VALUE" {
		t.Errorf("BuVID4 = %q，缺失时应与 buvid3 同值", s.BuVID4())
	}
	if s.BNut() == "" {
		t.Error("BNut 应被填上当前时间戳")
	}

	h := s.CookieHeader()
	for _, want := range []string{"SESSDATA=xyz", "bili_jct=tok", "DedeUserID=42",
		"buvid3=NEW-BUVID-VALUE", "buvid4=NEW-BUVID-VALUE", "b_nut="} {
		if !strings.Contains(h, want) {
			t.Errorf("CookieHeader 缺少 %q，实际 %q", want, h)
		}
	}
}

func TestEnsureDeviceFieldsKeepsExisting(t *testing.T) {
	s, err := ParseSession(fullCookie)
	if err != nil {
		t.Fatalf("ParseSession 失败: %v", err)
	}
	orig := s.BuVID3()

	s.EnsureDeviceFields("SHOULD-NOT-OVERWRITE")

	if s.BuVID3() != orig {
		t.Errorf("已有 buvid3 不应被覆盖，实际 %q", s.BuVID3())
	}
	if s.BNut() != "1700000000" {
		t.Errorf("已有 b_nut 不应被覆盖，实际 %q", s.BNut())
	}
}

func TestCookieHeaderRoundTrips(t *testing.T) {
	s, err := ParseSession(fullCookie)
	if err != nil {
		t.Fatalf("ParseSession 失败: %v", err)
	}
	s2, err := ParseSession(s.CookieHeader())
	if err != nil {
		t.Fatalf("回环解析失败: %v", err)
	}
	if s2.SESSDATA != s.SESSDATA || s2.CSRF != s.CSRF || s2.UID != s.UID || s2.BuVID3() != s.BuVID3() {
		t.Errorf("回环后字段不一致:\n原始 %+v\n回环 %+v", s, s2)
	}
}

// TestSessionConcurrentAccessDoesNotCrash 复现 PK 场景的真实拓扑：宿主
// Client 和它为每个对手另起的 Client 共享同一个 *Session（同一账号
// 登录信息）。-352 是账号级风控，宿主和全部对手连接会同时命中，于是
// 会有多个 goroutine 同时调 EnsureDeviceFields（写 pairs/order），
// 同时又有别的 goroutine 在每次 HTTP 请求里调 CookieHeader()/BuVID3()
// （读 pairs/order）。
//
// 不加锁的后果不是偶发脏读——Go 运行时对并发读写 map 有独立于
// -race 的运行时检测，会直接 `fatal error: concurrent map read and
// map write` 让整个进程崩溃，不可 recover。这条测试即使不带 -race
// 也应该能稳定复现这个崩溃（如果锁被去掉的话），所以能在跑不了
// -race 的环境里也当一份有效证据。多迭代、多 goroutine 是为了尽量
// 提高触发概率——Go 的 map 并发检测依赖调度时机，单次运行不一定
// 每次都命中。
func TestSessionConcurrentAccessDoesNotCrash(t *testing.T) {
	const iterations = 20
	const writersPerIteration = 20

	for iter := 0; iter < iterations; iter++ {
		s, err := ParseSession("SESSDATA=xyz; bili_jct=tok; DedeUserID=42")
		if err != nil {
			t.Fatalf("ParseSession 失败: %v", err)
		}

		var wg sync.WaitGroup
		wg.Add(writersPerIteration * 3)
		for i := 0; i < writersPerIteration; i++ {
			// 模拟多个 Client（宿主 + N 个对手）同时因为账号级风控
			// 触发 EnsureDeviceFields。
			go func() {
				defer wg.Done()
				s.EnsureDeviceFields("buvid-from-fetch")
			}()
			// 模拟同时有别的 Client 在发 HTTP 请求，读 Cookie 头。
			go func() {
				defer wg.Done()
				_ = s.CookieHeader()
			}()
			// 模拟 client.go authenticate() 并发读 BuVID3。
			go func() {
				defer wg.Done()
				_ = s.BuVID3()
			}()
		}
		wg.Wait()
	}
}
