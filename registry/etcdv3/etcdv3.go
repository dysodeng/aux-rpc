package etcdv3

import (
	"context"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	"github.com/dysodeng/aux-rpc/registry"

	"github.com/pkg/errors"

	clientv3 "go.etcd.io/etcd/client/v3"
)

var initOnceLock sync.Once

const (
	defaultLease     = 5      // 默认服务租约时长(秒)
	defaultTimeout   = 5      // 默认etcd连接超时时长(秒)
	defaultNamespace = "grpc" // 默认服务命名空间
)

// NewEtcdV3Registry new etcd registry
func NewEtcdV3Registry(serviceListenAddress string, etcdAddress []string, opts ...RegistryOption) (registry.Registry, error) {
	v3 := &etcdV3{
		serviceAddress: serviceListenAddress,
		etcdServers:    etcdAddress,
		namespace:      defaultNamespace,
		services:       make(map[string]service),
		lease:          defaultLease,
		metas:          make(map[string]string),
	}

	for _, opt := range opts {
		opt(v3)
	}

	var err error
	var cli *clientv3.Client

	initOnceLock.Do(func() {
		cli, err = clientv3.New(clientv3.Config{
			Endpoints:   v3.etcdServers,
			DialTimeout: defaultTimeout * time.Second,
		})

		timeoutCtx, cancel := context.WithTimeout(context.Background(), defaultTimeout*time.Second)
		defer cancel()
		_, err = cli.Status(timeoutCtx, v3.etcdServers[0])

		v3.kv = cli
	})

	if err != nil {
		return nil, errors.Wrap(err, "could not connect to etcd")
	}

	return v3, nil
}

// Register 服务注册
func (register *etcdV3) Register(serviceName, metadata string) error {

	ctx := context.Background()

	// 设置租约时间
	resp, err := register.kv.Grant(ctx, register.lease)
	if err != nil {
		return err
	}

	serviceName = "/" + register.namespace + "/" + serviceName + "/" + register.serviceAddress

	register.serviceLock.Lock()
	defer func() {
		register.serviceLock.Unlock()
	}()

	// 注册服务并绑定租约
	_, err = register.kv.Put(ctx, serviceName, register.serviceAddress, clientv3.WithLease(resp.ID))
	if err != nil {
		return err
	}

	// 设置续租 并定期发送续租请求(心跳)
	leaseRespChan, err := register.kv.KeepAlive(ctx, resp.ID)

	if err != nil {
		return err
	}

	register.services[serviceName] = service{
		host:          register.serviceAddress,
		key:           serviceName,
		leaseID:       resp.ID,
		keepAliveChan: leaseRespChan,
	}

	go func() {
		for {
			<-leaseRespChan
		}
	}()

	register.metasLock.Lock()
	if register.metas == nil {
		register.metas = make(map[string]string)
	}
	register.metas[serviceName] = metadata
	register.metasLock.Unlock()

	log.Printf("register gRPC service: %s", serviceName)

	return nil
}

// Unregister 注销服务
func (register *etcdV3) Unregister(serviceName string) error {
	if "" == strings.TrimSpace(serviceName) {
		return errors.New("register service `name` can't be empty")
	}

	var ser service
	var ok bool

	if ser, ok = register.services[serviceName]; !ok {
		return errors.New(fmt.Sprintf("service `%s` not registered", serviceName))
	}

	register.serviceLock.Lock()
	defer func() {
		register.serviceLock.Unlock()
	}()

	// 撤销租约
	if _, err := register.kv.Revoke(context.Background(), ser.leaseID); err != nil {
		return err
	}

	log.Printf("unregister gRPC service: %s", serviceName)

	return nil
}

// Stop 停止服务注册
func (register *etcdV3) Stop() error {
	// 注销所有服务
	for serviceName := range register.services {
		_ = register.Unregister(serviceName)
	}

	return register.kv.Close()
}
