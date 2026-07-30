package api

import (
	"context"
	"fmt"
	"net/url"
	"strconv"
)

// 直播状态取值。
const (
	LiveStatusOffline = 0 // 未开播
	LiveStatusLiving  = 1 // 直播中
	LiveStatusRound   = 2 // 轮播中
)

// RoomInfo 是直播间基础信息。
type RoomInfo struct {
	RoomID         string // 真实长房间号
	ShortID        string // 短号，可能为空
	UID            string // 主播 UID
	Title          string
	LiveStatus     int
	AreaID         string
	AreaName       string
	ParentAreaID   string
	ParentAreaName string
	Attention      int64  // 粉丝数
	Online         int64  // 在线人数
	LiveStartTime  string // 开播时间，形如 "2026-07-29 19:00:00"，未开播时为 "0000-00-00 00:00:00"
}

// IsLiving 判断是否正在直播。
func (r *RoomInfo) IsLiving() bool { return r.LiveStatus == LiveStatusLiving }

// RoomInfo 获取直播间基础信息。
//
// 传入短号也可调用，返回值中的 RoomID 是真实长号，
// 后续所有操作都应使用长号。
func (c *Client) RoomInfo(ctx context.Context, roomID string) (*RoomInfo, error) {
	params := url.Values{}
	params.Set("room_id", roomID)

	var data struct {
		RoomID         int64  `json:"room_id"`
		ShortID        int64  `json:"short_id"`
		UID            int64  `json:"uid"`
		Title          string `json:"title"`
		LiveStatus     int    `json:"live_status"`
		AreaID         int64  `json:"area_id"`
		AreaName       string `json:"area_name"`
		ParentAreaID   int64  `json:"parent_area_id"`
		ParentAreaName string `json:"parent_area_name"`
		Attention      int64  `json:"attention"`
		Online         int64  `json:"online"`
		LiveTime       string `json:"live_time"`
	}
	if err := c.GetJSON(ctx, c.URLFor("roomInfo"), params, false, &data); err != nil {
		return nil, fmt.Errorf("获取直播间信息失败: %w", err)
	}

	info := &RoomInfo{
		RoomID:         strconv.FormatInt(data.RoomID, 10),
		UID:            strconv.FormatInt(data.UID, 10),
		Title:          data.Title,
		LiveStatus:     data.LiveStatus,
		AreaID:         strconv.FormatInt(data.AreaID, 10),
		AreaName:       data.AreaName,
		ParentAreaID:   strconv.FormatInt(data.ParentAreaID, 10),
		ParentAreaName: data.ParentAreaName,
		Attention:      data.Attention,
		Online:         data.Online,
		LiveStartTime:  data.LiveTime,
	}
	if data.ShortID != 0 {
		info.ShortID = strconv.FormatInt(data.ShortID, 10)
	}
	return info, nil
}

// Host 是一个弹幕长连接服务器。
type Host struct {
	Host    string `json:"host"`
	Port    int    `json:"port"`
	WssPort int    `json:"wss_port"`
	WsPort  int    `json:"ws_port"`
}

// WSSURL 返回该服务器的 WebSocket 连接地址。
func (h Host) WSSURL() string {
	port := h.WssPort
	if port == 0 {
		port = 443
	}
	return fmt.Sprintf("wss://%s:%d/sub", h.Host, port)
}

// DanmuInfo 是建立弹幕长连接所需的认证信息。
type DanmuInfo struct {
	Token string // 认证包中的 key 字段
	Hosts []Host // 可用服务器列表，按优先级排序
}

// DanmuInfo 获取弹幕长连接的 token 与服务器列表。
//
// 该接口要求 wbi 签名；Cookie 缺少 buvid3 时会返回 -352 风控错误，
// 调用方应补齐设备字段后重试一次。
func (c *Client) DanmuInfo(ctx context.Context, roomID string) (*DanmuInfo, error) {
	params := url.Values{}
	params.Set("id", roomID)
	params.Set("type", "0")

	var data struct {
		Token    string `json:"token"`
		HostList []Host `json:"host_list"`
	}
	if err := c.GetJSON(ctx, c.URLFor("danmuInfo"), params, true, &data); err != nil {
		return nil, fmt.Errorf("获取弹幕服务器信息失败: %w", err)
	}
	if len(data.HostList) == 0 {
		return nil, fmt.Errorf("api: 弹幕服务器列表为空")
	}
	return &DanmuInfo{Token: data.Token, Hosts: data.HostList}, nil
}
