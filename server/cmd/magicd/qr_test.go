package main

import (
	"strings"
	"testing"
)

func TestRenderQRProducesSquareBlock(t *testing.T) {
	art, err := renderQR("https://passport.bilibili.com/h5-app/passport/login/scan?qrcode_key=abc123")
	if err != nil {
		t.Fatalf("renderQR 失败: %v", err)
	}
	lines := strings.Split(strings.TrimRight(art, "\n"), "\n")
	if len(lines) < 10 {
		t.Fatalf("行数 = %d，二维码过小", len(lines))
	}

	// 每行宽度必须一致，否则终端里会显示错乱
	width := len([]rune(lines[0]))
	for i, l := range lines {
		if got := len([]rune(l)); got != width {
			t.Fatalf("第 %d 行宽度 = %d, 期望 %d", i, got, width)
		}
	}

	// 半块渲染下，字符行数应约为模块行数的一半
	if width < len(lines) {
		t.Errorf("宽度 %d 应大于等于行数 %d（半块压缩后每行更宽）", width, len(lines))
	}
}

func TestRenderQROnlyUsesExpectedRunes(t *testing.T) {
	art, err := renderQR("test")
	if err != nil {
		t.Fatalf("renderQR 失败: %v", err)
	}
	allowed := map[rune]bool{blockBoth: true, blockUpper: true, blockLower: true, blockNone: true, '\n': true}
	for _, r := range art {
		if !allowed[r] {
			t.Fatalf("出现非预期字符 %q", r)
		}
	}
}

func TestRenderQRHasQuietZone(t *testing.T) {
	art, err := renderQR("test")
	if err != nil {
		t.Fatalf("renderQR 失败: %v", err)
	}
	lines := strings.Split(strings.TrimRight(art, "\n"), "\n")

	// 首行应完全是空白（静区），否则扫码器可能识别失败
	if strings.TrimSpace(lines[0]) != "" {
		t.Errorf("首行应为静区空白，实际 %q", lines[0])
	}
	// 每行首尾也应是静区空白
	for i, l := range lines {
		runes := []rune(l)
		if runes[0] != blockNone || runes[len(runes)-1] != blockNone {
			t.Errorf("第 %d 行左右静区缺失: %q", i, l)
			break
		}
	}
}

func TestRenderQRRejectsOversizedContent(t *testing.T) {
	// 超出二维码容量上限时应报错而非 panic
	huge := strings.Repeat("x", 10000)
	if _, err := renderQR(huge); err == nil {
		t.Error("超长内容应当报错")
	}
}
