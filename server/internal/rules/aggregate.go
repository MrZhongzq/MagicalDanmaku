package rules

import (
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/MrZhongzq/MagicalDanmaku/server/internal/event"
)

// Aggregator 在时间窗口内缓冲事件，到期后去重、合并并产出 Trigger。
//
// 处理分两步，顺序不可颠倒：
//
//	第一步  按 UID 逐字段合并（总是执行）
//	第二步  按 spec.By 分组产出 Trigger
//
// 第一步与分组方式无关，正是它解决了 P0 联调发现的进场重复问题：
// ENTRY_EFFECT 只有 UID 没有昵称，INTERACT_WORD_V2 信息完整，
// 两条合并后得到一条完整记录。
type Aggregator struct {
	spec AggregateSpec
	out  func(Trigger)

	mu       sync.Mutex
	buckets  map[string]*bucket // 键：事件类型 + UID（+ 礼物名）
	deadline *time.Timer        // 本轮窗口计时器，从本轮首个事件起只设一次
	closed   bool
}

// bucket 是同一 UID 同一类型的事件累积。
type bucket struct {
	typ    event.Type
	uid    string
	events []event.Event
	vars   map[string]any
	seq    int // 首次出现序号，用于保持 users 数组的顺序
}

// NewAggregator 创建合并器。out 在窗口到期时被调用，可能被调用多次
// （每个分组一次）。
func NewAggregator(spec AggregateSpec, out func(Trigger)) *Aggregator {
	return &Aggregator{
		spec:    spec,
		out:     out,
		buckets: make(map[string]*bucket),
	}
}

// Add 把事件投入当前窗口。
//
// 窗口固定从本轮首个事件起算：只在首个事件到达时设一次定时器，
// Window 时长后到点即结算，之后来的事件不会推迟结算。这与旧版
// 「静默计时」不同——旧版每来一个事件都把定时器往后推（Reset），
// 导致连续送礼时窗口被无限延长、迟迟不结算（例如连送 9 个盲盒
// 只会合并出 1 条答谢，而不是按 10 秒一轮切成多条）。
//
// 窗口关闭后缓冲区被清空，下一个到达的事件会重新起算一轮新窗口，
// 而不是落回某个固定的时间网格。
func (a *Aggregator) Add(ev event.Event) {
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.closed {
		return
	}

	uid := uidOf(ev)
	key := bucketKey(ev, uid)

	b, ok := a.buckets[key]
	if !ok {
		b = &bucket{typ: ev.Type, uid: uid, vars: VarsFromEvent(ev), seq: len(a.buckets)}
		b.events = append(b.events, ev)
		a.buckets[key] = b
	} else {
		b.events = append(b.events, ev)
		// 第一步：逐字段合并，空值不覆盖非空值
		MergeVars(b.vars, VarsFromEvent(ev))
		accumulateGift(b.vars, ev)
	}

	// 窗口计时器只在本轮首个事件到达时设一次，后续事件不重置它。
	if a.deadline == nil {
		a.deadline = time.AfterFunc(a.spec.Window, a.onTimeout)
	}
}

// stopTimersLocked 停掉窗口计时器。调用者需持有锁。
func (a *Aggregator) stopTimersLocked() {
	if a.deadline != nil {
		a.deadline.Stop()
		a.deadline = nil
	}
}

// onTimeout 是窗口到期回调。
func (a *Aggregator) onTimeout() {
	a.mu.Lock()
	triggers := a.drainLocked()
	a.mu.Unlock()

	for _, tr := range triggers {
		a.out(tr)
	}
}

// Flush 立即结算当前窗口。
func (a *Aggregator) Flush() {
	a.mu.Lock()
	a.stopTimersLocked()
	triggers := a.drainLocked()
	a.mu.Unlock()

	for _, tr := range triggers {
		a.out(tr)
	}
}

// Close 停止计时器并结算未决窗口。
func (a *Aggregator) Close() {
	a.mu.Lock()
	if a.closed {
		a.mu.Unlock()
		return
	}
	a.closed = true
	a.stopTimersLocked()
	triggers := a.drainLocked()
	a.mu.Unlock()

	for _, tr := range triggers {
		a.out(tr)
	}
}

// drainLocked 清空缓冲区，先按 Solo 摘出该单独结算的用户，剩下的再按
// By 分组产出 Trigger。调用者需持有锁。
func (a *Aggregator) drainLocked() []Trigger {
	if len(a.buckets) == 0 {
		a.stopTimersLocked()
		return nil
	}

	buckets := make([]*bucket, 0, len(a.buckets))
	for _, b := range a.buckets {
		buckets = append(buckets, b)
	}
	a.buckets = make(map[string]*bucket)
	a.stopTimersLocked()

	// 按首次出现顺序排序，保证 users/solo 分组顺序稳定可预测
	sortBucketsBySeq(buckets)

	solo, rest := a.splitSoloLocked(buckets)

	// 结算顺序：先出单人优先的，剩下的零散礼物再合并——用户原话「单人的
	// 礼物累加逻辑要比多人多礼物优先级高」。
	out := groupSoloByUser(solo)
	out = append(out, a.groupRest(rest)...)
	return out
}

