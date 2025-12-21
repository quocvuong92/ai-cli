// Package cmd implements the CLI commands for the AI CLI application.
package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/google/uuid"
	"github.com/quocvuong92/ai-cli/internal/api"
	"github.com/quocvuong92/ai-cli/internal/config"
	"github.com/quocvuong92/ai-cli/internal/display"
	"github.com/quocvuong92/ai-cli/internal/executor"
	settingspkg "github.com/quocvuong92/ai-cli/internal/settings"
)

// handleCommand processes slash commands in interactive mode.
// Returns true if the session should exit, false otherwise.
func (app *App) handleCommand(input string, messages *[]api.Message, client *api.AIClient, exec *executor.Executor, session *InteractiveSession) bool {
	parts := strings.SplitN(input, " ", 2)
	cmd := strings.ToLower(parts[0])

	switch cmd {
	case "/exit", "/quit", "/q":
		fmt.Println("Goodbye!")
		if session != nil {
			session.saveHistory()
		}
		return true

	case "/clear", "/c":
		*messages = []api.Message{
			{Role: "system", Content: config.DefaultSystemMessage},
		}
		// Start a new conversation ID when clearing
		if session != nil {
			session.conversationID = uuid.New().String()
		}
		fmt.Println("Conversation cleared.")

	case "/help", "/h":
		app.showHelp()

	case "/history":
		app.showHistory(session)

	case "/resume":
		app.resumeConversation(session, messages)

	case "/model":
		app.handleModelCommand(parts)

	case "/provider":
		if app.handleProviderCommand(parts, client) {
			// Provider switched, clear conversation history
			*messages = []api.Message{
				{Role: "system", Content: config.DefaultSystemMessage},
			}
		}

	case "/web":
		app.handleWebCommand(parts, messages, *client, exec, session)

	case "/allow-dangerous":
		exec.GetPermissionManager().EnableDangerous()
		fmt.Println("⚠️  Dangerous commands enabled for this session")
		fmt.Println("Note: You will still be asked to confirm before execution")

	case "/show-permissions":
		app.showPermissions(exec)

	case "/allow":
		app.handleAllowCommand(parts, exec)

	case "/deny":
		app.handleDenyCommand(parts, exec)

	case "/clear-session":
		exec.GetPermissionManager().ClearSessionAllowlist()
		fmt.Println("Session allowlist cleared.")

	case "/diff":
		app.handleDiffCommand()

	case "/commit":
		app.handleCommitCommand(session)

	case "/amend":
		app.handleAmendCommand(session)

	case "/plan":
		app.handlePlanCommand(session)

	default:
		fmt.Printf("Unknown command: %s\n", cmd)
		fmt.Println("Type /help for available commands")
	}

	return false
}

// showHelp displays the help message with all available commands.
func (app *App) showHelp() {
	fmt.Println("\nCommands:")
	fmt.Printf("  %-24s %s\n", "/exit, /quit, /q", "Exit interactive mode")
	fmt.Printf("  %-24s %s\n", "/clear, /c", "Clear conversation history")
	fmt.Printf("  %-24s %s\n", "/history", "Show recent conversations")
	fmt.Printf("  %-24s %s\n", "/resume", "Resume last conversation")
	fmt.Printf("  %-24s %s\n", "/web <query>", "Search web and ask about results")
	fmt.Printf("  %-24s %s\n", "/web on", "Enable auto web search for all messages")
	fmt.Printf("  %-24s %s\n", "/web off", "Disable auto web search")
	fmt.Printf("  %-24s %s\n", "/web <provider>", "Switch provider (tavily, linkup, brave)")
	fmt.Printf("  %-24s %s\n", "/model <name>", "Switch model")
	fmt.Printf("  %-24s %s\n", "/model", "Show current model")
	fmt.Printf("  %-24s %s\n", "/provider <name>", "Switch AI provider (copilot, azure)")
	fmt.Printf("  %-24s %s\n", "/provider", "Show current provider")
	fmt.Println()
	fmt.Println("Git commands:")
	fmt.Printf("  %-24s %s\n", "/diff", "Show current git changes (staged/unstaged)")
	fmt.Printf("  %-24s %s\n", "/commit", "AI-generate commit message and commit")
	fmt.Printf("  %-24s %s\n", "/amend", "AI-improve last commit message")
	fmt.Printf("  %-24s %s\n", "/plan", "Show current task plan/checklist")
	fmt.Println()
	fmt.Println("Permission commands:")
	fmt.Printf("  %-24s %s\n", "/allow-dangerous", "Allow dangerous commands (with confirmation)")
	fmt.Printf("  %-24s %s\n", "/show-permissions", "Show permission settings and rules")
	fmt.Printf("  %-24s %s\n", "/allow <pattern>", "Add persistent allow rule (e.g., git:*)")
	fmt.Printf("  %-24s %s\n", "/deny <pattern>", "Add persistent deny rule (takes precedence)")
	fmt.Printf("  %-24s %s\n", "/clear-session", "Clear session-only permissions")
	fmt.Printf("  %-24s %s\n", "/help, /h", "Show this help")
	fmt.Println()
}

