package core

import (
	"testing"
)

func TestAppError_Error(t *testing.T) {
	err := &AppError{
		Code:    ErrCodeSessionNotFound,
		Message: "session not found",
	}
	expected := "[SESSION_NOT_FOUND] session not found"
	if err.Error() != expected {
		t.Errorf("expected '%s', got '%s'", expected, err.Error())
	}
}

func TestAppError_WithCause(t *testing.T) {
	cause := &AppError{
		Code:    ErrCodeProviderFailed,
		Message: "API error",
	}
	err := &AppError{
		Code:    ErrCodeChannelFailed,
		Message: "failed to send message",
		Cause:   cause,
	}
	if err.Unwrap() != cause {
		t.Error("expected cause to be unwrapped")
	}
}

func TestIsAppError(t *testing.T) {
	appErr := &AppError{
		Code:    ErrCodeSessionNotFound,
		Message: "not found",
	}
	if !IsAppError(appErr) {
		t.Error("expected IsAppError to return true for AppError")
	}
	if IsAppError(nil) {
		t.Error("expected IsAppError to return false for nil")
	}
}

func TestGetErrorCode(t *testing.T) {
	appErr := &AppError{
		Code:    ErrCodeRateLimited,
		Message: "rate limited",
	}
	code := GetErrorCode(appErr)
	if code != ErrCodeRateLimited {
		t.Errorf("expected code %s, got %s", ErrCodeRateLimited, code)
	}
	code = GetErrorCode(nil)
	if code != "" {
		t.Errorf("expected empty code for nil, got %s", code)
	}
}
