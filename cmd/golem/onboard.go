package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/AlecAivazis/survey/v2"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

var onboardCmd = &cobra.Command{
	Use:   "onboard",
	Short: "Interactive setup wizard for first-time users",
	Long:  "Interactive wizard to configure Golem AI Agent, including LLM provider and Feishu bot integration.",
	Run:   runOnboard,
}

func runOnboard(cmd *cobra.Command, args []string) {
	fmt.Println("Golem AI Agent - Setup Wizard")
	fmt.Println("Let's configure your AI assistant.")
	fmt.Println()

	config := make(map[string]interface{})

	// Step 1: Select LLM provider
	var provider string
	prompt := &survey.Select{
		Message: "Select LLM provider:",
		Options: []string{"OpenAI", "Claude", "Skip"},
		Default: "OpenAI",
	}
	survey.AskOne(prompt, &provider)

	// Step 2: Configure LLM
	if provider != "Skip" {
		llmConfig := configureLLMWizard(provider)
		config["llm"] = llmConfig
	}

	// Step 3: Configure Feishu
	var enableFeishu bool
	prompt2 := &survey.Confirm{
		Message: "Enable Feishu bot?",
		Default: false,
	}
	survey.AskOne(prompt2, &enableFeishu)

	if enableFeishu {
		feishuConfig := configureFeishuWizard()
		config["feishu"] = feishuConfig
	}

	// Step 4: Save config
	if err := saveConfig(config); err != nil {
		fmt.Printf("Failed to save config: %v\n", err)
		os.Exit(1)
	}

	fmt.Println()
	fmt.Println("Setup complete!")
	fmt.Println()
	fmt.Println("Next steps:")
	fmt.Println("  1. Run 'golem start' to start the background service")
	fmt.Println("  2. Or run 'golem' to run in foreground")
}

func configureLLMWizard(provider string) map[string]interface{} {
	llmConfig := make(map[string]interface{})

	if provider == "OpenAI" {
		llmConfig["default_provider"] = "openai"

		var apiKey string
		prompt := &survey.Password{
			Message: "Enter OpenAI API Key:",
		}
		survey.AskOne(prompt, &apiKey)

		var model string
		prompt2 := &survey.Select{
			Message: "Select model:",
			Options: []string{"gpt-4o", "gpt-4", "gpt-3.5-turbo"},
			Default: "gpt-4o",
		}
		survey.AskOne(prompt2, &model)

		llmConfig["providers"] = map[string]interface{}{
			"openai": map[string]interface{}{
				"api_key": apiKey,
				"model":   model,
			},
		}
	} else {
		llmConfig["default_provider"] = "claude"

		var apiKey string
		prompt := &survey.Password{
			Message: "Enter Anthropic API Key:",
		}
		survey.AskOne(prompt, &apiKey)

		var model string
		prompt2 := &survey.Select{
			Message: "Select model:",
			Options: []string{"claude-3-opus-20240229", "claude-3-sonnet-20240229", "claude-3-haiku-20240229"},
			Default: "claude-3-opus-20240229",
		}
		survey.AskOne(prompt2, &model)

		llmConfig["providers"] = map[string]interface{}{
			"claude": map[string]interface{}{
				"api_key": apiKey,
				"model":   model,
			},
		}
	}

	return llmConfig
}

func configureFeishuWizard() map[string]interface{} {
	var appId, appSecret, verificationToken string

	prompt1 := &survey.Input{
		Message: "Enter Feishu App ID:",
	}
	survey.AskOne(prompt1, &appId)

	prompt2 := &survey.Password{
		Message: "Enter Feishu App Secret:",
	}
	survey.AskOne(prompt2, &appSecret)

	prompt3 := &survey.Password{
		Message: "Enter Feishu Verification Token:",
	}
	survey.AskOne(prompt3, &verificationToken)

	return map[string]interface{}{
		"enabled":            true,
		"app_id":             appId,
		"app_secret":         appSecret,
		"verification_token": verificationToken,
	}
}

func saveConfig(config map[string]interface{}) error {
	homeDir, _ := os.UserHomeDir()
	configDir := filepath.Join(homeDir, ".golem")

	if err := os.MkdirAll(configDir, 0755); err != nil {
		return err
	}

	// Add server config
	config["server"] = map[string]interface{}{
		"host": "127.0.0.1",
		"port": 9921,
	}

	// Add session config
	config["session"] = map[string]interface{}{
		"max_history": 50,
		"trim_to":     20,
	}

	// Add logging config
	config["logging"] = map[string]interface{}{
		"level":  "info",
		"format": "json",
	}

	data, err := yaml.Marshal(config)
	if err != nil {
		return err
	}

	configPath := filepath.Join(configDir, "golem.yaml")
	return os.WriteFile(configPath, data, 0644)
}
