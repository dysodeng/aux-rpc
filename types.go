package auxrpc

import (
	"github.com/dysodeng/aux-rpc/registry"
	"github.com/rcrowley/go-metrics"
	"google.golang.org/grpc"
)

// Server grpc服务注册器
type Server struct {
	// listen 服务监听地址
	listen string
	// registry 服务注册器
	registry registry.Registry
	// grpcServer grpc
	grpcServer *grpc.Server
	// metrics 监控
	metrics        metrics.Meter
	showMetricsLog bool
	// grpc server option
	grpcOptions []grpc.ServerOption
	// 拦截器
	interceptor       []grpc.UnaryServerInterceptor
	interceptorStream []grpc.StreamServerInterceptor
}

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

// WithGrpcUnaryServerInterceptor 设置grpc一元拦截器
func WithGrpcUnaryServerInterceptor(interceptor ...grpc.UnaryServerInterceptor) Option {
	return func(server *Server) {
		server.interceptor = interceptor
	}
}

// WithStreamServerInterceptor 设置grpc流式请求拦截器
func WithStreamServerInterceptor(interceptor ...grpc.StreamServerInterceptor) Option {
	return func(server *Server) {
		server.interceptorStream = interceptor
	}
}
