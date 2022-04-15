package etcdv3

import (
	"time"

	clientv3 "go.etcd.io/etcd/client/v3"
	grpcResolver "google.golang.org/grpc/resolver"
)

const (
	defaultResolveNowFreq = time.Minute * 30 // 强制 ResolveNow 默认间隔时长(分钟)
	defaultNamespace      = "grpc"           // 默认服务命名空间
)

// NewEtcdV3Builder new etcd Builder
func NewEtcdV3Builder(etcdClient *clientv3.Client, opts ...BuilderOption) *Builder {
	builder := &Builder{
		kv:                 etcdClient,
		serviceStore:       make(map[string]map[string]struct{}),
		namespace:          defaultNamespace,
		resolveNowFreqTime: defaultResolveNowFreq,
	}

	for _, opt := range opts {
		opt(builder)
	}

	grpcResolver.Register(builder)

	return builder
}
