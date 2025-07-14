package main

import (
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/kercylan98/vivid/core/vivid"
	"github.com/kercylan98/vivid/ecology/grpc-server"
)

func main() {
	// 创建 Actor 系统
	system := vivid.NewActorSystemWithOptions()
	defer system.Shutdown(true, "程序结束")

	// 创建 gRPC 服务器
	server := grpcserver.NewServer(
		grpcserver.WithPort(8080),
		grpcserver.WithHost("localhost"),
		grpcserver.WithActorSystem(system),
	)

	// 启动服务器
	if err := server.Start(); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
	defer server.Stop()

	log.Println("gRPC server started on localhost:8080")
	log.Println("Press Ctrl+C to stop")

	// 等待中断信号
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	<-c

	log.Println("Shutting down server...")
}