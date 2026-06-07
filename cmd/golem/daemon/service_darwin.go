//go:build darwin

package daemon

func newPlatformService(binaryPath string) Service {
	return NewLaunchdService(binaryPath)
}
