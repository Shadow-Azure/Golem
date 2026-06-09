// web/types_test.go
package web

import (
	"encoding/json"
	"testing"
)

func TestChatRequest_JSON(t *testing.T) {
	req := ChatRequest{Message: "hello"}
	data, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("failed to marshal: %v", err)
	}

	var decoded ChatRequest
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	if decoded.Message != "hello" {
		t.Errorf("expected 'hello', got '%s'", decoded.Message)
	}
}

func TestChatResponse_JSON(t *testing.T) {
	resp := ChatResponse{Content: "hi there", Error: ""}
	data, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("failed to marshal: %v", err)
	}

	var decoded ChatResponse
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	if decoded.Content != "hi there" {
		t.Errorf("expected 'hi there', got '%s'", decoded.Content)
	}
	if decoded.Error != "" {
		t.Errorf("expected empty error, got '%s'", decoded.Error)
	}
}

func TestHistoryResponse_JSON(t *testing.T) {
	resp := HistoryResponse{
		Messages: []MessageItem{
			{Role: "user", Content: "hello"},
			{Role: "assistant", Content: "hi"},
		},
	}
	data, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("failed to marshal: %v", err)
	}

	var decoded HistoryResponse
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	if len(decoded.Messages) != 2 {
		t.Errorf("expected 2 messages, got %d", len(decoded.Messages))
	}
}
