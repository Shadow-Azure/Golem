package core

import "time"

// Message represents a single message in a conversation.
type Message struct {
	Role      string                 `json:"role" yaml:"role"`
	Content   string                 `json:"content" yaml:"content"`
	Timestamp time.Time              `json:"timestamp" yaml:"timestamp"`
	Metadata  map[string]interface{} `json:"metadata,omitempty" yaml:"metadata,omitempty"`
}

// Session represents a conversation session between a user and the AI.
type Session struct {
	ID        string                 `json:"id" yaml:"id"`
	UserID    string                 `json:"user_id" yaml:"user_id"`
	Channel   string                 `json:"channel" yaml:"channel"`
	Messages  []Message              `json:"messages" yaml:"messages"`
	Metadata  map[string]interface{} `json:"metadata,omitempty" yaml:"metadata,omitempty"`
	CreatedAt time.Time              `json:"created_at" yaml:"created_at"`
	UpdatedAt time.Time              `json:"updated_at" yaml:"updated_at"`
}

// StreamChunk represents a piece of streaming response content.
type StreamChunk struct {
	Content string `json:"content"`
	Done    bool   `json:"done"`
	Error   error  `json:"error,omitempty"`
}

// ChatConfig contains configuration for a chat request.
type ChatConfig struct {
	Model       string  `json:"model" yaml:"model"`
	Temperature float64 `json:"temperature" yaml:"temperature"`
	MaxTokens   int     `json:"max_tokens" yaml:"max_tokens"`
	Stream      bool    `json:"stream" yaml:"stream"`
}

// ChatResponse represents the response from an LLM provider.
type ChatResponse struct {
	Content string `json:"content"`
	Usage   Usage  `json:"usage"`
}

// Usage contains token usage information.
type Usage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

// ToolDefinition defines a tool that can be used by the LLM.
type ToolDefinition struct {
	Name        string                 `json:"name" yaml:"name"`
	Description string                 `json:"description" yaml:"description"`
	Parameters  map[string]interface{} `json:"parameters" yaml:"parameters"`
}
