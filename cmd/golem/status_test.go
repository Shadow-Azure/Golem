package main

import (
	"testing"
	"time"
)

func TestFormatDuration(t *testing.T) {
	tests := []struct {
		name     string
		input    time.Duration
		expected string
	}{
		{
			name:     "seconds less than 1 minute",
			input:    30 * time.Second,
			expected: "30 秒",
		},
		{
			name:     "exactly 1 minute",
			input:    1 * time.Minute,
			expected: "1 分钟",
		},
		{
			name:     "minutes less than 1 hour",
			input:    45 * time.Minute,
			expected: "45 分钟",
		},
		{
			name:     "exactly 1 hour",
			input:    1 * time.Hour,
			expected: "1.0 小时",
		},
		{
			name:     "hours with decimal",
			input:    2*time.Hour + 30*time.Minute,
			expected: "2.5 小时",
		},
		{
			name:     "zero duration",
			input:    0,
			expected: "0 秒",
		},
		{
			name:     "59 seconds",
			input:    59 * time.Second,
			expected: "59 秒",
		},
		{
			name:     "59 minutes",
			input:    59 * time.Minute,
			expected: "59 分钟",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := formatDuration(tt.input)
			if result != tt.expected {
				t.Errorf("formatDuration(%v) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}
