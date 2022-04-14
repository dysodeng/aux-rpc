package registry

// Registry 服务注册接口
type Registry interface {
	// Register 服务注册
	// serviceName string 服务名称
	// metadata string 服务元数据
	Register(serviceName string, metadata string) error

	// Unregister 服务注销
	// serviceName string 服务名称
	Unregister(serviceName string) error

	// Stop 停止服务注册
	Stop() error

	// GetServiceListener 获取服务监听地址
	GetServiceListener() string
}
