package store

import (
	"context"
	"fmt"
	"strings"
	"time"
)

// StatsBy 是统计聚合的分组方式。
type StatsBy string

// 两种分组方式。
const (
	StatsByDay     StatsBy = "day"     // 按自然天（UTC）分组
	StatsBySession StatsBy = "session" // 按开播场次分组，见 rawLiveSessions 的说明
)

// StatsQuery 是统计聚合的查询条件。零值字段表示不限制。
//
// 形状特意与 ActivityQuery 对齐（AccountID/BindingID/Since/Until），
// 调用方是同一批 httpapi 处理器。
type StatsQuery struct {
	AccountID int64
	BindingID int64
	Since     time.Time
	Until     time.Time
}

// StatsBucket 是聚合后的一行统计。
//
// GiftCount/GiftKinds 只统计 event_type = "gift"（单次送礼 SEND_GIFT），
// 不含 "gift_combo"：cmdmap/gift.go 里写明 COMBO_SEND 与其对应的多条
// SEND_GIFT 是重复计数关系，两者都算就会让礼物数虚高。GiftKinds 是
// 礼物名去重数，从 detail JSONB 的 GiftName 字段取（event.Gift 没有
// json tag，序列化后键名就是 Go 字段名本身）。
//
// BlindBoxProfit 是这个分桶内全部盲盒礼物的盈亏之和，单位 1/100 电池
// （见 event.BlindBox 的注释：幸运盲盒 50 电池，报文里是 5000）。单行
// 盈亏 = Price*Count - TotalCoin（用户项目记忆里的口径：Price 是爆出
// 礼物的单价，TotalCoin 是这次投喂实际花掉的），直接按每一条盲盒送礼
// 事件的原始电池数量求和，不按礼物名分组——用户明确要求过盈亏必须按
// 电池数量统计，不能按礼物名统计（同一电池消耗量在不同盲盒池可能对应
// 不同的开出礼物，礼物名不是稳定的价值锚点）。「元」的换算留给展示层，
// 这里连同其余金额字段一样存 1/100 电池的原始整数。
type StatsBucket struct {
	// Bucket 按分组方式含义不同：
	//   by=day     "2026-08-01"（UTC 自然天）
	//   by=session 该场次开始时刻的 RFC3339（开始时刻未知时是查询窗口
	//              的 since，或整条时间线上该场次之前那次 live_stop 的
	//              时刻——见 rawLiveSessions 的说明）
	Bucket         string
	DanmakuCount   int64
	EnterCount     int64
	GiftCount      int64
	GiftKinds      int64
	GuardCount     int64
	LiveSeconds    int64
	BlindBoxProfit int64

	// GiftCoins 是这个分桶内全部**非盲盒、金瓜子**礼物的 TotalCoin 之和，
	// 单位 1/100 电池，供 WebUI 统计页「今日电池到账」卡片使用。
	//
	// 两条过滤缺一不可：
	//   - 不含盲盒——与 GiftCount/GiftKinds 一致，P4-4 的硬性要求（盲盒
	//     单独算，见 BlindBoxProfit），不能让盲盒的花费重复计进常规
	//     礼物的电池到账。
	//   - 只算 coin_type=gold（金瓜子）——免费礼物（小花花、人气票等）
	//     是 silver，不产生真实电池，即便它的 TotalCoin 字段本身是正数
	//     （可能承载着人气值一类的展示用数字），也绝不能计进电池到账。
	//     用 `TotalCoin > 0` 之类的近似判据在这类礼物上会判错，必须
	//     老老实实读 CoinType 字段。
	GiftCoins int64
}

// statsWhere 拼公共的 WHERE 片段：kind='event' 恒定（业务事件而非机器人
// 动作——RecordAction 也会把触发它的事件类型写进 event_type 列，不加这个
// 条件会把「因某条弹幕触发的动作」也算进弹幕数），account/binding 按需加。
func statsWhere(accountID, bindingID int64) ([]string, []any) {
	where := []string{"kind = 'event'"}
	var args []any
	add := func(cond string, val any) {
		args = append(args, val)
		where = append(where, fmt.Sprintf(cond, len(args)))
	}
	if accountID != 0 {
		add("account_id = $%d", accountID)
	}
	if bindingID != 0 {
		add("binding_id = $%d", bindingID)
	}
	return where, args
}

