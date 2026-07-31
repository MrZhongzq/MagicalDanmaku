// Package httpapi 是管理界面的 HTTP 后端。
//
// 与机器人同进程运行：实时事件流需要直接复用机器人已持有的事件通道，
// 拆成两个进程就要再造一套跨进程总线，对单机工具不划算。
//
// 路由用标准库的 ServeMux（Go 1.22+ 支持方法与通配模式），不引第三方
// 路由库或 Web 框架。
package httpapi

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/MrZhongzq/MagicalDanmaku/server/internal/perm"
	"github.com/MrZhongzq/MagicalDanmaku/server/internal/store"
)

// 默认值。
const (
	defaultSessionTTL      = 30 * 24 * time.Hour
	defaultReadTimeout     = 15 * time.Second
	defaultWriteTimeout    = 0 // SSE 是长连接，写超时必须为 0
	defaultIdleTimeout     = 120 * time.Second
	defaultShutdownTimeout = 10 * time.Second
)

// Options 配置 HTTP 服务。
type Options struct {
	// Addr 是监听地址。默认 127.0.0.1:8080——只监听本机，
	// 不因为用户忘了配防火墙就把管理界面暴露到公网。
	// Docker 部署需显式设为 0.0.0.0:8080。
	Addr string

	SessionTTL time.Duration // 会话有效期，0 用默认

	// SecureCookie 决定会话 Cookie 是否带 Secure 标志。
	// 默认关闭：默认监听 127.0.0.1 走明文 HTTP，打开会让 Cookie 根本发不出去。
	// 反向代理加了 TLS 的部署要显式打开。
	SecureCookie bool

	Logger *slog.Logger
}

// Server 持有 HTTP 服务的全部依赖。
type Server struct {
	store *store.Store
	opts  Options
	log   *slog.Logger
	mux   *http.ServeMux
}

// New 创建服务并注册全部路由。
func New(st *store.Store, opts Options) *Server {
	if opts.Addr == "" {
		opts.Addr = "127.0.0.1:8080"
	}
	if opts.SessionTTL <= 0 {
		opts.SessionTTL = defaultSessionTTL
	}
	if opts.Logger == nil {
		opts.Logger = slog.Default()
	}

	s := &Server{
		store: st,
		opts:  opts,
		log:   opts.Logger,
		mux:   http.NewServeMux(),
	}
	s.routes()
	return s
}

// routes 注册全部路由。后续任务往这里加。
func (s *Server) routes() {
	s.mux.HandleFunc("GET /api/health", s.handleHealth)

	// 认证。登录与登出都用 POST：SameSite=Lax 不拦截跨站顶层 GET 导航，
	// 所以一切改变状态的接口都不能用 GET。
	s.mux.HandleFunc("POST /api/auth/login", s.handleLogin)
	s.mux.HandleFunc("POST /api/auth/logout", s.handleLogout)
	s.mux.HandleFunc("GET /api/auth/me", s.requireAuth(s.handleMe))

	// 仅测试用：验证 panic 恢复中间件。生产路由表里不该有它，
	// 但把它放在这里比在测试里另起一个 mux 更能保证测的是真实中间件链。
	s.mux.HandleFunc("GET /api/test/panic", func(http.ResponseWriter, *http.Request) {
		panic("测试用的故意 panic")
	})

	// 仅测试用：验证权限守卫与可见范围过滤。放在真实中间件链上，
	// 才能证明守卫在真实路由里也是这样工作的。
	s.mux.HandleFunc("GET /api/test/guarded/{binding}",
		s.requirePerm(perm.RuleRead, func(w http.ResponseWriter, r *http.Request) {
			respondJSON(w, http.StatusOK, map[string]any{"binding": bindingFrom(r.Context()).Label()})
		}))
	s.mux.HandleFunc("POST /api/test/guarded-write/{binding}",
		s.requirePerm(perm.RuleWrite, func(w http.ResponseWriter, r *http.Request) {
			respondJSON(w, http.StatusOK, map[string]string{"status": "ok"})
		}))
	s.mux.HandleFunc("GET /api/test/visible-bindings",
		s.requireAuth(func(w http.ResponseWriter, r *http.Request) {
			bs, err := s.visibleBindings(r.Context(), userFrom(r.Context()))
			if err != nil {
				respondStoreError(w, err, "")
				return
			}
			out := make([]map[string]any, 0, len(bs))
			for _, b := range bs {
				out = append(out, map[string]any{
					"id": b.ID, "accountName": b.AccountName, "roomId": b.RoomID,
				})
			}
			respondJSON(w, http.StatusOK, out)
		}))
}

// Handler 返回套好中间件的处理器。
func (s *Server) Handler() http.Handler {
	var h http.Handler = s.mux
	h = s.withNotFoundJSON(h)
	h = s.withRequestLog(h)
	h = s.withRecover(h)
	return h
}

// ListenAndServe 阻塞运行直到 ctx 取消，然后优雅关停。
func (s *Server) ListenAndServe(ctx context.Context) error {
	srv := &http.Server{
		Addr:    s.opts.Addr,
		Handler: s.Handler(),
		// SSE 是长连接，写超时必须为 0，否则连接会被定期掐断
		ReadHeaderTimeout: defaultReadTimeout,
		WriteTimeout:      defaultWriteTimeout,
		IdleTimeout:       defaultIdleTimeout,
	}

	errCh := make(chan error, 1)
	go func() {
		s.log.Info("HTTP 服务已启动", "addr", s.opts.Addr)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
			return
		}
		errCh <- nil
	}()

	select {
	case err := <-errCh:
		return err
	case <-ctx.Done():
		shutCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), defaultShutdownTimeout)
		defer cancel()
		if err := srv.Shutdown(shutCtx); err != nil {
			s.log.Warn("HTTP 服务关停超时", "err", err)
		}
		s.log.Info("HTTP 服务已关停")
		return nil
	}
}

// handleHealth 是存活探针，无需认证。
func (s *Server) handleHealth(w http.ResponseWriter, _ *http.Request) {
	respondJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}
