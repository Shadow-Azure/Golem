package feishu

import (
	"context"
	"testing"

	"github.com/Shadow-Azure/Golem/internal/plugin"
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
	plugin := NewFeishuPlugin(FeishuConfig{
		AppID:     "test-app-id",
		AppSecret: "test-app-secret",
	})
	plugin.Initialize(nil)
	plugin.Start()

	// Use a valid session ID format
	err := plugin.SendMessage("feishu:ou_test123", "Hello")
	// This will fail because we don't have a real Lark client,
	// but it should not panic
	if err != nil {
		t.Logf("SendMessage returned error (expected without real client): %v", err)
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

func TestFeishuPlugin_ImplementsTypingCapable(t *testing.T) {
	p := NewFeishuPlugin(FeishuConfig{})
	var _ plugin.TypingCapable = p
}

func TestFeishuPlugin_ImplementsStreamingCapable(t *testing.T) {
	p := NewFeishuPlugin(FeishuConfig{})
	var _ plugin.StreamingCapable = p
}

func TestFeishuPlugin_StartTypingNoClient(t *testing.T) {
	p := NewFeishuPlugin(FeishuConfig{})
	// Should not panic even without a real Lark client
	err := p.StartTyping(context.Background(), "feishu:ou_test", "msg1")
	// Error expected because no real client, but should not panic
	if err != nil {
		t.Logf("StartTyping returned error (expected without real client): %v", err)
	}
}

func TestFeishuPlugin_StopTypingNoClient(t *testing.T) {
	p := NewFeishuPlugin(FeishuConfig{})
	err := p.StopTyping(context.Background(), "feishu:ou_test")
	if err != nil {
		t.Logf("StopTyping returned error (expected without real client): %v", err)
	}
}
