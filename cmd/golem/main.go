package main

import (
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/Shadow-Azure/Golem/internal/config"
	"github.com/Shadow-Azure/Golem/internal/core"
	"github.com/Shadow-Azure/Golem/internal/plugin"
	claudePlugin "github.com/Shadow-Azure/Golem/plugins/providers/claude"
	feishuPlugin "github.com/Shadow-Azure/Golem/plugins/channels/feishu"
	openaiPlugin "github.com/Shadow-Azure/Golem/plugins/providers/openai"
)

func main() {
	configPath := flag.String("config", "configs/golem.yaml", "Path to configuration file")
	flag.Parse()

	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))
	slog.SetDefault(logger)

	logger.Info("starting Golem AI Agent", "version", "1.0.0")

	cfg, err := config.Load(*configPath)
	if err != nil {
		logger.Error("failed to load configuration", "error", err)
		os.Exit(1)
	}

	engine, err := core.NewEngine(cfg)
	if err != nil {
		logger.Error("failed to create engine", "error", err)
		os.Exit(1)
	}

	pm := plugin.NewManager()
	engine.SetPluginManager(&pluginManagerAdapter{mgr: pm})

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

	// Register Feishu channel
	if feishuConfig, ok := cfg.Plugins.Channels["feishu"]; ok {
		feishu := feishuPlugin.NewFeishuPlugin(feishuPlugin.FeishuConfig{
			AppID:             fmt.Sprintf("%v", feishuConfig["app_id"]),
			AppSecret:         fmt.Sprintf("%v", feishuConfig["app_secret"]),
			VerificationToken: fmt.Sprintf("%v", feishuConfig["verification_token"]),
			EncryptKey:        fmt.Sprintf("%v", feishuConfig["encrypt_key"]),
		})
		feishu.SetEngine(engine)
		if err := pm.LoadPlugin("feishu", feishu); err != nil {
			logger.Error("failed to load Feishu plugin", "error", err)
		}
	}

	if err := engine.Start(); err != nil {
		logger.Error("failed to start engine", "error", err)
		os.Exit(1)
	}

	if err := pm.StartAll(); err != nil {
		logger.Error("failed to start plugins", "error", err)
		os.Exit(1)
	}

	logger.Info("Golem AI Agent started successfully",
		"host", cfg.Server.Host,
		"port", cfg.Server.Port,
	)

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	<-sigChan

	logger.Info("shutting down Golem AI Agent")

	if err := pm.StopAll(); err != nil {
		logger.Error("error stopping plugins", "error", err)
	}

	if err := engine.Shutdown(); err != nil {
		logger.Error("error shutting down engine", "error", err)
	}

	logger.Info("Golem AI Agent stopped")
	fmt.Println("Golem AI Agent stopped")
}

// pluginManagerAdapter wraps plugin.Manager to satisfy the Engine's
// anonymous interface for GetPlugin, bridging the return type difference
// between (plugin.Plugin, bool) and (interface{}, bool).
type pluginManagerAdapter struct {
	mgr *plugin.Manager
}

func (a *pluginManagerAdapter) GetPlugin(name string) (interface{}, bool) {
	return a.mgr.GetPlugin(name)
}
