package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"time"

	"github.com/MrZhongzq/MagicalDanmaku/server/internal/connector/bilibili/auth"
	"github.com/MrZhongzq/MagicalDanmaku/server/internal/store"
)

// runLogin 执行扫码登录流程。
//
// 两种去处：--save 直接写进数据库（推荐），-o 写文件（YAML 导入路径用）。
func runLogin(args []string) error {
	fs := flag.NewFlagSet("login", flag.ExitOnError)
	out := fs.String("o", "", "把 Cookie 写入指定文件；留空则打印到标准输出")
	save := fs.String("save", "", "把 Cookie 直接存进数据库，值为账号名")
	owner := fs.String("owner", "", "新账号归属的用户名，与 --save 搭配")
	dsn := addDBFlag(fs)
	if err := fs.Parse(args); err != nil {
		return err
	}

	// 参数校验放在扫码之前：让人扫完码才发现参数错了最气人
	if *save != "" && *out != "" {
		return fmt.Errorf("--save 与 -o 不能同时使用：前者存数据库，后者写文件，选一个")
	}
	if *owner != "" && *save == "" {
		return fmt.Errorf("--owner 要与 --save 搭配使用")
	}

	// 数据库也先连上再扫码，同理
	var st *store.Store
	if *save != "" {
		var err error
		st, err = openStoreChecked(context.Background(), *dsn)
		if err != nil {
			return err
		}
		defer st.Close()

		// 已存在的账号只换 Cookie，不需要 owner；新账号必须指定
		if _, err := st.GetAccountByName(context.Background(), *save); err != nil {
			if !errors.Is(err, store.ErrNotFound) {
				return err
			}
			if *owner == "" {
				return fmt.Errorf("账号 %q 还不存在，创建它需要用 --owner 指定归属的用户", *save)
			}
			if _, err := st.GetUserByName(context.Background(), *owner); err != nil {
				return err
			}
		}
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	l := auth.NewQRLogin(nil)
	qr, err := l.Generate(ctx)
	if err != nil {
		return err
	}

	fmt.Println("请用哔哩哔哩手机客户端扫描下方二维码：")
	fmt.Println()
	art, err := renderQR(qr.URL)
	if err != nil {
		// 渲染失败不阻断登录：地址本身仍可用。
		fmt.Println("（二维码渲染失败，请手动打开下面的地址）")
	} else {
		fmt.Print(art)
	}
	fmt.Println()
	fmt.Println("若终端显示错乱，可复制以下地址到手机浏览器或二维码生成器：")
	fmt.Println("   " + qr.URL)
	fmt.Println()
	fmt.Println("等待扫码中，按 Ctrl+C 取消...")

	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	last := auth.PollWaiting
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
		}

		res, err := l.Poll(ctx, qr.Key)
		if err != nil {
			return err
		}
		if res.Status != last {
			switch res.Status {
			case auth.PollScanned:
				fmt.Println("已扫码，请在手机上确认登录...")
			case auth.PollExpired:
				return fmt.Errorf("二维码已失效，请重新运行 login")
			}
			last = res.Status
		}
		if res.Status != auth.PollSuccess {
			continue
		}

		// 校验拿到的 Cookie 可用
		sess, err := auth.ParseSession(res.Cookie)
		if err != nil {
			return fmt.Errorf("登录成功但 Cookie 不完整: %w", err)
		}
		fmt.Printf("登录成功，UID=%s\n", sess.UID)

		switch {
		case st != nil:
			return saveAccountCookie(ctx, st, *save, *owner, res.Cookie, sess.UID)
		case *out == "":
			fmt.Println(res.Cookie)
			return nil
		default:
			// 0600 权限：Cookie 等同于账号密码，不得让同机其他用户读到。
			if err := os.WriteFile(*out, []byte(res.Cookie), 0o600); err != nil {
				return fmt.Errorf("写入 %s 失败: %w", *out, err)
			}
			fmt.Printf("Cookie 已写入 %s\n", *out)
			return nil
		}
	}
}

// saveAccountCookie 把 Cookie 存进数据库：账号已存在就换 Cookie，
// 否则新建。
func saveAccountCookie(ctx context.Context, st *store.Store, name, owner, cookie, uid string) error {
	if _, err := st.GetAccountByName(ctx, name); err == nil {
		if err := st.UpdateAccountCookie(ctx, name, cookie, uid); err != nil {
			return err
		}
		fmt.Printf("账号 %q 的 Cookie 已更新\n", name)
		return nil
	} else if !errors.Is(err, store.ErrNotFound) {
		return err
	}

	u, err := st.GetUserByName(ctx, owner)
	if err != nil {
		return err
	}
	if _, err := st.CreateAccount(ctx, store.AccountInput{
		Name: name, UID: uid, Cookie: cookie, OwnerID: u.ID,
	}); err != nil {
		return err
	}
	fmt.Printf("已创建账号 %q，归属用户 %s\n", name, u.Username)
	fmt.Printf("下一步：magicd binding add %s <房间号>\n", name)
	return nil
}
