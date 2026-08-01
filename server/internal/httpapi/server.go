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
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/MrZhongzq/MagicalDanmaku/server/internal/connector/bilibili/auth"
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

	// EnableTestRoutes 挂上一套仅供测试用的路由（/api/test/*），
	// 用来在真实中间件链上验证 panic 恢复、权限守卫、可见范围过滤。
	//
	// 生产环境必须为 false：其中 /api/test/panic 不需要认证、每次
	// 访问都主动 panic 并以 Error 级别打一整份 debug.Stack()，暴露在
	// 公网上等于给任何能连上端口的人一个刷爆日志、撑爆磁盘的开关。
	// 只有 testhelp_test.go 的 newTestServer 应该把它设为 true。
	EnableTestRoutes bool

	Logger *slog.Logger
}

// Server 持有 HTTP 服务的全部依赖。
type Server struct {
	store   *store.Store
	opts    Options
	log     *slog.Logger
	mux     *http.ServeMux
	qrs     *qrSessions
	qrLogin qrStarter
	hub     *Hub
	runtime *runtimeRegistry
	cfgHash configHashes

	// staticHandler 服务前端产物与 SPA 回退，由 mountStatic 装配。
	// 不挂在 mux 上，见 static.go 里的说明。
	staticHandler http.Handler
}

// qrTTL 是扫码会话在内存表里的存活时间，与 B 站二维码本身的
// 有效期（3 分钟）一致——二维码过期了，会话留着也没用。
const qrTTL = 3 * time.Minute

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
		store:   st,
		opts:    opts,
		log:     opts.Logger,
		mux:     http.NewServeMux(),
		qrs:     newQRSessions(qrTTL),
		qrLogin: auth.NewQRLogin(nil),
		hub:     NewHub(),
		runtime: newRuntimeRegistry(),
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

	if s.opts.EnableTestRoutes {
		s.testRoutes()
	}

	// 元数据：前端渲染规则编辑器需要。只需登录，不需绑定级权限。
	s.mux.HandleFunc("GET /api/meta/permissions", s.requireAuth(s.handleMetaPermissions))
	s.mux.HandleFunc("GET /api/meta/event-types", s.requireAuth(s.handleMetaEventTypes))
	s.mux.HandleFunc("GET /api/meta/action-types", s.requireAuth(s.handleMetaActionTypes))
	s.mux.HandleFunc("GET /api/meta/operators", s.requireAuth(s.handleMetaOperators))
	s.mux.HandleFunc("GET /api/meta/aggregate-by", s.requireAuth(s.handleMetaAggregateBy))

	// 用户管理。改密码不走 requireAdmin，因为普通用户要能改自己的，
	// 具体的授权判断在处理器里（这是唯一的例外，因为它不是绑定级权限）。
	s.mux.HandleFunc("GET /api/users", s.requireAdmin(s.handleListUsers))
	s.mux.HandleFunc("POST /api/users", s.requireAdmin(s.handleCreateUser))
	s.mux.HandleFunc("POST /api/users/{name}/password", s.requireAuth(s.handleChangePassword))
	s.mux.HandleFunc("DELETE /api/users/{name}", s.requireAdmin(s.handleDeleteUser))

	// B 站账号。扫码的两步都用 POST：轮询成功时会建账号，是状态改变。
	s.mux.HandleFunc("GET /api/accounts", s.requireAuth(s.handleListAccounts))
	s.mux.HandleFunc("POST /api/accounts/qrcode", s.requireAuth(s.handleQRCodeStart))
	s.mux.HandleFunc("POST /api/accounts/qrcode/{key}", s.requireAuth(s.handleQRCodePoll))
	s.mux.HandleFunc("PATCH /api/accounts/{name}", s.requireAuth(s.handlePatchAccount))
	s.mux.HandleFunc("DELETE /api/accounts/{name}", s.requireAuth(s.handleDeleteAccount))

	// 绑定。启停走 rule:write 守卫；创建与删除是账号所有权级别的操作，
	// 在处理器里判所有者。
	s.mux.HandleFunc("GET /api/bindings", s.requireAuth(s.handleListBindings))
	s.mux.HandleFunc("POST /api/bindings", s.requireAuth(s.handleCreateBinding))
	s.mux.HandleFunc("PATCH /api/bindings/{binding}", s.requirePerm(perm.RuleWrite, s.handlePatchBinding))
	s.mux.HandleFunc("DELETE /api/bindings/{binding}", s.requireAuth(s.handleDeleteBinding))

	// 规则。读走 rule:read，一切写操作走 rule:write。
	rules := "/api/bindings/{binding}/rules"
	s.mux.HandleFunc("GET "+rules, s.requirePerm(perm.RuleRead, s.handleListRules))
	s.mux.HandleFunc("POST "+rules, s.requirePerm(perm.RuleWrite, s.handleCreateRule))
	s.mux.HandleFunc("PUT "+rules, s.requirePerm(perm.RuleWrite, s.handleReplaceRules))
	s.mux.HandleFunc("POST "+rules+"/validate", s.requirePerm(perm.RuleRead, s.handleValidateRule))
	s.mux.HandleFunc("PUT "+rules+"/{name}", s.requirePerm(perm.RuleWrite, s.handlePutRule))
	s.mux.HandleFunc("PATCH "+rules+"/{name}", s.requirePerm(perm.RuleWrite, s.handlePatchRule))
	s.mux.HandleFunc("DELETE "+rules+"/{name}", s.requirePerm(perm.RuleWrite, s.handleDeleteRule))

	// 冷却组：读走 rule:read，写走 rule:write。
	cd := "/api/bindings/{binding}/cooldown-groups"
	s.mux.HandleFunc("GET "+cd, s.requirePerm(perm.RuleRead, s.handleGetCooldownGroups))
	s.mux.HandleFunc("PUT "+cd, s.requirePerm(perm.RuleWrite, s.handlePutCooldownGroups))

	// 禁言名单：全部走 user:block。
	bl := "/api/bindings/{binding}/blocklist"
	s.mux.HandleFunc("GET "+bl, s.requirePerm(perm.UserBlock, s.handleListBlockList))
	s.mux.HandleFunc("POST "+bl, s.requirePerm(perm.UserBlock, s.handleAddToBlockList))
	s.mux.HandleFunc("DELETE "+bl+"/{uid}", s.requirePerm(perm.UserBlock, s.handleRemoveFromBlockList))

	// 即时动作。全部用 POST——它们都会改变外部状态。
	s.mux.HandleFunc("POST /api/bindings/{binding}/danmaku",
		s.requirePerm(perm.DanmakuSend, s.handleSendDanmaku))
	s.mux.HandleFunc("POST /api/bindings/{binding}/block",
		s.requirePerm(perm.UserBlock, s.handleBlockUser))
	s.mux.HandleFunc("POST /api/bindings/{binding}/unblock",
		s.requirePerm(perm.UserBlock, s.handleUnblockUser))

	// 热重载：改完规则按保存才生效。走 rule:write——能改规则的人才能让它生效
	s.mux.HandleFunc("POST /api/bindings/{binding}/reload",
		s.requirePerm(perm.RuleWrite, s.handleReload))

	// 业务日志：事件与动作在同一条时间线，走 event:read。
	s.mux.HandleFunc("GET /api/bindings/{binding}/activity",
		s.requirePerm(perm.EventRead, s.handleQueryActivity))

	// 实时事件流：SSE 推送，权限与业务日志一致，同走 event:read。
	s.mux.HandleFunc("GET /api/bindings/{binding}/stream",
		s.requirePerm(perm.EventRead, s.handleStream))

	// 运行期元数据：每个绑定的连接状态 + 配置版本是否与数据库一致。
	s.mux.HandleFunc("GET /api/meta/runtime", s.requireAuth(s.handleRuntimeMeta))

	// 授权管理：把别人拉进某个绑定、给他权限点、撤销。全部走 member:manage。
	members := "/api/bindings/{binding}/members"
	s.mux.HandleFunc("GET "+members, s.requirePerm(perm.MemberManage, s.handleListMembers))
	s.mux.HandleFunc("PUT "+members+"/{username}", s.requirePerm(perm.MemberManage, s.handleGrantMember))
	s.mux.HandleFunc("DELETE "+members+"/{username}", s.requirePerm(perm.MemberManage, s.handleRevokeMember))

	// 前端静态资源与 SPA 回退。装配好的处理器存进 s.staticHandler，
	// 不挂在 mux 上——分流逻辑在 Handler() 里，理由见 static.go。
	s.mountStatic()
}

