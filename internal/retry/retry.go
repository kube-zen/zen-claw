// Package retry implements retry with exponential backoff for AI calls.
// Adapted from zen-brain's retry.go
package retry

import (
	"context"
	"fmt"
	"log"
	"math/rand"
	"strings"
	"time"
)

// Config configures retry behavior
type Config struct {
	Enabled     bool
	MaxAttempts int           // Max retry attempts (0 = no retries)
	BaseDelay   time.Duration // Initial delay before first retry
	MaxDelay    time.Duration // Maximum delay between retries
}

// DefaultConfig returns sensible defaults
func DefaultConfig() Config {
	return Config{
		Enabled:     true,
		MaxAttempts: 3,
		BaseDelay:   500 * time.Millisecond,
		MaxDelay:    30 * time.Second,
	}
}

// Result holds the result of a retryable operation
type Result struct {
	Response string
	Tokens   int
	Error    error
}

// Do executes fn with retry logic
// Returns the first successful result or the last error after all attempts
func Do(ctx context.Context, cfg Config, fn func() (string, int, error)) (string, int, error) {
	if !cfg.Enabled || cfg.MaxAttempts == 0 {
		return fn()
	}

	var lastErr error

	for attempt := 0; attempt <= cfg.MaxAttempts; attempt++ {
		response, tokens, err := fn()

		if err == nil {
			if attempt > 0 {
				log.Printf("[Retry] Success on attempt %d/%d", attempt+1, cfg.MaxAttempts+1)
			}
			return response, tokens, nil
		}

		lastErr = err

		// Don't retry on non-retryable errors
		if !IsRetryable(err) {
			log.Printf("[Retry] Non-retryable error: %v", err)
			return "", 0, err
		}

		// Last attempt, don't sleep
		if attempt == cfg.MaxAttempts {
			break
		}

		// Calculate exponential backoff with jitter
		delay := calculateBackoff(attempt, cfg.BaseDelay, cfg.MaxDelay)

		log.Printf("[Retry] Attempt %d/%d failed: %v. Retrying in %v...",
			attempt+1, cfg.MaxAttempts+1, err, delay)

		// Wait with context cancellation support
		select {
		case <-ctx.Done():
			return "", 0, fmt.Errorf("retry cancelled: %w", ctx.Err())
		case <-time.After(delay):
			continue
		}
	}

	return "", 0, fmt.Errorf("max retries exceeded (%d attempts): %w", cfg.MaxAttempts+1, lastErr)
}

// calculateBackoff computes delay with exponential backoff and jitter
func calculateBackoff(attempt int, baseDelay, maxDelay time.Duration) time.Duration {
	// Exponential: 1x, 2x, 4x, 8x...
	backoff := baseDelay * time.Duration(1<<uint(attempt))

	// Cap at max delay
	if backoff > maxDelay {
		backoff = maxDelay
	}

	// Add jitter (0-50% of backoff)
	jitter := time.Duration(rand.Int63n(int64(backoff / 2)))

	return backoff + jitter
}

// IsRetryable determines if an error should trigger a retry
func IsRetryable(err error) bool {
	if err == nil {
		return false
	}

	errStr := strings.ToLower(err.Error())

	// Not retryable: client errors (bad request, auth, etc.)
	nonRetryable := []string{
		"400", "bad request",
		"401", "unauthorized",
		"403", "forbidden",
		"invalid",
		"schema validation",
		"budget",
	}

	for _, s := range nonRetryable {
		if strings.Contains(errStr, s) {
			return false
		}
	}

	// Retryable: server errors, rate limits, timeouts, network issues
	retryable := []string{
		"429", "rate limit", "too many requests",
		"500", "502", "503", "504",
		"timeout", "deadline exceeded",
		"connection", "network",
		"temporary", "transient",
		"context canceled", // Might be from parent timeout, worth retrying
	}

	for _, s := range retryable {
		if strings.Contains(errStr, s) {
			return true
		}
	}

	// Default: retry on unknown errors (conservative)
	return true
}

// WithRetry is a convenience wrapper that creates a retryable function
func WithRetry(ctx context.Context, fn func() (string, int, error)) (string, int, error) {
	return Do(ctx, DefaultConfig(), fn)
}
