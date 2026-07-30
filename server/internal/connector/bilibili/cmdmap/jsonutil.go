package cmdmap

import (
	"encoding/json"
	"strconv"
	"time"
)

// 以下助手用于安全地从任意结构的 JSON 中取值。
// B 站的 CMD 结构不稳定且字段类型会变（数字有时是字符串），
// 因此所有取值都必须容错，缺失或类型不符时返回零值而非报错。

// getObject 返回 m 中键 k 对应的对象，不存在或类型不符时返回 nil。
func getObject(m map[string]any, k string) map[string]any {
	v, _ := m[k].(map[string]any)
	return v
}

// getArray 返回 m 中键 k 对应的数组，不存在或类型不符时返回 nil。
func getArray(m map[string]any, k string) []any {
	v, _ := m[k].([]any)
	return v
}

// getString 返回 m 中键 k 对应的字符串。
// 数字会被转成字符串，以应对 B 站同一字段时而数字时而字符串的情况。
func getString(m map[string]any, k string) string {
	switch v := m[k].(type) {
	case string:
		return v
	case float64:
		return strconv.FormatInt(int64(v), 10)
	case json.Number:
		return v.String()
	case bool:
		return strconv.FormatBool(v)
	default:
		return ""
	}
}

// getInt64 返回 m 中键 k 对应的整数，字符串会被尝试解析。
func getInt64(m map[string]any, k string) int64 {
	return toInt64(m[k])
}

// toInt64 把任意 JSON 标量转成 int64。
func toInt64(v any) int64 {
	switch t := v.(type) {
	case float64:
		return int64(t)
	case string:
		n, _ := strconv.ParseInt(t, 10, 64)
		return n
	case json.Number:
		n, _ := t.Int64()
		return n
	case bool:
		if t {
			return 1
		}
		return 0
	default:
		return 0
	}
}

// getBool 返回 m 中键 k 对应的布尔值，数字非零视为 true。
func getBool(m map[string]any, k string) bool {
	switch t := m[k].(type) {
	case bool:
		return t
	case float64:
		return t != 0
	case string:
		return t != "" && t != "0" && t != "false"
	default:
		return false
	}
}

// atInt64 返回数组 a 中下标 i 的整数，越界返回 0。
func atInt64(a []any, i int) int64 {
	if i < 0 || i >= len(a) {
		return 0
	}
	return toInt64(a[i])
}

// atString 返回数组 a 中下标 i 的字符串，越界返回空串。
func atString(a []any, i int) string {
	if i < 0 || i >= len(a) {
		return ""
	}
	switch v := a[i].(type) {
	case string:
		return v
	case float64:
		return strconv.FormatInt(int64(v), 10)
	default:
		return ""
	}
}

// atArray 返回数组 a 中下标 i 的子数组，越界或类型不符返回 nil。
func atArray(a []any, i int) []any {
	if i < 0 || i >= len(a) {
		return nil
	}
	v, _ := a[i].([]any)
	return v
}

// atObject 返回数组 a 中下标 i 的对象，越界或类型不符返回 nil。
func atObject(a []any, i int) map[string]any {
	if i < 0 || i >= len(a) {
		return nil
	}
	v, _ := a[i].(map[string]any)
	return v
}

// timeFromUnixSec 把 10 位秒级时间戳转成 time.Time，0 返回零值。
func timeFromUnixSec(sec int64) time.Time {
	if sec <= 0 {
		return time.Time{}
	}
	return time.Unix(sec, 0)
}

// timeFromUnixMilli 把 13 位毫秒级时间戳转成 time.Time，0 返回零值。
// 传入的若是 10 位秒级时间戳会被自动识别并放大。
func timeFromUnixMilli(ms int64) time.Time {
	if ms <= 0 {
		return time.Time{}
	}
	if ms < 1e11 { // 小于 1973 年的毫秒数，实为秒级时间戳
		return time.Unix(ms, 0)
	}
	return time.UnixMilli(ms)
}
