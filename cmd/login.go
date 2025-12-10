package cmd

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/spf13/cobra"

	"github.com/quocvuong92/ai-cli/internal/auth"
	"github.com/quocvuong92/ai-cli/internal/display"
)

func init() {
	// Login command will be added to rootCmd in Execute()
}

// NewLoginCmd creates the login command
func NewLoginCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "login",
		Short: "Authenticate with GitHub Copilot",
		Long: `Authenticate with GitHub Copilot using device flow.

This will open a browser window where you can authorize the application.
Your GitHub token will be stored locally for future use.

Examples:
  ai login`,
		RunE: runLogin,
	}
}

// NewLogoutCmd creates the logout command
func NewLogoutCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "logout",
		Short: "Remove stored GitHub credentials",
		Long: `Remove stored GitHub credentials.

This will delete your stored GitHub token. You will need to run 'ai login'
again to use GitHub Copilot.

Examples:
  ai logout`,
		RunE: runLogout,
	}
}

// NewStatusCmd creates the status command
func NewStatusCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Show authentication status",
		Long: `Show current authentication status.

Displays whether you are logged in and which provider is active.

Examples:
  ai status`,
		RunE: runStatus,
	}
}

func runLogin(cmd *cobra.Command, args []string) error {
	// Check if already logged in
	if auth.IsLoggedIn() {
		fmt.Println("Already logged in to GitHub Copilot.")
		fmt.Println("Run 'ai logout' first if you want to re-authenticate.")
		return nil
	}

	// Create context that cancels on interrupt
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Handle Ctrl+C
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-sigChan
		fmt.Println("\nLogin cancelled.")
		cancel()
		os.Exit(1)
	}()

	githubAuth := auth.NewGitHubAuth()

	// Step 1: Get device code
	fmt.Println("Requesting device code from GitHub...")
	deviceCode, err := githubAuth.GetDeviceCode(ctx)
	if err != nil {
		return fmt.Errorf("failed to get device code: %w", err)
	}

	// Step 2: Show instructions to user
	fmt.Println()
	fmt.Println("To authenticate, please:")
	fmt.Printf("  1. Open: %s\n", deviceCode.VerificationURI)
	fmt.Printf("  2. Enter code: %s\n", deviceCode.UserCode)
	fmt.Println()

	// Try to open browser automatically
	display.TryOpenBrowser(deviceCode.VerificationURI)

	// Step 3: Poll for access token
	sp := display.NewSpinner("Waiting for authorization...")
	sp.Start()

	accessToken, err := githubAuth.PollAccessToken(ctx, deviceCode)
	sp.Stop()

	if err != nil {
		return fmt.Errorf("authentication failed: %w", err)
	}

	// Step 4: Save token
	if err := auth.SaveGitHubToken(accessToken); err != nil {
		return fmt.Errorf("failed to save token: %w", err)
	}

	fmt.Println()
	fmt.Println("Successfully logged in to GitHub Copilot!")
	fmt.Println("You can now use 'ai' commands with GitHub Copilot.")

	return nil
}

func runLogout(cmd *cobra.Command, args []string) error {
	if !auth.IsLoggedIn() {
		fmt.Println("Not currently logged in.")
		return nil
	}

	if err := auth.DeleteGitHubToken(); err != nil {
		return fmt.Errorf("failed to logout: %w", err)
	}

	fmt.Println("Successfully logged out.")
	return nil
}

func runStatus(cmd *cobra.Command, args []string) error {
	fmt.Println("Authentication Status:")
	fmt.Println()

	// Check GitHub Copilot
	if auth.IsLoggedIn() {
		tokenPath, _ := auth.GetTokenPath()
		fmt.Printf("  GitHub Copilot: Logged in\n")
		fmt.Printf("  Token stored at: %s\n", tokenPath)
	} else {
		fmt.Printf("  GitHub Copilot: Not logged in\n")
		fmt.Printf("  Run 'ai login' to authenticate\n")
	}

	return nil
}
