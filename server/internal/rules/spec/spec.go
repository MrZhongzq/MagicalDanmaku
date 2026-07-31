// Package spec 是规则的序列化表示，同时服务三个通道：
//
//	YAML 配置导入 ──┐
//	数据库 JSONB  ──┼──→ spec.Rule ──→ rules.Rule（领域模型）
//	HTTP API      ──┘
//
// 三处若各写一份，字段名与默认值必然漂移。本包是唯一的线上格式定义，
// 与 internal/rules 的领域模型严格分离：领域模型只管求值，本包只管
// 序列化与转换。
package spec

// Config 是完整的配置树，只在 YAML 导入路径上用。
// 数据库路径下账号、直播间、规则各自成表，用不到这一层。
type Config struct {
	Accounts []Account `yaml:"accounts" json:"accounts"`
}

// Account 是一个账号及其连接的全部直播间。
type Account struct {
	Name       string   `yaml:"name"       json:"name"`
	CookieFile string   `yaml:"cookieFile" json:"cookieFile"`
	RateLimit  Duration `yaml:"rateLimit"  json:"rateLimit"`
	MaxLength  int      `yaml:"maxLength"  json:"maxLength"`
	Rooms      []Room   `yaml:"rooms"      json:"rooms"`
}

// Room 是某账号在某直播间的配置，即一个绑定。
type Room struct {
	ID             string              `yaml:"id"             json:"id"`
	CooldownGroups map[string]Duration `yaml:"cooldownGroups" json:"cooldownGroups"`
	Rules          []Rule              `yaml:"rules"          json:"rules"`
}

// Rule 是一条规则的序列化形式。
//
// 存进数据库时 name 与 enabled 会被提到列上，JSONB 里不含这两个字段——
// 同一个值存两处必然漂移。见 store/rule.go。
type Rule struct {
	Name          string     `yaml:"name"          json:"name,omitempty"`
	Enabled       *bool      `yaml:"enabled"       json:"enabled,omitempty"` // 指针以区分「未写」与「写了 false」
	On            []string   `yaml:"on"            json:"on,omitempty"`
	Schedule      string     `yaml:"schedule"      json:"schedule,omitempty"`
	When          *Condition `yaml:"when"          json:"when,omitempty"`
	Aggregate     *Aggregate `yaml:"aggregate"     json:"aggregate,omitempty"`
	Cooldown      Duration   `yaml:"cooldown"      json:"cooldown,omitempty"`
	CooldownGroup string     `yaml:"cooldownGroup" json:"cooldownGroup,omitempty"`
	Do            []Action   `yaml:"do"            json:"do,omitempty"`
}

// Condition 是条件树的一个节点。
type Condition struct {
	Field  string      `yaml:"field"  json:"field,omitempty"`
	Op     string      `yaml:"op"     json:"op,omitempty"`
	Value  any         `yaml:"value"  json:"value,omitempty"`
	All    []Condition `yaml:"all"    json:"all,omitempty"`
	Any    []Condition `yaml:"any"    json:"any,omitempty"`
	Not    *Condition  `yaml:"not"    json:"not,omitempty"`
	Script string      `yaml:"script" json:"script,omitempty"`
}

// Aggregate 是合并窗口的序列化形式。
type Aggregate struct {
	Window   Duration `yaml:"window"   json:"window"`
	MaxWait  Duration `yaml:"maxWait"  json:"maxWait,omitempty"`
	MinCount int      `yaml:"minCount" json:"minCount,omitempty"`
	By       string   `yaml:"by"       json:"by"`
}

// Action 是一个动作的序列化形式。
type Action struct {
	Type     string   `yaml:"type"     json:"type"`
	Template []string `yaml:"template" json:"template,omitempty"`
	Hours    int      `yaml:"hours"    json:"hours,omitempty"`
	Script   string   `yaml:"script"   json:"script,omitempty"`
}
