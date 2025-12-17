package config

import (
	"os"
	"path/filepath"
	"testing"
)

// createTempConfigFile creates a temporary config file for testing
func createTempConfigFile(t *testing.T, dir, content string) string {
	t.Helper()

	configDir := filepath.Join(dir, ".ai-cli")
	if err := os.MkdirAll(configDir, 0755); err != nil {
		t.Fatalf("failed to create config dir: %v", err)
	}

	configPath := filepath.Join(configDir, ConfigFileName)
	if err := os.WriteFile(configPath, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write config file: %v", err)
	}

	return configPath
}

// =============================================================================
// loadConfigFromPath Tests
// =============================================================================

func TestLoadConfigFromPath_ValidYAML(t *testing.T) {
	tmpDir := t.TempDir()
	configContent := `
provider: azure
model: gpt-4o

azure:
  endpoint: https://test.openai.azure.com
  api_key: test-key
  models:
    - gpt-4
    - gpt-4o

copilot:
  account_type: business
  models:
    - claude-3

web_search:
  provider: tavily

defaults:
  stream: true
  render: true
`
	configPath := createTempConfigFile(t, tmpDir, configContent)

	cfg, err := loadConfigFromPath(configPath)
	if err != nil {
		t.Fatalf("loadConfigFromPath() error = %v", err)
	}

	// Verify parsed values
	if cfg.Provider != "azure" {
		t.Errorf("Provider = %q, want %q", cfg.Provider, "azure")
	}
	if cfg.Model != "gpt-4o" {
		t.Errorf("Model = %q, want %q", cfg.Model, "gpt-4o")
	}
	if cfg.Azure == nil {
		t.Fatal("Azure config should not be nil")
	}
	if cfg.Azure.Endpoint != "https://test.openai.azure.com" {
		t.Errorf("Azure.Endpoint = %q, want %q", cfg.Azure.Endpoint, "https://test.openai.azure.com")
	}
	if len(cfg.Azure.Models) != 2 {
		t.Errorf("Azure.Models length = %d, want 2", len(cfg.Azure.Models))
	}
	if cfg.Copilot == nil {
		t.Fatal("Copilot config should not be nil")
	}
	if cfg.Copilot.AccountType != "business" {
		t.Errorf("Copilot.AccountType = %q, want %q", cfg.Copilot.AccountType, "business")
	}
	if cfg.WebSearch == nil {
		t.Fatal("WebSearch config should not be nil")
	}
	if cfg.WebSearch.Provider != "tavily" {
		t.Errorf("WebSearch.Provider = %q, want %q", cfg.WebSearch.Provider, "tavily")
	}
	if cfg.Defaults == nil {
		t.Fatal("Defaults config should not be nil")
	}
	if !cfg.Defaults.Stream {
		t.Error("Defaults.Stream should be true")
	}
	if !cfg.Defaults.Render {
		t.Error("Defaults.Render should be true")
	}
}

func TestLoadConfigFromPath_InvalidYAML(t *testing.T) {
	tmpDir := t.TempDir()
	invalidContent := `
provider: azure
model: [invalid yaml
  - this is broken
`
	configPath := createTempConfigFile(t, tmpDir, invalidContent)

	_, err := loadConfigFromPath(configPath)
	if err == nil {
		t.Error("loadConfigFromPath() should return error for invalid YAML")
	}
}

func TestLoadConfigFromPath_NotFound(t *testing.T) {
	_, err := loadConfigFromPath("/nonexistent/path/config.yaml")
	if err == nil {
		t.Error("loadConfigFromPath() should return error for non-existent file")
	}
}

func TestLoadConfigFromPath_EmptyFile(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := createTempConfigFile(t, tmpDir, "")

	cfg, err := loadConfigFromPath(configPath)
	if err != nil {
		t.Fatalf("loadConfigFromPath() error = %v", err)
	}

	// Should return empty FileConfig
	if cfg.Provider != "" {
		t.Errorf("Provider should be empty, got %q", cfg.Provider)
	}
}

func TestLoadConfigFromPath_PartialConfig(t *testing.T) {
	tmpDir := t.TempDir()
	configContent := `
provider: copilot
# Only provider set, everything else omitted
`
	configPath := createTempConfigFile(t, tmpDir, configContent)

	cfg, err := loadConfigFromPath(configPath)
	if err != nil {
		t.Fatalf("loadConfigFromPath() error = %v", err)
	}

	if cfg.Provider != "copilot" {
		t.Errorf("Provider = %q, want %q", cfg.Provider, "copilot")
	}
	if cfg.Azure != nil {
		t.Error("Azure should be nil when not specified")
	}
	if cfg.Copilot != nil {
		t.Error("Copilot should be nil when not specified")
	}
}

// =============================================================================
// LoadConfigFile Tests
// =============================================================================

