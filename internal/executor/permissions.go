package executor

import (
	"sync"

	"github.com/quocvuong92/ai-cli/internal/settings"
)

// ApprovalType represents the type of approval for a command
type ApprovalType int

const (
	// ApprovalOnce allows the command for this execution only
	ApprovalOnce ApprovalType = iota
	// ApprovalSession allows the command for the current session
	ApprovalSession
	// ApprovalAlways saves the command to persistent settings
	ApprovalAlways
)

// PermissionManager handles command execution permissions
type PermissionManager struct {
	mu             sync.RWMutex
	settings       *settings.Manager
	matcher        *settings.PatternMatcher
	sessionAllowed map[string]bool // Commands allowed for this session only
}

// NewPermissionManager creates a new permission manager with safe defaults
func NewPermissionManager() *PermissionManager {
	pm := &PermissionManager{
		settings:       settings.NewManager(),
		matcher:        settings.NewPatternMatcher(),
		sessionAllowed: make(map[string]bool),
	}

	// Load settings from disk - log error but continue with defaults
	if err := pm.settings.Load(); err != nil {
		// Settings file may not exist yet, which is fine
		// Only log if it's a different error (file exists but can't be parsed)
		// For now we silently continue with defaults
	}

	return pm
}

// CheckPermission checks if a command is allowed to execute
// Returns: (allowed, needsConfirm, reason)
func (pm *PermissionManager) CheckPermission(cmd string) (allowed bool, needsConfirm bool, reason string) {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	merged := pm.settings.GetMerged()
	if merged == nil {
		return false, true, "Settings not loaded"
	}

	// Check session allowlist first
	if pm.sessionAllowed[cmd] {
		return true, false, "Allowed for this session"
	}

	// Check persistent settings allowlist
	if pm.settings.IsSessionAllowed(cmd) {
		return true, false, "Previously approved"
	}

	// Check deny rules first (they take precedence)
	result := pm.matcher.CheckPermission(cmd, merged.Permissions)
	if result == settings.Deny {
		return false, false, "Command blocked by deny rule"
	}

	// Check allow rules
	if result == settings.Allow {
		return true, false, "Allowed by permission rule"
	}

	// Classify command by risk level
	risk := ClassifyCommand(cmd)

	switch risk {
	case Safe:
		if merged.AutoAllowSafeCommands {
			return true, false, "Safe read-only command"
		}
		return false, true, "Confirmation required"

	case NeedsConfirm:
		return false, true, "Command may modify system state"

	case Dangerous:
		if merged.DangerousEnabled {
			return false, true, "Dangerous command (requires explicit confirmation)"
		}
		return false, false, "Dangerous command blocked (use /allow-dangerous to enable)"
	}

	return false, true, "Unknown command type"
}

// AddToAllowlist adds a command based on approval type
func (pm *PermissionManager) AddToAllowlist(cmd string, approvalType ApprovalType) error {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	switch approvalType {
	case ApprovalOnce:
		// Do nothing, command will be executed this time only
		return nil

	case ApprovalSession:
		// Add to session-only allowlist
		pm.sessionAllowed[cmd] = true
		return nil

	case ApprovalAlways:
		// Add to persistent settings
		pm.settings.AddGlobalAllowRule(settings.PermissionRule{
			Pattern: cmd,
			Tool:    "Bash",
		})
		return pm.settings.Save()
	}

	return nil
}

// AddPatternRule adds a pattern-based rule to persistent settings
func (pm *PermissionManager) AddPatternRule(pattern string, deny bool) error {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	rule := settings.ParsePattern(pattern)

	if deny {
		pm.settings.AddGlobalDenyRule(rule)
	} else {
		pm.settings.AddGlobalAllowRule(rule)
	}

	return pm.settings.Save()
}

// EnableDangerous enables execution of dangerous commands (with confirmation)
func (pm *PermissionManager) EnableDangerous() {
	pm.mu.Lock()
	defer pm.mu.Unlock()
	pm.settings.SetDangerousEnabled(true)
}

// DisableDangerous disables execution of dangerous commands
func (pm *PermissionManager) DisableDangerous() {
	pm.mu.Lock()
	defer pm.mu.Unlock()
	pm.settings.SetDangerousEnabled(false)
}

// SetAutoAllowReads sets whether to auto-allow safe read-only commands
func (pm *PermissionManager) SetAutoAllowReads(enabled bool) {
	pm.mu.Lock()
	defer pm.mu.Unlock()
	pm.settings.SetAutoAllowSafe(enabled)
}

// GetSettings returns current permission settings for display
func (pm *PermissionManager) GetSettings() map[string]interface{} {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	merged := pm.settings.GetMerged()
	global := pm.settings.GetGlobal()

	result := map[string]interface{}{
		"auto_allow_safe":   merged.AutoAllowSafeCommands,
		"dangerous_enabled": merged.DangerousEnabled,
		"session_count":     len(pm.sessionAllowed),
		"global_path":       pm.settings.GetGlobalPath(),
		"project_path":      pm.settings.GetProjectPath(),
	}

	if global != nil {
		result["allow_rules"] = len(global.Permissions.Allow)
		result["deny_rules"] = len(global.Permissions.Deny)
	}

	return result
}

// GetAllowRules returns the current allow rules
func (pm *PermissionManager) GetAllowRules() []settings.PermissionRule {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	merged := pm.settings.GetMerged()
	if merged == nil {
		return nil
	}
	return merged.Permissions.Allow
}

// GetDenyRules returns the current deny rules
func (pm *PermissionManager) GetDenyRules() []settings.PermissionRule {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	merged := pm.settings.GetMerged()
	if merged == nil {
		return nil
	}
	return merged.Permissions.Deny
}

// ClearSessionAllowlist clears all session-only permissions
func (pm *PermissionManager) ClearSessionAllowlist() {
	pm.mu.Lock()
	defer pm.mu.Unlock()
	pm.sessionAllowed = make(map[string]bool)
	pm.settings.ClearSessionAllowlist()
}

// SaveSettings saves current settings to disk
func (pm *PermissionManager) SaveSettings() error {
	pm.mu.RLock()
	defer pm.mu.RUnlock()
	return pm.settings.Save()
}

// ReloadSettings reloads settings from disk
func (pm *PermissionManager) ReloadSettings() error {
	pm.mu.Lock()
	defer pm.mu.Unlock()
	return pm.settings.Load()
}
