package main

import "github.com/spf13/cobra"

var onboardCmd = &cobra.Command{
	Use:   "onboard",
	Short: "Interactive setup wizard for first-time users",
	Run: func(cmd *cobra.Command, args []string) {
		// TODO: implement
	},
}
