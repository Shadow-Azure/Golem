package plugin

import (
	"context"

	"github.com/Shadow-Azure/Golem/internal/core"
)

// HealthStatus represents the health status of a plugin.
type HealthStatus struct {
	Healthy bool   `json:"healthy"`
	Message string `json:"message"`
}

// Plugin is the base interface for all plugins.
type Plugin interface {
	// Name returns the unique plugin name.
	Name() string

	// Version returns the plugin version.
	Version() string

	// Initialize initializes the plugin with configuration.
	Initialize(config map[string]interface{}) error

	// Start starts the plugin.
	Start() error

	// Stop stops the plugin gracefully.
	Stop() error

	// HealthCheck returns the plugin health status.
	HealthCheck() HealthStatus
}

// ChannelPlugin extends Plugin for messaging channels.
type ChannelPlugin interface {
	Plugin

	// SendMessage sends a message through the channel.
	SendMessage(sessionID string, content string) error

	// SendStreamingMessage sends a streaming message.
	SendStreamingMessage(sessionID string, stream <-chan core.StreamChunk) error

	// GetChannelType returns the channel type identifier.
	GetChannelType() string
}

// ProviderPlugin extends Plugin for LLM providers.
type ProviderPlugin interface {
	Plugin

	// Chat sends a message and returns a response.
	Chat(ctx context.Context, messages []core.Message, config core.ChatConfig) (*core.ChatResponse, error)

	// ChatStream sends a message and returns a streaming response.
	ChatStream(ctx context.Context, messages []core.Message, config core.ChatConfig) (<-chan core.StreamChunk, error)

	// GetProviderType returns the provider type identifier.
	GetProviderType() string

	// SupportsStreaming returns whether the provider supports streaming.
	SupportsStreaming() bool
}

// ToolPlugin extends Plugin for tool capabilities.
type ToolPlugin interface {
	Plugin

	// Execute executes the tool with given parameters.
	Execute(ctx context.Context, params map[string]interface{}) (interface{}, error)

	// GetToolDefinition returns the tool definition for function calling.
	GetToolDefinition() core.ToolDefinition
}

// StreamingCapable is an optional interface for channels that support
// streaming reply output (e.g., Feishu Card Kit).
type StreamingCapable interface {
	// CreateStreamReply creates a streaming reply session.
	CreateStreamReply(sessionID string, opts StreamReplyOptions) (*StreamSession, error)

	// SendDelta sends a streaming delta update to the reply session.
	SendDelta(session *StreamSession, delta string) error

	// FinishStream completes the streaming reply.
	FinishStream(session *StreamSession) error
}

// TypingCapable is an optional interface for channels that support
// typing indicators (e.g., Feishu emoji reactions).
type TypingCapable interface {
	// StartTyping begins showing a typing indicator for the given message.
	StartTyping(sessionID string, messageID string) error

	// StopTyping stops showing the typing indicator.
	StopTyping(sessionID string) error
}

// StreamReplyOptions contains options for creating a streaming reply.
type StreamReplyOptions struct {
	MessageID string // The message ID to reply to
	ChatID    string // The chat/session ID
}

// StreamSession holds state for an active streaming reply.
type StreamSession struct {
	SessionID string
	CardID    string // Feishu card message ID for subsequent updates
}

// PluginInfo contains metadata about a loaded plugin.
type PluginInfo struct {
	Name    string `json:"name"`
	Version string `json:"version"`
	Type    string `json:"type"`
	Status  string `json:"status"`
}
