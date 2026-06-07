package claude

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Shadow-Azure/Golem/internal/core"
)

func TestClaudeProvider_Chat(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		response := MessageResponse{
			ID:   "test-id",
			Type: "message",
			Role: "assistant",
			Content: []Content{
				{Type: "text", Text: "Hello, world!"},
			},
			Model:      "claude-3-opus",
			StopReason: "end_turn",
			Usage:      Usage{InputTokens: 10, OutputTokens: 5},
		}
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	provider := NewProvider(ProviderConfig{
		APIKey:    "test-key",
		BaseURL:   server.URL,
		Model:     "claude-3-opus",
		MaxTokens: 100,
	})

	messages := []core.Message{{Role: "user", Content: "Hello"}}
	response, err := provider.Chat(context.Background(), messages, core.ChatConfig{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if response.Content != "Hello, world!" {
		t.Errorf("expected 'Hello, world!', got '%s'", response.Content)
	}

	if response.Usage.PromptTokens != 10 {
		t.Errorf("expected PromptTokens=10, got %d", response.Usage.PromptTokens)
	}

	if response.Usage.CompletionTokens != 5 {
		t.Errorf("expected CompletionTokens=5, got %d", response.Usage.CompletionTokens)
	}

	if response.Usage.TotalTokens != 15 {
		t.Errorf("expected TotalTokens=15, got %d", response.Usage.TotalTokens)
	}
}

func TestClaudeProvider_Chat_Error(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(ErrorResponse{
			Error: ErrorDetail{Message: "Invalid request"},
		})
	}))
	defer server.Close()

	provider := NewProvider(ProviderConfig{
		APIKey:  "test-key",
		BaseURL: server.URL,
	})

	messages := []core.Message{{Role: "user", Content: "Hello"}}
	_, err := provider.Chat(context.Background(), messages, core.ChatConfig{})
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestClaudeProvider_Chat_EmptyContent(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		response := MessageResponse{
			ID:      "test-id",
			Type:    "message",
			Role:    "assistant",
			Content: []Content{},
			Model:   "claude-3-opus",
			Usage:   Usage{InputTokens: 10, OutputTokens: 0},
		}
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	provider := NewProvider(ProviderConfig{
		APIKey:  "test-key",
		BaseURL: server.URL,
	})

	messages := []core.Message{{Role: "user", Content: "Hello"}}
	_, err := provider.Chat(context.Background(), messages, core.ChatConfig{})
	if err == nil {
		t.Fatal("expected error for empty content")
	}
}

func TestClaudeProvider_Chat_ModelOverride(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req MessageRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Errorf("failed to decode request: %v", err)
			return
		}

		if req.Model != "claude-3-sonnet" {
			t.Errorf("expected model 'claude-3-sonnet', got '%s'", req.Model)
		}

		if req.MaxTokens != 200 {
			t.Errorf("expected max_tokens 200, got %d", req.MaxTokens)
		}

		response := MessageResponse{
			ID:   "test-id",
			Type: "message",
			Role: "assistant",
			Content: []Content{
				{Type: "text", Text: "Response"},
			},
			Model: "claude-3-sonnet",
			Usage: Usage{InputTokens: 5, OutputTokens: 3},
		}
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	provider := NewProvider(ProviderConfig{
		APIKey:    "test-key",
		BaseURL:   server.URL,
		Model:     "claude-3-opus",
		MaxTokens: 100,
	})

	messages := []core.Message{{Role: "user", Content: "Hello"}}
	response, err := provider.Chat(context.Background(), messages, core.ChatConfig{
		Model:     "claude-3-sonnet",
		MaxTokens: 200,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if response.Content != "Response" {
		t.Errorf("expected 'Response', got '%s'", response.Content)
	}
}

func TestClaudeProvider_GetProviderType(t *testing.T) {
	provider := NewProvider(ProviderConfig{})
	if provider.GetProviderType() != "claude" {
		t.Errorf("expected 'claude', got '%s'", provider.GetProviderType())
	}
}

func TestClaudeProvider_SupportsStreaming(t *testing.T) {
	provider := NewProvider(ProviderConfig{})
	if !provider.SupportsStreaming() {
		t.Error("Claude should support streaming")
	}
}

func TestClaudeProvider_Name(t *testing.T) {
	provider := NewProvider(ProviderConfig{})
	if provider.Name() != "claude" {
		t.Errorf("expected 'claude', got '%s'", provider.Name())
	}
}

func TestClaudeProvider_Version(t *testing.T) {
	provider := NewProvider(ProviderConfig{})
	if provider.Version() != "1.0.0" {
		t.Errorf("expected '1.0.0', got '%s'", provider.Version())
	}
}

func TestClaudeProvider_DefaultConfig(t *testing.T) {
	provider := NewProvider(ProviderConfig{})

	if provider.config.BaseURL != "https://api.anthropic.com" {
		t.Errorf("expected default BaseURL 'https://api.anthropic.com', got '%s'", provider.config.BaseURL)
	}

	if provider.config.Model != "claude-3-opus-20240229" {
		t.Errorf("expected default Model 'claude-3-opus-20240229', got '%s'", provider.config.Model)
	}

	if provider.config.MaxTokens != 4096 {
		t.Errorf("expected default MaxTokens 4096, got %d", provider.config.MaxTokens)
	}
}

func TestClaudeProvider_HealthCheck(t *testing.T) {
	provider := NewProvider(ProviderConfig{})
	health := provider.HealthCheck()
	if !health.Healthy {
		t.Error("expected healthy status")
	}
}

func TestClaudeProvider_Initialize(t *testing.T) {
	provider := NewProvider(ProviderConfig{})
	if err := provider.Initialize(nil); err != nil {
		t.Errorf("expected no error, got %v", err)
	}
}

func TestClaudeProvider_StartStop(t *testing.T) {
	provider := NewProvider(ProviderConfig{})
	if err := provider.Start(); err != nil {
		t.Errorf("expected no error on Start, got %v", err)
	}
	if err := provider.Stop(); err != nil {
		t.Errorf("expected no error on Stop, got %v", err)
	}
}

func TestClaudeProvider_ChatStream_NotImplemented(t *testing.T) {
	provider := NewProvider(ProviderConfig{})
	messages := []core.Message{{Role: "user", Content: "Hello"}}
	_, err := provider.ChatStream(context.Background(), messages, core.ChatConfig{})
	if err == nil {
		t.Fatal("expected error for unimplemented streaming")
	}
}

func TestClaudeProvider_RequestHeaders(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("x-api-key") != "test-api-key" {
			t.Errorf("expected x-api-key 'test-api-key', got '%s'", r.Header.Get("x-api-key"))
		}
		if r.Header.Get("anthropic-version") != "2023-06-01" {
			t.Errorf("expected anthropic-version '2023-06-01', got '%s'", r.Header.Get("anthropic-version"))
		}
		if r.Header.Get("Content-Type") != "application/json" {
			t.Errorf("expected Content-Type 'application/json', got '%s'", r.Header.Get("Content-Type"))
		}

		response := MessageResponse{
			ID:      "test-id",
			Type:    "message",
			Role:    "assistant",
			Content: []Content{{Type: "text", Text: "OK"}},
			Model:   "claude-3-opus",
			Usage:   Usage{InputTokens: 1, OutputTokens: 1},
		}
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	provider := NewProvider(ProviderConfig{
		APIKey:  "test-api-key",
		BaseURL: server.URL,
	})

	messages := []core.Message{{Role: "user", Content: "Test"}}
	_, err := provider.Chat(context.Background(), messages, core.ChatConfig{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}
