package settings

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDefaultSettings(t *testing.T) {
	s := DefaultSettings()

	if !s.AutoAllowSafeCommands {
		t.Error("AutoAllowSafeCommands should be true by default")
	}
	if s.DangerousEnabled {
		t.Error("DangerousEnabled should be false by default")
	}
	if len(s.Permissions.Allow) != 0 {
		t.Error("Allow rules should be empty by default")
	}
	if len(s.Permissions.Deny) != 0 {
		t.Error("Deny rules should be empty by default")
	}
	if s.sessionAllowlist == nil {
		t.Error("sessionAllowlist should be initialized")
	}
}

func TestManager_LoadAndSave(t *testing.T) {
	// Create a temporary directory for testing
	tmpDir, err := os.MkdirTemp("", "ai-cli-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Override XDG_DATA_HOME for testing
	oldDataHome := os.Getenv("XDG_DATA_HOME")
	os.Setenv("XDG_DATA_HOME", tmpDir)
	defer os.Setenv("XDG_DATA_HOME", oldDataHome)

	m := NewManager()

	// Load should work even without existing file
	if err := m.Load(); err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	// Add some rules
	m.AddGlobalAllowRule(PermissionRule{Pattern: "git:*", Tool: "Bash"})
	m.AddGlobalDenyRule(PermissionRule{Pattern: "rm -rf *", Tool: "Bash"})

	// Save
	if err := m.Save(); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	// Verify file exists
	settingsPath := filepath.Join(tmpDir, AppName, SettingsFile)
	if _, err := os.Stat(settingsPath); os.IsNotExist(err) {
		t.Fatalf("Settings file not created at %s", settingsPath)
	}

	// Create new manager and load
	m2 := NewManager()
	if err := m2.Load(); err != nil {
		t.Fatalf("Load failed on second manager: %v", err)
	}

	// Verify rules were loaded
	merged := m2.GetMerged()
	if len(merged.Permissions.Allow) != 1 {
		t.Errorf("Expected 1 allow rule, got %d", len(merged.Permissions.Allow))
	}
	if len(merged.Permissions.Deny) != 1 {
		t.Errorf("Expected 1 deny rule, got %d", len(merged.Permissions.Deny))
	}
}

func TestManager_SessionAllowlist(t *testing.T) {
	m := NewManager()
	_ = m.Load()

	// Initially not allowed
	if m.IsSessionAllowed("test-command") {
		t.Error("Command should not be in session allowlist initially")
	}

	// Add to session
	m.AddSessionAllowRule("test-command")

	// Now should be allowed
	if !m.IsSessionAllowed("test-command") {
		t.Error("Command should be in session allowlist after adding")
	}

	// Clear session
	m.ClearSessionAllowlist()

	// Should no longer be allowed
	if m.IsSessionAllowed("test-command") {
		t.Error("Command should not be in session allowlist after clearing")
	}
}

func TestManager_MergeSettings(t *testing.T) {
	// Create a temporary directory for testing
	tmpDir, err := os.MkdirTemp("", "ai-cli-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Override XDG_DATA_HOME for testing
	oldDataHome := os.Getenv("XDG_DATA_HOME")
	os.Setenv("XDG_DATA_HOME", tmpDir)
	defer os.Setenv("XDG_DATA_HOME", oldDataHome)

	m := NewManager()
	_ = m.Load()

	// Add global rules
	m.AddGlobalAllowRule(PermissionRule{Pattern: "git:*", Tool: "Bash"})
	m.AddGlobalAllowRule(PermissionRule{Pattern: "npm:*", Tool: "Bash"})
	m.AddGlobalDenyRule(PermissionRule{Pattern: "rm -rf *", Tool: "Bash"})

	merged := m.GetMerged()

	// Should have all rules
	if len(merged.Permissions.Allow) != 2 {
		t.Errorf("Expected 2 allow rules, got %d", len(merged.Permissions.Allow))
	}
	if len(merged.Permissions.Deny) != 1 {
		t.Errorf("Expected 1 deny rule, got %d", len(merged.Permissions.Deny))
	}
}

func TestManager_SetBehaviorSettings(t *testing.T) {
	m := NewManager()
	_ = m.Load()

	// Test auto-allow safe
	m.SetAutoAllowSafe(false)
	if m.GetMerged().AutoAllowSafeCommands {
		t.Error("AutoAllowSafeCommands should be false")
	}

	m.SetAutoAllowSafe(true)
	if !m.GetMerged().AutoAllowSafeCommands {
		t.Error("AutoAllowSafeCommands should be true")
	}

	// Test dangerous enabled
	m.SetDangerousEnabled(true)
	if !m.GetMerged().DangerousEnabled {
		t.Error("DangerousEnabled should be true")
	}

	m.SetDangerousEnabled(false)
	if m.GetMerged().DangerousEnabled {
		t.Error("DangerousEnabled should be false")
	}
}
