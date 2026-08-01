package rules

import (
	"fmt"
	"math/rand"
	"strings"
	"sync"
	"text/template"
)

// Renderer 渲染动作模板。
//
// 采用标准库 text/template 而非自造语法：已充分测试、支持条件与循环、
// 无需维护解析器。这与「弃用原项目自创 DSL」的决策一脉相承。
type Renderer struct {
	rnd *rand.Rand

	// 模板编译结果缓存：同一条规则会被反复触发，
	// 每次重新解析在高频事件下开销显著。
	mu    sync.Mutex
	cache map[string]*template.Template
}

// NewRenderer 创建渲染器。rnd 为 nil 时使用全局随机源。
func NewRenderer(rnd *rand.Rand) *Renderer {
	return &Renderer{rnd: rnd, cache: make(map[string]*template.Template)}
}

// Render 从多条模板中随机挑一条渲染，实现文案变化。
func (r *Renderer) Render(templates []string, vars map[string]any) (string, error) {
	if len(templates) == 0 {
		return "", fmt.Errorf("rules: 模板列表为空")
	}
	idx := 0
	if len(templates) > 1 {
		idx = r.intn(len(templates))
	}
	return r.RenderOne(templates[idx], vars)
}

// RenderAt 从多条模板中按下标渲染指定一条，供顺序轮询模式使用。
//
// idx 对 len(templates) 取模，负数或超界都不报错、不 panic——
// 调用方（Executor 的游标）不需要自己先做边界检查。
func (r *Renderer) RenderAt(templates []string, idx int, vars map[string]any) (string, error) {
	if len(templates) == 0 {
		return "", fmt.Errorf("rules: 模板列表为空")
	}
	n := len(templates)
	idx %= n
	if idx < 0 {
		idx += n
	}
	return r.RenderOne(templates[idx], vars)
}

// RenderOne 渲染单条模板。
func (r *Renderer) RenderOne(tmpl string, vars map[string]any) (string, error) {
	t, err := r.compile(tmpl)
	if err != nil {
		return "", err
	}
	var sb strings.Builder
	if err := t.Execute(&sb, vars); err != nil {
		return "", fmt.Errorf("rules: 模板渲染失败 %q: %w", truncateForError(tmpl), err)
	}
	return sb.String(), nil
}

// compile 解析模板并缓存。
func (r *Renderer) compile(tmpl string) (*template.Template, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if t, ok := r.cache[tmpl]; ok {
		return t, nil
	}
	t, err := template.New("action").
		Funcs(r.funcMap()).
		Parse(rewriteFieldChains(tmpl))
	if err != nil {
		return nil, fmt.Errorf("rules: 模板解析失败 %q: %w", truncateForError(tmpl), err)
	}
	r.cache[tmpl] = t
	return t, nil
}

// rewriteFieldChains 把模板中的字段访问改写成安全取值调用。
//
//	{{.user.medal.name}}  →  {{(get . "user.medal.name")}}
//
// 必要性：text/template 对 map[string]any 的深层访问，一旦中间层缺失
// 就会以 "nil pointer evaluating interface {}.name" 报错；而
// missingkey=zero 对 any 类型只会渲染出 "<no value>"。两者都违反
// 「字段缺失不得导致渲染失败」的约定。
//
// 改写只在 {{ }} 内部进行，且跳过引号内的字符串字面量，
// 避免误伤 {{pick "a.b" "c"}} 这类写法。
func rewriteFieldChains(tmpl string) string {
	var out strings.Builder
	out.Grow(len(tmpl) + 32)

	rest := tmpl
	for {
		start := strings.Index(rest, "{{")
		if start < 0 {
			out.WriteString(rest)
			break
		}
		end := strings.Index(rest[start:], "}}")
		if end < 0 {
			// 未闭合的动作原样保留，交由 Parse 报语法错误
			out.WriteString(rest)
			break
		}
		end += start

		out.WriteString(rest[:start+2])
		out.WriteString(rewriteAction(rest[start+2 : end]))
		out.WriteString("}}")
		rest = rest[end+2:]
	}
	return out.String()
}

// rewriteAction 改写单个动作体内的字段链，跳过字符串字面量。
func rewriteAction(action string) string {
	var out strings.Builder
	out.Grow(len(action) + 16)

	for i := 0; i < len(action); {
		c := action[i]

		// 跳过字符串字面量与反引号原始串
		if c == '"' || c == '`' {
			quote := c
			j := i + 1
			for j < len(action) {
				if action[j] == '\\' && quote == '"' {
					j += 2
					continue
				}
				if action[j] == quote {
					j++
					break
				}
				j++
			}
			out.WriteString(action[i:j])
			i = j
			continue
		}

		if c != '.' {
			out.WriteByte(c)
			i++
			continue
		}

		// 前一个字符若是标识符字符，说明这个点属于别处（如数字或已有调用）
		if i > 0 && isIdentByte(action[i-1]) {
			out.WriteByte(c)
			i++
			continue
		}

		path, next := scanFieldChain(action, i)
		// 孤立的点（如 {{.}} 或 range 的上下文点）原样保留
		if path == "" {
			out.WriteString(action[i:next])
			i = next
			continue
		}
		// 单段路径同样改写：原生 map 索引在键缺失时渲染出 "<no value>"，
		// 而非约定要求的空串。
		fmt.Fprintf(&out, `(get . %q)`, path)
		i = next
	}
	return out.String()
}

