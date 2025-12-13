package settings

import (
	"regexp"
	"strings"
)

// MatchResult represents the result of permission matching
type MatchResult int

const (
	// NoMatch indicates no matching rule was found
	NoMatch MatchResult = iota
	// Allow indicates the command matches an allow rule
	Allow
	// Deny indicates the command matches a deny rule
	Deny
)

// PatternMatcher handles glob/wildcard pattern matching for permissions
type PatternMatcher struct{}

// NewPatternMatcher creates a new pattern matcher
func NewPatternMatcher() *PatternMatcher {
	return &PatternMatcher{}
}

// Match checks if a command matches a permission pattern
// Patterns support:
//   - Exact match: "ls -la"
//   - Glob wildcard: "git:*" matches "git status", "git commit", etc.
//   - Prefix match: "npm run *" matches "npm run test", "npm run build"
//   - Tool-specific: "Bash(git:*)" for tool-qualified patterns
func (pm *PatternMatcher) Match(command string, rule PermissionRule) bool {
	pattern := rule.Pattern
	command = strings.TrimSpace(command)

	// Handle tool-qualified patterns like "Bash(git:*)"
	if rule.Tool != "" && strings.Contains(pattern, "(") {
		// Extract pattern from tool qualification
		pattern = pm.extractPatternFromTool(pattern)
	}

	// Normalize pattern
	pattern = strings.TrimSpace(pattern)

	// Exact match
	if pattern == command {
		return true
	}

	// Handle colon-style patterns: "git:*" means "git " + anything
	if strings.Contains(pattern, ":") {
		return pm.matchColonPattern(command, pattern)
	}

	// Handle glob-style patterns with *
	if strings.Contains(pattern, "*") {
		return pm.matchGlobPattern(command, pattern)
	}

	// Prefix match (pattern without wildcard matches command prefix)
	if strings.HasPrefix(command, pattern+" ") || command == pattern {
		return true
	}

	return false
}

// extractPatternFromTool extracts the inner pattern from "Tool(pattern)"
func (pm *PatternMatcher) extractPatternFromTool(pattern string) string {
	// Handle "Bash(git:*)" -> "git:*"
	start := strings.Index(pattern, "(")
	end := strings.LastIndex(pattern, ")")
	if start != -1 && end > start {
		return pattern[start+1 : end]
	}
	return pattern
}

// matchColonPattern matches patterns like "git:*", "npm:*"
// "git:*" matches any command starting with "git " (note the space)
// "git:status" matches exactly "git status"
func (pm *PatternMatcher) matchColonPattern(command, pattern string) bool {
	parts := strings.SplitN(pattern, ":", 2)
	if len(parts) != 2 {
		return false
	}

	prefix := parts[0]
	suffix := parts[1]

	// Command must start with the prefix followed by a space (or be exactly the prefix)
	if !strings.HasPrefix(command, prefix+" ") && command != prefix {
		return false
	}

	// Get the rest of the command after prefix and space
	rest := strings.TrimPrefix(command, prefix)
	rest = strings.TrimPrefix(rest, " ")

	// Wildcard: match anything (including empty - just the command itself)
	if suffix == "*" {
		return true
	}

	// Specific subcommand
	if strings.Contains(suffix, "*") {
		return pm.matchGlobPattern(rest, suffix)
	}

	// Exact subcommand match
	cmdParts := strings.Fields(rest)
	if len(cmdParts) > 0 && cmdParts[0] == suffix {
		return true
	}

	return rest == suffix
}

// matchGlobPattern matches patterns with * wildcards
// Converts glob pattern to regex for matching
func (pm *PatternMatcher) matchGlobPattern(command, pattern string) bool {
	// Escape regex special characters except *
	regexPattern := regexp.QuoteMeta(pattern)

	// Replace escaped \* with .*
	regexPattern = strings.ReplaceAll(regexPattern, `\*`, `.*`)

	// Anchor the pattern
	regexPattern = "^" + regexPattern + "$"

	re, err := regexp.Compile(regexPattern)
	if err != nil {
		return false
	}

	return re.MatchString(command)
}

// MatchRules checks a command against a list of permission rules
// Returns the first matching result (Deny rules should be checked first)
func (pm *PatternMatcher) MatchRules(command string, rules []PermissionRule) bool {
	for _, rule := range rules {
		if pm.Match(command, rule) {
			return true
		}
	}
	return false
}

// CheckPermission checks a command against allow and deny rules
// Deny rules take precedence over allow rules
func (pm *PatternMatcher) CheckPermission(command string, permissions Permissions) MatchResult {
	// Check deny rules first (they take precedence)
	if pm.MatchRules(command, permissions.Deny) {
		return Deny
	}

	// Check allow rules
	if pm.MatchRules(command, permissions.Allow) {
		return Allow
	}

	return NoMatch
}

// ParsePattern parses a pattern string into a PermissionRule
// Supports formats:
//   - "git:*" -> PermissionRule{Pattern: "git:*", Tool: "Bash"}
//   - "Bash(git:*)" -> PermissionRule{Pattern: "git:*", Tool: "Bash"}
//   - "Read(*)" -> PermissionRule{Pattern: "*", Tool: "Read"}
func ParsePattern(pattern string) PermissionRule {
	pattern = strings.TrimSpace(pattern)

	// Check for tool-qualified pattern
	if idx := strings.Index(pattern, "("); idx != -1 {
		tool := pattern[:idx]
		inner := strings.TrimSuffix(strings.TrimPrefix(pattern[idx:], "("), ")")
		return PermissionRule{
			Pattern: inner,
			Tool:    tool,
		}
	}

	// Default to Bash tool for command patterns
	return PermissionRule{
		Pattern: pattern,
		Tool:    "Bash",
	}
}

// FormatPattern formats a PermissionRule back to string
func FormatPattern(rule PermissionRule) string {
	if rule.Tool == "" || rule.Tool == "Bash" {
		return rule.Pattern
	}
	return rule.Tool + "(" + rule.Pattern + ")"
}
