package core

import "fmt"

// ErrorCode represents a categorized error type for the application.
type ErrorCode string

const (
	ErrCodeSessionNotFound ErrorCode = "SESSION_NOT_FOUND"
	ErrCodeProviderFailed  ErrorCode = "PROVIDER_FAILED"
	ErrCodeChannelFailed   ErrorCode = "CHANNEL_FAILED"
	ErrCodePluginFailed    ErrorCode = "PLUGIN_FAILED"
	ErrCodeConfigInvalid   ErrorCode = "CONFIG_INVALID"
	ErrCodeRateLimited     ErrorCode = "RATE_LIMITED"
	ErrCodeTimeout         ErrorCode = "TIMEOUT"
)

// AppError is the standard error type used throughout the application.
type AppError struct {
	Code    ErrorCode
	Message string
	Cause   error
}

func (e *AppError) Error() string {
	return fmt.Sprintf("[%s] %s", e.Code, e.Message)
}

func (e *AppError) Unwrap() error {
	return e.Cause
}

// IsAppError checks whether an error is an *AppError.
func IsAppError(err error) bool {
	_, ok := err.(*AppError)
	return ok
}

// GetErrorCode extracts the ErrorCode from an AppError, or returns "" if the error is not an AppError.
func GetErrorCode(err error) ErrorCode {
	if appErr, ok := err.(*AppError); ok {
		return appErr.Code
	}
	return ""
}
