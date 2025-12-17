// Package cmd implements the CLI commands for the AI CLI application.
package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/elk-language/go-prompt"
	istrings "github.com/elk-language/go-prompt/strings"
	"github.com/google/uuid"
	"github.com/quocvuong92/ai-cli/internal/api"
	"github.com/quocvuong92/ai-cli/internal/auth"
	"github.com/quocvuong92/ai-cli/internal/config"
	"github.com/quocvuong92/ai-cli/internal/display"
	"github.com/quocvuong92/ai-cli/internal/executor"
	"github.com/quocvuong92/ai-cli/internal/history"
	settingspkg "github.com/quocvuong92/ai-cli/internal/settings"
)

// InteractiveSession holds the state for an interactive chat session.
// It manages conversation history, command execution, and persistence.
type InteractiveSession struct {
	app            *App
	client         api.AIClient
	exec           *executor.Executor
	messages       []api.Message
	exitFlag       bool
	inputBuffer    []string // Buffer for multiline input
	history        *history.History
	conversationID string
}

// completer provides auto-completion suggestions for slash commands.
// It provides context-aware suggestions based on what the user is typing.
func (s *InteractiveSession) completer(d prompt.Document) ([]prompt.Suggest, istrings.RuneNumber, istrings.RuneNumber) {
	text := d.TextBeforeCursor()
	endIndex := d.CurrentRuneIndex()
	w := d.GetWordBeforeCursor()
	startIndex := endIndex - istrings.RuneCountInString(w)

	// Only show suggestions when input starts with "/"
	if !strings.HasPrefix(text, "/") {
		return []prompt.Suggest{}, startIndex, endIndex
	}

	// Context-aware suggestions based on command being typed
	textLower := strings.ToLower(text)

	// /model <name> - suggest available models
	if strings.HasPrefix(textLower, "/model ") {
		var suggestions []prompt.Suggest
		for _, model := range s.app.cfg.AvailableModels {
			desc := ""
			if model == s.app.cfg.Model {
				desc = "(current)"
			}
			suggestions = append(suggestions, prompt.Suggest{Text: model, Description: desc})
		}
		return prompt.FilterHasPrefix(suggestions, w, true), startIndex, endIndex
	}

	// /provider <name> - suggest providers
	if strings.HasPrefix(textLower, "/provider ") {
		suggestions := []prompt.Suggest{
			{Text: "copilot", Description: "GitHub Copilot (free with Pro)"},
			{Text: "azure", Description: "Azure OpenAI"},
		}
		return prompt.FilterHasPrefix(suggestions, w, true), startIndex, endIndex
	}

	// /web <option> - suggest web options
	if strings.HasPrefix(textLower, "/web ") {
		suggestions := []prompt.Suggest{
			{Text: "on", Description: "Enable auto web search for all messages"},
			{Text: "off", Description: "Disable auto web search"},
			{Text: "tavily", Description: "Use Tavily search provider"},
			{Text: "linkup", Description: "Use Linkup search provider"},
			{Text: "brave", Description: "Use Brave search provider"},
		}
		return prompt.FilterHasPrefix(suggestions, w, true), startIndex, endIndex
	}

	// Main command suggestions
	suggestions := []prompt.Suggest{
		// Most used commands first
		{Text: "/model", Description: "Show/switch model (current: " + s.app.cfg.Model + ")"},
		{Text: "/clear", Description: "Clear conversation history"},
		{Text: "/web", Description: "Web search commands"},
		{Text: "/help", Description: "Show all available commands"},
		{Text: "/exit", Description: "Exit interactive mode"},

		// History commands
		{Text: "/history", Description: "Show recent conversations"},
		{Text: "/resume", Description: "Resume last conversation"},

		// Provider
		{Text: "/provider", Description: "Show/switch provider (current: " + s.app.getProviderName() + ")"},

		// Permission commands
		{Text: "/show-permissions", Description: "Show command execution permissions"},
		{Text: "/allow-dangerous", Description: "Enable dangerous commands"},
		{Text: "/allow", Description: "Add allow rule (e.g., /allow git:*)"},
		{Text: "/deny", Description: "Add deny rule (e.g., /deny rm *)"},
		{Text: "/clear-session", Description: "Clear session-only permissions"},

		// Aliases
		{Text: "/q", Description: "Exit (alias)"},
		{Text: "/c", Description: "Clear (alias)"},
		{Text: "/h", Description: "Help (alias)"},
	}

	return prompt.FilterHasPrefix(suggestions, w, true), startIndex, endIndex
}

