# Implementation Plan: Replace Azure with GitHub Copilot

## Status: COMPLETED

## Overview

Replace Azure OpenAI API calls with GitHub Copilot API while maintaining the same CLI interface. This gives you access to GPT-4, Claude, and other models through your 2-year Copilot license.

## Architecture Changes

```
Current:                          After:
┌─────────┐                      ┌─────────┐
│   CLI   │                      │   CLI   │
└────┬────┘                      └────┬────┘
     │                                │
     ▼                                ▼
┌─────────────┐                  ┌──────────────┐
│ AzureClient │                  │CopilotClient │
└──────┬──────┘                  └──────┬───────┘
       │                                │
       ▼                                ▼
┌──────────────────┐             ┌─────────────────────┐
│ Azure OpenAI API │             │ api.githubcopilot.com│
└──────────────────┘             └─────────────────────┘
```

## Implementation Steps

### Step 1: Add GitHub Auth Package (`internal/auth/github.go`)

Create a new package to handle GitHub OAuth device flow:

```go
// Key constants (from copilot-api reference)
const (
    GitHubClientID     = "Iv1.b507a08c87ecfe98"
    GitHubBaseURL      = "https://github.com"
    GitHubAPIBaseURL   = "https://api.github.com"
    GitHubAppScopes    = "read:user"
)

// DeviceCodeResponse from GitHub
type DeviceCodeResponse struct {
    DeviceCode      string `json:"device_code"`
    UserCode        string `json:"user_code"`
    VerificationURI string `json:"verification_uri"`
    ExpiresIn       int    `json:"expires_in"`
    Interval        int    `json:"interval"`
}

// Functions to implement:
// - GetDeviceCode() (*DeviceCodeResponse, error)
// - PollAccessToken(deviceCode *DeviceCodeResponse) (string, error)
// - SaveGitHubToken(token string) error
// - LoadGitHubToken() (string, error)
```

**Token storage location:** `~/.local/share/gh-ai-cli/github-token`

### Step 2: Add Copilot Token Manager (`internal/auth/copilot.go`)

Manage the short-lived Copilot API token:

```go
// CopilotTokenResponse from GitHub API
type CopilotTokenResponse struct {
    Token     string `json:"token"`
    ExpiresAt int64  `json:"expires_at"`
    RefreshIn int    `json:"refresh_in"`
}

// TokenManager handles token lifecycle
type TokenManager struct {
    githubToken  string
    copilotToken string
    expiresAt    time.Time
    mu           sync.RWMutex
}

// Functions to implement:
// - NewTokenManager(githubToken string) *TokenManager
// - (tm *TokenManager) GetCopilotToken() (string, error)
// - (tm *TokenManager) refreshToken() error
// - (tm *TokenManager) StartAutoRefresh(ctx context.Context)
```

**Key behavior:**
- Copilot token expires every ~15 minutes
- Auto-refresh 60 seconds before expiry
- Thread-safe access with RWMutex

### Step 3: Create Copilot Client (`internal/api/copilot.go`)

New API client that mirrors AzureClient interface:

```go
const (
    CopilotVersion       = "0.26.7"
    EditorPluginVersion  = "copilot-chat/0.26.7"
    UserAgent           = "GitHubCopilotChat/0.26.7"
    APIVersion          = "2025-04-01"
    VSCodeVersion       = "1.96.0"
)

type CopilotClient struct {
    httpClient   *http.Client
    config       *config.Config
    tokenManager *auth.TokenManager
}

// Required headers for Copilot API
func (c *CopilotClient) buildHeaders(enableVision bool) map[string]string {
    return map[string]string{
        "Authorization":           "Bearer " + c.tokenManager.GetCopilotToken(),
        "Content-Type":            "application/json",
        "copilot-integration-id":  "vscode-chat",
        "editor-version":          "vscode/" + VSCodeVersion,
        "editor-plugin-version":   EditorPluginVersion,
        "user-agent":              UserAgent,
        "openai-intent":           "conversation-panel",
        "x-github-api-version":    APIVersion,
        "x-request-id":            uuid.New().String(),
    }
}

// Base URL depends on account type
func (c *CopilotClient) getBaseURL() string {
    if c.config.AccountType == "individual" {
        return "https://api.githubcopilot.com"
    }
    return fmt.Sprintf("https://api.%s.githubcopilot.com", c.config.AccountType)
}
```

**Methods to implement (same interface as AzureClient):**
- `Query(systemPrompt, userMessage string) (*ChatResponse, error)`
- `QueryWithContext(ctx, systemPrompt, userMessage) (*ChatResponse, error)`
- `QueryWithHistoryContext(ctx, messages) (*ChatResponse, error)`
- `QueryWithHistoryAndToolsContext(ctx, messages, tools) (*ChatResponse, error)`
- `QueryStreamWithHistoryAndToolsContext(ctx, messages, tools, onChunk, onDone) error`

### Step 4: Add Login Command (`cmd/login.go`)

New CLI command to authenticate with GitHub:

```go
// cobra command: gh-ai login
var loginCmd = &cobra.Command{
    Use:   "login",
    Short: "Authenticate with GitHub Copilot",
    RunE: func(cmd *cobra.Command, args []string) error {
        // 1. Request device code
        // 2. Display: "Please visit https://github.com/login/device"
        // 3. Display: "Enter code: XXXX-XXXX"
        // 4. Poll for access token
        // 5. Save token to ~/.local/share/gh-ai-cli/github-token
        // 6. Print success message
    },
}
```

