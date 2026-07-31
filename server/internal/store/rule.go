package store

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"

	"github.com/MrZhongzq/MagicalDanmaku/server/internal/rules"
	"github.com/MrZhongzq/MagicalDanmaku/server/internal/rules/spec"
)

// RuleRecord 是一条存好的规则。
//
// Name 与 Enabled 既是列也出现在 Spec 里，但库里的 JSONB **不含**这两个
// 字段——同一个值存两处必然漂移。拆装只发生在 splitRule 与 assembleRule。
type RuleRecord struct {
	ID        int64
	BindingID int64
	Name      string
	Enabled   bool
	Position  int
	Spec      spec.Rule
}

// Domain 转成规则引擎用的领域模型，顺带完成校验。
func (r RuleRecord) Domain() (rules.Rule, error) {
	return r.Spec.ToRule()
}

// splitRule 把规则拆成「列」与「JSONB 主体」。
//
// 主体里清掉 Name 与 Enabled，因为它们已经是列。
func splitRule(r spec.Rule) (name string, enabled bool, body []byte, err error) {
	name = r.Name
	enabled = true
	if r.Enabled != nil {
		enabled = *r.Enabled
	}

	// 转换一次，非法规则不许进库——写进去了，run 每次启动都会炸
	if _, err := r.ToRule(); err != nil {
		return "", false, nil, fmt.Errorf("store: 规则 %q 非法: %w", name, err)
	}

	r.Name = ""
	r.Enabled = nil
	body, err = json.Marshal(r)
	if err != nil {
		return "", false, nil, fmt.Errorf("store: 序列化规则 %q 失败: %w", name, err)
	}
	return name, enabled, body, nil
}

// assembleRule 把列与 JSONB 主体拼回完整的 spec.Rule。
func assembleRule(name string, enabled bool, body []byte) (spec.Rule, error) {
	var r spec.Rule
	if err := json.Unmarshal(body, &r); err != nil {
		return r, fmt.Errorf("store: 解析规则 %q 失败: %w", name, err)
	}
	r.Name = name
	r.Enabled = &enabled
	return r, nil
}

func scanRule(row pgx.Row) (*RuleRecord, error) {
	var rec RuleRecord
	var body []byte
	if err := row.Scan(&rec.ID, &rec.BindingID, &rec.Name, &rec.Enabled,
		&rec.Position, &body); err != nil {
		return nil, err
	}
	s, err := assembleRule(rec.Name, rec.Enabled, body)
	if err != nil {
		return nil, err
	}
	rec.Spec = s
	return &rec, nil
}

const ruleColumns = `id, binding_id, name, enabled, position, spec`

// SaveRule 保存一条规则，同名则更新。
func (s *Store) SaveRule(ctx context.Context, bindingID int64, position int, r spec.Rule) (*RuleRecord, error) {
	name, enabled, body, err := splitRule(r)
	if err != nil {
		return nil, err
	}
	if name == "" {
		return nil, fmt.Errorf("store: 规则名不能为空")
	}

	rec, err := scanRule(s.pool.QueryRow(ctx, `
		INSERT INTO rules (binding_id, name, enabled, position, spec)
		VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (binding_id, name) DO UPDATE SET
			enabled    = EXCLUDED.enabled,
			position   = EXCLUDED.position,
			spec       = EXCLUDED.spec,
			updated_at = now()
		RETURNING `+ruleColumns,
		bindingID, name, enabled, position, body))
	if err != nil {
		return nil, fmt.Errorf("store: 保存规则 %q 失败: %w", name, err)
	}
	return rec, nil
}

// GetRule 按绑定与规则名查询。
func (s *Store) GetRule(ctx context.Context, bindingID int64, name string) (*RuleRecord, error) {
	rec, err := scanRule(s.pool.QueryRow(ctx,
		`SELECT `+ruleColumns+` FROM rules WHERE binding_id = $1 AND name = $2`,
		bindingID, name))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, fmt.Errorf("store: 规则 %q 不存在: %w", name, ErrNotFound)
	}
	if err != nil {
		return nil, fmt.Errorf("store: 查询规则失败: %w", err)
	}
	return rec, nil
}

// ListRules 按 position 顺序列出某绑定的全部规则。
func (s *Store) ListRules(ctx context.Context, bindingID int64) ([]RuleRecord, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT `+ruleColumns+` FROM rules WHERE binding_id = $1 ORDER BY position, id`,
		bindingID)
	if err != nil {
		return nil, fmt.Errorf("store: 列出规则失败: %w", err)
	}
	defer rows.Close()

	var out []RuleRecord
	for rows.Next() {
		rec, err := scanRule(rows)
		if err != nil {
			return nil, fmt.Errorf("store: 读取规则失败: %w", err)
		}
		out = append(out, *rec)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("store: 列出规则失败: %w", err)
	}
	return out, nil
}

// SetRuleEnabled 启停规则。只动列，不重写 JSONB。
func (s *Store) SetRuleEnabled(ctx context.Context, bindingID int64, name string, enabled bool) error {
	tag, err := s.pool.Exec(ctx, `
		UPDATE rules SET enabled = $1, updated_at = now()
		WHERE binding_id = $2 AND name = $3`, enabled, bindingID, name)
	if err != nil {
		return fmt.Errorf("store: 更新规则 %q 状态失败: %w", name, err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("store: 规则 %q 不存在: %w", name, ErrNotFound)
	}
	return nil
}

// DeleteRule 删除一条规则。
func (s *Store) DeleteRule(ctx context.Context, bindingID int64, name string) error {
	tag, err := s.pool.Exec(ctx,
		`DELETE FROM rules WHERE binding_id = $1 AND name = $2`, bindingID, name)
	if err != nil {
		return fmt.Errorf("store: 删除规则 %q 失败: %w", name, err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("store: 规则 %q 不存在: %w", name, ErrNotFound)
	}
	return nil
}

// ReplaceRules 用给定的规则集整体替换某绑定的规则。
//
// 供 import 使用：YAML 里删掉的规则，重新导入后库里也该没有。
// 整批在一个事务里，中途有一条非法就整体回滚——留下半套规则比
// 直接失败更难排查。
func (s *Store) ReplaceRules(ctx context.Context, bindingID int64, rs []spec.Rule) error {
	// 先校验并拆好，再开事务：非法输入不该浪费一次事务
	type prepared struct {
		name    string
		enabled bool
		body    []byte
	}
	items := make([]prepared, 0, len(rs))
	seen := make(map[string]bool, len(rs))

	for _, r := range rs {
		name, enabled, body, err := splitRule(r)
		if err != nil {
			return err
		}
		if name == "" {
			return fmt.Errorf("store: 规则名不能为空")
		}
		// 规则名只需在单个绑定内唯一，但同绑定内重名会让冷却互相干扰
		if seen[name] {
			return fmt.Errorf("store: 同一绑定下规则名 %q 重复", name)
		}
		seen[name] = true
		items = append(items, prepared{name, enabled, body})
	}

	err := pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, `DELETE FROM rules WHERE binding_id = $1`, bindingID); err != nil {
			return err
		}
		for i, it := range items {
			if _, err := tx.Exec(ctx, `
				INSERT INTO rules (binding_id, name, enabled, position, spec)
				VALUES ($1, $2, $3, $4, $5)`,
				bindingID, it.name, it.enabled, i, it.body); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return fmt.Errorf("store: 替换规则失败: %w", err)
	}
	return nil
}
