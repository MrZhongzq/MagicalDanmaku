package event

import "testing"

// TestAllTypesHasNoDuplicatesAndMatchesDocumentedCount 钉住 allTypes 与
// Type 上方注释里写的数字（21）一致，且没有重复项——这两条都是曾经真的
// 漂移过的东西（注释曾经写着「18 种」，实际已经是 21 种）。不能用反射
// 直接数 const 块里有几个 Type 常量（Go 没有枚举反射），allTypes 本身
// 就是唯一的权威来源，这条测试只能钉住它的形状自洽，钉不住「跟 const
// 块完全一致」——后者只能靠人工 review 或者别处的交叉测试（比如
// httpapi.TestMetaEventTypesNonEmpty 用真实 HTTP 响应比对）兜底。
func TestAllTypesHasNoDuplicatesAndMatchesDocumentedCount(t *testing.T) {
	got := AllTypes()
	if len(got) != 21 {
		t.Errorf("AllTypes() 返回 %d 个，期望 21 个——如果这个数字变了，"+
			"记得同步更新 Type 类型上方注释里写的数字", len(got))
	}
	seen := make(map[Type]bool, len(got))
	for _, ty := range got {
		if seen[ty] {
			t.Errorf("AllTypes() 里重复出现 %q", ty)
		}
		seen[ty] = true
	}
}
