package api

import (
	"context"
	"errors"
	"net/http"
	"testing"
	"time"
)

func TestShouldRetryAPICall(t *testing.T) {
	tests := []struct {
		name       string
		statusCode int
		want       bool
	}{
		{"429 Too Many Requests", http.StatusTooManyRequests, true},
		{"500 Internal Server Error", http.StatusInternalServerError, true},
		{"502 Bad Gateway", http.StatusBadGateway, true},
		{"503 Service Unavailable", http.StatusServiceUnavailable, true},
		{"504 Gateway Timeout", http.StatusGatewayTimeout, true},
		{"400 Bad Request", http.StatusBadRequest, false},
		{"401 Unauthorized", http.StatusUnauthorized, false},
		{"403 Forbidden", http.StatusForbidden, false},
		{"404 Not Found", http.StatusNotFound, false},
		{"200 OK", http.StatusOK, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ShouldRetryAPICall(tt.statusCode)
			if got != tt.want {
				t.Errorf("ShouldRetryAPICall(%d) = %v, want %v", tt.statusCode, got, tt.want)
			}
		})
	}
}

func TestCalculateAPIBackoff(t *testing.T) {
	tests := []struct {
		name    string
		attempt int
		want    time.Duration
	}{
		{"attempt 0", 0, APIInitialBackoff},
		{"attempt 1", 1, APIInitialBackoff * 2},
		{"attempt 2", 2, APIInitialBackoff * 4},
		{"attempt large", 10, APIMaxBackoff}, // Should cap at max
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := CalculateAPIBackoff(tt.attempt)
			if got != tt.want {
				t.Errorf("CalculateAPIBackoff(%d) = %v, want %v", tt.attempt, got, tt.want)
			}
		})
	}
}

func TestCalculateBackoff(t *testing.T) {
	tests := []struct {
		name    string
		attempt int
		want    time.Duration
	}{
		{"attempt 0", 0, InitialBackoff},
		{"attempt 1", 1, InitialBackoff * 2},
		{"attempt 2", 2, InitialBackoff * 4},
		{"attempt large", 20, MaxBackoff}, // Should cap at max
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := CalculateBackoff(tt.attempt)
			if got != tt.want {
				t.Errorf("CalculateBackoff(%d) = %v, want %v", tt.attempt, got, tt.want)
			}
		})
	}
}

func TestWithRetry_Success(t *testing.T) {
	ctx := context.Background()
	callCount := 0

	result, err := WithRetry(ctx, func() (string, error) {
		callCount++
		return "success", nil
	})

	if err != nil {
		t.Errorf("WithRetry() unexpected error: %v", err)
	}
	if result != "success" {
		t.Errorf("WithRetry() = %v, want %v", result, "success")
	}
	if callCount != 1 {
		t.Errorf("WithRetry() called %d times, want 1", callCount)
	}
}

func TestWithRetry_RetryableError(t *testing.T) {
	ctx := context.Background()
	callCount := 0

	_, err := WithRetry(ctx, func() (string, error) {
		callCount++
		return "", &APIError{StatusCode: http.StatusTooManyRequests, Message: "rate limited"}
	})

	if err == nil {
		t.Error("WithRetry() expected error, got nil")
	}
	if callCount != MaxAPIRetryAttempts {
		t.Errorf("WithRetry() called %d times, want %d", callCount, MaxAPIRetryAttempts)
	}
}

func TestWithRetry_NonRetryableError(t *testing.T) {
	ctx := context.Background()
	callCount := 0

	_, err := WithRetry(ctx, func() (string, error) {
		callCount++
		return "", &APIError{StatusCode: http.StatusBadRequest, Message: "bad request"}
	})

	if err == nil {
		t.Error("WithRetry() expected error, got nil")
	}
	if callCount != 1 {
		t.Errorf("WithRetry() called %d times, want 1 (no retry for non-retryable)", callCount)
	}
}

func TestWithRetry_NonAPIError(t *testing.T) {
	ctx := context.Background()
	callCount := 0

	_, err := WithRetry(ctx, func() (string, error) {
		callCount++
		return "", errors.New("some other error")
	})

	if err == nil {
		t.Error("WithRetry() expected error, got nil")
	}
	if callCount != 1 {
		t.Errorf("WithRetry() called %d times, want 1 (no retry for non-API error)", callCount)
	}
}

func TestWithRetry_ContextCancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	_, err := WithRetry(ctx, func() (string, error) {
		return "success", nil
	})

	if err == nil {
		t.Error("WithRetry() expected error for cancelled context, got nil")
	}
}

func TestWithRetry_SuccessAfterRetry(t *testing.T) {
	ctx := context.Background()
	callCount := 0

	result, err := WithRetry(ctx, func() (string, error) {
		callCount++
		if callCount < 2 {
			return "", &APIError{StatusCode: http.StatusTooManyRequests, Message: "rate limited"}
		}
		return "success", nil
	})

	if err != nil {
		t.Errorf("WithRetry() unexpected error: %v", err)
	}
	if result != "success" {
		t.Errorf("WithRetry() = %v, want %v", result, "success")
	}
	if callCount != 2 {
		t.Errorf("WithRetry() called %d times, want 2", callCount)
	}
}

func TestAPIError_Error(t *testing.T) {
	err := &APIError{
		StatusCode: 429,
		Message:    "rate limited",
	}

	got := err.Error()
	want := "rate limited"
	if got != want {
		t.Errorf("APIError.Error() = %v, want %v", got, want)
	}
}
