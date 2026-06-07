package main

import "github.com/spf13/cobra"

var stopCmd = &cobra.Command{
	Use:   "stop",
	Short: "Stop the running Golem AI Agent service",
	Run: func(cmd *cobra.Command, args []string) {
		// TODO: implement
	},
}