// runInteractive starts the interactive chat mode with a REPL interface.
// It initializes the AI client, sets up command completion, and handles
// user input until the session is terminated. Supports multiline input
// with backslash continuation and various slash commands.
func (app *App) runInteractive() {
	// Create AI client
	client, err := api.NewClient(app.cfg)
	if err != nil {
		display.ShowError(err.Error())
		return
	}
	// Ensure client resources are cleaned up on exit
	defer client.Close()

	fmt.Println("AI CLI - Interactive Mode")
	fmt.Printf("Model: %s\n", app.cfg.Model)
	fmt.Printf("Provider: %s\n", app.getProviderName())
	if app.cfg.WebSearch {
		fmt.Printf("Web search: enabled (provider: %s)\n", app.cfg.WebSearchProvider)
	}
	fmt.Println("Type /help for commands, Ctrl+C or Ctrl+D to quit")
	fmt.Println("Commands auto-complete as you type")
	fmt.Println("End a line with \\ for multiline input")
	fmt.Println()

	// Initialize history
	hist := history.NewHistory()
	if err := hist.Load(); err != nil {
		// History load failed, continue without it
		fmt.Fprintf(os.Stderr, "Note: Could not load history: %v\n", err)
	}

	session := &InteractiveSession{
		app:    app,
		client: client,
		exec:   executor.NewExecutor(),
		messages: []api.Message{
			{Role: "system", Content: config.DefaultSystemMessage},
		},
		exitFlag:       false,
		history:        hist,
		conversationID: uuid.New().String(),
	}

	p := prompt.New(
		session.executor,
		prompt.WithCompleter(session.completer),
		prompt.WithPrefix("> "),
		prompt.WithTitle("AI CLI"),
		prompt.WithPrefixTextColor(prompt.Green),
		// Suggestion box styling - better contrast and visibility
		prompt.WithSuggestionBGColor(prompt.DarkBlue),
		prompt.WithSuggestionTextColor(prompt.White),
		prompt.WithSelectedSuggestionBGColor(prompt.Cyan),
		prompt.WithSelectedSuggestionTextColor(prompt.Black),
		prompt.WithDescriptionBGColor(prompt.DarkBlue),
		prompt.WithDescriptionTextColor(prompt.LightGray),
		prompt.WithSelectedDescriptionBGColor(prompt.Cyan),
		prompt.WithSelectedDescriptionTextColor(prompt.Black),
		prompt.WithScrollbarBGColor(prompt.DarkGray),
		prompt.WithScrollbarThumbColor(prompt.White),
		// Show more suggestions at once
		prompt.WithMaxSuggestion(15),
		prompt.WithCompletionOnDown(),
		prompt.WithExitChecker(func(in string, breakline bool) bool {
			return session.exitFlag
		}),
		prompt.WithKeyBind(prompt.KeyBind{
			Key: prompt.ControlC,
			Fn: func(p *prompt.Prompt) bool {
				fmt.Println("\nGoodbye!")
				session.saveHistory()
				session.exitFlag = true
				return false
			},
		}),
		prompt.WithKeyBind(prompt.KeyBind{
			Key: prompt.ControlD,
			Fn: func(p *prompt.Prompt) bool {
				if p.Buffer().Text() == "" {
					fmt.Println("Goodbye!")
					session.saveHistory()
					session.exitFlag = true
				}
				return false
			},
		}),
	)

	p.Run()
}

