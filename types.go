package auxrpc

import (
	"context"
	"github.com/dysodeng/aux-rpc/registry"
	"github.com/rcrowley/go-metrics"
	"google.golang.org/grpc"
)

// Server grpc服务注册
type Server struct {
	// registry 服务注册器
	registry registry.Registry
	// grpcServer grpc
	grpcServer *grpc.Server
	// metrics 监控
	metrics        metrics.Meter
	showMetricsLog bool
	grpcOptions    []grpc.ServerOption
}

type AuthFunc func(ctx context.Context, token string) error

// Option 服务选项
type Option func(server *Server)

// WithMetrics 设置监控
func WithMetrics(metrics metrics.Meter, showMetricsLog bool) Option {
	return func(server *Server) {
		server.metrics = metrics
		server.showMetricsLog = showMetricsLog
	}
}

// WithGrpcServiceOption 设置grpc选项
func WithGrpcServiceOption(opts ...grpc.ServerOption) Option {
	return func(server *Server) {
		server.grpcOptions = opts
	}
}