func TestLoadConfigFile_NoConfigFile(t *testing.T) {
	// Change to a temp directory with no config file
	tmpDir := t.TempDir()
	oldWd, _ := os.Getwd()
	os.Chdir(tmpDir)
	t.Cleanup(func() { os.Chdir(oldWd) })

	// Create an empty .ai-cli directory to ensure no config is loaded from current dir
	os.MkdirAll(filepath.Join(tmpDir, ".ai-cli"), 0755)

	cfg, err := LoadConfigFile()
	if err != nil {
		t.Fatalf("LoadConfigFile() error = %v", err)
	}

	// LoadConfigFile returns empty FileConfig when no file exists
	// It doesn't return error, just empty config
	if cfg == nil {
		t.Error("LoadConfigFile() should return non-nil config even when no file exists")
	}
}

func TestLoadConfigFile_CurrentDirectory(t *testing.T) {
	tmpDir := t.TempDir()
	configContent := `provider: copilot`
	createTempConfigFile(t, tmpDir, configContent)

	oldWd, _ := os.Getwd()
	os.Chdir(tmpDir)
	t.Cleanup(func() { os.Chdir(oldWd) })

	cfg, err := LoadConfigFile()
	if err != nil {
		t.Fatalf("LoadConfigFile() error = %v", err)
	}

	if cfg.Provider != "copilot" {
		t.Errorf("Provider = %q, want %q", cfg.Provider, "copilot")
	}
}

// =============================================================================
// GetConfigPaths Tests
// =============================================================================

func TestGetConfigPaths(t *testing.T) {
	paths := GetConfigPaths()

	if len(paths) == 0 {
		t.Error("GetConfigPaths() should return at least one path")
	}

	// First path should be current directory
	if paths[0] != filepath.Join(".", ".ai-cli", ConfigFileName) {
		t.Errorf("First path = %q, want current directory path", paths[0])
	}

	// All paths should end with config.yaml
	for i, p := range paths {
		if filepath.Base(p) != ConfigFileName {
			t.Errorf("Path %d = %q, should end with %q", i, p, ConfigFileName)
		}
	}
}

// =============================================================================
// ApplyFileConfig Tests
// =============================================================================

func TestConfig_ApplyFileConfig_Nil(t *testing.T) {
	cfg := NewConfig()
	cfg.Provider = "existing"

	// Should not panic
	cfg.ApplyFileConfig(nil)

	if cfg.Provider != "existing" {
		t.Error("ApplyFileConfig(nil) should not modify config")
	}
}

func TestConfig_ApplyFileConfig_Provider(t *testing.T) {
	tests := []struct {
		name           string
		existingValue  string
		fileValue      string
		expectedResult string
	}{
		{
			name:           "set when empty",
			existingValue:  "",
			fileValue:      "azure",
			expectedResult: "azure",
		},
		{
			name:           "no overwrite when set",
			existingValue:  "copilot",
			fileValue:      "azure",
			expectedResult: "copilot", // Should NOT change
		},
		{
			name:           "empty file value",
			existingValue:  "",
			fileValue:      "",
			expectedResult: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := NewConfig()
			cfg.Provider = tt.existingValue

			fc := &FileConfig{Provider: tt.fileValue}
			cfg.ApplyFileConfig(fc)

			if cfg.Provider != tt.expectedResult {
				t.Errorf("Provider = %q, want %q", cfg.Provider, tt.expectedResult)
			}
		})
	}
}

func TestConfig_ApplyFileConfig_Model(t *testing.T) {
	cfg := NewConfig()
	cfg.Model = ""

	fc := &FileConfig{Model: "gpt-4o"}
	cfg.ApplyFileConfig(fc)

	if cfg.Model != "gpt-4o" {
		t.Errorf("Model = %q, want %q", cfg.Model, "gpt-4o")
	}
}

func TestConfig_ApplyFileConfig_Azure(t *testing.T) {
	cfg := NewConfig()

	fc := &FileConfig{
		Azure: &AzureConfig{
			Endpoint: "https://test.azure.com",
			APIKey:   "test-key",
			Models:   []string{"gpt-4", "gpt-4o"},
		},
	}
	cfg.ApplyFileConfig(fc)

	if cfg.AzureEndpoint != "https://test.azure.com" {
		t.Errorf("AzureEndpoint = %q, want %q", cfg.AzureEndpoint, "https://test.azure.com")
	}
	if cfg.AzureAPIKey != "test-key" {
		t.Errorf("AzureAPIKey = %q, want %q", cfg.AzureAPIKey, "test-key")
	}
	if len(cfg.AzureModels) != 2 {
		t.Errorf("AzureModels length = %d, want 2", len(cfg.AzureModels))
	}
}

func TestConfig_ApplyFileConfig_Azure_NoOverwrite(t *testing.T) {
	cfg := NewConfig()
	cfg.AzureEndpoint = "https://existing.azure.com"
	cfg.AzureAPIKey = "existing-key"

	fc := &FileConfig{
		Azure: &AzureConfig{
			Endpoint: "https://new.azure.com",
			APIKey:   "new-key",
		},
	}
	cfg.ApplyFileConfig(fc)

	// Should NOT overwrite existing values
	if cfg.AzureEndpoint != "https://existing.azure.com" {
		t.Errorf("AzureEndpoint = %q, should not be overwritten", cfg.AzureEndpoint)
	}
	if cfg.AzureAPIKey != "existing-key" {
		t.Errorf("AzureAPIKey = %q, should not be overwritten", cfg.AzureAPIKey)
	}
}

