# Golem CLI Commands Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add CLI subcommands (start/stop/restart/status/onboard/configure/version) with interactive configuration wizard and system service integration.

**Architecture:** Modular CLI using cobra framework, survey for interactive prompts, launchd/systemd for service management.

**Tech Stack:** Go 1.21+, cobra, survey/v2

---

## File Structure

```
golem/
├── cmd/golem/
│   ├── main.go              # Entry point (modify)
│   ├── root.go              # Root command (create)
│   ├── start.go             # Start command (create)
│   ├── stop.go              # Stop command (create)
│   ├── restart.go           # Restart command (create)
│   ├── status.go            # Status command (create)
│   ├── onboard.go           # Onboard wizard (create)
│   ├── configure.go         # Configure command (create)
│   ├── version.go           # Version command (create)
│   ├── foreground.go        # Foreground run logic (create)
│   └── daemon/
│       ├── service.go       # Service interface (create)
│       ├── launchd.go       # macOS service (create)
│       └── systemd.go       # Linux service (create)
├── go.mod                   # Update dependencies
└── go.sum                   # Update
```

---

## Task 1: Add Dependencies

**Files:**
- Modify: `go.mod`

- [ ] **Step 1: Add cobra and survey dependencies**

```bash
cd /Users/zn-ice/2026/Golem
go get github.com/spf13/cobra@latest
go get github.com/AlecAivazis/survey/v2@latest
go mod tidy
```

- [ ] **Step 2: Verify dependencies**

```bash
go mod graph | grep -E "(cobra|survey)"
```

Expected: Shows cobra and survey in dependency graph

- [ ] **Step 3: Commit**

```bash
git add go.mod go.sum
git commit -m "chore(deps): add cobra and survey dependencies for CLI"
```

---

## Task 2: Create Root Command and Main Entry

**Files:**
- Create: `cmd/golem/root.go`
- Modify: `cmd/golem/main.go`

- [ ] **Step 1: Create root.go**

```go
// cmd/golem/root.go
package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var cfgFile string

var rootCmd = &cobra.Command{
	Use:   "golem",
	Short: "Golem AI Agent - 本地 AI 助手框架",
	Long: `Golem 是一个任务导向的 AI 助手框架，支持飞书机器人集成。
它可以作为前台进程运行，也可以作为后台服务运行。`,
	Run: func(cmd *cobra.Command, args []string) {
		// Default behavior: run in foreground
		runForeground()
	},
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func init() {
	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "配置文件路径 (默认: ~/.golem/golem.yaml)")

	// Add all subcommands
	rootCmd.AddCommand(startCmd)
	rootCmd.AddCommand(stopCmd)
	rootCmd.AddCommand(restartCmd)
	rootCmd.AddCommand(statusCmd)
	rootCmd.AddCommand(onboardCmd)
	rootCmd.AddCommand(configureCmd)
	rootCmd.AddCommand(versionCmd)
}
```

- [ ] **Step 2: Update main.go**

```go
// cmd/golem/main.go
package main

func main() {
	Execute()
}
```

- [ ] **Step 3: Create placeholder commands**

Create temporary placeholder files for each command to ensure compilation:

```go
// cmd/golem/start.go
package main

import "github.com/spf13/cobra"

var startCmd = &cobra.Command{
	Use:   "start",
	Short: "启动后台服务",
	Run: func(cmd *cobra.Command, args []string) {
		// TODO: implement
	},
}
```

Repeat for stop.go, restart.go, status.go, onboard.go, configure.go, version.go

- [ ] **Step 4: Verify compilation**

```bash
cd /Users/zn-ice/2026/Golem
go build ./cmd/golem
```

Expected: Build succeeds

- [ ] **Step 5: Commit**

```bash
git add cmd/golem/
git commit -m "feat(cli): add root command with cobra framework"
```

---

## Task 3: Create Foreground Run Logic

