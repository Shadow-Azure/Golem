package main

import (
	"fmt"
	"os"

	"github.com/Shadow-Azure/Golem/cmd/golem/daemon"
	"github.com/spf13/cobra"
)

var stopCmd = &cobra.Command{
	Use:   "stop",
	Short: "Stop the running Golem AI Agent service",
	Long:  "停止正在运行的 Golem AI Agent 后台服务。",
	Run: func(cmd *cobra.Command, args []string) {
		binaryPath := getBinaryPath()
		service := daemon.GetService(binaryPath)

		status, _ := service.Status()
		if !status.Running {
			fmt.Println("Golem 未运行。")
			return
		}

		fmt.Println("正在停止服务...")
		if err := service.Stop(); err != nil {
			fmt.Printf("停止服务失败: %v\n", err)
			os.Exit(1)
		}

		fmt.Println("✅ Golem 已停止。")
	},
}
