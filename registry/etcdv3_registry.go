package registry

import (
	"context"
	"errors"
	"fmt"
	"github.com/rcrowley/go-metrics"
	"go.etcd.io/etcd/client/v3"
	"log"
	"strings"
	"sync"
	"time"
)

// EtcdV3Register implements etcd registry.
type EtcdV3Register struct {

	// service address, for example, 127.0.0.1:8972
	ServiceAddress string

	// etcd addresses
	EtcdServers []string

	// base path for dpcx server, for example example/dpcx
	BasePath string

	// Registered services
	services    map[string]service
	serviceLock sync.Mutex

	// 租约时长(秒)
	Lease     int64
	metasLock sync.RWMutex
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
func (register *EtcdV3Register) initEtcd() error {

	var err error
	var cli *clientv3.Client

	initOnceLock.Do(func() {
		cli, err = clientv3.New(clientv3.Config{
			Endpoints:   register.EtcdServers,
			DialTimeout: 5 * time.Second,
		})

		register.kv = cli
	})

	if err != nil {
		return err
	}

	return nil
}

// Init 初始化Etcd注册
func (register *EtcdV3Register) Init() error {
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
func (register *EtcdV3Register) Stop() error {
	// 注销所有服务
	for serviceName := range register.services {
		_ = register.Unregister(serviceName)
	}

	return register.kv.Close()
}

// Register 服务注册
func (register *EtcdV3Register) Register(serviceName string, metadata string) error {
	// 设置租约时间
	resp, err := register.kv.Grant(context.Background(), register.Lease)
	if err != nil {
		return err
	}

	serviceName = "/" + register.BasePath + "/" + serviceName + "/" + register.ServiceAddress

	register.serviceLock.Lock()
	defer func() {
		register.serviceLock.Unlock()
	}()

	// 注册服务并绑定租约
	_, err = register.kv.Put(context.Background(), serviceName, register.ServiceAddress, clientv3.WithLease(resp.ID))
	if err != nil {
		return err
	}

	// 设置续租 并定期发送续租请求(心跳)
	leaseRespChan, err := register.kv.KeepAlive(context.Background(), resp.ID)

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

	log.Printf("register grpc service: %s", serviceName)

	return nil
}

// Unregister 注销服务
func (register *EtcdV3Register) Unregister(serviceName string) error {
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
	if _, err := register.kv.Revoke(context.Background(), ser.leaseID); err != nil {
		return err
	}

	log.Printf("unregister service: %s", serviceName)

	return nil
}


// GetMetrics 获取Meter
func (register *EtcdV3Register) GetMetrics() metrics.Meter {
	return register.Metrics
}

// IsShowMetricsLog 是否显示监控日志
func (register *EtcdV3Register) IsShowMetricsLog() bool {
	return register.ShowMetricsLog
}
