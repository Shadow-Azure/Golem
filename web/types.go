// web/types.go
package web

// ChatRequest represents a chat request from the client
type ChatRequest struct {
	Message string `json:"message"`
}

// ChatResponse represents a chat response to the client
type ChatResponse struct {
	Content string `json:"content"`
	Error   string `json:"error,omitempty"`
}

// HistoryResponse represents the session history
type HistoryResponse struct {
	Messages []MessageItem `json:"messages"`
}

// MessageItem represents a single message in history
type MessageItem struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// ErrorResponse represents an error response
type ErrorResponse struct {
	Error string `json:"error"`
}
