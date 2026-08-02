package config

import (
	"os"
	"testing"
)

// 仓库里的示例配置必须能被解析器接受。
//
// 这条测试不是形式主义：加盲盒规则时分组键写成了 blindbox（实际是
// blindBox），示例配置就成了一份复制粘贴之后跑不起来的东西——而用户
// 拿到项目第一件事往往就是照着它改。
//
// 现在 Parse 开了 KnownFields(true)，字段拼错也会在这里被抓到。
func TestExampleConfigParses(t *testing.T) {
	b, err := os.ReadFile("../../../../config.example.yaml")
	if err != nil {
		t.Fatalf("读示例配置报错: %v", err)
	}
	if _, err := Parse(b); err != nil {
		t.Fatalf("示例配置解析失败: %v", err)
	}
}
