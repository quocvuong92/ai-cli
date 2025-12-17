// Package constants provides shared constants used across the application
// to avoid circular dependencies between packages.
package constants

import "time"

// Timeout constants used across the application
const (
	// DefaultAPITimeout is the timeout for AI API requests (streaming can take a while)
	DefaultAPITimeout = 120 * time.Second
	// DefaultCommandTimeout is the timeout for shell command execution
	DefaultCommandTimeout = 30 * time.Second
	// DefaultOAuthTimeout is the timeout for OAuth HTTP requests
	DefaultOAuthTimeout = 30 * time.Second
)

// Application defaults
const (
	DefaultModel          = "gpt-5-mini"
	DefaultSystemMessage  = "Be precise and concise."
	DefaultSearchProvider = "tavily"
	DefaultAccountType    = "individual"
)

// DefaultCopilotModels are the models available through GitHub Copilot
// Updated: 2025-12-17
var DefaultCopilotModels = []string{
	// GPT models
	"gpt-5-mini",
	"gpt-4.1",
	"gpt-5.1",
	"gpt-5.1-codex",
	"gpt-5.1-codex-mini",
	"gpt-5.2",
	// Grok models
	"grok-code-fast-1",
	// Claude models
	"claude-sonnet-4.5",
	"claude-opus-4.5",
	"claude-haiku-4.5",
	// Gemini models
	"gemini-2.5-pro",
	"gemini-3-pro-preview",
}
