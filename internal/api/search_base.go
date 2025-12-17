package api

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/quocvuong92/ai-cli/internal/config"
)

// DefaultSearchTimeout is the default timeout for search requests
const DefaultSearchTimeout = 30 * time.Second

// BaseSearchClient provides common functionality for search clients
type BaseSearchClient struct {
	HTTPClient    *http.Client
	KeyRotator    *config.KeyRotator
	ProviderName  string
	OnKeyRotation KeyRotationCallback
}

// NewBaseSearchClient creates a new base search client
func NewBaseSearchClient(keyRotator *config.KeyRotator, providerName string) *BaseSearchClient {
	return &BaseSearchClient{
		HTTPClient: &http.Client{
			Timeout: DefaultSearchTimeout,
		},
		KeyRotator:   keyRotator,
		ProviderName: providerName,
	}
}

// SetKeyRotationCallback sets a callback function for key rotation events
func (b *BaseSearchClient) SetKeyRotationCallback(callback KeyRotationCallback) {
	b.OnKeyRotation = callback
}

// GetCurrentKey returns the current API key
func (b *BaseSearchClient) GetCurrentKey() string {
	return b.KeyRotator.GetCurrentKey()
}

// RotateKey attempts to switch to the next available API key and notifies via callback
func (b *BaseSearchClient) RotateKey() error {
	oldIndex := b.KeyRotator.GetCurrentIndex()
	_, err := b.KeyRotator.Rotate()
	if err != nil {
		return err
	}

	if b.OnKeyRotation != nil {
		b.OnKeyRotation(oldIndex+1, b.KeyRotator.GetCurrentIndex()+1, b.KeyRotator.GetKeyCount())
	}

	return nil
}

// SearchFunc is a function type for performing a single search attempt
type SearchFunc[T any] func(ctx context.Context, query string) (T, error)

// SearchWithRetry performs search with automatic key rotation on failure.
// It uses generics to support different response types from different providers.
//
// Parameters:
//   - ctx: Context for cancellation
//   - query: Search query string
//   - base: Base search client with key rotation support
//   - doSearch: Provider-specific search function
//
// Returns the search result or an error after all retries are exhausted.
func SearchWithRetry[T any](
	ctx context.Context,
	query string,
	base *BaseSearchClient,
	doSearch SearchFunc[T],
) (T, error) {
	var zero T

	// If only one key or no keys, skip retry logic
	if base.KeyRotator.GetKeyCount() <= 1 {
		return doSearch(ctx, query)
	}

	var lastErr error
	for attempt := 0; attempt < MaxRetryAttempts; attempt++ {
		// Check for context cancellation
		if err := ctx.Err(); err != nil {
			return zero, fmt.Errorf("search cancelled: %w", err)
		}

		resp, err := doSearch(ctx, query)
		if err == nil {
			return resp, nil
		}
		lastErr = err

		// Check if error is retryable (should rotate key)
		apiErr, ok := err.(*APIError)
		if !ok || !ShouldRotateKey(apiErr.StatusCode) {
			return zero, err
		}

		// Try to rotate key
		if rotateErr := base.RotateKey(); rotateErr != nil {
			return zero, fmt.Errorf("%v (no more %s API keys available)", err, base.ProviderName)
		}

		// Apply backoff before retry (except for last attempt)
		if attempt < MaxRetryAttempts-1 {
			select {
			case <-ctx.Done():
				return zero, fmt.Errorf("search cancelled: %w", ctx.Err())
			case <-time.After(CalculateBackoff(attempt)):
			}
		}
	}

	return zero, fmt.Errorf("max retry attempts (%d) exceeded: %v", MaxRetryAttempts, lastErr)
}
