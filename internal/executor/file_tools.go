// Package executor provides command and file operation execution with safety checks.
package executor

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// MaxFileSize is the maximum file size for read operations (512KB)
const MaxFileSize = 512 * 1024

// MaxSearchResults limits search output to prevent flooding
const MaxSearchResults = 50

// FileOperationTimeout is the timeout for file operations that run external commands
const FileOperationTimeout = 30 * time.Second

// blockedPaths are system directories that cannot be modified
var blockedPaths = []string{
	"/etc/", "/usr/", "/bin/", "/sbin/", "/boot/",
	"/sys/", "/proc/", "/dev/", "/var/", "/lib/",
	"/System/", "/Library/", // macOS system paths
}

// FileToolResult provides consistent return values for file operations
type FileToolResult struct {
	Success   bool
	Output    string
	Truncated bool
}

// IsPathSafe checks if a path is safe for write/delete operations.
// Returns (safe, reason) where reason explains why the path is blocked.
func IsPathSafe(path string) (bool, string) {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return false, "invalid path"
	}

	// Resolve the entire path (including symlinks) to get the real path
	// This handles macOS where /etc -> /private/etc, /var -> /private/var
	if resolved, err := filepath.EvalSymlinks(absPath); err == nil {
		absPath = resolved
	} else {
		// If the file doesn't exist yet, resolve the parent directory
		dir := filepath.Dir(absPath)
		if resolvedDir, err := filepath.EvalSymlinks(dir); err == nil {
			absPath = filepath.Join(resolvedDir, filepath.Base(absPath))
		}
	}

	// Check against blocked paths (both original and resolved)
	for _, blocked := range blockedPaths {
		if strings.HasPrefix(absPath, blocked) {
			return false, fmt.Sprintf("path %s is protected", blocked)
		}
		// Also check with /private prefix for macOS
		if strings.HasPrefix(absPath, "/private"+blocked) {
			return false, fmt.Sprintf("path %s is protected", blocked)
		}
	}

	return true, ""
}

// ReadFile reads file contents with a size limit.
// Files larger than MaxFileSize are truncated with a warning.
func ReadFile(path string) FileToolResult {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return FileToolResult{Output: fmt.Sprintf("Error: invalid path: %v", err)}
	}

	info, err := os.Stat(absPath)
	if err != nil {
		if os.IsNotExist(err) {
			return FileToolResult{Output: fmt.Sprintf("Error: file not found: %s", path)}
		}
		return FileToolResult{Output: fmt.Sprintf("Error: %v", err)}
	}

	if info.IsDir() {
		return FileToolResult{Output: fmt.Sprintf("Error: %s is a directory, use list_directory instead", path)}
	}

	data, err := os.ReadFile(absPath)
	if err != nil {
		return FileToolResult{Output: fmt.Sprintf("Error: %v", err)}
	}

	truncated := false
	if info.Size() > MaxFileSize {
		data = data[:MaxFileSize]
		truncated = true
	}

	output := string(data)
	if truncated {
		output += fmt.Sprintf("\n\n[Truncated: file is %d bytes, showing first 512KB]", info.Size())
	}

	return FileToolResult{Success: true, Output: output, Truncated: truncated}
}

// WriteFile creates or overwrites a file with the given content.
// Creates parent directories if they don't exist.
func WriteFile(path, content string) FileToolResult {
	if safe, reason := IsPathSafe(path); !safe {
		return FileToolResult{Output: fmt.Sprintf("Blocked: %s", reason)}
	}

	absPath, err := filepath.Abs(path)
	if err != nil {
		return FileToolResult{Output: fmt.Sprintf("Error: invalid path: %v", err)}
	}

	// Create parent directories if needed
	dir := filepath.Dir(absPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return FileToolResult{Output: fmt.Sprintf("Error creating directory: %v", err)}
	}

	if err := os.WriteFile(absPath, []byte(content), 0644); err != nil {
		return FileToolResult{Output: fmt.Sprintf("Error: %v", err)}
	}

	return FileToolResult{
		Success: true,
		Output:  fmt.Sprintf("Wrote %d bytes to %s", len(content), path),
	}
}

// EditFile performs a search and replace operation on a file.
// Returns the result and a diff string for preview.
func EditFile(path, oldText, newText string) (FileToolResult, string) {
	if safe, reason := IsPathSafe(path); !safe {
		return FileToolResult{Output: fmt.Sprintf("Blocked: %s", reason)}, ""
	}

	absPath, err := filepath.Abs(path)
	if err != nil {
		return FileToolResult{Output: fmt.Sprintf("Error: invalid path: %v", err)}, ""
	}

	data, err := os.ReadFile(absPath)
	if err != nil {
		if os.IsNotExist(err) {
			return FileToolResult{Output: fmt.Sprintf("Error: file not found: %s", path)}, ""
		}
		return FileToolResult{Output: fmt.Sprintf("Error: %v", err)}, ""
	}

	content := string(data)
	if !strings.Contains(content, oldText) {
		return FileToolResult{Output: "Error: text not found in file"}, ""
	}

	// Count occurrences
	count := strings.Count(content, oldText)

	// Generate diff before making changes
	diff := GenerateDiff(oldText, newText)

	// Perform replacement (first occurrence only for safety)
	newContent := strings.Replace(content, oldText, newText, 1)

	if err := os.WriteFile(absPath, []byte(newContent), 0644); err != nil {
		return FileToolResult{Output: fmt.Sprintf("Error: %v", err)}, diff
	}

	msg := fmt.Sprintf("Edited %s", path)
	if count > 1 {
		msg += fmt.Sprintf(" (replaced 1 of %d occurrences)", count)
	}

	return FileToolResult{Success: true, Output: msg}, diff
}

