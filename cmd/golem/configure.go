package main

import "github.com/spf13/cobra"

var configureCmd = &cobra.Command{
	Use:   "configure",
	Short: "View or modify Golem configuration",
	Run: func(cmd *cobra.Command, args []string) {
		// TODO: implement
	},
}
