// Package api provides unified AI client interfaces for multiple providers.
// It supports GitHub Copilot and Azure OpenAI with automatic provider detection,
// streaming responses, tool/function calling, and retry logic for transient failures.
package api

import (
	"context"
	"fmt"

	"github.com/quocvuong92/ai-cli/internal/auth"
	"github.com/quocvuong92/ai-cli/internal/config"
)

// AIClient defines the interface for AI API clients.
// Both CopilotClient and AzureClient implement this interface,
// allowing transparent switching between providers.
type AIClient interface {
	// Query sends a simple query (non-streaming)
	Query(systemPrompt, userMessage string) (*ChatResponse, error)

	// QueryWithContext sends a query with context support (non-streaming)
	QueryWithContext(ctx context.Context, systemPrompt, userMessage string) (*ChatResponse, error)

	// QueryWithHistory sends a query with full message history (non-streaming)
	QueryWithHistory(messages []Message) (*ChatResponse, error)

	// QueryWithHistoryContext sends a query with full message history and context support (non-streaming)
	QueryWithHistoryContext(ctx context.Context, messages []Message) (*ChatResponse, error)

	// QueryWithHistoryAndToolsContext sends a query with full message history, tools, and context support (non-streaming)
	QueryWithHistoryAndToolsContext(ctx context.Context, messages []Message, tools []Tool) (*ChatResponse, error)

	// QueryStream sends a streaming query
	QueryStream(systemPrompt, userMessage string, onChunk func(content string), onDone func(resp *ChatResponse)) error

	// QueryStreamWithContext sends a streaming query with context support
	QueryStreamWithContext(ctx context.Context, systemPrompt, userMessage string, onChunk func(content string), onDone func(resp *ChatResponse)) error

	// QueryStreamWithHistory sends a streaming query with full message history
	QueryStreamWithHistory(messages []Message, onChunk func(content string), onDone func(resp *ChatResponse)) error

	// QueryStreamWithHistoryContext sends a streaming query with full message history and context support
	QueryStreamWithHistoryContext(ctx context.Context, messages []Message, onChunk func(content string), onDone func(resp *ChatResponse)) error

	// QueryStreamWithHistoryAndToolsContext sends a streaming query with full message history, tools, and context support
	QueryStreamWithHistoryAndToolsContext(ctx context.Context, messages []Message, tools []Tool, onChunk func(content string), onDone func(resp *ChatResponse)) error

	// Close releases any resources held by the client (e.g., stops background goroutines)
	Close()
}

// Ensure both clients implement AIClient interface
var _ AIClient = (*AzureClient)(nil)
var _ AIClient = (*CopilotClient)(nil)

// NewClient creates an AI client based on configuration.
// Provider selection follows this priority:
//  1. Explicit provider in cfg.Provider ("copilot", "github", or "azure")
//  2. Auto-detect: GitHub Copilot if logged in, otherwise Azure if configured
//
// For Copilot, it automatically manages token refresh in the background.
// Returns an error if no provider is available or configured.
func NewClient(cfg *config.Config) (AIClient, error) {
	switch cfg.Provider {
	case "copilot", "github":
		// Load GitHub token
		githubToken, err := auth.LoadGitHubToken()
		if err != nil {
			return nil, fmt.Errorf("GitHub Copilot requires login: %w", err)
		}

		// Create token manager
		tokenManager := auth.NewTokenManager(githubToken)

		// Start auto-refresh in background
		tokenManager.StartAutoRefresh(context.Background())

		return NewCopilotClient(cfg, tokenManager), nil

	case "azure":
		if cfg.AzureEndpoint == "" || cfg.AzureAPIKey == "" {
			return nil, fmt.Errorf("Azure provider requires AZURE_OPENAI_ENDPOINT and AZURE_OPENAI_API_KEY")
		}
		return NewAzureClient(cfg), nil

	default:
		// Auto-detect: prefer Copilot if logged in, otherwise Azure
		if auth.IsLoggedIn() {
			githubToken, _ := auth.LoadGitHubToken()
			tokenManager := auth.NewTokenManager(githubToken)
			tokenManager.StartAutoRefresh(context.Background())
			return NewCopilotClient(cfg, tokenManager), nil
		}

		if cfg.AzureEndpoint != "" && cfg.AzureAPIKey != "" {
			return NewAzureClient(cfg), nil
		}

		return nil, fmt.Errorf("no AI provider configured. Run 'ai login' for GitHub Copilot or set AZURE_OPENAI_* environment variables")
	}
}

// NewClientWithProvider creates an AI client for a specific provider,
// temporarily overriding the provider setting in the config.
// This is useful for testing or when the caller needs to explicitly
// specify a provider regardless of the config's current setting.
func NewClientWithProvider(cfg *config.Config, provider string) (AIClient, error) {
	originalProvider := cfg.Provider
	cfg.Provider = provider
	defer func() { cfg.Provider = originalProvider }()

	return NewClient(cfg)
}
