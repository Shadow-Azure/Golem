package daemon

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"text/template"
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

	exec.Command("systemctl", "daemon-reload").Run()
	exec.Command("systemctl", "enable", s.unitName).Run()

	return nil
}

// Uninstall removes the systemd service
func (s *SystemdService) Uninstall() error {
	s.Stop()
	exec.Command("systemctl", "disable", s.unitName).Run()
	os.Remove(s.unitPath)
	exec.Command("systemctl", "daemon-reload").Run()
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
		cmd = exec.Command("systemctl", "show", s.unitName, "--property=MainPID")
		output, err = cmd.CombinedOutput()
		if err == nil {
			fmt.Sscanf(string(output), "MainPID=%d", &status.PID)
		}
	}

	return status, nil
}

// Compile-time check that SystemdService implements Service
var _ Service = (*SystemdService)(nil)
