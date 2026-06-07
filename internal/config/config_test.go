package config

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"
)

// projectRoot returns the absolute path to the project root directory.
func projectRoot(t *testing.T) string {
	t.Helper()
	// Navigate from the test file to the project root: ../../
	_, filename, _, _ := runtime.Caller(0)
	return filepath.Join(filepath.Dir(filename), "..", "..")
}

func TestLoadConfig_FromFile(t *testing.T) {
	configPath := filepath.Join(projectRoot(t), "configs", "test.yaml")
	config, err := Load(configPath)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if config.Server.Host != "localhost" {
		t.Errorf("expected host 'localhost', got '%s'", config.Server.Host)
	}
	if config.Server.Port != 8080 {
		t.Errorf("expected port 8080, got %d", config.Server.Port)
	}
	if config.LLM.DefaultProvider != "openai" {
		t.Errorf("expected default provider 'openai', got '%s'", config.LLM.DefaultProvider)
	}

	provider := config.LLM.Providers["openai"]
	if provider.APIKey != "test-key" {
		t.Errorf("expected API key 'test-key', got '%s'", provider.APIKey)
	}
	if provider.BaseURL != "https://api.openai.com/v1" {
		t.Errorf("expected base_url 'https://api.openai.com/v1', got '%s'", provider.BaseURL)
	}
	if provider.Model != "gpt-4" {
		t.Errorf("expected model 'gpt-4', got '%s'", provider.Model)
	}
	if provider.Temperature != 0.5 {
		t.Errorf("expected temperature 0.5, got %f", provider.Temperature)
	}
	if provider.MaxTokens != 2048 {
		t.Errorf("expected max_tokens 2048, got %d", provider.MaxTokens)
	}

	if config.Session.MaxHistory != 100 {
		t.Errorf("expected max_history 100, got %d", config.Session.MaxHistory)
	}
	if config.Session.TrimTo != 50 {
		t.Errorf("expected trim_to 50, got %d", config.Session.TrimTo)
	}
	if config.Session.IdleTimeout != 60*time.Minute {
		t.Errorf("expected idle_timeout 60m, got %v", config.Session.IdleTimeout)
	}
	if config.Session.CleanupInterval != 10*time.Minute {
		t.Errorf("expected cleanup_interval 10m, got %v", config.Session.CleanupInterval)
	}

	feishu := config.Plugins.Channels["feishu"]
	if feishu["app_id"] != "test-app-id" {
		t.Errorf("expected feishu app_id 'test-app-id', got '%v'", feishu["app_id"])
	}

	if config.Logging.Level != "debug" {
		t.Errorf("expected logging level 'debug', got '%s'", config.Logging.Level)
	}
	if config.Logging.Format != "text" {
		t.Errorf("expected logging format 'text', got '%s'", config.Logging.Format)
	}
}

func TestLoadConfig_WithEnvVars(t *testing.T) {
	os.Setenv("TEST_API_KEY", "env-api-key")
	defer os.Unsetenv("TEST_API_KEY")

	configPath := filepath.Join(projectRoot(t), "configs", "test_env.yaml")
	config, err := Load(configPath)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	provider := config.LLM.Providers["openai"]
	if provider.APIKey != "env-api-key" {
		t.Errorf("expected API key from env var, got '%s'", provider.APIKey)
	}
}

func TestLoadConfig_Defaults(t *testing.T) {
	config := DefaultConfig()

	if config.Server.Host != "0.0.0.0" {
		t.Errorf("expected default host '0.0.0.0', got '%s'", config.Server.Host)
	}
	if config.Server.Port != 9921 {
		t.Errorf("expected default port 9921, got %d", config.Server.Port)
	}
	if config.Session.MaxHistory != 50 {
		t.Errorf("expected default max_history 50, got %d", config.Session.MaxHistory)
	}
	if config.Session.TrimTo != 20 {
		t.Errorf("expected default trim_to 20, got %d", config.Session.TrimTo)
	}
	if config.Session.IdleTimeout != 30*time.Minute {
		t.Errorf("expected default idle_timeout 30m, got %v", config.Session.IdleTimeout)
	}
	if config.Session.CleanupInterval != 5*time.Minute {
		t.Errorf("expected default cleanup_interval 5m, got %v", config.Session.CleanupInterval)
	}
	if config.Logging.Level != "info" {
		t.Errorf("expected default logging level 'info', got '%s'", config.Logging.Level)
	}
	if config.Logging.Format != "json" {
		t.Errorf("expected default logging format 'json', got '%s'", config.Logging.Format)
	}
}

func TestLoadConfig_NotFound(t *testing.T) {
	_, err := Load("nonexistent.yaml")
	if err == nil {
		t.Fatal("expected error for nonexistent config file")
	}
}
