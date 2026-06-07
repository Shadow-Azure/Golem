package claude

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"

	"github.com/Shadow-Azure/Golem/internal/core"
	"github.com/Shadow-Azure/Golem/internal/plugin"
)

// ProviderConfig holds configuration for the Claude provider.
type ProviderConfig struct {
	APIKey    string
	BaseURL   string
	Model     string
	MaxTokens int
}

// Provider implements the LLM provider interface for Claude.
type Provider struct {
	config     ProviderConfig
	httpClient *http.Client
	logger     *slog.Logger
}

// NewProvider creates a new Claude provider with the given configuration.
func NewProvider(config ProviderConfig) *Provider {
	if config.BaseURL == "" {
		config.BaseURL = "https://api.anthropic.com"
	}
	if config.Model == "" {
		config.Model = "claude-3-opus-20240229"
	}
	if config.MaxTokens == 0 {
		config.MaxTokens = 4096
	}
	return &Provider{
		config:     config,
		httpClient: &http.Client{},
		logger:     slog.Default().With("component", "claude_provider"),
	}
}

func (p *Provider) Name() string                            { return "claude" }
func (p *Provider) Version() string                         { return "1.0.0" }
func (p *Provider) Initialize(config map[string]interface{}) error { return nil }
func (p *Provider) Start() error                            { return nil }
func (p *Provider) Stop() error                             { return nil }
func (p *Provider) HealthCheck() plugin.HealthStatus         { return plugin.HealthStatus{Healthy: true} }
func (p *Provider) GetProviderType() string                  { return "claude" }
func (p *Provider) SupportsStreaming() bool                  { return true }

// Chat sends messages to the Claude API and returns a completion response.
func (p *Provider) Chat(ctx context.Context, messages []core.Message, config core.ChatConfig) (*core.ChatResponse, error) {
	claudeMessages := make([]Message, len(messages))
	for i, msg := range messages {
		claudeMessages[i] = Message{Role: msg.Role, Content: msg.Content}
	}

	model := p.config.Model
	if config.Model != "" {
		model = config.Model
	}

	maxTokens := p.config.MaxTokens
	if config.MaxTokens > 0 {
		maxTokens = config.MaxTokens
	}

	request := MessageRequest{
		Model:     model,
		Messages:  claudeMessages,
		MaxTokens: maxTokens,
	}

	response, err := p.sendRequest(ctx, "/v1/messages", request)
	if err != nil {
		return nil, err
	}

	var messageResp MessageResponse
	if err := json.Unmarshal(response, &messageResp); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	if len(messageResp.Content) == 0 {
		return nil, fmt.Errorf("no content in response")
	}

	return &core.ChatResponse{
		Content: messageResp.Content[0].Text,
		Usage: core.Usage{
			PromptTokens:     messageResp.Usage.InputTokens,
			CompletionTokens: messageResp.Usage.OutputTokens,
			TotalTokens:      messageResp.Usage.InputTokens + messageResp.Usage.OutputTokens,
		},
	}, nil
}

// ChatStream sends messages to the Claude API and returns a streaming response.
func (p *Provider) ChatStream(ctx context.Context, messages []core.Message, config core.ChatConfig) (<-chan core.StreamChunk, error) {
	return nil, fmt.Errorf("streaming not yet implemented for Claude")
}

func (p *Provider) sendRequest(ctx context.Context, path string, request interface{}) ([]byte, error) {
	body, err := json.Marshal(request)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", p.config.BaseURL+path, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", p.config.APIKey)
	req.Header.Set("anthropic-version", "2023-06-01")

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		var errResp ErrorResponse
		if err := json.Unmarshal(respBody, &errResp); err == nil {
			return nil, fmt.Errorf("API error: %s", errResp.Error.Message)
		}
		return nil, fmt.Errorf("API error: status %d", resp.StatusCode)
	}

	return respBody, nil
}

// Compile-time check that Provider implements ProviderPlugin.
var _ plugin.ProviderPlugin = (*Provider)(nil)
