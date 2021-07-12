package registry

import "github.com/rcrowley/go-metrics"

// Registry 服务注册接口
type Registry interface {

	// Init 初始化服务注册
	Init() error

	// Stop 停止服务注册
	Stop() error

	// Register 服务注册
	// serviceName string 服务名称
	// metadata string 服务元数据
	Register(serviceName string, metadata string) error

	// Unregister 服务注销
	// serviceName string 服务名称
	Unregister(serviceName string) error

	// GetMetrics 获取Meter
	GetMetrics() metrics.Meter

	// IsShowMetricsLog 是否显示监控日志
	IsShowMetricsLog() bool
}
