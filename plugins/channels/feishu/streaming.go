package feishu

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/Shadow-Azure/Golem/internal/plugin"
)

// StreamingManagerConfig configures the StreamingManager.
type StreamingManagerConfig struct {
	MinUpdateInterval time.Duration                                                       // Min interval between card updates (default 160ms)
	MinCharsDelta     int                                                                 // Min char change before triggering update (default 18)
	OnCreateCard      func(ctx context.Context, chatID, messageID string) (string, error) // Create card, return cardID
	OnUpdateCard      func(ctx context.Context, cardID, content string) error             // Update card content
	OnCloseCard       func(ctx context.Context, cardID, content string) error             // Finalize card
	OnFallback        func(ctx context.Context, sessionID, content string) error          // Fallback when card fails
}

// streamingSession holds state for an active streaming reply.
type streamingSession struct {
	sessionID      string
	cardID         string
	content        string
	lastUpdateTime time.Time
	pendingContent string
}

// StreamingManager manages Feishu Card Kit streaming replies.
type StreamingManager struct {
	config StreamingManagerConfig
	mu     sync.Mutex
	cards  map[string]*streamingSession // sessionID -> session
	logger *slog.Logger
}

// NewStreamingManager creates a new StreamingManager.
// MinUpdateInterval and MinCharsDelta default to 160ms and 18 chars if not set (zero values).
// Pass explicit small values like time.Nanosecond and 1 to disable throttling in tests.
func NewStreamingManager(cfg StreamingManagerConfig) *StreamingManager {
	if cfg.OnCreateCard == nil {
		panic("StreamingManager: OnCreateCard callback must not be nil")
	}
	if cfg.OnUpdateCard == nil {
		panic("StreamingManager: OnUpdateCard callback must not be nil")
	}
	if cfg.OnCloseCard == nil {
		panic("StreamingManager: OnCloseCard callback must not be nil")
	}
	if cfg.MinUpdateInterval == 0 {
		cfg.MinUpdateInterval = 160 * time.Millisecond
	}
	if cfg.MinCharsDelta == 0 {
		cfg.MinCharsDelta = 18
	}
	return &StreamingManager{
		config: cfg,
		cards:  make(map[string]*streamingSession),
		logger: slog.Default().With("component", "streaming_manager"),
	}
}

// CreateStreamReply creates a new streaming reply session.
// Returns an error if the sessionID already has an active streaming session.
func (m *StreamingManager) CreateStreamReply(ctx context.Context, sessionID string, opts plugin.StreamReplyOptions) (*plugin.StreamSession, error) {
	m.mu.Lock()
	if _, exists := m.cards[sessionID]; exists {
		m.mu.Unlock()
		return nil, fmt.Errorf("duplicate streaming session: %s", sessionID)
	}
	// Reserve the slot before releasing the lock to call the external callback.
	m.cards[sessionID] = &streamingSession{
		sessionID:      sessionID,
		lastUpdateTime: time.Now(),
	}
	m.mu.Unlock()

	cardID, err := m.config.OnCreateCard(ctx, opts.ChatID, opts.MessageID)
	if err != nil {
		m.mu.Lock()
		delete(m.cards, sessionID)
		m.mu.Unlock()
		return nil, fmt.Errorf("failed to create card: %w", err)
	}

	m.mu.Lock()
	m.cards[sessionID].cardID = cardID
	m.mu.Unlock()

	m.logger.Info("streaming session created", "sessionID", sessionID, "cardID", cardID)

	return &plugin.StreamSession{
		SessionID: sessionID,
		CardID:    cardID,
	}, nil
}

// SendDelta appends delta content to the streaming session.
// Respects throttle: min interval and min chars delta.
func (m *StreamingManager) SendDelta(ctx context.Context, session *plugin.StreamSession, delta string) error {
	m.mu.Lock()
	card, exists := m.cards[session.SessionID]
	if !exists {
		m.mu.Unlock()
		return fmt.Errorf("streaming session not found: %s", session.SessionID)
	}

	card.content += delta
	card.pendingContent += delta

	now := time.Now()
	elapsed := now.Sub(card.lastUpdateTime)
	pendingLen := len(card.pendingContent)

	if elapsed < m.config.MinUpdateInterval || pendingLen < m.config.MinCharsDelta {
		// Throttled — will send on next tick or FinishStream
		m.mu.Unlock()
		return nil
	}

	// Send update
	content := card.content
	card.pendingContent = ""
	card.lastUpdateTime = now
	m.mu.Unlock()

	return m.config.OnUpdateCard(ctx, card.cardID, content)
}

// FinishStream completes the streaming reply and sends final content.
// If session is nil (e.g., card creation failed), triggers the fallback callback.
func (m *StreamingManager) FinishStream(ctx context.Context, session *plugin.StreamSession) error {
	if session == nil {
		if m.config.OnFallback != nil {
			return m.config.OnFallback(ctx, "", "")
		}
		return nil
	}

	m.mu.Lock()
	card, exists := m.cards[session.SessionID]
	if !exists {
		m.mu.Unlock()
		return nil
	}

	content := card.content
	delete(m.cards, session.SessionID)
	m.mu.Unlock()

	m.logger.Info("streaming session finished", "sessionID", session.SessionID, "contentLen", len(content))
	return m.config.OnCloseCard(ctx, card.cardID, content)
}
