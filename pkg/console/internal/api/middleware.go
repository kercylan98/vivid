package api

import (
	"net/http"
	"runtime/debug"
)

// Middleware 对 http.Handler 的包装函数类型。
type Middleware func(next http.Handler) http.Handler

// Chain 按顺序应用多个中间件，返回包装后的 Handler。
func Chain(inner http.Handler, mws ...Middleware) http.Handler {
	h := inner
	for i := len(mws) - 1; i >= 0; i-- {
		h = mws[i](h)
	}
	return h
}

// Recovery 捕获 panic，返回 500 并记录堆栈，避免进程退出。
func Recovery(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if err := recover(); err != nil {
				_ = err
				debug.PrintStack()
				FailServerError(w, "internal server error")
			}
		}()
		next.ServeHTTP(w, r)
	})
}

// corsDefaultOrigin 无 Origin 请求头时的默认来源（如同源或默认前端地址）。
const corsDefaultOrigin = "http://localhost:3000"

// CORS 为跨域请求添加 Access-Control-* 头并处理 OPTIONS 预检。
// 有 Origin 时回填该 Origin，否则使用 corsDefaultOrigin。
func CORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		if origin == "" {
			origin = corsDefaultOrigin
		}
		w.Header().Set("Access-Control-Allow-Origin", origin)
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		w.Header().Set("Access-Control-Max-Age", "86400")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}
