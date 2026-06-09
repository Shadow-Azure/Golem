package core

import (
	"fmt"
	"log/slog"
	"sync"
	"time"
)

// EventType represents the type of event.
type EventType string

const (
	EventMessageReceived EventType = "message.received"
	EventMessageSent     EventType = "message.sent"
	EventStreamDelta     EventType = "stream.delta"
	EventStreamDone      EventType = "stream.done"
	EventStreamError     EventType = "stream.error"
	EventSessionCreated  EventType = "session.created"
	EventSessionDeleted  EventType = "session.deleted"
	EventPluginLoaded    EventType = "plugin.loaded"
	EventPluginUnloaded  EventType = "plugin.unloaded"
)

// Event represents an event in the system.
type Event struct {
	Type      EventType              `json:"type"`
	Source    string                 `json:"source"`
	Data      interface{}            `json:"data,omitempty"`
	Timestamp time.Time              `json:"timestamp"`
	Metadata  map[string]interface{} `json:"metadata,omitempty"`
}

// EventHandler is a function that handles events.
type EventHandler func(event Event) error

// EventBusInterface defines the interface for the event bus.
type EventBusInterface interface {
	Publish(event Event) error
	Subscribe(eventType EventType, handler EventHandler) error
	Unsubscribe(eventType EventType, handler EventHandler) error
}

// EventBus implements a pub/sub event system.
type EventBus struct {
	handlers map[EventType][]EventHandler
	mu       sync.RWMutex
	logger   *slog.Logger
}

// NewEventBus creates a new EventBus.
func NewEventBus() *EventBus {
	return &EventBus{
		handlers: make(map[EventType][]EventHandler),
		logger:   slog.Default().With("component", "eventbus"),
	}
}

// Publish publishes an event to all subscribers.
// If any handler returns an error, the error is logged and collected.
// Returns an aggregate error if any handler failed.
func (eb *EventBus) Publish(event Event) error {
	eb.mu.RLock()
	handlers, exists := eb.handlers[event.Type]
	eb.mu.RUnlock()

	if !exists {
		return nil
	}

	var errs []error
	for _, handler := range handlers {
		if err := handler(event); err != nil {
			errs = append(errs, err)
			eb.logger.Error("event handler error",
				"event_type", event.Type,
				"source", event.Source,
				"error", err,
			)
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("event handler errors: %v", errs)
	}

	return nil
}

// Subscribe subscribes a handler to an event type.
func (eb *EventBus) Subscribe(eventType EventType, handler EventHandler) error {
	eb.mu.Lock()
	defer eb.mu.Unlock()

	eb.handlers[eventType] = append(eb.handlers[eventType], handler)
	eb.logger.Debug("subscribed to event", "event_type", eventType)

	return nil
}

// Unsubscribe removes all handlers for an event type.
func (eb *EventBus) Unsubscribe(eventType EventType, handler EventHandler) error {
	eb.mu.Lock()
	defer eb.mu.Unlock()

	_, exists := eb.handlers[eventType]
	if !exists {
		return nil
	}

	// Clear all handlers for this event type.
	eb.handlers[eventType] = eb.handlers[eventType][:0]
	eb.logger.Debug("unsubscribed from event", "event_type", eventType)

	return nil
}

// Ensure EventBus implements EventBusInterface.
var _ EventBusInterface = (*EventBus)(nil)
