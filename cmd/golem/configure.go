package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/AlecAivazis/survey/v2"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

var configureCmd = &cobra.Command{
	Use:   "configure [llm|feishu]",
	Short: "Update Golem configuration interactively",
	Long:  "Interactively update Golem AI Agent configuration for LLM provider or Feishu bot.",
	Args:  cobra.MaximumNArgs(1),
	Run:   runConfigure,
}

func runConfigure(cmd *cobra.Command, args []string) {
	var section string
	if len(args) > 0 {
		section = args[0]
	} else {
		prompt := &survey.Select{
			Message: "Select section to configure:",
			Options: []string{"llm", "feishu"},
		}
		survey.AskOne(prompt, &section)
	}

	homeDir, _ := os.UserHomeDir()
	configPath := filepath.Join(homeDir, ".golem", "golem.yaml")

	var config map[string]interface{}
	data, err := os.ReadFile(configPath)
	if err != nil {
		config = make(map[string]interface{})
	} else {
		yaml.Unmarshal(data, &config)
	}

	switch section {
	case "llm":
		llmConfig := configureLLMWizard("OpenAI")
		config["llm"] = llmConfig
	case "feishu":
		feishuConfig := configureFeishuWizard()
		config["feishu"] = feishuConfig
	default:
		fmt.Printf("Unknown config section: %s\n", section)
		fmt.Println("Valid sections: llm, feishu")
		os.Exit(1)
	}

	if err := saveConfig(config); err != nil {
		fmt.Printf("Failed to save config: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("Configuration updated.")
	fmt.Println("Run 'golem restart' to apply changes.")
}
