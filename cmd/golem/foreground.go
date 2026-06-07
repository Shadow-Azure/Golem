package main

import (
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"github.com/Shadow-Azure/Golem/internal/config"
	"github.com/Shadow-Azure/Golem/internal/core"
	"github.com/Shadow-Azure/Golem/internal/plugin"
	feishuPlugin "github.com/Shadow-Azure/Golem/plugins/channels/feishu"
	claudePlugin "github.com/Shadow-Azure/Golem/plugins/providers/claude"
	openaiPlugin "github.com/Shadow-Azure/Golem/plugins/providers/openai"
)

func runForeground() {
	// Setup logger
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))
	slog.SetDefault(logger)

	logger.Info("starting Golem AI Agent", "version", Version)

	// Determine config path
	configPath := cfgFile
	if configPath == "" {
		homeDir, _ := os.UserHomeDir()
		configPath = filepath.Join(homeDir, ".golem", "golem.yaml")
	}

	// Load configuration
	cfg, err := config.Load(configPath)
	if err != nil {
		logger.Error("failed to load configuration", "error", err)
		os.Exit(1)
	}

	// Create engine
	engine, err := core.NewEngine(cfg)
	if err != nil {
		logger.Error("failed to create engine", "error", err)
		os.Exit(1)
	}

	// Create plugin manager
	pm := plugin.NewManager()

	// Register OpenAI provider
	if openaiConfig, ok := cfg.LLM.Providers["openai"]; ok {
		openaiProv := openaiPlugin.NewProvider(openaiPlugin.ProviderConfig{
			APIKey:      openaiConfig.APIKey,
			BaseURL:     openaiConfig.BaseURL,
			Model:       openaiConfig.Model,
			Temperature: openaiConfig.Temperature,
			MaxTokens:   openaiConfig.MaxTokens,
		})
		if err := pm.LoadPlugin("openai", openaiProv); err != nil {
			logger.Error("failed to load OpenAI provider", "error", err)
		}
	}

	// Register Claude provider
	if claudeConfig, ok := cfg.LLM.Providers["claude"]; ok {
		claudeProv := claudePlugin.NewProvider(claudePlugin.ProviderConfig{
			APIKey:    claudeConfig.APIKey,
			BaseURL:   claudeConfig.BaseURL,
			Model:     claudeConfig.Model,
			MaxTokens: claudeConfig.MaxTokens,
		})
		if err := pm.LoadPlugin("claude", claudeProv); err != nil {
			logger.Error("failed to load Claude provider", "error", err)
		}
	}

	// Register Feishu channel if configured
	if feishuCfg, ok := cfg.Plugins.Channels["feishu"]; ok {
		feishu := feishuPlugin.NewFeishuPlugin(feishuPlugin.FeishuConfig{
			AppID:             fmt.Sprintf("%v", feishuCfg["app_id"]),
			AppSecret:         fmt.Sprintf("%v", feishuCfg["app_secret"]),
			VerificationToken: fmt.Sprintf("%v", feishuCfg["verification_token"]),
			EncryptKey:        fmt.Sprintf("%v", feishuCfg["encrypt_key"]),
		})
		feishu.SetEngine(engine)
		if err := pm.LoadPlugin("feishu", feishu); err != nil {
			logger.Error("failed to load Feishu plugin", "error", err)
		}
	}

	// Start engine
	if err := engine.Start(); err != nil {
		logger.Error("failed to start engine", "error", err)
		os.Exit(1)
	}

	// Start all plugins
	if err := pm.StartAll(); err != nil {
		logger.Error("failed to start plugins", "error", err)
		os.Exit(1)
	}

	logger.Info("Golem AI Agent started successfully",
		"host", cfg.Server.Host,
		"port", cfg.Server.Port,
	)

	// Wait for shutdown signal
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	<-sigChan

	logger.Info("shutting down Golem AI Agent")

	// Stop all plugins
	if err := pm.StopAll(); err != nil {
		logger.Error("error stopping plugins", "error", err)
	}

	// Shutdown engine
	if err := engine.Shutdown(); err != nil {
		logger.Error("error shutting down engine", "error", err)
	}

	logger.Info("Golem AI Agent stopped")
	fmt.Println("Golem AI Agent stopped")
}
