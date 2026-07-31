package rules

import (
	"errors"
	"fmt"
	"regexp"
	"strings"
	"sync"
)

// ErrNoScriptRunner 表示条件使用了脚本，但未配置脚本执行器。
var ErrNoScriptRunner = errors.New("rules: 条件使用了脚本，但未配置脚本执行器")

// ScriptRunner 执行 JS 表达式并返回布尔结果。
type ScriptRunner interface {
	// EvalBool 求值一段 JS 表达式，vars 会作为全局 event 注入。
	EvalBool(code string, vars map[string]any) (bool, error)
}

// Evaluator 求值条件树。
type Evaluator interface {
	Eval(c Condition, vars map[string]any) (bool, error)
}

// evaluator 是默认实现。
type evaluator struct {
	script ScriptRunner

	// 正则编译结果缓存：同一条规则会被反复求值，
	// 每次重新编译在高频弹幕下开销显著。
	mu      sync.RWMutex
	reCache map[string]*regexp.Regexp
}

// NewEvaluator 创建条件求值器。script 可为 nil，此时不支持脚本条件。
func NewEvaluator(script ScriptRunner) Evaluator {
	return &evaluator{script: script, reCache: make(map[string]*regexp.Regexp)}
}

// Eval 递归求值条件树。
//
// 零值条件视为无条件通过，便于 Rule.When == nil 时统一处理。
func (e *evaluator) Eval(c Condition, vars map[string]any) (bool, error) {
	switch {
	case len(c.All) > 0:
		for _, sub := range c.All {
			ok, err := e.Eval(sub, vars)
			if err != nil {
				return false, err
			}
			if !ok {
				return false, nil
			}
		}
		return true, nil

	case len(c.Any) > 0:
		for _, sub := range c.Any {
			ok, err := e.Eval(sub, vars)
			if err != nil {
				return false, err
			}
			if ok {
				return true, nil
			}
		}
		return false, nil

	case c.Not != nil:
		ok, err := e.Eval(*c.Not, vars)
		if err != nil {
			return false, err
		}
		return !ok, nil

	case c.Script != "":
		if e.script == nil {
			return false, ErrNoScriptRunner
		}
		return e.script.EvalBool(c.Script, vars)

	case c.Field != "":
		return e.evalLeaf(c, vars)

	default:
		// 零值条件：无条件通过
		return true, nil
	}
}

// evalLeaf 求值单个字段比较。
//
// 字段缺失一律视为不匹配而非报错——B 站的字段时有时无，
// 规则不该因此崩掉。
func (e *evaluator) evalLeaf(c Condition, vars map[string]any) (bool, error) {
	actual, ok := LookupPath(vars, c.Field)
	if !ok {
		return false, nil
	}

	switch c.Op {
	case "eq":
		return looseEqual(actual, c.Value), nil
	case "ne":
		return !looseEqual(actual, c.Value), nil

	case "gt", "gte", "lt", "lte":
		a, ok1 := toFloat(actual)
		b, ok2 := toFloat(c.Value)
		if !ok1 || !ok2 {
			return false, nil // 非数值，视为不匹配
		}
		switch c.Op {
		case "gt":
			return a > b, nil
		case "gte":
			return a >= b, nil
		case "lt":
			return a < b, nil
		default:
			return a <= b, nil
		}

	case "contains":
		return strings.Contains(toString(actual), toString(c.Value)), nil
	case "prefix":
		return strings.HasPrefix(toString(actual), toString(c.Value)), nil
	case "suffix":
		return strings.HasSuffix(toString(actual), toString(c.Value)), nil

	case "regex":
		re, err := e.compile(toString(c.Value))
		if err != nil {
			return false, err
		}
		return re.MatchString(toString(actual)), nil

	case "in":
		list, ok := c.Value.([]any)
		if !ok {
			return false, nil
		}
		for _, item := range list {
			if looseEqual(actual, item) {
				return true, nil
			}
		}
		return false, nil

	default:
		return false, fmt.Errorf("rules: 未知的操作符 %q", c.Op)
	}
}

// compile 编译正则并缓存。
func (e *evaluator) compile(pattern string) (*regexp.Regexp, error) {
	e.mu.RLock()
	re, ok := e.reCache[pattern]
	e.mu.RUnlock()
	if ok {
		return re, nil
	}

	re, err := regexp.Compile(pattern)
	if err != nil {
		return nil, fmt.Errorf("rules: 非法的正则表达式 %q: %w", pattern, err)
	}

	e.mu.Lock()
	e.reCache[pattern] = re
	e.mu.Unlock()
	return re, nil
}

// looseEqual 做宽松相等比较：数值跨类型可比，其余按字符串比。
//
// 必要性：YAML 解析出的整数是 int，而事件里的计数是 int64，
// 严格比较会让 {field: gift.count, op: eq, value: 10} 意外失败。
func looseEqual(a, b any) bool {
	if af, ok1 := toFloat(a); ok1 {
		if bf, ok2 := toFloat(b); ok2 {
			return af == bf
		}
	}
	if ab, ok1 := a.(bool); ok1 {
		if bb, ok2 := b.(bool); ok2 {
			return ab == bb
		}
	}
	return toString(a) == toString(b)
}

// toFloat 把任意数值类型转成 float64。
func toFloat(v any) (float64, bool) {
	switch t := v.(type) {
	case int:
		return float64(t), true
	case int32:
		return float64(t), true
	case int64:
		return float64(t), true
	case float32:
		return float64(t), true
	case float64:
		return t, true
	default:
		return 0, false
	}
}

// toString 把任意值转成字符串用于文本比较。
func toString(v any) string {
	switch t := v.(type) {
	case string:
		return t
	case nil:
		return ""
	default:
		return fmt.Sprintf("%v", t)
	}
}
