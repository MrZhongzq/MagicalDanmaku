package httpapi

import (
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"

	"github.com/MrZhongzq/MagicalDanmaku/server/internal/store"
)

// respondJSON 是所有成功响应的唯一出口。
func respondJSON(w http.ResponseWriter, status int, v any) {
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
func respondStoreError(w http.ResponseWriter, err error, notFoundMsg string) {
	switch {
	case errors.Is(err, store.ErrNotFound):
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
