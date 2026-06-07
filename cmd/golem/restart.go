package main

import "github.com/spf13/cobra"

var restartCmd = &cobra.Command{
	Use:   "restart",
	Short: "Restart the Golem AI Agent service",
	Run: func(cmd *cobra.Command, args []string) {
		// TODO: implement
	},
}
