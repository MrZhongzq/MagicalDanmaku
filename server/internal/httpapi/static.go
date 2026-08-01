package httpapi

import (
	"net/http"
	"strings"

	"github.com/MrZhongzq/MagicalDanmaku/server/internal/webui"
)

// mountStatic 准备前端产物的静态文件服务与 SPA 回退处理器，存进
// s.staticHandler。
//
// 特意不挂在 s.mux 的 "/" 模式上：Go 1.22+ 的 ServeMux 把没有方法
// 限定的 "/" 当成能匹配任意方法的兜底模式，一旦注册它，mux 自带的
// 「路径对但方法不对 -> 405」判断就会被这个兜底模式抢先命中——因为
// 对 mux 来说 "/" 本身就是一个能匹配当前方法的合法路由，根本不会
// 再去找「有没有别的模式方法对不上」。实测验证过：注册 "/" 之后，
// GET /api/auth/login（该路径只注册了 POST）会变成 200，而不是
// 期望的 405。
//
// 所以 /api 与静态资源的分流放到 mux 外层的 Handler() 里做，mux
// 本身只认注册过的 /api/* 路由，405/404 判断完全不受影响。
func (s *Server) mountStatic() {
	sub, err := webui.FS()
	if err != nil {
		s.log.Error("挂载前端资源失败", "err", err)
		return
	}
	fileServer := http.FileServer(http.FS(sub))

	s.staticHandler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// 存在的静态文件直接发
		p := strings.TrimPrefix(r.URL.Path, "/")
		if p != "" {
			if f, err := sub.Open(p); err == nil {
				f.Close()
				fileServer.ServeHTTP(w, r)
				return
			}
		}

		// 其余一律回退到 index.html，交给浏览器端路由
		r2 := r.Clone(r.Context())
		r2.URL.Path = "/"
		fileServer.ServeHTTP(w, r2)
	})
}
