// Package main 提供了一个在 HTTP 场景下集成 Vivid 框架的示例。
//
// 推荐将 Vivid 作为组件集成到 HTTP 服务器中，而非直接用于承接每一个 HTTP 请求。
// 由于 Vivid 基于 Actor 模型，在运行过程中会引入额外的系统资源消耗，例如消息寻址、邮箱投递等机制，
// 若直接用来处理高并发的 HTTP 请求，可能导致性能开销和复杂性提升，带来不必要的负担。
// 将 Vivid 嵌入 HTTP 服务器，适合用于有持久化状态或需要并发控制的特殊场景（如：WebSocket、HTTP 长连接等），
// 能充分发挥 Actor 模型在状态管理与异步处理方面的优势。
package main

import (
	"net/http"

	"github.com/kercylan98/vivid"
	"github.com/kercylan98/vivid/pkg/bootstrap"
)

func NewMyServer(actorSystem vivid.ActorSystem) *MyServer {
	httpSrv := http.NewServeMux()
	return &MyServer{
		actorSystem: actorSystem,
		httpSrv:     httpSrv,
	}
}

type MyServer struct {
	actorSystem vivid.ActorSystem
	httpSrv     *http.ServeMux
}

func (s *MyServer) genActorSystemHttpHandler(handler func(w http.ResponseWriter, r *http.Request, system vivid.ActorSystem) error) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := handler(w, r, s.actorSystem); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
	}
}

func main() {
	actorSystem := bootstrap.NewActorSystem()
	if err := actorSystem.Start(); err != nil {
		panic(err)
	}
	defer actorSystem.Stop()

	myServer := NewMyServer(actorSystem)
	myServer.httpSrv.Handle("/hello", myServer.genActorSystemHttpHandler(func(w http.ResponseWriter, r *http.Request, system vivid.ActorSystem) error {
		// use actorSystem to do something
		// actorSystem.ActorOf(vivid.ActorFN(func(ctx vivid.ActorContext) {
		// 	switch ctx.Message().(type) {
		// 	case *vivid.OnLaunch:
		// 		ctx.Kill(ctx.Ref(), false)
		// 	}
		// }))

		w.WriteHeader(http.StatusOK)
		w.Write([]byte("Hello, World!"))
		return nil
	}))
	http.ListenAndServe(":8080", myServer.httpSrv)
}
