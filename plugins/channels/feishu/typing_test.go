package feishu

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestTypingManager_StartAndStop(t *testing.T) {
	var addCount, removeCount atomic.Int32

	mgr := NewTypingManager(TypingManagerConfig{
		MaxAge:      2 * time.Minute,
		TTLTimeout:  60 * time.Second,
		MaxFailures: 2,
		OnAddReaction: func(ctx context.Context, messageID, emojiType string) (string, error) {
			addCount.Add(1)
			return "reaction-123", nil
		},
		OnRemoveReaction: func(ctx context.Context, messageID, reactionID string) error {
			removeCount.Add(1)
			return nil
		},
	})

	// Start typing
	err := mgr.StartTyping(context.Background(), "sess1", "msg1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if addCount.Load() != 1 {
		t.Errorf("expected 1 add call, got %d", addCount.Load())
	}

	// Stop typing
	err = mgr.StopTyping(context.Background(), "sess1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if removeCount.Load() != 1 {
		t.Errorf("expected 1 remove call, got %d", removeCount.Load())
	}
}

func TestTypingManager_Deduplication(t *testing.T) {
	var addCount atomic.Int32

	mgr := NewTypingManager(TypingManagerConfig{
		MaxAge: 2 * time.Minute,
		OnAddReaction: func(ctx context.Context, messageID, emojiType string) (string, error) {
			addCount.Add(1)
			return "reaction-123", nil
		},
		OnRemoveReaction: func(ctx context.Context, messageID, reactionID string) error {
			return nil
		},
	})

	// Start typing twice for same session — should only call add once
	mgr.StartTyping(context.Background(), "sess1", "msg1")
	mgr.StartTyping(context.Background(), "sess1", "msg1")

	if addCount.Load() != 1 {
		t.Errorf("expected 1 add call (dedup), got %d", addCount.Load())
	}
}

func TestTypingManager_OldMessageSkipped(t *testing.T) {
	var addCount atomic.Int32

	mgr := NewTypingManager(TypingManagerConfig{
		MaxAge: 2 * time.Minute,
		OnAddReaction: func(ctx context.Context, messageID, emojiType string) (string, error) {
			addCount.Add(1)
			return "reaction-123", nil
		},
		OnRemoveReaction: func(ctx context.Context, messageID, reactionID string) error {
			return nil
		},
	})

	// Simulate old message by setting messageCreateTimeMs to 5 minutes ago
	oldTime := time.Now().Add(-5 * time.Minute).UnixMilli()
	err := mgr.StartTypingWithTime(context.Background(), "sess1", "msg1", oldTime)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should NOT have called add (message too old)
	if addCount.Load() != 0 {
		t.Errorf("expected 0 add calls for old message, got %d", addCount.Load())
	}
}

func TestTypingManager_CircuitBreaker(t *testing.T) {
	var addCount atomic.Int32

	mgr := NewTypingManager(TypingManagerConfig{
		MaxAge:      2 * time.Minute,
		MaxFailures: 2,
		OnAddReaction: func(ctx context.Context, messageID, emojiType string) (string, error) {
			addCount.Add(1)
			return "", &RateLimitError{Code: 99991400}
		},
		OnRemoveReaction: func(ctx context.Context, messageID, reactionID string) error {
			return nil
		},
	})

	// First two calls fail and trip the breaker
	mgr.StartTyping(context.Background(), "sess1", "msg1")
	mgr.StartTyping(context.Background(), "sess2", "msg2")

	// Third call should be skipped (breaker tripped)
	mgr.StartTyping(context.Background(), "sess3", "msg3")

	if addCount.Load() != 2 {
		t.Errorf("expected 2 add calls (breaker tripped after 2), got %d", addCount.Load())
	}
}

func TestTypingManager_ConcurrentAccess(t *testing.T) {
	var addCount atomic.Int32

	mgr := NewTypingManager(TypingManagerConfig{
		MaxAge: 2 * time.Minute,
		OnAddReaction: func(ctx context.Context, messageID, emojiType string) (string, error) {
			addCount.Add(1)
			return "reaction-123", nil
		},
		OnRemoveReaction: func(ctx context.Context, messageID, reactionID string) error {
			return nil
		},
	})

	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			mgr.StartTyping(context.Background(), "sess1", "msg1")
		}(i)
	}
	wg.Wait()

	// Should only add once despite 10 concurrent calls
	if addCount.Load() != 1 {
		t.Errorf("expected 1 add call (concurrent dedup), got %d", addCount.Load())
	}
}

// RateLimitError simulates a Feishu rate limit error.
type RateLimitError struct {
	Code int
}

func (e *RateLimitError) Error() string {
	return "rate limited"
}
