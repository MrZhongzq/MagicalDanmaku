// Package perm 定义授权用的权限点。
//
// 不设固定角色。角色（「运营」「房管」）只是若干权限点的预设组合，
// 展开发生在界面层，存储层里永远只有权限点本身——这样新增一种职责
// 分工不需要改数据模型。
package perm

import (
	"fmt"
	"strings"
)

// Permission 是一个权限点。
type Permission string

// 全部权限点。授权单位是「账号-直播间」绑定，即持有者能对某个账号在
// 某个直播间做什么。
//
// 曾经有过第七个 account:manage（「修改 Cookie 与账号参数」），已删除。
// 它是绑定级的，而账号设置（Cookie、限流、字数上限）是账号级的——在
// 绑定 A 上授予它，持有者就能改到同账号下绑定 B 的行为，这是越界。
// 账号级操作因此一律走「账号所有者或管理员」，判定在 guard.go 的
// isAccountOwner，不在这个权限点体系里。
const (
	RuleRead     Permission = "rule:read"     // 查看规则
	RuleWrite    Permission = "rule:write"    // 增删改规则、启停规则
	DanmakuSend  Permission = "danmaku:send"  // 手动发送弹幕
	UserBlock    Permission = "user:block"    // 禁言与解禁，含维护禁言名单
	MemberManage Permission = "member:manage" // 授权他人、撤销授权
	EventRead    Permission = "event:read"    // 查看事件流与历史业务日志
)

// all 按声明顺序排列，All 与错误提示都依赖这个顺序。
var all = []Permission{
	RuleRead, RuleWrite, DanmakuSend,
	UserBlock, MemberManage, EventRead,
}

// known 用于 Parse 的查表。
var known = func() map[Permission]bool {
	m := make(map[Permission]bool, len(all))
	for _, p := range all {
		m[p] = true
	}
	return m
}()

// All 返回全部权限点的副本。
func All() []Permission {
	out := make([]Permission, len(all))
	copy(out, all)
	return out
}

// Parse 把字符串解析成权限点。
//
// 未知权限点的错误信息会列出全部合法值：用户拼错时不该被迫去翻文档。
func Parse(s string) (Permission, error) {
	p := Permission(strings.TrimSpace(s))
	if !known[p] {
		return "", fmt.Errorf("perm: 未知的权限点 %q，合法值为 %s",
			s, strings.Join(Strings(all), ", "))
	}
	return p, nil
}

// ParseList 解析逗号分隔的权限点列表，去重并保持首次出现的顺序。
func ParseList(s string) ([]Permission, error) {
	var out []Permission
	seen := make(map[Permission]bool)

	for _, part := range strings.Split(s, ",") {
		if strings.TrimSpace(part) == "" {
			continue
		}
		p, err := Parse(part)
		if err != nil {
			return nil, err
		}
		if seen[p] {
			continue
		}
		seen[p] = true
		out = append(out, p)
	}

	if len(out) == 0 {
		return nil, fmt.Errorf("perm: 权限点列表为空，至少要给一个")
	}
	return out, nil
}

// OwnerBypass 判断账号所有者是否凭所有权就拥有权限点 p。
//
// 除 MemberManage 外全部为真。
//
// 所有者凭所有权拿到这些权限点，理由是它们严格弱于他已经握着的
// 权力：删掉整个账号、删掉绑定（连带全部规则、冷却组、KV 与授权）、
// 替换账号的 Cookie。但那些权力**全是收缩性的**——能清空别人的
// 访问，不能凭空赋予一个新人访问。「能删光所有协作者」推不出
// 「能新增一个协作者」，二者是两个方向而不是强弱。
//
// 所以 MemberManage 不在内：把第三方拉进授权体系是管理员级别的
// 决定，不是账号所有权的附带品。
//
// **这条规则只此一处定义。** store.Can 与 httpapi 的 permissionSet
// 都引它——各写一遍必然漂，而漂掉的后果是「列表说你没权限、请求
// 却成了」，比直接报错更难查。
func OwnerBypass(p Permission) bool { return p != MemberManage }

// Strings 把权限点列表转成字符串切片，用于写库与打印。
func Strings(ps []Permission) []string {
	out := make([]string, len(ps))
	for i, p := range ps {
		out[i] = string(p)
	}
	return out
}
