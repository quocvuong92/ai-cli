// Package cmd implements the CLI commands for the AI CLI application.
//
// # Architecture
//
// This package is organized into the following logical groups:
//
// ## Core CLI
//
//   - root.go: Main entry point, App struct, cobra command setup, and flags
//   - run.go: Query execution logic (streaming and non-streaming modes)
//   - login.go: Authentication commands (login, logout, status)
//
// ## Interactive Mode
//
//   - interactive.go: Interactive REPL session management, context handling
//   - slash_commands.go: Slash command handlers (/model, /web, /git, etc.)
//   - websearch.go: Web search integration and query optimization
//
// ## Tool/Function Calling
//
//   - tool_handlers.go: Handlers for AI tool calls (execute_command, read_file, etc.)
//   - constants.go: Prompts and configuration constants
//
// # Key Components
//
// ## App
//
// The App struct holds application state and configuration. It's created
// in Execute() and passed through command handlers.
//
// ## InteractiveSession
//
// Manages interactive chat sessions including:
//   - Conversation history
//   - Command execution with permission checking
//   - Multiline input handling
//   - Graceful Ctrl+C cancellation
//
// ## Tool Processing
//
// The processToolCall function in tool_handlers.go handles AI function calls:
//   - execute_command: Run shell commands with permission checking
//   - read_file, write_file, edit_file, delete_file: File operations
//   - search_files, list_directory: File system exploration
//   - update_plan: Task planning and checklist management
//
// # Usage
//
//	// Main entry point
//	func main() {
//	    cmd.Execute()
//	}
package cmd
