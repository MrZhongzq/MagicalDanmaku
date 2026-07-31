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
	perm.RuleRead:      "查看规则",
	perm.RuleWrite:     "增删改规则、启停规则",
	perm.DanmakuSend:   "手动发送弹幕",
	perm.UserBlock:     "禁言与解禁，含维护禁言名单",
	perm.AccountManage: "修改 Cookie 与账号参数",
	perm.MemberManage:  "授权他人、撤销授权",
	perm.EventRead:     "查看事件流与历史业务日志",
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
	{event.TypeUnknown, "未识别事件"},
}

// actionTypeLabels 是动作类型的中文说明。
var actionTypeLabels = []struct {
	Type  rules.ActionType
	Label string
}{
	{rules.ActionDanmaku, "发送弹幕"},
	{rules.ActionBlock, "禁言"},
	{rules.ActionScript, "执行脚本"},
	{rules.ActionLog, "只记日志（调试规则用）"},
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

// aggregateByLabels 是合并窗口分组方式的中文说明。
//
// 用 rules.AggregateByType/User/Gift 常量而不是字符串字面量：
// 紧邻的 actionTypeLabels 已经这么做了，字面量在这里只是把
// 「写死会悄悄漂移」的问题从前端搬到了后端。
var aggregateByLabels = []metaItem{
	{string(rules.AggregateByType), "按事件类型：窗口内全部合成一条"},
	{string(rules.AggregateByUser), "按类型 + 用户：仅去重不聚合"},
	{string(rules.AggregateByGift), "按类型 + 用户 + 礼物：数量累加"},
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
	out := make([]metaItem, 0, len(actionTypeLabels))
	for _, it := range actionTypeLabels {
		out = append(out, metaItem{Value: string(it.Type), Label: it.Label})
	}
	respondJSON(w, http.StatusOK, out)
}

func (s *Server) handleMetaOperators(w http.ResponseWriter, _ *http.Request) {
	respondJSON(w, http.StatusOK, operatorLabels)
}

func (s *Server) handleMetaAggregateBy(w http.ResponseWriter, _ *http.Request) {
	respondJSON(w, http.StatusOK, aggregateByLabels)
}