// showHistory displays recent conversation history.
func (app *App) showHistory(session *InteractiveSession) {
	if session == nil || session.history == nil {
		fmt.Println("History not available.")
		return
	}

	conversations := session.history.GetRecentConversations(10)
	if len(conversations) == 0 {
		fmt.Println("No conversation history.")
		return
	}

	fmt.Println("\nRecent conversations:")
	for i, conv := range conversations {
		msgCount := len(conv.Messages) - 1 // Exclude system message
		if msgCount < 0 {
			msgCount = 0
		}
		fmt.Printf("  %d. [%s] %s - %s (%d messages)\n",
			i+1,
			conv.UpdatedAt.Format("2006-01-02 15:04"),
			conv.Provider,
			conv.Model,
			msgCount,
		)
	}
	fmt.Println()
}

// resumeConversation resumes the last conversation from history.
func (app *App) resumeConversation(session *InteractiveSession, messages *[]api.Message) {
	if session == nil || session.history == nil {
		fmt.Println("History not available.")
		return
	}

	lastConv := session.history.GetLastConversation()
	if lastConv == nil {
		fmt.Println("No conversation to resume.")
		return
	}

	*messages = make([]api.Message, len(lastConv.Messages))
	copy(*messages, lastConv.Messages)
	session.conversationID = lastConv.ID
	msgCount := len(lastConv.Messages) - 1
	if msgCount < 0 {
		msgCount = 0
	}
	fmt.Printf("Resumed conversation from %s (%d messages)\n\n",
		lastConv.UpdatedAt.Format("2006-01-02 15:04"),
		msgCount,
	)

	// Display the conversation history
	for _, msg := range lastConv.Messages {
		// Skip system messages
		if msg.Role == "system" {
			continue
		}

		// Display user messages
		if msg.Role == "user" {
			fmt.Printf("👤 You:\n%s\n\n", msg.Content)
		}

		// Display assistant messages
		if msg.Role == "assistant" && msg.Content != "" {
			fmt.Printf("🤖 Assistant:\n")
			if app.cfg.Render {
				display.ShowContentRendered(msg.Content)
			} else {
				display.ShowContent(msg.Content)
			}
			fmt.Println()
		}

		// Display tool messages (if any)
		if msg.Role == "tool" && msg.Content != "" {
			fmt.Printf("🔧 Tool Result:\n%s\n\n", msg.Content)
		}
	}

	fmt.Println("--- End of conversation history ---")
}

// handleModelCommand processes the /model command to show or switch models.
func (app *App) handleModelCommand(parts []string) {
	if len(parts) > 1 {
		newModel := strings.TrimSpace(parts[1])
		if newModel == "" {
			fmt.Printf("Current model: %s\n", app.cfg.Model)
			if len(app.cfg.AvailableModels) > 0 {
				fmt.Printf("Available: %s\n", app.cfg.GetAvailableModelsString())
			}
		} else if len(app.cfg.AvailableModels) > 0 && !app.cfg.ValidateModel(newModel) {
			fmt.Printf("Invalid model: %s\n", newModel)
			fmt.Printf("Available: %s\n", app.cfg.GetAvailableModelsString())
		} else {
			app.cfg.Model = newModel
			fmt.Printf("Switched to model: %s\n", app.cfg.Model)
		}
	} else {
		fmt.Printf("Current model: %s\n", app.cfg.Model)
		if len(app.cfg.AvailableModels) > 0 {
			fmt.Printf("Available: %s\n", app.cfg.GetAvailableModelsString())
		}
	}
}

