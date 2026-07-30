package auth

import (
	"crypto/md5"
	"encoding/hex"
	"errors"
	"net/url"
	"strconv"
	"testing"
	"time"
)

func TestMixinKeyEncTabShape(t *testing.T) {
	if len(MixinKeyEncTab) != 64 {
		t.Fatalf("置换表长度 = %d, 期望 64", len(MixinKeyEncTab))
	}
	seen := make(map[int]bool, 64)
	for i, v := range MixinKeyEncTab {
		if v < 0 || v > 63 {
			t.Errorf("下标 %d 处的值 %d 越界", i, v)
		}
		if seen[v] {
			t.Errorf("值 %d 重复出现", v)
		}
		seen[v] = true
	}
	if len(seen) != 64 {
		t.Errorf("置换表应覆盖 0..63，实际只有 %d 个不同值", len(seen))
	}
}

func TestDeriveMixinKey(t *testing.T) {
	// 构造 64 个可区分的字符：前 32 位为 '0'..'9''a'..'v'，后 32 位为大写
	imgName := "0123456789abcdefghijklmnopqrstuv"
	subName := "0123456789ABCDEFGHIJKLMNOPQRSTUV"
	imgURL := "https://i0.hdslb.com/bfs/wbi/" + imgName + ".png"
	subURL := "https://i0.hdslb.com/bfs/wbi/" + subName + ".png"

	got, err := DeriveMixinKey(imgURL, subURL)
	if err != nil {
		t.Fatalf("DeriveMixinKey 失败: %v", err)
	}
	if len(got) != 32 {
		t.Fatalf("mixinKey 长度 = %d, 期望 32", len(got))
	}

	concat := imgName + subName
	want := make([]byte, 0, 32)
	for i := 0; i < 32; i++ {
		want = append(want, concat[MixinKeyEncTab[i]])
	}
	if got != string(want) {
		t.Errorf("mixinKey = %q, 期望 %q", got, string(want))
	}
}

func TestDeriveMixinKeyRejectsBadURL(t *testing.T) {
	if _, err := DeriveMixinKey("https://example.com/notahash.png", "https://example.com/x.png"); err == nil {
		t.Error("非法 URL 应当报错")
	}
}

func TestSignProducesCorrectWRid(t *testing.T) {
	s := NewSigner()
	const key = "abcdefghijklmnopqrstuvwxyz012345"
	s.SetMixinKey(key)

	in := url.Values{}
	in.Set("id", "21452505")
	in.Set("type", "0")

	out, err := s.Sign(in)
	if err != nil {
		t.Fatalf("Sign 失败: %v", err)
	}

	wts := out.Get("wts")
	if wts == "" {
		t.Fatal("缺少 wts")
	}
	if n, err := strconv.ParseInt(wts, 10, 64); err != nil {
		t.Fatalf("wts 不是整数: %v", err)
	} else if d := time.Since(time.Unix(n, 0)); d < 0 || d > time.Minute {
		t.Errorf("wts 偏离当前时间过多: %v", d)
	}

	// 手工复算期望值
	expect := url.Values{}
	expect.Set("id", "21452505")
	expect.Set("type", "0")
	expect.Set("wts", wts)
	sum := md5.Sum([]byte(expect.Encode() + key))
	want := hex.EncodeToString(sum[:])

	if got := out.Get("w_rid"); got != want {
		t.Errorf("w_rid = %q, 期望 %q", got, want)
	}
}

func TestSignFiltersForbiddenChars(t *testing.T) {
	s := NewSigner()
	s.SetMixinKey("abcdefghijklmnopqrstuvwxyz012345")

	in := url.Values{}
	in.Set("name", "a!b'c(d)e*f")

	out, err := s.Sign(in)
	if err != nil {
		t.Fatalf("Sign 失败: %v", err)
	}
	if got := out.Get("name"); got != "abcdef" {
		t.Errorf("name = %q, 期望 abcdef（应剔除 !'()* ）", got)
	}
}

func TestSignDoesNotMutateInput(t *testing.T) {
	s := NewSigner()
	s.SetMixinKey("abcdefghijklmnopqrstuvwxyz012345")

	in := url.Values{}
	in.Set("id", "1")

	if _, err := s.Sign(in); err != nil {
		t.Fatalf("Sign 失败: %v", err)
	}
	if in.Get("wts") != "" || in.Get("w_rid") != "" {
		t.Errorf("Sign 不得修改入参，实际 %v", in)
	}
}

func TestSignWithoutKeyFails(t *testing.T) {
	s := NewSigner()
	if _, err := s.Sign(url.Values{}); !errors.Is(err, ErrNoMixinKey) {
		t.Errorf("err = %v, 期望 ErrNoMixinKey", err)
	}
}

func TestNeedsRefresh(t *testing.T) {
	s := NewSigner()
	if !s.NeedsRefresh() {
		t.Error("未设置 mixinKey 时应需要刷新")
	}
	s.SetMixinKey("abcdefghijklmnopqrstuvwxyz012345")
	if s.NeedsRefresh() {
		t.Error("刚设置后不应需要刷新")
	}
}
