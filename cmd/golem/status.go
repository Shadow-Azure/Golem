package main

import "github.com/spf13/cobra"

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show the current status of Golem AI Agent",
	Run: func(cmd *cobra.Command, args []string) {
		// TODO: implement
	},
}
