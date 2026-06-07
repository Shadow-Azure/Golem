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
	plistDir := filepath.Dir(s.plistPath)
	if err := os.MkdirAll(plistDir, 0755); err != nil {
		return fmt.Errorf("failed to create LaunchAgents directory: %w", err)
	}

	logDir := filepath.Dir(s.logPath)
	if err := os.MkdirAll(logDir, 0755); err != nil {
		return fmt.Errorf("failed to create log directory: %w", err)
	}

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
	s.Stop()
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
		return ServiceStatus{Running: false}, nil
	}

	status := ServiceStatus{Running: true}
	lines := strings.Split(string(output), "\n")
	for _, line := range lines {
		if strings.HasPrefix(line, "\"PID\"") {
			fmt.Sscanf(line, "\"PID\" = %d", &status.PID)
		}
	}

	return status, nil
}

// Compile-time check that LaunchdService implements Service
var _ Service = (*LaunchdService)(nil)
