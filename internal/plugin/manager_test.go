package plugin

import (
	"context"
	"testing"

	"github.com/Shadow-Azure/Golem/internal/core"
)

// MockPlugin implements Plugin for testing.
type MockPlugin struct {
	name        string
	version     string
	initialized bool
	started     bool
	stopped     bool
}

func (p *MockPlugin) Name() string    { return p.name }
func (p *MockPlugin) Version() string { return p.version }
func (p *MockPlugin) Initialize(config map[string]interface{}) error {
	p.initialized = true
	return nil
}
func (p *MockPlugin) Start() error {
	p.started = true
	return nil
}
func (p *MockPlugin) Stop() error {
	p.stopped = true
	return nil
}
func (p *MockPlugin) HealthCheck() HealthStatus {
	return HealthStatus{Healthy: true}
}

// MockChannelPlugin implements ChannelPlugin for testing.
type MockChannelPlugin struct {
	MockPlugin
	channelType string
}

func (p *MockChannelPlugin) SendMessage(sessionID, content string) error { return nil }
func (p *MockChannelPlugin) SendStreamingMessage(sessionID string, stream <-chan core.StreamChunk) error {
	return nil
}
func (p *MockChannelPlugin) GetChannelType() string { return p.channelType }

// MockProviderPlugin implements ProviderPlugin for testing.
type MockProviderPlugin struct {
	MockPlugin
	providerType string
}

func (p *MockProviderPlugin) Chat(ctx context.Context, messages []core.Message, config core.ChatConfig) (*core.ChatResponse, error) {
	return &core.ChatResponse{Content: "test"}, nil
}
func (p *MockProviderPlugin) ChatStream(ctx context.Context, messages []core.Message, config core.ChatConfig) (<-chan core.StreamChunk, error) {
	ch := make(chan core.StreamChunk, 1)
	ch <- core.StreamChunk{Content: "test", Done: true}
	close(ch)
	return ch, nil
}
func (p *MockProviderPlugin) GetProviderType() string { return p.providerType }
func (p *MockProviderPlugin) SupportsStreaming() bool  { return true }

// MockToolPlugin implements ToolPlugin for testing.
type MockToolPlugin struct {
	MockPlugin
	toolDef core.ToolDefinition
}

func (p *MockToolPlugin) Execute(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	return "result", nil
}
func (p *MockToolPlugin) GetToolDefinition() core.ToolDefinition { return p.toolDef }

func TestPluginManager_LoadPlugin(t *testing.T) {
	pm := NewManager()
	plugin := &MockPlugin{name: "test", version: "1.0.0"}

	err := pm.LoadPlugin("test", plugin)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !plugin.initialized {
		t.Error("plugin should have been initialized")
	}
}

func TestPluginManager_LoadPlugin_Duplicate(t *testing.T) {
	pm := NewManager()
	plugin1 := &MockPlugin{name: "test", version: "1.0.0"}
	plugin2 := &MockPlugin{name: "test", version: "1.0.0"}

	if err := pm.LoadPlugin("test", plugin1); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	err := pm.LoadPlugin("test", plugin2)
	if err == nil {
		t.Fatal("expected error for duplicate plugin")
	}
}

func TestPluginManager_GetPlugin(t *testing.T) {
	pm := NewManager()
	plugin := &MockPlugin{name: "test", version: "1.0.0"}
	pm.LoadPlugin("test", plugin)

	retrieved, exists := pm.GetPlugin("test")
	if !exists {
		t.Fatal("plugin should exist")
	}
	if retrieved.Name() != "test" {
		t.Errorf("expected name 'test', got '%s'", retrieved.Name())
	}
}

func TestPluginManager_GetPlugin_NotFound(t *testing.T) {
	pm := NewManager()

	_, exists := pm.GetPlugin("nonexistent")
	if exists {
		t.Error("plugin should not exist")
	}
}

func TestPluginManager_GetChannel(t *testing.T) {
	pm := NewManager()
	channel := &MockChannelPlugin{
		MockPlugin:  MockPlugin{name: "feishu", version: "1.0.0"},
		channelType: "feishu",
	}
	pm.LoadPlugin("feishu", channel)

	retrieved, exists := pm.GetChannel("feishu")
	if !exists {
		t.Fatal("channel should exist")
	}
	if retrieved.GetChannelType() != "feishu" {
		t.Errorf("expected channel type 'feishu', got '%s'", retrieved.GetChannelType())
	}
}

