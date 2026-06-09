package plugin

import (
	"fmt"
	"log/slog"
	"sync"
)

// ManagerInterface defines the contract for plugin management.
type ManagerInterface interface {
	LoadPlugin(name string, plugin Plugin) error
	UnloadPlugin(name string) error
	GetPlugin(name string) (Plugin, bool)
	GetChannel(channelType string) (ChannelPlugin, bool)
	GetProvider(providerType string) (ProviderPlugin, bool)
	GetTool(name string) (ToolPlugin, bool)
	ListPlugins() []PluginInfo
	StartAll() error
	StopAll() error
	HealthCheckAll() map[string]HealthStatus
}

// Manager handles loading, unloading, and retrieving plugins.
type Manager struct {
	plugins   map[string]Plugin
	channels  map[string]ChannelPlugin
	providers map[string]ProviderPlugin
	tools     map[string]ToolPlugin
	mu        sync.RWMutex
	logger    *slog.Logger
}

// NewManager creates a new plugin Manager.
func NewManager() *Manager {
	return &Manager{
		plugins:   make(map[string]Plugin),
		channels:  make(map[string]ChannelPlugin),
		providers: make(map[string]ProviderPlugin),
		tools:     make(map[string]ToolPlugin),
		logger:    slog.Default().With("component", "plugin_manager"),
	}
}

// LoadPlugin registers and initializes a plugin.
func (m *Manager) LoadPlugin(name string, plugin Plugin) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.plugins[name]; exists {
		return fmt.Errorf("plugin already loaded: %s", name)
	}

	if err := plugin.Initialize(nil); err != nil {
		return fmt.Errorf("failed to initialize plugin %s: %w", name, err)
	}

	m.plugins[name] = plugin

	if ch, ok := plugin.(ChannelPlugin); ok {
		m.channels[ch.GetChannelType()] = ch
		m.logger.Info("loaded channel plugin", "name", name, "channel_type", ch.GetChannelType())
	}
	if prov, ok := plugin.(ProviderPlugin); ok {
		m.providers[prov.GetProviderType()] = prov
		m.logger.Info("loaded provider plugin", "name", name, "provider_type", prov.GetProviderType())
	}
	if tool, ok := plugin.(ToolPlugin); ok {
		m.tools[tool.GetToolDefinition().Name] = tool
		m.logger.Info("loaded tool plugin", "name", name)
	}

	return nil
}

// UnloadPlugin stops and removes a plugin.
func (m *Manager) UnloadPlugin(name string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	plugin, exists := m.plugins[name]
	if !exists {
		return fmt.Errorf("plugin not found: %s", name)
	}

	if err := plugin.Stop(); err != nil {
		m.logger.Error("error stopping plugin", "name", name, "error", err)
	}

	if ch, ok := plugin.(ChannelPlugin); ok {
		delete(m.channels, ch.GetChannelType())
	}
	if prov, ok := plugin.(ProviderPlugin); ok {
		delete(m.providers, prov.GetProviderType())
	}
	if tool, ok := plugin.(ToolPlugin); ok {
		delete(m.tools, tool.GetToolDefinition().Name)
	}

	delete(m.plugins, name)
	m.logger.Info("unloaded plugin", "name", name)

	return nil
}

// GetPlugin retrieves a plugin by name.
func (m *Manager) GetPlugin(name string) (Plugin, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	plugin, exists := m.plugins[name]
	return plugin, exists
}

// GetChannel retrieves a channel plugin by channel type.
func (m *Manager) GetChannel(channelType string) (ChannelPlugin, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	channel, exists := m.channels[channelType]
	return channel, exists
}

// GetProvider retrieves a provider plugin by provider type.
func (m *Manager) GetProvider(providerType string) (ProviderPlugin, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	provider, exists := m.providers[providerType]
	return provider, exists
}

// GetTool retrieves a tool plugin by tool name.
func (m *Manager) GetTool(name string) (ToolPlugin, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	tool, exists := m.tools[name]
	return tool, exists
}

// ListPlugins returns metadata for all loaded plugins.
func (m *Manager) ListPlugins() []PluginInfo {
	m.mu.RLock()
	defer m.mu.RUnlock()

	infos := make([]PluginInfo, 0, len(m.plugins))
	for name, plugin := range m.plugins {
		info := PluginInfo{
			Name:    name,
			Version: plugin.Version(),
			Status:  "loaded",
		}

		if _, ok := plugin.(ChannelPlugin); ok {
			info.Type = "channel"
		} else if _, ok := plugin.(ProviderPlugin); ok {
			info.Type = "provider"
		} else if _, ok := plugin.(ToolPlugin); ok {
			info.Type = "tool"
		} else {
			info.Type = "generic"
		}

		infos = append(infos, info)
	}

	return infos
}

// StartAll starts every loaded plugin.
func (m *Manager) StartAll() error {
	m.mu.RLock()
	defer m.mu.RUnlock()

	for name, plugin := range m.plugins {
		if err := plugin.Start(); err != nil {
			return fmt.Errorf("failed to start plugin %s: %w", name, err)
		}
		m.logger.Info("started plugin", "name", name)
	}

	return nil
}

// StopAll stops every loaded plugin.
func (m *Manager) StopAll() error {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var errs []error
	for name, plugin := range m.plugins {
		if err := plugin.Stop(); err != nil {
			errs = append(errs, fmt.Errorf("failed to stop plugin %s: %w", name, err))
		}
		m.logger.Info("stopped plugin", "name", name)
	}

	if len(errs) > 0 {
		return fmt.Errorf("errors stopping plugins: %v", errs)
	}

	return nil
}

// HealthCheckAll returns health status for every loaded plugin.
func (m *Manager) HealthCheckAll() map[string]HealthStatus {
	m.mu.RLock()
	defer m.mu.RUnlock()

	status := make(map[string]HealthStatus, len(m.plugins))
	for name, plugin := range m.plugins {
		status[name] = plugin.HealthCheck()
	}

	return status
}

// Compile-time interface check.
var _ ManagerInterface = (*Manager)(nil)
