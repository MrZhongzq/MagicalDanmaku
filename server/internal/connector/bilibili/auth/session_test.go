package auth

import (
	"errors"
	"strings"
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
	if s.BuVID3 != "DE04FB9D-9A3C-09E7-3B1E-A0FBF55CE628infoc" {
		t.Errorf("BuVID3 = %q", s.BuVID3)
	}
	if s.BNut != "1700000000" {
		t.Errorf("BNut = %q", s.BNut)
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
	if s.BuVID3 != "" {
		t.Fatalf("前置条件错误，BuVID3 应为空")
	}

	s.EnsureDeviceFields("NEW-BUVID-VALUE")

	if s.BuVID3 != "NEW-BUVID-VALUE" {
		t.Errorf("BuVID3 = %q", s.BuVID3)
	}
	if s.BuVID4 != "NEW-BUVID-VALUE" {
		t.Errorf("BuVID4 = %q，缺失时应与 buvid3 同值", s.BuVID4)
	}
	if s.BNut == "" {
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
	orig := s.BuVID3

	s.EnsureDeviceFields("SHOULD-NOT-OVERWRITE")

	if s.BuVID3 != orig {
		t.Errorf("已有 buvid3 不应被覆盖，实际 %q", s.BuVID3)
	}
	if s.BNut != "1700000000" {
		t.Errorf("已有 b_nut 不应被覆盖，实际 %q", s.BNut)
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
	if s2.SESSDATA != s.SESSDATA || s2.CSRF != s.CSRF || s2.UID != s.UID || s2.BuVID3 != s.BuVID3 {
		t.Errorf("回环后字段不一致:\n原始 %+v\n回环 %+v", s, s2)
	}
}
