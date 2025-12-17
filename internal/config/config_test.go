package config

import (
	"os"
	"path/filepath"
	"testing"
)

// Helper to set environment variable for test and restore after
func setEnvForTest(t *testing.T, key, value string) {
	t.Helper()
	old, existed := os.LookupEnv(key)
	if err := os.Setenv(key, value); err != nil {
		t.Fatalf("failed to set env %s: %v", key, err)
	}
	t.Cleanup(func() {
		if existed {
			os.Setenv(key, old)
		} else {
			os.Unsetenv(key)
		}
	})
}

// Helper to unset environment variable for test and restore after
func unsetEnvForTest(t *testing.T, key string) {
	t.Helper()
	old, existed := os.LookupEnv(key)
	os.Unsetenv(key)
	t.Cleanup(func() {
		if existed {
			os.Setenv(key, old)
		}
	})
}

// clearAllEnvVars clears all config-related environment variables for clean tests
func clearAllEnvVars(t *testing.T) {
	t.Helper()
	envVars := []string{
		EnvAzureEndpoint, EnvAzureAPIKey, EnvAzureModels,
		EnvCopilotAccountType, EnvCopilotModels,
		EnvAIProvider,
		EnvTavilyAPIKeys, EnvLinkupAPIKeys, EnvBraveAPIKeys,
		EnvWebSearchProvider,
	}
	for _, env := range envVars {
		unsetEnvForTest(t, env)
	}
}

// runInTempDir runs the test in a temporary directory to isolate from config files
func runInTempDir(t *testing.T) {
	t.Helper()
	tmpDir := t.TempDir()
	oldWd, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get working dir: %v", err)
	}
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("failed to change to temp dir: %v", err)
	}
	t.Cleanup(func() {
		os.Chdir(oldWd)
	})

	// Override HOME to prevent loading user config files
	oldHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	t.Cleanup(func() {
		os.Setenv("HOME", oldHome)
	})

	// Create empty .ai-cli and .config/ai-cli dirs to prevent loading from elsewhere
	os.MkdirAll(filepath.Join(tmpDir, ".ai-cli"), 0755)
	os.MkdirAll(filepath.Join(tmpDir, ".config", "ai-cli"), 0755)
}

// =============================================================================
// KeyRotator Tests
// =============================================================================

func TestNewKeyRotator_SingleKey(t *testing.T) {
	setEnvForTest(t, "TEST_KEYS", "key1")

	kr := NewKeyRotator("TEST_KEYS")

	if !kr.HasKeys() {
		t.Error("HasKeys() should return true")
	}
	if kr.GetKeyCount() != 1 {
		t.Errorf("GetKeyCount() = %d, want 1", kr.GetKeyCount())
	}
	if kr.GetCurrentKey() != "key1" {
		t.Errorf("GetCurrentKey() = %q, want %q", kr.GetCurrentKey(), "key1")
	}
	if kr.GetCurrentIndex() != 0 {
		t.Errorf("GetCurrentIndex() = %d, want 0", kr.GetCurrentIndex())
	}
}

func TestNewKeyRotator_MultipleKeys(t *testing.T) {
	setEnvForTest(t, "TEST_KEYS", "key1,key2,key3")

	kr := NewKeyRotator("TEST_KEYS")

	if kr.GetKeyCount() != 3 {
		t.Errorf("GetKeyCount() = %d, want 3", kr.GetKeyCount())
	}
	if kr.GetCurrentKey() != "key1" {
		t.Errorf("GetCurrentKey() = %q, want %q", kr.GetCurrentKey(), "key1")
	}
}

func TestNewKeyRotator_EmptyEnvVar(t *testing.T) {
	unsetEnvForTest(t, "TEST_KEYS_EMPTY")

	kr := NewKeyRotator("TEST_KEYS_EMPTY")

	if kr.HasKeys() {
		t.Error("HasKeys() should return false for empty env var")
	}
	if kr.GetKeyCount() != 0 {
		t.Errorf("GetKeyCount() = %d, want 0", kr.GetKeyCount())
	}
	if kr.GetCurrentKey() != "" {
		t.Errorf("GetCurrentKey() = %q, want empty string", kr.GetCurrentKey())
	}
}

