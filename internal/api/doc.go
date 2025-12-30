// Package api provides unified AI client interfaces and web search functionality.
//
// # Architecture
//
// This package is organized into the following logical groups:
//
// ## AI Clients
//
// The package provides a unified interface for multiple AI providers:
//
//   - client.go: AIClient interface and factory function (NewClient)
//   - copilot.go: GitHub Copilot API client implementation
//   - azure.go: Azure OpenAI API client implementation
//   - stream.go: Server-Sent Events (SSE) processor for streaming responses
//   - tools.go: Tool/function definitions for AI function calling
//
// ## Web Search Clients
//
// The package also provides web search functionality through multiple providers:
//
//   - search.go: SearchClient interface and unified response types
//   - search_base.go: Base search client with key rotation and retry logic
//   - tavily.go: Tavily search provider implementation
//   - linkup.go: Linkup search provider implementation
//   - brave.go: Brave search provider implementation
//
// ## Retry and Error Handling
//
//   - retry.go: Exponential backoff retry logic for both AI and search clients
//
// # Usage
//
// ## Creating an AI Client
//
//	cfg := config.NewConfig()
//	cfg.Validate()
//	client, err := api.NewClient(cfg)
//	if err != nil {
//	    // handle error
//	}
//	defer client.Close()
//
// ## Creating a Search Client
//
//	keyRotator := config.NewKeyRotator("TAVILY_API_KEYS")
//	searchClient := api.NewTavilyClient(keyRotator)
//	results, err := searchClient.Search(ctx, "query")
//
// # Interface Design
//
// Both AIClient and SearchClient interfaces support dependency injection
// for easier testing. The concrete implementations can be mocked using
// the interface types.
package api
