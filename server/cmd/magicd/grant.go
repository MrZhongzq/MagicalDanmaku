package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strings"
	"text/tabwriter"

	"github.com/MrZhongzq/MagicalDanmaku/server/internal/perm"
)

// runGrant 授予某用户对某绑定的权限点。
func runGrant(args []string) error {
	fs := flag.NewFlagSet("grant", flag.ExitOnError)
	dsn := addDBFlag(fs)
	list := fs.Bool("list", false, "列出全部合法权限点后退出")
	if err := fs.Parse(args); err != nil {
		return err
	}

	if *list {
		fmt.Println("合法的权限点：")
		for _, p := range perm.All() {
			fmt.Printf("  %s\n", p)
		}
		return nil
	}

	if fs.NArg() != 3 {
		return fmt.Errorf("用法: magicd grant <用户名> <账号名@房间号> <权限点,...>\n" +
			"权限点清单: magicd grant -list")
	}
	username := fs.Arg(0)
	accName, roomID, err := parseBindingRef(fs.Arg(1))
	if err != nil {
		return err
	}
	ps, err := perm.ParseList(fs.Arg(2))
	if err != nil {
		return err
	}

	ctx := context.Background()
	s, err := openStoreChecked(ctx, *dsn)
	if err != nil {
		return err
	}
	defer s.Close()

	if err := s.Grant(ctx, username, accName, roomID, ps); err != nil {
		return err
	}
	// 说明是替换而非累加，避免用户以为原有权限还在
	fmt.Printf("%s 在 %s@%s 上的权限已设为: %s\n",
		username, accName, roomID, strings.Join(perm.Strings(ps), ", "))
	return nil
}

// runRevoke 撤销某用户对某绑定的全部权限。
func runRevoke(args []string) error {
	fs := flag.NewFlagSet("revoke", flag.ExitOnError)
	dsn := addDBFlag(fs)
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 2 {
		return fmt.Errorf("用法: magicd revoke <用户名> <账号名@房间号>")
	}
	username := fs.Arg(0)
	accName, roomID, err := parseBindingRef(fs.Arg(1))
	if err != nil {
		return err
	}

	ctx := context.Background()
	s, err := openStoreChecked(ctx, *dsn)
	if err != nil {
		return err
	}
	defer s.Close()

	if err := s.Revoke(ctx, username, accName, roomID); err != nil {
		return err
	}
	fmt.Printf("已撤销 %s 在 %s@%s 上的全部权限\n", username, accName, roomID)
	return nil
}

// runPerms 列出某用户已有的全部授权。
func runPerms(args []string) error {
	fs := flag.NewFlagSet("perms", flag.ExitOnError)
	dsn := addDBFlag(fs)
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 1 {
		return fmt.Errorf("用法: magicd perms <用户名>")
	}

	ctx := context.Background()
	s, err := openStoreChecked(ctx, *dsn)
	if err != nil {
		return err
	}
	defer s.Close()

	u, err := s.GetUserByName(ctx, fs.Arg(0))
	if err != nil {
		return err
	}
	if u.IsAdmin {
		fmt.Printf("%s 是管理员，对全部绑定拥有全部权限\n", u.Username)
		return nil
	}

	ms, err := s.ListMemberships(ctx, u.Username)
	if err != nil {
		return err
	}
	if len(ms) == 0 {
		fmt.Printf("%s 还没有任何授权\n", u.Username)
		return nil
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "绑定\t权限点")
	for _, m := range ms {
		fmt.Fprintf(w, "%s@%s\t%s\n",
			m.AccountName, m.RoomID, strings.Join(perm.Strings(m.Permissions), ", "))
	}
	return w.Flush()
}

// runCan 检查某用户对某绑定是否拥有某个权限点。
//
// 排障用：「为什么李四改不了规则」这类问题，直接问一句比对着
// perms 的输出人肉比对可靠。
func runCan(args []string) error {
	fs := flag.NewFlagSet("can", flag.ExitOnError)
	dsn := addDBFlag(fs)
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 3 {
		return fmt.Errorf("用法: magicd can <用户名> <账号名@房间号> <权限点>")
	}
	accName, roomID, err := parseBindingRef(fs.Arg(1))
	if err != nil {
		return err
	}
	p, err := perm.Parse(fs.Arg(2))
	if err != nil {
		return err
	}

	ctx := context.Background()
	s, err := openStoreChecked(ctx, *dsn)
	if err != nil {
		return err
	}
	defer s.Close()

	u, err := s.GetUserByName(ctx, fs.Arg(0))
	if err != nil {
		return err
	}
	b, err := s.GetBinding(ctx, accName, roomID)
	if err != nil {
		return err
	}

	ok, err := s.Can(ctx, u.ID, b.ID, p)
	if err != nil {
		return err
	}
	if ok {
		reason := ""
		if u.IsAdmin {
			reason = "（管理员绕过全部检查）"
		}
		fmt.Printf("是：%s 在 %s 上拥有 %s%s\n", u.Username, b.Label(), p, reason)
		return nil
	}
	fmt.Printf("否：%s 在 %s 上没有 %s\n", u.Username, b.Label(), p)
	fmt.Printf("授予：magicd grant %s %s %s\n", u.Username, b.Label(), p)
	return nil
}
