// cmd/golem/chat.go
package main

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/Shadow-Azure/Golem/internal/config"
	"github.com/Shadow-Azure/Golem/internal/core"
	"github.com/Shadow-Azure/Golem/internal/plugin"
	claudePlugin "github.com/Shadow-Azure/Golem/plugins/providers/claude"
	openaiPlugin "github.com/Shadow-Azure/Golem/plugins/providers/openai"
	"github.com/spf13/cobra"
)

var chatProvider string

var chatCmd = &cobra.Command{
	Use:   "chat",
	Short: "启动终端聊天",
	Long:  "在终端中直接与 AI 助手对话。",
	Run:   runChat,
}

func init() {
	chatCmd.Flags().StringVar(&chatProvider, "provider", "", "LLM 提供商 (openai, claude, minimax)")
}

func runChat(cmd *cobra.Command, args []string) {
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
	providerName := cfg.LLM.DefaultProvider
	if chatProvider != "" {
		providerName = chatProvider
	}

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
		} else if providerName == "openai" {
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
		} else if providerName == "claude" {
			provider = claudeProv
		}
	}

	// MiniMax uses OpenAI-compatible API
	if minimaxConfig, ok := cfg.LLM.Providers["minimax"]; ok {
		minimaxProv := openaiPlugin.NewProvider(openaiPlugin.ProviderConfig{
			APIKey:      minimaxConfig.APIKey,
			BaseURL:     minimaxConfig.BaseURL,
			Model:       minimaxConfig.Model,
			Temperature: minimaxConfig.Temperature,
			MaxTokens:   minimaxConfig.MaxTokens,
		})
		if err := pm.LoadPlugin("minimax", minimaxProv); err != nil {
			fmt.Printf("加载 MiniMax 提供商失败: %v\n", err)
		} else if providerName == "minimax" {
			provider = minimaxProv
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

	session, err := engine.GetSessionManager().CreateSession("tui", "terminal")
	if err != nil {
		fmt.Printf("创建会话失败: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("Golem AI Chat")
	fmt.Printf("提供商: %s\n", providerName)
	fmt.Println("输入消息开始对话，输入 /help 查看命令")
	fmt.Println()

	reader := bufio.NewReader(os.Stdin)

	for {
		fmt.Print("You: ")
		input, err := reader.ReadString('\n')
		if err != nil {
			fmt.Println("\n再见！")
			break
		}
		input = strings.TrimSpace(input)

		if input == "" {
			continue
		}

		if strings.HasPrefix(input, "/") {
			if handleChatCommand(input, engine, session) {
				continue
			}
			break
		}

		if err := engine.GetSessionManager().AddMessage(session.ID, core.Message{
			Role:    "user",
			Content: input,
		}); err != nil {
			fmt.Printf("添加消息失败: %v\n", err)
			continue
		}

		history, err := engine.GetSessionManager().GetHistory(session.ID, 50)
		if err != nil {
			fmt.Printf("获取历史失败: %v\n", err)
			continue
		}

		fmt.Print("AI: ")
		fullResponse, err := streamResponse(cmd.Context(), provider, history)
		if err != nil {
			fmt.Printf("\n错误: %v\n", err)
			continue
		}

		fmt.Println()
		fmt.Println()

		if err := engine.GetSessionManager().AddMessage(session.ID, core.Message{
			Role:    "assistant",
			Content: fullResponse,
		}); err != nil {
			fmt.Printf("添加助手消息失败: %v\n", err)
		}
	}
}

func handleChatCommand(input string, engine *core.Engine, session *core.Session) bool {
	switch input {
	case "/help":
		fmt.Println("可用命令:")
		fmt.Println("  /help     - 显示帮助")
		fmt.Println("  /clear    - 清空历史")
		fmt.Println("  /history  - 显示历史")
		fmt.Println("  /quit     - 退出")
		return true
	case "/clear":
		if err := engine.GetSessionManager().DeleteSession(session.ID); err != nil {
			fmt.Printf("清空历史失败: %v\n", err)
		} else {
			fmt.Println("历史已清空")
		}
		return true
	case "/history":
		history, err := engine.GetSessionManager().GetHistory(session.ID, 20)
		if err != nil {
			fmt.Printf("获取历史失败: %v\n", err)
			return true
		}
		if len(history) == 0 {
			fmt.Println("暂无历史记录")
		} else {
			for _, msg := range history {
				role := "You"
				if msg.Role == "assistant" {
					role = "AI"
				}
				fmt.Printf("%s: %s\n", role, msg.Content)
			}
		}
		return true
	case "/quit":
		fmt.Println("再见！")
		return false
	default:
		fmt.Printf("未知命令: %s\n", input)
		return true
	}
}

// streamResponse streams the LLM response with typewriter effect
func streamResponse(ctx context.Context, provider plugin.ProviderPlugin, messages []core.Message) (string, error) {
	stream, err := provider.ChatStream(ctx, messages, core.ChatConfig{
		Stream: true,
	})
	if err != nil {
		return "", err
	}

	var fullResponse string
	for chunk := range stream {
		if chunk.Error != nil {
			return fullResponse, chunk.Error
		}
		if chunk.Done {
			break
		}

		// Character-by-character output
		for _, char := range chunk.Content {
			fmt.Print(string(char))
			time.Sleep(20 * time.Millisecond)
		}

		fullResponse += chunk.Content
	}

	return fullResponse, nil
}