// SearchFiles searches for a pattern in files using ripgrep or grep.
// Uses a timeout to prevent hanging on large codebases.
func SearchFiles(pattern, path, fileType string) FileToolResult {
	if path == "" {
		path = "."
	}

	ctx, cancel := context.WithTimeout(context.Background(), FileOperationTimeout)
	defer cancel()

	// Try ripgrep first (faster and more features)
	result := searchWithRipgrep(ctx, pattern, path, fileType)
	if result.Success || result.Output != "" {
		return result
	}

	// Fall back to grep
	return searchWithGrep(ctx, pattern, path)
}

func searchWithRipgrep(ctx context.Context, pattern, path, fileType string) FileToolResult {
	args := []string{
		"-n",                                      // line numbers
		"--color=never",                           // no color codes
		"-m", fmt.Sprintf("%d", MaxSearchResults), // limit matches
	}

	if fileType != "" {
		args = append(args, "-t", fileType)
	}
	args = append(args, pattern, path)

	cmd := exec.CommandContext(ctx, "rg", args...)
	output, err := cmd.CombinedOutput()

	// Check for timeout
	if ctx.Err() == context.DeadlineExceeded {
		return FileToolResult{Output: "Error: search timed out (30s limit)"}
	}

	// ripgrep returns exit code 1 for no matches, which is not an error
	result := strings.TrimSpace(string(output))
	if err != nil && len(result) == 0 {
		// Check if ripgrep exists
		if _, pathErr := exec.LookPath("rg"); pathErr != nil {
			return FileToolResult{} // Signal to try grep
		}
	}

	if result == "" {
		return FileToolResult{Success: true, Output: "No matches found"}
	}

	return FileToolResult{Success: true, Output: result}
}

func searchWithGrep(ctx context.Context, pattern, path string) FileToolResult {
	args := []string{"-rn", pattern, path}
	cmd := exec.CommandContext(ctx, "grep", args...)
	output, _ := cmd.CombinedOutput()

	// Check for timeout
	if ctx.Err() == context.DeadlineExceeded {
		return FileToolResult{Output: "Error: search timed out (30s limit)"}
	}

	result := strings.TrimSpace(string(output))
	if result == "" {
		return FileToolResult{Success: true, Output: "No matches found"}
	}

	// Limit output lines
	lines := strings.Split(result, "\n")
	if len(lines) > MaxSearchResults {
		lines = lines[:MaxSearchResults]
		result = strings.Join(lines, "\n") + fmt.Sprintf("\n[Truncated: showing first %d matches]", MaxSearchResults)
	}

	return FileToolResult{Success: true, Output: result}
}

// ListDirectory lists the contents of a directory.
// Uses a timeout to prevent hanging on very large directories.
func ListDirectory(path string, recursive bool) FileToolResult {
	if path == "" {
		path = "."
	}

	ctx, cancel := context.WithTimeout(context.Background(), FileOperationTimeout)
	defer cancel()

	args := []string{"-la"}
	if recursive {
		args = []string{"-laR"}
	}
	args = append(args, path)

	cmd := exec.CommandContext(ctx, "ls", args...)
	output, err := cmd.CombinedOutput()

	// Check for timeout
	if ctx.Err() == context.DeadlineExceeded {
		return FileToolResult{Output: "Error: list directory timed out (30s limit)"}
	}

	if err != nil {
		return FileToolResult{Output: fmt.Sprintf("Error: %v\n%s", err, string(output))}
	}

	return FileToolResult{Success: true, Output: string(output)}
}

// DeleteFile removes a single file (not directories).
func DeleteFile(path string) FileToolResult {
	if safe, reason := IsPathSafe(path); !safe {
		return FileToolResult{Output: fmt.Sprintf("Blocked: %s", reason)}
	}

	absPath, err := filepath.Abs(path)
	if err != nil {
		return FileToolResult{Output: fmt.Sprintf("Error: invalid path: %v", err)}
	}

	info, err := os.Stat(absPath)
	if err != nil {
		if os.IsNotExist(err) {
			return FileToolResult{Output: fmt.Sprintf("Error: file not found: %s", path)}
		}
		return FileToolResult{Output: fmt.Sprintf("Error: %v", err)}
	}

	if info.IsDir() {
		return FileToolResult{Output: "Error: cannot delete directories with this tool, use execute_command with 'rm -r' instead"}
	}

	if err := os.Remove(absPath); err != nil {
		return FileToolResult{Output: fmt.Sprintf("Error: %v", err)}
	}

	return FileToolResult{Success: true, Output: fmt.Sprintf("Deleted %s", path)}
}

// GenerateDiff creates a simple unified diff for display.
func GenerateDiff(oldText, newText string) string {
	var sb strings.Builder

	oldLines := strings.Split(oldText, "\n")
	newLines := strings.Split(newText, "\n")

	sb.WriteString("--- old\n")
	sb.WriteString("+++ new\n")

	for _, line := range oldLines {
		sb.WriteString("- ")
		sb.WriteString(line)
		sb.WriteString("\n")
	}
	for _, line := range newLines {
		sb.WriteString("+ ")
		sb.WriteString(line)
		sb.WriteString("\n")
	}

	return sb.String()
}
