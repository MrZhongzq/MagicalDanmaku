package store

import (
	"context"
	"fmt"
	"time"

	"github.com/MrZhongzq/MagicalDanmaku/server/internal/rules"
)

// RunConfig 是一个「账号-直播间」绑定的完整运行配置。
//
// 字段与 P2 的 config.Binding 一一对应，只把 CookieFile 换成了 Cookie
// 本身——run 的装配逻辑因此不必改动。
type RunConfig struct {
	BindingID      int64
	AccountID      int64
	AccountName    string
	Cookie         string
	RoomID         string
	RateLimit      time.Duration
	MaxLength      int
	CooldownGroups map[string]time.Duration
	Rules          []rules.Rule
}

// Label 返回用于日志的标识，形如 "小号@1706666491"。
func (c RunConfig) Label() string {
	return c.AccountName + "@" + c.RoomID
}

// LoadRunConfig 载入全部启用的绑定及其规则与冷却组。
//
// 停用的绑定直接跳过；停用的规则照常带出，由引擎自己跳过——引擎需要
// 知道它存在，才能在日志里报告「共 5 条规则，3 条启用」。
func (s *Store) LoadRunConfig(ctx context.Context) ([]RunConfig, error) {
	bindings, err := s.ListBindings(ctx)
	if err != nil {
		return nil, err
	}

	// 账号可能被多个绑定共用，只查一次
	accounts := make(map[int64]*Account)

	out := make([]RunConfig, 0, len(bindings))
	for _, b := range bindings {
		if !b.Enabled {
			continue
		}

		acc, ok := accounts[b.AccountID]
		if !ok {
			acc, err = s.GetAccountByName(ctx, b.AccountName)
			if err != nil {
				return nil, err
			}
			accounts[b.AccountID] = acc
		}

		groups, err := s.CooldownGroups(ctx, b.ID)
		if err != nil {
			return nil, err
		}

		recs, err := s.ListRules(ctx, b.ID)
		if err != nil {
			return nil, err
		}
		rs := make([]rules.Rule, 0, len(recs))
		for _, rec := range recs {
			r, err := rec.Domain()
			if err != nil {
				return nil, fmt.Errorf("store: %s 的规则 %q 非法: %w", b.Label(), rec.Name, err)
			}
			rs = append(rs, r)
		}

		out = append(out, RunConfig{
			BindingID:      b.ID,
			AccountID:      b.AccountID,
			AccountName:    b.AccountName,
			Cookie:         acc.Cookie,
			RoomID:         b.RoomID,
			RateLimit:      acc.RateLimit,
			MaxLength:      acc.MaxLength,
			CooldownGroups: groups,
			Rules:          rs,
		})
	}
	return out, nil
}