// splitSoloLocked 把窗口内的桶拆成两拨：件数（不含盲盒）达到
// spec.Solo.MinItems 阈值的用户的全部桶（solo，稍后跨礼物合并成一条），
// 与其余桶（rest，走 spec.By 原有分组）。Solo 为 nil 或 MinItems<=0
// 时不拆分，全部落入 rest，等价于旧版行为。
//
// 盲盒桶不计入件数统计、也不会被划进 solo——盲盒必须继续按「送礼人+
// 盲盒类型」单独结算盈亏（P4-4 硬性要求），让它参与这里的件数阈值判定
// 会把它的数量算进常规礼物统计，且丢失盲盒专属的盈亏字段。正常情况下
// 盲盒会被前端拼的 when 条件挡在这个 Aggregator 之外（buildGiftRule 的
// gift.isBlindBox==false 排除条件），这里的排除是双重保险：哪怕规则
// 配置疏漏让盲盒事件混进了这个 Aggregator，也不会污染单人阈值判定或被
// 卷进某个凑巧同 UID 达标用户的合并结果里。
func (a *Aggregator) splitSoloLocked(buckets []*bucket) (solo, rest []*bucket) {
	if a.spec.Solo == nil || a.spec.Solo.MinItems <= 0 {
		return nil, buckets
	}

	totals := make(map[string]int64, len(buckets))
	for _, b := range buckets {
		if isBlindBoxBucket(b) {
			continue
		}
		totals[b.uid] += giftCountOf(b)
	}

	qualifying := make(map[string]bool, len(totals))
	for uid, total := range totals {
		if total >= int64(a.spec.Solo.MinItems) {
			qualifying[uid] = true
		}
	}

	for _, b := range buckets {
		if !isBlindBoxBucket(b) && qualifying[b.uid] {
			solo = append(solo, b)
		} else {
			rest = append(rest, b)
		}
	}
	return solo, rest
}

// groupSoloByUser 把 solo 桶按 uid 分组，每个用户合并成一条 Trigger——
// 跨礼物合并统计，是用户裁决「单人累加可以跨礼物合并」在这里唯一的
// 落地点：分组键只用 uid，不像 AggregateByGift 那样再带礼物名。
func groupSoloByUser(solo []*bucket) []Trigger {
	if len(solo) == 0 {
		return nil
	}
	groups := make(map[string][]*bucket, len(solo))
	var order []string
	for _, b := range solo {
		if _, ok := groups[b.uid]; !ok {
			order = append(order, b.uid)
		}
		groups[b.uid] = append(groups[b.uid], b)
	}
	out := make([]Trigger, 0, len(order))
	for _, uid := range order {
		out = append(out, mergeBuckets(groups[uid]))
	}
	return out
}

// groupRest 把（未被 Solo 摘走的）桶按 spec.By 分组产出 Trigger。
// 这是旧版 drainLocked 第二步的原样迁移，Solo 为 nil 时输入即是全部
// 桶，行为与引入双轨逻辑之前完全一致。
func (a *Aggregator) groupRest(buckets []*bucket) []Trigger {
	if len(buckets) == 0 {
		return nil
	}

	groups := make(map[string][]*bucket)
	var order []string
	for _, b := range buckets {
		g := a.groupKey(b)
		if _, ok := groups[g]; !ok {
			order = append(order, g)
		}
		groups[g] = append(groups[g], b)
	}

	// MinCount 未达标时不合并，每个条目各出一条 Trigger。
	// 「一波人进场合并欢迎」只在人确实多的时候才有意义，
	// 单独一个人被说成「欢迎 某某」比合并句式自然。
	if a.spec.MinCount > 1 && len(buckets) < a.spec.MinCount {
		out := make([]Trigger, 0, len(buckets))
		for _, b := range buckets {
			out = append(out, mergeBuckets([]*bucket{b}))
		}
		return out
	}

	out := make([]Trigger, 0, len(order))
	for _, g := range order {
		out = append(out, mergeBuckets(groups[g]))
	}
	return out
}

// isBlindBoxBucket 判断桶是否属于盲盒开出的礼物——splitSoloLocked 靠它
// 把盲盒排除在单人优先的件数统计与分组之外。
func isBlindBoxBucket(b *bucket) bool {
	name, ok := LookupPath(b.vars, "gift.blindBox.name")
	if !ok {
		return false
	}
	return toString(name) != ""
}

