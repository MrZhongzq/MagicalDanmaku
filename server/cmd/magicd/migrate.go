package main

import (
	"context"
	"flag"
	"fmt"
)

// runMigrate 建表或升级 schema，并在空库上创建首个管理员。
func runMigrate(args []string) error {
	fs := flag.NewFlagSet("migrate", flag.ExitOnError)
	dsn := addDBFlag(fs)
	if err := fs.Parse(args); err != nil {
		return err
	}

	ctx := context.Background()
	// 用 openStore 而非 openStoreChecked：migrate 正是用来消除版本落后的
	s, err := openStore(ctx, *dsn)
	if err != nil {
		return err
	}
	defer s.Close()

	before, err := s.SchemaVersion(ctx)
	if err != nil {
		return err
	}
	if err := s.Migrate(ctx); err != nil {
		return err
	}
	after, err := s.SchemaVersion(ctx)
	if err != nil {
		return err
	}

	if before == after {
		fmt.Printf("schema 已是最新版本（v%d），无需迁移\n", after)
	} else {
		fmt.Printf("schema 已从 v%d 升级到 v%d\n", before, after)
	}

	// 造数据与改表分开：migrate 可以反复跑，建管理员只在空库上发生一次
	name, pass, created, err := s.EnsureAdmin(ctx)
	if err != nil {
		return err
	}
	if created {
		fmt.Println()
		fmt.Println("已创建管理员账户，密码只显示这一次，请立即保存：")
		fmt.Printf("  用户名: %s\n", name)
		fmt.Printf("  密码:   %s\n", pass)
		fmt.Println()
		fmt.Printf("改密码：magicd user passwd %s\n", name)
	}
	return nil
}