// handleProviderCommand processes the /provider command to show or switch AI providers.
// Returns true if the provider was switched (conversation history is cleared), false otherwise.
func (app *App) handleProviderCommand(parts []string, client *api.AIClient) bool {
	if len(parts) > 1 {
		newProvider := strings.ToLower(strings.TrimSpace(parts[1]))
		if newProvider == "" {
			fmt.Printf("Current provider: %s\n", app.getProviderName())
			fmt.Println("Available: copilot, azure")
			return false
		}

		if newProvider != "copilot" && newProvider != "azure" && newProvider != "github" {
			fmt.Printf("Invalid provider: %s\n", newProvider)
			fmt.Println("Available: copilot, azure")
			return false
		}

		// Update config
		app.cfg.Provider = newProvider

		// Reset model to empty so Validate() will set it to the first available model
		app.cfg.Model = ""

		// Update available models based on new provider
		if err := app.cfg.Validate(); err != nil {
			display.ShowError(fmt.Sprintf("Configuration error: %v", err))
			return false
		}

		// Recreate client with new provider
		newClient, err := api.NewClient(app.cfg)
		if err != nil {
			display.ShowError(fmt.Sprintf("Failed to switch provider: %v", err))
			return false
		}

		// Close old client to stop background goroutines
		(*client).Close()
		*client = newClient

		fmt.Printf("✓ Switched to %s\n", app.getProviderName())
		fmt.Printf("  Model: %s\n", app.cfg.Model)
		fmt.Printf("  Available models: %s\n", app.cfg.GetAvailableModelsString())
		fmt.Println("  Conversation history cleared")
		return true
	}

	fmt.Printf("Current provider: %s\n", app.getProviderName())
	fmt.Println("Available: copilot, azure")
	return false
}

// handleWebCommand processes the /web command for web search operations.
// Supports: /web <query>, /web on/off, /web <provider>.
func (app *App) handleWebCommand(parts []string, messages *[]api.Message, client api.AIClient, exec *executor.Executor, session *InteractiveSession) {
	if len(parts) < 2 {
		status := "off"
		if app.cfg.WebSearch {
			status = fmt.Sprintf("on (provider: %s)", app.cfg.WebSearchProvider)
		}
		fmt.Printf("Web search: %s\n", status)
		fmt.Println("Available providers: tavily, linkup, brave")
		fmt.Println("Usage: /web <query> | /web on | /web off | /web provider <name>")
		return
	}

	arg := strings.TrimSpace(parts[1])
	switch strings.ToLower(arg) {
	case "on":
		app.cfg.WebSearch = true
		fmt.Printf("Web search enabled (provider: %s).\n", app.cfg.WebSearchProvider)
	case "off":
		app.cfg.WebSearch = false
		fmt.Println("Web search disabled.")
	case "provider":
		// Check if there's a provider name after "provider"
		providerParts := strings.SplitN(parts[1], " ", 2)
		if len(providerParts) > 1 {
			newProvider := strings.ToLower(strings.TrimSpace(providerParts[1]))
			if newProvider == "tavily" || newProvider == "linkup" || newProvider == "brave" {
				app.cfg.WebSearchProvider = newProvider
				fmt.Printf("Web search provider changed to: %s\n", app.cfg.WebSearchProvider)
			} else {
				fmt.Printf("Invalid provider: %s\n", newProvider)
				fmt.Println("Available providers: tavily, linkup, brave")
			}
		} else {
			fmt.Printf("Current provider: %s\n", app.cfg.WebSearchProvider)
			fmt.Println("Available providers: tavily, linkup, brave")
			fmt.Println("Usage: /web provider <name>")
		}
	case "tavily", "linkup", "brave":
		// Allow shorthand: /web tavily, /web linkup, /web brave
		app.cfg.WebSearchProvider = strings.ToLower(arg)
		fmt.Printf("Web search provider changed to: %s\n", app.cfg.WebSearchProvider)
	default:
		app.handleWebSearch(arg, messages, client, exec, session)
	}
}