func TestNewKeyRotator_WhitespaceHandling(t *testing.T) {
	setEnvForTest(t, "TEST_KEYS", "  key1  ,  key2  , key3 ")

	kr := NewKeyRotator("TEST_KEYS")

	if kr.GetKeyCount() != 3 {
		t.Errorf("GetKeyCount() = %d, want 3", kr.GetKeyCount())
	}
	if kr.GetCurrentKey() != "key1" {
		t.Errorf("GetCurrentKey() = %q, want %q (trimmed)", kr.GetCurrentKey(), "key1")
	}
}

func TestNewKeyRotator_EmptyEntries(t *testing.T) {
	setEnvForTest(t, "TEST_KEYS", "key1,,key2,,,key3")

	kr := NewKeyRotator("TEST_KEYS")

	if kr.GetKeyCount() != 3 {
		t.Errorf("GetKeyCount() = %d, want 3 (empty entries filtered)", kr.GetKeyCount())
	}
}

func TestKeyRotator_Rotate_Success(t *testing.T) {
	setEnvForTest(t, "TEST_KEYS", "key1,key2,key3")

	kr := NewKeyRotator("TEST_KEYS")

	// Rotate to key2
	newKey, err := kr.Rotate()
	if err != nil {
		t.Fatalf("Rotate() returned error: %v", err)
	}
	if newKey != "key2" {
		t.Errorf("Rotate() returned %q, want %q", newKey, "key2")
	}
	if kr.GetCurrentKey() != "key2" {
		t.Errorf("GetCurrentKey() = %q, want %q", kr.GetCurrentKey(), "key2")
	}
	if kr.GetCurrentIndex() != 1 {
		t.Errorf("GetCurrentIndex() = %d, want 1", kr.GetCurrentIndex())
	}

	// Rotate to key3
	newKey, err = kr.Rotate()
	if err != nil {
		t.Fatalf("Rotate() returned error: %v", err)
	}
	if newKey != "key3" {
		t.Errorf("Rotate() returned %q, want %q", newKey, "key3")
	}
}

func TestKeyRotator_Rotate_Exhausted(t *testing.T) {
	setEnvForTest(t, "TEST_KEYS", "key1,key2")

	kr := NewKeyRotator("TEST_KEYS")

	// Rotate to key2
	_, err := kr.Rotate()
	if err != nil {
		t.Fatalf("First Rotate() returned error: %v", err)
	}

	// Try to rotate again - should fail
	_, err = kr.Rotate()
	if err != ErrNoAvailableKeys {
		t.Errorf("Rotate() error = %v, want ErrNoAvailableKeys", err)
	}
}

func TestKeyRotator_Rotate_SingleKey(t *testing.T) {
	setEnvForTest(t, "TEST_KEYS", "key1")

	kr := NewKeyRotator("TEST_KEYS")

	_, err := kr.Rotate()
	if err != ErrNoAvailableKeys {
		t.Errorf("Rotate() with single key error = %v, want ErrNoAvailableKeys", err)
	}
}

// =============================================================================
// Config.Validate() Tests
// =============================================================================

func TestConfig_Validate_AzureProvider_MissingEndpoint(t *testing.T) {
	runInTempDir(t)
	clearAllEnvVars(t)

	cfg := NewConfig()
	cfg.Provider = "azure"
	// No endpoint set

	err := cfg.Validate()
	if err != ErrEndpointNotFound {
		t.Errorf("Validate() error = %v, want ErrEndpointNotFound", err)
	}
}

func TestConfig_Validate_AzureProvider_MissingAPIKey(t *testing.T) {
	runInTempDir(t)
	clearAllEnvVars(t)

	cfg := NewConfig()
	cfg.Provider = "azure"
	cfg.AzureEndpoint = "https://test.openai.azure.com"
	// No API key set

	err := cfg.Validate()
	if err != ErrAPIKeyNotFound {
		t.Errorf("Validate() error = %v, want ErrAPIKeyNotFound", err)
	}
}

