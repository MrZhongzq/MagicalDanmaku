package httpapi

import (
	"encoding/json"
	"errors"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/MrZhongzq/MagicalDanmaku/server/internal/rules/spec"
	"github.com/MrZhongzq/MagicalDanmaku/server/internal/store"
)

// ruleView 是规则对外的表示。
//
// 规则体直接就是 spec.Rule——它带 json 标签，是 P3 定好的「唯一序列化
// 表示」。API 不再定义第二套规则 DTO，那会成为第二处字段展开，
// 而字段展开有两处必然漂移。
type ruleView struct {
	spec.Rule
	Position int `json:"position"`
}

func (s *Server) handleListRules(w http.ResponseWriter, r *http.Request) {
	b := bindingFrom(r.Context())

	recs, err := s.store.ListRules(r.Context(), b.ID)
	if err != nil {
		respondStoreError(w, err, "")
		return
	}
	out := make([]ruleView, 0, len(recs))
	for _, rec := range recs {
		out = append(out, ruleView{Rule: rec.Spec, Position: rec.Position})
	}
	respondJSON(w, http.StatusOK, out)
}

// decodeRuleBody 只解析请求体，不做业务校验。
//
// PUT 单条规则时请求体允许不带 name（URL 里的名字是权威的，见
// handlePutRule），如果在这里就校验，body 里没有 name 会被
// rules.Rule.Validate() 当成「规则名不能为空」直接拒掉——那时候
// 名字还没从 URL 补上，是校验时机错了，不是请求真的不合法。
// 校验被拆到 validateRule，由调用方在补好 Name 之后再调用。
func decodeRuleBody(w http.ResponseWriter, r *http.Request) (spec.Rule, bool) {
	var rule spec.Rule
	if !decodeJSON(w, r, &rule) {
		return rule, false
	}
	return rule, true
}

// validateRule 校验一条规则。失败时已写过 422 响应。
//
// 校验用 spec.Rule.ToRule()——它是 P3 唯一的规则校验入口，
// 未知事件类型、非法正则、非法 cron、空动作列表都在那里被拒。
func validateRule(w http.ResponseWriter, rule spec.Rule) bool {
	if _, err := rule.ToRule(); err != nil {
		respondError(w, http.StatusUnprocessableEntity, "规则不合法: %v", err)
		return false
	}
	return true
}

// decodeRule 解析并校验一条规则，供请求体本身自带 name 的接口
// （POST 创建、validate）使用。校验失败时已写过 422 响应。
func decodeRule(w http.ResponseWriter, r *http.Request) (spec.Rule, bool) {
	rule, ok := decodeRuleBody(w, r)
	if !ok {
		return rule, false
	}
	if !validateRule(w, rule) {
		return rule, false
	}
	return rule, true
}

func (s *Server) handleCreateRule(w http.ResponseWriter, r *http.Request) {
	b := bindingFrom(r.Context())

	rule, ok := decodeRule(w, r)
	if !ok {
		return
	}
	if rule.Name == "" {
		respondError(w, http.StatusUnprocessableEntity, "规则名不能为空")
		return
	}

	// 新规则排在最后
	existing, err := s.store.ListRules(r.Context(), b.ID)
	if err != nil {
		respondStoreError(w, err, "")
		return
	}

	rec, err := s.store.SaveRule(r.Context(), b.ID, len(existing), rule)
	if err != nil {
		respondStoreError(w, err, "")
		return
	}
	respondJSON(w, http.StatusCreated, ruleView{Rule: rec.Spec, Position: rec.Position})
}

// handlePutRule 覆盖单条规则，URL 里的名字是权威的。
func (s *Server) handlePutRule(w http.ResponseWriter, r *http.Request) {
	b := bindingFrom(r.Context())
	name := r.PathValue("name")

	rule, ok := decodeRuleBody(w, r)
	if !ok {
		return
	}
	// URL 里的名字说了算，避免请求体改名导致「改一条变成建一条」。
	// 必须先补名字再校验：body 允许不带 name，若在补名字前就校验，
	// 会被 Validate() 当成「规则名不能为空」误拒——见 decodeRuleBody。
	rule.Name = name
	if !validateRule(w, rule) {
		return
	}

	// PUT 是 upsert：已存在就保住它原来的位置，不存在就排到最后。
	//
	// **必须把 ErrNotFound 与真正的数据库错误分开。** 写成
	// `pos := 0; if err == nil { pos = existing.Position }` 的话，
	// 一次网络抖动就会被当成「这条规则不存在」，接着 SaveRule 的
	// ON CONFLICT DO UPDATE SET position = 0 把一条现有规则悄悄挪到
	// 最前面。规则顺序决定谁先触发，这是不报任何错的行为改变。
	pos := 0
	existing, err := s.store.GetRule(r.Context(), b.ID, name)
	switch {
	case err == nil:
		pos = existing.Position
	case errors.Is(err, store.ErrNotFound):
		// 新规则排到最后，与 handleCreateRule 一致。放在 0 会插到
		// 现有第一条前面——PUT 一条新规则不该改变别人的先后
		all, err := s.store.ListRules(r.Context(), b.ID)
		if err != nil {
			respondStoreError(w, err, "")
			return
		}
		pos = len(all)
	default:
		respondStoreError(w, err, "")
		return
	}

	rec, err := s.store.SaveRule(r.Context(), b.ID, pos, rule)
	if err != nil {
		respondStoreError(w, err, "")
		return
	}
	respondJSON(w, http.StatusOK, ruleView{Rule: rec.Spec, Position: rec.Position})
}

