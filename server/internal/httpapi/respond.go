package httpapi

import (
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"

	"github.com/MrZhongzq/MagicalDanmaku/server/internal/store"
)

// jsonMarker 让本包自己写的 JSON 响应能被中间件识别。
//
// withNotFoundJSON 需要区分两种 404：ServeMux 兜底的（要替换成 JSON）
// 与处理器自己写的（要原样放行）。没有这个标记，处理器写的
// 「绑定 X 不存在」会被改写成「接口 GET /xxx 不存在」，
// 而后者对调用者毫无用处。
type jsonMarker interface{ markJSON() }

// respondJSON 是所有成功响应的唯一出口。
func respondJSON(w http.ResponseWriter, status int, v any) {
	if m, ok := w.(jsonMarker); ok {
		m.markJSON()
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	if v == nil {
		return
	}
	if err := json.NewEncoder(w).Encode(v); err != nil {
		// 头已经发出去了，改不了状态码，只能记日志
		slog.Error("写响应体失败", "err", err)
	}
}

// respondError 是所有错误响应的唯一出口。
//
// 错误体统一为 {"error": "中文说明"}，前端才能用一套逻辑处理。
func respondError(w http.ResponseWriter, status int, format string, args ...any) {
	respondJSON(w, status, map[string]string{"error": fmt.Sprintf(format, args...)})
}

// respondStoreError 把存储层的哨兵错误翻译成 HTTP 状态码。
//
// notFoundMsg 由调用方给，因为「什么不存在」只有调用方知道。
// 传空串时兜底成通用文案：今天的调用点都对应不会 404 的 store 方法，
// 所以走不到这里，但这是留给后面任务的安全网——一旦某次改动让
// 调用点开始可能触发 404，而调用方忘了传具体文案，也不该吐出
// {"error":""} 这种空响应。
func respondStoreError(w http.ResponseWriter, err error, notFoundMsg string) {
	switch {
	case errors.Is(err, store.ErrNotFound):
		if notFoundMsg == "" {
			notFoundMsg = "记录不存在"
		}
		respondError(w, http.StatusNotFound, "%s", notFoundMsg)
	case errors.Is(err, store.ErrDuplicate):
		respondError(w, http.StatusConflict, "已存在同名记录")
	default:
		// 内部错误不把细节吐给客户端：那既没用又可能泄漏结构
		slog.Error("请求处理失败", "err", err)
		respondError(w, http.StatusInternalServerError, "服务器内部错误")
	}
}

// decodeJSON 解析请求体。字段不认识就报错，不静默忽略——
// 前端把字段名拼错时，静默忽略会让人查很久。
func decodeJSON(w http.ResponseWriter, r *http.Request, dst any) bool {
	dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20))
	dec.DisallowUnknownFields()
	if err := dec.Decode(dst); err != nil {
		respondError(w, http.StatusUnprocessableEntity, "请求体不合法: %v", err)
		return false
	}
	return true
}
