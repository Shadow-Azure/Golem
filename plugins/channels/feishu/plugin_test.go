package feishu

import (
	"testing"
)

func TestFeishuPlugin_Name(t *testing.T) {
	plugin := NewFeishuPlugin(FeishuConfig{})
	if plugin.Name() != "feishu" {
		t.Errorf("expected 'feishu', got '%s'", plugin.Name())
	}
}

func TestFeishuPlugin_Version(t *testing.T) {
	plugin := NewFeishuPlugin(FeishuConfig{})
	if plugin.Version() != "1.0.0" {
		t.Errorf("expected '1.0.0', got '%s'", plugin.Version())
	}
}

func TestFeishuPlugin_GetChannelType(t *testing.T) {
	plugin := NewFeishuPlugin(FeishuConfig{})
	if plugin.GetChannelType() != "feishu" {
		t.Errorf("expected 'feishu', got '%s'", plugin.GetChannelType())
	}
}

func TestFeishuPlugin_Initialize(t *testing.T) {
	plugin := NewFeishuPlugin(FeishuConfig{
		AppID:     "test-app-id",
		AppSecret: "test-secret",
	})

	err := plugin.Initialize(nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestFeishuPlugin_StartStop(t *testing.T) {
	plugin := NewFeishuPlugin(FeishuConfig{})
	plugin.Initialize(nil)

	err := plugin.Start()
	if err != nil {
		t.Fatalf("unexpected error starting: %v", err)
	}

	health := plugin.HealthCheck()
	if !health.Healthy {
		t.Error("plugin should be healthy after start")
	}

	err = plugin.Stop()
	if err != nil {
		t.Fatalf("unexpected error stopping: %v", err)
	}

	health = plugin.HealthCheck()
	if health.Healthy {
		t.Error("plugin should not be healthy after stop")
	}
}

func TestFeishuPlugin_StartAlreadyStarted(t *testing.T) {
	plugin := NewFeishuPlugin(FeishuConfig{})
	plugin.Initialize(nil)

	err := plugin.Start()
	if err != nil {
		t.Fatalf("unexpected error starting: %v", err)
	}

	err = plugin.Start()
	if err == nil {
		t.Error("expected error when starting already started plugin")
	}
}

func TestFeishuPlugin_SendMessage(t *testing.T) {
	plugin := NewFeishuPlugin(FeishuConfig{})
	plugin.Initialize(nil)
	plugin.Start()

	err := plugin.SendMessage("test-session", "Hello")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestFeishuPlugin_Deduplication(t *testing.T) {
	plugin := NewFeishuPlugin(FeishuConfig{})

	if plugin.dedup.IsDuplicate("msg1") {
		t.Error("first message should not be duplicate")
	}

	if !plugin.dedup.IsDuplicate("msg1") {
		t.Error("second message should be duplicate")
	}
}