**Files:**
- Create: `cmd/golem/foreground.go`

- [ ] **Step 1: Extract current main logic to foreground.go**

```go
// cmd/golem/foreground.go
package main

import (
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/Shadow-Azure/Golem/internal/config"
	"github.com/Shadow-Azure/Golem/internal/core"
	feishuPlugin "github.com/Shadow-Azure/Golem/plugins/channels/feishu"
	openaiPlugin "github.com/Shadow-Azure/Golem/plugins/providers/openai"
	claudePlugin "github.com/Shadow-Azure/Golem/plugins/providers/claude"
)

func runForeground() {
	// Setup logger
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))
	slog.SetDefault(logger)

	logger.Info("starting Golem AI Agent", "version", Version)

	// Determine config path
	configPath := cfgFile
	if configPath == "" {
		homeDir, _ := os.UserHomeDir()
		configPath = homeDir + "/.golem/golem.yaml"
	}

	// Load configuration
	cfg, err := config.Load(configPath)
	if err != nil {
		logger.Error("failed to load configuration", "error", err)
		os.Exit(1)
	}

	// Create engine
	engine, err := core.NewEngine(cfg)
	if err != nil {
		logger.Error("failed to create engine", "error", err)
		os.Exit(1)
	}

	// Create plugin manager
	pm := core.NewPluginManager()

	// Register plugins
	if openaiConfig, ok := cfg.LLM.Providers["openai"]; ok {
		openaiProv := openaiPlugin.NewProvider(openaiPlugin.ProviderConfig{
			APIKey:      openaiConfig.APIKey,
			BaseURL:     openaiConfig.BaseURL,
			Model:       openaiConfig.Model,
			Temperature: openaiConfig.Temperature,
			MaxTokens:   openaiConfig.MaxTokens,
		})
		if err := pm.LoadPlugin("openai", openaiProv); err != nil {
			logger.Error("failed to load OpenAI provider", "error", err)
		}
	}

	if claudeConfig, ok := cfg.LLM.Providers["claude"]; ok {
		claudeProv := claudePlugin.NewProvider(claudePlugin.ProviderConfig{
			APIKey:    claudeConfig.APIKey,
			BaseURL:   claudeConfig.BaseURL,
			Model:     claudeConfig.Model,
			MaxTokens: claudeConfig.MaxTokens,
		})
		if err := pm.LoadPlugin("claude", claudeProv); err != nil {
			logger.Error("failed to load Claude provider", "error", err)
		}
	}

	if feishuConfig, ok := cfg.Feishu.(map[string]interface{}); ok {
		if enabled, ok := feishuConfig["enabled"].(bool); ok && enabled {
			feishu := feishuPlugin.NewFeishuPlugin(feishuPlugin.FeishuConfig{
				AppID:             fmt.Sprintf("%v", feishuConfig["app_id"]),
				AppSecret:         fmt.Sprintf("%v", feishuConfig["app_secret"]),
				VerificationToken: fmt.Sprintf("%v", feishuConfig["verification_token"]),
			})
			feishu.SetEngine(engine)
			if err := pm.LoadPlugin("feishu", feishu); err != nil {
				logger.Error("failed to load Feishu plugin", "error", err)
			}
		}
	}

	// Start engine
	if err := engine.Start(); err != nil {
		logger.Error("failed to start engine", "error", err)
		os.Exit(1)
	}

	// Start all plugins
	if err := pm.StartAll(); err != nil {
		logger.Error("failed to start plugins", "error", err)
		os.Exit(1)
	}

	logger.Info("Golem AI Agent started successfully",
		"host", cfg.Server.Host,
		"port", cfg.Server.Port,
	)

	// Wait for shutdown signal
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	<-sigChan

	logger.Info("shutting down Golem AI Agent")

	// Stop all plugins
	if err := pm.StopAll(); err != nil {
		logger.Error("error stopping plugins", "error", err)
	}

	// Shutdown engine
	if err := engine.Shutdown(); err != nil {
		logger.Error("error shutting down engine", "error", err)
	}

	logger.Info("Golem AI Agent stopped")
	fmt.Println("Golem AI Agent stopped")
}
```

