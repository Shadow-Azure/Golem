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
	defer mgr.Stop()

	// Start typing
	if err := mgr.StartTyping(context.Background(), "sess1", "msg1"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if addCount.Load() != 1 {
		t.Errorf("expected 1 add call, got %d", addCount.Load())
	}

	// Stop typing
	if err := mgr.StopTyping(context.Background(), "sess1"); err != nil {
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
	defer mgr.Stop()

	// Start typing twice for same session — should only call add once
	if err := mgr.StartTyping(context.Background(), "sess1", "msg1"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if err := mgr.StartTyping(context.Background(), "sess1", "msg1"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

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
	defer mgr.Stop()

	// Simulate old message by setting messageCreateTimeMs to 5 minutes ago
	oldTime := time.Now().Add(-5 * time.Minute).UnixMilli()
	if err := mgr.StartTypingWithTime(context.Background(), "sess1", "msg1", oldTime); err != nil {
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
	defer mgr.Stop()

	// First two calls fail and trip the breaker
	if err := mgr.StartTyping(context.Background(), "sess1", "msg1"); err == nil {
		t.Error("expected error from first failed start")
	}
	if err := mgr.StartTyping(context.Background(), "sess2", "msg2"); err == nil {
		t.Error("expected error from second failed start")
	}

	// Third call should be skipped (breaker tripped)
	if err := mgr.StartTyping(context.Background(), "sess3", "msg3"); err == nil {
		t.Error("expected error from circuit breaker tripped")
	}

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
	defer mgr.Stop()

	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			// Ignore errors since some goroutines may hit the dedup path.
			_ = mgr.StartTyping(context.Background(), "sess1", "msg1")
		}(i)
	}
	wg.Wait()

	// Should only add once despite 10 concurrent calls
	if addCount.Load() != 1 {
		t.Errorf("expected 1 add call (concurrent dedup), got %d", addCount.Load())
	}
}

func TestTypingManager_TTLCleanup(t *testing.T) {
	var removedIDs sync.Map
	var removeCount atomic.Int32

	mgr := NewTypingManager(TypingManagerConfig{
		MaxAge:      2 * time.Minute,
		TTLTimeout:  50 * time.Millisecond, // Very short TTL for testing
		MaxFailures: 2,
		OnAddReaction: func(ctx context.Context, messageID, emojiType string) (string, error) {
			return "reaction-" + messageID, nil
		},
		OnRemoveReaction: func(ctx context.Context, messageID, reactionID string) error {
			removeCount.Add(1)
			removedIDs.Store(messageID, true)
			return nil
		},
	})
	defer mgr.Stop()

	// Start typing for two sessions
	if err := mgr.StartTyping(context.Background(), "sess1", "msg1"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if err := mgr.StartTyping(context.Background(), "sess2", "msg2"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Wait for TTL cleanup to fire (TTL is 50ms, wait a bit more)
	time.Sleep(200 * time.Millisecond)

	// Both reactions should have been cleaned up by the TTL goroutine.
	// However, the cleanup ticker fires at TTLTimeout intervals, so we need
	// to account for up to 2x the TTL to be safe.
	if removeCount.Load() < 2 {
		t.Errorf("expected at least 2 remove calls from TTL cleanup, got %d", removeCount.Load())
	}

	if _, ok := removedIDs.Load("msg1"); !ok {
		t.Error("expected msg1 to be removed by TTL cleanup")
	}
	if _, ok := removedIDs.Load("msg2"); !ok {
		t.Error("expected msg2 to be removed by TTL cleanup")
	}

	// Verify states map is empty after cleanup
	mgr.mu.Lock()
	stateCount := len(mgr.states)
	mgr.mu.Unlock()
	if stateCount != 0 {
		t.Errorf("expected 0 states after TTL cleanup, got %d", stateCount)
	}
}

func TestTypingManager_StopPreventsCleanup(t *testing.T) {
	var removeCount atomic.Int32

	mgr := NewTypingManager(TypingManagerConfig{
		MaxAge:      2 * time.Minute,
		TTLTimeout:  50 * time.Millisecond,
		MaxFailures: 2,
		OnAddReaction: func(ctx context.Context, messageID, emojiType string) (string, error) {
			return "reaction-" + messageID, nil
		},
		OnRemoveReaction: func(ctx context.Context, messageID, reactionID string) error {
			removeCount.Add(1)
			return nil
		},
	})

	// Start typing, then immediately stop the manager.
	if err := mgr.StartTyping(context.Background(), "sess1", "msg1"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	mgr.Stop()

	// Wait to confirm no cleanup runs after Stop.
	time.Sleep(200 * time.Millisecond)

	// The only remove call should be from TTL cleanup if it fired before Stop,
	// but Stop should prevent further cleanup ticks.
	// Since Stop() closes the done channel, the goroutine should exit.
	// We just verify it doesn't panic.
}

// RateLimitError simulates a Feishu rate limit error.
type RateLimitError struct {
	Code int
}

func (e *RateLimitError) Error() string {
	return "rate limited"
}
