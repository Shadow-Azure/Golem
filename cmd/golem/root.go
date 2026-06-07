package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var cfgFile string

var rootCmd = &cobra.Command{
	Use:   "golem",
	Short: "Golem AI Agent - local AI assistant framework",
	Long: `Golem is a task-oriented AI assistant framework with Feishu bot integration.
It can run as a foreground process or as a background service.`,
	Run: func(cmd *cobra.Command, args []string) {
		// Default behavior: run in foreground
		runForeground()
	},
}

// Execute runs the root command.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func init() {
	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file path (default: ~/.golem/golem.yaml)")

	// Add all subcommands
	rootCmd.AddCommand(startCmd)
	rootCmd.AddCommand(stopCmd)
	rootCmd.AddCommand(restartCmd)
	rootCmd.AddCommand(statusCmd)
	rootCmd.AddCommand(onboardCmd)
	rootCmd.AddCommand(configureCmd)
	rootCmd.AddCommand(versionCmd)
}