- [ ] **Step 2: Verify compilation**

```bash
cd /Users/zn-ice/2026/Golem
go build ./cmd/golem
```

- [ ] **Step 3: Commit**

```bash
git add cmd/golem/foreground.go
git commit -m "feat(cli): extract foreground run logic"
```

---

## Task 4: Create Daemon Service Interface

**Files:**
- Create: `cmd/golem/daemon/service.go`

- [ ] **Step 1: Create daemon directory**

```bash
mkdir -p cmd/golem/daemon
```

- [ ] **Step 2: Create service.go**

```go
// cmd/golem/daemon/service.go
package daemon

import "time"

// Service defines the interface for daemon service management
type Service interface {
	// Install installs the service
	Install() error

	// Uninstall uninstalls the service
	Uninstall() error

	// Start starts the service
	Start() error

	// Stop stops the service
	Stop() error

	// Restart restarts the service
	Restart() error

	// Status returns the service status
	Status() (ServiceStatus, error)
}

// ServiceStatus represents the service status
type ServiceStatus struct {
	Running   bool
	PID       int
	StartTime time.Time
	Uptime    time.Duration
}

// GetService returns the appropriate service implementation for the current OS
func GetService(binaryPath string) Service {
	return newPlatformService(binaryPath)
}
```

- [ ] **Step 3: Commit**

```bash
git add cmd/golem/daemon/
git commit -m "feat(daemon): add service interface"
```

---

## Task 5: Create macOS Launchd Service

**Files:**
- Create: `cmd/golem/daemon/launchd.go`
- Create: `cmd/golem/daemon/service_darwin.go`

- [ ] **Step 1: Create launchd.go**

```go
// cmd/golem/daemon/launchd.go
package daemon

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"text/template"
	"time"
)

const launchdPlistTmpl = `<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
    <key>Label</key>
    <string>com.shadow-azure.golem</string>
    <key>ProgramArguments</key>
    <array>
        <string>{{.BinaryPath}}</string>
    </array>
    <key>RunAtLoad</key>
    <true/>
    <key>KeepAlive</key>
    <true/>
    <key>StandardOutPath</key>
    <string>{{.LogPath}}</string>
    <key>StandardErrorPath</key>
    <string>{{.LogPath}}</string>
    <key>WorkingDirectory</key>
    <string>{{.WorkingDir}}</string>