func TestConfig_Validate_AzureProvider_Valid(t *testing.T) {
	runInTempDir(t)
	clearAllEnvVars(t)

	cfg := NewConfig()
	cfg.Provider = "azure"
	cfg.AzureEndpoint = "https://test.openai.azure.com"
	cfg.AzureAPIKey = "test-api-key"
	cfg.AzureModels = []string{"gpt-4", "gpt-4o"}

	err := cfg.Validate()
	if err != nil {
		t.Errorf("Validate() error = %v, want nil", err)
	}
	if len(cfg.AvailableModels) != 2 {
		t.Errorf("AvailableModels length = %d, want 2", len(cfg.AvailableModels))
	}
}

func TestConfig_Validate_CopilotProvider(t *testing.T) {
	runInTempDir(t)
	clearAllEnvVars(t)

	cfg := NewConfig()
	cfg.Provider = "copilot"

	err := cfg.Validate()
	if err != nil {
		t.Errorf("Validate() error = %v, want nil", err)
	}

	// Should use default Copilot models
	if len(cfg.AvailableModels) == 0 {
		t.Error("AvailableModels should not be empty for Copilot provider")
	}
	if cfg.AccountType != DefaultAccountType {
		t.Errorf("AccountType = %q, want %q", cfg.AccountType, DefaultAccountType)
	}
}

func TestConfig_Validate_GithubProviderAlias(t *testing.T) {
	runInTempDir(t)
	clearAllEnvVars(t)

	cfg := NewConfig()
	cfg.Provider = "github" // Should be treated same as "copilot"

	err := cfg.Validate()
	if err != nil {
		t.Errorf("Validate() error = %v, want nil", err)
	}

	if len(cfg.AvailableModels) == 0 {
		t.Error("AvailableModels should not be empty for 'github' provider")
	}
}

func TestConfig_Validate_InvalidModel(t *testing.T) {
	runInTempDir(t)
	clearAllEnvVars(t)

	cfg := NewConfig()
	cfg.Provider = "copilot"
	cfg.Model = "nonexistent-model"

	err := cfg.Validate()
	// Model should be kept as-is, user's choice is respected
	if err != nil {
		t.Errorf("Validate() error = %v, want nil", err)
	}
	if cfg.Model != "nonexistent-model" {
		t.Errorf("Model should be kept as user specified, got %s", cfg.Model)
	}
}

func TestConfig_Validate_WebSearchProvider_Invalid(t *testing.T) {
	runInTempDir(t)
	clearAllEnvVars(t)

	cfg := NewConfig()
	cfg.Provider = "copilot"
	cfg.WebSearchProvider = "invalid-provider"

	err := cfg.Validate()
	if err != ErrInvalidSearchProvider {
		t.Errorf("Validate() error = %v, want ErrInvalidSearchProvider", err)
	}
}

func TestConfig_Validate_WebSearch_NoKeys(t *testing.T) {
	runInTempDir(t)
	clearAllEnvVars(t)

	cfg := NewConfig()
	cfg.Provider = "copilot"
	cfg.WebSearch = true
	cfg.WebSearchProvider = "tavily"
	// No Tavily keys set

	err := cfg.Validate()
	if err != ErrWebSearchKeyNotFound {
		t.Errorf("Validate() error = %v, want ErrWebSearchKeyNotFound", err)
	}
}

func TestConfig_Validate_WebSearch_WithKeys(t *testing.T) {
	runInTempDir(t)
	clearAllEnvVars(t)
	setEnvForTest(t, EnvTavilyAPIKeys, "test-tavily-key")

	cfg := NewConfig()
	cfg.Provider = "copilot"
	cfg.WebSearch = true
	cfg.WebSearchProvider = "tavily"

	err := cfg.Validate()
	if err != nil {
		t.Errorf("Validate() error = %v, want nil", err)
	}
}

func TestConfig_Validate_WebSearchProvider_AutoDetect(t *testing.T) {
	runInTempDir(t)
	clearAllEnvVars(t)
	setEnvForTest(t, EnvBraveAPIKeys, "brave-key")

	cfg := NewConfig()
	cfg.Provider = "copilot"

	err := cfg.Validate()
	if err != nil {
		t.Errorf("Validate() error = %v, want nil", err)
	}

	// Should auto-detect Brave since only Brave keys are available
	if cfg.WebSearchProvider != "brave" {
		t.Errorf("WebSearchProvider = %q, want %q (auto-detected)", cfg.WebSearchProvider, "brave")
	}
}

