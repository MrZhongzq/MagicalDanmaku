package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"time"

	"github.com/MrZhongzq/MagicalDanmaku/server/internal/connector/bilibili/auth"
)

// runLogin 执行扫码登录流程。
func runLogin(args []string) error {
	fs := flag.NewFlagSet("login", flag.ExitOnError)
	out := fs.String("o", "", "把 Cookie 写入指定文件；留空则打印到标准输出")
	if err := fs.Parse(args); err != nil {
		return err
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

		if *out == "" {
			fmt.Println(res.Cookie)
			return nil
		}
		// 0600 权限：Cookie 等同于账号密码，不得让同机其他用户读到。
		if err := os.WriteFile(*out, []byte(res.Cookie), 0o600); err != nil {
			return fmt.Errorf("写入 %s 失败: %w", *out, err)
		}
		fmt.Printf("Cookie 已写入 %s\n", *out)
		return nil
	}
}
