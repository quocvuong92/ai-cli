package auth

import (
	"context"
	"testing"
	"time"
)

func TestNewTokenManager(t *testing.T) {
	tm := NewTokenManager("test_github_token")
	if tm == nil {
		t.Fatal("NewTokenManager() returned nil")
	}
	if tm.httpClient == nil {
		t.Error("NewTokenManager().httpClient is nil")
	}
	if tm.githubToken != "test_github_token" {
		t.Errorf("NewTokenManager().githubToken = %q, want %q", tm.githubToken, "test_github_token")
	}
}

func TestTokenManager_GetCopilotToken_Cached(t *testing.T) {
	tm := NewTokenManager("test_github_token")

	// Manually set a cached token
	tm.copilotToken = "cached_copilot_token"
	tm.expiresAt = time.Now().Add(10 * time.Minute) // Valid for 10 more minutes

	ctx := context.Background()
	token, err := tm.GetCopilotToken(ctx)
	if err != nil {
		t.Fatalf("GetCopilotToken() unexpected error: %v", err)
	}

	if token != "cached_copilot_token" {
		t.Errorf("GetCopilotToken() = %q, want cached token", token)
	}
}

func TestTokenManager_StopAutoRefresh(t *testing.T) {
	tm := NewTokenManager("test_github_token")

	// Start auto refresh
	ctx := context.Background()
	tm.StartAutoRefresh(ctx)

	// Verify cancel function is set
	if tm.cancelRefresh == nil {
		t.Error("StartAutoRefresh() did not set cancelRefresh")
	}

	// Stop should not panic
	tm.StopAutoRefresh()

	// Cancel function should be nil after stop
	if tm.cancelRefresh != nil {
		t.Error("StopAutoRefresh() did not clear cancelRefresh")
	}

	// Calling stop again should not panic
	tm.StopAutoRefresh()
}

func TestTokenManager_StartAutoRefresh_ReplacesExisting(t *testing.T) {
	tm := NewTokenManager("test_github_token")

	// Start auto refresh twice
	ctx := context.Background()
	tm.StartAutoRefresh(ctx)

	// Store if first cancel was set
	firstCancelSet := tm.cancelRefresh != nil

	tm.StartAutoRefresh(ctx)

	// Both should have set a cancel function
	secondCancelSet := tm.cancelRefresh != nil

	if !firstCancelSet || !secondCancelSet {
		t.Error("StartAutoRefresh() should set cancelRefresh")
	}

	// Clean up
	tm.StopAutoRefresh()
}

func TestGetCopilotBaseURL(t *testing.T) {
	tests := []struct {
		name        string
		accountType string
		want        string
	}{
		{
			name:        "empty account type",
			accountType: "",
			want:        "https://api.githubcopilot.com",
		},
		{
			name:        "individual",
			accountType: "individual",
			want:        "https://api.githubcopilot.com",
		},
		{
			name:        "business",
			accountType: "business",
			want:        "https://api.business.githubcopilot.com",
		},
		{
			name:        "enterprise",
			accountType: "enterprise",
			want:        "https://api.enterprise.githubcopilot.com",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := GetCopilotBaseURL(tt.accountType)
			if got != tt.want {
				t.Errorf("GetCopilotBaseURL(%q) = %q, want %q", tt.accountType, got, tt.want)
			}
		})
	}
}

func TestBuildCopilotHeaders(t *testing.T) {
	token := "test_copilot_token"

	t.Run("without vision", func(t *testing.T) {
		headers := BuildCopilotHeaders(token, false)

		if headers["Authorization"] != "Bearer "+token {
			t.Errorf("Authorization header = %q, want %q", headers["Authorization"], "Bearer "+token)
		}
		if headers["Content-Type"] != "application/json" {
			t.Errorf("Content-Type header = %q, want %q", headers["Content-Type"], "application/json")
		}
		if _, exists := headers["Copilot-Vision-Request"]; exists {
			t.Error("Copilot-Vision-Request header should not be set when vision is disabled")
		}
	})

	t.Run("with vision", func(t *testing.T) {
		headers := BuildCopilotHeaders(token, true)

		if headers["Copilot-Vision-Request"] != "true" {
			t.Errorf("Copilot-Vision-Request header = %q, want %q", headers["Copilot-Vision-Request"], "true")
		}
	})

	t.Run("required headers present", func(t *testing.T) {
		headers := BuildCopilotHeaders(token, false)

		requiredHeaders := []string{
			"Authorization",
			"Content-Type",
			"Copilot-Integration-Id",
			"Editor-Version",
			"Editor-Plugin-Version",
			"User-Agent",
			"Openai-Intent",
			"X-Github-Api-Version",
		}

		for _, h := range requiredHeaders {
			if _, exists := headers[h]; !exists {
				t.Errorf("Required header %q is missing", h)
			}
		}
	})
}

func TestCopilotConstants(t *testing.T) {
	// Verify constants are set correctly
	if CopilotTokenURL == "" {
		t.Error("CopilotTokenURL is empty")
	}
	if CopilotVersion == "" {
		t.Error("CopilotVersion is empty")
	}
	if EditorPluginVersion == "" {
		t.Error("EditorPluginVersion is empty")
	}
	if UserAgent == "" {
		t.Error("UserAgent is empty")
	}
	if APIVersion == "" {
		t.Error("APIVersion is empty")
	}
}