// isBlindBoxGiftSQL 判断一条 gift 行是不是盲盒——**必须用 `->>`（取
// 文本），不能用 `->`（取 jsonb）**。event.Gift 序列化后 BlindBox 字段
// 不是盲盒时是 JSON **null**（不是键缺失——没有 json tag、没有
// omitempty，`json.Marshal` 老老实实输出 `"BlindBox":null"`），是盲盒时
// 是一个对象。PostgreSQL 的 `jsonb ->` 对「值是 JSON null」返回的是
// jsonb 的 null（一个非 SQL NULL 的值），`(x -> 'k') IS NOT NULL` 对这种
// 情况会判成真——这是这里真实踩过的一个坑：写成 `->` 时，全部普通礼物
// 都会被误判成盲盒，Price*Count-TotalCoin 会把普通礼物的价格也算进
// 「盲盒盈亏」，数字全错但不报错、看起来还挺正常。`->>`（取文本）在
// JSON null 与键缺失两种情况下都老实返回 SQL NULL，只有真的是对象时
// 才返回非 NULL 文本，两个场景（真实生产序列化的 `"BlindBox":null`、
// 手写测试 JSON 里干脆不写这个键）都能正确处理。
const isBlindBoxGiftSQL = `detail->>'BlindBox' IS NOT NULL`

// isGoldCoinGiftSQL 判断一条 gift 行是不是用金瓜子付的——`->>`（取文本）
// 与 isBlindBoxGiftSQL 是同一个理由：银瓜子免费礼物、老数据没有这个键
// 两种情况都要能正确落到「不是金瓜子」这一侧，不能只处理其中一种。
const isGoldCoinGiftSQL = `detail->>'CoinType' = 'gold'`

// countExprs 是六个业务计数的 SQL 表达式，QueryStatsByDay（GROUP BY）与
// aggregateEventCounts（单行）共用，避免两处口径漂移。
//
// **gift_count/gift_kinds 不含盲盒**——计划文件明文的硬性要求（用户
// 原话「盲盒类单独计算」）：盲盒送礼行不计入礼物件数，爆出的礼物名
// （星光铃铛/棒棒糖/…）也不进 `COUNT(DISTINCT GiftName)`，否则「礼物
// 种类」会被盲盒池的开奖结果污染。
//
// blind_box_profit 只对是盲盒的 gift 行求和，三个数字字段都用 ->> 取成
// 文本再转 bigint：detail 是 event.Gift 整个结构体的原样 JSON，
// Price/Count/TotalCoin 键名与 Go 字段名相同（没有 json tag）。COALESCE
// 是因为一个分桶如果压根没有盲盒行，SUM 会是 SQL NULL，不该让调用方
// 看到「拿不到」——这里统计的是「这个时间段有没有盲盒」，没有就是真的
// 0，不是外部接口失败那种需要区分「未知」的场景（那类区分只用在 PK
// 对面快照上，见 connector/bilibili/opponent_snapshot.go）。
const countExprs = `COUNT(*) FILTER (WHERE event_type = 'danmaku') AS danmaku_count,
	COUNT(*) FILTER (WHERE event_type = 'user_enter') AS enter_count,
	COUNT(*) FILTER (WHERE event_type = 'gift' AND NOT (` + isBlindBoxGiftSQL + `)) AS gift_count,
	COUNT(DISTINCT detail->>'GiftName') FILTER (WHERE event_type = 'gift' AND NOT (` + isBlindBoxGiftSQL + `)) AS gift_kinds,
	COUNT(*) FILTER (WHERE event_type = 'guard_buy') AS guard_count,
	COALESCE(SUM(
		CASE WHEN event_type = 'gift' AND ` + isBlindBoxGiftSQL + `
			THEN (detail->>'Price')::bigint * (detail->>'Count')::bigint - (detail->>'TotalCoin')::bigint
			ELSE 0 END
	), 0) AS blind_box_profit,
	COALESCE(SUM(
		CASE WHEN event_type = 'gift' AND NOT (` + isBlindBoxGiftSQL + `) AND ` + isGoldCoinGiftSQL + `
			THEN (detail->>'TotalCoin')::bigint
			ELSE 0 END
	), 0) AS gift_coins`

