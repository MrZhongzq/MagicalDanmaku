package main

import (
	"strings"
	"testing"
)

func TestParseBindingRef(t *testing.T) {
	name, room, err := parseBindingRef("小号@1706666491")
	if err != nil {
		t.Fatalf("解析报错: %v", err)
	}
	if name != "小号" || room != "1706666491" {
		t.Errorf("= %q / %q", name, room)
	}
}

func TestParseBindingRefRejectsMissingAt(t *testing.T) {
	_, _, err := parseBindingRef("小号")
	if err == nil {
		t.Fatal("缺少 @ 应报错")
	}
	// 报错要给出正确写法，否则用户只能猜
	if !strings.Contains(err.Error(), "@") {
		t.Errorf("错误信息应示范格式，实际: %v", err)
	}
}

func TestParseBindingRefRejectsEmptyParts(t *testing.T) {
	for _, s := range []string{"@123", "小号@", "@"} {
		if _, _, err := parseBindingRef(s); err == nil {
			t.Errorf("%q 应报错", s)
		}
	}
}

// 账号名里可能带 @，取最后一个 @ 作为分隔符
func TestParseBindingRefSplitsOnLastAt(t *testing.T) {
	name, room, err := parseBindingRef("a@b@123")
	if err != nil {
		t.Fatalf("解析报错: %v", err)
	}
	if name != "a@b" || room != "123" {
		t.Errorf("= %q / %q, 期望 \"a@b\" / \"123\"", name, room)
	}
}

func TestOpenStoreRequiresDSN(t *testing.T) {
	t.Setenv("MAGICD_DATABASE_URL", "")
	_, err := openStore(t.Context(), "")
	if err == nil {
		t.Fatal("没有连接串应报错")
	}
	if !strings.Contains(err.Error(), "MAGICD_DATABASE_URL") {
		t.Errorf("错误信息应提示怎么配置，实际: %v", err)
	}
}
