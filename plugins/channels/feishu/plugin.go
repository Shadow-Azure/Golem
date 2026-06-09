package feishu

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"sync"
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
	config      FeishuConfig
	client      *lark.Client
	wsCli       *larkws.Client
	engine      core.EngineInterface
	provider    plugin.ProviderPlugin
	logger      *slog.Logger
	dedup       *Deduplicator
	started     bool
	typingMgr   *TypingManager
	streamMgr   *StreamingManager
	typingOnce  sync.Once
	streamOnce  sync.Once
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
	eventHandler := dispatcher.NewEventDispatcher(p.config.VerificationToken, p.config.EncryptKey)
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
	if p.typingMgr != nil {
		p.typingMgr.Stop()
	}
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

	// Parse session ID to get receive_id
	// Format: "feishu:ou_xxxx" (p2p) or "feishu:group:oc_xxxx:ou_xxxx" (group)
	receiveID := ""
	receiveIDType := "open_id"

	if len(sessionID) > 7 && sessionID[:7] == "feishu:" {
		parts := splitSessionID(sessionID)
		if len(parts) == 2 {
			// p2p: feishu:ou_xxxx
			receiveID = parts[1]
			receiveIDType = "open_id"
		} else if len(parts) == 4 && parts[1] == "group" {
			// group: feishu:group:oc_xxxx:ou_xxxx
			receiveID = parts[2]
			receiveIDType = "chat_id"
		}
	}

	if receiveID == "" {
		return fmt.Errorf("invalid session ID: %s", sessionID)
	}

	// Build message request
	req := larkim.NewCreateMessageReqBuilder().
		ReceiveIdType(receiveIDType).
		Body(larkim.NewCreateMessageReqBodyBuilder().
			ReceiveId(receiveID).
			MsgType("text").
			Content(fmt.Sprintf(`{"text":"%s"}`, escapeJSON(content))).
			Build()).
		Build()

	// Send message
	resp, err := p.client.Im.Message.Create(context.Background(), req)
	if err != nil {
		p.logger.Error("failed to send message", "error", err)
		return fmt.Errorf("failed to send message: %w", err)
	}

	if !resp.Success() {
		p.logger.Error("failed to send message", "code", resp.Code, "msg", resp.Msg)
		return fmt.Errorf("failed to send message: %s", resp.Msg)
	}

	p.logger.Info("message sent successfully", "session_id", sessionID)
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

// SetProvider sets the LLM provider for the plugin.
func (p *FeishuPlugin) SetProvider(provider plugin.ProviderPlugin) {
	p.provider = provider
}

// handleMessage handles incoming Feishu messages.
func (p *FeishuPlugin) handleMessage(ctx context.Context, event *larkim.P2MessageReceiveV1) error {
	// Extract message info
	msgID := derefString(event.Event.Message.MessageId)
	chatID := derefString(event.Event.Message.ChatId)
	chatType := derefString(event.Event.Message.ChatType)
	senderID := derefString(event.Event.Sender.SenderId.OpenId)

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
		content = derefString(event.Event.Message.Content)
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
		Role:      "user",
		Content:   content,
		Timestamp: time.Now(),
	})

	// Get session history
	history, _ := p.engine.GetSessionManager().GetHistory(session.ID, 50)

	// Check if provider is set
	if p.provider == nil {
		p.logger.Error("no LLM provider configured")
		return fmt.Errorf("no LLM provider configured")
	}

	// Start typing indicator
	if err := p.StartTyping(ctx, sessionID, msgID); err != nil {
		p.logger.Debug("failed to start typing", "error", err)
	}
	defer func() {
		if err := p.StopTyping(ctx, sessionID); err != nil {
			p.logger.Debug("failed to stop typing", "error", err)
		}
	}()

	// Try streaming reply
	if p.provider.SupportsStreaming() {
		return p.handleStreamingMessage(ctx, sessionID, chatID, msgID, session.ID, history)
	}

	// Fallback to non-streaming
	return p.handleNonStreamingMessage(ctx, sessionID, session.ID, history)
}

