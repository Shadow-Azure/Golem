package core

import (
	"testing"
	"time"
)

func TestSessionManager_CreateSession(t *testing.T) {
	sm := NewSessionManager(SessionConfig{
		MaxHistory: 50,
		TrimTo:     20,
	})

	session, err := sm.CreateSession("user123", "feishu")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if session.UserID != "user123" {
		t.Errorf("expected userID 'user123', got '%s'", session.UserID)
	}
	if session.Channel != "feishu" {
		t.Errorf("expected channel 'feishu', got '%s'", session.Channel)
	}
	if session.ID == "" {
		t.Error("session ID should not be empty")
	}
	if session.ID != "feishu:user123" {
		t.Errorf("expected session ID 'feishu:user123', got '%s'", session.ID)
	}
}

func TestSessionManager_GetSession(t *testing.T) {
	sm := NewSessionManager(SessionConfig{
		MaxHistory: 50,
		TrimTo:     20,
	})

	created, _ := sm.CreateSession("user123", "feishu")

	retrieved, err := sm.GetSession(created.ID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if retrieved.ID != created.ID {
		t.Errorf("expected session ID '%s', got '%s'", created.ID, retrieved.ID)
	}
}

func TestSessionManager_GetSession_NotFound(t *testing.T) {
	sm := NewSessionManager(SessionConfig{
		MaxHistory: 50,
		TrimTo:     20,
	})

	_, err := sm.GetSession("nonexistent")
	if err == nil {
		t.Fatal("expected error for nonexistent session")
	}
}

func TestSessionManager_AddMessage(t *testing.T) {
	sm := NewSessionManager(SessionConfig{
		MaxHistory: 50,
		TrimTo:     20,
	})

	session, _ := sm.CreateSession("user123", "feishu")

	msg := Message{
		Role:    "user",
		Content: "Hello, world!",
	}

	err := sm.AddMessage(session.ID, msg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	retrieved, _ := sm.GetSession(session.ID)
	if len(retrieved.Messages) != 1 {
		t.Fatalf("expected 1 message, got %d", len(retrieved.Messages))
	}
	if retrieved.Messages[0].Content != "Hello, world!" {
		t.Errorf("expected message content 'Hello, world!', got '%s'", retrieved.Messages[0].Content)
	}
}

func TestSessionManager_AddMessage_NotFound(t *testing.T) {
	sm := NewSessionManager(SessionConfig{
		MaxHistory: 50,
		TrimTo:     20,
	})

	err := sm.AddMessage("nonexistent", Message{
		Role:    "user",
		Content: "test",
	})
	if err == nil {
		t.Fatal("expected error for nonexistent session")
	}
}

func TestSessionManager_AddMessage_TimestampAutoFill(t *testing.T) {
	sm := NewSessionManager(SessionConfig{
		MaxHistory: 50,
		TrimTo:     20,
	})

	session, _ := sm.CreateSession("user123", "feishu")

	msg := Message{
		Role:    "user",
		Content: "no timestamp",
	}

	err := sm.AddMessage(session.ID, msg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	retrieved, _ := sm.GetSession(session.ID)
	if retrieved.Messages[0].Timestamp.IsZero() {
		t.Error("expected timestamp to be auto-filled")
	}
}

func TestSessionManager_HistoryTrimming(t *testing.T) {
	sm := NewSessionManager(SessionConfig{
		MaxHistory: 5,
		TrimTo:     3,
	})

	session, _ := sm.CreateSession("user123", "feishu")

	// Add 6 messages (exceeds max_history of 5)
	for i := 0; i < 6; i++ {
		sm.AddMessage(session.ID, Message{
			Role:    "user",
			Content: "message",
		})
	}

	retrieved, _ := sm.GetSession(session.ID)
	if len(retrieved.Messages) != 3 {
		t.Errorf("expected 3 messages after trimming, got %d", len(retrieved.Messages))
	}
}

func TestSessionManager_GetHistory(t *testing.T) {
	sm := NewSessionManager(SessionConfig{
		MaxHistory: 50,
		TrimTo:     20,
	})

	session, _ := sm.CreateSession("user123", "feishu")

	for i := 0; i < 10; i++ {
		sm.AddMessage(session.ID, Message{
			Role:    "user",
			Content: "message",
		})
	}

	history, err := sm.GetHistory(session.ID, 5)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(history) != 5 {
		t.Errorf("expected 5 messages, got %d", len(history))
	}
}

func TestSessionManager_GetHistory_AllMessages(t *testing.T) {
	sm := NewSessionManager(SessionConfig{
		MaxHistory: 50,
		TrimTo:     20,
	})

	session, _ := sm.CreateSession("user123", "feishu")

	for i := 0; i < 5; i++ {
		sm.AddMessage(session.ID, Message{
			Role:    "user",
			Content: "message",
		})
	}

	// limit <= 0 should return all messages
	history, err := sm.GetHistory(session.ID, 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(history) != 5 {
		t.Errorf("expected 5 messages, got %d", len(history))
	}

	// limit > len should also return all messages
	history, err = sm.GetHistory(session.ID, 100)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(history) != 5 {
		t.Errorf("expected 5 messages, got %d", len(history))
	}
}

func TestSessionManager_GetHistory_NotFound(t *testing.T) {
	sm := NewSessionManager(SessionConfig{
		MaxHistory: 50,
		TrimTo:     20,
	})

	_, err := sm.GetHistory("nonexistent", 10)
	if err == nil {
		t.Fatal("expected error for nonexistent session")
	}
}

func TestSessionManager_DeleteSession(t *testing.T) {
	sm := NewSessionManager(SessionConfig{
		MaxHistory: 50,
		TrimTo:     20,
	})

	session, _ := sm.CreateSession("user123", "feishu")

	err := sm.DeleteSession(session.ID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	_, err = sm.GetSession(session.ID)
	if err == nil {
		t.Fatal("expected error after deleting session")
	}
}

func TestSessionManager_DeleteSession_NotFound(t *testing.T) {
	sm := NewSessionManager(SessionConfig{
		MaxHistory: 50,
		TrimTo:     20,
	})

	err := sm.DeleteSession("nonexistent")
	if err == nil {
		t.Fatal("expected error for nonexistent session")
	}
}

func TestSessionManager_CleanupStale(t *testing.T) {
	sm := NewSessionManager(SessionConfig{
		MaxHistory: 50,
		TrimTo:     20,
	})

	// Create sessions with old timestamps
	session1, _ := sm.CreateSession("user1", "feishu")
	sm.mu.Lock()
	sm.sessions[session1.ID].UpdatedAt = time.Now().Add(-2 * time.Hour)
	sm.mu.Unlock()

	session2, _ := sm.CreateSession("user2", "feishu")

	cleaned := sm.CleanupStale(1 * time.Hour)
	if cleaned != 1 {
		t.Errorf("expected 1 session cleaned, got %d", cleaned)
	}

	// Verify session2 still exists
	_, err := sm.GetSession(session2.ID)
	if err != nil {
		t.Fatal("session2 should still exist")
	}
}

func TestSessionManager_CleanupStale_NoneExpired(t *testing.T) {
	sm := NewSessionManager(SessionConfig{
		MaxHistory: 50,
		TrimTo:     20,
	})

	sm.CreateSession("user1", "feishu")
	sm.CreateSession("user2", "feishu")

	cleaned := sm.CleanupStale(1 * time.Hour)
	if cleaned != 0 {
		t.Errorf("expected 0 sessions cleaned, got %d", cleaned)
	}
}

func TestSessionManager_GetOrCreateSession(t *testing.T) {
	sm := NewSessionManager(SessionConfig{
		MaxHistory: 50,
		TrimTo:     20,
	})

	// First call should create
	session1, err := sm.GetOrCreateSession("feishu:user1", "user1", "feishu")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Second call should return existing
	session2, err := sm.GetOrCreateSession("feishu:user1", "user1", "feishu")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if session1.ID != session2.ID {
		t.Errorf("expected same session ID, got %s and %s", session1.ID, session2.ID)
	}
}

func TestSessionManager_InterfaceCompliance(t *testing.T) {
	var _ SessionManagerInterface = (*SessionManager)(nil)
}