</dict>
</plist>`

// LaunchdService implements Service for macOS using launchd
type LaunchdService struct {
	binaryPath string
	plistPath  string
	logPath    string
	workingDir string
	label      string
}

// NewLaunchdService creates a new LaunchdService
func NewLaunchdService(binaryPath string) *LaunchdService {
	homeDir, _ := os.UserHomeDir()
	return &LaunchdService{
		binaryPath: binaryPath,
		plistPath:  filepath.Join(homeDir, "Library", "LaunchAgents", "com.shadow-azure.golem.plist"),
		logPath:    filepath.Join(homeDir, ".golem", "golem.log"),
		workingDir: homeDir,
		label:      "com.shadow-azure.golem",
	}
}

// Install installs the launchd service
func (s *LaunchdService) Install() error {
	// Create LaunchAgents directory if it doesn't exist
	plistDir := filepath.Dir(s.plistPath)
	if err := os.MkdirAll(plistDir, 0755); err != nil {
		return fmt.Errorf("failed to create LaunchAgents directory: %w", err)
	}

	// Create log directory
	logDir := filepath.Dir(s.logPath)
	if err := os.MkdirAll(logDir, 0755); err != nil {
		return fmt.Errorf("failed to create log directory: %w", err)
	}

	// Generate plist file
	tmpl, err := template.New("plist").Parse(launchdPlistTmpl)
	if err != nil {
		return fmt.Errorf("failed to parse plist template: %w", err)
	}

	file, err := os.Create(s.plistPath)
	if err != nil {
		return fmt.Errorf("failed to create plist file: %w", err)
	}
	defer file.Close()

	data := struct {
		BinaryPath string
		LogPath    string
		WorkingDir string
	}{
		BinaryPath: s.binaryPath,
		LogPath:    s.logPath,
		WorkingDir: s.workingDir,
	}

	if err := tmpl.Execute(file, data); err != nil {
		return fmt.Errorf("failed to generate plist: %w", err)
	}

	return nil
}

// Uninstall removes the launchd service
func (s *LaunchdService) Uninstall() error {
	// Stop service first
	s.Stop()

	// Remove plist file
	if err := os.Remove(s.plistPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to remove plist: %w", err)
	}

	return nil
}

// Start starts the launchd service
func (s *LaunchdService) Start() error {
	cmd := exec.Command("launchctl", "load", "-w", s.plistPath)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to start service: %s - %w", string(output), err)
	}
	return nil
}

// Stop stops the launchd service
func (s *LaunchdService) Stop() error {
	cmd := exec.Command("launchctl", "unload", "-w", s.plistPath)
	output, err := cmd.CombinedOutput()
	if err != nil {
		// Ignore error if service is not loaded
		if strings.Contains(string(output), "Could not find specified service") {
			return nil
		}
		return fmt.Errorf("failed to stop service: %s - %w", string(output), err)
	}
	return nil
}

// Restart restarts the launchd service
func (s *LaunchdService) Restart() error {
	if err := s.Stop(); err != nil {
		return err
	}
	time.Sleep(500 * time.Millisecond)
	return s.Start()
}

// Status returns the service status
func (s *LaunchdService) Status() (ServiceStatus, error) {
	cmd := exec.Command("launchctl", "list", s.label)
	output, err := cmd.CombinedOutput()
	if err != nil {
		// Service not loaded
		return ServiceStatus{Running: false}, nil
	}

	// Parse output to get PID
	status := ServiceStatus{Running: true}
	lines := strings.Split(string(output), "\n")
	for _, line := range lines {
		if strings.HasPrefix(line, "\"PID\"") {
			fmt.Sscanf(line, "\"PID\" = %d", &status.PID)
		}
	}

	return status, nil
}
```

- [ ] **Step 2: Create service_darwin.go**

```go
// cmd/golem/daemon/service_darwin.go
//go:build darwin

package daemon

func newPlatformService(binaryPath string) Service {
	return NewLaunchdService(binaryPath)
}
```

- [ ] **Step 3: Commit**

```bash
git add cmd/golem/daemon/
git commit -m "feat(daemon): implement macOS launchd service"
```

---

## Task 6: Create Linux Systemd Service

**Files:**
- Create: `cmd/golem/daemon/systemd.go`
- Create: `cmd/golem/daemon/service_linux.go`

- [ ] **Step 1: Create systemd.go**

```go
// cmd/golem/daemon/systemd.go
package daemon

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"text/template"
	"time"
)

const systemdUnitTmpl = `[Unit]
Description=Golem AI Agent
After=network.target

[Service]
Type=simple
ExecStart={{.BinaryPath}}
Restart=always
RestartSec=5
User={{.User}}
WorkingDirectory={{.WorkingDir}}

