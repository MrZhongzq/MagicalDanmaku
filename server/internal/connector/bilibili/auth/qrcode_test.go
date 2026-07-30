package auth

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestQRGenerate(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"code":0,"data":{
			"url":"https://passport.bilibili.com/h5-app/passport/login/scan?navhide=1&qrcode_key=abc123",
			"qrcode_key":"abc123"
		}}`))
	}))
	defer srv.Close()

	l := NewQRLogin(srv.Client())
	l.SetGenerateURL(srv.URL)

	qr, err := l.Generate(context.Background())
	if err != nil {
		t.Fatalf("Generate 失败: %v", err)
	}
	if qr.Key != "abc123" {
		t.Errorf("Key = %q", qr.Key)
	}
	if !strings.Contains(qr.URL, "qrcode_key=abc123") {
		t.Errorf("URL = %q", qr.URL)
	}
}

func TestQRPollStatuses(t *testing.T) {
	cases := []struct {
		body string
		want PollStatus
	}{
		{`{"code":0,"data":{"code":86101,"message":"未扫码"}}`, PollWaiting},
		{`{"code":0,"data":{"code":86090,"message":"已扫码未确认"}}`, PollScanned},
		{`{"code":0,"data":{"code":86038,"message":"二维码已失效"}}`, PollExpired},
	}
	for _, tc := range cases {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Write([]byte(tc.body))
		}))
		l := NewQRLogin(srv.Client())
		l.SetPollURL(srv.URL)

		got, err := l.Poll(context.Background(), "k")
		srv.Close()
		if err != nil {
			t.Fatalf("Poll 失败: %v", err)
		}
		if got.Status != tc.want {
			t.Errorf("body=%s Status = %s, 期望 %s", tc.body, got.Status, tc.want)
		}
	}
}

func TestQRPollSuccessCollectsCookie(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := w.Header()
		h.Add("Set-Cookie", "SESSDATA=sess-value; Path=/; Domain=.bilibili.com; HttpOnly")
		h.Add("Set-Cookie", "bili_jct=jct-value; Path=/; Domain=.bilibili.com")
		h.Add("Set-Cookie", "DedeUserID=1234; Path=/; Domain=.bilibili.com")
		w.Write([]byte(`{"code":0,"data":{"code":0,"message":"","refresh_token":"rt"}}`))
	}))
	defer srv.Close()

	l := NewQRLogin(srv.Client())
	l.SetPollURL(srv.URL)

	got, err := l.Poll(context.Background(), "k")
	if err != nil {
		t.Fatalf("Poll 失败: %v", err)
	}
	if got.Status != PollSuccess {
		t.Fatalf("Status = %s, 期望 %s", got.Status, PollSuccess)
	}

	sess, err := ParseSession(got.Cookie)
	if err != nil {
		t.Fatalf("登录结果无法解析为会话: %v", err)
	}
	if sess.SESSDATA != "sess-value" || sess.CSRF != "jct-value" || sess.UID != "1234" {
		t.Errorf("会话字段错误: %+v", sess)
	}
}
