package config

import (
	"fmt"
	"os"
	"regexp"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// Load loads configuration from a YAML file.
func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	// Expand environment variables
	expanded := expandEnvVars(string(data))

	var config Config
	if err := yaml.Unmarshal([]byte(expanded), &config); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}

	// Apply defaults for missing values
	applyDefaults(&config)

	return &config, nil
}

// DefaultConfig returns a Config with default values.
func DefaultConfig() *Config {
	config := &Config{}
	applyDefaults(config)
	return config
}

// expandEnvVars expands ${VAR} or $VAR patterns in the string.
func expandEnvVars(s string) string {
	re := regexp.MustCompile(`\$\{([^}]+)\}|\$([A-Za-z_][A-Za-z0-9_]*)`)
	return re.ReplaceAllStringFunc(s, func(match string) string {
		// Extract variable name
		var varName string
		if strings.HasPrefix(match, "${") {
			varName = match[2 : len(match)-1]
		} else {
			varName = match[1:]
		}

		// Get environment variable
		if val, exists := os.LookupEnv(varName); exists {
			return val
		}

		// Return original if not found
		return match
	})
}

// applyDefaults applies default values to missing configuration fields.
func applyDefaults(config *Config) {
	if config.Server.Host == "" {
		config.Server.Host = "0.0.0.0"
	}
	if config.Server.Port == 0 {
		config.Server.Port = 9921
	}
	if config.Session.MaxHistory == 0 {
		config.Session.MaxHistory = 50
	}
	if config.Session.TrimTo == 0 {
		config.Session.TrimTo = 20
	}
	if config.Session.IdleTimeout == 0 {
		config.Session.IdleTimeout = 30 * time.Minute
	}
	if config.Session.CleanupInterval == 0 {
		config.Session.CleanupInterval = 5 * time.Minute
	}
	if config.Logging.Level == "" {
		config.Logging.Level = "info"
	}
	if config.Logging.Format == "" {
		config.Logging.Format = "json"
	}
}