// handleStreamingMessage handles a message using streaming reply via Feishu Card Kit.
func (p *FeishuPlugin) handleStreamingMessage(ctx context.Context, sessionID, chatID, msgID, dbSessionID string, history []core.Message) error {
	streamSession, err := p.CreateStreamReply(ctx, sessionID, plugin.StreamReplyOptions{
		MessageID: msgID,
		ChatID:    chatID,
	})
	if err != nil {
		p.logger.Warn("failed to create stream reply, falling back to non-streaming", "error", err)
		return p.handleNonStreamingMessage(ctx, sessionID, dbSessionID, history)
	}

	streamCh, err := p.provider.ChatStream(ctx, history, core.ChatConfig{})
	if err != nil {
		p.logger.Error("stream error", "error", err)
		p.FinishStream(ctx, streamSession)
		return err
	}

	fullResponse := ""
	var streamErr error
	for chunk := range streamCh {
		if chunk.Error != nil {
			p.logger.Error("stream chunk error", "error", chunk.Error)
			streamErr = chunk.Error
			break
		}
		if chunk.Done {
			break
		}
		fullResponse += chunk.Content
		if err := p.SendDelta(ctx, streamSession, chunk.Content); err != nil {
			p.logger.Debug("failed to send delta", "error", err)
		}
	}

	if err := p.FinishStream(ctx, streamSession); err != nil {
		p.logger.Warn("failed to finish stream", "error", err)
	}

	// Save assistant message
	p.engine.GetSessionManager().AddMessage(dbSessionID, core.Message{
		Role:      "assistant",
		Content:   fullResponse,
		Timestamp: time.Now(),
	})

	return streamErr
}

// handleNonStreamingMessage handles a message using non-streaming LLM call.
func (p *FeishuPlugin) handleNonStreamingMessage(ctx context.Context, sessionID, dbSessionID string, history []core.Message) error {
	response, err := p.provider.Chat(ctx, history, core.ChatConfig{})
	if err != nil {
		p.logger.Error("LLM error", "error", err)
		return err
	}

	p.engine.GetSessionManager().AddMessage(dbSessionID, core.Message{
		Role:      "assistant",
		Content:   response.Content,
		Timestamp: time.Now(),
	})

	return p.SendMessage(sessionID, response.Content)
}

// StartTyping begins showing a typing indicator for the given message.
func (p *FeishuPlugin) StartTyping(ctx context.Context, sessionID string, messageID string) error {
	p.typingOnce.Do(p.initTypingManager)
	return p.typingMgr.StartTyping(ctx, sessionID, messageID)
}

// StopTyping stops showing the typing indicator.
func (p *FeishuPlugin) StopTyping(ctx context.Context, sessionID string) error {
	if p.typingMgr == nil {
		return nil
	}
	return p.typingMgr.StopTyping(ctx, sessionID)
}

// initTypingManager initializes the typing manager with Feishu API callbacks.
func (p *FeishuPlugin) initTypingManager() {
	p.typingMgr = NewTypingManager(TypingManagerConfig{
		MaxAge:      2 * time.Minute,
		TTLTimeout:  60 * time.Second,
		MaxFailures: 2,
		OnAddReaction: func(ctx context.Context, messageID, emojiType string) (string, error) {
			if p.client == nil {
				return "", fmt.Errorf("feishu client not initialized")
			}
			req := larkim.NewCreateMessageReactionReqBuilder().
				MessageId(messageID).
				Body(larkim.NewCreateMessageReactionReqBodyBuilder().
					ReactionType(larkim.NewEmojiBuilder().
						EmojiType(emojiType).
						Build()).
					Build()).
				Build()

			resp, err := p.client.Im.MessageReaction.Create(ctx, req)
			if err != nil {
				return "", err
			}
			if !resp.Success() {
				return "", fmt.Errorf("feishu API error: %d %s", resp.Code, resp.Msg)
			}
			return derefString(resp.Data.ReactionId), nil
		},
		OnRemoveReaction: func(ctx context.Context, messageID, reactionID string) error {
			if p.client == nil {
				return fmt.Errorf("feishu client not initialized")
			}
			req := larkim.NewDeleteMessageReactionReqBuilder().
				MessageId(messageID).
				ReactionId(reactionID).
				Build()

			resp, err := p.client.Im.MessageReaction.Delete(ctx, req)
			if err != nil {
				return err
			}
			if !resp.Success() {
				return fmt.Errorf("feishu API error: %d %s", resp.Code, resp.Msg)
			}
			return nil
		},
	})
}

// CreateStreamReply creates a Feishu Card Kit streaming reply.
func (p *FeishuPlugin) CreateStreamReply(ctx context.Context, sessionID string, opts plugin.StreamReplyOptions) (*plugin.StreamSession, error) {
	p.streamOnce.Do(p.initStreamingManager)
	return p.streamMgr.CreateStreamReply(ctx, sessionID, opts)
}

// SendDelta sends a streaming delta to the card.
func (p *FeishuPlugin) SendDelta(ctx context.Context, session *plugin.StreamSession, delta string) error {
	if p.streamMgr == nil {
		return fmt.Errorf("streaming manager not initialized")
	}
	return p.streamMgr.SendDelta(ctx, session, delta)
}

