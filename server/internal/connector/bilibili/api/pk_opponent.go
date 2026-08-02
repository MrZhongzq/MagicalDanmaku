package api

import (
	"context"
	"fmt"
	"net/url"
	"strconv"
)

// 以下参数取自原 C++ 项目 bili_liveservice.cpp 里真实调用过的取值
// （getGuardCount / getPkOnlineGuardPage），不是随意选的默认值：
// appkey 是抓包核实过的固定值，page_size 是原实现用的档位。
const (
	guardTotalPageSize  = "20"
	guardOnlinePageSize = "30"
	guardOnlineAppKey   = "27eb53fc9058f8c3"

	// guardOnlineMaxPages 是翻页安全上限，防止接口返回异常的分页信息
	// （data.info.now 卡住不推进）导致无限请求下去。原 C++ 实现没有这层
	// 保护，翻页逻辑完全信任接口——这是本次移植额外加的防御。
	guardOnlineMaxPages = 100
)

// RoomOnlineCount 获取指定直播间当前在线人数。
//
// 用的是 xlive/web-room/v1/index/getInfoByRoom，跟 RoomInfo 用的
// room/v1/Room/get_info 是两个不同接口——原 C++ 项目对「自己」房间就是
// 用这个接口取 online（bili_liveservice.cpp:495-551），这里换成任意
// 房间号，专给「查对面直播间人数」这个场景用。
func (c *Client) RoomOnlineCount(ctx context.Context, roomID string) (int64, error) {
	params := url.Values{}
	params.Set("room_id", roomID)

	var data struct {
		RoomInfo struct {
			Online int64 `json:"online"`
		} `json:"room_info"`
	}
	if err := c.GetJSON(ctx, c.URLFor("roomOnline"), params, false, &data); err != nil {
		return 0, fmt.Errorf("获取直播间在线人数失败: %w", err)
	}
	return data.RoomInfo.Online, nil
}

// GuardTotal 获取指定直播间的大航海总数。单次请求即可拿到，不需要翻页
// （原 C++ 项目 getGuardCount 就是这么用的，bili_liveservice.cpp:1129-1133）。
func (c *Client) GuardTotal(ctx context.Context, roomID, uid string) (int64, error) {
	params := url.Values{}
	params.Set("roomid", roomID)
	params.Set("page", "1")
	params.Set("ruid", uid)
	params.Set("page_size", guardTotalPageSize)
	params.Set("typ", "0")

	var data struct {
		Info struct {
			Num int64 `json:"num"`
		} `json:"info"`
	}
	if err := c.GetJSON(ctx, c.URLFor("guardTotal"), params, false, &data); err != nil {
		return 0, fmt.Errorf("获取大航海总数失败: %w", err)
	}
	return data.Info.Num, nil
}

// GuardOnlineCounts 是在线大航海按等级分档的统计。
type GuardOnlineCounts struct {
	Governor int64 // 总督（guard_level=1）
	Admiral  int64 // 提督（guard_level=2）
	Captain  int64 // 舰长（guard_level 其余取值，实际只有 3）
}

// Total 返回三档之和，即「在线」的大航海总数。
func (g GuardOnlineCounts) Total() int64 { return g.Governor + g.Admiral + g.Captain }

// guardOnlineMember 是 topList 接口 top3/list 数组里一个成员的形状。
type guardOnlineMember struct {
	GuardLevel int `json:"guard_level"`
	IsAlive    int `json:"is_alive"` // 0 表示已掉线/不在线，非 0 才算「在线」
}

type guardOnlinePage struct {
	Top3 []guardOnlineMember `json:"top3"`
	List []guardOnlineMember `json:"list"`
	Info struct {
		Page int `json:"page"` // 总页数
		Now  int `json:"now"`  // 当前页
	} `json:"info"`
}

// GuardOnline 统计指定直播间当前在线的大航海人数，按等级分档。
//
// 两条语义原样照搬自原 C++ 项目 getPkOnlineGuardPage
// （bili_liveservice.cpp:1265-1314），少一条都会算错：
//  1. 只数 is_alive != 0 的——「在线」不等于榜单里的全部成员；
//  2. data.top3 只在第一页累加一次，后续翻页如果重复累加会重复计数
//     （list 则每页都要累加，它是真正分页的数据）。
func (c *Client) GuardOnline(ctx context.Context, roomID, uid string) (GuardOnlineCounts, error) {
	var counts GuardOnlineCounts
	page := 1
	// 用独立的请求计数器而不是 page 本身来判断是否超过安全上限：
	// 接口返回的 data.info.now 如果异常卡住不推进，page 会在同一个值上
	// 反复打转，永远摸不到 guardOnlineMaxPages，靠 page 判断会死循环。
	for requests := 0; ; requests++ {
		if err := ctx.Err(); err != nil {
			return counts, err
		}
		if requests >= guardOnlineMaxPages {
			break
		}

		params := url.Values{}
		params.Set("actionKey", "appkey")
		params.Set("appkey", guardOnlineAppKey)
		params.Set("roomid", roomID)
		params.Set("page", strconv.Itoa(page))
		params.Set("ruid", uid)
		params.Set("page_size", guardOnlinePageSize)

		var data guardOnlinePage
		if err := c.GetJSON(ctx, c.URLFor("guardOnline"), params, false, &data); err != nil {
			return counts, fmt.Errorf("获取在线大航海失败(page=%d): %w", page, err)
		}

		if page == 1 {
			addGuardOnlineCounts(&counts, data.Top3)
		}
		addGuardOnlineCounts(&counts, data.List)

		if data.Info.Now >= data.Info.Page || data.Info.Now <= 0 {
			break
		}
		page = data.Info.Now + 1
	}
	return counts, nil
}

func addGuardOnlineCounts(counts *GuardOnlineCounts, members []guardOnlineMember) {
	for _, m := range members {
		if m.IsAlive == 0 {
			continue
		}
		switch m.GuardLevel {
		case 1:
			counts.Governor++
		case 2:
			counts.Admiral++
		default:
			counts.Captain++
		}
	}
}
