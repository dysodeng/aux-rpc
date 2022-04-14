package etcdv3

import (
	clientv3 "go.etcd.io/etcd/client/v3"
	"sync"
)

// etcdV3 implements etcd registry.
type etcdV3 struct {
	// service address, for example, 127.0.0.1:8972
	serviceAddress string

	// etcd addresses
	etcdServers []string

	// namespace for auxrcx server, eg. example/auxrcx
	namespace string

	// Registered services
	services    map[string]service
	serviceLock sync.Mutex

	// 租约时长(秒)
	lease     int64
	metasLock sync.Mutex
	metas     map[string]string

	// etcd client
	kv *clientv3.Client
}

// RegistryOption registry option.
type RegistryOption func(v3 *etcdV3)

// WithNamespace 设置命名空间
func WithNamespace(namespace string) RegistryOption {
	return func(v3 *etcdV3) {
		v3.namespace = namespace
	}
}

// WithLease 设置服务租约时长(秒)
func WithLease(lease int64) RegistryOption {
	return func(v3 *etcdV3) {
		v3.lease = lease
	}
}

// service 注册服务类型
type service struct {
	// 服务标识key
	key string
	// 服务host
	host string
	// 租约ID
	leaseID clientv3.LeaseID
	// 租约KeepAlive
	keepAliveChan <-chan *clientv3.LeaseKeepAliveResponse
}
