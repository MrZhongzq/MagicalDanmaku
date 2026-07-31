package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"text/tabwriter"
)

const userUsage = `用法:
  magicd user add <用户名> [--admin]   创建用户，交互式设置密码
  magicd user passwd <用户名>          修改密码
  magicd user list                     列出全部用户
`

// runUser 分发 user 的子命令。
func runUser(args []string) error {
	if len(args) == 0 {
		fmt.Fprint(os.Stderr, userUsage)
		return fmt.Errorf("user 需要一个子命令")
	}
	switch args[0] {
	case "add":
		return runUserAdd(args[1:])
	case "passwd":
		return runUserPasswd(args[1:])
	case "list":
		return runUserList(args[1:])
	default:
		fmt.Fprint(os.Stderr, userUsage)
		return fmt.Errorf("未知的 user 子命令: %s", args[0])
	}
}

func runUserAdd(args []string) error {
	fs := flag.NewFlagSet("user add", flag.ExitOnError)
	dsn := addDBFlag(fs)
	admin := fs.Bool("admin", false, "创建为管理员，绕过全部权限检查")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 1 {
		return fmt.Errorf("用法: magicd user add <用户名> [--admin]")
	}
	username := fs.Arg(0)

	pass, err := readPassword(fmt.Sprintf("为 %s 设置密码: ", username))
	if err != nil {
		return err
	}
	again, err := readPassword("再输入一次: ")
	if err != nil {
		return err
	}
	if pass != again {
		return fmt.Errorf("两次输入的密码不一致")
	}

	ctx := context.Background()
	s, err := openStoreChecked(ctx, *dsn)
	if err != nil {
		return err
	}
	defer s.Close()

	u, err := s.CreateUser(ctx, username, pass, *admin)
	if err != nil {
		return err
	}
	role := "普通用户"
	if u.IsAdmin {
		role = "管理员"
	}
	fmt.Printf("已创建%s %s（ID %d）\n", role, u.Username, u.ID)
	return nil
}

func runUserPasswd(args []string) error {
	fs := flag.NewFlagSet("user passwd", flag.ExitOnError)
	dsn := addDBFlag(fs)
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 1 {
		return fmt.Errorf("用法: magicd user passwd <用户名>")
	}
	username := fs.Arg(0)

	pass, err := readPassword(fmt.Sprintf("为 %s 设置新密码: ", username))
	if err != nil {
		return err
	}
	again, err := readPassword("再输入一次: ")
	if err != nil {
		return err
	}
	if pass != again {
		return fmt.Errorf("两次输入的密码不一致")
	}

	ctx := context.Background()
	s, err := openStoreChecked(ctx, *dsn)
	if err != nil {
		return err
	}
	defer s.Close()

	if err := s.SetPassword(ctx, username, pass); err != nil {
		return err
	}
	fmt.Printf("%s 的密码已修改\n", username)
	return nil
}

func runUserList(args []string) error {
	fs := flag.NewFlagSet("user list", flag.ExitOnError)
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

	us, err := s.ListUsers(ctx)
	if err != nil {
		return err
	}
	if len(us) == 0 {
		fmt.Println("还没有任何用户。运行 magicd migrate 会创建首个管理员。")
		return nil
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "ID\t用户名\t角色\t创建时间")
	for _, u := range us {
		role := "普通用户"
		if u.IsAdmin {
			role = "管理员"
		}
		fmt.Fprintf(w, "%d\t%s\t%s\t%s\n",
			u.ID, u.Username, role, u.CreatedAt.Format("2006-01-02 15:04"))
	}
	return w.Flush()
}
