package httpapi_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/MrZhongzq/MagicalDanmaku/server/internal/httpapi"
	"github.com/MrZhongzq/MagicalDanmaku/server/internal/store"
)

// newTestServerWithAPI 与 newTestServer 相同，但额外返回 *httpapi.Server，
// 供需要注入假实现（如扫码登录）的测试使用。
//
// 用 httptest.NewServer 打真实 HTTP 而不是直接调 handler 函数：
// 路由匹配、方法限制、中间件顺序都是要测的行为，绕过它们就等于没测。
func newTestServerWithAPI(t *testing.T) (*httptest.Server, *store.Store, *httpapi.Server) {
	t.Helper()

	dsn := os.Getenv("MAGICD_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("未设置 MAGICD_TEST_DATABASE_URL，跳过 HTTP API 测试。\n" +
			"本地起库：docker compose -f docker-compose.dev.yml up -d\n" +
			"然后：export MAGICD_TEST_DATABASE_URL='postgres://magicd:magicd@localhost:5433/magicd?sslmode=disable'")
	}

	schema := schemaNameFor(t.Name())
	ctx := context.Background()

	admin, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatalf("连接数据库失败: %v", err)
	}
	if _, err := admin.Exec(ctx, `DROP SCHEMA IF EXISTS `+schema+` CASCADE`); err != nil {
		admin.Close()
		t.Fatalf("清理旧 schema 失败: %v", err)
	}
	if _, err := admin.Exec(ctx, `CREATE SCHEMA `+schema); err != nil {
		admin.Close()
		t.Fatalf("创建 schema 失败: %v", err)
	}

	var st *store.Store
	t.Cleanup(func() {
		if st != nil {
			st.Close()
		}
		if _, err := admin.Exec(context.Background(),
			`DROP SCHEMA IF EXISTS `+schema+` CASCADE`); err != nil {
			t.Logf("清理 schema %s 失败: %v", schema, err)
		}
		admin.Close()
	})

	st, err = store.OpenWithSchema(ctx, dsn, schema)
	if err != nil {
		t.Fatalf("打开存储失败: %v", err)
	}
	if err := st.Migrate(ctx); err != nil {
		t.Fatalf("迁移失败: %v", err)
	}

	api := httpapi.New(st, httpapi.Options{
		SessionTTL: time.Hour,
		// 仅测试用：挂上 /api/test/* 以在真实中间件链上验证 panic 恢复、
		// 权限守卫、可见范围过滤。生产环境绝不能打开这个开关。
		EnableTestRoutes: true,
	})
	srv := httptest.NewServer(api.Handler())
	t.Cleanup(srv.Close)
	return srv, st, api
}

// newTestServer 与 newTestServerWithAPI 相同，只是不需要 *httpapi.Server 的
// 调用方可以少写一个返回值。
func newTestServer(t *testing.T) (*httptest.Server, *store.Store) {
	srv, st, _ := newTestServerWithAPI(t)
	return srv, st
}

// schemaNameFor 把测试名转成合法的 PostgreSQL 标识符。
func schemaNameFor(name string) string {
	var b strings.Builder
	b.WriteString("h_")
	for _, r := range name {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			b.WriteRune(r)
		case r >= 'A' && r <= 'Z':
			b.WriteRune(r + 32)
		default:
			b.WriteByte('_')
		}
	}
	s := b.String()
	if len(s) > 50 {
		s = s[:44] + "_" + string(rune('a'+len(name)%26))
	}
	return s
}

// jsonRequest 发一个带 JSON 体的请求。
func jsonRequest(t *testing.T, client *http.Client, method, url, body string) *http.Response {
	t.Helper()
	var r *http.Request
	var err error
	if body == "" {
		r, err = http.NewRequest(method, url, nil)
	} else {
		r, err = http.NewRequest(method, url, strings.NewReader(body))
		r.Header.Set("Content-Type", "application/json")
	}
	if err != nil {
		t.Fatalf("构造请求报错: %v", err)
	}
	resp, err := client.Do(r)
	if err != nil {
		t.Fatalf("请求 %s %s 报错: %v", method, url, err)
	}
	return resp
}
