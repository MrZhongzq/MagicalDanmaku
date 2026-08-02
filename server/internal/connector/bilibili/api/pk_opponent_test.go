package api

import (
	"context"
	"fmt"
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

// TestGuardTotal 验证单次请求即可拿到大航海总数，用的是
// getGuardCountByRoomId 的参数形状（roomid/page=1/ruid，没有 page_size/typ
// ——那是写死查自己房间的 getGuardCount 才带的参数，任意房间版本不需要）。
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

// guardOnlinePageJSON 拼一页 queryContributionRank 的响应体。
func guardOnlinePageJSON(count int, items string) string {
	return fmt.Sprintf(`{"code":0,"data":{"item":%s,"count":%d}}`, items, count)
}

// TestGuardOnlineDedupesByUID 覆盖研究文件里明确写出的、这个接口特有的坑：
// 同一个 uid 会跨页重复出现，不去重会多算。用 count=150、page_size=100
// 强制产生 2 页（pageCount=ceil(150/100)=2）；page2 里混入一个 page1 出现过
// 的 uid=1，正确结果里这个 uid 只能被数一次。
func TestGuardOnlineDedupesByUID(t *testing.T) {
	var requests []url.Values
	c, srv := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		requests = append(requests, q)
		switch q.Get("page") {
		case "1":
			w.Write([]byte(guardOnlinePageJSON(150,
				`[{"uid":1,"guard_level":1},{"uid":2,"guard_level":2}]`)))
		case "2":
			// uid=1 重复出现，guard_level 换了个不同值也不该被二次计数；
			// uid=3 是这一页真正的新成员。
			w.Write([]byte(guardOnlinePageJSON(150,
				`[{"uid":1,"guard_level":1},{"uid":3,"guard_level":3}]`)))
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
		t.Errorf("Governor(总督) = %d, 期望 1（uid=1 跨页重复，只能算一次）", counts.Governor)
	}
	if counts.Admiral != 1 {
		t.Errorf("Admiral(提督) = %d, 期望 1", counts.Admiral)
	}
	if counts.Captain != 1 {
		t.Errorf("Captain(舰长) = %d, 期望 1", counts.Captain)
	}
	if counts.Total() != 3 {
		t.Errorf("Total() = %d, 期望 3", counts.Total())
	}
	if len(requests) != 2 {
		t.Fatalf("请求次数 = %d, 期望 2（page < pageCount 就该继续翻页）", len(requests))
	}
	if requests[0].Get("room_id") != "33333" || requests[0].Get("ruid") != "22222222" {
		t.Errorf("room_id/ruid 参数不对: %v", requests[0])
	}
	if requests[0].Get("type") != "online_rank" || requests[0].Get("switch") != "contribution_rank" || requests[0].Get("platform") != "web" {
		t.Errorf("缺少固定参数 type/switch/platform: %v", requests[0])
	}
}

// TestGuardOnlineFiltersNonGuardAndUnknownLevel 覆盖两条互相独立的过滤规则：
//  1. guard_level<=0 的不是大航海成员，跳过；
//  2. guard_level 只精确匹配 1/2/3，不认识的档位（如未来新增的 4）
//     不计入任何一档——不是「非 1/2 就算舰长」。
func TestGuardOnlineFiltersNonGuardAndUnknownLevel(t *testing.T) {
	c, srv := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(guardOnlinePageJSON(4, `[
			{"uid":1,"guard_level":0},
			{"uid":2,"guard_level":-1},
			{"uid":3,"guard_level":4},
			{"uid":4,"guard_level":3}
		]`)))
	})
	c.SetBaseURL("guardOnline", srv.URL)

	counts, err := c.GuardOnline(context.Background(), "33333", "22222222")
	if err != nil {
		t.Fatalf("GuardOnline 失败: %v", err)
	}
	if counts.Total() != 1 {
		t.Errorf("Total() = %d, 期望 1（只有 uid=4 的 guard_level=3 该被计入）", counts.Total())
	}
	if counts.Captain != 1 {
		t.Errorf("Captain(舰长) = %d, 期望 1", counts.Captain)
	}
	if counts.Governor != 0 || counts.Admiral != 0 {
		t.Errorf("Governor/Admiral 应为 0，实际 %d/%d", counts.Governor, counts.Admiral)
	}
}

// TestGuardOnlineInvalidLevelDoesNotConsumeDedup 覆盖一处容易被忽略的顺序依赖：
// guard_level<=0 的过滤必须发生在「按 uid 去重」之前，不能反过来。如果顺序反了，
// 同一个 uid 先以无效等级出现一次，会被误标记为「已处理」，等它后面真的带着
// 有效等级出现时反而被当成重复而跳过——白白漏掉一个人。
// 这条测试跟 TestGuardOnlineFiltersNonGuardAndUnknownLevel 不同：那条测试里
// 每个 uid 只出现一次，测不出「过滤」和「去重」谁先谁后的差异；这里让同一个
// uid 先后带着无效/有效等级各出现一次，才能把顺序错误的实现和顺序正确的
// 实现区分开。
func TestGuardOnlineInvalidLevelDoesNotConsumeDedup(t *testing.T) {
	c, srv := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(guardOnlinePageJSON(2, `[
			{"uid":5,"guard_level":0},
			{"uid":5,"guard_level":3}
		]`)))
	})
	c.SetBaseURL("guardOnline", srv.URL)

	counts, err := c.GuardOnline(context.Background(), "33333", "22222222")
	if err != nil {
		t.Fatalf("GuardOnline 失败: %v", err)
	}
	if counts.Total() != 1 {
		t.Errorf("Total() = %d, 期望 1（无效等级那次不该抢占 uid=5 的去重名额）", counts.Total())
	}
	if counts.Captain != 1 {
		t.Errorf("Captain(舰长) = %d, 期望 1", counts.Captain)
	}
}

// TestGuardOnlineSinglePage 验证只有一页时不会多请求一次。
func TestGuardOnlineSinglePage(t *testing.T) {
	calls := 0
	c, srv := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		calls++
		w.Write([]byte(guardOnlinePageJSON(1, `[{"uid":1,"guard_level":1}]`)))
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

// TestGuardOnlineStopsOnUntrustworthyCount 验证 data.count 异常巨大（服务端
// 给的值，不可信）时不会被算出的天文数字 pageCount 拖着无限翻页下去，而是
// 有一个跟服务端返回值无关的硬上限——用请求计数器而不是「page 值本身」
// 判断是否超限，避免退回到「靠外部字段判断上限」这种同样不可信的写法。
func TestGuardOnlineStopsOnUntrustworthyCount(t *testing.T) {
	calls := 0
	c, srv := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		calls++
		// count 大到需要几百万页才翻得完，模拟接口返回异常/被污染的总数。
		w.Write([]byte(guardOnlinePageJSON(999999999, `[]`)))
	})
	c.SetBaseURL("guardOnline", srv.URL)

	_, err := c.GuardOnline(context.Background(), "33333", "22222222")
	if err != nil {
		t.Fatalf("GuardOnline 失败: %v", err)
	}
	if calls != guardOnlineMaxPages {
		t.Errorf("请求次数 = %d, 期望恰好 %d（应被安全上限精确拦住）", calls, guardOnlineMaxPages)
	}
}