// handleReplaceRules 整组替换。整批在一个事务里，中途一条非法就整批回滚。
func (s *Server) handleReplaceRules(w http.ResponseWriter, r *http.Request) {
	b := bindingFrom(r.Context())

	var list []spec.Rule
	dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4<<20))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&list); err != nil {
		respondError(w, http.StatusUnprocessableEntity, "请求体不合法: %v", err)
		return
	}

	// 先逐条校验，报错时能指出是第几条——整批失败却不说哪条错，很难查。
	// 重名也在这里查：ReplaceRules 内部也会拒，但它只知道名字、不知道
	// 是第几条，而且从那条路径回不来「这是客户端的错」这个信息
	seen := make(map[string]bool, len(list))
	for i, rule := range list {
		if rule.Name == "" {
			respondError(w, http.StatusUnprocessableEntity, "第 %d 条规则缺少名字", i+1)
			return
		}
		if seen[rule.Name] {
			respondError(w, http.StatusUnprocessableEntity,
				"第 %d 条规则的名字 %q 与前面重复——同绑定内重名会让冷却互相干扰",
				i+1, rule.Name)
			return
		}
		seen[rule.Name] = true
		if _, err := rule.ToRule(); err != nil {
			respondError(w, http.StatusUnprocessableEntity,
				"第 %d 条规则(%s)不合法: %v", i+1, rule.Name, err)
			return
		}
	}

	// 跨规则校验：Suppress 指向的名字必须在这一批里存在。
	//
	// 只有 rules.NewEngine 做这个检查是不够的——那要等到热重载（POST
	// /reload）才暴露，而此时库里已经存了一份非法配置：PUT 本身会
	// 成功返回 200，用户看不出问题。下次进程启动时装配循环里对这个
	// 绑定的 NewEngine 会失败——虽然现在已经不会拖累整个进程（见
	// cmd/magicd/run.go 的 buildRoomRuntime/assembleRuntimes），但这个
	// 绑定本身依然跑不起来，而用户本可以在 PUT 的这一刻就被拦下来，
	// 不必等到下次启动才发现。
	//
	// 拿本次请求体里的名字集合校验就够——整组替换的语义就是「库里最后
	// 只剩这些」，不用去查库里现存的其他规则名。
	for i, rule := range list {
		for _, sup := range rule.Suppress {
			if !seen[sup] {
				respondError(w, http.StatusUnprocessableEntity,
					"第 %d 条规则(%s)的 suppress 指向不存在的规则名 %q",
					i+1, rule.Name, sup)
				return
			}
		}
	}

	// 走到这里请求体已经全部合法，ReplaceRules 再报错就是存储层的问题，
	// 不是客户端的错。无条件回 422 会把数据库故障说成「你的请求不合法」，
	// 还会把原始数据库错误文本抄给客户端
	if err := s.store.ReplaceRules(r.Context(), b.ID, list); err != nil {
		respondStoreError(w, err, "")
		return
	}
	respondJSON(w, http.StatusOK, map[string]int{"count": len(list)})
}

