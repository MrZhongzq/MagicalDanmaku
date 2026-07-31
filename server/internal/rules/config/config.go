// Package config 负责从 YAML 加载并校验规则配置。
//
// 配置结构与运行模型同构，三层嵌套：账号 → 直播间 → 规则。
// 每个「账号-直播间」组合是一个独立的运行单元（绑定）。
//
// 线上格式与规则转换住在 internal/rules/spec，本包只做两件事：
// 读文件，以及校验配置树的形状（账号名重复、缺 cookieFile 之类）。
// P3 之后 YAML 降级为导入入口，运行期配置从数据库读。
package config

import (
	"fmt"
	"os"
	"time"

	"gopkg.in/yaml.v3"

	"github.com/MrZhongzq/MagicalDanmaku/server/internal/rules"
	"github.com/MrZhongzq/MagicalDanmaku/server/internal/rules/spec"
)

// Duration 是 spec.Duration 的别名。别名而非新类型：两处各定义一遍
// 解析逻辑，迟早会在某个边角上不一致。
type Duration = spec.Duration

// Config 是完整配置。
type Config struct {
	Accounts []Account
}

// Account 是一个已登录账号及其连接的全部直播间。
type Account struct {
	Name       string
	CookieFile string
	RateLimit  Duration // 该账号全部直播间共享的发送间隔
	MaxLength  int      // 单条弹幕字符上限，0 表示用默认值
	Rooms      []Room
}

// Room 是某个账号在某个直播间的配置，即一个绑定。
type Room struct {
	ID             string
	CooldownGroups map[string]Duration
	Rules          []rules.Rule
}

// Binding 是摊平后的运行单元。
type Binding struct {
	AccountName    string
	CookieFile     string
	RateLimit      time.Duration
	MaxLength      int
	RoomID         string
	CooldownGroups map[string]time.Duration
	Rules          []rules.Rule
}

// Bindings 把三层结构摊平成运行单元列表，保持配置顺序。
func (c *Config) Bindings() []Binding {
	var out []Binding
	for _, a := range c.Accounts {
		for _, r := range a.Rooms {
			groups := make(map[string]time.Duration, len(r.CooldownGroups))
			for k, v := range r.CooldownGroups {
				groups[k] = time.Duration(v)
			}
			out = append(out, Binding{
				AccountName:    a.Name,
				CookieFile:     a.CookieFile,
				RateLimit:      time.Duration(a.RateLimit),
				MaxLength:      a.MaxLength,
				RoomID:         r.ID,
				CooldownGroups: groups,
				Rules:          r.Rules,
			})
		}
	}
	return out
}

// Load 从文件加载配置。
func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("config: 读取配置文件 %s 失败: %w", path, err)
	}
	c, err := Parse(data)
	if err != nil {
		return nil, fmt.Errorf("config: 解析 %s 失败: %w", path, err)
	}
	return c, nil
}

// Parse 解析配置内容并做完整校验。
//
// 校验失败即报错退出，不允许带病运行——配置写错却静默忽略，
// 比直接报错更难排查。
//
// 单条规则的校验在 spec.Rule.ToRule 里，这里只管配置树的形状。
func Parse(data []byte) (*Config, error) {
	var raw spec.Config
	if err := yaml.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("YAML 语法错误: %w", err)
	}
	if len(raw.Accounts) == 0 {
		return nil, fmt.Errorf("配置中没有任何账号")
	}

	c := &Config{}
	seenAccount := make(map[string]bool, len(raw.Accounts))

	for i, ay := range raw.Accounts {
		if ay.Name == "" {
			return nil, fmt.Errorf("第 %d 个账号缺少 name", i+1)
		}
		if seenAccount[ay.Name] {
			return nil, fmt.Errorf("账号名 %q 重复", ay.Name)
		}
		seenAccount[ay.Name] = true

		if ay.CookieFile == "" {
			return nil, fmt.Errorf("账号 %q 缺少 cookieFile", ay.Name)
		}
		if len(ay.Rooms) == 0 {
			return nil, fmt.Errorf("账号 %q 未配置任何直播间", ay.Name)
		}

		acc := Account{
			Name:       ay.Name,
			CookieFile: ay.CookieFile,
			RateLimit:  ay.RateLimit,
			MaxLength:  ay.MaxLength,
		}

		seenRoom := make(map[string]bool, len(ay.Rooms))
		for j, ry := range ay.Rooms {
			if ry.ID == "" {
				return nil, fmt.Errorf("账号 %q 的第 %d 个直播间缺少 id", ay.Name, j+1)
			}
			if seenRoom[ry.ID] {
				return nil, fmt.Errorf("账号 %q 下的直播间 %s 重复配置", ay.Name, ry.ID)
			}
			seenRoom[ry.ID] = true

			room := Room{ID: ry.ID, CooldownGroups: ry.CooldownGroups}

			// 规则名只需在单个绑定内唯一——同一条「进场欢迎」本来就会
			// 出现在多个绑定下。冷却按规则名记录，同绑定内重名会互相干扰。
			seenRule := make(map[string]bool, len(ry.Rules))
			for k, rl := range ry.Rules {
				r, err := rl.ToRule()
				if err != nil {
					return nil, fmt.Errorf("账号 %q 的直播间 %s 第 %d 条规则(%s)非法: %w",
						ay.Name, ry.ID, k+1, rl.Name, err)
				}
				if seenRule[r.Name] {
					return nil, fmt.Errorf("账号 %q 的直播间 %s 下规则名 %q 重复",
						ay.Name, ry.ID, r.Name)
				}
				seenRule[r.Name] = true
				room.Rules = append(room.Rules, r)
			}

			acc.Rooms = append(acc.Rooms, room)
		}
		c.Accounts = append(c.Accounts, acc)
	}
	return c, nil
}
