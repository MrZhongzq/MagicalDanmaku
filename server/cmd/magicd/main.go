// Command magicd 是神奇弹幕的服务端可执行文件。
//
// 配置的唯一真相是 PostgreSQL。YAML 只是导入入口，run 不读它。
package main

import (
	"fmt"
	"os"

	"github.com/MrZhongzq/MagicalDanmaku/server/internal/buildinfo"
	"github.com/MrZhongzq/MagicalDanmaku/server/internal/logging"
)

const usage = `magicd —— 神奇弹幕服务端

配置存在 PostgreSQL 里。先设置连接串：
  export MAGICD_DATABASE_URL='postgres://magicd:magicd@localhost:5432/magicd?sslmode=disable'

初次使用:
  magicd migrate                              建表，并在空库上创建管理员
  magicd login --save 小号 --owner admin       扫码登录一个 B 站账号并入库
  magicd binding add 小号 1706666491           让这个账号连接一个直播间
  magicd import -c config.yaml --owner admin   或者：直接导入现成的 YAML
  magicd run                                   启动机器人

用户与授权:
  magicd user add <用户名> [--admin]           创建用户
  magicd user passwd <用户名>                  修改密码
  magicd user list                             列出用户
  magicd grant <用户名> <账号名@房间号> <权限点,...>
  magicd revoke <用户名> <账号名@房间号>
  magicd perms <用户名>                        查看某人的授权
  magicd can <用户名> <账号名@房间号> <权限点>   检查某人有没有某个权限
  magicd grant -list                           列出全部权限点

账号与绑定:
  magicd account list                          列出 B 站账号
  magicd binding add <账号名> <房间号>
  magicd binding list
  magicd binding rm|enable|disable <账号名@房间号>

排障:
  magicd login [-o cookie.txt]                 扫码登录，Cookie 写文件（YAML 路径用）
  magicd probe -room <房间号> [-cookie-file cookie.txt] [-type <事件类型>]
                                [-dump <CMD名>] [-dump-file dump.jsonl]
        连接直播间并打印实时事件流；-dump 可把指定 CMD 的原始 JSON 落盘
  magicd roominfo -room <房间号> [-cookie-file cookie.txt]
        查一次房间人数 / 大航海总数 / 大航海在线数，用来验证这三个接口还能不能用
  magicd version                               显示版本信息

环境变量:
  MAGICD_DATABASE_URL        PostgreSQL 连接串
  MAGICD_ADMIN_PASSWORD      空库首次 migrate 建管理员时用的密码，必填
                             （至少 8 位）；库里已有管理员时不生效，
                             不能拿它改已有密码
  MAGICD_LOG_LEVEL           debug / info / warn / error，默认 info
  MAGICD_LOG_FILE            系统日志文件路径，留空则只写 stderr
  MAGICD_LOG_RETENTION_DAYS  业务日志保留天数，默认 30，0 表示不清理
  MAGICD_HTTP_ADDR           Web 管理界面的监听地址，默认 127.0.0.1:8080
                             （只监听本机；Docker 部署需设为 0.0.0.0:8080，
                              设为空串或 off 则不启动 Web 服务）
  MAGICD_HTTP_SECURE_COOKIE  设为 1 时会话 Cookie 带 Secure 标志，
                             反向代理加了 TLS 时才打开
`

func main() {
	if len(os.Args) < 2 {
		fmt.Fprint(os.Stderr, usage)
		os.Exit(2)
	}

	// 系统日志在分发子命令之前装配好，这样连接失败之类的错误也有去处
	closer, err := logging.SetupSystem(logging.SystemOptionsFromEnv())
	if err != nil {
		fmt.Fprintf(os.Stderr, "错误: %v\n", err)
		os.Exit(1)
	}
	defer func() { _ = closer.Close() }()

	switch os.Args[1] {
	case "login":
		err = runLogin(os.Args[2:])
	case "probe":
		err = runProbe(os.Args[2:])
	case "roominfo":
		err = runRoomInfo(os.Args[2:])
	case "run":
		err = runRun(os.Args[2:])
	case "migrate":
		err = runMigrate(os.Args[2:])
	case "import":
		err = runImport(os.Args[2:])
	case "user":
		err = runUser(os.Args[2:])
	case "account":
		err = runAccount(os.Args[2:])
	case "binding":
		err = runBinding(os.Args[2:])
	case "grant":
		err = runGrant(os.Args[2:])
	case "revoke":
		err = runRevoke(os.Args[2:])
	case "perms":
		err = runPerms(os.Args[2:])
	case "can":
		err = runCan(os.Args[2:])
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
