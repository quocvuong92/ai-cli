// Package cmd implements the CLI commands for the AI CLI application.
package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/quocvuong92/ai-cli/internal/api"
	"github.com/quocvuong92/ai-cli/internal/display"
	"github.com/quocvuong92/ai-cli/internal/executor"
)

// processToolCall dispatches a tool call to the appropriate handler.
func (app *App) processToolCall(tc api.ToolCall, exec *executor.Executor, ctx context.Context, session *InteractiveSession) string {
	switch tc.Function.Name {
	case "execute_command":
		return app.handleExecuteCommand(tc, exec, ctx)
	case "read_file":
		return app.handleReadFile(tc)
	case "write_file":
		return app.handleWriteFile(tc)
	case "edit_file":
		return app.handleEditFile(tc)
	case "search_files":
		return app.handleSearchFiles(tc)
	case "list_directory":
		return app.handleListDirectory(tc)
	case "delete_file":
		return app.handleDeleteFile(tc, exec)
	case "update_plan":
		return app.handleUpdatePlan(tc, session)
	default:
		return fmt.Sprintf("Unknown tool: %s", tc.Function.Name)
	}
}

// handleExecuteCommand handles the execute_command tool call.
func (app *App) handleExecuteCommand(tc api.ToolCall, exec *executor.Executor, ctx context.Context) string {
	var args struct {
		Command   string `json:"command"`
		Reasoning string `json:"reasoning"`
	}
	if err := json.Unmarshal([]byte(tc.Function.Arguments), &args); err != nil {
		return fmt.Sprintf("Error parsing arguments: %v", err)
	}

	// Check permission
	allowed, needsConfirm, reason := exec.GetPermissionManager().CheckPermission(args.Command)

	if !allowed && !needsConfirm {
		display.ShowCommandBlocked(args.Command, reason)
		return fmt.Sprintf("Command blocked: %s", reason)
	}

	// Ask for confirmation if needed
	if needsConfirm {
		choice := display.AskCommandConfirmationExtended(args.Command, args.Reasoning)
		if choice == display.ApprovalDenied {
			return "Command execution denied by user"
		}
		// Handle different approval types
		switch choice {
		case display.ApprovalSession:
			_ = exec.GetPermissionManager().AddToAllowlist(args.Command, executor.ApprovalSession)
		case display.ApprovalAlways:
			if err := exec.GetPermissionManager().AddToAllowlist(args.Command, executor.ApprovalAlways); err != nil {
				fmt.Fprintf(os.Stderr, "Warning: Failed to save permission: %v\n", err)
			}
		}
	}

	// Execute the command
	display.ShowCommandExecuting(args.Command)
	result, err := exec.Execute(ctx, args.Command)

	if err != nil || !result.IsSuccess() {
		display.ShowCommandError(args.Command, result.Error)
		return result.FormatResult()
	}

	display.ShowCommandOutput(result.Output)
	if result.Output == "" {
		return "Command executed successfully (no output)"
	}
	return result.Output
}

// handleReadFile handles the read_file tool call (safe - no confirmation).
func (app *App) handleReadFile(tc api.ToolCall) string {
	var args struct {
		Path string `json:"path"`
	}
	if err := json.Unmarshal([]byte(tc.Function.Arguments), &args); err != nil {
		return fmt.Sprintf("Error parsing arguments: %v", err)
	}

	display.ShowFileOperation("read", args.Path)
	result := executor.ReadFile(args.Path)

	if result.Truncated {
		display.ShowWarning("File truncated to 512KB")
	}

	return result.Output
}

// handleWriteFile handles the write_file tool call (needs confirmation).
func (app *App) handleWriteFile(tc api.ToolCall) string {
	var args struct {
		Path    string `json:"path"`
		Content string `json:"content"`
	}
	if err := json.Unmarshal([]byte(tc.Function.Arguments), &args); err != nil {
		return fmt.Sprintf("Error parsing arguments: %v", err)
	}

	// Check path safety first
	if safe, reason := executor.IsPathSafe(args.Path); !safe {
		display.ShowFileBlocked("write", args.Path, reason)
		return fmt.Sprintf("Blocked: %s", reason)
	}

	// Show preview and ask for confirmation
	display.ShowFileOperation("write", args.Path)
	fmt.Fprintf(os.Stderr, "Content length: %d bytes\n", len(args.Content))

	choice := display.AskFileConfirmation("write", args.Path)
	if choice == display.ApprovalDenied {
		return "Write denied by user"
	}

	result := executor.WriteFile(args.Path, args.Content)
	return result.Output
}

