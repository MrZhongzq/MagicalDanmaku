package bilibili

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/MrZhongzq/MagicalDanmaku/server/internal/connector/bilibili/api"
	"github.com/MrZhongzq/MagicalDanmaku/server/internal/event"
)

// newSnapshotTestClient 起一个能同时应答 roomOnline/guardTotal/guardOnline
// 三个接口的假服务器。三个接口的参数有重叠（roomOnline 和 guardOnline
// 都带 room_id），按各自独有的参数区分「谁在问谁」：guardOnline 独有
// switch 参数，guardTotal 独有 roomid（不带下划线）参数，剩下的才是
// roomOnline。
func newSnapshotTestClient(t *testing.T, selfRoomID string, roomOnline map[string]int64, guardTotal map[string]int64, guardOnline map[string]int64) (*Client, *int) {
	t.Helper()
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		q := r.URL.Query()
		switch {
		case q.Has("switch"): // guardOnline（queryContributionRank）
			n := guardOnline[q.Get("room_id")]
			// 全部塞进 item、guard_level=3，uid 各不相同以免被去重逻辑误伤，
			// 只为验证总数管道打通，分档/去重/翻页语义已在 api 包的单元
			// 测试里覆盖，这里不重复造样例。
			items := "["
			for i := int64(0); i < n; i++ {
				if i > 0 {
					items += ","
				}
				items += fmt.Sprintf(`{"uid":%d,"guard_level":3}`, i+1)
			}
			items += "]"
			w.Write([]byte(`{"code":0,"data":{"item":` + items + `,"count":` + itoa(n) + `}}`))
		case q.Has("roomid"): // guardTotal
			total := guardTotal[q.Get("roomid")]
			w.Write([]byte(`{"code":0,"data":{"info":{"num":` + itoa(total) + `}}}`))
		default: // roomOnline
			online := roomOnline[q.Get("room_id")]
			w.Write([]byte(`{"code":0,"data":{"room_info":{"online":` + itoa(online) + `}}}`))
		}
	}))
	t.Cleanup(srv.Close)

	apiClient := api.New(nil, api.WithHTTPClient(srv.Client()))
	apiClient.SetBaseURL("roomOnline", srv.URL)
	apiClient.SetBaseURL("guardTotal", srv.URL)
	apiClient.SetBaseURL("guardOnline", srv.URL)

	c := NewClient(selfRoomID, apiClient, WithLogger(slog.New(slog.NewTextHandler(discardWriter{}, nil))))
	return c, &calls
}

func itoa(n int64) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var buf [20]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		buf[i] = '-'
	}
	return string(buf[i:])
}

type discardWriter struct{}

func (discardWriter) Write(p []byte) (int, error) { return len(p), nil }

// TestFetchOpponentSnapshotsSkipsSelf 验证「对面」的唯一合法判定是
// member.RoomID != c.roomID——绝不能用 init_info/match_info（Task 4 的教训）。
// 这里故意把 self 排在 members 的中间，防止实现依赖下标顺序。
func TestFetchOpponentSnapshotsSkipsSelf(t *testing.T) {
	c, calls := newSnapshotTestClient(t, "21452505",
		map[string]int64{"33333": 100, "44444": 200},
		map[string]int64{"33333": 10, "44444": 20},
		map[string]int64{"33333": 5, "44444": 8},
	)

	members := []event.PkMember{
		{RoomID: "33333", UID: "u33333"},
		{RoomID: "21452505", UID: "u-self"}, // 自己，必须被跳过
		{RoomID: "44444", UID: "u44444"},
	}

	snaps := c.FetchOpponentSnapshots(context.Background(), members)
	if len(snaps) != 2 {
		t.Fatalf("快照数量 = %d, 期望 2（自己不应出现）", len(snaps))
	}

	byRoom := make(map[string]OpponentSnapshot, len(snaps))
	for _, s := range snaps {
		byRoom[s.RoomID] = s
		if s.RoomID == "21452505" {
			t.Fatal("自己不应出现在对面快照里")
		}
	}

	a, ok := byRoom["33333"]
	if !ok {
		t.Fatal("缺少 RoomID=33333 的快照")
	}
	if a.Online == nil || *a.Online != 100 {
		t.Errorf("33333.Online = %v, 期望 100", a.Online)
	}
	if a.GuardTotal == nil || *a.GuardTotal != 10 {
		t.Errorf("33333.GuardTotal = %v, 期望 10", a.GuardTotal)
	}
	if a.GuardOnline == nil || *a.GuardOnline != 5 {
		t.Errorf("33333.GuardOnline = %v, 期望 5", a.GuardOnline)
	}

	b, ok := byRoom["44444"]
	if !ok {
		t.Fatal("缺少 RoomID=44444 的快照")
	}
	if b.Online == nil || *b.Online != 200 {
		t.Errorf("44444.Online = %v, 期望 200", b.Online)
	}

	// 三个对手 * 三个接口 = 6 次请求；不该因为多一方就漏查或错配。
	if *calls != 6 {
		t.Errorf("请求次数 = %d, 期望 6", *calls)
	}
}

