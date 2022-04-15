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

// NewServer 新建服务注册
func NewServer(listen string, registry registry.Registry, opts ...Option) (*Server, error) {
	server := &Server{
		listen:   listen,
		registry: registry,
	}

	for _, opt := range opts {
		opt(server)
	}

	// Metrics监控
	if server.metrics != nil {
		metrics.GetOrRegister("grpc_tps", server.metrics)
		server.interceptor = append(server.interceptor, func(
			ctx context.Context,
			req interface{},
			info *grpc.UnaryServerInfo,
			handler grpc.UnaryHandler,
		) (resp interface{}, err error) {
			server.metrics.Mark(1)
			return handler(ctx, req)
		})
		server.interceptorStream = append(server.interceptorStream, func(
			srv interface{},
			ss grpc.ServerStream,
			info *grpc.StreamServerInfo,
			handler grpc.StreamHandler,
		) error {
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

	if len(server.interceptor) > 0 {
		server.grpcOptions = append(server.grpcOptions, grpc.UnaryInterceptor(
			grpcMiddleware.ChainUnaryServer(server.interceptor...)))
	}
	if len(server.interceptorStream) > 0 {
		server.grpcOptions = append(server.grpcOptions, grpc.StreamInterceptor(
			grpcMiddleware.ChainStreamServer(server.interceptorStream...)))
	}

	server.grpcServer = grpc.NewServer(server.grpcOptions...)

	return server, nil
}

// Register 注册服务
func (s *Server) Register(serviceName string, service interface{}, grpcRegister interface{}, metadata string) error {
	// 注册grpc服务
	fn := reflect.ValueOf(grpcRegister)
	if fn.Kind() != reflect.Func {
		return errors.New("`grpcRegister` is not a valid grpc registration function")
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
	listener, err := net.Listen("tcp", s.listen)
	if err != nil {
		return errors.Wrap(err, "gRPC server net.Listen err")
	}
	log.Printf("listening and serving gRPC on %s\n", s.listen)

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
