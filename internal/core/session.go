package core

import (
	"fmt"
	"log/slog"
	"sync"
	"time"
)

// SessionConfig holds session configuration.
type SessionConfig struct {
	MaxHistory      int           `yaml:"max_history"`
	TrimTo          int           `yaml:"trim_to"`
	IdleTimeout     time.Duration `yaml:"idle_timeout"`
	CleanupInterval time.Duration `yaml:"cleanup_interval"`
}

// SessionManagerInterface defines the interface for session management.
type SessionManagerInterface interface {
	CreateSession(userID, channel string) (*Session, error)
	GetSession(sessionID string) (*Session, error)
	GetOrCreateSession(sessionID, userID, channel string) (*Session, error)
	AddMessage(sessionID string, msg Message) error
	GetHistory(sessionID string, limit int) ([]Message, error)
	DeleteSession(sessionID string) error
	CleanupStale(maxAge time.Duration) int
}

// SessionManager manages conversation sessions.
type SessionManager struct {
	sessions map[string]*Session
	mu       sync.RWMutex
	config   SessionConfig
	logger   *slog.Logger
}

// NewSessionManager creates a new SessionManager.
func NewSessionManager(config SessionConfig) *SessionManager {
	return &SessionManager{
		sessions: make(map[string]*Session),
		config:   config,
		logger:   slog.Default().With("component", "session_manager"),
	}
}

// CreateSession creates a new session.
func (sm *SessionManager) CreateSession(userID, channel string) (*Session, error) {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	sessionID := fmt.Sprintf("%s:%s", channel, userID)

	session := &Session{
		ID:        sessionID,
		UserID:    userID,
		Channel:   channel,
		Messages:  make([]Message, 0),
		Metadata:  make(map[string]interface{}),
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	sm.sessions[sessionID] = session
	sm.logger.Info("session created", "session_id", sessionID, "channel", channel)

	return session, nil
}

// GetSession returns a deep copy of a session by ID.
func (sm *SessionManager) GetSession(sessionID string) (*Session, error) {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	session, exists := sm.sessions[sessionID]
	if !exists {
		return nil, fmt.Errorf("session not found: %s", sessionID)
	}

	// Return a copy to prevent data races
	copied := *session
	copied.Messages = make([]Message, len(session.Messages))
	copy(copied.Messages, session.Messages)
	if session.Metadata != nil {
		copied.Metadata = make(map[string]interface{}, len(session.Metadata))
		for k, v := range session.Metadata {
			copied.Metadata[k] = v
		}
	}
	return &copied, nil
}

// GetOrCreateSession gets an existing session or creates a new one.
// For existing sessions, returns a deep copy to prevent data races.
func (sm *SessionManager) GetOrCreateSession(sessionID, userID, channel string) (*Session, error) {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	if session, exists := sm.sessions[sessionID]; exists {
		// Return a copy to prevent data races, consistent with GetSession.
		copied := *session
		copied.Messages = make([]Message, len(session.Messages))
		copy(copied.Messages, session.Messages)
		if session.Metadata != nil {
			copied.Metadata = make(map[string]interface{}, len(session.Metadata))
			for k, v := range session.Metadata {
				copied.Metadata[k] = v
			}
		}
		return &copied, nil
	}

	session := &Session{
		ID:        sessionID,
		UserID:    userID,
		Channel:   channel,
		Messages:  make([]Message, 0),
		Metadata:  make(map[string]interface{}),
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	sm.sessions[sessionID] = session
	sm.logger.Info("session created", "session_id", sessionID, "channel", channel)

	return session, nil
}

// AddMessage adds a message to a session.
func (sm *SessionManager) AddMessage(sessionID string, msg Message) error {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	session, exists := sm.sessions[sessionID]
	if !exists {
		return fmt.Errorf("session not found: %s", sessionID)
	}

	if msg.Timestamp.IsZero() {
		msg.Timestamp = time.Now()
	}

	session.Messages = append(session.Messages, msg)
	session.UpdatedAt = time.Now()

	// Trim history if necessary.
	if sm.config.MaxHistory > 0 && sm.config.TrimTo > 0 && len(session.Messages) > sm.config.MaxHistory {
		trimmed := len(session.Messages) - sm.config.TrimTo
		if trimmed > 0 {
			session.Messages = session.Messages[trimmed:]
			sm.logger.Debug("trimmed session history",
				"session_id", sessionID,
				"trimmed_count", trimmed,
			)
		}
	}

	return nil
}

// GetHistory returns a deep copy of the message history for a session.
func (sm *SessionManager) GetHistory(sessionID string, limit int) ([]Message, error) {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	session, exists := sm.sessions[sessionID]
	if !exists {
		return nil, fmt.Errorf("session not found: %s", sessionID)
	}

	var messages []Message
	if limit <= 0 || limit > len(session.Messages) {
		messages = session.Messages
	} else {
		messages = session.Messages[len(session.Messages)-limit:]
	}

	// Return a copy to prevent data races
	result := make([]Message, len(messages))
	copy(result, messages)
	return result, nil
}

// DeleteSession deletes a session.
func (sm *SessionManager) DeleteSession(sessionID string) error {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	if _, exists := sm.sessions[sessionID]; !exists {
		return fmt.Errorf("session not found: %s", sessionID)
	}

	delete(sm.sessions, sessionID)
	sm.logger.Info("session deleted", "session_id", sessionID)

	return nil
}

// CleanupStale removes sessions that haven't been updated within maxAge.
func (sm *SessionManager) CleanupStale(maxAge time.Duration) int {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	cleaned := 0
	cutoff := time.Now().Add(-maxAge)

	for id, session := range sm.sessions {
		if session.UpdatedAt.Before(cutoff) {
			delete(sm.sessions, id)
			cleaned++
		}
	}

	if cleaned > 0 {
		sm.logger.Info("cleaned up stale sessions", "count", cleaned)
	}

	return cleaned
}

// Ensure SessionManager implements SessionManagerInterface.
var _ SessionManagerInterface = (*SessionManager)(nil)
