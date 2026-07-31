package store

import (
	"context"
	"fmt"
	"time"

	"github.com/MrZhongzq/MagicalDanmaku/server/internal/rules/spec"
)

// ImportAccount 是待导入的一个账号及其直播间。
//
// 没有 UID 字段：YAML 里无处写 UID，这里加了也是零调用者的死字段。
// UID 由 login --save 扫码写入，UpsertAccount 的 uid 列用 CASE 保留
// 已有值，import 不会去动它。
type ImportAccount struct {
	Name      string
	Cookie    string
	RateLimit time.Duration
	MaxLength int
	Rooms     []ImportRoom
}

// ImportRoom 是待导入的一个直播间配置。
type ImportRoom struct {
	RoomID         string
	CooldownGroups map[string]time.Duration
	Rules          []spec.Rule
}

// ImportResult 是导入的统计。
type ImportResult struct {
	Accounts int
	Bindings int
	Rules    int
}

// ImportConfig 把配置导入数据库，按名字 upsert。
//
// 幂等：同一份 YAML 导两次结果一致。规则整组替换，因此 YAML 里删掉的
// 规则重新导入后库里也会消失；绑定的 enabled 不动，导入不该把用户手动
// 停用的绑定又打开。
func (s *Store) ImportConfig(ctx context.Context, ownerID int64, accounts []ImportAccount) (*ImportResult, error) {
	res := &ImportResult{}

	for _, a := range accounts {
		acc, err := s.UpsertAccount(ctx, AccountInput{
			Name:      a.Name,
			Cookie:    a.Cookie,
			RateLimit: a.RateLimit,
			MaxLength: a.MaxLength,
			OwnerID:   ownerID,
		})
		if err != nil {
			return nil, err
		}
		res.Accounts++

		for _, r := range a.Rooms {
			b, err := s.UpsertBinding(ctx, acc.ID, r.RoomID)
			if err != nil {
				return nil, fmt.Errorf("导入账号 %q 直播间 %s 失败: %w", a.Name, r.RoomID, err)
			}
			res.Bindings++

			if err := s.SetCooldownGroups(ctx, b.ID, r.CooldownGroups); err != nil {
				return nil, fmt.Errorf("导入 %s 的冷却组失败: %w", b.Label(), err)
			}
			if err := s.ReplaceRules(ctx, b.ID, r.Rules); err != nil {
				return nil, fmt.Errorf("导入 %s 的规则失败: %w", b.Label(), err)
			}
			res.Rules += len(r.Rules)
		}
	}
	return res, nil
}
