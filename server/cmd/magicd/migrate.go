package main

import (
	"context"
	"flag"
	"fmt"
	"os"

	"github.com/MrZhongzq/MagicalDanmaku/server/internal/store"
)

// adminPasswordEnvVar 是无头部署下指定管理员初始密码的唯一入口。
//
// 只在空库首次建管理员时读取——见 resolveAdminPassword 与
// store.EnsureAdmin 的说明。
const adminPasswordEnvVar = "MAGICD_ADMIN_PASSWORD"

// rejectedAdminPasswords 是已知会被人误当作"随手填一个"抄进生产环境的
// 字面量。
//
// **Important-2**：`.env.example` 里 MAGICD_ADMIN_PASSWORD 的占位符是
// 「在这里填 openssl rand -base64 18 的输出」这句中文说明本身——它按
// 字节数远超 MinAdminPasswordLength（8），只靠长度校验挡不住
// `cp .env.example .env && docker compose up -d` 这条最常见的路径：
// 什么都没编辑，也会成功建出一个密码就是这句仓库里公开写着的字符串的
// 管理员，且没有任何报错。POSTGRES_PASSWORD 那边有个天然的强制纠错点
// （占位符含空格与中文，拼进连接串必然解析失败），管理员密码这条没有，
// 需要在这里显式堵上。
//
// 这与"强制必填"的取舍无关（那条已经认可），只堵占位符字面量本身，
// 以及几个业界公认的弱密码，防止它们被当作"够用了"的答案。
var rejectedAdminPasswords = map[string]bool{
	"在这里填 openssl rand -base64 18 的输出": true,
	"changeme": true,
	"password": true,
	"admin":    true,
	"12345678": true,
}

// resolveAdminPassword 决定这次 migrate 该用哪个密码去建管理员。
//
// existingUsers 非零时，EnsureAdmin 本来就是空操作（库里已经有管理员，
// 这个环境变量绝不能被拿来当改密码的后门），直接放行、返回空串，
// 调用方不必关心 MAGICD_ADMIN_PASSWORD 有没有设置。
//
// existingUsers 为零（真的要建管理员）时才是这个函数存在的意义：
// envPassword 为空就报错，而不是退回到"生成一个随机密码、只在标准
// 输出打印一次"的旧行为——那正是本任务要解决的真机反馈：无头部署下
// 密码只能 docker exec 进容器翻日志才能找到。**这里选择强制必填**，
// 理由：
//  1. 本项目已有先例——MAGICD_DATABASE_URL、docker-compose.yml 里的
//     POSTGRES_PASSWORD 都是"没有就报错，说清楚怎么设"，不是这条规则
//     的例外；
//  2. 无头部署原本就必须先设好 MAGICD_DATABASE_URL 才能跑任何子命令，
//     多设一个环境变量不是额外的新门槛；
//  3. 反过来"不设也能用"的代价更大：图省事的用户不会去读文档，会
//     直接继续吃这次要修的那个坑——docker exec 翻日志找一次性密码。
//
// 本地"随手跑一个实例、不在意密码"的场景不会被这条规则卡住：直接
// 在命令行内联传一个环境变量（`MAGICD_ADMIN_PASSWORD=xxx magicd migrate`）
// 比翻日志找密码省事得多，而 store.EnsureAdmin 本身仍然保留"password
// 传空串就生成随机密码"的通用能力，供其他调用方（以及本文件之外的
// 测试）在需要时使用——只是 migrate 这条 CLI 路径不再把这条能力暴露
// 给无头部署，避免默认值本身就是那个坑。
func resolveAdminPassword(existingUsers int, envPassword string) (string, error) {
	if existingUsers > 0 {
		return "", nil
	}
	if envPassword == "" {
		return "", fmt.Errorf("库是空的，需要创建管理员账户，但未设置环境变量 %s。\n"+
			"无头部署（Docker 等）下这是唯一能提前拿到初始密码的方式，避免密码"+
			"只在标准输出打印一次、之后只能翻日志找。\n"+
			"在 .env 里设置一个够强的密码（至少 %d 位），例如：\n"+
			"  %s=$(openssl rand -base64 18)\n"+
			"本地随手起一个实例、不在意密码的场景，也可以设成任意足够长的值。",
			adminPasswordEnvVar, store.MinAdminPasswordLength, adminPasswordEnvVar)
	}
	if rejectedAdminPasswords[envPassword] {
		return "", fmt.Errorf("环境变量 %s 的值是一个已知的占位符/弱密码（很可能是"+
			"复制了 .env.example 里的说明文字，却忘了替换成真正的密码），拒绝拿它建管理员。\n"+
			"在 .env 里设置一个真正随机的密码，例如：\n"+
			"  %s=$(openssl rand -base64 18)",
			adminPasswordEnvVar, adminPasswordEnvVar)
	}
	return envPassword, nil
}

// runMigrate 建表或升级 schema，并在空库上创建首个管理员。
func runMigrate(args []string) error {
	fs := flag.NewFlagSet("migrate", flag.ExitOnError)
	dsn := addDBFlag(fs)
	if err := fs.Parse(args); err != nil {
		return err
	}

	ctx := context.Background()
	// 用 openStore 而非 openStoreChecked：migrate 正是用来消除版本落后的
	s, err := openStore(ctx, *dsn)
	if err != nil {
		return err
	}
	defer s.Close()

	before, err := s.SchemaVersion(ctx)
	if err != nil {
		return err
	}
	if err := s.Migrate(ctx); err != nil {
		return err
	}
	after, err := s.SchemaVersion(ctx)
	if err != nil {
		return err
	}

	if before == after {
		fmt.Printf("schema 已是最新版本（v%d），无需迁移\n", after)
	} else {
		fmt.Printf("schema 已从 v%d 升级到 v%d\n", before, after)
	}

	// 造数据与改表分开：migrate 可以反复跑，建管理员只在空库上发生一次。
	// 先查一次用户数，只有真的要建管理员时才要求 MAGICD_ADMIN_PASSWORD——
	// 库里已有用户的部署每次重启都会重新跑一遍 migrate，不该被这个环境
	// 变量卡住。
	existingUsers, err := s.CountUsers(ctx)
	if err != nil {
		return err
	}
	adminPass, err := resolveAdminPassword(existingUsers, os.Getenv(adminPasswordEnvVar))
	if err != nil {
		return err
	}

	name, pass, created, err := s.EnsureAdmin(ctx, adminPass)
	if err != nil {
		return err
	}
	if created {
		fmt.Println()
		if adminPass != "" {
			fmt.Println("已创建管理员账户（密码取自环境变量 " + adminPasswordEnvVar + "）：")
			fmt.Printf("  用户名: %s\n", name)
		} else {
			// 不会发生在 migrate 这条路径上：resolveAdminPassword 已经在
			// existingUsers==0 时把空密码挡在前面。留着这个分支是因为
			// EnsureAdmin 的通用契约仍然允许空密码走随机生成，防止将来
			// 这里的调用方式被改动后悄悄回退到"随机密码只打印一次"却
			// 没人发现。
			fmt.Println("已创建管理员账户，密码只显示这一次，请立即保存：")
			fmt.Printf("  用户名: %s\n", name)
			fmt.Printf("  密码:   %s\n", pass)
		}
		fmt.Println()
		fmt.Printf("改密码：magicd user passwd %s\n", name)
	}
	return nil
}
