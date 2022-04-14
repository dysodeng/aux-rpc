package auxrpc

import (
	"context"
	"log"
	"net"
	"os"
	"reflect"
	"time"

	"github.com/pkg/errors"

	"github.com/dysodeng/aux-rpc/registry"
	grpcMiddleware "github.com/grpc-ecosystem/go-grpc-middleware"
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

// NewServer 新建服务注册
func NewServer(registry registry.Registry, opts ...Option) (*Server, error) {

	server := &Server{
		registry: registry,
	}

	for _, opt := range opts {
		opt(server)
	}

	var interceptorStream []grpc.StreamServerInterceptor
	var interceptor []grpc.UnaryServerInterceptor

	// Metrics监控
	if server.metrics != nil {
		metrics.GetOrRegister("grpc_tps", server.metrics)
		interceptor = append(interceptor, func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (resp interface{}, err error) {
			server.metrics.Mark(1)
			return handler(ctx, req)
		})
		interceptorStream = append(interceptorStream, func(srv interface{}, ss grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
			server.metrics.Mark(1)
			return handler(srv, ss)
		})

		if server.showMetricsLog {
			go func() {
				metrics.Log(metrics.DefaultRegistry,
					30*time.Second,
					log.New(os.Stdout, "metrics: ", log.LstdFlags))
			}()
		}
	}

	if len(interceptor) > 0 {
		server.grpcOptions = append(server.grpcOptions, grpc.UnaryInterceptor(grpcMiddleware.ChainUnaryServer(interceptor...)))
	}
	if len(interceptorStream) > 0 {
		server.grpcOptions = append(server.grpcOptions, grpc.StreamInterceptor(grpcMiddleware.ChainStreamServer(interceptorStream...)))
	}

	server.grpcServer = grpc.NewServer(server.grpcOptions...)

	return server, nil
}

// Register 注册服务
func (s *Server) Register(serviceName string, service interface{}, grpcRegister interface{}, metadata string) error {
	// 注册grpc服务
	fn := reflect.ValueOf(grpcRegister)
	if fn.Kind() != reflect.Func {
		return errors.New("`grpcRegister` is not a grpc registration function")
	}

	params := make([]reflect.Value, 2)
	params[0] = reflect.ValueOf(s.grpcServer)
	params[1] = reflect.ValueOf(service)
	fn.Call(params)

	// 服务发现注册
	err := s.registry.Register(serviceName, metadata)
	if err != nil {
		return err
	}

	return nil
}

// Serve 启动服务监听
func (s *Server) Serve() error {
	address := s.registry.GetServiceListener()
	listener, err := net.Listen("tcp", address)
	if err != nil {
		return errors.Wrap(err, "gRPC server net.Listen err")
	}
	log.Printf("listening and serving gRPC on %s\n", address)

	err = s.grpcServer.Serve(listener)
	if err != nil {
		return errors.Wrap(err, "grpcServer.Serve err")
	}

	return nil
}

// Stop 服务停止
func (s *Server) Stop() error {
	err := s.registry.Stop()
	if err != nil {
		return err
	}
	s.grpcServer.Stop()

	log.Println("gRPC server stop")

	return nil
}
