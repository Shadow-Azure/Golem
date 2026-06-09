package core

import (
	"errors"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestEventBus_Publish(t *testing.T) {
	bus := NewEventBus()

	received := make(chan Event, 1)
	bus.Subscribe(EventMessageReceived, func(event Event) error {
		received <- event
		return nil
	})

	event := Event{
		Type:      EventMessageReceived,
		Source:    "test",
		Data:      "test data",
		Timestamp: time.Now(),
	}

	err := bus.Publish(event)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	select {
	case e := <-received:
		if e.Type != EventMessageReceived {
			t.Errorf("expected event type %s, got %s", EventMessageReceived, e.Type)
		}
		if e.Data != "test data" {
			t.Errorf("expected data 'test data', got %v", e.Data)
		}
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for event")
	}
}

func TestEventBus_MultipleSubscribers(t *testing.T) {
	bus := NewEventBus()

	var mu sync.Mutex
	received := make([]string, 0)

	bus.Subscribe(EventMessageReceived, func(event Event) error {
		mu.Lock()
		received = append(received, "subscriber1")
		mu.Unlock()
		return nil
	})

	bus.Subscribe(EventMessageReceived, func(event Event) error {
		mu.Lock()
		received = append(received, "subscriber2")
		mu.Unlock()
		return nil
	})

	event := Event{
		Type:      EventMessageReceived,
		Source:    "test",
		Timestamp: time.Now(),
	}

	err := bus.Publish(event)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	time.Sleep(100 * time.Millisecond)

	mu.Lock()
	if len(received) != 2 {
		t.Errorf("expected 2 subscribers to receive event, got %d", len(received))
	}
	mu.Unlock()
}

func TestEventBus_Unsubscribe(t *testing.T) {
	bus := NewEventBus()

	received := false
	handler := func(event Event) error {
		received = true
		return nil
	}

	bus.Subscribe(EventMessageReceived, handler)
	bus.Unsubscribe(EventMessageReceived, handler)

	event := Event{
		Type:      EventMessageReceived,
		Source:    "test",
		Timestamp: time.Now(),
	}

	bus.Publish(event)
	time.Sleep(100 * time.Millisecond)

	if received {
		t.Error("handler should not have been called after unsubscribe")
	}
}

func TestEventBus_HandlerError(t *testing.T) {
	bus := NewEventBus()

	expectedErr := errors.New("handler error")
	bus.Subscribe(EventMessageReceived, func(event Event) error {
		return expectedErr
	})

	event := Event{
		Type:      EventMessageReceived,
		Source:    "test",
		Timestamp: time.Now(),
	}

	err := bus.Publish(event)
	if err == nil {
		t.Fatal("expected error from publish")
	}
	if !strings.Contains(err.Error(), "handler error") {
		t.Errorf("expected error containing 'handler error', got %v", err)
	}
}

func TestEventBus_NoSubscribers(t *testing.T) {
	bus := NewEventBus()

	event := Event{
		Type:      EventMessageReceived,
		Source:    "test",
		Timestamp: time.Now(),
	}

	err := bus.Publish(event)
	if err != nil {
		t.Fatalf("unexpected error when no subscribers: %v", err)
	}
}

func TestEventBus_MultipleEventTypes(t *testing.T) {
	bus := NewEventBus()

	msgReceived := make(chan Event, 1)
	msgSent := make(chan Event, 1)

	bus.Subscribe(EventMessageReceived, func(event Event) error {
		msgReceived <- event
		return nil
	})
	bus.Subscribe(EventMessageSent, func(event Event) error {
		msgSent <- event
		return nil
	})

	receivedEvent := Event{Type: EventMessageReceived, Source: "test", Timestamp: time.Now()}
	sentEvent := Event{Type: EventMessageSent, Source: "test", Timestamp: time.Now()}

	bus.Publish(receivedEvent)
	bus.Publish(sentEvent)

	select {
	case e := <-msgReceived:
		if e.Type != EventMessageReceived {
			t.Errorf("expected EventMessageReceived, got %s", e.Type)
		}
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for received event")
	}

	select {
	case e := <-msgSent:
		if e.Type != EventMessageSent {
			t.Errorf("expected EventMessageSent, got %s", e.Type)
		}
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for sent event")
	}
}

func TestEventBus_EventMetadata(t *testing.T) {
	bus := NewEventBus()

	received := make(chan Event, 1)
	bus.Subscribe(EventStreamDelta, func(event Event) error {
		received <- event
		return nil
	})

	event := Event{
		Type:      EventStreamDelta,
		Source:    "llm-provider",
		Data:      "chunk content",
		Timestamp: time.Now(),
		Metadata: map[string]interface{}{
			"model":    "gpt-4",
			"chunk_id": 42,
		},
	}

	bus.Publish(event)

	select {
	case e := <-received:
		if e.Metadata["model"] != "gpt-4" {
			t.Errorf("expected model 'gpt-4', got %v", e.Metadata["model"])
		}
		if e.Metadata["chunk_id"] != 42 {
			t.Errorf("expected chunk_id 42, got %v", e.Metadata["chunk_id"])
		}
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for event")
	}
}

func TestEventBus_UnsubscribeNonExistent(t *testing.T) {
	bus := NewEventBus()

	handler := func(event Event) error {
		return nil
	}

	err := bus.Unsubscribe(EventMessageReceived, handler)
	if err != nil {
		t.Fatalf("unexpected error unsubscribing from non-existent type: %v", err)
	}
}

func TestEventBus_PublishWithMultipleErrors(t *testing.T) {
	bus := NewEventBus()

	err1 := errors.New("handler 1 error")
	err2 := errors.New("handler 2 error")

	bus.Subscribe(EventMessageReceived, func(event Event) error {
		return err1
	})
	bus.Subscribe(EventMessageReceived, func(event Event) error {
		return err2
	})

	event := Event{
		Type:      EventMessageReceived,
		Source:    "test",
		Timestamp: time.Now(),
	}

	err := bus.Publish(event)
	if err == nil {
		t.Fatal("expected error from publish")
	}

	errStr := err.Error()
	if !strings.Contains(errStr, "handler 1 error") || !strings.Contains(errStr, "handler 2 error") {
		t.Errorf("expected error containing both handler errors, got: %s", errStr)
	}
}

func TestEventBus_ConcurrentPublish(t *testing.T) {
	bus := NewEventBus()

	var mu sync.Mutex
	count := 0

	bus.Subscribe(EventMessageReceived, func(event Event) error {
		mu.Lock()
		count++
		mu.Unlock()
		return nil
	})

	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			event := Event{
				Type:      EventMessageReceived,
				Source:    "test",
				Timestamp: time.Now(),
			}
			bus.Publish(event)
		}()
	}

	wg.Wait()
	time.Sleep(100 * time.Millisecond)

	mu.Lock()
	if count != 100 {
		t.Errorf("expected 100 events received, got %d", count)
	}
	mu.Unlock()
}
