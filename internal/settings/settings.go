package settings

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
)

const (
	// AppName is the application name used for settings directories
	AppName = "ai-cli"

	// SettingsFile is the name of the settings file
	SettingsFile = "settings.json"

	// ProjectSettingsDir is the directory name for project-level settings
	ProjectSettingsDir = ".ai-cli"
)

// PermissionRule represents a single permission rule with glob pattern
type PermissionRule struct {
	Pattern string `json:"pattern"` // Glob pattern like "git:*", "npm:*", "rm -rf *"
	Tool    string `json:"tool"`    // Tool name: "Bash", "Read", "Write", etc.
}

// Permissions holds allow and deny rules
type Permissions struct {
	Allow []PermissionRule `json:"allow"` // Patterns to always allow
	Deny  []PermissionRule `json:"deny"`  // Patterns to always deny (takes precedence)
}

// Settings represents the application settings
type Settings struct {
	// Permission settings
	Permissions Permissions `json:"permissions"`

	// Behavior settings
	AutoAllowSafeCommands bool `json:"auto_allow_safe_commands"` // Auto-allow safe read-only commands
	DangerousEnabled      bool `json:"dangerous_enabled"`        // Allow dangerous commands with confirmation

	// Session-only allowlist (not persisted)
	sessionAllowlist map[string]bool `json:"-"`
}

// Manager handles loading, saving, and merging settings from multiple sources
type Manager struct {
	mu          sync.RWMutex
	global      *Settings // ~/.local/share/ai-cli/settings.json
	project     *Settings // ./.ai-cli/settings.json
	merged      *Settings // Merged effective settings
	globalPath  string
	projectPath string
}

// NewManager creates a new settings manager
func NewManager() *Manager {
	return &Manager{
		global:  DefaultSettings(),
		project: nil,
		merged:  DefaultSettings(),
	}
}

// DefaultSettings returns settings with safe defaults
func DefaultSettings() *Settings {
	return &Settings{
		Permissions: Permissions{
			Allow: []PermissionRule{},
			Deny:  []PermissionRule{},
		},
		AutoAllowSafeCommands: true,
		DangerousEnabled:      false,
		sessionAllowlist:      make(map[string]bool),
	}
}

// Load loads settings from all sources and merges them
func (m *Manager) Load() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Load global settings
	globalPath, err := m.getGlobalSettingsPath()
	if err != nil {
		return err
	}
	m.globalPath = globalPath

	global, err := m.loadFromFile(globalPath)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	if global != nil {
		m.global = global
	} else {
		m.global = DefaultSettings()
	}

	// Load project settings
	projectPath, err := m.getProjectSettingsPath()
	if err == nil {
		m.projectPath = projectPath
		project, err := m.loadFromFile(projectPath)
		if err != nil && !os.IsNotExist(err) {
			// Log but don't fail on project settings error
			m.project = nil
		} else {
			m.project = project
		}
	}

	// Merge settings (project overrides global)
	m.merged = m.mergeSettings()

	return nil
}

// getGlobalSettingsPath returns the path to global settings file
func (m *Manager) getGlobalSettingsPath() (string, error) {
	// Use XDG_DATA_HOME or default to ~/.local/share
	dataHome := os.Getenv("XDG_DATA_HOME")
	if dataHome == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		dataHome = filepath.Join(home, ".local", "share")
	}

	return filepath.Join(dataHome, AppName, SettingsFile), nil
}

// getProjectSettingsPath returns the path to project settings file
func (m *Manager) getProjectSettingsPath() (string, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return "", err
	}

	return filepath.Join(cwd, ProjectSettingsDir, SettingsFile), nil
}

// loadFromFile loads settings from a JSON file
func (m *Manager) loadFromFile(path string) (*Settings, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var settings Settings
	if err := json.Unmarshal(data, &settings); err != nil {
		return nil, err
	}

	// Initialize session allowlist
	settings.sessionAllowlist = make(map[string]bool)

	return &settings, nil
}

