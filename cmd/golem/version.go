package main

import (
	"fmt"

	"github.com/spf13/cobra"
)

// Version is set at build time via -ldflags.
var Version = "dev"

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print the version of Golem AI Agent",
	Long:  "Display version information for Golem AI Agent.",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Printf("Golem AI Agent v%s\n", Version)
	},
}
