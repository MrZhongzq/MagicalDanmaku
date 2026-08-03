package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/MrZhongzq/MagicalDanmaku/server/internal/connector/bilibili/api"
	"github.com/MrZhongzq/MagicalDanmaku/server/internal/connector/bilibili/auth"
)

// runRoomInfo 对任意直播间跑一次「对面快照」用到的三个接口并打印结果。
//
// 为什么单独做一个子命令：PK 期间抓对面数据的那三个接口
// （房间人数 / 大航海总数 / 大航海在线数）本身只是对任意房间号的普通查询，
// 跟 PK 没有任何关系——但它们在代码里唯一的调用点是 PK 编排。
// 结果就是想验证「接口还能不能用」必须先凑一场真 PK，太被动了。
//
// 这三个接口的参数全部来自原 Qt/C++ 项目，而那份代码里同一功能留了三个
// 版本、只有最后一个还在跑（前两个的接口早已失效）。所以「能跑通」这件事
// 必须能随时廉价地验证，而不是等到 PK 打起来才发现拿不到数据。
func runRoomInfo(args []string) error {
	fs := flag.NewFlagSet("roominfo", flag.ExitOnError)
	room := fs.String("room", "", "直播间号（必填，短号长号都行）")
	cookieFile := fs.String("cookie-file", "", "Cookie 文件路径；留空则匿名查询")
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
		fmt.Println("未提供 Cookie，将以匿名身份查询")
	}

	client := api.New(sess)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// 先取房间信息：三个接口里有两个需要主播 UID，而且传进来的可能是短号，
	// 必须换成长号——PK 报文里的 room_id 也一律是长号。
	info, err := client.RoomInfo(ctx, *room)
	if err != nil {
		return fmt.Errorf("获取直播间信息失败: %w", err)
	}
	fmt.Printf("\n直播间 %s（短号 %q）主播 UID=%s\n", info.RoomID, info.ShortID, info.UID)
	fmt.Printf("  标题：%s\n", info.Title)
	fmt.Printf("  开播中：%v\n\n", info.LiveStatus == 1)

	// 三个接口各自独立打印成功/失败。任何一个挂掉都不影响另外两个的结论——
	// 线上的降级语义也是这样：拿不到就留空，不是整批放弃。
	fail := 0

	online, err := client.RoomOnlineCount(ctx, info.RoomID)
	if err != nil {
		fmt.Printf("✗ 房间人数（getInfoByRoom）失败：%v\n", err)
		fail++
	} else {
		fmt.Printf("✓ 房间人数：%d\n", online)
	}

	total, err := client.GuardTotal(ctx, info.RoomID, info.UID)
	if err != nil {
		fmt.Printf("✗ 大航海总数（guardTab/topList）失败：%v\n", err)
		fail++
	} else {
		fmt.Printf("✓ 大航海总数：%d\n", total)
	}

	g, err := client.GuardOnline(ctx, info.RoomID, info.UID)
	if err != nil {
		fmt.Printf("✗ 大航海在线数（queryContributionRank）失败：%v\n", err)
		fail++
	} else {
		fmt.Printf("✓ 大航海在线数：%d（总督 %d / 提督 %d / 舰长 %d）\n",
			g.Total(), g.Governor, g.Admiral, g.Captain)
	}

	fmt.Println()
	if fail > 0 {
		return fmt.Errorf("%d 个接口不可用（详见上面的 ✗）", fail)
	}
	fmt.Println("三个接口都可用。")
	return nil
}
