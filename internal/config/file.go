package config

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// ConfigFileName is the name of the config file
const ConfigFileName = "config.yaml"

// FileConfig represents the configuration file structure
type FileConfig struct {
	// Provider selection
	Provider string `yaml:"provider,omitempty"` // "copilot", "azure"

	// Model settings
	Model string `yaml:"model,omitempty"`

	// Copilot settings
	Copilot *CopilotConfig `yaml:"copilot,omitempty"`

	// Azure settings
	Azure *AzureConfig `yaml:"azure,omitempty"`

	// Web search settings
	WebSearch *WebSearchConfig `yaml:"web_search,omitempty"`

	// Default flags
	Defaults *DefaultsConfig `yaml:"defaults,omitempty"`
}

// CopilotConfig holds GitHub Copilot-specific configuration
type CopilotConfig struct {
	AccountType string   `yaml:"account_type,omitempty"` // "individual", "business", "enterprise"
	Models      []string `yaml:"models,omitempty"`       // Additional models to allow
}

// AzureConfig holds Azure-specific configuration
type AzureConfig struct {
	Endpoint string   `yaml:"endpoint,omitempty"`
	APIKey   string   `yaml:"api_key,omitempty"`
	Models   []string `yaml:"models,omitempty"`
}

// WebSearchConfig holds web search configuration
type WebSearchConfig struct {
	Provider   string   `yaml:"provider,omitempty"` // "tavily", "linkup", "brave"
	TavilyKeys []string `yaml:"tavily_keys,omitempty"`
	LinkupKeys []string `yaml:"linkup_keys,omitempty"`
	BraveKeys  []string `yaml:"brave_keys,omitempty"`
}

// DefaultsConfig holds default flag values
type DefaultsConfig struct {
	Stream    bool `yaml:"stream,omitempty"`
	Render    bool `yaml:"render,omitempty"`
	WebSearch bool `yaml:"web_search,omitempty"`
	Citations bool `yaml:"citations,omitempty"`
}

// GetConfigPaths returns the paths to check for config files (in order of priority)
func GetConfigPaths() []string {
	var paths []string

	// 1. Current directory
	paths = append(paths, filepath.Join(".", ".ai-cli", ConfigFileName))

	// 2. User config directory
	if configDir, err := os.UserConfigDir(); err == nil {
		paths = append(paths, filepath.Join(configDir, "ai-cli", ConfigFileName))
	}

	// 3. Home directory
	if homeDir, err := os.UserHomeDir(); err == nil {
		paths = append(paths, filepath.Join(homeDir, ".config", "ai-cli", ConfigFileName))
	}

	return paths
}

// LoadConfigFile attempts to load configuration from a file
func LoadConfigFile() (*FileConfig, error) {
	paths := GetConfigPaths()

	for _, path := range paths {
		if _, err := os.Stat(path); err == nil {
			return loadConfigFromPath(path)
		}
	}

	// No config file found, return empty config
	return &FileConfig{}, nil
}

// loadConfigFromPath loads config from a specific path
func loadConfigFromPath(path string) (*FileConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file %s: %w", path, err)
	}

	var cfg FileConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("failed to parse config file %s: %w", path, err)
	}

	return &cfg, nil
}

// ApplyFileConfig applies file configuration to the main Config
// File config has lower priority than environment variables and CLI flags
func (c *Config) ApplyFileConfig(fc *FileConfig) {
	if fc == nil {
		return
	}

	// Provider (only if not set by env/flag)
	if c.Provider == "" && fc.Provider != "" {
		c.Provider = fc.Provider
	}

	// Model (only if not set by env/flag)
	if c.Model == "" && fc.Model != "" {
		c.Model = fc.Model
	}

	// Azure config
	if fc.Azure != nil {
		if c.AzureEndpoint == "" && fc.Azure.Endpoint != "" {
			c.AzureEndpoint = fc.Azure.Endpoint
		}
		if c.AzureAPIKey == "" && fc.Azure.APIKey != "" {
			c.AzureAPIKey = fc.Azure.APIKey
		}
		// Store Azure models from config file
		if len(fc.Azure.Models) > 0 {
			c.AzureModels = fc.Azure.Models
		}
	}

	// Copilot config
	if fc.Copilot != nil {
		if c.AccountType == "" && fc.Copilot.AccountType != "" {
			c.AccountType = fc.Copilot.AccountType
		}
		// Append models from config file to CopilotModels
		if len(fc.Copilot.Models) > 0 && len(c.CopilotModels) == 0 {
			c.CopilotModels = fc.Copilot.Models
		}
	}

	// Web search config
	if fc.WebSearch != nil {
		if c.WebSearchProvider == "" && fc.WebSearch.Provider != "" {
			c.WebSearchProvider = fc.WebSearch.Provider
		}
	}

	// Apply defaults (these are applied unless explicitly overridden by flags)
	if fc.Defaults != nil {
		// Note: These only apply if the flags weren't explicitly set
		// Since we can't distinguish between "flag not set" and "flag set to false",
		// we apply defaults only for "true" values in the config file
		if fc.Defaults.Stream && !c.Stream {
			c.Stream = true
		}
		if fc.Defaults.Render && !c.Render {
			c.Render = true
		}
		if fc.Defaults.WebSearch && !c.WebSearch {
			c.WebSearch = true
		}
		if fc.Defaults.Citations && !c.Citations {
			c.Citations = true
		}
	}
}

// CreateDefaultConfigFile creates a default config file at the user config directory
func CreateDefaultConfigFile() (string, error) {
	configDir, err := os.UserConfigDir()
	if err != nil {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("could not determine config directory: %w", err)
		}
		configDir = filepath.Join(homeDir, ".config")
	}

	dir := filepath.Join(configDir, "ai-cli")
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", fmt.Errorf("failed to create config directory: %w", err)
	}

	path := filepath.Join(dir, ConfigFileName)
	if _, err := os.Stat(path); err == nil {
		return path, fmt.Errorf("config file already exists at %s", path)
	}

	defaultConfig := `# AI CLI Configuration
# Location: ~/.config/ai-cli/config.yaml

# AI provider: "copilot" or "azure" (default: auto-detect)
# provider: copilot

# Default model to use (must be valid for your provider)
# model: gpt-4o

# GitHub Copilot settings
# copilot:
#   account_type: individual  # individual, business, or enterprise
#   models:  # Models you want to use (validates /model command)
#     - gpt-4o
#     - gpt-4.1
#     - gpt-5-mini
#     - claude-sonnet-4
#     - claude-opus-4.5
#     - gemini-2.5-pro
# Use 'ai-cli --list-models' to see default available models

# Azure OpenAI settings (required if provider: azure)
# azure:
#   endpoint: https://your-resource.openai.azure.com
#   api_key: your-api-key
#   models:  # List your deployed model names
#     - gpt-4
#     - gpt-4o

# Web search settings
# web_search:
#   provider: tavily  # tavily, linkup, or brave
#   tavily_keys:
#     - your-tavily-key
#   linkup_keys:
#     - your-linkup-key
#   brave_keys:
#     - your-brave-key

# Default flags for interactive mode
# defaults:
#   stream: true
#   render: true
#   web_search: false
#   citations: false
`

	if err := os.WriteFile(path, []byte(defaultConfig), 0600); err != nil {
		return "", fmt.Errorf("failed to write config file: %w", err)
	}

	return path, nil
}
