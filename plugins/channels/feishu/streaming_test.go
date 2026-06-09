package feishu

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/Shadow-Azure/Golem/internal/plugin"
)

func TestStreamingManager_CreateAndFinish(t *testing.T) {
	var created, closed bool
	var mu sync.Mutex

	mgr := NewStreamingManager(StreamingManagerConfig{
		MinUpdateInterval: time.Nanosecond, // No throttle for test
		MinCharsDelta:     1,
		OnCreateCard: func(ctx context.Context, chatID, messageID string) (string, error) {
			mu.Lock()
			created = true
			mu.Unlock()
			return "card-123", nil
		},
		OnUpdateCard: func(ctx context.Context, cardID, content string) error {
			return nil
		},
		OnCloseCard: func(ctx context.Context, cardID, content string) error {
			mu.Lock()
			closed = true
			mu.Unlock()
			return nil
		},
		OnFallback: func(ctx context.Context, sessionID, content string) error {
			return nil
		},
	})

	session, err := mgr.CreateStreamReply(context.Background(), "sess1", plugin.StreamReplyOptions{
		MessageID: "msg1",
		ChatID:    "chat1",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if session.CardID != "card-123" {
		t.Errorf("expected card ID 'card-123', got '%s'", session.CardID)
	}

	mu.Lock()
	if !created {
		t.Error("expected card to be created")
	}
	mu.Unlock()

	err = mgr.FinishStream(context.Background(), session)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	mu.Lock()
	if !closed {
		t.Error("expected card to be closed")
	}
	mu.Unlock()
}

func TestStreamingManager_SendDelta(t *testing.T) {
	var lastContent string
	var updateCount int
	var mu sync.Mutex

	mgr := NewStreamingManager(StreamingManagerConfig{
		MinUpdateInterval: time.Nanosecond, // No throttle for test
		MinCharsDelta:     1,
		OnCreateCard: func(ctx context.Context, chatID, messageID string) (string, error) {
			return "card-123", nil
		},
		OnUpdateCard: func(ctx context.Context, cardID, content string) error {
			mu.Lock()
			lastContent = content
			updateCount++
			mu.Unlock()
			return nil
		},
		OnCloseCard: func(ctx context.Context, cardID, content string) error {
			return nil
		},
		OnFallback: func(ctx context.Context, sessionID, content string) error {
			return nil
		},
	})

	session, _ := mgr.CreateStreamReply(context.Background(), "sess1", plugin.StreamReplyOptions{
		MessageID: "msg1",
		ChatID:    "chat1",
	})

	mgr.SendDelta(context.Background(), session, "Hello")
	mgr.SendDelta(context.Background(), session, " World")

	mu.Lock()
	if lastContent != "Hello World" {
		t.Errorf("expected content 'Hello World', got '%s'", lastContent)
	}
	mu.Unlock()
}

func TestStreamingManager_Throttle(t *testing.T) {
	var updateCount int32
	var mu sync.Mutex

	mgr := NewStreamingManager(StreamingManagerConfig{
		MinUpdateInterval: 100 * time.Millisecond,
		MinCharsDelta:     10,
		OnCreateCard: func(ctx context.Context, chatID, messageID string) (string, error) {
			return "card-123", nil
		},
		OnUpdateCard: func(ctx context.Context, cardID, content string) error {
			mu.Lock()
			updateCount++
			mu.Unlock()
			return nil
		},
		OnCloseCard: func(ctx context.Context, cardID, content string) error {
			return nil
		},
		OnFallback: func(ctx context.Context, sessionID, content string) error {
			return nil
		},
	})

	session, _ := mgr.CreateStreamReply(context.Background(), "sess1", plugin.StreamReplyOptions{
		MessageID: "msg1",
		ChatID:    "chat1",
	})

	// Send multiple small deltas quickly — should be throttled
	mgr.SendDelta(context.Background(), session, "ab")
	mgr.SendDelta(context.Background(), session, "cd")
	mgr.SendDelta(context.Background(), session, "ef")

	mu.Lock()
	count := updateCount
	mu.Unlock()

	// With minCharsDelta=10, "abcdef" is only 6 chars, so 0 updates
	if count != 0 {
		t.Errorf("expected 0 updates (throttled), got %d", count)
	}
}

func TestStreamingManager_FallbackOnCreateFailure(t *testing.T) {
	var fallbackCalled bool
	var mu sync.Mutex

	mgr := NewStreamingManager(StreamingManagerConfig{
		OnCreateCard: func(ctx context.Context, chatID, messageID string) (string, error) {
			return "", fmt.Errorf("card creation failed")
		},
		OnUpdateCard: func(ctx context.Context, cardID, content string) error {
			return nil
		},
		OnCloseCard: func(ctx context.Context, cardID, content string) error {
			return nil
		},
		OnFallback: func(ctx context.Context, sessionID, content string) error {
			mu.Lock()
			fallbackCalled = true
			mu.Unlock()
			return nil
		},
	})

	session, err := mgr.CreateStreamReply(context.Background(), "sess1", plugin.StreamReplyOptions{
		MessageID: "msg1",
		ChatID:    "chat1",
	})
	if err == nil {
		t.Fatal("expected error on card creation failure")
	}
	if session != nil {
		t.Error("expected nil session on failure")
	}

	// FinishStream with nil session should trigger fallback
	mgr.FinishStream(context.Background(), nil)

	mu.Lock()
	if !fallbackCalled {
		t.Error("expected fallback to be called")
	}
	mu.Unlock()
}
