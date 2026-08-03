package httpapi

import (
	"net/http"

	"github.com/MrZhongzq/MagicalDanmaku/server/internal/event"
	"github.com/MrZhongzq/MagicalDanmaku/server/internal/perm"
	"github.com/MrZhongzq/MagicalDanmaku/server/internal/rules"
)

// metaItem 是一个「值 + 中文说明」的枚举项。
//
// 这些清单从后端下发而不是在前端写死：写死会在后端新增事件类型时
// 悄悄漂移，而漂移的表现是「配了规则却不生效」，很难查。
type metaItem struct {
	Value string `json:"value"`
	Label string `json:"label"`
}

// permissionLabels 是权限点的中文说明。
//
// 与 perm.All() 一起使用：说明缺失时用值本身兜底，
// 这样新增权限点忘了加说明也不会让前端渲染不出那一项。
var permissionLabels = map[perm.Permission]string{
	perm.RuleRead:     "查看规则",
	perm.RuleWrite:    "增删改规则、启停规则",
	perm.DanmakuSend:  "手动发送弹幕",
	perm.UserBlock:    "禁言与解禁，含维护禁言名单",
	perm.MemberManage: "授权他人、撤销授权",
	perm.EventRead:    "查看事件流与历史业务日志",
}

// eventTypeLabels 是事件类型的中文说明，顺序即前端下拉框的顺序：
// 常用的排前面。
var eventTypeLabels = []struct {
	Type  event.Type
	Label string
}{
	{event.TypeDanmaku, "弹幕"},
	{event.TypeGift, "礼物"},
	{event.TypeGiftCombo, "礼物连击"},
	{event.TypeGuardBuy, "上舰"},
	{event.TypeSuperChat, "醒目留言"},
	{event.TypeSuperChatDelete, "醒目留言撤销"},
	{event.TypeUserEnter, "进入直播间"},
	{event.TypeUserFollow, "关注"},
	{event.TypeUserShare, "分享"},
	{event.TypeUserLike, "点赞"},
	{event.TypeUserBlocked, "被禁言"},
	{event.TypeLiveStart, "开播"},
	{event.TypeLiveStop, "下播"},
	{event.TypeRoomChange, "房间信息变更"},
	{event.TypeOnlineRankUpdate, "高能榜更新"},
	{event.TypeRoomStatsUpdate, "房间统计更新"},
	{event.TypeBattle, "PK 大乱斗"},
	{event.TypeVisitFromOpponent, "PK 串门：对面来访（欢迎）"},
	{event.TypeVisitToOpponent, "PK 串门：我方观众去对面（警示）"},
	{event.TypeManual, "手动操作"},
	{event.TypeUnknown, "未识别事件"},
}

// actionTypeLabelText 是动作类型的中文说明，与 rules.AllActionTypes()
// 一起使用：说明缺失时用值本身兜底，新增动作类型忘了写说明也不会让
// 前端渲染不出那一项（同 permissionLabels 的写法）。
var actionTypeLabelText = map[rules.ActionType]string{
	rules.ActionDanmaku: "发送弹幕",
	rules.ActionBlock:   "禁言",
	rules.ActionScript:  "执行脚本",
	rules.ActionLog:     "只记日志（调试规则用）",
}

// operatorLabels 是条件操作符的中文说明。
var operatorLabels = []metaItem{
	{"eq", "等于"},
	{"ne", "不等于"},
	{"gt", "大于"},
	{"gte", "大于等于"},
	{"lt", "小于"},
	{"lte", "小于等于"},
	{"contains", "包含"},
	{"prefix", "以……开头"},
	{"suffix", "以……结尾"},
	{"regex", "匹配正则"},
	{"in", "属于列表之一"},
}

