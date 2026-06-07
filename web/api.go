// web/api.go
package web

import (
	"encoding/json"
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
