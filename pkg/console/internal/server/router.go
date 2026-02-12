package server

import (
	"net/http"

	_ "net/http/pprof" // 注册 /debug/pprof 到 DefaultServeMux

	"github.com/kercylan98/vivid/pkg/console/internal/api"
	handler2 "github.com/kercylan98/vivid/pkg/console/internal/handler"
)

// RouterDeps 路由依赖：注入各 handler 所需依赖，未注入的接口返回 503 或占位。
type RouterDeps struct {
	Node   *handler2.Node
	Recent *handler2.Recent
}

// newRouter 注册所有路由并应用中间件，返回 http.Handler。
// 路径遵循 REST：资源复数、层级清晰；GET 只读。
func newRouter(deps *RouterDeps) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/health", handler2.Health)
	mux.HandleFunc("GET /api/nodes/current/cluster", deps.Node.CurrentNodeCluster)
	mux.HandleFunc("GET /api/nodes/current/state", deps.Node.CurrentNodeState)
	mux.HandleFunc("GET /api/nodes/current/metrics", deps.Node.CurrentNodeMetrics)
	mux.HandleFunc("GET /api/nodes/current/events", deps.Recent.RecentEvents)
	mux.HandleFunc("GET /api/nodes/current/death-letters", deps.Recent.RecentDeathLetters)
	// pprof：挂载到 /debug/pprof/，与 Go 标准一致
	mux.Handle("/debug/pprof/", http.DefaultServeMux)
	return api.Chain(mux, api.CORS, api.Recovery)
}