// saveHistory persists the current conversation to the history file.
// Only saves if there are messages beyond the initial system prompt.
func (s *InteractiveSession) saveHistory() {
	if s.history == nil {
		return
	}
	// Only save if there are messages beyond the system prompt
	if len(s.messages) > 1 {
		s.history.AddConversation(
			s.conversationID,
			s.app.cfg.Model,
			s.app.getProviderName(),
			s.messages,
		)
		if err := s.history.Save(); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: Could not save history: %v\n", err)
		}
	}
}

// executor handles the execution of each input line in the REPL.
// It processes multiline input (backslash continuation), slash commands,
// web search queries, and regular chat messages with tool support.
func (s *InteractiveSession) executor(input string) {
	// Check if we should exit
	if s.exitFlag {
		return
	}

	// Handle multiline input with backslash continuation
	if strings.HasSuffix(input, "\\") {
		// Remove the trailing backslash and add to buffer
		line := strings.TrimSuffix(input, "\\")
		s.inputBuffer = append(s.inputBuffer, line)
		fmt.Print("... ") // Show continuation prompt
		return
	}

	// If we have buffered lines, combine them with current input
	if len(s.inputBuffer) > 0 {
		s.inputBuffer = append(s.inputBuffer, input)
		input = strings.Join(s.inputBuffer, "\n")
		s.inputBuffer = nil // Clear the buffer
	}

	input = strings.TrimSpace(input)
	if input == "" {
		return
	}

	// Handle commands (only if not in multiline mode - first line determines if it's a command)
	if strings.HasPrefix(input, "/") {
		if s.app.handleCommand(input, &s.messages, &s.client, s.exec, s) {
			s.exitFlag = true
		}
		return
	}

	// Web search mode: automatically search for every message
	if s.app.cfg.WebSearch {
		s.app.handleWebSearch(input, &s.messages, s.client, s.exec)
		return
	}

	// Regular chat with tool support
	s.messages = append(s.messages, api.Message{Role: "user", Content: input})
	fmt.Println()
	response, err := s.app.sendInteractiveMessageWithTools(s.client, s.exec, &s.messages)
	if err != nil {
		display.ShowError(err.Error())
		s.messages = s.messages[:len(s.messages)-1]
		return
	}
	if response != "" {
		s.messages = append(s.messages, api.Message{Role: "assistant", Content: response})
	}
	fmt.Println()
}

// getProviderName returns a human-readable provider name.
// It checks explicit provider setting first, then auto-detects based on
// available credentials (GitHub Copilot login or Azure environment variables).
func (app *App) getProviderName() string {
	// Check explicit provider setting first
	switch app.cfg.Provider {
	case "copilot", "github":
		return "GitHub Copilot"
	case "azure":
		return "Azure OpenAI"
	}

	// Auto-detect based on what NewClient will choose
	if auth.IsLoggedIn() {
		return "GitHub Copilot"
	}

	if app.cfg.AzureEndpoint != "" && app.cfg.AzureAPIKey != "" {
		return "Azure OpenAI"
	}

	return "Unknown"
}

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
		fmt.Printf("  %-24s %s\n", "/allow-dangerous", "Allow dangerous commands (with confirmation)")
		fmt.Printf("  %-24s %s\n", "/show-permissions", "Show permission settings and rules")
		fmt.Printf("  %-24s %s\n", "/allow <pattern>", "Add persistent allow rule (e.g., git:*)")
		fmt.Printf("  %-24s %s\n", "/deny <pattern>", "Add persistent deny rule (takes precedence)")
		fmt.Printf("  %-24s %s\n", "/clear-session", "Clear session-only permissions")
		fmt.Printf("  %-24s %s\n", "/help, /h", "Show this help")
		fmt.Println()

	case "/history":
		if session == nil || session.history == nil {
			fmt.Println("History not available.")
		} else {
			conversations := session.history.GetRecentConversations(10)
			if len(conversations) == 0 {
				fmt.Println("No conversation history.")
			} else {
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
		}

	case "/resume":
		if session == nil || session.history == nil {
			fmt.Println("History not available.")
		} else {
			lastConv := session.history.GetLastConversation()
			if lastConv == nil {
				fmt.Println("No conversation to resume.")
			} else {
				*messages = make([]api.Message, len(lastConv.Messages))
				copy(*messages, lastConv.Messages)
				session.conversationID = lastConv.ID
				msgCount := len(lastConv.Messages) - 1
				if msgCount < 0 {
					msgCount = 0
				}
				fmt.Printf("Resumed conversation from %s (%d messages)\n",
					lastConv.UpdatedAt.Format("2006-01-02 15:04"),
					msgCount,
				)
			}
		}

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
		app.handleWebCommand(parts, messages, *client, exec)

	case "/allow-dangerous":
		exec.GetPermissionManager().EnableDangerous()
		fmt.Println("⚠️  Dangerous commands enabled for this session")
		fmt.Println("Note: You will still be asked to confirm before execution")

	case "/show-permissions":
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

	case "/allow":
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

	case "/deny":
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

	case "/clear-session":
		exec.GetPermissionManager().ClearSessionAllowlist()
		fmt.Println("Session allowlist cleared.")

	default:
		fmt.Printf("Unknown command: %s\n", cmd)
		fmt.Println("Type /help for available commands")
	}

	return false
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
	} else {
		fmt.Printf("Current provider: %s\n", app.getProviderName())
		fmt.Println("Available: copilot, azure")
		return false
	}
}