// TestFetchOpponentSnapshotsDegradesOnPartialFailure 证明约束：
// 「PK 播报绝不能因为拿不到人数就整个不播」——guardTotal 接口失败时，
// 该对手的 GuardTotal 留 nil（标记未知），但 Online/GuardOnline 仍然
// 正常拿到，其余对手也不受影响，函数本身不返回 error 让调用方整体失败。
func TestFetchOpponentSnapshotsDegradesOnPartialFailure(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		switch {
		case q.Has("switch"): // guardOnline
			w.Write([]byte(`{"code":0,"data":{"item":[{"uid":1,"guard_level":3}],"count":1}}`))
		case q.Has("roomid"): // guardTotal：故意让它失败
			w.Write([]byte(`{"code":-352,"message":"风控"}`))
		default: // roomOnline
			w.Write([]byte(`{"code":0,"data":{"room_info":{"online":999}}}`))
		}
	}))
	t.Cleanup(srv.Close)

	apiClient := api.New(nil, api.WithHTTPClient(srv.Client()))
	apiClient.SetBaseURL("roomOnline", srv.URL)
	apiClient.SetBaseURL("guardTotal", srv.URL)
	apiClient.SetBaseURL("guardOnline", srv.URL)

	c := NewClient("21452505", apiClient, WithLogger(slog.New(slog.NewTextHandler(discardWriter{}, nil))))

	members := []event.PkMember{
		{RoomID: "33333", UID: "u33333"},
		{RoomID: "44444", UID: "u44444"},
	}

	snaps := c.FetchOpponentSnapshots(context.Background(), members)
	if len(snaps) != 2 {
		t.Fatalf("快照数量 = %d, 期望 2（一个接口失败不该导致整体丢对手）", len(snaps))
	}
	for _, s := range snaps {
		if s.GuardTotal != nil {
			t.Errorf("RoomID=%s: GuardTotal 应为 nil（接口失败后降级），实际 %v", s.RoomID, *s.GuardTotal)
		}
		if s.Online == nil || *s.Online != 999 {
			t.Errorf("RoomID=%s: Online 应为 999，不受 GuardTotal 失败影响，实际 %v", s.RoomID, s.Online)
		}
		if s.GuardOnline == nil || *s.GuardOnline != 1 {
			t.Errorf("RoomID=%s: GuardOnline 应为 1，不受 GuardTotal 失败影响，实际 %v", s.RoomID, s.GuardOnline)
		}
	}
}

// TestFetchOpponentSnapshotsEmptyMembers 空 Members（无 PK_INFO 数据时）
// 不应 panic，应返回空切片。
func TestFetchOpponentSnapshotsEmptyMembers(t *testing.T) {
	apiClient := api.New(nil)
	c := NewClient("21452505", apiClient)
	snaps := c.FetchOpponentSnapshots(context.Background(), nil)
	if len(snaps) != 0 {
		t.Errorf("空 Members 应返回空快照，实际 %d 条", len(snaps))
	}
}
