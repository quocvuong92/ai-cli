// Package executor provides command execution with permission checking.
package executor

import (
	"context"
	"time"

	"github.com/quocvuong92/ai-cli/internal/settings"
)

// CommandExecutor defines the interface for executing shell commands.
// This interface enables dependency injection and easier testing.
type CommandExecutor interface {
	// Execute runs a shell command and returns the result
	Execute(ctx context.Context, command string) (*ExecutionResult, error)

	// GetPermissionManager returns the permission manager
	GetPermissionManager() PermissionChecker

	// SetTimeout sets the command execution timeout
	SetTimeout(timeout time.Duration)
}

// PermissionChecker defines the interface for checking command permissions.
// This interface enables dependency injection and easier testing of permission logic.
type PermissionChecker interface {
	// CheckPermission checks if a command is allowed to execute
	// Returns: (allowed, needsConfirm, reason)
	CheckPermission(cmd string) (allowed bool, needsConfirm bool, reason string)

	// AddToAllowlist adds a command based on approval type
	AddToAllowlist(cmd string, approvalType ApprovalType) error

	// AddPatternRule adds a pattern-based rule to persistent settings
	AddPatternRule(pattern string, deny bool) error

	// EnableDangerous enables execution of dangerous commands (with confirmation)
	EnableDangerous()

	// DisableDangerous disables execution of dangerous commands
	DisableDangerous()

	// SetAutoAllowReads sets whether to auto-allow safe read-only commands
	SetAutoAllowReads(enabled bool)

	// GetSettings returns current permission settings for display
	GetSettings() map[string]interface{}

	// GetAllowRules returns the current allow rules
	GetAllowRules() []settings.PermissionRule

	// GetDenyRules returns the current deny rules
	GetDenyRules() []settings.PermissionRule

	// ClearSessionAllowlist clears all session-only permissions
	ClearSessionAllowlist()

	// SaveSettings saves current settings to disk
	SaveSettings() error

	// ReloadSettings reloads settings from disk
	ReloadSettings() error
}

// Ensure concrete types implement the interfaces
var _ CommandExecutor = (*Executor)(nil)
var _ PermissionChecker = (*PermissionManager)(nil)
