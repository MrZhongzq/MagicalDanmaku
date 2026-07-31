package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"text/tabwriter"
)

const bindingUsage = `用法:
  magicd binding add <账号名> <房间号>   让账号连接一个直播间
  magicd binding list                    列出全部绑定
  magicd binding rm <账号名@房间号>      删除绑定及其规则
  magicd binding enable  <账号名@房间号>
  magicd binding disable <账号名@房间号>
`

// runBinding 分发 binding 的子命令。
func runBinding(args []string) error {
	if len(args) == 0 {
		fmt.Fprint(os.Stderr, bindingUsage)
		return fmt.Errorf("binding 需要一个子命令")
	}
	switch args[0] {
	case "add":
		return runBindingAdd(args[1:])
	case "list":
		return runBindingList(args[1:])
	case "rm":
		return runBindingRemove(args[1:])
	case "enable":
		return runBindingSetEnabled(args[1:], true)
	case "disable":
		return runBindingSetEnabled(args[1:], false)
	default:
		fmt.Fprint(os.Stderr, bindingUsage)
		return fmt.Errorf("未知的 binding 子命令: %s", args[0])
	}
}

func runBindingAdd(args []string) error {
	fs := flag.NewFlagSet("binding add", flag.ExitOnError)
	dsn := addDBFlag(fs)
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 2 {
		return fmt.Errorf("用法: magicd binding add <账号名> <房间号>")
	}

	ctx := context.Background()
	s, err := openStoreChecked(ctx, *dsn)
	if err != nil {
		return err
	}
	defer s.Close()

	acc, err := s.GetAccountByName(ctx, fs.Arg(0))
	if err != nil {
		return err
	}
	b, err := s.UpsertBinding(ctx, acc.ID, fs.Arg(1))
	if err != nil {
		return err
	}
	fmt.Printf("已添加绑定 %s（ID %d）\n", b.Label(), b.ID)
	return nil
}

func runBindingList(args []string) error {
	fs := flag.NewFlagSet("binding list", flag.ExitOnError)
	dsn := addDBFlag(fs)
	if err := fs.Parse(args); err != nil {
		return err
	}

	ctx := context.Background()
	s, err := openStoreChecked(ctx, *dsn)
	if err != nil {
		return err
	}
	defer s.Close()

	bs, err := s.ListBindings(ctx)
	if err != nil {
		return err
	}
	if len(bs) == 0 {
		fmt.Println("还没有任何绑定。先 magicd login --save <账号名>，再 magicd binding add <账号名> <房间号>。")
		return nil
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "绑定\t状态\t规则数")
	for _, b := range bs {
		status := "启用"
		if !b.Enabled {
			status = "停用"
		}
		rs, err := s.ListRules(ctx, b.ID)
		if err != nil {
			return err
		}
		fmt.Fprintf(w, "%s\t%s\t%d\n", b.Label(), status, len(rs))
	}
	return w.Flush()
}

func runBindingRemove(args []string) error {
	fs := flag.NewFlagSet("binding rm", flag.ExitOnError)
	dsn := addDBFlag(fs)
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 1 {
		return fmt.Errorf("用法: magicd binding rm <账号名@房间号>")
	}
	name, room, err := parseBindingRef(fs.Arg(0))
	if err != nil {
		return err
	}

	ctx := context.Background()
	s, err := openStoreChecked(ctx, *dsn)
	if err != nil {
		return err
	}
	defer s.Close()

	if err := s.DeleteBinding(ctx, name, room); err != nil {
		return err
	}
	fmt.Printf("已删除绑定 %s@%s 及其规则\n", name, room)
	return nil
}

func runBindingSetEnabled(args []string, enabled bool) error {
	verb := "停用"
	if enabled {
		verb = "启用"
	}
	fs := flag.NewFlagSet("binding "+verb, flag.ExitOnError)
	dsn := addDBFlag(fs)
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 1 {
		return fmt.Errorf("用法: magicd binding enable|disable <账号名@房间号>")
	}
	name, room, err := parseBindingRef(fs.Arg(0))
	if err != nil {
		return err
	}

	ctx := context.Background()
	s, err := openStoreChecked(ctx, *dsn)
	if err != nil {
		return err
	}
	defer s.Close()

	if err := s.SetBindingEnabled(ctx, name, room, enabled); err != nil {
		return err
	}
	fmt.Printf("已%s绑定 %s@%s\n", verb, name, room)
	return nil
}

// runAccountList 列出全部 B 站账号。不打印 Cookie。
func runAccountList(args []string) error {
	fs := flag.NewFlagSet("account list", flag.ExitOnError)
	dsn := addDBFlag(fs)
	if err := fs.Parse(args); err != nil {
		return err
	}

	ctx := context.Background()
	s, err := openStoreChecked(ctx, *dsn)
	if err != nil {
		return err
	}
	defer s.Close()

	as, err := s.ListAccounts(ctx)
	if err != nil {
		return err
	}
	if len(as) == 0 {
		fmt.Println("还没有任何 B 站账号。运行 magicd login --save <账号名> --owner <用户名>。")
		return nil
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "账号名\tUID\t发送间隔\t字数上限")
	for _, a := range as {
		// 不打印 Cookie：它等同于账号密码，不该出现在终端回滚缓冲里
		fmt.Fprintf(w, "%s\t%s\t%v\t%d\n", a.Name, a.UID, a.RateLimit, a.MaxLength)
	}
	return w.Flush()
}

// runAccount 分发 account 的子命令。
func runAccount(args []string) error {
	if len(args) == 0 || args[0] != "list" {
		fmt.Fprintln(os.Stderr, "用法: magicd account list")
		return fmt.Errorf("account 目前只支持 list 子命令")
	}
	return runAccountList(args[1:])
}
