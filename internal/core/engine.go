package core

import (
	"context"
	"fmt"
	"log/slog"
	"sync"

	"github.com/Shadow-Azure/Golem/internal/config"
)

// EngineInterface defines the interface for the central engine orchestrator.
type EngineInterface interface {
	Start() error
	Shutdown() error
	GetSessionManager() SessionManagerInterface
	GetEventBus() EventBusInterface
	GetPluginManager() interface{ GetPlugin(name string) (interface{}, bool) }
	GetConfig() *config.Config
}

// Engine is the central orchestrator that wires together session management,
// event publishing, and configuration.
type Engine struct {
	config     *config.Config
	sessionMgr *SessionManager
	eventBus   *EventBus
	logger     *slog.Logger
	ctx        context.Context
	cancel     context.CancelFunc
	mu         sync.RWMutex
	started    bool
}

// NewEngine creates a new Engine from the provided configuration.
// Returns an error if cfg is nil.
func NewEngine(cfg *config.Config) (*Engine, error) {
	if cfg == nil {
		return nil, fmt.Errorf("config cannot be nil")
	}

	ctx, cancel := context.WithCancel(context.Background())

	logger := slog.Default().With("component", "engine")

	// Convert config.SessionConfig to core.SessionConfig (same fields, distinct types).
	sessionCfg := SessionConfig{
		MaxHistory:      cfg.Session.MaxHistory,
		TrimTo:          cfg.Session.TrimTo,
		IdleTimeout:     cfg.Session.IdleTimeout,
		CleanupInterval: cfg.Session.CleanupInterval,
	}

	engine := &Engine{
		config:     cfg,
		sessionMgr: NewSessionManager(sessionCfg),
		eventBus:   NewEventBus(),
		logger:     logger,
		ctx:        ctx,
		cancel:     cancel,
	}

	return engine, nil
}

// Start starts the engine. Returns an error if already started.
func (e *Engine) Start() error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if e.started {
		return fmt.Errorf("engine already started")
	}

	e.logger.Info("starting engine",
		"host", e.config.Server.Host,
		"port", e.config.Server.Port,
	)

	e.started = true
	return nil
}

// Shutdown gracefully shuts down the engine.
func (e *Engine) Shutdown() error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if !e.started {
		return nil
	}

	e.logger.Info("shutting down engine")

	e.cancel()
	e.started = false

	return nil
}

// GetSessionManager returns the session manager.
func (e *Engine) GetSessionManager() SessionManagerInterface {
	return e.sessionMgr
}

// GetEventBus returns the event bus.
func (e *Engine) GetEventBus() EventBusInterface {
	return e.eventBus
}

// GetPluginManager returns the plugin manager (stub).
func (e *Engine) GetPluginManager() interface{ GetPlugin(name string) (interface{}, bool) } {
	return nil
}

// GetConfig returns the engine configuration.
func (e *Engine) GetConfig() *config.Config {
	return e.config
}

// Ensure Engine implements EngineInterface.
var _ EngineInterface = (*Engine)(nil)
