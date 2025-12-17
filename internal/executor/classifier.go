package executor

import (
	"regexp"
	"strings"
)

// RiskLevel represents the risk level of a command
type RiskLevel int

const (
	// Safe commands are read-only and auto-approved
	Safe RiskLevel = iota
	// NeedsConfirm commands modify state and require user confirmation
	NeedsConfirm
	// Dangerous commands are potentially destructive and blocked by default
	Dangerous
)

// Safe read-only commands that can be auto-executed
// Note: curl and wget are intentionally NOT included here as they can
// exfiltrate data or download malicious content
var safeCommands = []string{
	"ls", "cat", "pwd", "echo", "head", "tail", "grep", "find",
	"which", "whoami", "date", "wc", "sort", "uniq", "diff",
	"env", "printenv", "df", "du", "ps", "top", "tree",
	"file", "stat", "basename", "dirname", "realpath",
	"ping", "traceroute", "nslookup", "dig",
}

// Safe command patterns (regex) for read-only operations
var safePatterns = []*regexp.Regexp{
	regexp.MustCompile(`^git\s+(status|log|diff|branch|show|remote)`),
	regexp.MustCompile(`^npm\s+(list|ls|view|info|outdated)`),
	regexp.MustCompile(`^pip\s+(list|show|freeze)`),
	regexp.MustCompile(`^cargo\s+(tree|search|check)`),
	regexp.MustCompile(`^go\s+(list|version|env)`),
	regexp.MustCompile(`^docker\s+(ps|images|inspect|logs)`),
	regexp.MustCompile(`^kubectl\s+(get|describe|logs)`),
}

// Dangerous command patterns that are blocked by default
var dangerousPatterns = []*regexp.Regexp{
	regexp.MustCompile(`rm\s+(-[rf]*\s+)?/`),       // rm -rf / or variations
	regexp.MustCompile(`\bsudo\b`),                 // Any sudo command
	regexp.MustCompile(`\bsu\b`),                   // Switch user command
	regexp.MustCompile(`dd\s+if=`),                 // dd commands
	regexp.MustCompile(`mkfs`),                     // Format filesystem
	regexp.MustCompile(`:\(\)\{`),                  // Fork bomb
	regexp.MustCompile(`curl.*\|\s*(sh|bash|zsh)`), // Pipe to shell
	regexp.MustCompile(`wget.*\|\s*(sh|bash|zsh)`), // Pipe to shell
	regexp.MustCompile(`>\s*/dev/sd`),              // Write to disk device
	regexp.MustCompile(`chmod.*777`),               // Overly permissive chmod
	regexp.MustCompile(`chown.*-R\s+`),             // Recursive ownership change
	regexp.MustCompile(`\beval\b`),                 // Eval command (can execute arbitrary code)
	regexp.MustCompile(`\bsource\b`),               // Source command (executes file content)
	regexp.MustCompile(`\bexec\b`),                 // Exec command (replaces shell)
	regexp.MustCompile(`>\s*/etc/`),                // Write to /etc directory
	regexp.MustCompile(`rm\s+-rf\s+[~$]`),          // rm -rf with home or variable
	regexp.MustCompile(`>\s*/dev/null\s*2>&1\s*&`), // Background with hidden output (suspicious)
	regexp.MustCompile(`\|.*base64.*-d`),           // Decode base64 in pipeline (often malicious)
	regexp.MustCompile(`python.*-c.*exec`),         // Python exec in command line
	regexp.MustCompile(`perl.*-e`),                 // Perl one-liner execution
	regexp.MustCompile(`ruby.*-e`),                 // Ruby one-liner execution
}

// commandChainingPattern detects command chaining operators that could bypass security
var commandChainingPattern = regexp.MustCompile(`[;&|]{1,2}`)

// ClassifyCommand determines the risk level of a shell command
func ClassifyCommand(cmd string) RiskLevel {
	cmd = strings.TrimSpace(cmd)

	if cmd == "" {
		return Dangerous
	}

	// Check dangerous patterns first (highest priority)
	for _, pattern := range dangerousPatterns {
		if pattern.MatchString(cmd) {
			return Dangerous
		}
	}

	// Check for command chaining - if present, require confirmation
	// as chained commands could include dangerous operations
	if commandChainingPattern.MatchString(cmd) {
		return NeedsConfirm
	}

	// Extract first word (command name)
	fields := strings.Fields(cmd)
	if len(fields) == 0 {
		return Dangerous
	}
	firstWord := fields[0]

	// Check if it's a known safe command
	for _, safe := range safeCommands {
		if firstWord == safe {
			return Safe
		}
	}

	// Check safe patterns
	for _, pattern := range safePatterns {
		if pattern.MatchString(cmd) {
			return Safe
		}
	}

	// Default: needs confirmation for anything that modifies state
	return NeedsConfirm
}

// GetRiskDescription returns a human-readable description of the risk level
func GetRiskDescription(level RiskLevel) string {
	switch level {
	case Safe:
		return "Safe read-only command"
	case NeedsConfirm:
		return "Command may modify system state"
	case Dangerous:
		return "Potentially dangerous command"
	default:
		return "Unknown risk level"
	}
}
