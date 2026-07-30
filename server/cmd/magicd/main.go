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
)

const usage = `magicd —— 神奇弹幕服务端

用法:
  magicd login [-o cookie.txt]
        扫码登录，把 Cookie 写入文件（默认输出到标准输出）

  magicd probe -room <房间号> [-cookie-file cookie.txt] [-type <事件类型>]
        连接直播间并打印实时事件流

示例:
  magicd login -o cookie.txt
  magicd probe -room 21452505 -cookie-file cookie.txt
  magicd probe -room 21452505 -cookie-file cookie.txt -type danmaku,gift
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
