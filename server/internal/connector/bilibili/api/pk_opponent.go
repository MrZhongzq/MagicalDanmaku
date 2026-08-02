package api

import (
	"bytes"
	"context"
	"fmt"
	"net/url"
	"strconv"
	"strings"
)

// 以下参数取自原 C++ 项目 bili_liveservice.cpp 里真实调用过的取值，
// 不是随意选的默认值。
const (
	// guardOnlinePageSize 取自 getPkOnlineGuardPageNew2（第 1339/1412 行
	// 附近，最终生效版本用的是 100）。
	guardOnlinePageSize = 100

	// guardOnlineMaxPages 是翻页安全上限，防止接口返回异常的 data.count
	// （服务端给的值，不可信）导致算出一个超大 pageCount、无限请求下去。
	// 原 C++ 实现没有这层保护——这是本次移植额外加的防御，用的是独立的
	// 请求次数计数器，不依赖任何服务端返回值（见 GuardOnline 里的注释）。
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

// GuardTotal 获取指定直播间的大航海总数。单次请求即可拿到，不需要翻页。
//
// 用的是 getGuardCountByRoomId（bili_liveservice.cpp:5137-5171，调用点
// 第 5276 行），不是同样读 guardTab/topList{New} 但写死查自己房间的
// getGuardCount（第 1127-1136 行）——本任务查的是任意房间（对面），
// 只有前者是为「传入任意房间号」这个场景设计并真实调用过的。
// 两者响应结构相同，都读 data.info.num。
func (c *Client) GuardTotal(ctx context.Context, roomID, uid string) (int64, error) {
	params := url.Values{}
	params.Set("roomid", roomID)
	params.Set("page", "1")
	params.Set("ruid", uid)

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
	Governor int64 // 总督（guard_level==1）
	Admiral  int64 // 提督（guard_level==2）
	Captain  int64 // 舰长（guard_level==3，精确匹配，不是「其余」）
}

// Total 返回三档之和，即「在线」的大航海总数。
func (g GuardOnlineCounts) Total() int64 { return g.Governor + g.Admiral + g.Captain }

// guardUID 保存 uid 字段的原始 JSON 字面量（数字去掉引号后长得和数字
// 字符串一样，字符串去掉外层引号后还原成原始内容），只当一个不透明的
// 去重键使用，从不参与算术。
//
// 不用 int64：B 站不同接口对 uid 是数字还是带引号字符串并不统一，硬编码
// int64 遇到字符串会让整页解码失败（GuardOnline 直接 return error，字段
// 降级为 nil——不崩，但白拿不到数据）。但也不能反过来图省事「解析失败
// 就当 0」——如果真这么做，一旦响应里 uid 解析失败，所有成员的 UID 都会
// 塌缩成同一个值 0，seenUIDs[0] 只会命中一次，会把整个房间的大航海错误
// 地数成 1 个：一个看起来合理、实则彻底错误的数字，比「报错后降级」危险
// 得多。保留原始字面量当字符串比较，两种表示形式都不会解码失败，也不会
// 把「解析失败」悄悄等同于任何具体数值。
type guardUID string

func (u *guardUID) UnmarshalJSON(b []byte) error {
	*u = guardUID(bytes.Trim(b, `"`))
	return nil
}

// guardOnlineMember 是 queryContributionRank 接口 data.item[] 数组里
// 一个成员的形状。这个接口本身就是「在线」高能榜（type=online_rank），
// 不带 is_alive 字段，不需要也不能再按 is_alive 过滤一次。
type guardOnlineMember struct {
	UID        guardUID `json:"uid"`
	GuardLevel int      `json:"guard_level"`
}

// flexibleCount 兼容 data.count 可能是数字也可能是带引号字符串两种形式。
// 跟 guardUID 不同，这个字段要真正参与算术（算 pageCount），不能简单地
// 当字符串收着；解析失败时返回 error（连带让 GuardOnline 整页失败、
// 降级为未知），而不是悄悄当 0——0 会被当成「已经翻完了」，把「拿不到
// 总数」误判成「数完了」，产出一个偏小但看着正常的数字。
type flexibleCount int

func (n *flexibleCount) UnmarshalJSON(b []byte) error {
	s := strings.Trim(string(b), `"`)
	v, err := strconv.Atoi(s)
	if err != nil {
		return fmt.Errorf("count 不是合法整数: %q: %w", s, err)
	}
	*n = flexibleCount(v)
	return nil
}

type guardOnlinePage struct {
	Item  []guardOnlineMember `json:"item"`
	Count flexibleCount       `json:"count"` // 总条数，用于算总页数
}

// GuardOnline 统计指定直播间当前在线的大航海人数，按等级分档。
//
// 语义原样照搬自原 C++ 项目**真正生效**的版本 getPkOnlineGuardPageNew2
// （bili_liveservice.cpp:1378-1452）。这不是最初以为的 getPkOnlineGuardPage
// （第 1249-1315 行那套 topList + is_alive + top3 仅 page1 的逻辑）——
// 那个函数第一行就是 `getPkOnlineGuardPageNew2(); return;`，是死代码，
// 后面的实现永远不会跑到。真正调用的是 queryContributionRank 接口，
// 语义完全不同，少一条都会算错：
//  1. 累加对象是 data.item[]，没有 top3/list 之分，不存在「仅 page1」这回事；
//  2. 跳过 guard_level <= 0 的——只数真正的大航海成员；
//  3. 按 uid 去重（原代码用 QSet）——这个接口会在不同页之间重复返回同一个
//     uid，不去重会多算；
//  4. 不看 is_alive——type=online_rank 本身就是「在线」高能榜，返回的
//     已经只有在线的人，这个接口压根不带这个字段；
//  5. 分档是 guard_level==3 精确匹配舰长，不是「非 1/2 就算舰长」，未知
//     等级（如未来 B 站新增档位）不计入任何一档；
//  6. 翻页看 data.count（总条数）算 pageCount = ceil(count/page_size)，
//     page < pageCount 就继续。
func (c *Client) GuardOnline(ctx context.Context, roomID, uid string) (GuardOnlineCounts, error) {
	var counts GuardOnlineCounts
	seenUIDs := make(map[guardUID]bool)
	page := 1
	// 用独立的请求计数器判断是否超过安全上限，不依赖 data.count 算出的
	// pageCount——data.count 是服务端给的，不可信；即便它异常地一直很大，
	// 这里的 page 也是我们自己单调 +1 推进的，不会像「跟着服务端字段走」
	// 那样被卡住，但依然需要一个跟服务端返回值无关的硬上限兜底。
	for requests := 0; ; requests++ {
		if err := ctx.Err(); err != nil {
			return counts, err
		}
		if requests >= guardOnlineMaxPages {
			break
		}

		params := url.Values{}
		params.Set("ruid", uid)
		params.Set("room_id", roomID)
		params.Set("page", strconv.Itoa(page))
		params.Set("page_size", strconv.Itoa(guardOnlinePageSize))
		params.Set("type", "online_rank")
		params.Set("switch", "contribution_rank")
		params.Set("platform", "web")

		var data guardOnlinePage
		if err := c.GetJSON(ctx, c.URLFor("guardOnline"), params, false, &data); err != nil {
			return counts, fmt.Errorf("获取在线大航海失败(page=%d): %w", page, err)
		}

		addGuardOnlineCounts(&counts, seenUIDs, data.Item)

		pageCount := (int(data.Count) + guardOnlinePageSize - 1) / guardOnlinePageSize
		if page >= pageCount {
			break
		}
		page++
	}
	return counts, nil
}

func addGuardOnlineCounts(counts *GuardOnlineCounts, seenUIDs map[guardUID]bool, members []guardOnlineMember) {
	for _, m := range members {
		if m.GuardLevel <= 0 {
			continue
		}
		if seenUIDs[m.UID] {
			continue
		}
		seenUIDs[m.UID] = true

		switch m.GuardLevel {
		case 1:
			counts.Governor++
		case 2:
			counts.Admiral++
		case 3:
			counts.Captain++
			// 其余等级（当前接口不会出现，但防御性地不假设）不计入任何档位。
		}
	}
}
