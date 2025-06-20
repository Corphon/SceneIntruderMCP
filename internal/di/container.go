// internal/di/container.go
package di

import (
	"sync"
)

// Container 是一个简单的依赖注入容器
type Container struct {
	services map[string]interface{}
	mutex    sync.RWMutex
}

// 全局容器实例（单例模式）
var (
	globalContainer *Container
	once            sync.Once
)

// NewContainer 创建一个新的依赖注入容器
func NewContainer() *Container {
	container := &Container{
		services: make(map[string]interface{}),
	}

	return container
}

// GetContainer 获取全局容器实例
func GetContainer() *Container {
	if globalContainer == nil {
		once.Do(func() {
			globalContainer = NewContainer()
		})
	}
	return globalContainer
}

// Register 在容器中注册一个服务实例
func (c *Container) Register(name string, service interface{}) {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	c.services[name] = service
}

// Get 从容器中获取一个服务实例
func (c *Container) Get(name string) interface{} {
	c.mutex.RLock()
	defer c.mutex.RUnlock()

	service, exists := c.services[name]
	if !exists {
		return nil
	}

	return service
}

// GetTyped 获取指定类型的服务实例（带类型转换的辅助方法）
func (c *Container) GetTyped(name string, defaultVal interface{}) interface{} {
	service := c.Get(name)
	if service == nil {
		return defaultVal
	}
	return service
}

// Has 检查容器中是否存在指定名称的服务
func (c *Container) Has(name string) bool {
	c.mutex.RLock()
	defer c.mutex.RUnlock()

	_, exists := c.services[name]
	return exists
}

// Remove 从容器中移除一个服务
func (c *Container) Remove(name string) {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	delete(c.services, name)
}

// Clear 清空容器中的所有服务
func (c *Container) Clear() {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	c.services = make(map[string]interface{})
}

// GetNames 获取所有已注册服务的名称
func (c *Container) GetNames() []string {
	c.mutex.RLock()
	defer c.mutex.RUnlock()

	names := make([]string, 0, len(c.services))
	for name := range c.services {
		names = append(names, name)
	}

	return names
}
