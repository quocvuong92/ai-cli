package auth

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/quocvuong92/ai-cli/internal/constants"
)

// GitHub OAuth constants (from GitHub Copilot)
const (
	GitHubClientID   = "Iv1.b507a08c87ecfe98"
	GitHubBaseURL    = "https://github.com"
	GitHubAPIBaseURL = "https://api.github.com"
	GitHubAppScopes  = "read:user"
)

// DeviceCodeResponse represents the response from GitHub device code endpoint
type DeviceCodeResponse struct {
	DeviceCode      string `json:"device_code"`
	UserCode        string `json:"user_code"`
	VerificationURI string `json:"verification_uri"`
	ExpiresIn       int    `json:"expires_in"`
	Interval        int    `json:"interval"`
}

// AccessTokenResponse represents the response from GitHub access token endpoint
type AccessTokenResponse struct {
	AccessToken string `json:"access_token"`
	TokenType   string `json:"token_type"`
	Scope       string `json:"scope"`
	Error       string `json:"error,omitempty"`
	ErrorDesc   string `json:"error_description,omitempty"`
}

// GitHubAuth handles GitHub OAuth device flow
type GitHubAuth struct {
	httpClient *http.Client
}

// NewGitHubAuth creates a new GitHubAuth instance
func NewGitHubAuth() *GitHubAuth {
	return &GitHubAuth{
		httpClient: &http.Client{
			Timeout: constants.DefaultOAuthTimeout,
		},
	}
}

// GetDeviceCode requests a device code from GitHub
func (g *GitHubAuth) GetDeviceCode(ctx context.Context) (*DeviceCodeResponse, error) {
	reqBody := map[string]string{
		"client_id": GitHubClientID,
		"scope":     GitHubAppScopes,
	}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		GitHubBaseURL+"/login/device/code", bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	resp, err := g.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to get device code: status %d, body: %s", resp.StatusCode, string(body))
	}

	var deviceCode DeviceCodeResponse
	if err := json.Unmarshal(body, &deviceCode); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &deviceCode, nil
}

// PollAccessToken polls GitHub for the access token after user authorization
func (g *GitHubAuth) PollAccessToken(ctx context.Context, deviceCode *DeviceCodeResponse) (string, error) {
	// Interval is in seconds, add 1 second buffer
	pollInterval := time.Duration(deviceCode.Interval+1) * time.Second
	expiresAt := time.Now().Add(time.Duration(deviceCode.ExpiresIn) * time.Second)

	reqBody := map[string]string{
		"client_id":   GitHubClientID,
		"device_code": deviceCode.DeviceCode,
		"grant_type":  "urn:ietf:params:oauth:grant-type:device_code",
	}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("failed to marshal request: %w", err)
	}

	for {
		// Check if context cancelled
		if err := ctx.Err(); err != nil {
			return "", fmt.Errorf("polling cancelled: %w", err)
		}

		// Check if expired
		if time.Now().After(expiresAt) {
			return "", fmt.Errorf("device code expired, please try again")
		}

		req, err := http.NewRequestWithContext(ctx, http.MethodPost,
			GitHubBaseURL+"/login/oauth/access_token", bytes.NewBuffer(jsonData))
		if err != nil {
			return "", fmt.Errorf("failed to create request: %w", err)
		}

		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Accept", "application/json")

		resp, err := g.httpClient.Do(req)
		if err != nil {
			// Network error, wait and retry
			select {
			case <-ctx.Done():
				return "", ctx.Err()
			case <-time.After(pollInterval):
				continue
			}
		}

		body, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			select {
			case <-ctx.Done():
				return "", ctx.Err()
			case <-time.After(pollInterval):
				continue
			}
		}

		var tokenResp AccessTokenResponse
		if err := json.Unmarshal(body, &tokenResp); err != nil {
			select {
			case <-ctx.Done():
				return "", ctx.Err()
			case <-time.After(pollInterval):
				continue
			}
		}

		// Check for access token
		if tokenResp.AccessToken != "" {
			return tokenResp.AccessToken, nil
		}

		// Check for errors
		switch tokenResp.Error {
		case "authorization_pending":
			// User hasn't authorized yet, continue polling
		case "slow_down":
			// We're polling too fast, increase interval
			pollInterval += 5 * time.Second
		case "expired_token":
			return "", fmt.Errorf("device code expired, please try again")
		case "access_denied":
			return "", fmt.Errorf("access denied by user")
		default:
			if tokenResp.Error != "" {
				return "", fmt.Errorf("OAuth error: %s - %s", tokenResp.Error, tokenResp.ErrorDesc)
			}
		}

		// Wait before next poll
		select {
		case <-ctx.Done():
			return "", ctx.Err()
		case <-time.After(pollInterval):
		}
	}
}

// GetTokenPath returns the path where the GitHub token is stored
func GetTokenPath() (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to get home directory: %w", err)
	}

	return filepath.Join(homeDir, ".local", "share", "ai-cli", "github-token"), nil
}

// SaveGitHubToken saves the GitHub token to disk
func SaveGitHubToken(token string) error {
	tokenPath, err := GetTokenPath()
	if err != nil {
		return err
	}

	// Create directory if it doesn't exist
	dir := filepath.Dir(tokenPath)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	// Write token with restricted permissions
	if err := os.WriteFile(tokenPath, []byte(token), 0600); err != nil {
		return fmt.Errorf("failed to write token: %w", err)
	}

	return nil
}

// LoadGitHubToken loads the GitHub token from disk
func LoadGitHubToken() (string, error) {
	tokenPath, err := GetTokenPath()
	if err != nil {
		return "", err
	}

	data, err := os.ReadFile(tokenPath)
	if err != nil {
		if os.IsNotExist(err) {
			return "", fmt.Errorf("not logged in, please run 'ai login' first")
		}
		return "", fmt.Errorf("failed to read token: %w", err)
	}

	token := string(data)
	if token == "" {
		return "", fmt.Errorf("token file is empty, please run 'ai login' again")
	}

	return token, nil
}

// DeleteGitHubToken removes the stored GitHub token
func DeleteGitHubToken() error {
	tokenPath, err := GetTokenPath()
	if err != nil {
		return err
	}

	if err := os.Remove(tokenPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to delete token: %w", err)
	}

	return nil
}

// IsLoggedIn checks if a GitHub token exists
func IsLoggedIn() bool {
	token, err := LoadGitHubToken()
	return err == nil && token != ""
}
