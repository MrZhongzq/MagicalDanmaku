package httpapi

import (
	"testing"
	"time"
)

func TestQRSessionsPutAndTake(t *testing.T) {
	q := newQRSessions(time.Minute)
	q.put("k1", qrPending{AccountName: "小号", UserID: 7})

	got, ok := q.take("k1")
	if !ok {
		t.Fatal("刚放进去的应该取得到")
	}
	if got.AccountName != "小号" || got.UserID != 7 {
		t.Errorf("取到 = %+v", got)
	}
}

// take 不删除：轮询会被调用很多次，每次都要拿得到
func TestQRSessionsTakeIsRepeatable(t *testing.T) {
	q := newQRSessions(time.Minute)
	q.put("k1", qrPending{AccountName: "小号", UserID: 7})

	for i := 0; i < 3; i++ {
		if _, ok := q.take("k1"); !ok {
			t.Fatalf("第 %d 次取失败，轮询要能反复取", i+1)
		}
	}
}

func TestQRSessionsDelete(t *testing.T) {
	q := newQRSessions(time.Minute)
	q.put("k1", qrPending{AccountName: "小号"})
	q.delete("k1")
	if _, ok := q.take("k1"); ok {
		t.Error("删除后不该取得到")
	}
}

func TestQRSessionsExpires(t *testing.T) {
	q := newQRSessions(-time.Second) // 立即过期
	q.put("k1", qrPending{AccountName: "小号"})
	if _, ok := q.take("k1"); ok {
		t.Error("过期的会话不该取得到")
	}
}

func TestQRSessionsUnknownKey(t *testing.T) {
	q := newQRSessions(time.Minute)
	if _, ok := q.take("没这个键"); ok {
		t.Error("不存在的键不该取得到")
	}
}

func TestQRSessionsPurge(t *testing.T) {
	q := newQRSessions(-time.Second)
	q.put("k1", qrPending{})
	q.put("k2", qrPending{})
	if n := q.purgeExpired(); n != 2 {
		t.Errorf("清理数 = %d, 期望 2", n)
	}
	if n := q.purgeExpired(); n != 0 {
		t.Errorf("二次清理数 = %d, 期望 0", n)
	}
}

func TestQRSessionsConcurrent(t *testing.T) {
	q := newQRSessions(time.Minute)
	done := make(chan struct{})
	for g := 0; g < 8; g++ {
		go func(g int) {
			for i := 0; i < 100; i++ {
				k := string(rune('a'+g)) + itoaInt(i)
				q.put(k, qrPending{AccountName: k})
				q.take(k)
				q.purgeExpired()
			}
			done <- struct{}{}
		}(g)
	}
	for g := 0; g < 8; g++ {
		<-done
	}
}

func itoaInt(v int) string {
	if v == 0 {
		return "0"
	}
	var b []byte
	for v > 0 {
		b = append([]byte{byte('0' + v%10)}, b...)
		v /= 10
	}
	return string(b)
}
