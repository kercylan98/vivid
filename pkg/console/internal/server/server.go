package server

import (
	"context"
	"net/http"
	"time"

	"github.com/kercylan98/vivid"
	"github.com/kercylan98/vivid/pkg/console/internal/handler"
	"github.com/kercylan98/vivid/pkg/console/internal/store"
)

// Server 原生 HTTP 服务，使用标准请求响应格式与中间件链。
type Server struct {
	http *http.Server
}

func NewServer(addr string, readTimeout time.Duration, writeTimeout time.Duration, system vivid.ActorSystem, recentStore *store.RecentStore) *Server {
	return &Server{
		http: &http.Server{
			Addr: addr,
			Handler: newRouter(&RouterDeps{
				Node:   handler.NewNode(system),
				Recent: handler.NewRecent(recentStore),
			}),
			ReadTimeout:  readTimeout,
			WriteTimeout: writeTimeout,
		},
	}
}

// Start 阻塞监听，通常放在 goroutine 中调用。
func (s *Server) Start() error {
	return s.http.ListenAndServe()
}

// Shutdown 优雅关闭。
func (s *Server) Shutdown(ctx context.Context) error {
	return s.http.Shutdown(ctx)
}