func TestConfig_Validate_EnvVarLoading(t *testing.T) {
	runInTempDir(t)
	clearAllEnvVars(t)
	setEnvForTest(t, EnvAIProvider, "copilot")
	setEnvForTest(t, EnvCopilotAccountType, "business")
	setEnvForTest(t, EnvCopilotModels, "model1,model2")

	cfg := NewConfig()
	err := cfg.Validate()
	if err != nil {
		t.Errorf("Validate() error = %v, want nil", err)
	}

	if cfg.Provider != "copilot" {
		t.Errorf("Provider = %q, want %q", cfg.Provider, "copilot")
	}
	if cfg.AccountType != "business" {
		t.Errorf("AccountType = %q, want %q", cfg.AccountType, "business")
	}
	if len(cfg.CopilotModels) != 2 {
		t.Errorf("CopilotModels length = %d, want 2", len(cfg.CopilotModels))
	}
}

func TestConfig_Validate_AzureEndpointTrailingSlash(t *testing.T) {
	runInTempDir(t)
	clearAllEnvVars(t)

	cfg := NewConfig()
	cfg.Provider = "azure"
	cfg.AzureEndpoint = "https://test.openai.azure.com/"
	cfg.AzureAPIKey = "test-key"

	err := cfg.Validate()
	if err != nil {
		t.Errorf("Validate() error = %v, want nil", err)
	}

	// Trailing slash should be removed
	if cfg.AzureEndpoint != "https://test.openai.azure.com" {
		t.Errorf("AzureEndpoint = %q, want trailing slash removed", cfg.AzureEndpoint)
	}
}

// =============================================================================
// Helper Method Tests
// =============================================================================

func TestConfig_GetAzureAPIURL(t *testing.T) {
	cfg := &Config{
		AzureEndpoint: "https://myresource.openai.azure.com",
	}

	url := cfg.GetAzureAPIURL()
	expected := "https://myresource.openai.azure.com/openai/v1/chat/completions"

	if url != expected {
		t.Errorf("GetAzureAPIURL() = %q, want %q", url, expected)
	}
}

func TestConfig_ValidateModel(t *testing.T) {
	cfg := &Config{
		AvailableModels: []string{"gpt-4", "gpt-4o", "claude-3"},
	}

	tests := []struct {
		model    string
		expected bool
	}{
		{"gpt-4", true},
		{"gpt-4o", true},
		{"claude-3", true},
		{"nonexistent", false},
		{"GPT-4", false}, // Case sensitive
	}

	for _, tt := range tests {
		t.Run(tt.model, func(t *testing.T) {
			result := cfg.ValidateModel(tt.model)
			if result != tt.expected {
				t.Errorf("ValidateModel(%q) = %v, want %v", tt.model, result, tt.expected)
			}
		})
	}
}

func TestConfig_ValidateModel_EmptyAvailable(t *testing.T) {
	cfg := &Config{
		AvailableModels: []string{},
	}

	// Should return true when no models configured (no validation)
	if !cfg.ValidateModel("any-model") {
		t.Error("ValidateModel() should return true when AvailableModels is empty")
	}
}

func TestConfig_GetAvailableModelsString(t *testing.T) {
	tests := []struct {
		name     string
		models   []string
		expected string
	}{
		{
			name:     "multiple models",
			models:   []string{"gpt-4", "gpt-4o", "claude-3"},
			expected: "gpt-4, gpt-4o, claude-3",
		},
		{
			name:     "single model",
			models:   []string{"gpt-4"},
			expected: "gpt-4",
		},
		{
			name:     "empty models",
			models:   []string{},
			expected: "(not configured - set AZURE_OPENAI_MODELS)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &Config{AvailableModels: tt.models}
			result := cfg.GetAvailableModelsString()
			if result != tt.expected {
				t.Errorf("GetAvailableModelsString() = %q, want %q", result, tt.expected)
			}
		})
	}
}
