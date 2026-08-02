package api

import (
	"context"
	"net/http"
	"net/url"
	"testing"
)

// TestRoomOnlineCount 验证走的是 xlive/web-room/v1/index/getInfoByRoom，
// 取 data.room_info.online——跟 RoomInfo 用的 room/v1/Room/get_info
// 是两个不同接口，字段路径不能混用。
func TestRoomOnlineCount(t *testing.T) {
	var gotRoomID string
	c, srv := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		gotRoomID = r.URL.Query().Get("room_id")
		w.Write([]byte(`{"code":0,"data":{"room_info":{"online":888}}}`))
	})
	c.SetBaseURL("roomOnline", srv.URL)

	online, err := c.RoomOnlineCount(context.Background(), "33333")
	if err != nil {
		t.Fatalf("RoomOnlineCount 失败: %v", err)
	}
	if gotRoomID != "33333" {
		t.Errorf("room_id = %q", gotRoomID)
	}
	if online != 888 {
		t.Errorf("online = %d, 期望 888", online)
	}
}

// TestRoomOnlineCountDegradesOnAPIError 验证接口返回业务错误时，
// 调用方能拿到 error 自行降级，而不是 panic 或吞掉错误装作成功。
func TestRoomOnlineCountDegradesOnAPIError(t *testing.T) {
	c, srv := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"code":-400,"message":"参数错误"}`))
	})
	c.SetBaseURL("roomOnline", srv.URL)

	_, err := c.RoomOnlineCount(context.Background(), "33333")
	if err == nil {
		t.Fatal("应当返回错误")
	}
}

// TestGuardTotal 验证单次请求即可拿到大航海总数，不需要翻页。
func TestGuardTotal(t *testing.T) {
	var gotQuery url.Values
	c, srv := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		gotQuery = r.URL.Query()
		w.Write([]byte(`{"code":0,"data":{"info":{"num":42}}}`))
	})
	c.SetBaseURL("guardTotal", srv.URL)

	total, err := c.GuardTotal(context.Background(), "33333", "22222222")
	if err != nil {
		t.Fatalf("GuardTotal 失败: %v", err)
	}
	if total != 42 {
		t.Errorf("total = %d, 期望 42", total)
	}
	if gotQuery.Get("roomid") != "33333" {
		t.Errorf("roomid = %q", gotQuery.Get("roomid"))
	}
	if gotQuery.Get("ruid") != "22222222" {
		t.Errorf("ruid = %q", gotQuery.Get("ruid"))
	}
	if gotQuery.Get("page") != "1" {
		t.Errorf("page = %q, 期望 1（单次请求即可拿到总数）", gotQuery.Get("page"))
	}
}

// TestGuardOnlinePagination 覆盖研究文件里明确写出的两条易错语义：
//  1. top3 只在 page==1 时累加，否则翻页会重复计数；
//  2. 只数 is_alive != 0 的，「在线」不等于全部成员。
//
// page1: top3 有 2 个总督（1 个 is_alive=0），list 有 1 提督 + 1 舰长；
// page2: top3 又出现 1 个总督（不该被算，因为不是 page1），list 有 1 舰长。
// 正确结果：总督=1（只数 page1 的 is_alive=1 那个）、提督=1、舰长=2，合计 4。
func TestGuardOnlinePagination(t *testing.T) {
	var requests []url.Values
	c, srv := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		requests = append(requests, q)
		switch q.Get("page") {
		case "1":
			w.Write([]byte(`{"code":0,"data":{
				"top3":[
					{"guard_level":1,"is_alive":1,"username":"总督甲"},
					{"guard_level":1,"is_alive":0,"username":"总督乙(已掉线)"}
				],
				"list":[
					{"guard_level":2,"is_alive":1,"username":"提督丙"},
					{"guard_level":3,"is_alive":1,"username":"舰长丁"}
				],
				"info":{"page":2,"now":1}
			}}`))
		case "2":
			w.Write([]byte(`{"code":0,"data":{
				"top3":[
					{"guard_level":1,"is_alive":1,"username":"不该被算的总督"}
				],
				"list":[
					{"guard_level":3,"is_alive":1,"username":"舰长戊"}
				],
				"info":{"page":2,"now":2}
			}}`))
		default:
			t.Errorf("意外的分页请求: page=%q", q.Get("page"))
		}
	})
	c.SetBaseURL("guardOnline", srv.URL)

	counts, err := c.GuardOnline(context.Background(), "33333", "22222222")
	if err != nil {
		t.Fatalf("GuardOnline 失败: %v", err)
	}
	if counts.Governor != 1 {
		t.Errorf("Governor(总督) = %d, 期望 1", counts.Governor)
	}
	if counts.Admiral != 1 {
		t.Errorf("Admiral(提督) = %d, 期望 1", counts.Admiral)
	}
	if counts.Captain != 2 {
		t.Errorf("Captain(舰长) = %d, 期望 2", counts.Captain)
	}
	if counts.Total() != 4 {
		t.Errorf("Total() = %d, 期望 4", counts.Total())
	}
	if len(requests) != 2 {
		t.Fatalf("请求次数 = %d, 期望 2（翻到 now>=page 为止）", len(requests))
	}
	if requests[0].Get("appkey") == "" || requests[0].Get("actionKey") == "" {
		t.Error("缺少 appkey/actionKey 参数")
	}
	if requests[0].Get("page_size") != "30" {
		t.Errorf("page_size = %q, 期望 30", requests[0].Get("page_size"))
	}
}

// TestGuardOnlineSinglePage 验证只有一页时不会多请求一次。
func TestGuardOnlineSinglePage(t *testing.T) {
	calls := 0
	c, srv := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		calls++
		w.Write([]byte(`{"code":0,"data":{
			"top3":[{"guard_level":1,"is_alive":1,"username":"总督甲"}],
			"list":[],
			"info":{"page":1,"now":1}
		}}`))
	})
	c.SetBaseURL("guardOnline", srv.URL)

	counts, err := c.GuardOnline(context.Background(), "33333", "22222222")
	if err != nil {
		t.Fatalf("GuardOnline 失败: %v", err)
	}
	if counts.Total() != 1 {
		t.Errorf("Total() = %d, 期望 1", counts.Total())
	}
	if calls != 1 {
		t.Errorf("请求次数 = %d, 期望 1", calls)
	}
}

// TestGuardOnlineStopsOnMalformedPagination 验证分页信息异常（now 不推进）
// 时不会无限请求下去，而是有上限地退出——外部接口的分页字段不可信任。
func TestGuardOnlineStopsOnMalformedPagination(t *testing.T) {
	calls := 0
	c, srv := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		calls++
		// info.now 永远卡在 1，但 page 一直是 2：模拟接口分页信息异常。
		w.Write([]byte(`{"code":0,"data":{"top3":[],"list":[],"info":{"page":2,"now":1}}}`))
	})
	c.SetBaseURL("guardOnline", srv.URL)

	_, err := c.GuardOnline(context.Background(), "33333", "22222222")
	if err != nil {
		t.Fatalf("GuardOnline 失败: %v", err)
	}
	if calls <= 1 {
		t.Fatal("前置条件错误：应至少翻页一次才能验证上限")
	}
	if calls > guardOnlineMaxPages {
		t.Errorf("请求次数 = %d, 超过上限 %d，未能及时退出", calls, guardOnlineMaxPages)
	}
}
