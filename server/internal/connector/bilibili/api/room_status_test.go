package api

import (
	"context"
	"net/http"
	"testing"
)

// TestRoomStatus 验证走的是 xlive/web-room/v1/index/getInfoByRoom
// （roomOnline），一次请求同时拿到开播状态、房间号与主播 UID/昵称——
// data.room_info.uid/live_status 与 data.anchor_info.base_info.uname
// 的字段路径取自原 C++ 项目 bili_liveservice.cpp:495-506 真实调用过的
// 取值（见 getInfoByRoom 响应解析那段），不是从通用知识猜的。这是唯一
// 确认过带主播昵称的接口——RoomInfo 用的 room/v1/Room/get_info 没有
// 这个字段。
func TestRoomStatus(t *testing.T) {
	var gotRoomID string
	c, srv := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		gotRoomID = r.URL.Query().Get("room_id")
		w.Write([]byte(`{"code":0,"data":{
			"room_info":{"room_id":21452505,"uid":20285041,"title":"今天也在唱歌","live_status":1},
			"anchor_info":{"base_info":{"uname":"舞月雅白"}}
		}}`))
	})
	c.SetBaseURL("roomOnline", srv.URL)

	status, err := c.RoomStatus(context.Background(), "21452505")
	if err != nil {
		t.Fatalf("RoomStatus 失败: %v", err)
	}
	if gotRoomID != "21452505" {
		t.Errorf("room_id = %q", gotRoomID)
	}
	if status.RoomID != "21452505" {
		t.Errorf("RoomID = %q", status.RoomID)
	}
	if status.AnchorUID != "20285041" {
		t.Errorf("AnchorUID = %q, 期望 20285041（主播 UID，不是房间号）", status.AnchorUID)
	}
	if status.AnchorName != "舞月雅白" {
		t.Errorf("AnchorName = %q", status.AnchorName)
	}
	if status.Title != "今天也在唱歌" {
		t.Errorf("Title = %q", status.Title)
	}
	if status.LiveStatus != 1 {
		t.Errorf("LiveStatus = %d", status.LiveStatus)
	}
	if !status.IsLiving() {
		t.Error("live_status=1 时 IsLiving 应为 true")
	}
}

// TestRoomStatusNotLiving 覆盖未开播（live_status=0）时 IsLiving 应为 false，
// 与 TestRoomStatus 的开播场景成对，避免只测过"开播"这一种取值就断言
// IsLiving 的实现是对的。
func TestRoomStatusNotLiving(t *testing.T) {
	c, srv := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"code":0,"data":{
			"room_info":{"room_id":21452505,"uid":20285041,"title":"下播了","live_status":0},
			"anchor_info":{"base_info":{"uname":"舞月雅白"}}
		}}`))
	})
	c.SetBaseURL("roomOnline", srv.URL)

	status, err := c.RoomStatus(context.Background(), "21452505")
	if err != nil {
		t.Fatalf("RoomStatus 失败: %v", err)
	}
	if status.IsLiving() {
		t.Error("live_status=0 时 IsLiving 应为 false")
	}
}

// TestRoomStatusDegradesOnAPIError 验证接口返回业务错误（如风控 -352）
// 时能拿到 error 自行降级，而不是把"拿不到"悄悄当成"没开播"——这是
// P5-2 反复强调的一条红线，必须在数据结构上可区分：这里的判据是
// RoomStatus 返回 (nil, non-nil error)，调用方据此走 unknown 分支，
// 不能把 err 吞掉再返回一个 LiveStatus 全零值的 *RoomStatus。
func TestRoomStatusDegradesOnAPIError(t *testing.T) {
	c, srv := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"code":-352,"message":"风控校验失败"}`))
	})
	c.SetBaseURL("roomOnline", srv.URL)

	status, err := c.RoomStatus(context.Background(), "21452505")
	if err == nil {
		t.Fatal("应当返回错误")
	}
	if status != nil {
		t.Errorf("探测失败时不该返回非 nil 的 status，实际: %+v", status)
	}
	if !IsRiskControl(err) {
		t.Errorf("应判定为风控，实际 %v", err)
	}
}