[Install]
WantedBy=multi-user.target`

// SystemdService implements Service for Linux using systemd
type SystemdService struct {
	binaryPath string
	unitPath   string
	user       string
	workingDir string
	unitName   string
}

// NewSystemdService creates a new SystemdService
func NewSystemdService(binaryPath string) *SystemdService {
	homeDir, _ := os.UserHomeDir()
	return &SystemdService{
		binaryPath: binaryPath,
		unitPath:   "/etc/systemd/system/golem.service",
		user:       os.Getenv("USER"),
		workingDir: homeDir,
		unitName:   "golem.service",
	}
}

// Install installs the systemd service
func (s *SystemdService) Install() error {
	// Generate unit file
	tmpl, err := template.New("unit").Parse(systemdUnitTmpl)
	if err != nil {
		return fmt.Errorf("failed to parse unit template: %w", err)
	}

	file, err := os.Create(s.unitPath)
	if err != nil {
		return fmt.Errorf("failed to create unit file: %w (try with sudo)", err)
	}
	defer file.Close()

	data := struct {
		BinaryPath string
		User       string
		WorkingDir string
	}{
		BinaryPath: s.binaryPath,
		User:       s.user,
		WorkingDir: s.workingDir,
	}

	if err := tmpl.Execute(file, data); err != nil {
		return fmt.Errorf("failed to generate unit: %w", err)
	}

	// Reload systemd
	cmd := exec.Command("systemctl", "daemon-reload")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to reload systemd: %w", err)
	}

	// Enable service
	cmd = exec.Command("systemctl", "enable", s.unitName)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to enable service: %w", err)
	}

	return nil
}

// Uninstall removes the systemd service
func (s *SystemdService) Uninstall() error {
	// Stop service first
	s.Stop()

	// Disable service
	cmd := exec.Command("systemctl", "disable", s.unitName)
	cmd.Run()

	// Remove unit file
	if err := os.Remove(s.unitPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to remove unit file: %w", err)
	}

	// Reload systemd
	cmd = exec.Command("systemctl", "daemon-reload")
	cmd.Run()

	return nil
}

// Start starts the systemd service
func (s *SystemdService) Start() error {
	cmd := exec.Command("systemctl", "start", s.unitName)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to start service: %s - %w", string(output), err)
	}
	return nil
}

// Stop stops the systemd service
func (s *SystemdService) Stop() error {
	cmd := exec.Command("systemctl", "stop", s.unitName)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to stop service: %s - %w", string(output), err)
	}
	return nil
}

// Restart restarts the systemd service
func (s *SystemdService) Restart() error {
	cmd := exec.Command("systemctl", "restart", s.unitName)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to restart service: %s - %w", string(output), err)
	}
	return nil
}

// Status returns the service status
func (s *SystemdService) Status() (ServiceStatus, error) {
	cmd := exec.Command("systemctl", "is-active", s.unitName)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return ServiceStatus{Running: false}, nil
	}

	status := ServiceStatus{
		Running: strings.TrimSpace(string(output)) == "active",
	}

	if status.Running {
		// Get PID
		cmd = exec.Command("systemctl", "show", s.unitName, "--property=MainPID")
		output, err = cmd.CombinedOutput()
		if err == nil {
			fmt.Sscanf(string(output), "MainPID=%d", &status.PID)
		}

		// Get start time
		cmd = exec.Command("systemctl", "show", s.unitName, "--property=ActiveEnterTimestamp")
		output, err = cmd.CombinedOutput()
		if err == nil {
			timeStr := strings.TrimPrefix(strings.TrimSpace(string(output)), "ActiveEnterTimestamp=")
			if t, err := time.Parse("Mon 2006-01-02 15:04:05 MST", timeStr); err == nil {
				status.StartTime = t
				status.Uptime = time.Since(t)
			}
		}
	}

	return status, nil
}
```

- [ ] **Step 2: Create service_linux.go**

```go
// cmd/golem/daemon/service_linux.go
//go:build linux

package daemon

func newPlatformService(binaryPath string) Service {
	return NewSystemdService(binaryPath)
}
```

- [ ] **Step 3: Commit**

```bash
git add cmd/golem/daemon/
git commit -m "feat(daemon): implement Linux systemd service"
```

---

## Task 7: Implement Start/Stop/Restart/Status Commands

