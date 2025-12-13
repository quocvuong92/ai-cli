package settings

import (
	"testing"
)

func TestPatternMatcher_Match(t *testing.T) {
	pm := NewPatternMatcher()

	tests := []struct {
		name     string
		command  string
		rule     PermissionRule
		expected bool
	}{
		// Exact match
		{"exact match", "ls -la", PermissionRule{Pattern: "ls -la", Tool: "Bash"}, true},
		{"exact match single word", "ls", PermissionRule{Pattern: "ls", Tool: "Bash"}, true},

		// Colon-style patterns (like GitHub Copilot CLI)
		{"git:* matches git status", "git status", PermissionRule{Pattern: "git:*", Tool: "Bash"}, true},
		{"git:* matches git commit", "git commit -m 'test'", PermissionRule{Pattern: "git:*", Tool: "Bash"}, true},
		{"git:* doesn't match gitignore", "gitignore", PermissionRule{Pattern: "git:*", Tool: "Bash"}, false},
		{"git:* matches just git", "git", PermissionRule{Pattern: "git:*", Tool: "Bash"}, true},
		{"npm:* matches npm install", "npm install express", PermissionRule{Pattern: "npm:*", Tool: "Bash"}, true},
		{"npm:run matches npm run test", "npm run test", PermissionRule{Pattern: "npm:run", Tool: "Bash"}, true},
		{"npm:run doesn't match npm install", "npm install", PermissionRule{Pattern: "npm:run", Tool: "Bash"}, false},

		// Glob patterns with *
		{"glob prefix", "npm run test", PermissionRule{Pattern: "npm run *", Tool: "Bash"}, true},
		{"glob prefix no match", "npm install", PermissionRule{Pattern: "npm run *", Tool: "Bash"}, false},
		{"glob suffix", "test.go", PermissionRule{Pattern: "*.go", Tool: "Bash"}, true},
		{"glob middle", "file-test-123", PermissionRule{Pattern: "file-*-123", Tool: "Bash"}, true},

		// Prefix match (implicit)
		{"prefix match", "ls -la --color", PermissionRule{Pattern: "ls", Tool: "Bash"}, true},
		{"prefix exact", "ls", PermissionRule{Pattern: "ls", Tool: "Bash"}, true},
		{"prefix no match different command", "lsof", PermissionRule{Pattern: "ls", Tool: "Bash"}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := pm.Match(tt.command, tt.rule)
			if result != tt.expected {
				t.Errorf("Match(%q, %+v) = %v, want %v", tt.command, tt.rule, result, tt.expected)
			}
		})
	}
}

func TestPatternMatcher_CheckPermission(t *testing.T) {
	pm := NewPatternMatcher()

	tests := []struct {
		name        string
		command     string
		permissions Permissions
		expected    MatchResult
	}{
		{
			name:    "deny takes precedence",
			command: "rm -rf /tmp/test",
			permissions: Permissions{
				Allow: []PermissionRule{{Pattern: "rm:*", Tool: "Bash"}},
				Deny:  []PermissionRule{{Pattern: "rm -rf *", Tool: "Bash"}},
			},
			expected: Deny,
		},
		{
			name:    "allow when no deny match",
			command: "git status",
			permissions: Permissions{
				Allow: []PermissionRule{{Pattern: "git:*", Tool: "Bash"}},
				Deny:  []PermissionRule{{Pattern: "rm:*", Tool: "Bash"}},
			},
			expected: Allow,
		},
		{
			name:    "no match",
			command: "docker ps",
			permissions: Permissions{
				Allow: []PermissionRule{{Pattern: "git:*", Tool: "Bash"}},
				Deny:  []PermissionRule{{Pattern: "rm:*", Tool: "Bash"}},
			},
			expected: NoMatch,
		},
		{
			name:        "empty rules",
			command:     "ls",
			permissions: Permissions{},
			expected:    NoMatch,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := pm.CheckPermission(tt.command, tt.permissions)
			if result != tt.expected {
				t.Errorf("CheckPermission(%q) = %v, want %v", tt.command, result, tt.expected)
			}
		})
	}
}

func TestParsePattern(t *testing.T) {
	tests := []struct {
		input    string
		expected PermissionRule
	}{
		{"git:*", PermissionRule{Pattern: "git:*", Tool: "Bash"}},
		{"Bash(git:*)", PermissionRule{Pattern: "git:*", Tool: "Bash"}},
		{"Read(*)", PermissionRule{Pattern: "*", Tool: "Read"}},
		{"Write(*.go)", PermissionRule{Pattern: "*.go", Tool: "Write"}},
		{"npm run *", PermissionRule{Pattern: "npm run *", Tool: "Bash"}},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := ParsePattern(tt.input)
			if result.Pattern != tt.expected.Pattern || result.Tool != tt.expected.Tool {
				t.Errorf("ParsePattern(%q) = %+v, want %+v", tt.input, result, tt.expected)
			}
		})
	}
}

func TestFormatPattern(t *testing.T) {
	tests := []struct {
		input    PermissionRule
		expected string
	}{
		{PermissionRule{Pattern: "git:*", Tool: "Bash"}, "git:*"},
		{PermissionRule{Pattern: "git:*", Tool: ""}, "git:*"},
		{PermissionRule{Pattern: "*", Tool: "Read"}, "Read(*)"},
		{PermissionRule{Pattern: "*.go", Tool: "Write"}, "Write(*.go)"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			result := FormatPattern(tt.input)
			if result != tt.expected {
				t.Errorf("FormatPattern(%+v) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}
