package feishu

import (
	"fmt"
	"log/slog"
	"time"

	"github.com/Shadow-Azure/Golem/internal/core"
	"github.com/Shadow-Azure/Golem/internal/plugin"
)

// FeishuConfig holds the configuration for the Feishu plugin.
type FeishuConfig struct {
	AppID             string `yaml:"app_id"`
	AppSecret         string `yaml:"app_secret"`
	VerificationToken string `yaml:"verification_token"`
	EncryptKey        string `yaml:"encrypt_key"`
}

// FeishuPlugin implements the ChannelPlugin interface for Feishu messaging.
type FeishuPlugin struct {
	config  FeishuConfig
	engine  core.EngineInterface
	logger  *slog.Logger
	dedup   *Deduplicator
	started bool
}

// NewFeishuPlugin creates a new FeishuPlugin with the given configuration.
func NewFeishuPlugin(config FeishuConfig) *FeishuPlugin {
	return &FeishuPlugin{
		config: config,
		logger: slog.Default().With("component", "feishu_plugin"),
		dedup:  NewDeduplicator(5 * time.Minute),
	}
}

// Name returns the plugin name.
func (p *FeishuPlugin) Name() string { return "feishu" }

// Version returns the plugin version.
func (p *FeishuPlugin) Version() string { return "1.0.0" }

// Initialize initializes the plugin with configuration.
func (p *FeishuPlugin) Initialize(config map[string]interface{}) error {
	p.logger.Info("Feishu plugin initialized",
		"app_id", p.config.AppID,
	)
	return nil
}

// Start starts the plugin.
func (p *FeishuPlugin) Start() error {
	if p.started {
		return fmt.Errorf("plugin already started")
	}

	p.started = true
	p.logger.Info("Feishu plugin started")
	return nil
}

// Stop stops the plugin gracefully.
func (p *FeishuPlugin) Stop() error {
	p.dedup.Stop()
	p.started = false
	p.logger.Info("Feishu plugin stopped")
	return nil
}

// HealthCheck returns the plugin health status.
func (p *FeishuPlugin) HealthCheck() plugin.HealthStatus {
	return plugin.HealthStatus{
		Healthy: p.started,
		Message: "Feishu plugin status",
	}
}

// GetChannelType returns the channel type identifier.
func (p *FeishuPlugin) GetChannelType() string {
	return "feishu"
}

// SendMessage sends a message through the Feishu channel.
func (p *FeishuPlugin) SendMessage(sessionID string, content string) error {
	p.logger.Info("sending message", "session_id", sessionID)
	return nil
}

// SendStreamingMessage sends a streaming message through the Feishu channel.
func (p *FeishuPlugin) SendStreamingMessage(sessionID string, stream <-chan core.StreamChunk) error {
	p.logger.Info("sending streaming message", "session_id", sessionID)
	return nil
}

// SetEngine sets the engine reference for the plugin.
func (p *FeishuPlugin) SetEngine(engine core.EngineInterface) {
	p.engine = engine
}

// Ensure FeishuPlugin implements plugin.ChannelPlugin.
var _ plugin.ChannelPlugin = (*FeishuPlugin)(nil)