// showPermissions displays current permission settings and rules.
func (app *App) showPermissions(exec *executor.Executor) {
	settings := exec.GetPermissionManager().GetSettings()
	display.ShowPermissionSettings(settings)

	// Show rules
	allowRules := exec.GetPermissionManager().GetAllowRules()
	denyRules := exec.GetPermissionManager().GetDenyRules()
	var allowStrs, denyStrs []string
	for _, r := range allowRules {
		allowStrs = append(allowStrs, settingspkg.FormatPattern(r))
	}
	for _, r := range denyRules {
		denyStrs = append(denyStrs, settingspkg.FormatPattern(r))
	}
	display.ShowPermissionRules(allowStrs, denyStrs)
}

// handleAllowCommand processes the /allow command to add allow rules.
func (app *App) handleAllowCommand(parts []string, exec *executor.Executor) {
	if len(parts) > 1 {
		pattern := strings.TrimSpace(parts[1])
		if err := exec.GetPermissionManager().AddPatternRule(pattern, false); err != nil {
			display.ShowError(fmt.Sprintf("Failed to add allow rule: %v", err))
		} else {
			fmt.Printf("✓ Added allow rule: %s\n", pattern)
		}
	} else {
		fmt.Println("Usage: /allow <pattern>")
		fmt.Println("Examples:")
		fmt.Println("  /allow git:*         Allow all git commands")
		fmt.Println("  /allow npm run *     Allow npm run with any script")
		fmt.Println("  /allow ls -la        Allow specific command")
	}
}

// handleDenyCommand processes the /deny command to add deny rules.
func (app *App) handleDenyCommand(parts []string, exec *executor.Executor) {
	if len(parts) > 1 {
		pattern := strings.TrimSpace(parts[1])
		if err := exec.GetPermissionManager().AddPatternRule(pattern, true); err != nil {
			display.ShowError(fmt.Sprintf("Failed to add deny rule: %v", err))
		} else {
			fmt.Printf("✓ Added deny rule: %s (takes precedence over allow rules)\n", pattern)
		}
	} else {
		fmt.Println("Usage: /deny <pattern>")
		fmt.Println("Examples:")
		fmt.Println("  /deny rm *           Block all rm commands")
		fmt.Println("  /deny curl *         Block all curl commands")
	}
}

// handleDiffCommand shows current git changes.
func (app *App) handleDiffCommand() {
	// Check if we're in a git repo
	if !isGitRepo() {
		fmt.Println("Not a git repository.")
		return
	}

	// Show staged changes first
	stagedOutput, _ := exec.Command("git", "diff", "--staged", "--stat").Output()
	if len(stagedOutput) > 0 {
		fmt.Println("📦 Staged changes:")
		fmt.Println(string(stagedOutput))
	}

	// Show unstaged changes
	unstagedOutput, _ := exec.Command("git", "diff", "--stat").Output()
	if len(unstagedOutput) > 0 {
		fmt.Println("📝 Unstaged changes:")
		fmt.Println(string(unstagedOutput))
	}

	// Show untracked files
	untrackedOutput, _ := exec.Command("git", "ls-files", "--others", "--exclude-standard").Output()
	if len(untrackedOutput) > 0 {
		fmt.Println("❓ Untracked files:")
		fmt.Println(string(untrackedOutput))
	}

	if len(stagedOutput) == 0 && len(unstagedOutput) == 0 && len(untrackedOutput) == 0 {
		fmt.Println("No changes detected.")
	}
}

