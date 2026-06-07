// web/server_test.go
package web

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Shadow-Azure/Golem/internal/config"
	"github.com/Shadow-Azure/Golem/internal/core"
	"github.com/Shadow-Azure/Golem/internal/plugin"
)

// mockProvider implements plugin.ProviderPlugin for testing.
type mockProvider struct {
	response   string
	callCount  int
	failOnChat bool
}

var _ plugin.ProviderPlugin = (*mockProvider)(nil)

func (m *mockProvider) Name() string              { return "mock-provider" }
func (m *mockProvider) Version() string            { return "0.1.0" }
func (m *mockProvider) Initialize(_ map[string]interface{}) error { return nil }
func (m *mockProvider) Start() error               { return nil }
func (m *mockProvider) Stop() error                { return nil }
func (m *mockProvider) HealthCheck() plugin.HealthStatus {
	return plugin.HealthStatus{Healthy: true, Message: "ok"}
}
func (m *mockProvider) GetProviderType() string    { return "mock" }
func (m *mockProvider) SupportsStreaming() bool    { return false }

func (m *mockProvider) Chat(_ context.Context, _ []core.Message, _ core.ChatConfig) (*core.ChatResponse, error) {
	m.callCount++
	if m.failOnChat {
		return nil, context.DeadlineExceeded
	}
	return &core.ChatResponse{
		Content: m.response,
		Usage:   core.Usage{},
	}, nil
}

func (m *mockProvider) ChatStream(_ context.Context, _ []core.Message, _ core.ChatConfig) (<-chan core.StreamChunk, error) {
	ch := make(chan core.StreamChunk, 1)
	ch <- core.StreamChunk{Content: m.response, Done: true}
	close(ch)
	return ch, nil
}

// newTestServer creates a Server wired to a real Engine and a mock provider.
func newTestServer(provider *mockProvider) *Server {
	cfg := config.DefaultConfig()
	engine, _ := core.NewEngine(cfg)
	return NewServer(engine, provider, ":0")
}

// newTestRequest creates an HTTP request for testing.
func newTestRequest(method, path string, body interface{}) *http.Request {
	var buf bytes.Buffer
	if body != nil {
		json.NewEncoder(&buf).Encode(body)
	}
	req := httptest.NewRequest(method, path, &buf)
	req.Header.Set("Content-Type", "application/json")
	return req
}

// --- Tests ---