func TestConfig_ApplyFileConfig_Copilot(t *testing.T) {
	cfg := NewConfig()

	fc := &FileConfig{
		Copilot: &CopilotConfig{
			AccountType: "enterprise",
			Models:      []string{"custom-model"},
		},
	}
	cfg.ApplyFileConfig(fc)

	if cfg.AccountType != "enterprise" {
		t.Errorf("AccountType = %q, want %q", cfg.AccountType, "enterprise")
	}
	if len(cfg.CopilotModels) != 1 || cfg.CopilotModels[0] != "custom-model" {
		t.Errorf("CopilotModels = %v, want [custom-model]", cfg.CopilotModels)
	}
}

func TestConfig_ApplyFileConfig_WebSearch(t *testing.T) {
	cfg := NewConfig()

	fc := &FileConfig{
		WebSearch: &WebSearchConfig{
			Provider: "brave",
		},
	}
	cfg.ApplyFileConfig(fc)

	if cfg.WebSearchProvider != "brave" {
		t.Errorf("WebSearchProvider = %q, want %q", cfg.WebSearchProvider, "brave")
	}
}

func TestConfig_ApplyFileConfig_Defaults_Stream(t *testing.T) {
	cfg := NewConfig()
	cfg.Stream = false

	fc := &FileConfig{
		Defaults: &DefaultsConfig{
			Stream: true,
		},
	}
	cfg.ApplyFileConfig(fc)

	if !cfg.Stream {
		t.Error("Stream should be true after applying defaults")
	}
}

func TestConfig_ApplyFileConfig_Defaults_NoOverwrite(t *testing.T) {
	cfg := NewConfig()
	cfg.Stream = true // Already set

	fc := &FileConfig{
		Defaults: &DefaultsConfig{
			Stream: false, // File says false
		},
	}
	cfg.ApplyFileConfig(fc)

	// Should remain true (flag takes precedence)
	// Note: Current implementation only applies "true" values from defaults
	if !cfg.Stream {
		t.Error("Stream should remain true (flag precedence)")
	}
}

func TestConfig_ApplyFileConfig_Defaults_AllFlags(t *testing.T) {
	cfg := NewConfig()

	fc := &FileConfig{
		Defaults: &DefaultsConfig{
			Stream:    true,
			Render:    true,
			WebSearch: true,
			Citations: true,
		},
	}
	cfg.ApplyFileConfig(fc)

	if !cfg.Stream {
		t.Error("Stream should be true")
	}
	if !cfg.Render {
		t.Error("Render should be true")
	}
	if !cfg.WebSearch {
		t.Error("WebSearch should be true")
	}
	if !cfg.Citations {
		t.Error("Citations should be true")
	}
}

// =============================================================================
// CreateDefaultConfigFile Tests
// =============================================================================

func TestCreateDefaultConfigFile_Success(t *testing.T) {
	// Use a custom temp directory to avoid affecting real config
	tmpDir := t.TempDir()

	// Override home directory for test
	oldHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	t.Cleanup(func() {
		os.Setenv("HOME", oldHome)
	})

	// Also set XDG_CONFIG_HOME to control where config goes
	oldXdg := os.Getenv("XDG_CONFIG_HOME")
	os.Setenv("XDG_CONFIG_HOME", "")
	t.Cleanup(func() {
		if oldXdg != "" {
			os.Setenv("XDG_CONFIG_HOME", oldXdg)
		} else {
			os.Unsetenv("XDG_CONFIG_HOME")
		}
	})

	path, err := CreateDefaultConfigFile()
	if err != nil {
		t.Fatalf("CreateDefaultConfigFile() error = %v", err)
	}

	// Verify file was created
	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Errorf("Config file was not created at %s", path)
	}

	// Verify content is valid YAML
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("Failed to read created config file: %v", err)
	}
	if len(content) == 0 {
		t.Error("Created config file is empty")
	}
}

func TestCreateDefaultConfigFile_AlreadyExists(t *testing.T) {
	tmpDir := t.TempDir()

	// Create existing config
	configDir := filepath.Join(tmpDir, "ai-cli")
	os.MkdirAll(configDir, 0755)
	existingPath := filepath.Join(configDir, ConfigFileName)
	os.WriteFile(existingPath, []byte("existing content"), 0644)

	// Override config dir
	oldXdg := os.Getenv("XDG_CONFIG_HOME")
	os.Setenv("XDG_CONFIG_HOME", tmpDir)
	t.Cleanup(func() {
		if oldXdg != "" {
			os.Setenv("XDG_CONFIG_HOME", oldXdg)
		} else {
			os.Unsetenv("XDG_CONFIG_HOME")
		}
	})

	_, err := CreateDefaultConfigFile()
	if err == nil {
		t.Error("CreateDefaultConfigFile() should return error when file exists")
	}
}