// handleCommitCommand generates a commit message using AI and commits.
func (app *App) handleCommitCommand(session *InteractiveSession) {
	if !isGitRepo() {
		fmt.Println("Not a git repository.")
		return
	}

	// Get the diff - prefer staged, fall back to unstaged
	diff, _ := exec.Command("git", "diff", "--staged").Output()
	if len(diff) == 0 {
		diff, _ = exec.Command("git", "diff").Output()
		if len(diff) == 0 {
			fmt.Println("No changes to commit.")
			return
		}
		// Stage all changes
		fmt.Println("No staged changes. Staging all changes...")
		if _, err := exec.Command("git", "add", "-A").Output(); err != nil {
			display.ShowError(fmt.Sprintf("Failed to stage changes: %v", err))
			return
		}
		diff, _ = exec.Command("git", "diff", "--staged").Output()
	}

	// Truncate diff if too long (keep under 8K for context)
	diffStr := string(diff)
	if len(diffStr) > 8000 {
		diffStr = diffStr[:8000] + "\n... (truncated)"
	}

	// Ask AI to generate commit message
	fmt.Println("🤖 Generating commit message...")

	prompt := fmt.Sprintf(`Generate a concise git commit message for these changes. Follow conventional commits format (feat:, fix:, docs:, refactor:, etc.). Be specific but brief. Only output the commit message, nothing else.

Diff:
%s`, diffStr)

	// Send to AI
	resp, err := session.client.QueryWithHistory([]api.Message{
		{Role: "user", Content: prompt},
	})
	if err != nil {
		display.ShowError(fmt.Sprintf("Failed to generate message: %v", err))
		return
	}

	commitMsg := strings.TrimSpace(resp.GetContent())
	fmt.Printf("\n📝 Suggested commit message:\n%s\n\n", commitMsg)

	// Ask for confirmation
	fmt.Print("Commit with this message? [y]es / [e]dit / [n]o: ")
	var response string
	fmt.Scanln(&response)
	response = strings.ToLower(strings.TrimSpace(response))

	switch response {
	case "y", "yes":
		output, err := exec.Command("git", "commit", "-m", commitMsg).CombinedOutput()
		if err != nil {
			display.ShowError(fmt.Sprintf("Commit failed: %v\n%s", err, string(output)))
			return
		}
		fmt.Println("✓ Committed successfully!")
		fmt.Println(string(output))
	case "e", "edit":
		// Open editor for manual editing - need to connect stdin/stdout
		cmd := exec.Command("git", "commit")
		cmd.Stdin = os.Stdin
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			display.ShowError(fmt.Sprintf("Commit failed: %v", err))
			return
		}
		fmt.Println("✓ Committed successfully!")
	default:
		fmt.Println("Commit cancelled.")
	}
}

// handleAmendCommand amends the last commit with a new AI-generated message.
func (app *App) handleAmendCommand(session *InteractiveSession) {
	if !isGitRepo() {
		fmt.Println("Not a git repository.")
		return
	}

	// Get the last commit info
	lastMsg, _ := exec.Command("git", "log", "-1", "--pretty=%B").Output()
	lastDiff, _ := exec.Command("git", "diff", "HEAD~1", "--staged").Output()

	if len(lastDiff) == 0 {
		lastDiff, _ = exec.Command("git", "show", "--stat", "HEAD").Output()
	}

	diffStr := string(lastDiff)
	if len(diffStr) > 8000 {
		diffStr = diffStr[:8000] + "\n... (truncated)"
	}

	fmt.Printf("📝 Current commit message:\n%s\n", string(lastMsg))
	fmt.Println("🤖 Generating improved commit message...")

	prompt := fmt.Sprintf(`Improve this git commit message. Follow conventional commits format. Be specific but brief. Only output the new commit message, nothing else.

Current message: %s

Changes:
%s`, string(lastMsg), diffStr)

	resp, err := session.client.QueryWithHistory([]api.Message{
		{Role: "user", Content: prompt},
	})
	if err != nil {
		display.ShowError(fmt.Sprintf("Failed to generate message: %v", err))
		return
	}

	newMsg := strings.TrimSpace(resp.GetContent())
	fmt.Printf("\n📝 New commit message:\n%s\n\n", newMsg)

	fmt.Print("Amend with this message? [y]es / [n]o: ")
	var response string
	fmt.Scanln(&response)

	if strings.ToLower(strings.TrimSpace(response)) == "y" {
		output, err := exec.Command("git", "commit", "--amend", "-m", newMsg).CombinedOutput()
		if err != nil {
			display.ShowError(fmt.Sprintf("Amend failed: %v\n%s", err, string(output)))
			return
		}
		fmt.Println("✓ Commit amended successfully!")
		fmt.Println(string(output))
	} else {
		fmt.Println("Amend cancelled.")
	}
}

// handlePlanCommand shows or clears the current plan.
func (app *App) handlePlanCommand(session *InteractiveSession) {
	if session.currentPlan == nil || len(session.currentPlan.Items) == 0 {
		fmt.Println("No active plan. The AI will create one when working on complex tasks.")
		return
	}
	display.ShowPlan(session.currentPlan)
}

// isGitRepo checks if current directory is a git repository.
func isGitRepo() bool {
	_, err := exec.Command("git", "rev-parse", "--git-dir").Output()
	return err == nil
}
