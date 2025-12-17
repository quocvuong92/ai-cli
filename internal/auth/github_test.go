package auth

import (
	"os"
	"path/filepath"
	"testing"
)

func TestGetTokenPath(t *testing.T) {
	path, err := GetTokenPath()
	if err != nil {
		t.Fatalf("GetTokenPath() unexpected error: %v", err)
	}

	if path == "" {
		t.Error("GetTokenPath() returned empty path")
	}

	// Should contain expected path components
	if !filepath.IsAbs(path) {
		t.Errorf("GetTokenPath() = %q, want absolute path", path)
	}

	expectedSuffix := filepath.Join(".local", "share", "ai-cli", "github-token")
	if !containsPath(path, expectedSuffix) {
		t.Errorf("GetTokenPath() = %q, want path containing %q", path, expectedSuffix)
	}
}

func TestSaveAndLoadGitHubToken(t *testing.T) {
	// Use a temporary directory for testing
	tempDir := t.TempDir()
	originalHome := os.Getenv("HOME")
	defer os.Setenv("HOME", originalHome)
	os.Setenv("HOME", tempDir)

	testToken := "ghp_testtoken123456"

	// Test saving
	err := SaveGitHubToken(testToken)
	if err != nil {
		t.Fatalf("SaveGitHubToken() unexpected error: %v", err)
	}

	// Test loading
	loaded, err := LoadGitHubToken()
	if err != nil {
		t.Fatalf("LoadGitHubToken() unexpected error: %v", err)
	}

	if loaded != testToken {
		t.Errorf("LoadGitHubToken() = %q, want %q", loaded, testToken)
	}
}

func TestLoadGitHubToken_NotExists(t *testing.T) {
	// Use a temporary directory for testing
	tempDir := t.TempDir()
	originalHome := os.Getenv("HOME")
	defer os.Setenv("HOME", originalHome)
	os.Setenv("HOME", tempDir)

	_, err := LoadGitHubToken()
	if err == nil {
		t.Error("LoadGitHubToken() expected error for non-existent token, got nil")
	}
}

func TestDeleteGitHubToken(t *testing.T) {
	// Use a temporary directory for testing
	tempDir := t.TempDir()
	originalHome := os.Getenv("HOME")
	defer os.Setenv("HOME", originalHome)
	os.Setenv("HOME", tempDir)

	// First save a token
	err := SaveGitHubToken("test_token")
	if err != nil {
		t.Fatalf("SaveGitHubToken() setup error: %v", err)
	}

	// Then delete it
	err = DeleteGitHubToken()
	if err != nil {
		t.Fatalf("DeleteGitHubToken() unexpected error: %v", err)
	}

	// Verify it's deleted
	_, err = LoadGitHubToken()
	if err == nil {
		t.Error("LoadGitHubToken() expected error after delete, got nil")
	}
}

func TestDeleteGitHubToken_NotExists(t *testing.T) {
	// Use a temporary directory for testing
	tempDir := t.TempDir()
	originalHome := os.Getenv("HOME")
	defer os.Setenv("HOME", originalHome)
	os.Setenv("HOME", tempDir)

	// Delete non-existent token should not error
	err := DeleteGitHubToken()
	if err != nil {
		t.Errorf("DeleteGitHubToken() unexpected error for non-existent token: %v", err)
	}
}

func TestIsLoggedIn(t *testing.T) {
	// Use a temporary directory for testing
	tempDir := t.TempDir()
	originalHome := os.Getenv("HOME")
	defer os.Setenv("HOME", originalHome)
	os.Setenv("HOME", tempDir)

	// Should return false when not logged in
	if IsLoggedIn() {
		t.Error("IsLoggedIn() = true, want false when no token")
	}

	// Save a token
	err := SaveGitHubToken("test_token")
	if err != nil {
		t.Fatalf("SaveGitHubToken() setup error: %v", err)
	}

	// Should return true when logged in
	if !IsLoggedIn() {
		t.Error("IsLoggedIn() = false, want true when token exists")
	}
}

func TestNewGitHubAuth(t *testing.T) {
	auth := NewGitHubAuth()
	if auth == nil {
		t.Error("NewGitHubAuth() returned nil")
	}
	if auth.httpClient == nil {
		t.Error("NewGitHubAuth().httpClient is nil")
	}
}

func TestGitHubConstants(t *testing.T) {
	// Verify constants are set correctly
	if GitHubClientID == "" {
		t.Error("GitHubClientID is empty")
	}
	if GitHubBaseURL == "" {
		t.Error("GitHubBaseURL is empty")
	}
	if GitHubAPIBaseURL == "" {
		t.Error("GitHubAPIBaseURL is empty")
	}
}

// containsPath checks if path contains the expected suffix
func containsPath(path, suffix string) bool {
	// Normalize paths for comparison
	cleanPath := filepath.Clean(path)
	cleanSuffix := filepath.Clean(suffix)
	return len(cleanPath) >= len(cleanSuffix) &&
		cleanPath[len(cleanPath)-len(cleanSuffix):] == cleanSuffix
}
