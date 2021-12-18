package registry

import (
	"context"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	"github.com/pkg/errors"

	"github.com/rcrowley/go-metrics"
	clientv3 "go.etcd.io/etcd/client/v3"
)

// EtcdV3Registry implements etcd registry.
type EtcdV3Registry struct {

	// service address, for example, 127.0.0.1:8972
	ServiceAddress string

	// etcd addresses
	EtcdServers []string

	// base path for auxrcx server, for example example/auxrcx
	BasePath string

	// Registered services
	services    map[string]service
	serviceLock sync.Mutex

	// 租约时长(秒)
	Lease     int64
	metasLock sync.Mutex
	metas     map[string]string

	// Metrics 监控
	Metrics        metrics.Meter
	ShowMetricsLog bool

	// etcd client
	kv *clientv3.Client
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

var initOnceLock sync.Once

// initEtcd 初始化etcd连接
func (register *EtcdV3Registry) initEtcd() error {

	var err error
	var cli *clientv3.Client

	initOnceLock.Do(func() {
		cli, err = clientv3.New(clientv3.Config{
			Endpoints:   register.EtcdServers,
			DialTimeout: 5 * time.Second,
		})

		timeoutCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_, err = cli.Status(timeoutCtx, register.EtcdServers[0])

		register.kv = cli
	})

	if err != nil {
		return errors.Wrap(err, "could not connect to etcd")
	}

	return nil
}

// Init 初始化Etcd注册
func (register *EtcdV3Registry) Init() error {
	err := register.initEtcd()
	if err != nil {
		return err
	}
	if register.services == nil {
		register.services = make(map[string]service)
	}
	if register.metas == nil {
		register.metas = make(map[string]string)
	}

	return nil
}

// Stop 停止服务注册
func (register *EtcdV3Registry) Stop() error {
	// 注销所有服务
	for serviceName := range register.services {
		_ = register.Unregister(serviceName)
	}

	return register.kv.Close()
}

// Register 服务注册
func (register *EtcdV3Registry) Register(serviceName string, metadata string) error {

	ctx := context.Background()

	// 设置租约时间
	resp, err := register.kv.Grant(ctx, register.Lease)
	if err != nil {
		return err
	}

	serviceName = "/" + register.BasePath + "/" + serviceName + "/" + register.ServiceAddress

	register.serviceLock.Lock()
	defer func() {
		register.serviceLock.Unlock()
	}()

	// 注册服务并绑定租约
	_, err = register.kv.Put(ctx, serviceName, register.ServiceAddress, clientv3.WithLease(resp.ID))
	if err != nil {
		return err
	}

	// 设置续租 并定期发送续租请求(心跳)
	leaseRespChan, err := register.kv.KeepAlive(ctx, resp.ID)

	if err != nil {
		return err
	}

	if register.services == nil {
		register.services = make(map[string]service)
	}
	register.services[serviceName] = service{
		host:          register.ServiceAddress,
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
func (register *EtcdV3Registry) Unregister(serviceName string) error {
	if "" == strings.TrimSpace(serviceName) {
		return errors.New("register service `name` can't be empty")
	}

	err := register.initEtcd()
	if err != nil {
		return err
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
	if _, err = register.kv.Revoke(context.Background(), ser.leaseID); err != nil {
		return err
	}

	log.Printf("unregister gRPC service: %s", serviceName)

	return nil
}

// GetMetrics 获取Meter
func (register *EtcdV3Registry) GetMetrics() metrics.Meter {
	return register.Metrics
}

// IsShowMetricsLog 是否显示监控日志
func (register *EtcdV3Registry) IsShowMetricsLog() bool {
	return register.ShowMetricsLog
}
