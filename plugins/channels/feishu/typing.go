package feishu

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"
)

// Feishu rate limit error codes.
const (
	ErrCodeRateLimit     = 99991400
	ErrCodeQuotaExceeded = 99991403
	ErrCodeHTTP429       = 429
)

// TypingManagerConfig configures the TypingManager.
type TypingManagerConfig struct {
	MaxAge           time.Duration                                         // Max message age to show typing (default 2min)
	TTLTimeout       time.Duration                                         // Auto-remove typing after this duration (default 60s)
	MaxFailures      int                                                   // Max consecutive failures before circuit breaker trips (default 2)
	OnAddReaction    func(ctx context.Context, messageID, emojiType string) (string, error) // Callback to add emoji reaction
	OnRemoveReaction func(ctx context.Context, messageID, reactionID string) error          // Callback to remove emoji reaction
}

// TypingState holds the state of an active typing indicator.
type TypingState struct {
	MessageID  string
	ReactionID string
	StartTime  time.Time
}

// TypingManager manages typing indicators for Feishu messages.
type TypingManager struct {
	config       TypingManagerConfig
	mu           sync.Mutex
	states       map[string]*TypingState // sessionID -> state
	consecutiveF int
	tripped      bool
	logger       *slog.Logger
}

// NewTypingManager creates a new TypingManager.
func NewTypingManager(cfg TypingManagerConfig) *TypingManager {
	if cfg.MaxAge == 0 {
		cfg.MaxAge = 2 * time.Minute
	}
	if cfg.TTLTimeout == 0 {
		cfg.TTLTimeout = 60 * time.Second
	}
	if cfg.MaxFailures == 0 {
		cfg.MaxFailures = 2
	}
	return &TypingManager{
		config: cfg,
		states: make(map[string]*TypingState),
		logger: slog.Default().With("component", "typing_manager"),
	}
}

// StartTyping begins showing a typing indicator for the given message.
func (m *TypingManager) StartTyping(ctx context.Context, sessionID string, messageID string) error {
	return m.StartTypingWithTime(ctx, sessionID, messageID, 0)
}

// StartTypingWithTime begins showing a typing indicator, with an explicit message creation time.
// If messageCreateTimeMs is 0, the current time is used.
func (m *TypingManager) StartTypingWithTime(ctx context.Context, sessionID string, messageID string, messageCreateTimeMs int64) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Circuit breaker check
	if m.tripped {
		return fmt.Errorf("typing circuit breaker tripped")
	}

	// Dedup: if already showing typing for this session, skip
	if state, exists := m.states[sessionID]; exists && state.ReactionID != "" {
		return nil
	}

	// Message age check
	if messageCreateTimeMs > 0 {
		msgAge := time.Since(time.UnixMilli(messageCreateTimeMs))
		if msgAge > m.config.MaxAge {
			m.logger.Debug("skipping typing for old message",
				"session_id", sessionID,
				"age", msgAge,
				"max_age", m.config.MaxAge,
			)
			return nil
		}
	}

	// Add reaction
	reactionID, err := m.config.OnAddReaction(ctx, messageID, "Typing")
	if err != nil {
		m.consecutiveF++
		if m.consecutiveF >= m.config.MaxFailures {
			m.tripped = true
			m.logger.Warn("typing circuit breaker tripped",
				"failures", m.consecutiveF,
				"error", err,
			)
		}
		return err
	}

	// Reset failure counter on success
	m.consecutiveF = 0

	m.states[sessionID] = &TypingState{
		MessageID:  messageID,
		ReactionID: reactionID,
		StartTime:  time.Now(),
	}

	return nil
}

// StopTyping stops showing the typing indicator.
func (m *TypingManager) StopTyping(ctx context.Context, sessionID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	state, exists := m.states[sessionID]
	if !exists {
		return nil
	}

	delete(m.states, sessionID)

	if state.ReactionID == "" {
		return nil
	}

	if err := m.config.OnRemoveReaction(ctx, state.MessageID, state.ReactionID); err != nil {
		m.logger.Warn("failed to remove typing reaction",
			"session_id", sessionID,
			"error", err,
		)
		return err
	}

	return nil
}

// Reset resets the circuit breaker. Called when a new message arrives.
func (m *TypingManager) Reset() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.tripped = false
	m.consecutiveF = 0
}