// testRoutes 注册仅供测试用的路由（/api/test/*）。
//
// 只有在 Options.EnableTestRoutes 为 true 时才会被 routes() 调用，
// 生产环境必须保持 false——尤其是 /api/test/panic：它不需要认证，
// 每次访问都主动 panic 并以 Error 级别打一整份 debug.Stack()，暴露
// 在公网上就是一个免认证的日志/磁盘炸弹。
//
// 之所以挂在真实的 s.mux 上而不是测试里另起一个 mux，是为了让
// panic 恢复、权限守卫、可见范围过滤都在真实中间件链上被验证到，
// 跳过中间件链就等于没测。
func (s *Server) testRoutes() {
	s.mux.HandleFunc("GET /api/test/panic", func(http.ResponseWriter, *http.Request) {
		panic("测试用的故意 panic")
	})

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
	// /api 下的请求全部交给 mux：未知路径、方法不对都由 mux 自带的
	// 404/405 判断处理，再经 withNotFoundJSON 包成 JSON。
	//
	// 其余路径走前端静态资源与 SPA 回退（s.staticHandler），不让它们
	// 经过 mux 的 "/" 模式——mux 根本没注册 "/"，静态资源与 SPA 回退
	// 的分流完全在这一层做，理由见 static.go 里 mountStatic 的注释。
	var h http.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// 裸路径 /api 也算 API 前缀。少了这一句，GET /api 会掉进 SPA 回退
		// 拿到 200 + HTML —— 而这正是本任务最该守住的那条线的反例。
		// 简报给的测试用的是 /api/xxx，自带斜杠，测不出这个变体。
		if s.staticHandler == nil || strings.HasPrefix(r.URL.Path, "/api/") || r.URL.Path == "/api" {
			s.mux.ServeHTTP(w, r)
			return
		}
		s.staticHandler.ServeHTTP(w, r)
	})
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

		// srv.Shutdown 只关监听器与 idle 连接，不会取消在途请求的
		// context；handleStream 只在 r.Context().Done() 时才退出循环，
		// keepalive 是 30 秒一次。不把请求 ctx 系在 ListenAndServe 的
		// ctx 上，SSE 连接就会挂满整个 defaultShutdownTimeout，Ctrl+C
		// 固定多等一截，容易被误读成「机器人卡死」。
		BaseContext: func(net.Listener) context.Context { return ctx },
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

// Hub 返回事件中枢，供 run 把机器人收到的事件扇出进来。
func (s *Server) Hub() *Hub { return s.hub }
