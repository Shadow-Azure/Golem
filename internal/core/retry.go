package core

import (
	"context"
	"math"
	"time"
)

// RetryConfig defines the parameters for retry behavior.
type RetryConfig struct {
	MaxRetries  int
	BaseDelay   time.Duration
	MaxDelay    time.Duration
	RetryableFn func(error) bool
}

// WithRetry executes fn up to config.MaxRetries+1 times, using exponential backoff
// between retries. It respects context cancellation and only retries errors that
// are deemed retryable by config.RetryableFn.
func WithRetry(ctx context.Context, config RetryConfig, fn func() error) error {
	var lastErr error

	for i := 0; i <= config.MaxRetries; i++ {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		if err := fn(); err != nil {
			lastErr = err
			if !config.RetryableFn(err) {
				return err
			}
			if i < config.MaxRetries {
				delay := calculateBackoff(i, config)
				select {
				case <-ctx.Done():
					return ctx.Err()
				case <-time.After(delay):
					continue
				}
			}
		} else {
			return nil
		}
	}

	return lastErr
}

// calculateBackoff returns the delay for the given attempt using exponential
// backoff, capped at config.MaxDelay.
func calculateBackoff(attempt int, config RetryConfig) time.Duration {
	delay := float64(config.BaseDelay) * math.Pow(2, float64(attempt))
	if delay > float64(config.MaxDelay) {
		delay = float64(config.MaxDelay)
	}
	return time.Duration(delay)
}
