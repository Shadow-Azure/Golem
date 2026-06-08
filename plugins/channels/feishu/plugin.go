package feishu

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	lark "github.com/larksuite/oapi-sdk-go/v3"
	larkcore "github.com/larksuite/oapi-sdk-go/v3/core"
	"github.com/larksuite/oapi-sdk-go/v3/event/dispatcher"
	larkim "github.com/larksuite/oapi-sdk-go/v3/service/im/v1"
	larkws "github.com/larksuite/oapi-sdk-go/v3/ws"

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
	client  *lark.Client
	wsCli   *larkws.Client
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
	// Create Lark client
	p.client = lark.NewClient(p.config.AppID, p.config.AppSecret)

	// Create event handler
	eventHandler := dispatcher.NewEventDispatcherHandler(p.config.VerificationToken, p.config.EncryptKey)
	eventHandler.OnP2MessageReceiveV1(func(ctx context.Context, event *larkim.P2MessageReceiveV1) error {
		return p.handleMessage(ctx, event)
	})

	// Create WebSocket client
	p.wsCli = larkws.NewClient(p.config.AppID, p.config.AppSecret,
		larkws.WithEventHandler(eventHandler),
		larkws.WithLogLevel(larkcore.LogLevelInfo),
	)

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

	// Start WebSocket connection in background
	go func() {
		p.logger.Info("starting Feishu WebSocket connection...")
		err := p.wsCli.Start(context.Background())
		if err != nil {
			p.logger.Error("Feishu WebSocket connection failed", "error", err)
		}
	}()

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
	// TODO: Implement actual message sending via Feishu API
	return nil
}

// SendStreamingMessage sends a streaming message through the Feishu channel.
func (p *FeishuPlugin) SendStreamingMessage(sessionID string, stream <-chan core.StreamChunk) error {
	p.logger.Info("sending streaming message", "session_id", sessionID)
	// TODO: Implement streaming message via Feishu Card Kit
	return nil
}

// SetEngine sets the engine reference for the plugin.
func (p *FeishuPlugin) SetEngine(engine core.EngineInterface) {
	p.engine = engine
}

// handleMessage handles incoming Feishu messages.
func (p *FeishuPlugin) handleMessage(ctx context.Context, event *larkim.P2MessageReceiveV1) error {
	// Extract message info
	msgID := event.Event.Message.MessageId
	chatID := event.Event.Message.ChatId
	chatType := event.Event.Message.ChatType
	senderID := event.Event.Sender.SenderId.OpenId

	p.logger.Info("received message",
		"msg_id", msgID,
		"chat_id", chatID,
		"chat_type", chatType,
		"sender_id", senderID,
	)

	// Deduplication check
	if p.dedup.IsDuplicate(msgID) {
		p.logger.Debug("duplicate message ignored", "msg_id", msgID)
		return nil
	}

	// Extract text content
	content := ""
	if event.Event.Message.Content != nil {
		// Parse JSON content to get text
		content = *event.Event.Message.Content
	}

	// Generate session ID
	sessionID := fmt.Sprintf("feishu:%s", senderID)
	if chatType == "group" {
		sessionID = fmt.Sprintf("feishu:group:%s:%s", chatID, senderID)
	}

	// Get or create session
	session, err := p.engine.GetSessionManager().GetOrCreateSession(sessionID, senderID, "feishu")
	if err != nil {
		p.logger.Error("failed to get session", "error", err)
		return err
	}

	// Add user message to session
	p.engine.GetSessionManager().AddMessage(session.ID, core.Message{
		Role:    "user",
		Content: content,
	})

	// Get LLM provider
	providerName := p.engine.GetConfig().LLM.DefaultProvider
	provider, exists := p.engine.GetPluginManager().GetProvider(providerName)
	if !exists {
		p.logger.Error("provider not found", "provider", providerName)
		return fmt.Errorf("provider not found: %s", providerName)
	}

	// Get session history
	history, _ := p.engine.GetSessionManager().GetHistory(session.ID, 50)

	// Call LLM
	response, err := provider.Chat(ctx, history, core.ChatConfig{})
	if err != nil {
		p.logger.Error("LLM error", "error", err)
		return err
	}

	// Add assistant message to session
	p.engine.GetSessionManager().AddMessage(session.ID, core.Message{
		Role:    "assistant",
		Content: response.Content,
	})

	// Send response back to Feishu
	return p.SendMessage(sessionID, response.Content)
}

// Ensure FeishuPlugin implements plugin.ChannelPlugin.
var _ plugin.ChannelPlugin = (*FeishuPlugin)(nil)