// QueryStatsByDay 按 UTC 自然天聚合业务事件计数。
//
// 全部计数在 SQL 里用 GROUP BY + FILTER 算，不把原始行拉到 Go 里累加——
// 一个活跃房间一天几万行，拉全量再在内存里数，跟前端在 500 条上算是
// 同一类错误，只是发生的地方从浏览器换成了服务器。
//
// LiveSeconds 是例外：直播时长要靠 live_start/live_stop 配对，天然不是
// 一句 GROUP BY 能表达的关系（配对是有状态的，SQL 没有原生的「按顺序
// 消费两个事件」）。live_start/live_stop 频率极低（一天几次），把它们
// 单独查出来在 Go 里配对、按天分摊，不违反上面那条「不要整表拉过来」
// 的约束——真正的洪水（弹幕/礼物/进场）从始至终没有离开过 SQL 聚合。
//
// 简化：一场直播若跨了 UTC 零点，全部时长记在开播那一天，不按零点切分。
// 这是刻意的简化而非疏漏——按秒切分需要在 Go 里对每一天做区间求交，
// 复杂度不成比例，而「哪天开的播算哪天」对日维度的统计卡片够用。
func (s *Store) QueryStatsByDay(ctx context.Context, q StatsQuery) ([]StatsBucket, error) {
	where, args := statsWhere(q.AccountID, q.BindingID)
	if !q.Since.IsZero() {
		args = append(args, q.Since)
		where = append(where, fmt.Sprintf("occurred_at >= $%d", len(args)))
	}
	if !q.Until.IsZero() {
		args = append(args, q.Until)
		where = append(where, fmt.Sprintf("occurred_at <= $%d", len(args)))
	}

	sql := `SELECT date_trunc('day', occurred_at) AS bucket, ` + countExprs + `
	        FROM activity_logs WHERE ` + strings.Join(where, " AND ") + `
	        GROUP BY bucket ORDER BY bucket`

	rows, err := s.pool.Query(ctx, sql, args...)
	if err != nil {
		return nil, fmt.Errorf("store: 按天聚合统计失败: %w", err)
	}
	defer rows.Close()

	index := map[string]int{}
	var out []StatsBucket
	for rows.Next() {
		var bucket time.Time
		var b StatsBucket
		if err := rows.Scan(&bucket, &b.DanmakuCount, &b.EnterCount,
			&b.GiftCount, &b.GiftKinds, &b.GuardCount, &b.BlindBoxProfit, &b.GiftCoins); err != nil {
			return nil, fmt.Errorf("store: 读取按天统计失败: %w", err)
		}
		b.Bucket = bucket.UTC().Format("2006-01-02")
		index[b.Bucket] = len(out)
		out = append(out, b)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("store: 按天聚合统计失败: %w", err)
	}

	sessions, err := s.rawLiveSessions(ctx, q.AccountID, q.BindingID)
	if err != nil {
		return nil, err
	}
	now := time.Now()
	for _, sess := range sessions {
		start, end := effectiveSessionBounds(sess, q.Since, q.Until, now)
		if !sessionOverlaps(start, end, q.Since, q.Until) {
			continue
		}
		day := start.UTC().Format("2006-01-02")
		if i, ok := index[day]; ok {
			out[i].LiveSeconds += int64(end.Sub(start).Seconds())
		}
		// 该天在事件计数里没出现（窗口把当天唯一的 live_start/live_stop
		// 之外的证据都裁掉了），选择不新增一行——day 视图的行只代表
		// 「窗口内有业务事件的那些天」，这是已知的边界简化，见任务报告。
	}
	return out, nil
}

// QueryStatsBySession 按开播场次聚合。
//
// 场次配对逻辑见 rawLiveSessions；两种残缺边界（只有 start / 只有 stop）
// 都不会被静默丢弃，行为见 effectiveSessionBounds 的说明。
//
// 每个场次单独发一条聚合查询（aggregateEventCounts，单行 COUNT FILTER，
// 不是拉原始行）。场次数量天然稀疏（一天几次），N 次单行聚合查询的
// 总代价远小于一次全表拉取，且每一次本身仍然是 SQL 侧聚合，不违反
// 「不要拉全量到 Go 里聚合」的约束。
func (s *Store) QueryStatsBySession(ctx context.Context, q StatsQuery) ([]StatsBucket, error) {
	sessions, err := s.rawLiveSessions(ctx, q.AccountID, q.BindingID)
	if err != nil {
		return nil, err
	}

	now := time.Now()
	var out []StatsBucket
	for _, sess := range sessions {
		start, end := effectiveSessionBounds(sess, q.Since, q.Until, now)
		if !sessionOverlaps(start, end, q.Since, q.Until) {
			continue
		}
		if end.Before(start) {
			continue // 保险：正常配对不会出现，但顺序反了记 0 时长没有意义
		}

		danmaku, enter, gift, giftKinds, guard, blindBoxProfit, giftCoins, err := s.aggregateEventCounts(
			ctx, q.AccountID, q.BindingID, start, end)
		if err != nil {
			return nil, err
		}

		out = append(out, StatsBucket{
			Bucket:         start.UTC().Format(time.RFC3339),
			DanmakuCount:   danmaku,
			EnterCount:     enter,
			GiftCount:      gift,
			GiftKinds:      giftKinds,
			GuardCount:     guard,
			LiveSeconds:    int64(end.Sub(start).Seconds()),
			BlindBoxProfit: blindBoxProfit,
			GiftCoins:      giftCoins,
		})
	}
	return out, nil
}

