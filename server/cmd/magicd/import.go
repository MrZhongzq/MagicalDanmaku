package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"gopkg.in/yaml.v3"

	"github.com/MrZhongzq/MagicalDanmaku/server/internal/rules/config"
	"github.com/MrZhongzq/MagicalDanmaku/server/internal/rules/spec"
	"github.com/MrZhongzq/MagicalDanmaku/server/internal/store"
)

// runImport 把 YAML 配置导入数据库。
//
// 数据库是唯一真相，YAML 只是导入入口。run 不读 YAML。
func runImport(args []string) error {
	fs := flag.NewFlagSet("import", flag.ExitOnError)
	dsn := addDBFlag(fs)
	cfgPath := fs.String("c", "", "YAML 配置文件路径（必填）")
	owner := fs.String("owner", "", "导入的账号归属于哪个用户（必填）")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *cfgPath == "" {
		return fmt.Errorf("必须用 -c 指定配置文件")
	}
	if *owner == "" {
		return fmt.Errorf("必须用 -owner 指定账号归属的用户，例如 -owner admin")
	}

	// 先用 config.Load 走一遍完整校验：它会把「账号名重复」「未知事件类型」
	// 之类的问题报成人能读懂的错误，而不是等写库时才炸。
	if _, err := config.Load(*cfgPath); err != nil {
		return err
	}

	// 再解析一次拿规则的原始序列化形式。规则要以 spec.Rule 入库，而
	// config.Load 返回的是转换后的领域模型；写一个反向转换会成为第二处
	// 字段展开，正是要避免的。多解析一次的代价可以忽略。
	raw, err := os.ReadFile(*cfgPath)
	if err != nil {
		return fmt.Errorf("读取配置文件 %s 失败: %w", *cfgPath, err)
	}
	var sc spec.Config
	if err := yaml.Unmarshal(raw, &sc); err != nil {
		return fmt.Errorf("解析配置文件 %s 失败: %w", *cfgPath, err)
	}

	accounts := make([]store.ImportAccount, 0, len(sc.Accounts))
	for _, a := range sc.Accounts {
		cookie, err := readCookieFile(a.CookieFile)
		if err != nil {
			return fmt.Errorf("账号 %q: %w", a.Name, err)
		}

		rooms := make([]store.ImportRoom, 0, len(a.Rooms))
		for _, r := range a.Rooms {
			groups := make(map[string]time.Duration, len(r.CooldownGroups))
			for k, v := range r.CooldownGroups {
				groups[k] = time.Duration(v)
			}
			rooms = append(rooms, store.ImportRoom{
				RoomID:         r.ID,
				CooldownGroups: groups,
				Rules:          r.Rules,
			})
		}

		accounts = append(accounts, store.ImportAccount{
			Name:      a.Name,
			Cookie:    cookie,
			RateLimit: time.Duration(a.RateLimit),
			MaxLength: a.MaxLength,
			Rooms:     rooms,
		})
	}

	ctx := context.Background()
	s, err := openStoreChecked(ctx, *dsn)
	if err != nil {
		return err
	}
	defer s.Close()

	u, err := s.GetUserByName(ctx, *owner)
	if err != nil {
		return err
	}

	res, err := s.ImportConfig(ctx, u.ID, accounts)
	if err != nil {
		return err
	}

	fmt.Printf("已导入 %d 个账号、%d 个绑定、%d 条规则，归属用户 %s\n",
		res.Accounts, res.Bindings, res.Rules, u.Username)
	fmt.Println("数据库现在是配置的唯一真相，magicd run 不再需要这份 YAML。")
	return nil
}

// readCookieFile 读出 Cookie 文件的内容。
func readCookieFile(path string) (string, error) {
	if path == "" {
		return "", fmt.Errorf("配置里没写 cookieFile")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("读取 Cookie 文件 %s 失败: %w", path, err)
	}
	cookie := strings.TrimSpace(string(data))
	if cookie == "" {
		return "", fmt.Errorf("Cookie 文件 %s 是空的", path)
	}
	return cookie, nil
}