// aggregateByLabelText 是合并窗口分组方式的中文说明，与
// rules.AllAggregateBy() 一起使用。
//
// 【终审 Important-1】这里曾经是一份独立于 rules.AggregateBy 常量的
// 手抄清单（`[]metaItem{...}` 字面量），Task 3 新增 AggregateByBlindBox
// 时只改了 rule.go 的 const 块，这份清单没有同步——盲盒聚合本身在后端
// 完全可用（示例配置也在用），但自定义规则页的「分组方式」下拉框里
// 选不出「盲盒」，用户在 UI 上配不出这条规则。改成从
// rules.AllAggregateBy() 生成，说明缺失时用值本身兜底（同
// permissionLabels 的写法），避免同一类漏登记再发生第二次。
var aggregateByLabelText = map[rules.AggregateBy]string{
	rules.AggregateByType:     "按事件类型：窗口内全部合成一条",
	rules.AggregateByUser:     "按类型 + 用户：仅去重不聚合",
	rules.AggregateByGift:     "按类型 + 用户 + 礼物：数量累加",
	rules.AggregateByBlindBox: "按类型 + 用户 + 盲盒名称：盲盒单独聚合、结算盈亏",
}

func (s *Server) handleMetaPermissions(w http.ResponseWriter, _ *http.Request) {
	out := make([]metaItem, 0, len(perm.All()))
	for _, p := range perm.All() {
		label := permissionLabels[p]
		if label == "" {
			label = string(p) // 兜底：新增权限点忘了加说明也不该让前端渲染不出来
		}
		out = append(out, metaItem{Value: string(p), Label: label})
	}
	respondJSON(w, http.StatusOK, out)
}

func (s *Server) handleMetaEventTypes(w http.ResponseWriter, _ *http.Request) {
	out := make([]metaItem, 0, len(eventTypeLabels))
	for _, it := range eventTypeLabels {
		out = append(out, metaItem{Value: string(it.Type), Label: it.Label})
	}
	respondJSON(w, http.StatusOK, out)
}

func (s *Server) handleMetaActionTypes(w http.ResponseWriter, _ *http.Request) {
	all := rules.AllActionTypes()
	out := make([]metaItem, 0, len(all))
	for _, t := range all {
		label := actionTypeLabelText[t]
		if label == "" {
			label = string(t) // 兜底：新增动作类型忘了加说明也不该让前端渲染不出来
		}
		out = append(out, metaItem{Value: string(t), Label: label})
	}
	respondJSON(w, http.StatusOK, out)
}

func (s *Server) handleMetaOperators(w http.ResponseWriter, _ *http.Request) {
	respondJSON(w, http.StatusOK, operatorLabels)
}

func (s *Server) handleMetaAggregateBy(w http.ResponseWriter, _ *http.Request) {
	all := rules.AllAggregateBy()
	out := make([]metaItem, 0, len(all))
	for _, by := range all {
		label := aggregateByLabelText[by]
		if label == "" {
			label = string(by) // 兜底：新增分组方式忘了加说明也不该让前端渲染不出来
		}
		out = append(out, metaItem{Value: string(by), Label: label})
	}
	respondJSON(w, http.StatusOK, out)
}

// variablesResponse 是 /api/meta/variables 的响应结构：公共变量 + 按
// 事件类型分组的变量。
type variablesResponse struct {
	Common  []rules.Variable                `json:"common"`
	ByEvent map[event.Type][]rules.Variable `json:"byEvent"`
}

// handleMetaVariables 下发条件构建器/模板编辑器要用的变量清单。
//
// 清单本身不在这里定义——它由 rules.VariableCatalog() 与
// rules.VarsFromEvent() 一起维护（同一个文件 vars.go），这里只是原样
// 转发。这个接口存在的唯一价值就是让前端不用再自己抄一份：抄来的那份
// （ConditionTree.vue 的 COMMON_FIELD_OPTIONS）是本任务要消灭的第二处
// 定义，改这个处理器时不要在这里另起一份清单，否则等于把第二处定义
// 从前端搬到了后端，价值归零。
func (s *Server) handleMetaVariables(w http.ResponseWriter, _ *http.Request) {
	common, byEvent := rules.VariableCatalog()
	respondJSON(w, http.StatusOK, variablesResponse{Common: common, ByEvent: byEvent})
}
