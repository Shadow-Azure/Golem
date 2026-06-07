package main

import (
	"fmt"

	"github.com/spf13/cobra"
)

const version = "0.1.0"

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print the version of Golem AI Agent",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Printf("golem version %s\n", version)
	},
}
