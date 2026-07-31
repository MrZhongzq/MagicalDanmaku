package main

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"os"
	"strings"

	"golang.org/x/term"

	"github.com/MrZhongzq/MagicalDanmaku/server/internal/store"
)

// addDBFlag 给子命令挂上 -db 标志。
func addDBFlag(fs *flag.FlagSet) *string {
	return fs.String("db", "", "PostgreSQL 连接串，留空则读环境变量 MAGICD_DATABASE_URL")
}

// openStore 连接数据库。dsn 为空时回落到环境变量。
func openStore(ctx context.Context, dsn string) (*store.Store, error) {
	if dsn == "" {
		dsn = os.Getenv("MAGICD_DATABASE_URL")
	}
	if dsn == "" {
		return nil, fmt.Errorf("未指定数据库连接串。设置环境变量 MAGICD_DATABASE_URL 或用 -db 传入，例如：\n" +
			"  export MAGICD_DATABASE_URL='postgres://magicd:magicd@localhost:5432/magicd?sslmode=disable'")
	}
	return store.Open(ctx, dsn)
}

// openStoreChecked 连接数据库并校验 schema 版本。
//
// 版本落后就拒绝启动而不是自动迁移：多实例部署下，让每个实例各自
// 决定何时改表是危险的。
func openStoreChecked(ctx context.Context, dsn string) (*store.Store, error) {
	s, err := openStore(ctx, dsn)
	if err != nil {
		return nil, err
	}

	current, err := s.SchemaVersion(ctx)
	if err != nil {
		s.Close()
		return nil, err
	}
	latest, err := store.LatestSchemaVersion()
	if err != nil {
		s.Close()
		return nil, err
	}
	if current < latest {
		s.Close()
		return nil, fmt.Errorf("%w（当前 %d，需要 %d）", store.ErrSchemaOutdated, current, latest)
	}
	return s, nil
}

// parseBindingRef 解析形如 "小号@1706666491" 的绑定引用。
//
// 从最后一个 @ 切分：账号名是用户起的，可能自带 @。
func parseBindingRef(s string) (accountName, roomID string, err error) {
	i := strings.LastIndex(s, "@")
	if i < 0 {
		return "", "", fmt.Errorf("绑定要写成「账号名@房间号」的形式，例如 小号@1706666491，实际收到 %q", s)
	}
	accountName = strings.TrimSpace(s[:i])
	roomID = strings.TrimSpace(s[i+1:])
	if accountName == "" || roomID == "" {
		return "", "", fmt.Errorf("绑定 %q 的账号名或房间号为空，应写成「账号名@房间号」", s)
	}
	return accountName, roomID, nil
}

// readPassword 从终端读密码，不回显。
//
// 终端不可用时（管道、CI）回落到读一行明文：让脚本能用，
// 代价是密码会出现在进程的标准输入里。
func readPassword(prompt string) (string, error) {
	fmt.Print(prompt)

	fd := int(os.Stdin.Fd())
	if term.IsTerminal(fd) {
		b, err := term.ReadPassword(fd)
		fmt.Println()
		if err != nil {
			return "", fmt.Errorf("读取密码失败: %w", err)
		}
		return string(b), nil
	}

	line, err := bufio.NewReader(os.Stdin).ReadString('\n')
	if err != nil && line == "" {
		return "", fmt.Errorf("读取密码失败: %w", err)
	}
	return strings.TrimRight(line, "\r\n"), nil
}
