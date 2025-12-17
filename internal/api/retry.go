package api

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/quocvuong92/ai-cli/internal/config"
)

// Package api provides HTTP clients for AI providers with retry logic.

// Retry configuration constants
const (
	MaxRetryAttempts  = 5
	InitialBackoff    = 100 * time.Millisecond
	MaxBackoff        = 2 * time.Second
	BackoffMultiplier = 2.0

	// AI API retry configuration (separate from search retry)
	MaxAPIRetryAttempts = 3
	APIInitialBackoff   = 500 * time.Millisecond
	APIMaxBackoff       = 5 * time.Second
)

// RetryableStatusCodes are HTTP status codes that should trigger a retry for AI API calls
var RetryableStatusCodes = []int{
	http.StatusTooManyRequests,     // 429 - Rate limited
	http.StatusServiceUnavailable,  // 503 - Service unavailable
	http.StatusGatewayTimeout,      // 504 - Gateway timeout
	http.StatusBadGateway,          // 502 - Bad gateway
	http.StatusInternalServerError, // 500 - Internal server error (transient)
}

// ShouldRotateKey checks if the error status code indicates we should try another key
func ShouldRotateKey(statusCode int) bool {
	for _, code := range config.RotatableErrorCodes {
		if statusCode == code {
			return true
		}
	}
	return false
}

// ShouldRetryAPICall checks if the error status code indicates we should retry the API call
func ShouldRetryAPICall(statusCode int) bool {
	for _, code := range RetryableStatusCodes {
		if statusCode == code {
			return true
		}
	}
	return false
}

// CalculateBackoff returns the backoff duration for a given attempt number
func CalculateBackoff(attempt int) time.Duration {
	backoff := InitialBackoff
	for i := 0; i < attempt; i++ {
		backoff = time.Duration(float64(backoff) * BackoffMultiplier)
		if backoff > MaxBackoff {
			backoff = MaxBackoff
			break
		}
	}
	return backoff
}

// CalculateAPIBackoff returns the backoff duration for AI API retry attempts
func CalculateAPIBackoff(attempt int) time.Duration {
	backoff := APIInitialBackoff
	for i := 0; i < attempt; i++ {
		backoff = time.Duration(float64(backoff) * BackoffMultiplier)
		if backoff > APIMaxBackoff {
			backoff = APIMaxBackoff
			break
		}
	}
	return backoff
}

// RetryableFunc is a function that can be retried
type RetryableFunc[T any] func() (T, error)

// WithRetry executes a function with retry logic for transient failures.
// It retries on retryable HTTP status codes (429, 500, 502, 503, 504)
// with exponential backoff between attempts.
func WithRetry[T any](ctx context.Context, fn RetryableFunc[T]) (T, error) {
	var lastErr error
	var zero T

	for attempt := 0; attempt < MaxAPIRetryAttempts; attempt++ {
		// Check for context cancellation
		if err := ctx.Err(); err != nil {
			return zero, fmt.Errorf("operation cancelled: %w", err)
		}

		result, err := fn()
		if err == nil {
			return result, nil
		}
		lastErr = err

		// Check if error is retryable
		if apiErr, ok := err.(*APIError); ok {
			if !ShouldRetryAPICall(apiErr.StatusCode) {
				// Non-retryable error, return immediately
				return zero, err
			}
		} else {
			// Non-API error, don't retry
			return zero, err
		}

		// Apply backoff before retry (except for last attempt)
		if attempt < MaxAPIRetryAttempts-1 {
			select {
			case <-ctx.Done():
				return zero, fmt.Errorf("operation cancelled: %w", ctx.Err())
			case <-time.After(CalculateAPIBackoff(attempt)):
			}
		}
	}

	return zero, fmt.Errorf("max retry attempts (%d) exceeded: %w", MaxAPIRetryAttempts, lastErr)
}

// StreamRetryableFunc is a function that returns an HTTP response for streaming.
// The caller is responsible for closing the response body on success.
type StreamRetryableFunc func() (*http.Response, error)

// WithStreamRetry executes a streaming request with retry logic for transient failures.
// It retries the initial connection on retryable HTTP status codes, then processes
// the SSE stream using the provided callbacks. Once streaming starts, retries are
// not attempted (partial responses cannot be safely retried).
func WithStreamRetry(ctx context.Context, fn StreamRetryableFunc, onChunk func(content string), onDone func(resp *ChatResponse)) error {
	var lastErr error

	for attempt := 0; attempt < MaxAPIRetryAttempts; attempt++ {
		// Check for context cancellation
		if err := ctx.Err(); err != nil {
			return fmt.Errorf("operation cancelled: %w", err)
		}

		resp, err := fn()
		if err == nil {
			// Successfully connected, process the stream
			defer func() { _ = resp.Body.Close() }()

			processor := NewSSEProcessor(resp.Body)
			if err := processor.Process(ctx, onChunk); err != nil {
				return fmt.Errorf("failed to process stream: %w", err)
			}

			// Build and return final response
			if onDone != nil {
				onDone(processor.BuildResponse())
			}
			return nil
		}
		lastErr = err

		// Check if error is retryable
		if apiErr, ok := err.(*APIError); ok {
			if !ShouldRetryAPICall(apiErr.StatusCode) {
				// Non-retryable error, return immediately
				return err
			}
		} else {
			// Non-API error, don't retry
			return err
		}

		// Apply backoff before retry (except for last attempt)
		if attempt < MaxAPIRetryAttempts-1 {
			select {
			case <-ctx.Done():
				return fmt.Errorf("operation cancelled: %w", ctx.Err())
			case <-time.After(CalculateAPIBackoff(attempt)):
			}
		}
	}

	return fmt.Errorf("max retry attempts (%d) exceeded: %w", MaxAPIRetryAttempts, lastErr)
}