// handleWebCommand processes the /web command for web search operations.
// Supports: /web <query>, /web on/off, /web <provider>.
func (app *App) handleWebCommand(parts []string, messages *[]api.Message, client api.AIClient, exec *executor.Executor) {
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
		app.handleWebSearch(arg, messages, client, exec)
	}
}

// sendInteractiveMessage sends a message to the AI and displays the response.
// Supports both streaming and non-streaming modes based on app configuration.
func (app *App) sendInteractiveMessage(client api.AIClient, messages []api.Message) (string, error) {
	if app.cfg.Stream {
		var fullContent strings.Builder
		firstChunk := true

		sp := display.NewSpinner("Thinking...")
		sp.Start()

		err := client.QueryStreamWithHistory(messages,
			func(content string) {
				if firstChunk {
					firstChunk = false
					if app.cfg.Render {
						sp.UpdateMessage("Receiving...")
					} else {
						sp.Stop()
					}
				}
				if app.cfg.Render {
					fullContent.WriteString(content)
				} else {
					fmt.Print(content)
				}
			},
			nil,
		)

		sp.Stop()

		if err != nil {
			return "", err
		}

		if app.cfg.Render {
			display.ShowContentRendered(fullContent.String())
			return fullContent.String(), nil
		}
		fmt.Println()
		return fullContent.String(), nil
	}

	// Non-streaming
	sp := display.NewSpinner("Thinking...")
	sp.Start()

	resp, err := client.QueryWithHistory(messages)
	sp.Stop()

	if err != nil {
		return "", err
	}

	content := resp.GetContent()
	if app.cfg.Render {
		display.ShowContentRendered(content)
	} else {
		display.ShowContent(content)
	}

	return content, nil
}