**Files:**
- Modify: `cmd/golem/start.go`
- Modify: `cmd/golem/stop.go`
- Modify: `cmd/golem/restart.go`
- Modify: `cmd/golem/status.go`

- [ ] **Step 1: Implement start.go**

```go
// cmd/golem/start.go
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
	Short: "启动后台服务",
	Long:  "将 Golem AI Agent 作为后台服务启动。",
	Run: func(cmd *cobra.Command, args []string) {
		binaryPath := getBinaryPath()
		service := daemon.GetService(binaryPath)

		// Check if already running
		status, _ := service.Status()
		if status.Running {
			fmt.Println("Golem 已在运行中。")
			return
		}

		// Install and start service
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
```

- [ ] **Step 2: Implement stop.go**

```go
// cmd/golem/stop.go
package main

import (
	"fmt"
	"os"

	"github.com/Shadow-Azure/Golem/cmd/golem/daemon"
	"github.com/spf13/cobra"
)

var stopCmd = &cobra.Command{
	Use:   "stop",
	Short: "停止后台服务",
	Long:  "停止正在运行的 Golem AI Agent 后台服务。",
	Run: func(cmd *cobra.Command, args []string) {
		binaryPath := getBinaryPath()
		service := daemon.GetService(binaryPath)

		// Check if running
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
```

- [ ] **Step 3: Implement restart.go**

```go
// cmd/golem/restart.go
package main

import (
	"fmt"
	"os"

	"github.com/Shadow-Azure/Golem/cmd/golem/daemon"
	"github.com/spf13/cobra"
)

var restartCmd = &cobra.Command{
	Use:   "restart",
	Short: "重启后台服务",
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
```

- [ ] **Step 4: Implement status.go**

```go
// cmd/golem/status.go
package main

import (
	"fmt"

	"github.com/Shadow-Azure/Golem/cmd/golem/daemon"
	"github.com/spf13/cobra"
)

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "查看服务状态",
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
```

- [ ] **Step 5: Verify compilation**

```bash
cd /Users/zn-ice/2026/Golem
go build ./cmd/golem
```

- [ ] **Step 6: Commit**

```bash
git add cmd/golem/start.go cmd/golem/stop.go cmd/golem/restart.go cmd/golem/status.go
git commit -m "feat(cli): implement start/stop/restart/status commands"
```

---

## Task 8: Implement Onboard Wizard

**Files:**
- Modify: `cmd/golem/onboard.go`

- [ ] **Step 1: Implement onboard.go**

