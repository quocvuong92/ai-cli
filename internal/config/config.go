package config

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/quocvuong92/ai-cli/internal/auth"
	"github.com/quocvuong92/ai-cli/internal/constants"
)

// Environment variable names
const (
	// Azure settings
	EnvAzureEndpoint = "AZURE_OPENAI_ENDPOINT"
	EnvAzureAPIKey   = "AZURE_OPENAI_API_KEY"
	EnvAzureModels   = "AZURE_OPENAI_MODELS"

	// Copilot settings
	EnvCopilotAccountType = "COPILOT_ACCOUNT_TYPE"
	EnvCopilotModels      = "COPILOT_MODELS"

	// Provider selection
	EnvAIProvider = "AI_PROVIDER"

	// Web search settings
	EnvTavilyAPIKeys     = "TAVILY_API_KEYS"
	EnvLinkupAPIKeys     = "LINKUP_API_KEYS"
	EnvBraveAPIKeys      = "BRAVE_API_KEYS"
	EnvWebSearchProvider = "WEB_SEARCH_PROVIDER"
)

// Defaults - re-exported from constants for convenience
const (
	DefaultModel          = constants.DefaultModel
	DefaultSystemMessage  = constants.DefaultSystemMessage
	DefaultSearchProvider = constants.DefaultSearchProvider
	DefaultAccountType    = constants.DefaultAccountType
	DefaultProvider       = "" // Auto-detect
)

// Timeout constants - re-exported from constants for convenience
const (
	DefaultAPITimeout     = constants.DefaultAPITimeout
	DefaultCommandTimeout = constants.DefaultCommandTimeout
	DefaultOAuthTimeout   = constants.DefaultOAuthTimeout
)

// DefaultCopilotModels - re-exported from constants for convenience
var DefaultCopilotModels = constants.DefaultCopilotModels

// Errors
var (
	ErrEndpointNotFound      = errors.New("Azure endpoint not found. Set AZURE_OPENAI_ENDPOINT environment variable")
	ErrAPIKeyNotFound        = errors.New("Azure API key not found. Set AZURE_OPENAI_API_KEY environment variable")
	ErrModelNotFound         = errors.New("model not found. Set AZURE_OPENAI_MODEL or use --model flag")
	ErrInvalidModel          = errors.New("invalid model specified")
	ErrNoAvailableKeys       = errors.New("all API keys exhausted")
	ErrWebSearchKeyNotFound  = errors.New("web search API key not found. Set TAVILY_API_KEYS, LINKUP_API_KEYS, or BRAVE_API_KEYS to use --web flag")
	ErrInvalidSearchProvider = errors.New("invalid search provider. Use 'tavily', 'linkup', or 'brave'")
)

// Error codes that should trigger key rotation
var RotatableErrorCodes = []int{401, 403, 429}

// KeyRotator manages a pool of API keys with rotation support
type KeyRotator struct {
	keys       []string
	currentIdx int
	currentKey string
}

// NewKeyRotator creates a new KeyRotator from an environment variable
func NewKeyRotator(envVar string) *KeyRotator {
	keys := getKeysFromEnv(envVar)
	kr := &KeyRotator{
		keys:       keys,
		currentIdx: 0,
	}
	if len(keys) > 0 {
		kr.currentKey = keys[0]
	}
	return kr
}

// GetCurrentKey returns the current active API key
func (kr *KeyRotator) GetCurrentKey() string {
	return kr.currentKey
}

// GetKeyCount returns the total number of keys
func (kr *KeyRotator) GetKeyCount() int {
	return len(kr.keys)
}

// GetCurrentIndex returns the current key index (0-based)
func (kr *KeyRotator) GetCurrentIndex() int {
	return kr.currentIdx
}

// HasKeys returns true if there are any keys configured
func (kr *KeyRotator) HasKeys() bool {
	return len(kr.keys) > 0
}

// Rotate moves to the next available API key
func (kr *KeyRotator) Rotate() (string, error) {
	if len(kr.keys) <= 1 {
		return "", ErrNoAvailableKeys
	}
	nextIndex := kr.currentIdx + 1
	if nextIndex >= len(kr.keys) {
		return "", ErrNoAvailableKeys
	}
	kr.currentIdx = nextIndex
	kr.currentKey = kr.keys[nextIndex]
	return kr.currentKey, nil
}

// getKeysFromEnv retrieves API keys from an environment variable (comma-separated)
func getKeysFromEnv(envVar string) []string {
	keysEnv := os.Getenv(envVar)
	if keysEnv == "" {
		return nil
	}
	keys := strings.Split(keysEnv, ",")
	var result []string
	for _, key := range keys {
		key = strings.TrimSpace(key)
		if key != "" {
			result = append(result, key)
		}
	}
	return result
}

// Config holds the application configuration
type Config struct {
	// Provider selection
	Provider string // "copilot", "azure", or "" (auto-detect)

	// Azure OpenAI settings
	AzureEndpoint   string
	AzureAPIKey     string
	AzureModels     []string // Available Azure models (from config file or env)
	Model           string
	AvailableModels []string

	// Copilot settings
	AccountType   string   // "individual", "business", or "enterprise"
	CopilotModels []string // Available Copilot models

	// Key rotators for search providers
	TavilyKeys *KeyRotator
	LinkupKeys *KeyRotator
	BraveKeys  *KeyRotator

	// Web search provider selection
	WebSearchProvider string // "tavily", "linkup", or "brave"

	// Flags
	Stream      bool
	Render      bool
	Usage       bool
	WebSearch   bool
	Citations   bool // Show citations/sources from web search
	Interactive bool // Interactive chat mode
}

