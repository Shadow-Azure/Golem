// cmd/golem/web.go
package main

import (
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"github.com/Shadow-Azure/Golem/internal/config"
	"github.com/Shadow-Azure/Golem/internal/core"
	"github.com/Shadow-Azure/Golem/internal/plugin"
	claudePlugin "github.com/Shadow-Azure/Golem/plugins/providers/claude"
	openaiPlugin "github.com/Shadow-Azure/Golem/plugins/providers/openai"
	"github.com/Shadow-Azure/Golem/web"
	"github.com/spf13/cobra"
)

var webPort int

var webCmd = &cobra.Command{
	Use:   "web",
	Short: "启动 WebChat 服务",
	Long:  "启动 Web 界面，在浏览器中与 AI 助手对话。",
	Run:   runWeb,
}

func init() {
	webCmd.Flags().IntVar(&webPort, "port", 8080, "WebChat 服务端口")
}

func runWeb(cmd *cobra.Command, args []string) {
	configPath := cfgFile
	if configPath == "" {
		homeDir, _ := os.UserHomeDir()
		configPath = filepath.Join(homeDir, ".golem", "golem.yaml")
	}

	cfg, err := config.Load(configPath)
	if err != nil {
		fmt.Printf("加载配置失败: %v\n", err)
		os.Exit(1)
	}

	engine, err := core.NewEngine(cfg)
	if err != nil {
		fmt.Printf("创建引擎失败: %v\n", err)
		os.Exit(1)
	}

	pm := plugin.NewManager()
	var provider plugin.ProviderPlugin

	if openaiConfig, ok := cfg.LLM.Providers["openai"]; ok {
		openaiProv := openaiPlugin.NewProvider(openaiPlugin.ProviderConfig{
			APIKey:      openaiConfig.APIKey,
			BaseURL:     openaiConfig.BaseURL,
			Model:       openaiConfig.Model,
			Temperature: openaiConfig.Temperature,
			MaxTokens:   openaiConfig.MaxTokens,
		})
		if err := pm.LoadPlugin("openai", openaiProv); err != nil {
			fmt.Printf("加载 OpenAI 提供商失败: %v\n", err)
		} else if cfg.LLM.DefaultProvider == "openai" {
			provider = openaiProv
		}
	}

	if claudeConfig, ok := cfg.LLM.Providers["claude"]; ok {
		claudeProv := claudePlugin.NewProvider(claudePlugin.ProviderConfig{
			APIKey:    claudeConfig.APIKey,
			BaseURL:   claudeConfig.BaseURL,
			Model:     claudeConfig.Model,
			MaxTokens: claudeConfig.MaxTokens,
		})
		if err := pm.LoadPlugin("claude", claudeProv); err != nil {
			fmt.Printf("加载 Claude 提供商失败: %v\n", err)
		} else if cfg.LLM.DefaultProvider == "claude" {
			provider = claudeProv
		}
	}

	if provider == nil {
		fmt.Println("未找到 LLM 提供商，请检查配置")
		os.Exit(1)
	}

	if err := engine.Start(); err != nil {
		fmt.Printf("启动引擎失败: %v\n", err)
		os.Exit(1)
	}

	_ = pm // plugin manager loaded but not started (providers don't need Start)

	addr := fmt.Sprintf(":%d", webPort)
	srv := web.NewServer(engine, provider, addr)

	fmt.Println("Golem WebChat 已启动")
	fmt.Printf("打开浏览器访问: http://localhost:%d\n", webPort)
	fmt.Println("按 Ctrl+C 停止")

	// Handle graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-sigChan
		fmt.Println("\n正在停止服务...")
		if err := engine.Shutdown(); err != nil {
			fmt.Printf("关闭引擎失败: %v\n", err)
		}
		os.Exit(0)
	}()

	if err := srv.Start(); err != nil {
		fmt.Printf("启动服务失败: %v\n", err)
		os.Exit(1)
	}
}