// aggregateEventCounts 在 [since, until] 闭区间上做一次单行 SQL 聚合。
func (s *Store) aggregateEventCounts(ctx context.Context, accountID, bindingID int64, since, until time.Time) (
	danmaku, enter, gift, giftKinds, guard, blindBoxProfit, giftCoins int64, err error) {
	where, args := statsWhere(accountID, bindingID)
	args = append(args, since)
	where = append(where, fmt.Sprintf("occurred_at >= $%d", len(args)))
	args = append(args, until)
	where = append(where, fmt.Sprintf("occurred_at <= $%d", len(args)))

	sql := `SELECT ` + countExprs + ` FROM activity_logs WHERE ` + strings.Join(where, " AND ")
	err = s.pool.QueryRow(ctx, sql, args...).
		Scan(&danmaku, &enter, &gift, &giftKinds, &guard, &blindBoxProfit, &giftCoins)
	if err != nil {
		err = fmt.Errorf("store: 聚合区间统计失败: %w", err)
	}
	return
}

// liveSession 是从 activity_logs 里配对出的一场直播，配对前的原始形态。
//
// Start/End 为 nil 表示这一头没有配上：
//   - Start == nil：只见到 live_stop，没见到对应的 live_start
//     （数据的最开头就是一场直播中途，或者漏记了开播事件）
//   - End == nil：只见到 live_start，没见到对应的 live_stop
//     （还在直播中，或者漏记了下播事件）
type liveSession struct {
	Start *time.Time
	End   *time.Time
}

// rawLiveSessions 把某个账号/绑定下的 live_start/live_stop 事件按时间
// 顺序配对成场次。
//
// 故意不按 since/until 过滤：配对必须在完整时间线上做，查询窗口只用来
// 决定「裁剪之后落在窗口内的场次要不要返回」（effectiveSessionBounds +
// sessionOverlaps），而不是在配对之前就把跨窗口的另一半事件切掉——
// 那样会把「只有 stop 没有 start」的边界从「本来有配对，只是配对的另
// 一半在窗口外」错误地降级成「完全没配对」。
//
// live_start/live_stop 一天最多几次，这里没有走 GROUP BY 而是拉原始行
// 是合理的——它不是本任务要守住的那条「不要整表拉到 Go 里聚合」的边界，
// 那条边界针对的是弹幕/礼物/进场这类洪水数据。
//
// 配对算法：顺序扫描，遇到 live_start 记为待配对的开始；遇到
// live_stop，若有待配对的开始就配成一场，否则单独记一场只有
// End 的场次；扫描结束后若还有待配对的开始，单独记一场只有
// Start 的场次。连续两个 live_start 之间没有夹 live_stop 视为
// 漏记下播——前一次开播单独收作只有 Start 的场次，从新的
// live_start 起重新计。
func (s *Store) rawLiveSessions(ctx context.Context, accountID, bindingID int64) ([]liveSession, error) {
	where := []string{"kind = 'event'", "event_type IN ('live_start', 'live_stop')"}
	var args []any
	add := func(cond string, val any) {
		args = append(args, val)
		where = append(where, fmt.Sprintf(cond, len(args)))
	}
	if accountID != 0 {
		add("account_id = $%d", accountID)
	}
	if bindingID != 0 {
		add("binding_id = $%d", bindingID)
	}

	sql := `SELECT event_type, occurred_at FROM activity_logs
	        WHERE ` + strings.Join(where, " AND ") + `
	        ORDER BY occurred_at ASC, id ASC`

	rows, err := s.pool.Query(ctx, sql, args...)
	if err != nil {
		return nil, fmt.Errorf("store: 查询开播/下播事件失败: %w", err)
	}
	defer rows.Close()

	var sessions []liveSession
	var openStart *time.Time
	for rows.Next() {
		var eventType string
		var at time.Time
		if err := rows.Scan(&eventType, &at); err != nil {
			return nil, fmt.Errorf("store: 读取开播/下播事件失败: %w", err)
		}
		t := at
		switch eventType {
		case "live_start":
			if openStart != nil {
				// 漏记了上一场的下播：单独收一场只有 Start 的场次
				sessions = append(sessions, liveSession{Start: openStart})
			}
			openStart = &t
		case "live_stop":
			if openStart != nil {
				sessions = append(sessions, liveSession{Start: openStart, End: &t})
				openStart = nil
			} else {
				sessions = append(sessions, liveSession{End: &t})
			}
		}
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("store: 查询开播/下播事件失败: %w", err)
	}
	if openStart != nil {
		sessions = append(sessions, liveSession{Start: openStart})
	}
	return sessions, nil
}

