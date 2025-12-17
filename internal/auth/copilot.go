package auth

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"

	"github.com/quocvuong92/ai-cli/internal/constants"
)

// Copilot API constants
const (
	CopilotTokenURL     = "https://api.github.com/copilot_internal/v2/token"
	CopilotVersion      = "0.26.7"
	EditorPluginVersion = "copilot-chat/" + CopilotVersion
	UserAgent           = "GitHubCopilotChat/" + CopilotVersion
	APIVersion          = "2025-04-01"
	VSCodeVersion       = "1.96.0"
)

// CopilotTokenResponse represents the response from Copilot token endpoint
type CopilotTokenResponse struct {
	Token     string `json:"token"`
	ExpiresAt int64  `json:"expires_at"`
	RefreshIn int    `json:"refresh_in"`
}

// TokenManager handles Copilot token lifecycle
type TokenManager struct {
	httpClient    *http.Client
	githubToken   string
	copilotToken  string
	expiresAt     time.Time
	mu            sync.RWMutex
	cancelRefresh context.CancelFunc
}

// NewTokenManager creates a new TokenManager with the given GitHub token
func NewTokenManager(githubToken string) *TokenManager {
	return &TokenManager{
		httpClient: &http.Client{
			Timeout: constants.DefaultOAuthTimeout,
		},
		githubToken: githubToken,
	}
}

// GetCopilotToken returns a valid Copilot token, refreshing if necessary
func (tm *TokenManager) GetCopilotToken(ctx context.Context) (string, error) {
	tm.mu.RLock()
	token := tm.copilotToken
	expiresAt := tm.expiresAt
	tm.mu.RUnlock()

	// If token is still valid (with 60 second buffer), return it
	if token != "" && time.Now().Add(60*time.Second).Before(expiresAt) {
		return token, nil
	}

	// Need to refresh
	return tm.refreshToken(ctx)
}

// refreshToken fetches a new Copilot token from GitHub API
func (tm *TokenManager) refreshToken(ctx context.Context) (string, error) {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	// Double-check after acquiring write lock
	if tm.copilotToken != "" && time.Now().Add(60*time.Second).Before(tm.expiresAt) {
		return tm.copilotToken, nil
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, CopilotTokenURL, nil)
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	// Set required headers (matching copilot-api reference)
	req.Header.Set("Authorization", "token "+tm.githubToken)
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Editor-Version", "vscode/"+VSCodeVersion)
	req.Header.Set("Editor-Plugin-Version", EditorPluginVersion)
	req.Header.Set("User-Agent", UserAgent)
	req.Header.Set("X-GitHub-Api-Version", APIVersion)

	resp, err := tm.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to get Copilot token: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		if resp.StatusCode == http.StatusUnauthorized {
			return "", fmt.Errorf("GitHub token is invalid or expired, please run 'ai login' again")
		}
		if resp.StatusCode == http.StatusForbidden {
			return "", fmt.Errorf("GitHub Copilot access denied. Make sure you have an active Copilot subscription")
		}
		return "", fmt.Errorf("failed to get Copilot token: status %d, body: %s", resp.StatusCode, string(body))
	}

	var tokenResp CopilotTokenResponse
	if err := json.Unmarshal(body, &tokenResp); err != nil {
		return "", fmt.Errorf("failed to parse response: %w", err)
	}

	tm.copilotToken = tokenResp.Token
	tm.expiresAt = time.Unix(tokenResp.ExpiresAt, 0)

	return tm.copilotToken, nil
}

// StartAutoRefresh starts a background goroutine that refreshes the token before expiry
func (tm *TokenManager) StartAutoRefresh(ctx context.Context) {
	// Cancel any existing refresh goroutine
	if tm.cancelRefresh != nil {
		tm.cancelRefresh()
	}

	ctx, cancel := context.WithCancel(ctx)
	tm.cancelRefresh = cancel

	go func() {
		for {
			tm.mu.RLock()
			expiresAt := tm.expiresAt
			tm.mu.RUnlock()

			// Calculate when to refresh (60 seconds before expiry)
			var refreshIn time.Duration
			if expiresAt.IsZero() {
				// No token yet, refresh immediately
				refreshIn = 0
			} else {
				refreshIn = time.Until(expiresAt) - 60*time.Second
				if refreshIn < 0 {
					refreshIn = 0
				}
			}

			select {
			case <-ctx.Done():
				return
			case <-time.After(refreshIn):
				_, err := tm.refreshToken(ctx)
				if err != nil {
					// Log error but continue trying
					// In production, you might want to notify the user
					continue
				}
			}
		}
	}()
}

// StopAutoRefresh stops the background refresh goroutine
func (tm *TokenManager) StopAutoRefresh() {
	if tm.cancelRefresh != nil {
		tm.cancelRefresh()
		tm.cancelRefresh = nil
	}
}

// GetBaseURL returns the Copilot API base URL based on account type
func GetCopilotBaseURL(accountType string) string {
	if accountType == "" || accountType == "individual" {
		return "https://api.githubcopilot.com"
	}
	return fmt.Sprintf("https://api.%s.githubcopilot.com", accountType)
}

// BuildCopilotHeaders builds the required headers for Copilot API requests
func BuildCopilotHeaders(copilotToken string, enableVision bool) map[string]string {
	headers := map[string]string{
		"Authorization":                       "Bearer " + copilotToken,
		"Content-Type":                        "application/json",
		"Copilot-Integration-Id":              "vscode-chat",
		"Editor-Version":                      "vscode/" + VSCodeVersion,
		"Editor-Plugin-Version":               EditorPluginVersion,
		"User-Agent":                          UserAgent,
		"Openai-Intent":                       "conversation-panel",
		"X-Github-Api-Version":                APIVersion,
		"X-Vscode-User-Agent-Library-Version": "electron-fetch",
	}

	if enableVision {
		headers["Copilot-Vision-Request"] = "true"
	}

	return headers
}