### Step 5: Update Config (`internal/config/config.go`)

Add new configuration options:

```go
// New environment variables
const (
    EnvGitHubToken     = "GITHUB_TOKEN"        // Optional override
    EnvAccountType     = "COPILOT_ACCOUNT_TYPE" // individual/business/enterprise
    EnvCopilotModels   = "COPILOT_MODELS"       // Available models
)

// New fields in Config struct
type Config struct {
    // ... existing fields ...
    
    // Copilot settings
    GitHubToken   string   // From file or env
    AccountType   string   // "individual", "business", "enterprise"
    CopilotModels []string // Available Copilot models
}

// Default Copilot models
var DefaultCopilotModels = []string{
    "gpt-4.1",
    "gpt-4o", 
    "gpt-4o-mini",
    "claude-3.7-sonnet",
    "o1-preview",
    "o1-mini",
}
```

### Step 6: Create Client Interface (`internal/api/client.go`)

Abstract interface so commands work with either backend:

```go
// AIClient interface for both Azure and Copilot
type AIClient interface {
    Query(systemPrompt, userMessage string) (*ChatResponse, error)
    QueryWithContext(ctx context.Context, systemPrompt, userMessage string) (*ChatResponse, error)
    QueryWithHistoryContext(ctx context.Context, messages []Message) (*ChatResponse, error)
    QueryWithHistoryAndToolsContext(ctx context.Context, messages []Message, tools []Tool) (*ChatResponse, error)
    QueryStreamWithHistoryAndToolsContext(ctx context.Context, messages []Message, tools []Tool, onChunk func(string), onDone func(*ChatResponse)) error
}

// NewClient creates the appropriate client based on config
func NewClient(cfg *config.Config) (AIClient, error) {
    if cfg.UseCopilot {
        return NewCopilotClient(cfg)
    }
    return NewAzureClient(cfg), nil
}
```

### Step 7: Update Commands

Update `cmd/run.go` and `cmd/interactive.go` to use the new interface:

```go
// Before:
client := api.NewAzureClient(cfg)

// After:
client, err := api.NewClient(cfg)
if err != nil {
    return err
}
```

### Step 8: Update Root Command (`cmd/root.go`)

Add global flag for backend selection:

```go
rootCmd.PersistentFlags().Bool("copilot", true, "Use GitHub Copilot (default)")
rootCmd.PersistentFlags().Bool("azure", false, "Use Azure OpenAI")
```

## File Structure After Implementation

```
internal/
├── api/
│   ├── azure.go       # Existing (keep for fallback)
│   ├── copilot.go     # NEW: Copilot API client
│   ├── client.go      # NEW: AIClient interface
│   ├── tools.go       # Existing (unchanged)
│   └── ...
├── auth/
│   ├── github.go      # NEW: GitHub OAuth device flow
│   └── copilot.go     # NEW: Copilot token manager
├── config/
│   └── config.go      # Updated with Copilot settings
cmd/
├── login.go           # NEW: Login command
├── root.go            # Updated with --copilot/--azure flags
├── run.go             # Updated to use AIClient interface
└── interactive.go     # Updated to use AIClient interface
```

## Environment Variables (After)

```bash
# Copilot (new default)
export COPILOT_ACCOUNT_TYPE="individual"  # or "business", "enterprise"
export COPILOT_MODELS="gpt-4.1,gpt-4o,claude-3.7-sonnet"

# Azure (optional fallback)
export AZURE_OPENAI_ENDPOINT="https://..."
export AZURE_OPENAI_API_KEY="..."
export AZURE_OPENAI_MODELS="..."
```

## Usage After Implementation

```bash
# First time: authenticate with GitHub
gh-ai login

# Use normally (Copilot is default)
gh-ai "explain this code"
gh-ai -i  # interactive mode

# Specify model
gh-ai --model gpt-4.1 "explain this"
gh-ai --model claude-3.7-sonnet "explain this"

# Force Azure backend (if configured)
gh-ai --azure "explain this"
```

## Testing Checklist

- [x] `ai login` completes OAuth flow successfully
- [x] Token saved to `~/.local/share/ai-cli/github-token`
- [x] Basic query works: `ai "hello"`
- [x] Streaming works: `ai -s "tell me a story"`
- [x] Tool calls work: command execution in interactive mode
- [x] Token auto-refresh works (background goroutine)
- [x] Model switching works: `--model gpt-4o`
- [x] Azure fallback works: `--provider azure` flag
- [x] CLI builds successfully
- [x] All existing tests pass

## Notes

1. **No request format changes needed** - Copilot uses the same OpenAI-compatible format
2. **Streaming works identically** - SSE with `data:` prefix
3. **Tool calls work identically** - Same schema
4. **The main work is authentication** - OAuth + token refresh

## Quick Start

```bash
# Build
go build -o ai ./main.go

# Login to GitHub Copilot
./ai login

# Use it
./ai "What is Go?"
./ai -i  # Interactive mode
./ai --model claude-3.7-sonnet "Explain this code"
```