// FinishStream completes the streaming reply.
func (p *FeishuPlugin) FinishStream(ctx context.Context, session *plugin.StreamSession) error {
	if p.streamMgr == nil {
		return fmt.Errorf("streaming manager not initialized")
	}
	return p.streamMgr.FinishStream(ctx, session)
}

// initStreamingManager initializes the streaming manager with Feishu Card Kit callbacks.
func (p *FeishuPlugin) initStreamingManager() {
	p.streamMgr = NewStreamingManager(StreamingManagerConfig{
		MinUpdateInterval: 160 * time.Millisecond,
		MinCharsDelta:     18,
		OnCreateCard: func(ctx context.Context, chatID, messageID string) (string, error) {
			if p.client == nil {
				return "", fmt.Errorf("feishu client not initialized")
			}
			cardJSON := buildFeishuCardJSON("⏳ thinking...")
			req := larkim.NewCreateMessageReqBuilder().
				ReceiveIdType("chat_id").
				Body(larkim.NewCreateMessageReqBodyBuilder().
					ReceiveId(chatID).
					MsgType("interactive").
					Content(cardJSON).
					Build()).
				Build()

			resp, err := p.client.Im.Message.Create(ctx, req)
			if err != nil {
				return "", err
			}
			if !resp.Success() {
				return "", fmt.Errorf("feishu API error: %d %s", resp.Code, resp.Msg)
			}
			return derefString(resp.Data.MessageId), nil
		},
		OnUpdateCard: func(ctx context.Context, cardID, content string) error {
			if p.client == nil {
				return fmt.Errorf("feishu client not initialized")
			}
			cardJSON := buildFeishuCardJSON(content)
			req := larkim.NewPatchMessageReqBuilder().
				MessageId(cardID).
				Body(larkim.NewPatchMessageReqBodyBuilder().
					Content(cardJSON).
					Build()).
				Build()

			resp, err := p.client.Im.Message.Patch(ctx, req)
			if err != nil {
				return err
			}
			if !resp.Success() {
				return fmt.Errorf("feishu API error: %d %s", resp.Code, resp.Msg)
			}
			return nil
		},
		OnCloseCard: func(ctx context.Context, cardID, content string) error {
			// Final update — same as OnUpdateCard
			if p.client == nil {
				return fmt.Errorf("feishu client not initialized")
			}
			cardJSON := buildFeishuCardJSON(content)
			req := larkim.NewPatchMessageReqBuilder().
				MessageId(cardID).
				Body(larkim.NewPatchMessageReqBodyBuilder().
					Content(cardJSON).
					Build()).
				Build()

			resp, err := p.client.Im.Message.Patch(ctx, req)
			if err != nil {
				return err
			}
			if !resp.Success() {
				return fmt.Errorf("feishu API error: %d %s", resp.Code, resp.Msg)
			}
			return nil
		},
		OnFallback: func(ctx context.Context, sessionID, content string) error {
			return p.SendMessage(sessionID, content)
		},
	})
}

// buildFeishuCardJSON builds a Feishu interactive card JSON with the given markdown content.
func buildFeishuCardJSON(content string) string {
	return fmt.Sprintf(`{"config":{"wide_screen_mode":true},"header":{"title":{"tag":"plain_text","content":"Golem"},"template":"blue"},"elements":[{"tag":"markdown","content":"%s"}]}`, escapeJSON(content))
}

// derefString safely dereferences a string pointer.
func derefString(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

// splitSessionID splits session ID into parts.
func splitSessionID(sessionID string) []string {
	parts := []string{}
	current := ""
	for _, c := range sessionID {
		if c == ':' {
			parts = append(parts, current)
			current = ""
		} else {
			current += string(c)
		}
	}
	if current != "" {
		parts = append(parts, current)
	}
	return parts
}

// escapeJSON escapes special characters in JSON string.
func escapeJSON(s string) string {
	s = strings.ReplaceAll(s, "\\", "\\\\")
	s = strings.ReplaceAll(s, "\"", "\\\"")
	s = strings.ReplaceAll(s, "\n", "\\n")
	s = strings.ReplaceAll(s, "\r", "\\r")
	s = strings.ReplaceAll(s, "\t", "\\t")
	return s
}

// Ensure FeishuPlugin implements plugin.ChannelPlugin.
var _ plugin.ChannelPlugin = (*FeishuPlugin)(nil)

// Ensure FeishuPlugin implements optional capability interfaces.
var _ plugin.StreamingCapable = (*FeishuPlugin)(nil)
var _ plugin.TypingCapable = (*FeishuPlugin)(nil)
