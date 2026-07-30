package cmdmap

import (
	"encoding/json"
	"fmt"

	"github.com/MrZhongzq/MagicalDanmaku/server/internal/event"
)

func init() {
	Register("DANMU_MSG", mapDanmuMsg)
}

// mapDanmuMsg 解析弹幕消息。
//
// info 数组的下标含义：
//
//	info[0][3]      弹幕颜色（十进制整数）
//	info[0][4]      发送时间（13 位毫秒）
//	info[0][12]     弹幕类型，1 为表情弹幕
//	info[0][15]     详情对象，含头像与回复信息
//	info[1]         弹幕正文
//	info[2]         [uid, uname, admin, vip, svip, uidentity, iphone, unameColor]
//	info[3]         勋章数组，未佩戴时为空
//	info[4][0]      UL 等级
//	info[7]         本房间大航海等级
//	info[16][0]     荣耀等级
//
// B 站会不定期在数组尾部追加字段，因此全部取值都必须做越界保护。
func mapDanmuMsg(ctx Context, raw json.RawMessage) ([]event.Event, error) {
	var msg struct {
		Info []any `json:"info"`
	}
	if err := json.Unmarshal(raw, &msg); err != nil {
		return nil, fmt.Errorf("cmdmap: DANMU_MSG 解析失败: %w", err)
	}
	info := msg.Info
	if len(info) < 3 {
		return nil, fmt.Errorf("cmdmap: DANMU_MSG 的 info 长度为 %d，至少需要 3", len(info))
	}

	meta := atArray(info, 0)    // 弹幕元信息
	userArr := atArray(info, 2) // 用户信息
	medalArr := atArray(info, 3)

	u := event.User{
		UID:         atString(userArr, 0),
		Username:    atString(userArr, 1),
		IsAdmin:     atInt64(userArr, 2) != 0,
		UserLevel:   int(atInt64(atArray(info, 4), 0)),
		GuardLevel:  int(atInt64(info, 7)),
		WealthLevel: int(atInt64(atArray(info, 16), 0)),
		Medal:       parseDanmuMedal(medalArr),
	}

	d := event.Danmaku{
		User:       u,
		Text:       atString(info, 1),
		Color:      formatColor(atInt64(meta, 3)),
		IsEmoticon: atInt64(meta, 12) == 1,
	}

	// info[0][15] 是 2023 年后新增的详情对象，含头像与回复信息。
	if detail := atObject(meta, 15); detail != nil {
		if base := getObject(getObject(detail, "user"), "base"); base != nil {
			d.User.AvatarURL = getString(base, "face")
		}
		// extra 是一个被再次编码为字符串的 JSON。
		if extraStr := getString(detail, "extra"); extraStr != "" {
			var extra map[string]any
			if err := json.Unmarshal([]byte(extraStr), &extra); err == nil {
				if getBool(extra, "show_reply") {
					if mid := getInt64(extra, "reply_mid"); mid != 0 {
						d.ReplyToUID = fmt.Sprintf("%d", mid)
						d.ReplyToName = getString(extra, "reply_uname")
					}
				}
			}
		}
	}

	ts := timeFromUnixMilli(atInt64(meta, 4))
	return []event.Event{NewEvent(ctx, event.TypeDanmaku, ts, d, raw)}, nil
}

// parseDanmuMedal 解析 info[3] 的勋章数组，未佩戴时返回 nil。
func parseDanmuMedal(a []any) *event.Medal {
	if len(a) < 4 {
		return nil
	}
	return &event.Medal{
		Level:      int(atInt64(a, 0)),
		Name:       atString(a, 1),
		AnchorName: atString(a, 2),
		RoomID:     atString(a, 3),
		GuardLevel: int(atInt64(a, 10)),
		IsLighted:  atInt64(a, 11) != 0,
		AnchorUID:  atString(a, 12),
	}
}

// formatColor 把十进制整数颜色转成 "#rrggbb"。
func formatColor(n int64) string {
	if n < 0 {
		n = 0
	}
	return fmt.Sprintf("#%06x", n&0xffffff)
}
