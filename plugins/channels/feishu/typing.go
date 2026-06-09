package feishu

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"
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
	config               TypingManagerConfig
	mu                   sync.Mutex
	states               map[string]*TypingState // sessionID -> state
	consecutiveFailures  int
	tripped              bool
	done                 chan struct{}
	logger               *slog.Logger
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
	m := &TypingManager{
		config: cfg,
		states: make(map[string]*TypingState),
		done:   make(chan struct{}),
		logger: slog.Default().With("component", "typing_manager"),
	}
	go m.cleanup()
	return m
}

// Stop signals the cleanup goroutine to exit.
func (m *TypingManager) Stop() {
	close(m.done)
}

// cleanup periodically removes expired typing states.
func (m *TypingManager) cleanup() {
	ticker := time.NewTicker(m.config.TTLTimeout)
	defer ticker.Stop()

	for {
		select {
		case <-m.done:
			return
		case <-ticker.C:
			m.mu.Lock()
			cutoff := time.Now().Add(-m.config.TTLTimeout)
			var expired []TypingState
			for sessionID, state := range m.states {
				if state.StartTime.Before(cutoff) {
					expired = append(expired, *state)
					delete(m.states, sessionID)
				}
			}
			m.mu.Unlock()

			// Call OnRemoveReaction outside the lock.
			for _, state := range expired {
				if state.ReactionID != "" {
					if err := m.config.OnRemoveReaction(context.Background(), state.MessageID, state.ReactionID); err != nil {
						m.logger.Warn("failed to remove expired typing reaction",
							"message_id", state.MessageID,
							"reaction_id", state.ReactionID,
							"error", err,
						)
					}
				}
			}
		}
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

	// Circuit breaker check
	if m.tripped {
		m.mu.Unlock()
		return fmt.Errorf("typing circuit breaker tripped")
	}

	// Dedup: if already showing typing for this session, skip
	if state, exists := m.states[sessionID]; exists && state.ReactionID != "" {
		m.mu.Unlock()
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
			m.mu.Unlock()
			return nil
		}
	}

	m.mu.Unlock()

	// Add reaction outside the lock to avoid blocking during network I/O.
	reactionID, err := m.config.OnAddReaction(ctx, messageID, "Typing")

	// Re-acquire lock to update state.
	m.mu.Lock()
	defer m.mu.Unlock()

	if err != nil {
		m.consecutiveFailures++
		if m.consecutiveFailures >= m.config.MaxFailures {
			m.tripped = true
			m.logger.Warn("typing circuit breaker tripped",
				"failures", m.consecutiveFailures,
				"error", err,
			)
		}
		return err
	}

	// Reset failure counter on success
	m.consecutiveFailures = 0

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

	state, exists := m.states[sessionID]
	if !exists {
		m.mu.Unlock()
		return nil
	}

	// Copy state and remove from map under lock.
	stateCopy := *state
	delete(m.states, sessionID)

	m.mu.Unlock()

	if stateCopy.ReactionID == "" {
		return nil
	}

	// Remove reaction outside the lock.
	if err := m.config.OnRemoveReaction(ctx, stateCopy.MessageID, stateCopy.ReactionID); err != nil {
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
	m.consecutiveFailures = 0
}