// sendInteractiveMessageWithTools sends a message with tool/function calling support.
// It handles the execute_command tool, processing permission checks and user confirmations
// before executing shell commands. Continues the conversation loop until no more tool
// calls are requested by the AI.
func (app *App) sendInteractiveMessageWithTools(client api.AIClient, exec *executor.Executor, messages *[]api.Message) (string, error) {
	ctx := context.Background()
	tools := api.GetDefaultTools()

	// Keep calling the API until there are no more tool calls
	for {
		var resp *api.ChatResponse
		var err error

		if app.cfg.Stream {
			// Streaming mode
			var fullContent strings.Builder
			firstChunk := true

			sp := display.NewSpinner("Thinking...")
			sp.Start()

			err = client.QueryStreamWithHistoryAndToolsContext(ctx, *messages, tools,
				func(content string) {
					if firstChunk {
						firstChunk = false
						if app.cfg.Render {
							sp.UpdateMessage("Receiving...")
						} else {
							sp.Stop()
						}
					}
					if app.cfg.Render {
						fullContent.WriteString(content)
					} else {
						fmt.Print(content)
					}
				},
				func(finalResp *api.ChatResponse) {
					resp = finalResp
				},
			)

			sp.Stop()

			if err != nil {
				return "", err
			}

			// If no tool calls were made and we have content, display final rendered content
			if resp != nil && !resp.Choices[0].HasToolCalls() {
				if app.cfg.Render && fullContent.Len() > 0 {
					display.ShowContentRendered(fullContent.String())
				} else if !app.cfg.Render {
					fmt.Println() // Add newline after streaming
				}
				return resp.GetContent(), nil
			}
		} else {
			// Non-streaming mode
			sp := display.NewSpinner("Thinking...")
			sp.Start()

			resp, err = client.QueryWithHistoryAndToolsContext(ctx, *messages, tools)
			sp.Stop()

			if err != nil {
				return "", err
			}
		}

		// Check if there are tool calls
		if len(resp.Choices) > 0 && resp.Choices[0].HasToolCalls() {
			toolCalls := resp.Choices[0].GetToolCalls()

			// Add assistant message with tool calls to history
			// Include content from response (may be empty, but structure matches API response)
			assistantMsg := api.Message{
				Role:      "assistant",
				ToolCalls: toolCalls,
			}
			// Only set content if it's not empty
			if resp.Choices[0].Message.Content != "" {
				assistantMsg.Content = resp.Choices[0].Message.Content
			}
			*messages = append(*messages, assistantMsg)

			// Process each tool call
			for _, toolCall := range toolCalls {
				if toolCall.Function.Name == "execute_command" {
					// Parse arguments
					var args struct {
						Command   string `json:"command"`
						Reasoning string `json:"reasoning"`
					}
					if err := json.Unmarshal([]byte(toolCall.Function.Arguments), &args); err != nil {
						display.ShowError(fmt.Sprintf("Failed to parse tool arguments: %v", err))
						continue
					}

					// Check permission
					allowed, needsConfirm, reason := exec.GetPermissionManager().CheckPermission(args.Command)

					var result *executor.ExecutionResult
					var toolResult string

					if !allowed && !needsConfirm {
						// Blocked
						display.ShowCommandBlocked(args.Command, reason)
						toolResult = fmt.Sprintf("Command blocked: %s", reason)
					} else {
						// Ask for confirmation if needed
						if needsConfirm {
							choice := display.AskCommandConfirmationExtended(args.Command, args.Reasoning)
							if choice == display.ApprovalDenied {
								toolResult = "Command execution denied by user"
								*messages = append(*messages, api.Message{
									Role:       "tool",
									Content:    toolResult,
									ToolCallID: toolCall.ID,
								})
								continue
							}
							// Handle different approval types
							switch choice {
							case display.ApprovalSession:
								_ = exec.GetPermissionManager().AddToAllowlist(args.Command, executor.ApprovalSession)
							case display.ApprovalAlways:
								if err := exec.GetPermissionManager().AddToAllowlist(args.Command, executor.ApprovalAlways); err != nil {
									fmt.Fprintf(os.Stderr, "Warning: Failed to save permission: %v\n", err)
								}
							}
						}

						// Execute the command
						display.ShowCommandExecuting(args.Command)
						result, err = exec.Execute(ctx, args.Command)

						if err != nil || !result.IsSuccess() {
							display.ShowCommandError(args.Command, result.Error)
							toolResult = result.FormatResult()
						} else {
							display.ShowCommandOutput(result.Output)
							toolResult = result.Output
							if toolResult == "" {
								toolResult = "Command executed successfully (no output)"
							}
						}
					}

					// Add tool result to messages
					*messages = append(*messages, api.Message{
						Role:       "tool",
						Content:    toolResult,
						ToolCallID: toolCall.ID,
					})
				}
			}

			// Continue loop to get AI's response to the tool results
			continue
		}

		// No tool calls, display the final response
		content := resp.GetContent()
		if content != "" {
			if app.cfg.Render {
				display.ShowContentRendered(content)
			} else {
				display.ShowContent(content)
			}
		}

		return content, nil
	}
}
