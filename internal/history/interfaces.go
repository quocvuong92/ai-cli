// Package history provides conversation history persistence for interactive sessions.
package history

import "github.com/quocvuong92/ai-cli/internal/api"

// HistoryManager defines the interface for managing conversation history.
// This interface enables dependency injection and easier testing.
type HistoryManager interface {
	// Load reads the history from disk
	Load() error

	// Save writes the history to disk
	Save() error

	// AddConversation adds a new conversation to history
	AddConversation(id, model, provider string, messages []api.Message)

	// UpdateConversation updates an existing conversation
	UpdateConversation(id string, messages []api.Message) bool

	// GetConversation retrieves a conversation by ID
	GetConversation(id string) *ConversationEntry

	// GetLastConversation returns the most recent conversation
	GetLastConversation() *ConversationEntry

	// GetRecentConversations returns the N most recent conversations
	GetRecentConversations(n int) []ConversationEntry

	// Clear removes all conversation history
	Clear()
}

// Ensure concrete type implements the interface
var _ HistoryManager = (*History)(nil)
