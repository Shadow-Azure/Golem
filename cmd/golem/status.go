package main

import (
	"fmt"
	"time"

	"github.com/Shadow-Azure/Golem/cmd/golem/daemon"
	"github.com/spf13/cobra"
)

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show the current status of Golem AI Agent",
	Long:  "显示 Golem AI Agent 的运行状态。",
	Run: func(cmd *cobra.Command, args []string) {
		binaryPath := getBinaryPath()
		service := daemon.GetService(binaryPath)

		status, err := service.Status()
		if err != nil {
			fmt.Printf("获取状态失败: %v\n", err)
			return
		}

		if status.Running {
			fmt.Println("✅ Golem 正在运行")
			fmt.Printf("   PID: %d\n", status.PID)
			if status.Uptime > 0 {
				fmt.Printf("   运行时间: %s\n", formatDuration(status.Uptime))
			}
		} else {
			fmt.Println("❌ Golem 未运行")
			fmt.Println("使用 'golem start' 启动服务。")
		}
	},
}

func formatDuration(d time.Duration) string {
	if d < time.Minute {
		return fmt.Sprintf("%.0f 秒", d.Seconds())
	}
	if d < time.Hour {
		return fmt.Sprintf("%.0f 分钟", d.Minutes())
	}
	return fmt.Sprintf("%.1f 小时", d.Hours())
}
