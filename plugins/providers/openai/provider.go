package openai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"regexp"
	"strings"

	"github.com/Shadow-Azure/Golem/internal/core"
	"github.com/Shadow-Azure/Golem/internal/plugin"
)

// ProviderConfig holds configuration for the OpenAI provider.
type ProviderConfig struct {
	APIKey      string
	BaseURL     string
	Model       string
	Temperature float64
	MaxTokens   int
}

// Provider implements the LLM provider interface for OpenAI.
type Provider struct {
	config     ProviderConfig
	httpClient *http.Client
	logger     *slog.Logger
}

// NewProvider creates a new OpenAI provider with the given configuration.
func NewProvider(config ProviderConfig) *Provider {
	if config.BaseURL == "" {
		config.BaseURL = "https://api.openai.com/v1"
	}
	if config.Model == "" {
		config.Model = "gpt-4o"
	}
	return &Provider{
		config:     config,
		httpClient: &http.Client{},
		logger:     slog.Default().With("component", "openai_provider"),
	}
}

func (p *Provider) Name() string                                   { return "openai" }
func (p *Provider) Version() string                                { return "1.0.0" }
func (p *Provider) Initialize(config map[string]interface{}) error { return nil }
func (p *Provider) Start() error                                   { return nil }
func (p *Provider) Stop() error                                    { return nil }
func (p *Provider) HealthCheck() plugin.HealthStatus               { return plugin.HealthStatus{Healthy: true} }
func (p *Provider) GetProviderType() string                        { return "openai" }
func (p *Provider) SupportsStreaming() bool                        { return true }

// Chat sends messages to the OpenAI API and returns a completion response.
func (p *Provider) Chat(ctx context.Context, messages []core.Message, config core.ChatConfig) (*core.ChatResponse, error) {
	openaiMessages := make([]Message, len(messages))
	for i, msg := range messages {
		openaiMessages[i] = Message{Role: msg.Role, Content: msg.Content}
	}

	model := p.config.Model
	if config.Model != "" {
		model = config.Model
	}

	request := ChatCompletionRequest{
		Model:       model,
		Messages:    openaiMessages,
		Temperature: p.config.Temperature,
		MaxTokens:   p.config.MaxTokens,
	}

	response, err := p.sendRequest(ctx, "/chat/completions", request)
	if err != nil {
		return nil, err
	}

	var completion ChatCompletionResponse
	if err := json.Unmarshal(response, &completion); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	if len(completion.Choices) == 0 {
		return nil, fmt.Errorf("no choices in response")
	}

	return &core.ChatResponse{
		Content: StripThinkingTags(completion.Choices[0].Message.Content),
		Usage: core.Usage{
			PromptTokens:     completion.Usage.PromptTokens,
			CompletionTokens: completion.Usage.CompletionTokens,
			TotalTokens:      completion.Usage.TotalTokens,
		},
	}, nil
}

// ChatStream sends messages to the OpenAI API and returns a streaming response.
func (p *Provider) ChatStream(ctx context.Context, messages []core.Message, config core.ChatConfig) (<-chan core.StreamChunk, error) {
	openaiMessages := make([]Message, len(messages))
	for i, msg := range messages {
		openaiMessages[i] = Message{Role: msg.Role, Content: msg.Content}
	}

	request := ChatCompletionRequest{
		Model:       p.config.Model,
		Messages:    openaiMessages,
		Temperature: p.config.Temperature,
		MaxTokens:   p.config.MaxTokens,
		Stream:      true,
	}

	chunks := make(chan core.StreamChunk, 100)

	go func() {
		defer close(chunks)

		resp, err := p.sendStreamingRequest(ctx, "/chat/completions", request)
		if err != nil {
			chunks <- core.StreamChunk{Error: err}
			return
		}
		defer resp.Body.Close()

		parser := NewStreamParser(resp.Body)
		filter := NewThinkingFilter()
		for chunk := range parser.Parse() {
			if chunk.Done || chunk.Error != nil {
				select {
				case <-ctx.Done():
					chunks <- core.StreamChunk{Error: ctx.Err()}
					return
				case chunks <- chunk:
				}
				continue
			}

			// Filter out thinking tags from stream content
			filtered := filter.Filter(chunk.Content)
			if filtered == "" {
				continue
			}

			select {
			case <-ctx.Done():
				chunks <- core.StreamChunk{Error: ctx.Err()}
				return
			case chunks <- core.StreamChunk{Content: filtered}:
			}
		}
	}()

	return chunks, nil
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
	req.Header.Set("Authorization", "Bearer "+p.config.APIKey)

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

func (p *Provider) sendStreamingRequest(ctx context.Context, path string, request interface{}) (*http.Response, error) {
	body, err := json.Marshal(request)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", p.config.BaseURL+path, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+p.config.APIKey)

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		defer resp.Body.Close()
		return nil, fmt.Errorf("API error: status %d", resp.StatusCode)
	}

	return resp, nil
}

// thinkingTagRegex matches <think>...</think> blocks (including multiline).
var thinkingTagRegex = regexp.MustCompile(`(?s)<think>.*?</think>`)

// StripThinkingTags removes <think>...</think> blocks from LLM output.
// Some models (e.g., MiniMax, DeepSeek) return thinking blocks in their response,
// which should not be shown to end users.
func StripThinkingTags(content string) string {
	// Remove thinking blocks
	result := thinkingTagRegex.ReplaceAllString(content, "")
	// Clean up extra whitespace left behind
	result = strings.TrimSpace(result)
	// Collapse multiple newlines into at most two
	for strings.Contains(result, "\n\n\n") {
		result = strings.ReplaceAll(result, "\n\n\n", "\n\n")
	}
	return result
}

// Compile-time check that Provider implements ProviderPlugin.
var _ plugin.ProviderPlugin = (*Provider)(nil)
