package api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

// TestRoomRecentDanmakuUIDsParsesBothUIDForms 验证 uid 字段兼容数字与
// 带引号字符串两种形式——跟 GuardOnline 的 guardUID 是同一个坑，B 站不同
// 接口对这一点并不统一。
func TestRoomRecentDanmakuUIDsParsesBothUIDForms(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.URL.Query().Get("roomid"); got != "33333" {
			t.Errorf("roomid = %q, 期望 33333", got)
		}
		w.Write([]byte(`{"code":0,"data":{"room":[{"uid":123,"nickname":"甲"},{"uid":"456","nickname":"乙"}]}}`))
	}))
	t.Cleanup(srv.Close)

	c := New(nil, WithHTTPClient(srv.Client()))
	c.SetBaseURL("roomAudience", srv.URL)

	uids, err := c.RoomRecentDanmakuUIDs(context.Background(), "33333")
	if err != nil {
		t.Fatalf("RoomRecentDanmakuUIDs 失败: %v", err)
	}
	if len(uids) != 2 {
		t.Fatalf("uids 数量 = %d, 期望 2", len(uids))
	}
	if uids[0] != "123" || uids[1] != "456" {
		t.Errorf("uids = %v, 期望 [123 456]", uids)
	}
}

// TestRoomRecentDanmakuUIDsEmptyRoom 房间没有历史弹幕（刚开播）时应返回
// 空切片而不是报错——PK 接通瞬间刚开播的对面房间是完全正常的场景。
func TestRoomRecentDanmakuUIDsEmptyRoom(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"code":0,"data":{"room":[]}}`))
	}))
	t.Cleanup(srv.Close)

	c := New(nil, WithHTTPClient(srv.Client()))
	c.SetBaseURL("roomAudience", srv.URL)

	uids, err := c.RoomRecentDanmakuUIDs(context.Background(), "33333")
	if err != nil {
		t.Fatalf("RoomRecentDanmakuUIDs 失败: %v", err)
	}
	if len(uids) != 0 {
		t.Errorf("uids 数量 = %d, 期望 0", len(uids))
	}
}

// TestRoomRecentDanmakuUIDsFailsOnBadCode 接口返回非 0 code 时应报错，
// 由调用方（PkLink 播种观众集合）决定如何降级——这里只负责如实传递失败。
func TestRoomRecentDanmakuUIDsFailsOnBadCode(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"code":-352,"message":"风控"}`))
	}))
	t.Cleanup(srv.Close)

	c := New(nil, WithHTTPClient(srv.Client()))
	c.SetBaseURL("roomAudience", srv.URL)

	if _, err := c.RoomRecentDanmakuUIDs(context.Background(), "33333"); err == nil {
		t.Fatal("期望返回 error")
	}
}
