package api

import (
	"context"
	"fmt"
	"net/url"
)

// 账号关系的两种写操作，来自用户 2026-08-04 的真实抓包（拉黑 → 校验 →
// 取消拉黑 → 校验，完整走了一遍），见
// .superpowers/sdd/2026-08-03-p5-真机反馈整改/task-p5-6-blackapi.md。
// 拉黑与取消拉黑是同一个接口，只有 act 不同。
const (
	relationActBlacklist   = 5
	relationActUnblacklist = 6
)

// relationBlacklistedAttribute 是 acc/relation 接口里「已拉黑」对应的
// attribute 取值。用户在自己号上连续三次请求实测：拉黑前 2（关注中）、
// 拉黑后 128（已拉黑）、取消拉黑后 0（无关系）——128 是唯一确认代表
// 「已拉黑」的值，其余取值不在本次判定范围内。
const relationBlacklistedAttribute = 128

// modifyRelation 是 x/relation/modify 的统一实现，拉黑与取消拉黑共用。
//
// 全部字段照抄 HAR 实测，不做任何精简：re_src/gaia_source/spmid/
// extend_content 是表单字段，statistics 是 URL 查询参数（HAR 里它挂在
// URL 上，不在表单里）。这些字段是否真的必需未经验证——抓包里浏览器
// 全带了，B 站的风控策略不透明，少发一个字段换来的可能是「有时候成功
// 有时候被风控」这种最难排查的故障，比多发几个字段的成本高得多。
func (c *Client) modifyRelation(ctx context.Context, fid string, act int) error {
	if fid == "" {
		return fmt.Errorf("api: 拉黑/取消拉黑缺少目标 UID")
	}

	form := url.Values{}
	form.Set("fid", fid)
	form.Set("act", fmt.Sprintf("%d", act))
	form.Set("re_src", "11")
	form.Set("gaia_source", "web_main")
	form.Set("spmid", "333.1387.0.0")
	form.Set("extend_content", fmt.Sprintf(`{"entity":"user","entity_id":%s}`, fid))

	rawURL := c.URLFor("relationModify") + "?statistics=" +
		url.QueryEscape(`{"appId":100,"platform":5}`)
	return c.PostForm(ctx, rawURL, form, nil)
}

// Blacklist 把 uid 加入当前账号（Session 对应的账号）的黑名单。
//
// 这是账号级操作，与直播间无关——「主播在直播间拉黑一个人和她从评论区
// 拉黑一个人没有区别」（用户原话）。调用方不应再传 RoomID。
func (c *Client) Blacklist(ctx context.Context, uid string) error {
	return c.modifyRelation(ctx, uid, relationActBlacklist)
}

// Unblacklist 把 uid 移出当前账号的黑名单。
func (c *Client) Unblacklist(ctx context.Context, uid string) error {
	return c.modifyRelation(ctx, uid, relationActUnblacklist)
}

// RelationAttribute 查询当前账号与 uid 的关系属性值（"白捡"的回读接口，
// 需要 wbi 签名）。调用方通过 IsBlacklisted 判断是否已拉黑，不要自己
// 拿 attribute 的原始值做比较——128 这个取值本身没有自解释性，含义
// 只在这两个函数配对使用时才成立。
func (c *Client) RelationAttribute(ctx context.Context, uid string) (int, error) {
	if uid == "" {
		return 0, fmt.Errorf("api: 查询关系状态缺少目标 UID")
	}

	var data struct {
		Relation struct {
			Attribute int `json:"attribute"`
		} `json:"relation"`
	}
	params := url.Values{}
	params.Set("mid", uid)
	params.Set("web_location", "333.1387")
	if err := c.GetJSON(ctx, c.URLFor("accRelation"), params, true, &data); err != nil {
		return 0, err
	}
	return data.Relation.Attribute, nil
}

// IsBlacklisted 判断 RelationAttribute 返回值是否代表「已拉黑」。
//
// 128 是用户实测的确定结论（拉黑前 2、拉黑后 128、取消拉黑后 0），
// 不是猜测——判据只认这一个值。
func IsBlacklisted(attribute int) bool { return attribute == relationBlacklistedAttribute }

// Nickname 查询 uid 对应账号的昵称。
//
// 用于「加入禁言名单」「拉黑前确认」时自动回填昵称，不要求调用方手填。
// **这不是 HAR 验证过的接口**——那次抓包只覆盖了拉黑流程，这里用的是
// B 站广泛使用的标准 wbi 签名只读接口。刻意设计成失败不影响调用方的
// 主流程：昵称拿不到就留空，不应该让拉黑/加名单这件事因为昵称查不到
// 而失败。
func (c *Client) Nickname(ctx context.Context, uid string) (string, error) {
	if uid == "" {
		return "", fmt.Errorf("api: 查询昵称缺少目标 UID")
	}

	var data struct {
		Name string `json:"name"`
	}
	params := url.Values{}
	params.Set("mid", uid)
	if err := c.GetJSON(ctx, c.URLFor("accInfo"), params, true, &data); err != nil {
		return "", err
	}
	return data.Name, nil
}
