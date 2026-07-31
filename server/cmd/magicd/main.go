// Command magicd 是神奇弹幕的服务端可执行文件。
//
// P0 阶段提供两个子命令：
//
//	login  扫码登录，输出 Cookie
//	probe  连接直播间并打印实时事件流
package main

import (
	"fmt"
	"os"

	"github.com/MrZhongzq/MagicalDanmaku/server/internal/buildinfo"
)

const usage = `magicd —— 神奇弹幕服务端

用法:
  magicd login [-o cookie.txt]
        扫码登录，把 Cookie 写入文件（默认输出到标准输出）

  magicd probe -room <房间号> [-cookie-file cookie.txt] [-type <事件类型>]
                             [-dump <CMD名>] [-dump-file dump.jsonl]
        连接直播间并打印实时事件流；-dump 可把指定 CMD 的原始 JSON 落盘，
        用于采集样本、补写映射

  magicd run -c config.yaml
        按配置文件运行弹幕机器人。配置为三层结构：账号 → 直播间 → 规则，
        每个「账号-直播间」组合独立运行，互不干扰

  magicd version
        显示版本信息

示例:
  magicd login -o cookie.txt
  magicd probe -room 21452505 -cookie-file cookie.txt
  magicd probe -room 21452505 -cookie-file cookie.txt -type danmaku,gift
  magicd probe -room 21452505 -cookie-file cookie.txt -dump unknown
  magicd run -c config.yaml
`

func main() {
	if len(os.Args) < 2 {
		fmt.Fprint(os.Stderr, usage)
		os.Exit(2)
	}

	var err error
	switch os.Args[1] {
	case "login":
		err = runLogin(os.Args[2:])
	case "probe":
		err = runProbe(os.Args[2:])
	case "run":
		err = runRun(os.Args[2:])
	case "version", "-v", "--version":
		fmt.Println(buildinfo.Get().Detail())
		return
	case "-h", "--help", "help":
		fmt.Print(usage)
		return
	default:
		fmt.Fprintf(os.Stderr, "未知的子命令: %s\n\n%s", os.Args[1], usage)
		os.Exit(2)
	}

	if err != nil {
		fmt.Fprintf(os.Stderr, "错误: %v\n", err)
		os.Exit(1)
	}
}
