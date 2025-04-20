// plugin.go 定义了 Vivid 的插件系统接口和相关类型

package vivid

import (
	"fmt"
	"sync"
)

// Plugin 定义了 Vivid 插件的接口
// 所有插件必须实现这个接口才能被 ActorSystem 加载
type Plugin interface {
	// ID 返回插件的唯一标识符
	ID() string

	// Name 返回插件的名称
	Name() string

	// Version 返回插件的版本
	Version() string

	// Description 返回插件的描述
	Description() string

 // Initialize 在 ActorSystem 启动时被调用，用于初始化插件
	// 如果返回错误，ActorSystem 将不会加载该插件
	Initialize(system ActorSystem) error

	// Shutdown 在 ActorSystem 关闭时被调用，用于清理资源
	Shutdown() error
}

// PluginRegistry 管理已注册的插件
type PluginRegistry struct {
	mu      sync.RWMutex
	plugins map[string]Plugin
}

// newPluginRegistry 创建一个新的插件注册表
func newPluginRegistry() *PluginRegistry {
	return &PluginRegistry{
		plugins: make(map[string]Plugin),
	}
}

// Register 注册一个插件
// 如果已存在同ID的插件，则返回错误
func (r *PluginRegistry) Register(p Plugin) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	id := p.ID()
	if _, exists := r.plugins[id]; exists {
		return fmt.Errorf("plugin with ID '%s' already registered", id)
	}

	r.plugins[id] = p
	return nil
}

// Get 获取指定ID的插件
// 如果插件不存在，则返回nil和错误
func (r *PluginRegistry) Get(id string) (Plugin, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	p, exists := r.plugins[id]
	if !exists {
		return nil, fmt.Errorf("plugin with ID '%s' not found", id)
	}

	return p, nil
}

// GetAll 返回所有已注册的插件
func (r *PluginRegistry) GetAll() []Plugin {
	r.mu.RLock()
	defer r.mu.RUnlock()

	plugins := make([]Plugin, 0, len(r.plugins))
	for _, p := range r.plugins {
		plugins = append(plugins, p)
	}

	return plugins
}

// Initialize 初始化所有已注册的插件
// 如果任何插件初始化失败，则返回错误
func (r *PluginRegistry) Initialize(system ActorSystem) error {
	r.mu.RLock()
	defer r.mu.RUnlock()

	for id, p := range r.plugins {
		if err := p.Initialize(system); err != nil {
			return fmt.Errorf("failed to initialize plugin '%s': %w", id, err)
		}
	}

	return nil
}

// Shutdown 关闭所有已注册的插件
func (r *PluginRegistry) Shutdown() error {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var lastErr error
	for id, p := range r.plugins {
		if err := p.Shutdown(); err != nil {
			lastErr = fmt.Errorf("failed to shutdown plugin '%s': %w", id, err)
		}
	}

	return lastErr
}

// BasePlugin 提供了 Plugin 接口的基本实现
// 可以被嵌入到具体的插件实现中，以减少重复代码
type BasePlugin struct {
	id          string
	name        string
	version     string
	description string
}

// NewBasePlugin 创建一个新的基本插件
func NewBasePlugin(id, name, version, description string) BasePlugin {
	return BasePlugin{
		id:          id,
		name:        name,
		version:     version,
		description: description,
	}
}

// ID 返回插件的唯一标识符
func (p BasePlugin) ID() string {
	return p.id
}

// Name 返回插件的名称
func (p BasePlugin) Name() string {
	return p.name
}

// Version 返回插件的版本
func (p BasePlugin) Version() string {
	return p.version
}

// Description 返回插件的描述
func (p BasePlugin) Description() string {
	return p.description
}

// Initialize 基本实现，不做任何操作
func (p BasePlugin) Initialize(system ActorSystem) error {
	return nil
}

// Shutdown 基本实现，不做任何操作
func (p BasePlugin) Shutdown() error {
	return nil
}