// effectiveSessionBounds 把一个可能残缺的 liveSession 换算成具体的
// [start, end) 时刻，供计数查询与时长计算使用。
//
//   - Start 已知：直接用。
//   - Start 未知（只有 live_stop）：查询区间从这场直播中间切开——用
//     since 兜底当作「这场从窗口一开始就在播」；since 也没给的话，没
//     有任何依据能推断开播时刻，退化为用 End 本身兜底（时长记 0，但
//     这场依然会被返回，不静默丢弃——否则用户会看到「那天没直播」而
//     实际直播过）。
//   - End 已知：直接用。
//   - End 未知（只有 live_start，还在播或漏了下播）：用 until 兜底
//     当作「查到这里为止」；until 没给就用 now 兜底，当作「播到现在」。
func effectiveSessionBounds(sess liveSession, since, until, now time.Time) (start, end time.Time) {
	switch {
	case sess.Start != nil:
		start = *sess.Start
	case !since.IsZero():
		start = since
	case sess.End != nil:
		start = *sess.End
	}

	switch {
	case sess.End != nil:
		end = *sess.End
	case !until.IsZero():
		end = until
	default:
		end = now
	}
	return start, end
}

// sessionOverlaps 判断 [start, end] 是否与查询窗口 [since, until] 有交集。
// since/until 为零值表示该侧不限制。
func sessionOverlaps(start, end, since, until time.Time) bool {
	if !since.IsZero() && end.Before(since) {
		return false
	}
	if !until.IsZero() && start.After(until) {
		return false
	}
	return true
}

// GiftBreakdownRow 是「礼物」明细列表的一行：礼物名 + 数量 + 电池数
// 加和，供 WebUI 统计页的礼物明细表使用。
type GiftBreakdownRow struct {
	GiftName string
	// Count 是这个礼物名下全部送礼事件的 Count 字段之和（送礼数量本身，
	// 不是行数——一行可能一次性送出多个，比如 num=10）。
	Count int64
	// Coins 只累加 coin_type=gold（金瓜子）那部分 TotalCoin，单位
	// 1/100 电池——免费礼物（coin_type=silver）的数量照常统计进
	// Count，但不产生电池，Coins 里那部分贡献恒为 0。
	Coins int64
}

// QueryGiftBreakdown 按礼物名分组统计 bindingID 名下的礼物明细，
// since/until 为零值表示不限制该侧。
//
// 与 GiftCount/GiftKinds/GiftCoins 一致地排除盲盒（P4-4 硬性要求：
// 盲盒继续单独算，不混进常规礼物统计）——盲盒爆出的礼物名不该出现在
// 这张明细表里。
func (s *Store) QueryGiftBreakdown(ctx context.Context, bindingID int64, since, until time.Time) ([]GiftBreakdownRow, error) {
	where := []string{"kind = 'event'", "event_type = 'gift'", "binding_id = $1", "NOT (" + isBlindBoxGiftSQL + ")"}
	args := []any{bindingID}
	if !since.IsZero() {
		args = append(args, since)
		where = append(where, fmt.Sprintf("occurred_at >= $%d", len(args)))
	}
	if !until.IsZero() {
		args = append(args, until)
		where = append(where, fmt.Sprintf("occurred_at <= $%d", len(args)))
	}

	sql := `SELECT detail->>'GiftName' AS gift_name,
		COALESCE(SUM((detail->>'Count')::bigint), 0) AS count,
		COALESCE(SUM(CASE WHEN ` + isGoldCoinGiftSQL + ` THEN (detail->>'TotalCoin')::bigint ELSE 0 END), 0) AS coins
		FROM activity_logs WHERE ` + strings.Join(where, " AND ") + `
		GROUP BY gift_name ORDER BY coins DESC, gift_name`

	rows, err := s.pool.Query(ctx, sql, args...)
	if err != nil {
		return nil, fmt.Errorf("store: 查询礼物明细失败: %w", err)
	}
	defer rows.Close()

	var out []GiftBreakdownRow
	for rows.Next() {
		var r GiftBreakdownRow
		if err := rows.Scan(&r.GiftName, &r.Count, &r.Coins); err != nil {
			return nil, fmt.Errorf("store: 读取礼物明细失败: %w", err)
		}
		out = append(out, r)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("store: 查询礼物明细失败: %w", err)
	}
	return out, nil
}
