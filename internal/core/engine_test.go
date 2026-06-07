package core

import (
	"testing"

	"github.com/Shadow-Azure/Golem/internal/config"
)

func TestEngine_Initialize(t *testing.T) {
	cfg := config.DefaultConfig()

	engine, err := NewEngine(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if engine.GetSessionManager() == nil {
		t.Error("session manager should not be nil")
	}
	if engine.GetEventBus() == nil {
		t.Error("event bus should not be nil")
	}
}

func TestEngine_NilConfig(t *testing.T) {
	_, err := NewEngine(nil)
	if err == nil {
		t.Fatal("expected error when config is nil")
	}
}

func TestEngine_StartStop(t *testing.T) {
	cfg := config.DefaultConfig()

	engine, err := NewEngine(cfg)
	if err != nil {
		t.Fatalf("unexpected error creating engine: %v", err)
	}

	err = engine.Start()
	if err != nil {
		t.Fatalf("unexpected error starting engine: %v", err)
	}

	err = engine.Shutdown()
	if err != nil {
		t.Fatalf("unexpected error shutting down engine: %v", err)
	}
}

func TestEngine_GetConfig(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Server.Port = 12345

	engine, err := NewEngine(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if engine.GetConfig().Server.Port != 12345 {
		t.Errorf("expected port 12345, got %d", engine.GetConfig().Server.Port)
	}
}

func TestEngine_DoubleStart(t *testing.T) {
	cfg := config.DefaultConfig()
	engine, err := NewEngine(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if err := engine.Start(); err != nil {
		t.Fatalf("unexpected error on first start: %v", err)
	}

	err = engine.Start()
	if err == nil {
		t.Fatal("expected error on double start")
	}
}

func TestEngine_ShutdownWithoutStart(t *testing.T) {
	cfg := config.DefaultConfig()
	engine, err := NewEngine(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	err = engine.Shutdown()
	if err != nil {
		t.Fatalf("shutdown before start should not error, got: %v", err)
	}
}

func TestEngine_GetPluginManager_Nil(t *testing.T) {
	cfg := config.DefaultConfig()
	engine, err := NewEngine(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Before SetPluginManager, should be nil.
	pm := engine.GetPluginManager()
	if pm != nil {
		t.Error("plugin manager should be nil before SetPluginManager")
	}
}

func TestEngine_SetPluginManager(t *testing.T) {
	cfg := config.DefaultConfig()
	engine, err := NewEngine(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Create a dummy plugin manager that satisfies the anonymous interface.
	dummy := &dummyPluginManager{plugins: map[string]interface{}{"test": true}}
	engine.SetPluginManager(dummy)

	got := engine.GetPluginManager()
	if got == nil {
		t.Error("plugin manager should not be nil after SetPluginManager")
	}

	// Verify it works.
	p, ok := got.GetPlugin("test")
	if !ok || p != true {
		t.Error("expected to retrieve 'test' plugin from plugin manager")
	}
}

// dummyPluginManager satisfies the EngineInterface's anonymous plugin manager interface for testing.
type dummyPluginManager struct {
	plugins map[string]interface{}
}

func (d *dummyPluginManager) GetPlugin(name string) (interface{}, bool) {
	p, ok := d.plugins[name]
	return p, ok
}

func TestEngine_SessionManagerReturnsInterface(t *testing.T) {
	cfg := config.DefaultConfig()
	engine, err := NewEngine(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	sm := engine.GetSessionManager()
	session, err := sm.CreateSession("user1", "cli")
	if err != nil {
		t.Fatalf("unexpected error creating session: %v", err)
	}
	if session.UserID != "user1" {
		t.Errorf("expected user_id 'user1', got '%s'", session.UserID)
	}
}