```go
// cmd/golem/onboard.go
package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/AlecAivazis/survey/v2"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

var onboardCmd = &cobra.Command{
	Use:   "onboard",
	Short: "首次配置向导",
	Long:  "交互式配置 Golem AI Agent，包括 LLM 提供商和飞书连接。",
	Run:   runOnboard,
}

func runOnboard(cmd *cobra.Command, args []string) {
	fmt.Println("🤖 欢迎使用 Golem AI Agent！")
	fmt.Println("让我们来配置你的 AI 助手。")
	fmt.Println()

	config := make(map[string]interface{})

	// Step 1: Select LLM provider
	var provider string
	prompt := &survey.Select{
		Message: "选择 LLM 提供商:",
		Options: []string{"OpenAI", "Claude", "跳过"},
		Default: "OpenAI",
	}
	survey.AskOne(prompt, &provider)

	// Step 2: Configure LLM
	if provider != "跳过" {
		llmConfig := configureLLMWizard(provider)
		config["llm"] = llmConfig
	}

	// Step 3: Configure Feishu
	var enableFeishu bool
	prompt2 := &survey.Confirm{
		Message: "是否启用飞书机器人?",
		Default: false,
	}
	survey.AskOne(prompt2, &enableFeishu)

	if enableFeishu {
		feishuConfig := configureFeishuWizard()
		config["feishu"] = feishuConfig
	}

	// Step 4: Save config
	if err := saveConfig(config); err != nil {
		fmt.Printf("保存配置失败: %v\n", err)
		os.Exit(1)
	}

	fmt.Println()
	fmt.Println("✅ 配置完成！")
	fmt.Println()
	fmt.Println("下一步:")
	fmt.Println("  1. 运行 'golem start' 启动后台服务")
	fmt.Println("  2. 或运行 'golem' 前台运行")
}

func configureLLMWizard(provider string) map[string]interface{} {
	llmConfig := make(map[string]interface{})

	if provider == "OpenAI" {
		llmConfig["default_provider"] = "openai"

		var apiKey string
		prompt := &survey.Password{
			Message: "输入 OpenAI API Key:",
		}
		survey.AskOne(prompt, &apiKey)

		var model string
		prompt2 := &survey.Select{
			Message: "选择模型:",
			Options: []string{"gpt-4o", "gpt-4", "gpt-3.5-turbo"},
			Default: "gpt-4o",
		}
		survey.AskOne(prompt2, &model)

		llmConfig["providers"] = map[string]interface{}{
			"openai": map[string]interface{}{
				"api_key": apiKey,
				"model":   model,
			},
		}
	} else {
		llmConfig["default_provider"] = "claude"

		var apiKey string
		prompt := &survey.Password{
			Message: "输入 Anthropic API Key:",
		}
		survey.AskOne(prompt, &apiKey)

		var model string
		prompt2 := &survey.Select{
			Message: "选择模型:",
			Options: []string{"claude-3-opus-20240229", "claude-3-sonnet-20240229", "claude-3-haiku-20240229"},
			Default: "claude-3-opus-20240229",
		}
		survey.AskOne(prompt2, &model)

		llmConfig["providers"] = map[string]interface{}{
			"claude": map[string]interface{}{
				"api_key": apiKey,
				"model":   model,
			},
		}
	}

	return llmConfig
}

func configureFeishuWizard() map[string]interface{} {
	var appId, appSecret, verificationToken string

	prompt1 := &survey.Input{
		Message: "输入飞书 App ID:",
	}
	survey.AskOne(prompt1, &appId)

	prompt2 := &survey.Password{
		Message: "输入飞书 App Secret:",
	}
	survey.AskOne(prompt2, &appSecret)

	prompt3 := &survey.Password{
		Message: "输入飞书 Verification Token:",
	}
	survey.AskOne(prompt3, &verificationToken)

	return map[string]interface{}{
		"enabled":            true,
		"app_id":             appId,
		"app_secret":         appSecret,
		"verification_token": verificationToken,
	}
}

func saveConfig(config map[string]interface{}) error {
	homeDir, _ := os.UserHomeDir()
	configDir := filepath.Join(homeDir, ".golem")

	// Create directory if it doesn't exist
	if err := os.MkdirAll(configDir, 0755); err != nil {
		return err
	}

	// Add server config
	config["server"] = map[string]interface{}{
		"host": "127.0.0.1",
		"port": 9921,
	}

	// Add session config
	config["session"] = map[string]interface{}{
		"max_history": 50,
		"trim_to":     20,
	}

	// Add logging config
	config["logging"] = map[string]interface{}{
		"level":  "info",
		"format": "json",
	}

	// Marshal to YAML
	data, err := yaml.Marshal(config)
	if err != nil {
		return err
	}

	// Write to file
	configPath := filepath.Join(configDir, "golem.yaml")
	return os.WriteFile(configPath, data, 0644)
}
```

- [ ] **Step 2: Verify compilation**

```bash
cd /Users/zn-ice/2026/Golem
go build ./cmd/golem
```

- [ ] **Step 3: Commit**

```bash
git add cmd/golem/onboard.go
git commit -m "feat(cli): implement onboard configuration wizard"
```

---

## Task 9: Implement Configure Command

**Files:**
- Modify: `cmd/golem/configure.go`

- [ ] **Step 1: Implement configure.go**

