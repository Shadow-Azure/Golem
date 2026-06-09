package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/Shadow-Azure/Golem/cmd/golem/daemon"
	"github.com/spf13/cobra"
)

var startCmd = &cobra.Command{
	Use:   "start",
	Short: "Start Golem AI Agent as a background service",
	Long:  "将 Golem AI Agent 作为后台服务启动。",
	Run: func(cmd *cobra.Command, args []string) {
		binaryPath := getBinaryPath()
		service := daemon.GetService(binaryPath)

		status, _ := service.Status()
		if status.Running {
			fmt.Println("Golem 已在运行中。")
			return
		}

		fmt.Println("正在安装服务...")
		if err := service.Install(); err != nil {
			fmt.Printf("安装服务失败: %v\n", err)
			os.Exit(1)
		}

		fmt.Println("正在启动服务...")
		if err := service.Start(); err != nil {
			fmt.Printf("启动服务失败: %v\n", err)
			os.Exit(1)
		}

		fmt.Println("✅ Golem 已启动。")
		fmt.Println("使用 'golem status' 查看运行状态。")
	},
}

func getBinaryPath() string {
	execPath, err := os.Executable()
	if err != nil {
		return "/usr/local/bin/golem"
	}
	absPath, err := filepath.Abs(execPath)
	if err != nil {
		return "/usr/local/bin/golem"
	}
	return absPath
}