// handleValidateRule 只校验不保存，供编辑器实时反馈。
func (s *Server) handleValidateRule(w http.ResponseWriter, r *http.Request) {
	if _, ok := decodeRule(w, r); !ok {
		return
	}
	respondJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// handlePatchRule 只切启停，不重写规则体。
func (s *Server) handlePatchRule(w http.ResponseWriter, r *http.Request) {
	b := bindingFrom(r.Context())
	name := r.PathValue("name")

	var req struct {
		Enabled *bool `json:"enabled"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	if req.Enabled == nil {
		respondError(w, http.StatusUnprocessableEntity, "请提供 enabled 字段")
		return
	}

	if err := s.store.SetRuleEnabled(r.Context(), b.ID, name, *req.Enabled); err != nil {
		respondStoreError(w, err, "规则 "+name+" 不存在")
		return
	}
	respondJSON(w, http.StatusOK, map[string]any{"enabled": *req.Enabled})
}

func (s *Server) handleDeleteRule(w http.ResponseWriter, r *http.Request) {
	b := bindingFrom(r.Context())
	name := r.PathValue("name")

	// 删之前检查有没有别的规则在压制它。没有这道检查，删掉一条被
	// Suppress 引用的规则会让引用它的那条规则从此指向一个不存在的
	// 名字——这与 handleReplaceRules 里补的跨规则校验是同一个漏洞的
	// 另一条现实可达路径：那边挡住了"新建时写错名字"，这边挡住
	// "先写对、后来被删掉"。同样是库被写脏、要等到下次启动
	// NewEngine 才会报错。
	rules, err := s.store.ListRules(r.Context(), b.ID)
	if err != nil {
		respondStoreError(w, err, "")
		return
	}
	for _, rec := range rules {
		if rec.Name == name {
			continue
		}
		for _, sup := range rec.Spec.Suppress {
			if sup == name {
				respondError(w, http.StatusUnprocessableEntity,
					"规则 %q 正在被规则 %q 压制引用，删除前请先修改或删除那条规则的 suppress 配置",
					name, rec.Name)
				return
			}
		}
	}

	if err := s.store.DeleteRule(r.Context(), b.ID, name); err != nil {
		respondStoreError(w, err, "规则 "+name+" 不存在")
		return
	}
	respondJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// maxCooldownMS 是单个冷却组间隔的上限，24 小时。
//
// 存在的理由不是「谁会设这么长」，而是 time.Duration 是 int64 纳秒：
// 毫秒数超过约 9.2e12 时 `time.Duration(ms) * time.Millisecond` 会溢出
// 成负数，绕过「不能为负」那道检查。有了上界，那条路就走不通了。
const maxCooldownMS int64 = 24 * 60 * 60 * 1000

// handleGetCooldownGroups 返回冷却组，值是毫秒。
//
// 对外用毫秒整数而不是 "5s" 字符串：前端表单里是数字输入框，
// 而规则体里的时长用字符串（那是给人读 YAML 用的）。两处格式不同
// 是有意的，各自贴合自己的使用场景。
func (s *Server) handleGetCooldownGroups(w http.ResponseWriter, r *http.Request) {
	b := bindingFrom(r.Context())

	groups, err := s.store.CooldownGroups(r.Context(), b.ID)
	if err != nil {
		respondStoreError(w, err, "")
		return
	}
	out := make(map[string]int64, len(groups))
	for k, v := range groups {
		out[k] = v.Milliseconds()
	}
	respondJSON(w, http.StatusOK, out)
}

// handlePutCooldownGroups 整组替换冷却组。
func (s *Server) handlePutCooldownGroups(w http.ResponseWriter, r *http.Request) {
	b := bindingFrom(r.Context())

	var req map[string]int64
	if !decodeJSON(w, r, &req) {
		return
	}

	// 遍历 map 的顺序是随机的，先排序再校验——否则同时有两条非法时
	// 报的是哪一条每次都不一样，用户按报错改完一条又冒出另一条
	names := make([]string, 0, len(req))
	for name := range req {
		names = append(names, name)
	}
	sort.Strings(names)

	groups := make(map[string]time.Duration, len(req))
	for _, name := range names {
		ms := req[name]

		// 存 trim 过的名字，不是原样存。规则体里写的是 cooldownGroup: 甲，
		// 这边存成 "  甲  " 就永远匹配不上，而且不报任何错——
		// 用户只会看到「冷却怎么不生效」
		key := strings.TrimSpace(name)
		if key == "" {
			respondError(w, http.StatusUnprocessableEntity, "冷却组名不能为空")
			return
		}
		if ms < 0 {
			respondError(w, http.StatusUnprocessableEntity, "冷却组 %s 的间隔不能为负", key)
			return
		}
		// 上界不是洁癖。time.Duration 是 int64 纳秒，ms 超过约 9.2e12
		// 时 `time.Duration(ms) * time.Millisecond` 会溢出成负数，
		// **绕过上面那行 ms < 0**，把负冷却写进库
		if ms > maxCooldownMS {
			respondError(w, http.StatusUnprocessableEntity,
				"冷却组 %s 的间隔 %d 毫秒超过上限（%d 毫秒，即 24 小时）",
				key, ms, maxCooldownMS)
			return
		}
		if _, dup := groups[key]; dup {
			respondError(w, http.StatusUnprocessableEntity,
				"冷却组名 %s 重复（首尾空白不算区别）", key)
			return
		}
		groups[key] = time.Duration(ms) * time.Millisecond
	}

	if err := s.store.SetCooldownGroups(r.Context(), b.ID, groups); err != nil {
		respondStoreError(w, err, "")
		return
	}
	respondJSON(w, http.StatusOK, map[string]int{"count": len(groups)})
}
