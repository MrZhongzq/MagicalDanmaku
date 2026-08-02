package bilibili

import (
	"context"

	"github.com/MrZhongzq/MagicalDanmaku/server/internal/event"
)

// OpponentSnapshot 是 PK 接通瞬间为某个「对面」直播间抓取的一次性快照。
//
// 三个指针字段各自独立、互不影响：任意一个接口失败只让它自己那个
// 字段留 nil（标记未知），不影响另外两个字段，也不影响其余对手——
// 这三个都是外部 HTTP 调用，会超时/限流/返回非 0 code，PK 播报绝不能
// 因为拿不到人数就整个不播，所以这里只做「有多少算多少」的降级，
// 从不返回会让调用方整体失败的 error。
type OpponentSnapshot struct {
	RoomID string
	UID    string

	Online      *int64 // 对面直播间在线人数；nil 表示未知（接口失败）
	GuardTotal  *int64 // 对面大航海总数；nil 表示未知
	GuardOnline *int64 // 对面大航海在线数；nil 表示未知
}

// FetchOpponentSnapshots 在 PK 接通的一瞬间为 members 里所有「对面」抓一次快照。
//
// 用户原话「只截取 PK 接通的一瞬间的数据」——这是一次性快照，不是轮询。
// 调用方负责保证只在 PK 刚接通、拿到完整 Members 时调用一次（比如按
// pk_id 去重），本函数本身不做任何缓存或节流，每调一次就真打一次接口。
//
// 「对面」的唯一合法判定是 member.RoomID != c.roomID——不读
// init_info/match_info，那两个字段的真实语义是发起方/被匹配方，
// 混用会在主播主动发起 PK 时把自己错认成对面（Task 4 的教训）。
// members 可能多于两方（B 站支持多人 PK），逐个处理，不假设只有一个对手。
//
// 超时契约：调用方不需要自己给 ctx 挂 deadline——本函数内部会用
// c.opponentSnapshotBudget（默认 defaultOpponentSnapshotBudget，可用
// WithOpponentSnapshotBudget 调整）兜底一个总预算，覆盖全部对手、全部
// 接口。即使传 context.Background()，最坏情况下（N 个对手 × 3 个接口，
// GuardOnline 还可能翻页）也不会真的按 N×3×15s 的上限跑下去——预算耗尽
// 后还没来得及跑的调用会立刻因 ctx 过期而失败，对应字段照常降级为 nil，
// 不影响已经拿到的字段和已经查完的其他对手。
func (c *Client) FetchOpponentSnapshots(ctx context.Context, members []event.PkMember) []OpponentSnapshot {
	budget := c.opponentSnapshotBudget
	if budget <= 0 {
		budget = defaultOpponentSnapshotBudget
	}
	ctx, cancel := context.WithTimeout(ctx, budget)
	defer cancel()

	snapshots := make([]OpponentSnapshot, 0, len(members))
	for _, m := range members {
		if m.RoomID == c.roomID {
			continue
		}
		snapshots = append(snapshots, c.fetchOneOpponentSnapshot(ctx, m))
	}
	return snapshots
}

// fetchOneOpponentSnapshot 抓单个对手的快照，三个接口相互独立降级。
func (c *Client) fetchOneOpponentSnapshot(ctx context.Context, m event.PkMember) OpponentSnapshot {
	snap := OpponentSnapshot{RoomID: m.RoomID, UID: m.UID}

	if online, err := c.api.RoomOnlineCount(ctx, m.RoomID); err != nil {
		c.log.Warn("获取对面直播间人数失败，降级为未知", "room", m.RoomID, "err", err)
	} else {
		snap.Online = &online
	}

	if total, err := c.api.GuardTotal(ctx, m.RoomID, m.UID); err != nil {
		c.log.Warn("获取对面大航海总数失败，降级为未知", "room", m.RoomID, "err", err)
	} else {
		snap.GuardTotal = &total
	}

	if counts, err := c.api.GuardOnline(ctx, m.RoomID, m.UID); err != nil {
		c.log.Warn("获取对面大航海在线数失败，降级为未知", "room", m.RoomID, "err", err)
	} else {
		total := counts.Total()
		snap.GuardOnline = &total
	}

	return snap
}
