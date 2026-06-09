package main

import (
	"fmt"
	"os"

	"github.com/Shadow-Azure/Golem/cmd/golem/daemon"
	"github.com/spf13/cobra"
)

var restartCmd = &cobra.Command{
	Use:   "restart",
	Short: "Restart the Golem AI Agent service",
	Long:  "重启 Golem AI Agent 后台服务。",
	Run: func(cmd *cobra.Command, args []string) {
		binaryPath := getBinaryPath()
		service := daemon.GetService(binaryPath)

		fmt.Println("正在重启服务...")
		if err := service.Restart(); err != nil {
			fmt.Printf("重启服务失败: %v\n", err)
			os.Exit(1)
		}

		fmt.Println("✅ Golem 已重启。")
	},
}
