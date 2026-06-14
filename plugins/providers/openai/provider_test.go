package openai

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Shadow-Azure/Golem/internal/core"
)

func TestOpenAIProvider_Chat(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		response := ChatCompletionResponse{
			ID:    "test-id",
			Model: "gpt-4",
			Choices: []Choice{
				{Message: Message{Role: "assistant", Content: "Hello, world!"}},
			},
			Usage: Usage{PromptTokens: 10, CompletionTokens: 5, TotalTokens: 15},
		}
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	provider := NewProvider(ProviderConfig{
		APIKey:  "test-key",
		BaseURL: server.URL + "/v1",
		Model:   "gpt-4",
	})

	messages := []core.Message{{Role: "user", Content: "Hello"}}
	response, err := provider.Chat(context.Background(), messages, core.ChatConfig{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if response.Content != "Hello, world!" {
		t.Errorf("expected 'Hello, world!', got '%s'", response.Content)
	}
}

func TestOpenAIProvider_Chat_Error(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(ErrorResponse{
			Error: ErrorDetail{Message: "Invalid request"},
		})
	}))
	defer server.Close()

	provider := NewProvider(ProviderConfig{
		APIKey:  "test-key",
		BaseURL: server.URL + "/v1",
	})

	messages := []core.Message{{Role: "user", Content: "Hello"}}
	_, err := provider.Chat(context.Background(), messages, core.ChatConfig{})
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestOpenAIProvider_GetProviderType(t *testing.T) {
	provider := NewProvider(ProviderConfig{})
	if provider.GetProviderType() != "openai" {
		t.Errorf("expected 'openai', got '%s'", provider.GetProviderType())
	}
}

func TestOpenAIProvider_SupportsStreaming(t *testing.T) {
	provider := NewProvider(ProviderConfig{})
	if !provider.SupportsStreaming() {
		t.Error("OpenAI should support streaming")
	}
}

func TestStripThinkingTags(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "no thinking tags",
			input:    "Hello, world!",
			expected: "Hello, world!",
		},
		{
			name:     "single thinking block",
			input:    "<think>\nreasoning\n</think>\n\nActual response",
			expected: "Actual response",
		},
		{
			name:     "multiple thinking blocks",
			input:    "<think>\nfirst\n</think>\n\nMiddle\n<think>\nsecond\n</think>\n\nEnd",
			expected: "Middle\n\nEnd",
		},
		{
			name:     "empty thinking block",
			input:    "<think></think>\n\nResponse",
			expected: "Response",
		},
		{
			name:     "thinking block with no content after",
			input:    "<think>\nonly thinking\n</think>",
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := StripThinkingTags(tt.input)
			if result != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, result)
			}
		})
	}
}

func TestNewProvider_DefaultValues(t *testing.T) {
	tests := []struct {
		name           string
		config         ProviderConfig
		expectedBase   string
		expectedModel  string
	}{
		{
			name:           "empty config uses defaults",
			config:         ProviderConfig{APIKey: "test"},
			expectedBase:   "https://api.openai.com/v1",
			expectedModel:  "gpt-4o",
		},
		{
			name:           "custom base URL",
			config:         ProviderConfig{APIKey: "test", BaseURL: "https://custom.api.com/v1"},
			expectedBase:   "https://custom.api.com/v1",
			expectedModel:  "gpt-4o",
		},
		{
			name:           "custom model",
			config:         ProviderConfig{APIKey: "test", Model: "gpt-3.5-turbo"},
			expectedBase:   "https://api.openai.com/v1",
			expectedModel:  "gpt-3.5-turbo",
		},
		{
			name:           "all custom values",
			config:         ProviderConfig{APIKey: "test", BaseURL: "https://custom.api.com/v1", Model: "gpt-4"},
			expectedBase:   "https://custom.api.com/v1",
			expectedModel:  "gpt-4",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			provider := NewProvider(tt.config)

			if provider.config.BaseURL != tt.expectedBase {
				t.Errorf("BaseURL = %q, want %q", provider.config.BaseURL, tt.expectedBase)
			}
			if provider.config.Model != tt.expectedModel {
				t.Errorf("Model = %q, want %q", provider.config.Model, tt.expectedModel)
			}
		})
	}
}

func TestOpenAIProvider_HealthCheck(t *testing.T) {
	provider := NewProvider(ProviderConfig{APIKey: "test"})
	status := provider.HealthCheck()

	if !status.Healthy {
		t.Error("expected healthy status")
	}
}

func TestOpenAIProvider_Initialize(t *testing.T) {
	provider := NewProvider(ProviderConfig{APIKey: "test"})
	err := provider.Initialize(nil)

	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
}

func TestOpenAIProvider_StartStop(t *testing.T) {
	provider := NewProvider(ProviderConfig{APIKey: "test"})

	// Test Start
	if err := provider.Start(); err != nil {
		t.Errorf("Start() error: %v", err)
	}

	// Test Stop
	if err := provider.Stop(); err != nil {
		t.Errorf("Stop() error: %v", err)
	}
}

func TestOpenAIProvider_ChatStream(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Set SSE headers
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)

		// Send streaming response
		chunks := []string{
			"data: {\"id\":\"1\",\"choices\":[{\"delta\":{\"content\":\"Hello\"}}]}\n\n",
			"data: {\"id\":\"1\",\"choices\":[{\"delta\":{\"content\":\" World\"}}]}\n\n",
			"data: [DONE]\n\n",
		}
		for _, chunk := range chunks {
			w.Write([]byte(chunk))
			if f, ok := w.(http.Flusher); ok {
				f.Flush()
			}
		}
	}))
	defer server.Close()

	provider := NewProvider(ProviderConfig{
		APIKey:  "test-key",
		BaseURL: server.URL + "/v1",
		Model:   "gpt-4",
	})

	messages := []core.Message{{Role: "user", Content: "Hello"}}
	stream, err := provider.ChatStream(context.Background(), messages, core.ChatConfig{Stream: true})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var contents []string
	for chunk := range stream {
		if chunk.Error != nil {
			t.Fatalf("unexpected error in stream: %v", chunk.Error)
		}
		if chunk.Done {
			break
		}
		contents = append(contents, chunk.Content)
	}

	expected := []string{"Hello", " World"}
	if len(contents) != len(expected) {
		t.Fatalf("expected %d chunks, got %d", len(expected), len(contents))
	}
	for i, c := range contents {
		if c != expected[i] {
			t.Errorf("chunk[%d] = %q, want %q", i, c, expected[i])
		}
	}
}

func TestOpenAIProvider_ChatStream_Error(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(ErrorResponse{
			Error: ErrorDetail{Message: "Invalid request"},
		})
	}))
	defer server.Close()

	provider := NewProvider(ProviderConfig{
		APIKey:  "test-key",
		BaseURL: server.URL + "/v1",
	})

	messages := []core.Message{{Role: "user", Content: "Hello"}}
	stream, err := provider.ChatStream(context.Background(), messages, core.ChatConfig{Stream: true})
	if err != nil {
		t.Fatalf("unexpected error from ChatStream: %v", err)
	}

	// Error should come through the channel
	var streamErr error
	for chunk := range stream {
		if chunk.Error != nil {
			streamErr = chunk.Error
			break
		}
	}

	if streamErr == nil {
		t.Fatal("expected error in stream")
	}
}
