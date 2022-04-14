package discovery

import (
	"context"
	"strings"
	"sync"
	"time"

	"github.com/pkg/errors"
	"go.etcd.io/etcd/api/v3/mvccpb"
	clientv3 "go.etcd.io/etcd/client/v3"
	"google.golang.org/grpc/resolver"
)

const defaultFreq = time.Minute * 30

// EtcdV3Discovery etcd服务发现
// implements grpc resolver.Builder
type EtcdV3Discovery struct {
	// etcd客户端连接
	kv *clientv3.Client
	// 全局服务快照表
	serviceStore map[string]map[string]struct{}
	// 服务快照表安全锁
	storeLock sync.Mutex
	// 基础路径
	basePath string
}

func NewEtcdV3Discovery(etcdClient *clientv3.Client, basePath string) *EtcdV3Discovery {

	d := &EtcdV3Discovery{
		kv:           etcdClient,
		serviceStore: make(map[string]map[string]struct{}),
		basePath:     basePath,
	}

	resolver.Register(d)

	return d
}

func (d *EtcdV3Discovery) Build(target resolver.Target, cc resolver.ClientConn, opts resolver.BuildOptions) (resolver.Resolver, error) {

	d.storeLock.Lock()
	d.serviceStore[target.Endpoint] = make(map[string]struct{})
	d.storeLock.Unlock()

	r := &etcdResolver{
		kv:       d.kv,
		target:   target,
		cc:       cc,
		store:    d.serviceStore[target.Endpoint],
		basePath: d.basePath,
		stopCh:   make(chan struct{}, 1),
		rn:       make(chan struct{}, 1),
		t:        time.NewTicker(defaultFreq),
	}

	go r.start(context.Background())

	r.ResolveNow(resolver.ResolveNowOptions{})

	return r, nil
}

func (d *EtcdV3Discovery) Scheme() string {
	return "etcdV3"
}

type etcdResolver struct {
	kv        *clientv3.Client
	target    resolver.Target
	cc        resolver.ClientConn
	store     map[string]struct{}
	basePath  string
	storeLock sync.Mutex
	stopCh    chan struct{}
	// rn channel is used by ResolveNow() to force an immediate resolution of the target.
	rn chan struct{}
	t  *time.Ticker
}

func (r *etcdResolver) start(ctx context.Context) {
	prefix := "/" + r.basePath + "/" + r.target.Endpoint + "/"
	rch := r.kv.Watch(ctx, prefix, clientv3.WithPrefix())
	for {
		select {
		case <-r.rn:
			r.resolveNow()
		case <-r.t.C:
			r.ResolveNow(resolver.ResolveNowOptions{})
		case <-r.stopCh:
			return
		case wresp := <-rch:
			for _, ev := range wresp.Events {
				switch ev.Type {
				case mvccpb.PUT:
					r.storeLock.Lock()
					r.store[string(ev.Kv.Value)] = struct{}{}
					r.storeLock.Unlock()
					r.updateTargetState()
				case mvccpb.DELETE:
					r.storeLock.Lock()
					delete(r.store, strings.Replace(string(ev.Kv.Key), prefix, "", 1))
					r.storeLock.Unlock()
					r.updateTargetState()
				}
			}
		}
	}
}

func (r *etcdResolver) resolveNow() {
	prefix := "/" + r.basePath + "/" + r.target.Endpoint + "/"
	resp, err := r.kv.Get(context.Background(), prefix, clientv3.WithPrefix())
	if err != nil {
		r.cc.ReportError(errors.Wrap(err, "get init endpoints"))
		return
	}

	for _, kv := range resp.Kvs {
		r.storeLock.Lock()
		r.store[string(kv.Value)] = struct{}{}
		r.storeLock.Unlock()
	}

	r.updateTargetState()
}

func (r *etcdResolver) updateTargetState() {
	addrs := make([]resolver.Address, len(r.store))
	i := 0
	for k := range r.store {
		addrs[i] = resolver.Address{Addr: k}
		i++
	}

	_ = r.cc.UpdateState(resolver.State{Addresses: addrs})
}

func (r *etcdResolver) ResolveNow(opt resolver.ResolveNowOptions) {
	select {
	case r.rn <- struct{}{}:
	default:
	}
}

func (r *etcdResolver) Close() {
	r.t.Stop()
	close(r.stopCh)
}
