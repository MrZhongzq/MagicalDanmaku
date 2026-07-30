package main

import (
	"fmt"
	"strings"

	"github.com/MrZhongzq/MagicalDanmaku/server/internal/event"
)

// Render 把一个事件渲染成一行可读文本。
func Render(ev event.Event) string {
	ts := ev.Timestamp.Format("15:04:05")
	typ := strings.ToUpper(string(ev.Type))
	return fmt.Sprintf("[%s] %-16s %s", ts, typ, describe(ev))
}

// describe 生成事件的正文描述。
func describe(ev event.Event) string {
	switch p := ev.Payload.(type) {
	case event.Danmaku:
		return fmt.Sprintf("%s: %s", userTag(p.User), p.Text)
	case event.SuperChat:
		return fmt.Sprintf("%s 醒目留言 ¥%d: %s", userTag(p.User), p.Price, p.Text)
	case event.SuperChatDelete:
		return fmt.Sprintf("删除了 %d 条醒目留言", len(p.IDs))
	case event.Gift:
		return fmt.Sprintf("%s 送出 %s x%d (%s)", userTag(p.User), p.GiftName, p.Count, coinLabel(p.CoinType, p.TotalCoin))
	case event.GiftCombo:
		return fmt.Sprintf("%s 连击 %s x%d", userTag(p.User), p.GiftName, p.Count)
	case event.GuardBuy:
		verb := "购买"
		if p.IsRenew {
			verb = "续费"
		}
		return fmt.Sprintf("%s %s %s x%d", userTag(p.User), verb, p.GuardName, p.Count)
	case event.UserEnter:
		return fmt.Sprintf("%s 进入直播间", userTag(p.User))
	case event.UserFollow:
		return fmt.Sprintf("%s 关注了主播", userTag(p.User))
	case event.UserShare:
		return fmt.Sprintf("%s 分享了直播间", userTag(p.User))
	case event.UserLike:
		return fmt.Sprintf("%s 点赞了", userTag(p.User))
	case event.LiveStart:
		return "主播开播了"
	case event.LiveStop:
		return "主播下播了"
	case event.RoomChange:
		return fmt.Sprintf("房间信息变更 标题=%q 分区=%s/%s", p.Title, p.ParentAreaName, p.AreaName)
	case event.UserBlocked:
		return fmt.Sprintf("%s 被禁言", userTag(p.User))
	case event.OnlineRankUpdate:
		if p.Count >= 0 {
			return fmt.Sprintf("高能榜人数 %d", p.Count)
		}
		return fmt.Sprintf("高能榜前 %d 名更新", len(p.Top))
	case event.RoomStatsUpdate:
		return statsText(p)
	case event.Battle:
		return fmt.Sprintf("大乱斗事件 %s", p.SubCommand)
	case event.Unknown:
		return fmt.Sprintf("cmd=%s (raw 已保留 %d 字节)", p.Command, len(ev.Raw))
	default:
		return fmt.Sprintf("未处理的载荷类型 %T", ev.Payload)
	}
}

// userTag 生成 "昵称(UID) UL等级 舰长" 形式的用户标签。
func userTag(u event.User) string {
	var b strings.Builder
	if u.Username != "" {
		b.WriteString(u.Username)
	} else {
		b.WriteString("(匿名)")
	}
	if u.UID != "" {
		fmt.Fprintf(&b, "(%s)", u.UID)
	}
	if u.UserLevel > 0 {
		fmt.Fprintf(&b, " UL%d", u.UserLevel)
	}
	switch u.GuardLevel {
	case event.GuardGovernor:
		b.WriteString(" 总督")
	case event.GuardAdmiral:
		b.WriteString(" 提督")
	case event.GuardCaptain:
		b.WriteString(" 舰长")
	}
	if u.Medal != nil {
		fmt.Fprintf(&b, " [%s%d]", u.Medal.Name, u.Medal.Level)
	}
	return b.String()
}

// coinLabel 描述礼物价值。
func coinLabel(coinType string, total int64) string {
	if coinType == "gold" {
		return fmt.Sprintf("¥%.1f", float64(total)/1000)
	}
	return "免费"
}

// statsText 描述房间统计变化。
func statsText(s event.RoomStatsUpdate) string {
	var parts []string
	if s.Fans != nil {
		parts = append(parts, fmt.Sprintf("粉丝 %d", *s.Fans))
	}
	if s.FansClub != nil {
		parts = append(parts, fmt.Sprintf("粉丝团 %d", *s.FansClub))
	}
	if s.Watched != nil {
		parts = append(parts, fmt.Sprintf("看过 %d", *s.Watched))
	}
	if s.LikeCount != nil {
		parts = append(parts, fmt.Sprintf("点赞 %d", *s.LikeCount))
	}
	if len(parts) == 0 {
		return "房间数据更新"
	}
	return strings.Join(parts, " ")
}