// handleEditFile handles the edit_file tool call (needs confirmation + diff preview).
func (app *App) handleEditFile(tc api.ToolCall) string {
	var args struct {
		Path    string `json:"path"`
		OldText string `json:"old_text"`
		NewText string `json:"new_text"`
	}
	if err := json.Unmarshal([]byte(tc.Function.Arguments), &args); err != nil {
		return fmt.Sprintf("Error parsing arguments: %v", err)
	}

	// Check path safety first
	if safe, reason := executor.IsPathSafe(args.Path); !safe {
		display.ShowFileBlocked("edit", args.Path, reason)
		return fmt.Sprintf("Blocked: %s", reason)
	}

	// Generate and show diff preview
	diff := executor.GenerateDiff(args.OldText, args.NewText)
	display.ShowFileOperation("edit", args.Path)
	display.ShowDiff(args.Path, diff)

	// Ask for confirmation
	choice := display.AskFileConfirmation("edit", args.Path)
	if choice == display.ApprovalDenied {
		return "Edit denied by user"
	}

	result, _ := executor.EditFile(args.Path, args.OldText, args.NewText)
	return result.Output
}

// handleSearchFiles handles the search_files tool call (safe - no confirmation).
func (app *App) handleSearchFiles(tc api.ToolCall) string {
	var args struct {
		Pattern  string `json:"pattern"`
		Path     string `json:"path"`
		FileType string `json:"file_type"`
	}
	if err := json.Unmarshal([]byte(tc.Function.Arguments), &args); err != nil {
		return fmt.Sprintf("Error parsing arguments: %v", err)
	}

	searchPath := args.Path
	if searchPath == "" {
		searchPath = "."
	}
	display.ShowFileOperation("search", fmt.Sprintf("%s in %s", args.Pattern, searchPath))

	result := executor.SearchFiles(args.Pattern, args.Path, args.FileType)
	return result.Output
}

// handleListDirectory handles the list_directory tool call (safe - no confirmation).
func (app *App) handleListDirectory(tc api.ToolCall) string {
	var args struct {
		Path      string `json:"path"`
		Recursive bool   `json:"recursive"`
	}
	if err := json.Unmarshal([]byte(tc.Function.Arguments), &args); err != nil {
		return fmt.Sprintf("Error parsing arguments: %v", err)
	}

	listPath := args.Path
	if listPath == "" {
		listPath = "."
	}
	display.ShowFileOperation("list", listPath)

	result := executor.ListDirectory(args.Path, args.Recursive)
	return result.Output
}

// handleDeleteFile handles the delete_file tool call (dangerous - needs confirmation).
func (app *App) handleDeleteFile(tc api.ToolCall, exec *executor.Executor) string {
	var args struct {
		Path string `json:"path"`
	}
	if err := json.Unmarshal([]byte(tc.Function.Arguments), &args); err != nil {
		return fmt.Sprintf("Error parsing arguments: %v", err)
	}

	// Check path safety first
	if safe, reason := executor.IsPathSafe(args.Path); !safe {
		display.ShowFileBlocked("delete", args.Path, reason)
		return fmt.Sprintf("Blocked: %s", reason)
	}

	// Check if dangerous commands are enabled
	settings := exec.GetPermissionManager().GetSettings()
	dangerousEnabled, _ := settings["dangerous_enabled"].(bool)

	if !dangerousEnabled {
		display.ShowFileBlocked("delete", args.Path, "Dangerous operations are disabled. Use /allow-dangerous to enable.")
		return "Delete blocked: dangerous operations are disabled. Use /allow-dangerous to enable."
	}

	display.ShowFileOperation("delete", args.Path)

	// Always ask for confirmation for delete
	choice := display.AskFileConfirmation("delete", args.Path)
	if choice == display.ApprovalDenied {
		return "Delete denied by user"
	}

	result := executor.DeleteFile(args.Path)
	return result.Output
}

// handleUpdatePlan handles the update_plan tool call for task tracking.
func (app *App) handleUpdatePlan(tc api.ToolCall, session *InteractiveSession) string {
	var args struct {
		Title string `json:"title"`
		Items []struct {
			Description string `json:"description"`
			Status      string `json:"status"`
		} `json:"items"`
	}
	if err := json.Unmarshal([]byte(tc.Function.Arguments), &args); err != nil {
		return fmt.Sprintf("Error parsing arguments: %v", err)
	}

	// Convert to display.Plan
	plan := &display.Plan{
		Title: args.Title,
		Items: make([]display.PlanItem, len(args.Items)),
	}
	for i, item := range args.Items {
		plan.Items[i] = display.PlanItem{
			Description: item.Description,
			Status:      item.Status,
		}
	}

	// Store in session
	session.currentPlan = plan

	// Display the plan
	display.ShowPlan(plan)

	return fmt.Sprintf("Plan updated: %s (%d items)", args.Title, len(args.Items))
}