func TestPluginManager_GetChannel_NotFound(t *testing.T) {
	pm := NewManager()

	_, exists := pm.GetChannel("nonexistent")
	if exists {
		t.Error("channel should not exist")
	}
}

func TestPluginManager_GetProvider(t *testing.T) {
	pm := NewManager()
	provider := &MockProviderPlugin{
		MockPlugin:   MockPlugin{name: "openai", version: "1.0.0"},
		providerType: "openai",
	}
	pm.LoadPlugin("openai", provider)

	retrieved, exists := pm.GetProvider("openai")
	if !exists {
		t.Fatal("provider should exist")
	}
	if retrieved.GetProviderType() != "openai" {
		t.Errorf("expected provider type 'openai', got '%s'", retrieved.GetProviderType())
	}
}

func TestPluginManager_GetProvider_NotFound(t *testing.T) {
	pm := NewManager()

	_, exists := pm.GetProvider("nonexistent")
	if exists {
		t.Error("provider should not exist")
	}
}

func TestPluginManager_GetTool(t *testing.T) {
	pm := NewManager()
	tool := &MockToolPlugin{
		MockPlugin: MockPlugin{name: "calculator", version: "1.0.0"},
		toolDef:    core.ToolDefinition{Name: "calculator", Description: "A calculator"},
	}
	pm.LoadPlugin("calculator", tool)

	retrieved, exists := pm.GetTool("calculator")
	if !exists {
		t.Fatal("tool should exist")
	}
	if retrieved.GetToolDefinition().Name != "calculator" {
		t.Errorf("expected tool name 'calculator', got '%s'", retrieved.GetToolDefinition().Name)
	}
}

func TestPluginManager_GetTool_NotFound(t *testing.T) {
	pm := NewManager()

	_, exists := pm.GetTool("nonexistent")
	if exists {
		t.Error("tool should not exist")
	}
}

func TestPluginManager_StartAll(t *testing.T) {
	pm := NewManager()
	plugin1 := &MockPlugin{name: "test1", version: "1.0.0"}
	plugin2 := &MockPlugin{name: "test2", version: "1.0.0"}

	pm.LoadPlugin("test1", plugin1)
	pm.LoadPlugin("test2", plugin2)

	err := pm.StartAll()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !plugin1.started {
		t.Error("plugin1 should have been started")
	}
	if !plugin2.started {
		t.Error("plugin2 should have been started")
	}
}

func TestPluginManager_StopAll(t *testing.T) {
	pm := NewManager()
	plugin1 := &MockPlugin{name: "test1", version: "1.0.0"}
	plugin2 := &MockPlugin{name: "test2", version: "1.0.0"}

	pm.LoadPlugin("test1", plugin1)
	pm.LoadPlugin("test2", plugin2)
	pm.StartAll()

	err := pm.StopAll()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !plugin1.stopped {
		t.Error("plugin1 should have been stopped")
	}
	if !plugin2.stopped {
		t.Error("plugin2 should have been stopped")
	}
}

func TestPluginManager_UnloadPlugin(t *testing.T) {
	pm := NewManager()
	plugin := &MockPlugin{name: "test", version: "1.0.0"}
	pm.LoadPlugin("test", plugin)

	err := pm.UnloadPlugin("test")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	_, exists := pm.GetPlugin("test")
	if exists {
		t.Error("plugin should not exist after unload")
	}
}

func TestPluginManager_UnloadPlugin_NotFound(t *testing.T) {
	pm := NewManager()

	err := pm.UnloadPlugin("nonexistent")
	if err == nil {
		t.Fatal("expected error for nonexistent plugin")
	}
}

func TestPluginManager_UnloadChannel(t *testing.T) {
	pm := NewManager()
	channel := &MockChannelPlugin{
		MockPlugin:  MockPlugin{name: "feishu", version: "1.0.0"},
		channelType: "feishu",
	}
	pm.LoadPlugin("feishu", channel)

	err := pm.UnloadPlugin("feishu")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	_, exists := pm.GetChannel("feishu")
	if exists {
		t.Error("channel should not exist after unload")
	}
}

