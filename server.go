package auxrpc

import (
	"context"
	"errors"
	"github.com/dysodeng/aux-rpc/registry"
	grpcMiddleware "github.com/grpc-ecosystem/go-grpc-middleware"
	"github.com/rcrowley/go-metrics"
	"google.golang.org/grpc"
	"log"
	"net"
	"os"
	"reflect"
	"time"
)

// Server grpc服务注册
type Server struct {
	// registry 服务注册器
	registry registry.Registry
	// grpcServer grpc
	grpcServer *grpc.Server
}

type AuthFunc func(ctx context.Context, token string) error

// NewServer 新建服务注册
func NewServer(registry registry.Registry, opt ...grpc.ServerOption) *Server {

	var interceptorStream []grpc.StreamServerInterceptor
	var interceptor []grpc.UnaryServerInterceptor

	// Metrics监控
	if registry.GetMetrics() != nil {
		m := registry.GetMetrics()
		metrics.GetOrRegister("grpc_tps", m)

		interceptor = append(interceptor, func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (resp interface{}, err error) {
			registry.GetMetrics().Mark(1)
			return handler(ctx, req)
		})
		interceptorStream = append(interceptorStream, func(srv interface{}, ss grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
			registry.GetMetrics().Mark(1)
			return handler(srv, ss)
		})

		if registry.IsShowMetricsLog() {
			go func() {
				metrics.Log(metrics.DefaultRegistry,
					30*time.Second,
					log.New(os.Stdout, "metrics: ", log.LstdFlags))
			}()
		}
	}

	if len(interceptor) > 0 {
		opt = append(opt, grpc.UnaryInterceptor(grpcMiddleware.ChainUnaryServer(interceptor...)))
	}
	if len(interceptorStream) > 0 {
		opt = append(opt, grpc.StreamInterceptor(grpcMiddleware.ChainStreamServer(interceptorStream...)))
	}

	server := &Server{
		registry:   registry,
		grpcServer: grpc.NewServer(opt...),
	}

	err := server.registry.Init()
	if err != nil {
		log.Panicln(err)
	}

	return server
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
func (s *Server) Serve(address string) {
	listener, err := net.Listen("tcp", address)
	if err != nil {
		log.Fatalf("rpc server net.Listen err: %v", err)
	}
	log.Printf("listening and serving grpc on %s\n", address)

	err = s.grpcServer.Serve(listener)
	if err != nil {
		log.Fatalf("grpcServer.Serve err: %v", err)
	}
}

// Stop 服务停止
func (s *Server) Stop() error {
	err := s.registry.Stop()
	if err != nil {
		return err
	}
	s.grpcServer.Stop()

	log.Println("grpc server stop")

	return nil
}
