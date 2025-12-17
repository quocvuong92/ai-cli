package api

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/quocvuong92/ai-cli/internal/config"
)

// mockKeyRotator creates a KeyRotator with test keys
func mockKeyRotator(t *testing.T, keys string) *config.KeyRotator {
	t.Helper()
	// Set env var temporarily
	envKey := "TEST_SEARCH_KEYS_" + t.Name()
	t.Setenv(envKey, keys)
	return config.NewKeyRotator(envKey)
}

func TestNewBaseSearchClient(t *testing.T) {
	kr := mockKeyRotator(t, "key1,key2")
	client := NewBaseSearchClient(kr, "TestProvider")

	if client.HTTPClient == nil {
		t.Error("HTTPClient should not be nil")
	}
	if client.KeyRotator != kr {
		t.Error("KeyRotator not set correctly")
	}
	if client.ProviderName != "TestProvider" {
		t.Errorf("ProviderName = %q, want %q", client.ProviderName, "TestProvider")
	}
	if client.HTTPClient.Timeout != DefaultSearchTimeout {
		t.Errorf("HTTPClient.Timeout = %v, want %v", client.HTTPClient.Timeout, DefaultSearchTimeout)
	}
}

func TestBaseSearchClient_GetCurrentKey(t *testing.T) {
	kr := mockKeyRotator(t, "key1,key2,key3")
	client := NewBaseSearchClient(kr, "TestProvider")

	if key := client.GetCurrentKey(); key != "key1" {
		t.Errorf("GetCurrentKey() = %q, want %q", key, "key1")
	}
}

func TestBaseSearchClient_RotateKey(t *testing.T) {
	kr := mockKeyRotator(t, "key1,key2,key3")
	client := NewBaseSearchClient(kr, "TestProvider")

	// Track callback calls
	var callbackCalls []struct {
		from, to, total int
	}
	client.SetKeyRotationCallback(func(from, to, total int) {
		callbackCalls = append(callbackCalls, struct{ from, to, total int }{from, to, total})
	})

	// Rotate to key2
	err := client.RotateKey()
	if err != nil {
		t.Fatalf("RotateKey() error = %v", err)
	}
	if key := client.GetCurrentKey(); key != "key2" {
		t.Errorf("GetCurrentKey() after rotate = %q, want %q", key, "key2")
	}

	// Verify callback was called
	if len(callbackCalls) != 1 {
		t.Fatalf("callback called %d times, want 1", len(callbackCalls))
	}
	if callbackCalls[0].from != 1 || callbackCalls[0].to != 2 || callbackCalls[0].total != 3 {
		t.Errorf("callback args = %+v, want {1, 2, 3}", callbackCalls[0])
	}
}

func TestBaseSearchClient_RotateKey_Exhausted(t *testing.T) {
	kr := mockKeyRotator(t, "key1")
	client := NewBaseSearchClient(kr, "TestProvider")

	err := client.RotateKey()
	if err == nil {
		t.Error("RotateKey() with single key should return error")
	}
}

func TestSearchWithRetry_Success(t *testing.T) {
	kr := mockKeyRotator(t, "key1,key2")
	client := NewBaseSearchClient(kr, "TestProvider")

	callCount := 0
	doSearch := func(ctx context.Context, query string) (string, error) {
		callCount++
		return "result", nil
	}

	result, err := SearchWithRetry(context.Background(), "test query", client, doSearch)
	if err != nil {
		t.Fatalf("SearchWithRetry() error = %v", err)
	}
	if result != "result" {
		t.Errorf("result = %q, want %q", result, "result")
	}
	if callCount != 1 {
		t.Errorf("doSearch called %d times, want 1", callCount)
	}
}

func TestSearchWithRetry_RetryOnRotatableError(t *testing.T) {
	kr := mockKeyRotator(t, "key1,key2,key3")
	client := NewBaseSearchClient(kr, "TestProvider")

	callCount := 0
	doSearch := func(ctx context.Context, query string) (string, error) {
		callCount++
		if callCount < 3 {
			// Return retryable error (429 - rate limited)
			return "", &APIError{StatusCode: 429, Message: "rate limited"}
		}
		return "success", nil
	}

	result, err := SearchWithRetry(context.Background(), "test query", client, doSearch)
	if err != nil {
		t.Fatalf("SearchWithRetry() error = %v", err)
	}
	if result != "success" {
		t.Errorf("result = %q, want %q", result, "success")
	}
	if callCount != 3 {
		t.Errorf("doSearch called %d times, want 3", callCount)
	}
}