// scanFieldChain 从 action[i] 处的点开始扫描字段链，
// 返回不含前导点的路径与链结束后的下标。
func scanFieldChain(action string, i int) (path string, next int) {
	j := i
	var sb strings.Builder
	for j < len(action) && action[j] == '.' {
		j++
		segStart := j
		for j < len(action) && isIdentByte(action[j]) {
			j++
		}
		if j == segStart { // 孤立的点，如 {{.}}
			break
		}
		if sb.Len() > 0 {
			sb.WriteByte('.')
		}
		sb.WriteString(action[segStart:j])
	}
	return sb.String(), j
}

// isIdentByte 判断字节是否可作为标识符的一部分。
// 非 ASCII 字节一律视为标识符字符，以支持中文字段名。
func isIdentByte(b byte) bool {
	return b >= 0x80 ||
		b == '_' ||
		(b >= 'a' && b <= 'z') ||
		(b >= 'A' && b <= 'Z') ||
		(b >= '0' && b <= '9')
}

// tmplGet 按点分路径安全取值，任何一层缺失都返回空串而非报错。
func tmplGet(vars map[string]any, path string) any {
	v, ok := LookupPath(vars, path)
	if !ok || v == nil {
		return ""
	}
	return v
}

// intn 返回 [0,n) 的随机数，线程安全。
func (r *Renderer) intn(n int) int {
	if r.rnd == nil {
		return rand.Intn(n)
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.rnd.Intn(n)
}

// funcMap 返回模板可用的内置函数。
func (r *Renderer) funcMap() template.FuncMap {
	return template.FuncMap{
		"get":          tmplGet,
		"join":         tmplJoin,
		"simplifyName": SimplifyName,
		"truncate":     tmplTruncate,
		"pick":         r.tmplPick,
		"int":          tmplInt,
	}
}

// tmplJoin 拼接数组，兼容 []string 与 []any。
func tmplJoin(v any, sep string) string {
	switch t := v.(type) {
	case []string:
		return strings.Join(t, sep)
	case []any:
		parts := make([]string, 0, len(t))
		for _, item := range t {
			parts = append(parts, toString(item))
		}
		return strings.Join(parts, sep)
	case nil:
		return ""
	default:
		return toString(v)
	}
}

// tmplTruncate 按字符（而非字节）截断，避免把中文切坏。
func tmplTruncate(v any, n int) string {
	s := toString(v)
	runes := []rune(s)
	if n < 0 || len(runes) <= n {
		return s
	}
	return string(runes[:n])
}

// tmplInt 把值转成 int，供模板里的数值比较使用。
// text/template 的 gt/lt 要求类型一致，而 Vars 中的数值类型不统一。
func tmplInt(v any) int {
	f, _ := toFloat(v)
	return int(f)
}

// tmplPick 从参数中随机取一个。
func (r *Renderer) tmplPick(items ...string) string {
	if len(items) == 0 {
		return ""
	}
	return items[r.intn(len(items))]
}

// nameDecorations 是昵称中常见的装饰性前后缀。
var nameDecorations = []string{
	"_official", "-official", "【官方】", "官方",
	"·-·", "-·-", "、", "丶",
}

// SimplifyName 去除昵称中常见的装饰性前后缀，让答谢弹幕更自然。
//
// 只做保守的前后缀剥离，不动昵称中间的内容——把「某某某-许许的蓷」
// 截成「某某某」会认错人。
func SimplifyName(v any) string {
	s := strings.TrimSpace(toString(v))
	if s == "" {
		return ""
	}

	changed := true
	for changed {
		changed = false
		for _, d := range nameDecorations {
			if len(s) > len(d) && strings.HasPrefix(s, d) {
				s = strings.TrimPrefix(s, d)
				changed = true
			}
			if len(s) > len(d) && strings.HasSuffix(s, d) {
				s = strings.TrimSuffix(s, d)
				changed = true
			}
		}
		s = strings.TrimSpace(s)
	}
	return s
}

// truncateForError 截断过长模板，避免错误信息刷屏。
func truncateForError(s string) string {
	r := []rune(s)
	if len(r) <= 40 {
		return s
	}
	return string(r[:40]) + "..."
}
