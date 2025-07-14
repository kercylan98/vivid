// Package grpcserver provides gRPC server implementation for Vivid Actor Framework.
//
// This package integrates gRPC with Vivid's Actor system, allowing actors to
// handle gRPC requests and responses seamlessly.
package grpcserver

import (
	"context"
	"fmt"
	"net"

	"google.golang.org/grpc"
	"github.com/kercylan98/vivid/core/vivid"
)

// Server represents a gRPC server integrated with Vivid Actor system
type Server struct {
	config       *Configuration
	actorSystem  vivid.ActorSystem
	grpcServer   *grpc.Server
	listener     net.Listener
}

// Configuration holds the server configuration
type Configuration struct {
	Port        int
	Host        string
	ActorSystem vivid.ActorSystem
	Options     []grpc.ServerOption
}

// Option represents a configuration option for the server
type Option func(*Configuration)

// WithPort sets the server port
func WithPort(port int) Option {
	return func(c *Configuration) {
		c.Port = port
	}
}

// WithHost sets the server host
func WithHost(host string) Option {
	return func(c *Configuration) {
		c.Host = host
	}
}

// WithActorSystem sets the actor system
func WithActorSystem(system vivid.ActorSystem) Option {
	return func(c *Configuration) {
		c.ActorSystem = system
	}
}

// WithGRPCOptions adds gRPC server options
func WithGRPCOptions(opts ...grpc.ServerOption) Option {
	return func(c *Configuration) {
		c.Options = append(c.Options, opts...)
	}
}

// NewServer creates a new gRPC server with the given options
func NewServer(opts ...Option) *Server {
	config := &Configuration{
		Port: 8080,
		Host: "localhost",
	}

	for _, opt := range opts {
		opt(config)
	}

	return &Server{
		config: config,
		actorSystem: config.ActorSystem,
		grpcServer: grpc.NewServer(config.Options...),
	}
}

// Start starts the gRPC server
func (s *Server) Start() error {
	addr := fmt.Sprintf("%s:%d", s.config.Host, s.config.Port)
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("failed to listen on %s: %w", addr, err)
	}

	s.listener = listener

	go func() {
		if err := s.grpcServer.Serve(listener); err != nil {
			s.actorSystem.Logger().Error("gRPC server error", "error", err)
		}
	}()

	s.actorSystem.Logger().Info("gRPC server started", "address", addr)
	return nil
}

// Stop stops the gRPC server
func (s *Server) Stop() error {
	if s.grpcServer != nil {
		s.grpcServer.GracefulStop()
	}
	if s.listener != nil {
		return s.listener.Close()
	}
	return nil
}

// RegisterService registers a gRPC service with actor integration
func (s *Server) RegisterService(desc *grpc.ServiceDesc, impl interface{}) {
	s.grpcServer.RegisterService(desc, impl)
}