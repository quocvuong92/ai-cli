package cmd

import (
	"fmt"
	"io"
	"log"
	"os"

	"github.com/spf13/cobra"

	"github.com/quocvuong92/ai-cli/internal/api"
	"github.com/quocvuong92/ai-cli/internal/config"
	"github.com/quocvuong92/ai-cli/internal/display"
)

// App holds the application state
type App struct {
	cfg           *config.Config
	client        api.AIClient
	verbose       bool
	listModels    bool
	searchResults *api.TavilyResponse // Store search results for citations
}

// NewApp creates a new App instance with default configuration
func NewApp() *App {
	return &App{
		cfg: config.NewConfig(),
	}
}

// Execute runs the root command
func Execute() {
	app := NewApp()

	rootCmd := &cobra.Command{
		Use:   "ai-cli [query]",
		Short: "A CLI client for AI models with web search",
		Long: `AI CLI is a command-line client for AI models (GitHub Copilot, Azure OpenAI),
with optional web search powered by Tavily, Linkup, or Brave.

Supports multiple providers and API keys with automatic rotation.

Examples:
  ai-cli "What is Kubernetes?"
  ai-cli -m gpt-4o "Explain Docker"
  ai-cli --web "Latest news on Go 1.24"
  ai-cli --web --provider brave "Latest AI news"
  ai-cli -i                             # Interactive mode
  ai-cli -ir                            # Interactive with markdown rendering`,
		Args: cobra.MaximumNArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			app.run(cmd, args)
		},
	}

	rootCmd.Flags().BoolVarP(&app.verbose, "verbose", "v", false, "Enable debug mode")
	rootCmd.Flags().BoolVarP(&app.cfg.Usage, "usage", "u", false, "Show token usage statistics")
	rootCmd.Flags().BoolVarP(&app.cfg.Stream, "stream", "s", false, "Stream output in real-time")
	rootCmd.Flags().BoolVarP(&app.cfg.Render, "render", "r", false, "Render markdown with colors and formatting")
	rootCmd.Flags().BoolVarP(&app.cfg.WebSearch, "web", "w", false, "Search web first (requires TAVILY_API_KEYS, LINKUP_API_KEYS, or BRAVE_API_KEYS)")
	rootCmd.Flags().BoolVarP(&app.cfg.Citations, "citations", "c", false, "Show citations/sources from web search")
	rootCmd.Flags().BoolVarP(&app.cfg.Interactive, "interactive", "i", false, "Interactive chat mode")
	rootCmd.Flags().StringVarP(&app.cfg.Model, "model", "m", "", "Model name (e.g., gpt-4.1, claude-3.7-sonnet)")
	rootCmd.Flags().StringVarP(&app.cfg.WebSearchProvider, "search-provider", "p", "", "Web search provider: tavily, linkup, or brave (default: auto-detect)")
	rootCmd.Flags().StringVar(&app.cfg.Provider, "provider", "", "AI provider: copilot, azure (default: auto-detect)")
	rootCmd.Flags().BoolVar(&app.listModels, "list-models", false, "List available models")

	// Add subcommands
	rootCmd.AddCommand(NewLoginCmd())
	rootCmd.AddCommand(NewLogoutCmd())
	rootCmd.AddCommand(NewStatusCmd())

	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func (app *App) run(cmd *cobra.Command, args []string) {
	if app.verbose {
		log.SetOutput(os.Stderr)
		log.SetFlags(log.Ldate | log.Ltime | log.Lshortfile)
	} else {
		log.SetOutput(io.Discard)
	}

	// Handle --list-models flag
	if app.listModels {
		_ = app.cfg.Validate()
		if len(app.cfg.AvailableModels) == 0 {
			fmt.Println("No models configured.")
			fmt.Println("Run 'ai-cli login' for GitHub Copilot or set AZURE_OPENAI_MODELS.")
			os.Exit(1)
		}
		display.ShowModels(app.cfg.AvailableModels, app.cfg.Model)
		return
	}

	// Validate config
	if err := app.cfg.Validate(); err != nil {
		display.ShowError(err.Error())
		os.Exit(1)
	}

	// Initialize markdown renderer if render flag is set
	if app.cfg.Render {
		if err := display.InitRenderer(); err != nil {
			log.Printf("Failed to initialize renderer: %v", err)
		}
	}

	// Interactive mode
	if app.cfg.Interactive {
		app.runInteractive()
		return
	}

	// Require query if not interactive mode
	if len(args) == 0 {
		_ = cmd.Help()
		os.Exit(1)
	}

	query := args[0]
	log.Printf("Query: %s", query)
	log.Printf("Model: %s", app.cfg.Model)
	log.Printf("Stream: %v", app.cfg.Stream)
	log.Printf("WebSearch: %v", app.cfg.WebSearch)

	// Build system prompt and user message
	systemPrompt := config.DefaultSystemMessage
	userMessage := query

	// Web search if requested
	if app.cfg.WebSearch {
		searchContext, err := app.performWebSearch(query)
		if err != nil {
			display.ShowError(err.Error())
			os.Exit(1)
		}
		systemPrompt = buildWebSearchPrompt(searchContext)
	}

	// Create AI client (auto-detects provider)
	client, err := api.NewClient(app.cfg)
	if err != nil {
		display.ShowError(err.Error())
		os.Exit(1)
	}
	app.client = client

	log.Printf("Sending request...")

	if app.cfg.Stream {
		app.runStream(client, systemPrompt, userMessage)
	} else {
		app.runNormal(client, systemPrompt, userMessage)
	}

	// Show citations if web search was used and citations flag is set
	if app.cfg.WebSearch && app.cfg.Citations && app.searchResults != nil && len(app.searchResults.Results) > 0 {
		fmt.Println()
		citations := make([]display.Citation, len(app.searchResults.Results))
		for i, r := range app.searchResults.Results {
			citations[i] = display.Citation{Title: r.Title, URL: r.URL}
		}
		display.ShowCitations(citations)
	}
}
