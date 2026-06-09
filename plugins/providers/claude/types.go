package claude

// MessageRequest represents a request to the Claude Messages API.
type MessageRequest struct {
	Model     string    `json:"model"`
	Messages  []Message `json:"messages"`
	MaxTokens int       `json:"max_tokens"`
	Stream    bool      `json:"stream,omitempty"`
}

// Message represents a single message in the Claude request/response format.
type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// MessageResponse represents a response from the Claude Messages API.
type MessageResponse struct {
	ID         string    `json:"id"`
	Type       string    `json:"type"`
	Role       string    `json:"role"`
	Content    []Content `json:"content"`
	Model      string    `json:"model"`
	StopReason string    `json:"stop_reason"`
	Usage      Usage     `json:"usage"`
}

// Content represents a content block in the Claude response.
type Content struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

// Usage contains token usage information from Claude.
type Usage struct {
	InputTokens  int `json:"input_tokens"`
	OutputTokens int `json:"output_tokens"`
}

// ErrorResponse represents an error response from the Claude API.
type ErrorResponse struct {
	Type  string      `json:"type"`
	Error ErrorDetail `json:"error"`
}

// ErrorDetail contains the details of a Claude API error.
type ErrorDetail struct {
	Type    string `json:"type"`
	Message string `json:"message"`
}
