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

	logger.Info("Starting Golem AI Agent v" + Version)

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

	// Register LLM providers
	var defaultProvider plugin.ProviderPlugin

	for providerName, providerConfig := range cfg.LLM.Providers {
		switch providerName {
		case "openai", "minimax": // MiniMax uses OpenAI-compatible API
			provider := openaiPlugin.NewProvider(openaiPlugin.ProviderConfig{
				APIKey:      providerConfig.APIKey,
				BaseURL:     providerConfig.BaseURL,
				Model:       providerConfig.Model,
				Temperature: providerConfig.Temperature,
				MaxTokens:   providerConfig.MaxTokens,
			})
			if err := pm.LoadPlugin(providerName, provider); err != nil {
				logger.Error("failed to load provider", "provider", providerName, "error", err)
			} else {
				logger.Info("loaded provider", "provider", providerName)
				// Set as default if this is the configured default provider
				if providerName == cfg.LLM.DefaultProvider {
					defaultProvider = provider
				}
			}

		case "claude":
			provider := claudePlugin.NewProvider(claudePlugin.ProviderConfig{
				APIKey:    providerConfig.APIKey,
				BaseURL:   providerConfig.BaseURL,
				Model:     providerConfig.Model,
				MaxTokens: providerConfig.MaxTokens,
			})
			if err := pm.LoadPlugin("claude", provider); err != nil {
				logger.Error("failed to load Claude provider", "error", err)
			} else {
				logger.Info("loaded provider", "provider", "claude")
				// Set as default if this is the configured default provider
				if providerName == cfg.LLM.DefaultProvider {
					defaultProvider = provider
				}
			}

		default:
			logger.Warn("unknown provider, skipping", "provider", providerName)
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

		// Set the default provider for Feishu
		if defaultProvider != nil {
			feishu.SetProvider(defaultProvider)
			logger.Info("set default provider for Feishu", "provider", cfg.LLM.DefaultProvider)
		} else {
			logger.Warn("no default provider available for Feishu")
		}

		if err := pm.LoadPlugin("feishu", feishu); err != nil {
			logger.Error("failed to load Feishu plugin", "error", err)
		} else {
			logger.Info("Feishu bot enabled")
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
	logger.Info("Ready to receive messages")

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
