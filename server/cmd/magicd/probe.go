package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"strings"

	"github.com/MrZhongzq/MagicalDanmaku/server/internal/connector/bilibili"
	"github.com/MrZhongzq/MagicalDanmaku/server/internal/connector/bilibili/api"
	"github.com/MrZhongzq/MagicalDanmaku/server/internal/connector/bilibili/auth"
	"github.com/MrZhongzq/MagicalDanmaku/server/internal/event"
)

// runProbe 连接直播间并打印事件流。
func runProbe(args []string) error {
	fs := flag.NewFlagSet("probe", flag.ExitOnError)
	room := fs.String("room", "", "直播间号（必填）")
	cookieFile := fs.String("cookie-file", "", "Cookie 文件路径；留空则匿名连接")
	typeFilter := fs.String("type", "", "只显示指定事件类型，逗号分隔，如 danmaku,gift")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *room == "" {
		return errors.New("必须通过 -room 指定直播间号")
	}

	var sess *auth.Session
	if *cookieFile != "" {
		b, err := os.ReadFile(*cookieFile)
		if err != nil {
			return fmt.Errorf("读取 Cookie 文件失败: %w", err)
		}
		sess, err = auth.ParseSession(strings.TrimSpace(string(b)))
		if err != nil {
			return err
		}
		fmt.Printf("已加载账号 UID=%s\n", sess.UID)
	} else {
		fmt.Println("未提供 Cookie，将以匿名身份连接（部分事件字段会缺失）")
	}

	allow := parseTypeFilter(*typeFilter)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	apiClient := api.New(sess)

	// 先解析真实房间号：用户可能填的是短号。
	if err := apiClient.RefreshNav(ctx); err != nil {
		return fmt.Errorf("初始化签名失败: %w", err)
	}
	info, err := apiClient.RoomInfo(ctx, *room)
	if err != nil {
		return err
	}
	status := "未开播"
	if info.IsLiving() {
		status = "直播中"
	}
	fmt.Printf("直播间 %s（%s）标题：%s  状态：%s\n\n",
		info.RoomID, info.ParentAreaName+"/"+info.AreaName, info.Title, status)

	c := bilibili.NewClient(info.RoomID, apiClient)

	go func() {
		for ev := range c.Events() {
			if allow != nil && !allow[ev.Type] {
				continue
			}
			fmt.Println(Render(ev))
		}
	}()

	err = c.Run(ctx)
	if errors.Is(err, context.Canceled) {
		fmt.Println("\n已断开连接")
		return nil
	}
	return err
}

// parseTypeFilter 解析 -type 参数，返回 nil 表示不过滤。
func parseTypeFilter(s string) map[event.Type]bool {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil
	}
	m := make(map[event.Type]bool)
	for _, part := range strings.Split(s, ",") {
		if part = strings.TrimSpace(part); part != "" {
			m[event.Type(part)] = true
		}
	}
	if len(m) == 0 {
		return nil
	}
	return m
}
