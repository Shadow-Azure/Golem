package tests

import (
	"testing"
	"time"

	"github.com/Shadow-Azure/Golem/internal/config"
	"github.com/Shadow-Azure/Golem/internal/core"
)

func TestIntegration_EngineLifecycle(t *testing.T) {
	cfg := config.DefaultConfig()

	engine, err := core.NewEngine(cfg)
	if err != nil {
		t.Fatalf("failed to create engine: %v", err)
	}

	if err := engine.Start(); err != nil {
		t.Fatalf("failed to start engine: %v", err)
	}

	if engine.GetSessionManager() == nil {
		t.Error("session manager should not be nil")
	}
	if engine.GetEventBus() == nil {
		t.Error("event bus should not be nil")
	}

	if err := engine.Shutdown(); err != nil {
		t.Fatalf("failed to shutdown engine: %v", err)
	}
}

func TestIntegration_SessionFlow(t *testing.T) {
	cfg := config.DefaultConfig()
	engine, _ := core.NewEngine(cfg)
	engine.Start()
	defer engine.Shutdown()

	sm := engine.GetSessionManager()

	session, err := sm.CreateSession("user123", "test")
	if err != nil {
		t.Fatalf("failed to create session: %v", err)
	}

	sm.AddMessage(session.ID, core.Message{
		Role:    "user",
		Content: "Hello",
	})

	sm.AddMessage(session.ID, core.Message{
		Role:    "assistant",
		Content: "Hi there!",
	})

	history, err := sm.GetHistory(session.ID, 10)
	if err != nil {
		t.Fatalf("failed to get history: %v", err)
	}

	if len(history) != 2 {
		t.Errorf("expected 2 messages, got %d", len(history))
	}
}

func TestIntegration_EventFlow(t *testing.T) {
	cfg := config.DefaultConfig()
	engine, _ := core.NewEngine(cfg)
	engine.Start()
	defer engine.Shutdown()

	eb := engine.GetEventBus()

	received := make(chan core.Event, 1)
	eb.Subscribe(core.EventMessageReceived, func(event core.Event) error {
		received <- event
		return nil
	})

	eb.Publish(core.Event{
		Type:   core.EventMessageReceived,
		Source: "test",
		Data:   "test data",
	})

	select {
	case event := <-received:
		if event.Data != "test data" {
			t.Errorf("expected data 'test data', got %v", event.Data)
		}
	case <-time.After(1 * time.Second):
		t.Error("timed out waiting for event to be received")
	}
}