func TestSearchWithRetry_NonRetryableError(t *testing.T) {
	kr := mockKeyRotator(t, "key1,key2")
	client := NewBaseSearchClient(kr, "TestProvider")

	doSearch := func(ctx context.Context, query string) (string, error) {
		// Return non-retryable error (400 - bad request)
		return "", &APIError{StatusCode: 400, Message: "bad request"}
	}

	_, err := SearchWithRetry(context.Background(), "test query", client, doSearch)
	if err == nil {
		t.Fatal("SearchWithRetry() should return error for non-retryable status")
	}
	apiErr, ok := err.(*APIError)
	if !ok {
		t.Fatalf("error should be *APIError, got %T", err)
	}
	if apiErr.StatusCode != 400 {
		t.Errorf("error status = %d, want 400", apiErr.StatusCode)
	}
}

func TestSearchWithRetry_NonAPIError(t *testing.T) {
	kr := mockKeyRotator(t, "key1,key2")
	client := NewBaseSearchClient(kr, "TestProvider")

	doSearch := func(ctx context.Context, query string) (string, error) {
		return "", errors.New("network error")
	}

	_, err := SearchWithRetry(context.Background(), "test query", client, doSearch)
	if err == nil {
		t.Fatal("SearchWithRetry() should return error")
	}
	if err.Error() != "network error" {
		t.Errorf("error = %q, want %q", err.Error(), "network error")
	}
}

func TestSearchWithRetry_KeysExhausted(t *testing.T) {
	kr := mockKeyRotator(t, "key1,key2")
	client := NewBaseSearchClient(kr, "TestProvider")

	callCount := 0
	doSearch := func(ctx context.Context, query string) (string, error) {
		callCount++
		return "", &APIError{StatusCode: 429, Message: "rate limited"}
	}

	_, err := SearchWithRetry(context.Background(), "test query", client, doSearch)
	if err == nil {
		t.Fatal("SearchWithRetry() should return error when keys exhausted")
	}
	// After 2 keys exhausted, error should mention no more keys
	if callCount != 2 {
		t.Errorf("doSearch called %d times, want 2 (one per key)", callCount)
	}
}

func TestSearchWithRetry_SingleKey_NoRetry(t *testing.T) {
	kr := mockKeyRotator(t, "key1")
	client := NewBaseSearchClient(kr, "TestProvider")

	callCount := 0
	doSearch := func(ctx context.Context, query string) (string, error) {
		callCount++
		return "result", nil
	}

	result, err := SearchWithRetry(context.Background(), "test query", client, doSearch)
	if err != nil {
		t.Fatalf("SearchWithRetry() error = %v", err)
	}
	if result != "result" {
		t.Errorf("result = %q, want %q", result, "result")
	}
	if callCount != 1 {
		t.Errorf("doSearch called %d times, want 1", callCount)
	}
}

func TestSearchWithRetry_ContextCancellation(t *testing.T) {
	kr := mockKeyRotator(t, "key1,key2")
	client := NewBaseSearchClient(kr, "TestProvider")

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	doSearch := func(ctx context.Context, query string) (string, error) {
		return "result", nil
	}

	_, err := SearchWithRetry(ctx, "test query", client, doSearch)
	if err == nil {
		t.Fatal("SearchWithRetry() should return error when context cancelled")
	}
}

func TestSearchWithRetry_ContextCancellationDuringBackoff(t *testing.T) {
	kr := mockKeyRotator(t, "key1,key2,key3")
	client := NewBaseSearchClient(kr, "TestProvider")

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	callCount := 0
	doSearch := func(ctx context.Context, query string) (string, error) {
		callCount++
		return "", &APIError{StatusCode: 429, Message: "rate limited"}
	}

	_, err := SearchWithRetry(ctx, "test query", client, doSearch)
	if err == nil {
		t.Fatal("SearchWithRetry() should return error when context cancelled during backoff")
	}
	// Should have tried at least once before timeout
	if callCount < 1 {
		t.Errorf("doSearch should have been called at least once, called %d times", callCount)
	}
}
