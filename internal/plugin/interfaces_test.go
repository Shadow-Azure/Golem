package plugin

import (
	"testing"
)

// mockStreamingChannel implements StreamingCapable for testing.
type mockStreamingChannel struct {
	created  bool
	deltas   []string
	finished bool
}

func (m *mockStreamingChannel) CreateStreamReply(sessionID string, opts StreamReplyOptions) (*StreamSession, error) {
	m.created = true
	return &StreamSession{SessionID: sessionID, CardID: "test-card-id"}, nil
}

func (m *mockStreamingChannel) SendDelta(session *StreamSession, delta string) error {
	m.deltas = append(m.deltas, delta)
	return nil
}

func (m *mockStreamingChannel) FinishStream(session *StreamSession) error {
	m.finished = true
	return nil
}

// mockTypingChannel implements TypingCapable for testing.
type mockTypingChannel struct {
	started bool
	stopped bool
}

func (m *mockTypingChannel) StartTyping(sessionID string, messageID string) error {
	m.started = true
	return nil
}

func (m *mockTypingChannel) StopTyping(sessionID string) error {
	m.stopped = true
	return nil
}

func TestStreamingCapable_TypeAssertion(t *testing.T) {
	var ch interface{} = &mockStreamingChannel{}
	sc, ok := ch.(StreamingCapable)
	if !ok {
		t.Fatal("expected type assertion to StreamingCapable to succeed")
	}

	session, err := sc.CreateStreamReply("sess1", StreamReplyOptions{MessageID: "msg1"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if session.CardID != "test-card-id" {
		t.Errorf("expected card ID 'test-card-id', got '%s'", session.CardID)
	}

	if err := sc.SendDelta(session, "hello"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if err := sc.FinishStream(session); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestTypingCapable_TypeAssertion(t *testing.T) {
	var ch interface{} = &mockTypingChannel{}
	tc, ok := ch.(TypingCapable)
	if !ok {
		t.Fatal("expected type assertion to TypingCapable to succeed")
	}

	if err := tc.StartTyping("sess1", "msg1"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if err := tc.StopTyping("sess1"); err != nil {
		t.Fatalf("unexpected error: %v", err)
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