// giftCountOf 取出桶累积的礼物件数，非礼物桶（如进场事件）返回 0——
// 单人优先本就只对礼物事件有意义，其它事件类型的桶永远凑不满阈值。
func giftCountOf(b *bucket) int64 {
	v, ok := LookupPath(b.vars, "gift.count")
	if !ok {
		return 0
	}
	return toInt64(v)
}

// groupKey 按 spec.By 计算分组键。
func (a *Aggregator) groupKey(b *bucket) string {
	switch a.spec.By {
	case AggregateByUser:
		return string(b.typ) + "\x00" + b.uid
	case AggregateByGift:
		name, _ := LookupPath(b.vars, "gift.name")
		return string(b.typ) + "\x00" + b.uid + "\x00" + toString(name)
	case AggregateByBlindBox:
		// 按盲盒名称分组，而不是按爆出的结果礼物名（那是 AggregateByGift
		// 的键）。同一个盲盒可能爆出不同的礼物（星光铃铛/梦雾纸签/…），
		// 这些结果礼物在第一步 bucketKey 已经被分到了不同的桶里；这里
		// 必须用盲盒名把它们重新收拢到同一组，否则「幸运盲盒」和「心动
		// 盲盒」交叉送时会被结果礼物名切得七零八落，且盈亏无法按盲盒
		// 类型分别结算（用户原话：「心动和幸运交叉送也要分开统计盈亏」）。
		name, _ := LookupPath(b.vars, "gift.blindBox.name")
		return string(b.typ) + "\x00" + b.uid + "\x00" + toString(name)
	default: // AggregateByType
		return string(b.typ)
	}
}

// mergeBuckets 把同组的多个 bucket 合成一个 Trigger。
func mergeBuckets(bs []*bucket) Trigger {
	first := bs[0]
	vars := make(map[string]any, len(first.vars)+2)
	MergeVars(vars, first.vars)

	events := make([]event.Event, 0, len(bs))
	users := make([]string, 0, len(bs))
	seenUser := make(map[string]bool, len(bs))
	gifts := make([]string, 0, len(bs))
	seenGift := make(map[string]bool, len(bs))

	// 盲盒盈亏结算：投入取 gift.totalCoin（用户实际花的），产出取
	// gift.price × gift.count（爆出礼物的价值 × 次数）。同一分组内
	// （AggregateByBlindBox 按盲盒名分组）可能混有多种不同的爆出结果
	// 礼物，它们在第一步已经被 bucketKey 分到了不同的 bucket，所以这里
	// 必须逐个 bucket 累加，不能只取 first 的一份。
	var blindBoxName string
	var blindBoxCount, blindBoxCost, blindBoxGain int64
	hasBlindBox := false

	for _, b := range bs {
		events = append(events, b.events...)
		if b != first {
			MergeVars(vars, b.vars)
		}
		if name, ok := LookupPath(b.vars, "user.username"); ok {
			if s := toString(name); s != "" && !seenUser[s] {
				seenUser[s] = true
				users = append(users, s)
			}
		}
		if name, ok := LookupPath(b.vars, "gift.name"); ok {
			if s := toString(name); s != "" && !seenGift[s] {
				seenGift[s] = true
				gifts = append(gifts, s)
			}
		}
		if name, ok := LookupPath(b.vars, "gift.blindBox.name"); ok {
			if s := toString(name); s != "" {
				hasBlindBox = true
				if blindBoxName == "" {
					blindBoxName = s
				}
				count, _ := LookupPath(b.vars, "gift.count")
				price, _ := LookupPath(b.vars, "gift.price")
				totalCoin, _ := LookupPath(b.vars, "gift.totalCoin")
				blindBoxCount += toInt64(count)
				blindBoxGain += toInt64(price) * toInt64(count)
				blindBoxCost += toInt64(totalCoin)
			}
		}
	}

	vars["count"] = len(bs)
	vars["users"] = users
	vars["gifts"] = gifts

	if hasBlindBox {
		profit := blindBoxGain - blindBoxCost
		absProfit := profit
		if absProfit < 0 {
			absProfit = -absProfit
		}
		vars["blindBox"] = map[string]any{
			"name":       blindBoxName,
			"count":      blindBoxCount,
			"cost":       blindBoxCost,
			"gain":       blindBoxGain,
			"profit":     profit,
			"costYuan":   formatYuan(blindBoxCost),
			"gainYuan":   formatYuan(blindBoxGain),
			"profitYuan": formatYuan(profit),
			// profitAbsYuan 是绝对值版本，专供播报模板避免「亏了 -11 元」
			// 这种双重否定用——终审 Important-2 抓到的真实缺陷：旧模板
			// `{{if gt .blindBox.profit 0}}赚了{{else}}亏了{{end}}
			// {{.blindBox.profitYuan}} 元！` 在亏损分支直接拼 profitYuan
			// （自带负号），"亏了" 和 "-11" 两个否定叠在一起。profitYuan
			// 本身不能改成无符号——它是独立展示字段，aggregate_test.go
			// 已经钉死了它的负号语义，其它消费方（如果将来有）不该被这里
			// 的模板措辞问题连带改变行为，所以新增一个专门给模板用的
			// 绝对值字段，而不是改 profitYuan 本身。
			"profitAbsYuan": formatYuan(absProfit),
		}
	}

	return Trigger{Type: first.typ, Events: events, Vars: vars}
}

