//go:build linux

package daemon

func newPlatformService(binaryPath string) Service {
	return NewSystemdService(binaryPath)
}
