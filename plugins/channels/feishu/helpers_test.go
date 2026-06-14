package feishu

import (
	"testing"
)

func TestDerefString(t *testing.T) {
	tests := []struct {
		name     string
		input    *string
		expected string
	}{
		{
			name:     "nil pointer",
			input:    nil,
			expected: "",
		},
		{
			name:     "empty string",
			input:    strPtr(""),
			expected: "",
		},
		{
			name:     "non-empty string",
			input:    strPtr("hello"),
			expected: "hello",
		},
		{
			name:     "string with spaces",
			input:    strPtr("hello world"),
			expected: "hello world",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := derefString(tt.input)
			if result != tt.expected {
				t.Errorf("derefString(%v) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestSplitSessionID(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected []string
	}{
		{
			name:     "simple session ID",
			input:    "user123",
			expected: []string{"user123"},
		},
		{
			name:     "session ID with channel prefix",
			input:    "feishu:user123",
			expected: []string{"feishu", "user123"},
		},
		{
			name:     "session ID with multiple colons",
			input:    "feishu:group:user123",
			expected: []string{"feishu", "group", "user123"},
		},
		{
			name:     "empty string",
			input:    "",
			expected: []string{},
		},
		{
			name:     "only colons",
			input:    "::",
			expected: []string{"", ""},
		},
		{
			name:     "trailing colon",
			input:    "feishu:user123:",
			expected: []string{"feishu", "user123"},
		},
		{
			name:     "leading colon",
			input:    ":feishu:user123",
			expected: []string{"", "feishu", "user123"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := splitSessionID(tt.input)
			if len(result) != len(tt.expected) {
				t.Errorf("splitSessionID(%q) returned %d parts, want %d", tt.input, len(result), len(tt.expected))
				return
			}
			for i, part := range result {
				if part != tt.expected[i] {
					t.Errorf("splitSessionID(%q)[%d] = %q, want %q", tt.input, i, part, tt.expected[i])
				}
			}
		})
	}
}

func TestEscapeJSON(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "no special characters",
			input:    "hello",
			expected: "hello",
		},
		{
			name:     "double quote",
			input:    `hello "world"`,
			expected: `hello \"world\"`,
		},
		{
			name:     "backslash",
			input:    `hello\world`,
			expected: `hello\\world`,
		},
		{
			name:     "newline",
			input:    "hello\nworld",
			expected: `hello\nworld`,
		},
		{
			name:     "carriage return",
			input:    "hello\rworld",
			expected: `hello\rworld`,
		},
		{
			name:     "tab",
			input:    "hello\tworld",
			expected: `hello\tworld`,
		},
		{
			name:     "multiple special characters",
			input:    "hello\n\t\"world\"",
			expected: `hello\n\t\"world\"`,
		},
		{
			name:     "empty string",
			input:    "",
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := escapeJSON(tt.input)
			if result != tt.expected {
				t.Errorf("escapeJSON(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

// Helper function to create string pointer
func strPtr(s string) *string {
	return &s
}
