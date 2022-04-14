package etcdv3

import (
	"context"
	clientv3 "go.etcd.io/etcd/client/v3"
	grpcResolver "google.golang.org/grpc/resolver"
	"sync"
	"time"
)

// Builder grpc etcd服务发现
// implements grpc resolver.Builder
type Builder struct {
	// etcd客户端连接
	kv *clientv3.Client
	// 全局服务快照表
	serviceStore map[string]map[string]struct{}
	// 服务快照表安全锁
	storeLock sync.Mutex
	// 命名空间
	namespace string
	// 强制 ResolveNow 间隔时长
	resolveNowFreqTime time.Duration
}

// BuilderOption builder option
type BuilderOption func(builder *Builder)

// WithNamespace 设置命名空间
func WithNamespace(namespace string) BuilderOption {
	return func(builder *Builder) {
		builder.namespace = namespace
	}
}

// WithResolveNowTime 设置强制 ResolveNow 间隔时长
func WithResolveNowTime(t time.Duration) BuilderOption {
	return func(builder *Builder) {
		builder.resolveNowFreqTime = t
	}
}

// Build creates a new resolver for the given target.
func (d *Builder) Build(target grpcResolver.Target, cc grpcResolver.ClientConn, opts grpcResolver.BuildOptions) (grpcResolver.Resolver, error) {

	d.storeLock.Lock()
	d.serviceStore[target.Endpoint] = make(map[string]struct{})
	d.storeLock.Unlock()

	r := &resolver{
		kv:        d.kv,
		target:    target,
		cc:        cc,
		store:     d.serviceStore[target.Endpoint],
		namespace: d.namespace,
		stopCh:    make(chan struct{}, 1),
		rn:        make(chan struct{}, 1),
		t:         time.NewTicker(d.resolveNowFreqTime),
	}

	go r.start(context.Background())

	r.ResolveNow(grpcResolver.ResolveNowOptions{})

	return r, nil
}

// Scheme returns the scheme supported by this resolver.
func (d *Builder) Scheme() string {
	return "etcdV3"
}
