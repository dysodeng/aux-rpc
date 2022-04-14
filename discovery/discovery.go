package discovery

import (
	"context"
	"fmt"

	"google.golang.org/grpc"
	"google.golang.org/grpc/resolver"
)

type option struct {
	// load balancing 负载均衡
	lb              string
	grpcDialOptions []grpc.DialOption
}

type Option func(o *option)

// WithLoadBalance 设置负载均衡
func WithLoadBalance(lb string) Option {
	return func(o *option) {
		o.lb = lb
	}
}

// WithGrpcDialOption 设置grpc连接选项
func WithGrpcDialOption(opts ...grpc.DialOption) Option {
	return func(o *option) {
		if len(opts) > 0 {
			o.grpcDialOptions = opts
		}
	}
}

// Discovery 服务发现
func Discovery(ctx context.Context, builder resolver.Builder, serviceName string, opts ...Option) (*grpc.ClientConn, error) {
	o := &option{
		lb: "round_robin",
	}
	for _, opt := range opts {
		opt(o)
	}

	if o.lb != "" {
		o.grpcDialOptions = append(o.grpcDialOptions, grpc.WithDefaultServiceConfig(fmt.Sprintf(`{"LoadBalancingPolicy": "%s"}`, o.lb)))
	}

	// 连接rpc服务
	conn, err := grpc.DialContext(
		ctx,
		fmt.Sprintf("%s:///%s", builder.Scheme(), serviceName),
		o.grpcDialOptions...,
	)

	if err != nil {
		return nil, err
	}

	return conn, nil
}