// NewConfig creates a new Config with defaults
func NewConfig() *Config {
	return &Config{}
}

// Validate validates the configuration and loads from environment
func (c *Config) Validate() error {
	// Load from config file first (lowest priority)
	if fileConfig, err := LoadConfigFile(); err == nil {
		c.ApplyFileConfig(fileConfig)
	}
	// Errors loading config file are silently ignored - env vars and flags take precedence

	// Load provider selection
	if c.Provider == "" {
		c.Provider = os.Getenv(EnvAIProvider)
	}

	// Load Copilot settings
	if c.AccountType == "" {
		c.AccountType = os.Getenv(EnvCopilotAccountType)
	}
	if c.AccountType == "" {
		c.AccountType = DefaultAccountType
	}

	// Load Copilot models
	if copilotModelsEnv := os.Getenv(EnvCopilotModels); copilotModelsEnv != "" {
		models := strings.Split(copilotModelsEnv, ",")
		for _, m := range models {
			m = strings.TrimSpace(m)
			if m != "" {
				c.CopilotModels = append(c.CopilotModels, m)
			}
		}
	}
	if len(c.CopilotModels) == 0 {
		c.CopilotModels = DefaultCopilotModels
	}

	// Load Azure endpoint (optional now)
	if c.AzureEndpoint == "" {
		c.AzureEndpoint = os.Getenv(EnvAzureEndpoint)
	}
	if c.AzureEndpoint != "" {
		c.AzureEndpoint = strings.TrimSuffix(c.AzureEndpoint, "/")
	}

	// Load Azure API key (optional now)
	if c.AzureAPIKey == "" {
		c.AzureAPIKey = strings.TrimSpace(os.Getenv(EnvAzureAPIKey))
	}

	// Load Azure models from env (env var overrides config file)
	if modelsEnv := os.Getenv(EnvAzureModels); modelsEnv != "" {
		c.AzureModels = nil // Clear config file models
		models := strings.Split(modelsEnv, ",")
		for _, m := range models {
			m = strings.TrimSpace(m)
			if m != "" {
				c.AzureModels = append(c.AzureModels, m)
			}
		}
	}

	// Set available models based on provider
	if c.Provider == "azure" {
		// Azure provider requires endpoint and key
		if c.AzureEndpoint == "" {
			return ErrEndpointNotFound
		}
		if c.AzureAPIKey == "" {
			return ErrAPIKeyNotFound
		}
		// Use Azure models
		if len(c.AzureModels) > 0 {
			c.AvailableModels = c.AzureModels
		}
	} else if c.Provider == "copilot" || c.Provider == "github" {
		// Use Copilot models
		c.AvailableModels = c.CopilotModels
	} else {
		// Auto-detect: prefer Copilot if logged in
		if auth.IsLoggedIn() {
			c.AvailableModels = c.CopilotModels
		} else if len(c.AzureModels) > 0 {
			c.AvailableModels = c.AzureModels
		} else {
			c.AvailableModels = c.CopilotModels
		}
	}

	// Load default model - pick first available if not set
	if c.Model == "" {
		if len(c.AvailableModels) > 0 {
			c.Model = c.AvailableModels[0]
		} else {
			c.Model = DefaultModel
		}
	}

	// Initialize key rotators
	c.TavilyKeys = NewKeyRotator(EnvTavilyAPIKeys)
	c.LinkupKeys = NewKeyRotator(EnvLinkupAPIKeys)
	c.BraveKeys = NewKeyRotator(EnvBraveAPIKeys)

	// Set web search provider (default to tavily, or auto-detect based on available keys)
	if c.WebSearchProvider == "" {
		c.WebSearchProvider = os.Getenv(EnvWebSearchProvider)
	}
	if c.WebSearchProvider == "" {
		// Auto-detect: prefer tavily if available, then linkup, then brave
		if c.TavilyKeys.HasKeys() {
			c.WebSearchProvider = "tavily"
		} else if c.LinkupKeys.HasKeys() {
			c.WebSearchProvider = "linkup"
		} else if c.BraveKeys.HasKeys() {
			c.WebSearchProvider = "brave"
		} else {
			c.WebSearchProvider = DefaultSearchProvider
		}
	}

	// Validate provider
	if c.WebSearchProvider != "tavily" && c.WebSearchProvider != "linkup" && c.WebSearchProvider != "brave" {
		return ErrInvalidSearchProvider
	}

	// Validate web search keys if web search is requested
	if c.WebSearch {
		if c.WebSearchProvider == "tavily" && !c.TavilyKeys.HasKeys() {
			return ErrWebSearchKeyNotFound
		}
		if c.WebSearchProvider == "linkup" && !c.LinkupKeys.HasKeys() {
			return ErrWebSearchKeyNotFound
		}
		if c.WebSearchProvider == "brave" && !c.BraveKeys.HasKeys() {
			return ErrWebSearchKeyNotFound
		}
	}

	return nil
}

// GetAzureAPIURL builds the full API URL for chat completions
func (c *Config) GetAzureAPIURL() string {
	return fmt.Sprintf("%s/openai/v1/chat/completions",
		c.AzureEndpoint)
}

// ValidateModel checks if the given model is in available models
func (c *Config) ValidateModel(model string) bool {
	if len(c.AvailableModels) == 0 {
		return true // No validation if models not configured
	}
	for _, m := range c.AvailableModels {
		if m == model {
			return true
		}
	}
	return false
}

// GetAvailableModelsString returns a formatted string of available models
func (c *Config) GetAvailableModelsString() string {
	if len(c.AvailableModels) == 0 {
		return "(not configured - set AZURE_OPENAI_MODELS)"
	}
	return strings.Join(c.AvailableModels, ", ")
}
