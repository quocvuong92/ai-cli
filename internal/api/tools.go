package api

// ExecuteCommandTool is the tool definition for command execution
var ExecuteCommandTool = Tool{
	Type: "function",
	Function: Function{
		Name:        "execute_command",
		Description: "Execute a shell command in the user's terminal and return the output. Use this to help users with system tasks, file operations, git commands, package management, and other terminal operations. The command will run in the user's current working directory.",
		Parameters: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"command": map[string]interface{}{
					"type":        "string",
					"description": "The shell command to execute (e.g., 'ls -la', 'git status', 'npm install')",
				},
				"reasoning": map[string]interface{}{
					"type":        "string",
					"description": "Brief explanation of why this command is needed to accomplish the user's request",
				},
			},
			"required": []string{"command", "reasoning"},
		},
	},
}

// ReadFileTool reads file contents with size limit
var ReadFileTool = Tool{
	Type: "function",
	Function: Function{
		Name:        "read_file",
		Description: "Read the contents of a file. Limited to 512KB. Use for viewing code, configs, or logs.",
		Parameters: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"path": map[string]interface{}{
					"type":        "string",
					"description": "File path (relative or absolute)",
				},
			},
			"required": []string{"path"},
		},
	},
}

// WriteFileTool creates or overwrites a file
var WriteFileTool = Tool{
	Type: "function",
	Function: Function{
		Name:        "write_file",
		Description: "Create a new file or overwrite an existing file. Creates parent directories if needed. Use for creating new files or completely replacing content.",
		Parameters: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"path": map[string]interface{}{
					"type":        "string",
					"description": "File path (relative or absolute)",
				},
				"content": map[string]interface{}{
					"type":        "string",
					"description": "Content to write to the file",
				},
			},
			"required": []string{"path", "content"},
		},
	},
}

// EditFileTool performs search and replace in a file
var EditFileTool = Tool{
	Type: "function",
	Function: Function{
		Name:        "edit_file",
		Description: "Edit a file by finding and replacing text. Shows diff preview before applying. The old_text must match exactly (including whitespace and indentation). Use for surgical edits to existing files.",
		Parameters: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"path": map[string]interface{}{
					"type":        "string",
					"description": "File path (relative or absolute)",
				},
				"old_text": map[string]interface{}{
					"type":        "string",
					"description": "Exact text to find and replace (must match exactly)",
				},
				"new_text": map[string]interface{}{
					"type":        "string",
					"description": "Text to replace with",
				},
			},
			"required": []string{"path", "old_text", "new_text"},
		},
	},
}

// SearchFilesTool searches for patterns in files
var SearchFilesTool = Tool{
	Type: "function",
	Function: Function{
		Name:        "search_files",
		Description: "Search for a pattern in files using ripgrep (with grep fallback). Returns matching lines with file paths and line numbers. Limited to 50 matches.",
		Parameters: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"pattern": map[string]interface{}{
					"type":        "string",
					"description": "Search pattern (supports regex)",
				},
				"path": map[string]interface{}{
					"type":        "string",
					"description": "Directory or file to search in (default: current directory)",
				},
				"file_type": map[string]interface{}{
					"type":        "string",
					"description": "File type filter, e.g., 'go', 'js', 'py', 'ts' (optional)",
				},
			},
			"required": []string{"pattern"},
		},
	},
}

// ListDirectoryTool lists directory contents
var ListDirectoryTool = Tool{
	Type: "function",
	Function: Function{
		Name:        "list_directory",
		Description: "List contents of a directory with file sizes and permissions.",
		Parameters: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"path": map[string]interface{}{
					"type":        "string",
					"description": "Directory path (default: current directory)",
				},
				"recursive": map[string]interface{}{
					"type":        "boolean",
					"description": "List recursively (default: false)",
				},
			},
			"required": []string{},
		},
	},
}

// DeleteFileTool removes a file
var DeleteFileTool = Tool{
	Type: "function",
	Function: Function{
		Name:        "delete_file",
		Description: "Delete a file (not directories). Requires confirmation. Cannot delete system files. Use with caution.",
		Parameters: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"path": map[string]interface{}{
					"type":        "string",
					"description": "Path to the file to delete",
				},
			},
			"required": []string{"path"},
		},
	},
}

// GetDefaultTools returns the default set of tools available to the AI
func GetDefaultTools() []Tool {
	return []Tool{
		ExecuteCommandTool,
		ReadFileTool,
		WriteFileTool,
		EditFileTool,
		SearchFilesTool,
		ListDirectoryTool,
		DeleteFileTool,
	}
}
