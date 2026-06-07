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