// mergeSettings merges global and project settings
// Project settings take precedence, deny rules always take precedence over allow
func (m *Manager) mergeSettings() *Settings {
	merged := DefaultSettings()

	// Start with global settings
	if m.global != nil {
		merged.Permissions.Allow = append(merged.Permissions.Allow, m.global.Permissions.Allow...)
		merged.Permissions.Deny = append(merged.Permissions.Deny, m.global.Permissions.Deny...)
		merged.AutoAllowSafeCommands = m.global.AutoAllowSafeCommands
		merged.DangerousEnabled = m.global.DangerousEnabled
	}

	// Overlay project settings
	if m.project != nil {
		merged.Permissions.Allow = append(merged.Permissions.Allow, m.project.Permissions.Allow...)
		merged.Permissions.Deny = append(merged.Permissions.Deny, m.project.Permissions.Deny...)
		// Project can override behavior settings
		merged.AutoAllowSafeCommands = m.project.AutoAllowSafeCommands
		merged.DangerousEnabled = m.project.DangerousEnabled
	}

	return merged
}

// Save saves settings to the global settings file
func (m *Manager) Save() error {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if m.globalPath == "" {
		path, err := m.getGlobalSettingsPath()
		if err != nil {
			return err
		}
		m.globalPath = path
	}

	return m.saveToFile(m.globalPath, m.global)
}

// SaveProject saves settings to the project settings file
func (m *Manager) SaveProject() error {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if m.projectPath == "" {
		path, err := m.getProjectSettingsPath()
		if err != nil {
			return err
		}
		m.projectPath = path
	}

	return m.saveToFile(m.projectPath, m.project)
}

// saveToFile saves settings to a JSON file
func (m *Manager) saveToFile(path string, settings *Settings) error {
	// Ensure directory exists
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	data, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(path, data, 0644)
}

// GetMerged returns the merged effective settings (read-only)
func (m *Manager) GetMerged() *Settings {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.merged
}

// GetGlobal returns the global settings
func (m *Manager) GetGlobal() *Settings {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.global
}

// GetProject returns the project settings
func (m *Manager) GetProject() *Settings {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.project
}

// AddGlobalAllowRule adds an allow rule to global settings
func (m *Manager) AddGlobalAllowRule(rule PermissionRule) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.global == nil {
		m.global = DefaultSettings()
	}
	m.global.Permissions.Allow = append(m.global.Permissions.Allow, rule)
	m.merged = m.mergeSettings()
}

// AddGlobalDenyRule adds a deny rule to global settings
func (m *Manager) AddGlobalDenyRule(rule PermissionRule) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.global == nil {
		m.global = DefaultSettings()
	}
	m.global.Permissions.Deny = append(m.global.Permissions.Deny, rule)
	m.merged = m.mergeSettings()
}

// AddSessionAllowRule adds a command to the session allowlist (not persisted)
func (m *Manager) AddSessionAllowRule(command string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.merged.sessionAllowlist == nil {
		m.merged.sessionAllowlist = make(map[string]bool)
	}
	m.merged.sessionAllowlist[command] = true
}

// IsSessionAllowed checks if a command is in the session allowlist
func (m *Manager) IsSessionAllowed(command string) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if m.merged.sessionAllowlist == nil {
		return false
	}
	return m.merged.sessionAllowlist[command]
}

// SetAutoAllowSafe sets whether to auto-allow safe commands
func (m *Manager) SetAutoAllowSafe(enabled bool) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.global.AutoAllowSafeCommands = enabled
	m.merged.AutoAllowSafeCommands = enabled
}

// SetDangerousEnabled sets whether dangerous commands are allowed
func (m *Manager) SetDangerousEnabled(enabled bool) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.global.DangerousEnabled = enabled
	m.merged.DangerousEnabled = enabled
}

// GetGlobalPath returns the path to the global settings file
func (m *Manager) GetGlobalPath() string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.globalPath
}

// GetProjectPath returns the path to the project settings file
func (m *Manager) GetProjectPath() string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.projectPath
}

// ClearSessionAllowlist clears all session-only permissions
func (m *Manager) ClearSessionAllowlist() {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.merged.sessionAllowlist = make(map[string]bool)
}
