package api

import (
	"context"
	"fmt"
	"net/url"
)

// RoomRecentDanmakuUIDs 取房间近期弹幕发送者的 uid 列表，用于 PK 连对面
// 时冷启动观众集合（原 C++ getRoomCurrentAudiences，
// bili_liveservice.cpp:3341-3365）。
//
// B 站没有「当前在线观众名单」这种接口，ajax/msg 返回的是最近一段时间
// 的弹幕记录，用发言者 uid 当观众集合的近似值——这是原项目的既有做法，
// 不是这次移植发明的语义，也不追求精确。
//
// uid 字段复用 guardUID（pk_opponent.go）做数字/字符串两种形式的兼容
// 解析：同一个坑，B 站不同接口对这一点并不统一。
func (c *Client) RoomRecentDanmakuUIDs(ctx context.Context, roomID string) ([]string, error) {
	params := url.Values{}
	params.Set("roomid", roomID)

	var data struct {
		Room []struct {
			UID guardUID `json:"uid"`
		} `json:"room"`
	}
	if err := c.GetJSON(ctx, c.URLFor("roomAudience"), params, false, &data); err != nil {
		return nil, fmt.Errorf("获取房间近期弹幕发送者失败: %w", err)
	}

	uids := make([]string, 0, len(data.Room))
	for _, d := range data.Room {
		if d.UID == "" {
			continue
		}
		uids = append(uids, string(d.UID))
	}
	return uids, nil
}