func TestHandleChat_Success(t *testing.T) {
	provider := &mockProvider{response: "Hello from AI"}
	srv := newTestServer(provider)

	req := newTestRequest(http.MethodPost, "/api/chat", ChatRequest{Message: "Hi there"})
	w := httptest.NewRecorder()

	srv.handleChat(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var resp ChatResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if resp.Content != "Hello from AI" {
		t.Errorf("expected 'Hello from AI', got '%s'", resp.Content)
	}

	if provider.callCount != 1 {
		t.Errorf("expected 1 LLM call, got %d", provider.callCount)
	}
}

func TestHandleChat_MethodNotAllowed(t *testing.T) {
	srv := newTestServer(&mockProvider{})

	req := newTestRequest(http.MethodGet, "/api/chat", nil)
	w := httptest.NewRecorder()

	srv.handleChat(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected 405, got %d", w.Code)
	}
}

func TestHandleChat_EmptyMessage(t *testing.T) {
	srv := newTestServer(&mockProvider{})

	req := newTestRequest(http.MethodPost, "/api/chat", ChatRequest{Message: ""})
	w := httptest.NewRecorder()

	srv.handleChat(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}

	var resp ErrorResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if resp.Error != "Message cannot be empty" {
		t.Errorf("expected 'Message cannot be empty', got '%s'", resp.Error)
	}
}

func TestHandleChat_InvalidBody(t *testing.T) {
	srv := newTestServer(&mockProvider{})

	req := httptest.NewRequest(http.MethodPost, "/api/chat", bytes.NewBufferString("not json"))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	srv.handleChat(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestHandleChat_LLMFailure(t *testing.T) {
	provider := &mockProvider{failOnChat: true}
	srv := newTestServer(provider)

	req := newTestRequest(http.MethodPost, "/api/chat", ChatRequest{Message: "Hello"})
	w := httptest.NewRecorder()

	srv.handleChat(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", w.Code)
	}
}

func TestHandleChat_MultipleMessages(t *testing.T) {
	provider := &mockProvider{response: "reply"}
	srv := newTestServer(provider)

	// First message
	req1 := newTestRequest(http.MethodPost, "/api/chat", ChatRequest{Message: "msg1"})
	w1 := httptest.NewRecorder()
	srv.handleChat(w1, req1)
	if w1.Code != http.StatusOK {
		t.Fatalf("first message: expected 200, got %d", w1.Code)
	}

	// Second message
	req2 := newTestRequest(http.MethodPost, "/api/chat", ChatRequest{Message: "msg2"})
	w2 := httptest.NewRecorder()
	srv.handleChat(w2, req2)
	if w2.Code != http.StatusOK {
		t.Fatalf("second message: expected 200, got %d", w2.Code)
	}

	// Verify history has 4 messages (2 user + 2 assistant)
	reqHist := newTestRequest(http.MethodGet, "/api/history", nil)
	wHist := httptest.NewRecorder()
	srv.handleHistory(wHist, reqHist)

	var histResp HistoryResponse
	if err := json.NewDecoder(wHist.Body).Decode(&histResp); err != nil {
		t.Fatalf("failed to decode history: %v", err)
	}

	if len(histResp.Messages) != 4 {
		t.Errorf("expected 4 messages in history, got %d", len(histResp.Messages))
	}
}

func TestHandleHistory_Empty(t *testing.T) {
	srv := newTestServer(&mockProvider{})

	req := newTestRequest(http.MethodGet, "/api/history", nil)
	w := httptest.NewRecorder()

	srv.handleHistory(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var resp HistoryResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if len(resp.Messages) != 0 {
		t.Errorf("expected 0 messages, got %d", len(resp.Messages))
	}
}

func TestHandleHistory_MethodNotAllowed(t *testing.T) {
	srv := newTestServer(&mockProvider{})

	req := newTestRequest(http.MethodPost, "/api/history", nil)
	w := httptest.NewRecorder()

	srv.handleHistory(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected 405, got %d", w.Code)
	}
}

func TestHandleHistory_WithMessages(t *testing.T) {
	provider := &mockProvider{response: "ai reply"}
	srv := newTestServer(provider)

	// Send a chat to populate history
	req := newTestRequest(http.MethodPost, "/api/chat", ChatRequest{Message: "hello"})
	w := httptest.NewRecorder()
	srv.handleChat(w, req)

	// Get history
	histReq := newTestRequest(http.MethodGet, "/api/history", nil)
	histW := httptest.NewRecorder()
	srv.handleHistory(histW, histReq)

	var resp HistoryResponse
	if err := json.NewDecoder(histW.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode: %v", err)
	}

	if len(resp.Messages) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(resp.Messages))
	}

	if resp.Messages[0].Role != "user" || resp.Messages[0].Content != "hello" {
		t.Errorf("first message mismatch: role=%s content=%s", resp.Messages[0].Role, resp.Messages[0].Content)
	}

	if resp.Messages[1].Role != "assistant" || resp.Messages[1].Content != "ai reply" {
		t.Errorf("second message mismatch: role=%s content=%s", resp.Messages[1].Role, resp.Messages[1].Content)
	}
}

func TestHandleClear(t *testing.T) {
	provider := &mockProvider{response: "reply"}
	srv := newTestServer(provider)

	// Populate a session
	req := newTestRequest(http.MethodPost, "/api/chat", ChatRequest{Message: "hello"})
	w := httptest.NewRecorder()
	srv.handleChat(w, req)

	// Clear
	clearReq := newTestRequest(http.MethodPost, "/api/clear", nil)
	clearW := httptest.NewRecorder()
	srv.handleClear(clearW, clearReq)

	if clearW.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", clearW.Code)
	}

	var resp map[string]string
	if err := json.NewDecoder(clearW.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode: %v", err)
	}

	if resp["status"] != "ok" {
		t.Errorf("expected status 'ok', got '%s'", resp["status"])
	}
}

func TestHandleClear_MethodNotAllowed(t *testing.T) {
	srv := newTestServer(&mockProvider{})

	req := newTestRequest(http.MethodGet, "/api/clear", nil)
	w := httptest.NewRecorder()

	srv.handleClear(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected 405, got %d", w.Code)
	}
}

func TestHandleClear_ResetsSession(t *testing.T) {
	provider := &mockProvider{response: "reply"}
	srv := newTestServer(provider)

	// First chat - creates session
	req1 := newTestRequest(http.MethodPost, "/api/chat", ChatRequest{Message: "hello"})
	w1 := httptest.NewRecorder()
	srv.handleChat(w1, req1)

	// Clear session
	clearReq := newTestRequest(http.MethodPost, "/api/clear", nil)
	clearW := httptest.NewRecorder()
	srv.handleClear(clearW, clearReq)

	// Chat again - creates a new session
	req2 := newTestRequest(http.MethodPost, "/api/chat", ChatRequest{Message: "world"})
	w2 := httptest.NewRecorder()
	srv.handleChat(w2, req2)

	// History should only have 2 messages (from second session)
	histReq := newTestRequest(http.MethodGet, "/api/history", nil)
	histW := httptest.NewRecorder()
	srv.handleHistory(histW, histReq)

	var resp HistoryResponse
	if err := json.NewDecoder(histW.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode: %v", err)
	}

	if len(resp.Messages) != 2 {
		t.Errorf("expected 2 messages after clear+re-chat, got %d", len(resp.Messages))
	}
}

func TestGetAddr(t *testing.T) {
	srv := NewServer(nil, nil, ":8080")
	if srv.GetAddr() != ":8080" {
		t.Errorf("expected ':8080', got '%s'", srv.GetAddr())
	}
}
