package api

import (
	"context"
	"fmt"
	"net/url"
	"strconv"
)

// RoomStatus 是心跳检测/立即检测用到的直播间状态快照：开播状态 +
// 主播身份（UID + 昵称）。
//
// 用的是 xlive/web-room/v1/index/getInfoByRoom（client.go 里已经存在
// 的 roomOnline，此前只被 RoomOnlineCount/GuardTotal 等 PK 场景用来查
// "对面"房间），不是 RoomInfo 用的 room/v1/Room/get_info——后者没有
// 主播昵称这个字段（核实见 room.go 里 RoomInfo 的结构体定义），
// data.anchor_info.base_info.uname 是唯一确认过的主播昵称来源，取自
// 原 C++ 项目 bili_liveservice.cpp:495-506 真实调用过的字段路径（那里
// 把它写进 ac->upName），不是从通用知识猜的接口。一次请求同时拿到
// live_status/uid/uname，比分别调两个接口更省一次风控敞口。
type RoomStatus struct {
	RoomID     string
	AnchorUID  string // 主播 UID，不是房间号
	AnchorName string
	Title      string
	LiveStatus int
}

// IsLiving 判断是否正在直播，语义与 RoomInfo.IsLiving 一致
// （只有 LiveStatusLiving 才算，轮播中不算）。
func (r *RoomStatus) IsLiving() bool { return r.LiveStatus == LiveStatusLiving }

// RoomStatus 获取直播间的开播状态与主播身份。
//
// 不需要登录态（signed=false）：这个接口本来就用于查任意房间（PK 对面），
// 查"自己"的房间同样不需要认证。
func (c *Client) RoomStatus(ctx context.Context, roomID string) (*RoomStatus, error) {
	params := url.Values{}
	params.Set("room_id", roomID)

	var data struct {
		RoomInfo struct {
			RoomID     int64  `json:"room_id"`
			UID        int64  `json:"uid"`
			Title      string `json:"title"`
			LiveStatus int    `json:"live_status"`
		} `json:"room_info"`
		AnchorInfo struct {
			BaseInfo struct {
				Uname string `json:"uname"`
			} `json:"base_info"`
		} `json:"anchor_info"`
	}
	if err := c.GetJSON(ctx, c.URLFor("roomOnline"), params, false, &data); err != nil {
		// 探测失败（网络错误、超时、风控等）必须原样把 error 冒泡上去，
		// 不能返回一个 LiveStatus 全零值（恰好等于 LiveStatusOffline）
		// 的 *RoomStatus——那样调用方没法把"拿不到"和"确认没开播"区分
		// 开，正是 P5-2 反复强调的一条红线。
		return nil, fmt.Errorf("获取直播间状态失败: %w", err)
	}

	return &RoomStatus{
		RoomID:     strconv.FormatInt(data.RoomInfo.RoomID, 10),
		AnchorUID:  strconv.FormatInt(data.RoomInfo.UID, 10),
		AnchorName: data.AnchorInfo.BaseInfo.Uname,
		Title:      data.RoomInfo.Title,
		LiveStatus: data.RoomInfo.LiveStatus,
	}, nil
}
