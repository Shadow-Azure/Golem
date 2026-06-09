package plugin

import (
	"context"
	"testing"
)

// mockStreamingChannel implements StreamingCapable for testing.
type mockStreamingChannel struct {
	created  bool
	deltas   []string
	finished bool
}

func (m *mockStreamingChannel) CreateStreamReply(ctx context.Context, sessionID string, opts StreamReplyOptions) (*StreamSession, error) {
	m.created = true
	return &StreamSession{SessionID: sessionID, CardID: "test-card-id"}, nil
}

func (m *mockStreamingChannel) SendDelta(ctx context.Context, session *StreamSession, delta string) error {
	m.deltas = append(m.deltas, delta)
	return nil
}

func (m *mockStreamingChannel) FinishStream(ctx context.Context, session *StreamSession) error {
	m.finished = true
	return nil
}

// mockTypingChannel implements TypingCapable for testing.
type mockTypingChannel struct {
	started bool
	stopped bool
}

func (m *mockTypingChannel) StartTyping(ctx context.Context, sessionID string, messageID string) error {
	m.started = true
	return nil
}

func (m *mockTypingChannel) StopTyping(ctx context.Context, sessionID string) error {
	m.stopped = true
	return nil
}

func TestStreamingCapable_TypeAssertion(t *testing.T) {
	ctx := context.Background()
	mock := &mockStreamingChannel{}
	var ch interface{} = mock
	sc, ok := ch.(StreamingCapable)
	if !ok {
		t.Fatal("expected type assertion to StreamingCapable to succeed")
	}

	session, err := sc.CreateStreamReply(ctx, "sess1", StreamReplyOptions{MessageID: "msg1"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if session.CardID != "test-card-id" {
		t.Errorf("expected card ID 'test-card-id', got '%s'", session.CardID)
	}
	if !mock.created {
		t.Error("expected mock.created to be true after CreateStreamReply")
	}

	if err := sc.SendDelta(ctx, session, "hello"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(mock.deltas) != 1 || mock.deltas[0] != "hello" {
		t.Errorf("expected mock.deltas to contain ['hello'], got %v", mock.deltas)
	}

	if err := sc.FinishStream(ctx, session); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !mock.finished {
		t.Error("expected mock.finished to be true after FinishStream")
	}
}

func TestTypingCapable_TypeAssertion(t *testing.T) {
	ctx := context.Background()
	mock := &mockTypingChannel{}
	var ch interface{} = mock
	tc, ok := ch.(TypingCapable)
	if !ok {
		t.Fatal("expected type assertion to TypingCapable to succeed")
	}

	if err := tc.StartTyping(ctx, "sess1", "msg1"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !mock.started {
		t.Error("expected mock.started to be true after StartTyping")
	}

	if err := tc.StopTyping(ctx, "sess1"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !mock.stopped {
		t.Error("expected mock.stopped to be true after StopTyping")
	}
}

func TestNonCapableChannel_TypeAssertionFails(t *testing.T) {
	var ch interface{} = struct{}{}
	if _, ok := ch.(StreamingCapable); ok {
		t.Error("expected type assertion to StreamingCapable to fail for non-capable channel")
	}
	if _, ok := ch.(TypingCapable); ok {
		t.Error("expected type assertion to TypingCapable to fail for non-capable channel")
	}
}

// mockTypingCapableChannel implements TypingCapable for testing (Engine perspective).
type mockTypingCapableChannel struct {
	started bool
	stopped bool
}

func (m *mockTypingCapableChannel) StartTyping(ctx context.Context, sessionID string, messageID string) error {
	m.started = true
	return nil
}

func (m *mockTypingCapableChannel) StopTyping(ctx context.Context, sessionID string) error {
	m.stopped = true
	return nil
}

// mockStreamingCapableChannel implements StreamingCapable for testing (Engine perspective).
type mockStreamingCapableChannel struct {
	deltas   []string
	finished bool
}

func (m *mockStreamingCapableChannel) CreateStreamReply(ctx context.Context, sessionID string, opts StreamReplyOptions) (*StreamSession, error) {
	return &StreamSession{SessionID: sessionID, CardID: "mock-card"}, nil
}

func (m *mockStreamingCapableChannel) SendDelta(ctx context.Context, session *StreamSession, delta string) error {
	m.deltas = append(m.deltas, delta)
	return nil
}

func (m *mockStreamingCapableChannel) FinishStream(ctx context.Context, session *StreamSession) error {
	m.finished = true
	return nil
}

func TestEngine_DetectsTypingCapable(t *testing.T) {
	ch := &mockTypingCapableChannel{}

	tc, ok := interface{}(ch).(TypingCapable)
	if !ok {
		t.Fatal("expected channel to implement TypingCapable")
	}

	tc.StartTyping(context.Background(), "sess", "msg")
	if !ch.started {
		t.Error("expected StartTyping to be called")
	}

	tc.StopTyping(context.Background(), "sess")
	if !ch.stopped {
		t.Error("expected StopTyping to be called")
	}
}

func TestEngine_DetectsStreamingCapable(t *testing.T) {
	ch := &mockStreamingCapableChannel{}

	sc, ok := interface{}(ch).(StreamingCapable)
	if !ok {
		t.Fatal("expected channel to implement StreamingCapable")
	}

	session, err := sc.CreateStreamReply(context.Background(), "sess", StreamReplyOptions{MessageID: "msg"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	sc.SendDelta(context.Background(), session, "hello")
	sc.FinishStream(context.Background(), session)

	if len(ch.deltas) != 1 || ch.deltas[0] != "hello" {
		t.Errorf("expected delta 'hello', got %v", ch.deltas)
	}
	if !ch.finished {
		t.Error("expected FinishStream to be called")
	}
}
