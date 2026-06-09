// web/api.go
package web

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/Shadow-Azure/Golem/internal/core"
)

// handleChat handles POST /api/chat.
func (s *Server) handleChat(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req ChatRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.writeError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	if req.Message == "" {
		s.writeError(w, http.StatusBadRequest, "Message cannot be empty")
		return
	}

	// Get or create session
	session, err := s.getOrCreateSession()
	if err != nil {
		s.writeError(w, http.StatusInternalServerError, "Failed to get session")
		return
	}

	// Add user message
	if err := s.engine.GetSessionManager().AddMessage(session.ID, core.Message{
		Role:    "user",
		Content: req.Message,
	}); err != nil {
		s.writeError(w, http.StatusInternalServerError, "Failed to add user message")
		return
	}

	// Get history
	history, _ := s.engine.GetSessionManager().GetHistory(session.ID, 50)

	// Call LLM
	response, err := s.provider.Chat(r.Context(), history, core.ChatConfig{})
	if err != nil {
		s.logger.Error("LLM error", "error", err)
		s.writeError(w, http.StatusInternalServerError, "Failed to get response")
		return
	}

	// Add assistant message
	if err := s.engine.GetSessionManager().AddMessage(session.ID, core.Message{
		Role:    "assistant",
		Content: response.Content,
	}); err != nil {
		s.writeError(w, http.StatusInternalServerError, "Failed to add assistant message")
		return
	}

	// Return response
	s.writeJSON(w, ChatResponse{Content: response.Content})
}

// handleHistory handles GET /api/history.
func (s *Server) handleHistory(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	session, err := s.getOrCreateSession()
	if err != nil {
		s.writeError(w, http.StatusInternalServerError, "Failed to get session")
		return
	}

	history, _ := s.engine.GetSessionManager().GetHistory(session.ID, 100)

	messages := make([]MessageItem, len(history))
	for i, msg := range history {
		messages[i] = MessageItem{
			Role:    msg.Role,
			Content: msg.Content,
		}
	}

	s.writeJSON(w, HistoryResponse{Messages: messages})
}

// handleClear handles POST /api/clear.
func (s *Server) handleClear(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	s.mu.Lock()
	s.session = nil
	s.mu.Unlock()

	s.writeJSON(w, map[string]string{"status": "ok"})
}

// getOrCreateSession gets or creates a session.
func (s *Server) getOrCreateSession() (*core.Session, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.session != nil {
		return s.session, nil
	}

	session, err := s.engine.GetSessionManager().CreateSession("webchat", "web")
	if err != nil {
		return nil, err
	}

	s.session = session
	return session, nil
}

// writeJSON writes a JSON response.
func (s *Server) writeJSON(w http.ResponseWriter, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(data)
}

// writeError writes an error response.
func (s *Server) writeError(w http.ResponseWriter, code int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(ErrorResponse{Error: message})
}

// handleStream handles GET /api/stream for SSE streaming.
func (s *Server) handleStream(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Check if response supports flushing
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "Streaming not supported", http.StatusInternalServerError)
		return
	}

	// Set SSE headers
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	// Get message from query params
	message := r.URL.Query().Get("message")
	if message == "" {
		s.writeSSEError(w, flusher, "Message required")
		return
	}

	// Get or create session
	session, err := s.getOrCreateSession()
	if err != nil {
		s.writeSSEError(w, flusher, "Failed to get session")
		return
	}

	// Add user message
	if err := s.engine.GetSessionManager().AddMessage(session.ID, core.Message{
		Role:    "user",
		Content: message,
	}); err != nil {
		s.writeSSEError(w, flusher, "Failed to add message")
		return
	}

	// Get history
	history, _ := s.engine.GetSessionManager().GetHistory(session.ID, 50)

	// Check if provider supports streaming
	if !s.provider.SupportsStreaming() {
		// Fall back to non-streaming
		response, err := s.provider.Chat(r.Context(), history, core.ChatConfig{})
		if err != nil {
			s.writeSSEError(w, flusher, "Failed to get response")
			return
		}
		s.writeSSEContent(w, flusher, response.Content)
		s.writeSSEDone(w, flusher)

		s.engine.GetSessionManager().AddMessage(session.ID, core.Message{
			Role:    "assistant",
			Content: response.Content,
		})
		return
	}

	// Call LLM with streaming
	stream, err := s.provider.ChatStream(r.Context(), history, core.ChatConfig{
		Stream: true,
	})
	if err != nil {
		s.writeSSEError(w, flusher, "Failed to get response")
		return
	}

	// Stream response
	var fullResponse string
	for chunk := range stream {
		if chunk.Error != nil {
			s.writeSSEError(w, flusher, chunk.Error.Error())
			break
		}
		if chunk.Done {
			s.writeSSEDone(w, flusher)
			break
		}

		// Send chunk
		s.writeSSEContent(w, flusher, chunk.Content)
		fullResponse += chunk.Content
	}

	// Add assistant message to session
	s.engine.GetSessionManager().AddMessage(session.ID, core.Message{
		Role:    "assistant",
		Content: fullResponse,
	})
}

func (s *Server) writeSSEContent(w http.ResponseWriter, f http.Flusher, content string) {
	data, _ := json.Marshal(map[string]string{"content": content})
	fmt.Fprintf(w, "data: %s\n\n", data)
	f.Flush()
}

func (s *Server) writeSSEError(w http.ResponseWriter, f http.Flusher, msg string) {
	data, _ := json.Marshal(map[string]string{"error": msg})
	fmt.Fprintf(w, "data: %s\n\n", data)
	f.Flush()
}

func (s *Server) writeSSEDone(w http.ResponseWriter, f http.Flusher) {
	data, _ := json.Marshal(map[string]bool{"done": true})
	fmt.Fprintf(w, "data: %s\n\n", data)
	f.Flush()
}
