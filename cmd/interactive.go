// Package cmd implements the CLI commands for the AI CLI application.
package cmd

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"

	"github.com/elk-language/go-prompt"
	istrings "github.com/elk-language/go-prompt/strings"
	"github.com/google/uuid"
	"github.com/quocvuong92/ai-cli/internal/api"
	"github.com/quocvuong92/ai-cli/internal/auth"
	"github.com/quocvuong92/ai-cli/internal/config"
	"github.com/quocvuong92/ai-cli/internal/display"
	"github.com/quocvuong92/ai-cli/internal/executor"
	"github.com/quocvuong92/ai-cli/internal/history"
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
	interruptCtx   *InterruptibleContext // For graceful Ctrl+C cancellation
	currentPlan    *display.Plan         // Current task plan/checklist
}

// InterruptibleContext manages a cancellable context for operations.
// It allows Ctrl+C to cancel the current operation instead of exiting the CLI.
type InterruptibleContext struct {
	ctx    context.Context
	cancel context.CancelFunc
	mu     sync.Mutex
	active bool
}

// NewInterruptibleContext creates a new interruptible context manager.
func NewInterruptibleContext() *InterruptibleContext {
	return &InterruptibleContext{}
}

// Start begins an interruptible operation, returning a context that will be
// cancelled if Ctrl+C is pressed during the operation.
func (ic *InterruptibleContext) Start() context.Context {
	ic.mu.Lock()
	defer ic.mu.Unlock()

	ic.ctx, ic.cancel = context.WithCancel(context.Background())
	ic.active = true

	// Set up signal handler for this operation
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGINT)

	go func() {
		select {
		case <-sigChan:
			ic.mu.Lock()
			if ic.active {
				fmt.Fprintf(os.Stderr, "\n⚠️  Operation cancelled\n")
				ic.cancel()
			}
			ic.mu.Unlock()
		case <-ic.ctx.Done():
			// Context completed normally
		}
		signal.Stop(sigChan)
		close(sigChan)
	}()

	return ic.ctx
}

// Stop ends the interruptible operation and cleans up.
func (ic *InterruptibleContext) Stop() {
	ic.mu.Lock()
	defer ic.mu.Unlock()

	ic.active = false
	if ic.cancel != nil {
		ic.cancel()
	}
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

		// Git commands
		{Text: "/diff", Description: "Show current git changes"},
		{Text: "/commit", Description: "AI-generate commit message and commit"},
		{Text: "/amend", Description: "AI-improve last commit message"},
		{Text: "/plan", Description: "Show current task plan/checklist"},

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
		interruptCtx:   NewInterruptibleContext(),
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
		s.app.handleWebSearch(input, &s.messages, s.client, s.exec, s)
		return
	}

	// Regular chat with tool support
	s.messages = append(s.messages, api.Message{Role: "user", Content: input})
	fmt.Println()
	response, err := s.app.sendInteractiveMessageWithTools(s.client, s.exec, &s.messages, s.interruptCtx, s)
	if err != nil {
		// Check if it was a cancellation
		if err == context.Canceled {
			// Remove the user message since we didn't complete
			s.messages = s.messages[:len(s.messages)-1]
			return
		}
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
func (app *App) sendInteractiveMessageWithTools(client api.AIClient, exec *executor.Executor, messages *[]api.Message, interruptCtx *InterruptibleContext, session *InteractiveSession) (string, error) {
	// Start interruptible context - Ctrl+C will cancel this operation
	ctx := interruptCtx.Start()
	defer interruptCtx.Stop()

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
				toolResult := app.processToolCall(toolCall, exec, ctx, session)
				*messages = append(*messages, api.Message{
					Role:       "tool",
					Content:    toolResult,
					ToolCallID: toolCall.ID,
				})
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
