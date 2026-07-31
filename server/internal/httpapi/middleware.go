package httpapi

import (
	"net/http"
	"runtime/debug"
	"strings"
	"time"
)

// withRecover 拦住处理器里的 panic。
//
// 一个处理器 panic 不该带崩整个进程——机器人还在同一个进程里跑着。
func (s *Server) withRecover(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if v := recover(); v != nil {
				s.log.Error("处理器 panic",
					"method", r.Method, "path", r.URL.Path,
					"panic", v, "stack", string(debug.Stack()))
				// 只回一句话：内部细节既对客户端无用，又可能泄漏结构
				respondError(w, http.StatusInternalServerError, "服务器内部错误")
			}
		}()
		next.ServeHTTP(w, r)
	})
}

// statusRecorder 记下实际写出的状态码，供日志使用。
type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (rec *statusRecorder) WriteHeader(code int) {
	rec.status = code
	rec.ResponseWriter.WriteHeader(code)
}

// Flush 透传给底层，SSE 需要它。
func (rec *statusRecorder) Flush() {
	if f, ok := rec.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}

// withRequestLog 记录每个请求。
func (s *Server) withRequestLog(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(rec, r)
		s.log.Debug("HTTP 请求",
			"method", r.Method, "path", r.URL.Path,
			"status", rec.status, "耗时", time.Since(start))
	})
}

// withNotFoundJSON 把 ServeMux 的纯文本 404/405 换成 JSON。
//
// 前端要能用一套逻辑处理全部错误响应，混进纯文本会让它必须先猜
// Content-Type。ServeMux 没有提供替换默认响应的钩子，所以这里
// 拦截响应：如果处理器没写过任何东西且状态是 404/405，就改写它。
func (s *Server) withNotFoundJSON(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rec := &interceptor{ResponseWriter: w}
		next.ServeHTTP(rec, r)

		if rec.wroteBody {
			return
		}
		switch rec.status {
		case http.StatusNotFound:
			respondError(w, http.StatusNotFound, "接口 %s %s 不存在", r.Method, r.URL.Path)
		case http.StatusMethodNotAllowed:
			allow := w.Header().Get("Allow")
			if allow != "" {
				respondError(w, http.StatusMethodNotAllowed,
					"接口 %s 不支持 %s 方法，支持的方法：%s", r.URL.Path, r.Method, allow)
			} else {
				respondError(w, http.StatusMethodNotAllowed, "接口 %s 不支持 %s 方法", r.URL.Path, r.Method)
			}
		}
	})
}

// interceptor 拦下状态码与「是否已写过响应体」，让 withNotFoundJSON
// 能判断该不该改写。
type interceptor struct {
	http.ResponseWriter
	status      int
	wroteHeader bool
	wroteBody   bool
}

func (i *interceptor) WriteHeader(code int) {
	if i.wroteHeader {
		return
	}
	i.status = code
	i.wroteHeader = true
	// 404/405 先不透传，交给外层决定怎么写
	if code == http.StatusNotFound || code == http.StatusMethodNotAllowed {
		return
	}
	i.ResponseWriter.WriteHeader(code)
}

func (i *interceptor) Write(b []byte) (int, error) {
	if !i.wroteHeader {
		i.WriteHeader(http.StatusOK)
	}
	// ServeMux 的默认 404/405 会写一行纯文本，丢掉它
	if i.status == http.StatusNotFound || i.status == http.StatusMethodNotAllowed {
		if strings.TrimSpace(string(b)) != "" {
			return len(b), nil
		}
		return len(b), nil
	}
	i.wroteBody = true
	return i.ResponseWriter.Write(b)
}

// Flush 透传给底层，SSE 需要它。
func (i *interceptor) Flush() {
	if f, ok := i.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}
