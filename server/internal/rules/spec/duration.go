package spec

import (
	"encoding/json"
	"fmt"
	"time"

	"gopkg.in/yaml.v3"
)

// Duration 是可读的时长，YAML 与 JSON 两侧都用 "1.5s"、"500ms"、"3m" 这种
// 字符串形式。
//
// 不接受裸数字：`window: 1500` 到底是毫秒还是纳秒，读的人猜不出来。
// JSONB 里也存字符串，是为了让人能直接看懂库里的行。
type Duration time.Duration

// UnmarshalYAML 解析形如 "1.5s" 的时长字符串。
func (d *Duration) UnmarshalYAML(n *yaml.Node) error {
	var s string
	if err := n.Decode(&s); err != nil {
		return fmt.Errorf("时长必须是字符串，如 \"1.5s\": %w", err)
	}
	return d.parse(s)
}

// MarshalYAML 输出形如 "1m30s" 的时长字符串。
func (d Duration) MarshalYAML() (any, error) {
	return time.Duration(d).String(), nil
}

// UnmarshalJSON 解析形如 "1.5s" 的时长字符串。
func (d *Duration) UnmarshalJSON(b []byte) error {
	var s string
	if err := json.Unmarshal(b, &s); err != nil {
		return fmt.Errorf("时长必须是字符串，如 \"1.5s\": %w", err)
	}
	return d.parse(s)
}

// MarshalJSON 输出形如 "1m30s" 的时长字符串。
func (d Duration) MarshalJSON() ([]byte, error) {
	return json.Marshal(time.Duration(d).String())
}

// parse 是两条反序列化路径共用的解析。
func (d *Duration) parse(s string) error {
	parsed, err := time.ParseDuration(s)
	if err != nil {
		return fmt.Errorf("非法的时长 %q: %w", s, err)
	}
	*d = Duration(parsed)
	return nil
}