```go
// cmd/golem/configure.go
package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/AlecAivazis/survey/v2"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

var configureCmd = &cobra.Command{
	Use:   "configure [llm|feishu]",
	Short: "更新配置",
	Long:  "交互式更新 Golem AI Agent 配置。",
	Args:  cobra.MaximumNArgs(1),
	Run:   runConfigure,
}

func runConfigure(cmd *cobra.Command, args []string) {
	var section string
	if len(args) > 0 {
		section = args[0]
	} else {
		prompt := &survey.Select{
			Message: "选择要配置的部分:",
			Options: []string{"llm", "feishu"},
		}
		survey.AskOne(prompt, &section)
	}

	// Load existing config
	homeDir, _ := os.UserHomeDir()
	configPath := filepath.Join(homeDir, ".golem", "golem.yaml")

	var config map[string]interface{}
	data, err := os.ReadFile(configPath)
	if err != nil {
		config = make(map[string]interface{})
	} else {
		yaml.Unmarshal(data, &config)
	}

	// Update config based on section
	switch section {
	case "llm":
		llmConfig := configureLLMWizard("OpenAI")
		config["llm"] = llmConfig
	case "feishu":
		feishuConfig := configureFeishuWizard()
		config["feishu"] = feishuConfig
	default:
		fmt.Printf("未知配置部分: %s\n", section)
		os.Exit(1)
	}

	// Save config
	if err := saveConfig(config); err != nil {
		fmt.Printf("保存配置失败: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("✅ 配置已更新。")
	fmt.Println("运行 'golem restart' 使配置生效。")
}
```

- [ ] **Step 2: Commit**

```bash
git add cmd/golem/configure.go
git commit -m "feat(cli): implement configure command"
```

---

## Task 10: Implement Version Command

**Files:**
- Create: `cmd/golem/version.go`

- [ ] **Step 1: Implement version.go**

```go
// cmd/golem/version.go
package main

import (
	"fmt"

	"github.com/spf13/cobra"
)

// Version is set at build time
var Version = "dev"

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "显示版本信息",
	Long:  "显示 Golem AI Agent 的版本信息。",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Printf("Golem AI Agent v%s\n", Version)
	},
}
```

- [ ] **Step 2: Commit**

```bash
git add cmd/golem/version.go
git commit -m "feat(cli): implement version command"
```

---

## Task 11: Update Release Script

**Files:**
- Modify: `release.sh`

- [ ] **Step 1: Update release.sh to set version**

Find the build step and add `-ldflags` to set version:

```bash
# In release.sh, update the build command:
CGO_ENABLED=0 GOOS=$GOOS GOARCH=$GOARCH go build -ldflags "-X main.Version=${VERSION}" -o release/golem ./cmd/golem
```

- [ ] **Step 2: Commit**

```bash
git add release.sh
git commit -m "build(release): set version at build time"
```

---

## Task 12: Final Testing

- [ ] **Step 1: Build and test all commands**

```bash
cd /Users/zn-ice/2026/Golem
go build -ldflags "-X main.Version=1.0.0" -o bin/golem ./cmd/golem
./bin/golem --help
./bin/golem version
```

- [ ] **Step 2: Test onboard wizard**

```bash
./bin/golem onboard
```

- [ ] **Step 3: Run all Go tests**

```bash
go test ./...
```

- [ ] **Step 4: Run release script**

```bash
./release.sh
```

- [ ] **Step 5: Final commit**

```bash
git add -A
git commit -m "feat(cli): complete CLI implementation with all commands"
```

---

## Summary

This plan implements:

1. **cobra CLI framework** - Root command with subcommands
2. **start/stop/restart/status** - Daemon service management
3. **onboard** - Interactive configuration wizard
4. **configure** - Configuration updates
5. **version** - Version information
6. **launchd/systemd** - Platform-specific service integration

**Total Tasks:** 12
**Estimated Time:** 2-3 hours
