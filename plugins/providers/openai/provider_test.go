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