func TestPluginManager_UnloadProvider(t *testing.T) {
	pm := NewManager()
	provider := &MockProviderPlugin{
		MockPlugin:   MockPlugin{name: "openai", version: "1.0.0"},
		providerType: "openai",
	}
	pm.LoadPlugin("openai", provider)

	err := pm.UnloadPlugin("openai")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	_, exists := pm.GetProvider("openai")
	if exists {
		t.Error("provider should not exist after unload")
	}
}

func TestPluginManager_UnloadTool(t *testing.T) {
	pm := NewManager()
	tool := &MockToolPlugin{
		MockPlugin: MockPlugin{name: "calculator", version: "1.0.0"},
		toolDef:    core.ToolDefinition{Name: "calculator", Description: "A calculator"},
	}
	pm.LoadPlugin("calculator", tool)

	err := pm.UnloadPlugin("calculator")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	_, exists := pm.GetTool("calculator")
	if exists {
		t.Error("tool should not exist after unload")
	}
}

func TestPluginManager_ListPlugins(t *testing.T) {
	pm := NewManager()
	plugin1 := &MockPlugin{name: "test1", version: "1.0.0"}
	plugin2 := &MockPlugin{name: "test2", version: "1.0.0"}

	pm.LoadPlugin("test1", plugin1)
	pm.LoadPlugin("test2", plugin2)

	plugins := pm.ListPlugins()
	if len(plugins) != 2 {
		t.Errorf("expected 2 plugins, got %d", len(plugins))
	}
}

func TestPluginManager_ListPlugins_Types(t *testing.T) {
	pm := NewManager()

	channel := &MockChannelPlugin{
		MockPlugin:  MockPlugin{name: "feishu", version: "1.0.0"},
		channelType: "feishu",
	}
	provider := &MockProviderPlugin{
		MockPlugin:   MockPlugin{name: "openai", version: "1.0.0"},
		providerType: "openai",
	}
	tool := &MockToolPlugin{
		MockPlugin: MockPlugin{name: "calculator", version: "1.0.0"},
		toolDef:    core.ToolDefinition{Name: "calculator", Description: "A calculator"},
	}
	generic := &MockPlugin{name: "generic", version: "1.0.0"}

	pm.LoadPlugin("feishu", channel)
	pm.LoadPlugin("openai", provider)
	pm.LoadPlugin("calculator", tool)
	pm.LoadPlugin("generic", generic)

	infos := pm.ListPlugins()
	if len(infos) != 4 {
		t.Fatalf("expected 4 plugins, got %d", len(infos))
	}

	typeMap := make(map[string]string)
	for _, info := range infos {
		typeMap[info.Name] = info.Type
	}

	if typeMap["feishu"] != "channel" {
		t.Errorf("expected feishu to be channel, got %s", typeMap["feishu"])
	}
	if typeMap["openai"] != "provider" {
		t.Errorf("expected openai to be provider, got %s", typeMap["openai"])
	}
	if typeMap["calculator"] != "tool" {
		t.Errorf("expected calculator to be tool, got %s", typeMap["calculator"])
	}
	if typeMap["generic"] != "generic" {
		t.Errorf("expected generic to be generic, got %s", typeMap["generic"])
	}
}

func TestPluginManager_HealthCheckAll(t *testing.T) {
	pm := NewManager()
	plugin := &MockPlugin{name: "test", version: "1.0.0"}
	pm.LoadPlugin("test", plugin)

	status := pm.HealthCheckAll()
	if len(status) != 1 {
		t.Errorf("expected 1 health status, got %d", len(status))
	}
	if !status["test"].Healthy {
		t.Error("plugin should be healthy")
	}
}

func TestPluginManager_HealthCheckAll_Multiple(t *testing.T) {
	pm := NewManager()
	plugin1 := &MockPlugin{name: "test1", version: "1.0.0"}
	plugin2 := &MockPlugin{name: "test2", version: "1.0.0"}

	pm.LoadPlugin("test1", plugin1)
	pm.LoadPlugin("test2", plugin2)

	status := pm.HealthCheckAll()
	if len(status) != 2 {
		t.Errorf("expected 2 health statuses, got %d", len(status))
	}
	if !status["test1"].Healthy {
		t.Error("test1 should be healthy")
	}
	if !status["test2"].Healthy {
		t.Error("test2 should be healthy")
	}
}
