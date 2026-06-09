package config

import "time"

// Config is the top-level configuration structure.
type Config struct {
	Server  ServerConfig  `yaml:"server"`
	LLM     LLMConfig     `yaml:"llm"`
	Session SessionConfig `yaml:"session"`
	Plugins PluginsConfig `yaml:"plugins"`
	Logging LoggingConfig `yaml:"logging"`
}

// ServerConfig contains HTTP server configuration.
type ServerConfig struct {
	Host string `yaml:"host" json:"host"`
	Port int    `yaml:"port" json:"port"`
}

// LLMConfig contains LLM provider configuration.
type LLMConfig struct {
	DefaultProvider string                    `yaml:"default_provider" json:"default_provider"`
	Providers       map[string]ProviderConfig `yaml:"providers" json:"providers"`
}

// ProviderConfig contains configuration for an LLM provider.
type ProviderConfig struct {
	APIKey      string  `yaml:"api_key" json:"api_key"`
	BaseURL     string  `yaml:"base_url" json:"base_url"`
	Model       string  `yaml:"model" json:"model"`
	Temperature float64 `yaml:"temperature" json:"temperature"`
	MaxTokens   int     `yaml:"max_tokens" json:"max_tokens"`
}

// SessionConfig contains session management configuration.
type SessionConfig struct {
	MaxHistory      int           `yaml:"max_history" json:"max_history"`
	TrimTo          int           `yaml:"trim_to" json:"trim_to"`
	IdleTimeout     time.Duration `yaml:"idle_timeout" json:"idle_timeout"`
	CleanupInterval time.Duration `yaml:"cleanup_interval" json:"cleanup_interval"`
}

// PluginsConfig contains plugin configuration.
type PluginsConfig struct {
	Channels  map[string]map[string]interface{} `yaml:"channels" json:"channels"`
	Providers map[string]map[string]interface{} `yaml:"providers" json:"providers"`
}

// FeishuChannelConfig contains Feishu-specific channel configuration.
type FeishuChannelConfig struct {
	TypingIndicator  bool `yaml:"typing_indicator" json:"typing_indicator"`
	Streaming        bool `yaml:"streaming" json:"streaming"`
	StreamThrottleMs int  `yaml:"stream_throttle_ms" json:"stream_throttle_ms"`
}

// LoggingConfig contains logging configuration.
type LoggingConfig struct {
	Level  string `yaml:"level" json:"level"`
	Format string `yaml:"format" json:"format"`
}