// toInt64 把 LookupPath 取回的任意数值类型转成 int64，取不到时返回 0。
func toInt64(v any) int64 {
	f, ok := toFloat(v)
	if !ok {
		return 0
	}
	return int64(f)
}

// formatYuan 把 1/1000 元的原始整数值格式化成「元」的展示字符串，
// 如 5000 → "5"，5200 → "5.2"，-11000 → "-11"。
//
// 用字符串而不是 float64：{{blindBox.profitYuan}} 在模板里直接就是
// "-4.1" 这种能读的数字；float64 存小数在模板渲染时可能出现
// "-4.099999999999999" 这种二进制浮点误差导致的展示问题。存储与
// 计算全程只用整数（1/100 电池），只在这里生成展示字符串时才做除法，
// 且用字符串拼接而非浮点除法，不引入任何浮点运算。
func formatYuan(raw int64) string {
	neg := raw < 0
	if neg {
		raw = -raw
	}
	whole := raw / 1000
	frac := raw % 1000
	s := strconv.FormatInt(whole, 10)
	if frac != 0 {
		fracStr := strings.TrimRight(fmt.Sprintf("%03d", frac), "0")
		s += "." + fracStr
	}
	if neg {
		s = "-" + s
	}
	return s
}

// accumulateGift 累加礼物数量与价值。
//
// MergeVars 的语义是「空值不覆盖非空值」，对计数字段不适用——
// 连击的两条礼物应当相加而非取其一。
func accumulateGift(dst map[string]any, ev event.Event) {
	g, ok := ev.Payload.(event.Gift)
	if !ok {
		return
	}
	gm, ok := dst["gift"].(map[string]any)
	if !ok {
		return
	}
	if cur, ok := gm["count"].(int64); ok {
		gm["count"] = cur + g.Count
	}
	if cur, ok := gm["totalCoin"].(int64); ok {
		gm["totalCoin"] = cur + g.TotalCoin
	}
}

// bucketKey 计算第一步合并的分桶键，即「同一用户的同一件事」。
//
// 仅按 类型+UID 分桶是不够的：同一用户在窗口内送出的不同礼物
// （小花花与辣条）会被合进一个桶，数量被错误地混加。因此礼物类
// 事件的桶键必须带上礼物名。
func bucketKey(ev event.Event, uid string) string {
	key := string(ev.Type) + "\x00" + uid

	switch p := ev.Payload.(type) {
	case event.Gift:
		return key + "\x00" + p.GiftName
	case event.GiftCombo:
		return key + "\x00" + p.GiftName
	default:
		return key
	}
}

// uidOf 提取事件的用户 UID，无用户的事件返回空串。
func uidOf(ev event.Event) string {
	v := VarsFromEvent(ev)
	uid, _ := LookupPath(v, "user.uid")
	return toString(uid)
}

// sortBucketsBySeq 按首次出现序号排序。
func sortBucketsBySeq(bs []*bucket) {
	for i := 1; i < len(bs); i++ {
		for j := i; j > 0 && bs[j].seq < bs[j-1].seq; j-- {
			bs[j], bs[j-1] = bs[j-1], bs[j]
		}
	}
}

// PassthroughTrigger 为未配置合并的规则构造直通 Trigger。
//
// 同样填上 count 与 users，让模板对合并与非合并事件可以统一写法。
func PassthroughTrigger(ev event.Event) Trigger {
	vars := VarsFromEvent(ev)
	vars["count"] = 1

	users := []string{}
	if name, ok := LookupPath(vars, "user.username"); ok {
		if s := toString(name); s != "" {
			users = append(users, s)
		}
	}
	vars["users"] = users

	// 同样填上 gifts：非合并触发只有一个事件，若是礼物事件就是那一个
	// 礼物名，否则（如弹幕）保持空数组——与 users 在无用户名时的处理一致，
	// 使模板在合并与非合并场景下写法统一。
	gifts := []string{}
	if name, ok := LookupPath(vars, "gift.name"); ok {
		if s := toString(name); s != "" {
			gifts = append(gifts, s)
		}
	}
	vars["gifts"] = gifts

	return Trigger{Type: ev.Type, Events: []event.Event{ev}, Vars: vars}
}
