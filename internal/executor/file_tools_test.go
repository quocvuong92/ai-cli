package executor

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// createTestDir creates a temporary directory that is not under blocked paths.
// On macOS, t.TempDir() returns /var/folders/... which is blocked.
func createTestDir(t *testing.T) string {
	t.Helper()
	dir, err := os.MkdirTemp("/tmp", "ai-cli-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	t.Cleanup(func() { os.RemoveAll(dir) })
	return dir
}

func TestIsPathSafe(t *testing.T) {
	tests := []struct {
		name     string
		path     string
		wantSafe bool
	}{
		{"safe relative path", "test.txt", true},
		{"safe absolute path in tmp", "/tmp/test.txt", true},
		{"blocked /etc path", "/etc/passwd", false},
		{"blocked /usr path", "/usr/bin/test", false},
		{"blocked /bin path", "/bin/bash", false},
		{"blocked /var path", "/var/log/test", false},
		{"blocked /sys path", "/sys/test", false},
		{"blocked /proc path", "/proc/1/status", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			safe, _ := IsPathSafe(tt.path)
			if safe != tt.wantSafe {
				t.Errorf("IsPathSafe(%q) = %v, want %v", tt.path, safe, tt.wantSafe)
			}
		})
	}
}

func TestReadFile(t *testing.T) {
	// Create a temporary file
	tmpDir := createTestDir(t)
	testFile := filepath.Join(tmpDir, "test.txt")
	content := "Hello, World!"
	if err := os.WriteFile(testFile, []byte(content), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	t.Run("read existing file", func(t *testing.T) {
		result := ReadFile(testFile)
		if !result.Success {
			t.Errorf("ReadFile failed: %s", result.Output)
		}
		if result.Output != content {
			t.Errorf("ReadFile output = %q, want %q", result.Output, content)
		}
	})

	t.Run("read non-existent file", func(t *testing.T) {
		result := ReadFile("/nonexistent/file.txt")
		if result.Success {
			t.Error("ReadFile should fail for non-existent file")
		}
		if !strings.Contains(result.Output, "Error") {
			t.Errorf("Expected error message, got: %s", result.Output)
		}
	})

	t.Run("read directory", func(t *testing.T) {
		result := ReadFile(tmpDir)
		if result.Success {
			t.Error("ReadFile should fail for directory")
		}
		if !strings.Contains(result.Output, "directory") {
			t.Errorf("Expected directory error, got: %s", result.Output)
		}
	})
}

func TestReadFileTruncation(t *testing.T) {
	// Create a file larger than MaxFileSize
	tmpDir := createTestDir(t)
	testFile := filepath.Join(tmpDir, "large.txt")

	// Create content larger than 512KB
	largeContent := strings.Repeat("x", MaxFileSize+1000)
	if err := os.WriteFile(testFile, []byte(largeContent), 0644); err != nil {
		t.Fatalf("Failed to create large test file: %v", err)
	}

	result := ReadFile(testFile)
	if !result.Success {
		t.Errorf("ReadFile failed: %s", result.Output)
	}
	if !result.Truncated {
		t.Error("Expected file to be truncated")
	}
	if !strings.Contains(result.Output, "Truncated") {
		t.Error("Expected truncation message in output")
	}
}

func TestWriteFile(t *testing.T) {
	tmpDir := createTestDir(t)

	t.Run("write new file", func(t *testing.T) {
		testFile := filepath.Join(tmpDir, "new.txt")
		content := "New content"

		result := WriteFile(testFile, content)
		if !result.Success {
			t.Errorf("WriteFile failed: %s", result.Output)
		}

		// Verify content was written
		data, err := os.ReadFile(testFile)
		if err != nil {
			t.Fatalf("Failed to read written file: %v", err)
		}
		if string(data) != content {
			t.Errorf("Written content = %q, want %q", string(data), content)
		}
	})

	t.Run("write creates parent directories", func(t *testing.T) {
		testFile := filepath.Join(tmpDir, "subdir", "deep", "file.txt")
		content := "Deep content"

		result := WriteFile(testFile, content)
		if !result.Success {
			t.Errorf("WriteFile failed: %s", result.Output)
		}

		// Verify file exists
		if _, err := os.Stat(testFile); os.IsNotExist(err) {
			t.Error("WriteFile did not create parent directories")
		}
	})

	t.Run("write to blocked path", func(t *testing.T) {
		result := WriteFile("/etc/test.txt", "blocked")
		if result.Success {
			t.Error("WriteFile should fail for blocked path")
		}
		if !strings.Contains(result.Output, "Blocked") {
			t.Errorf("Expected blocked message, got: %s", result.Output)
		}
	})
}

func TestEditFile(t *testing.T) {
	tmpDir := createTestDir(t)

	t.Run("edit existing text", func(t *testing.T) {
		testFile := filepath.Join(tmpDir, "edit.txt")
		original := "Hello, World!"
		if err := os.WriteFile(testFile, []byte(original), 0644); err != nil {
			t.Fatalf("Failed to create test file: %v", err)
		}

		result, diff := EditFile(testFile, "World", "Go")
		if !result.Success {
			t.Errorf("EditFile failed: %s", result.Output)
		}

		// Verify content was changed
		data, _ := os.ReadFile(testFile)
		if string(data) != "Hello, Go!" {
			t.Errorf("Edited content = %q, want %q", string(data), "Hello, Go!")
		}

		// Verify diff was generated
		if diff == "" {
			t.Error("Expected diff to be generated")
		}
	})

	t.Run("edit text not found", func(t *testing.T) {
		testFile := filepath.Join(tmpDir, "edit2.txt")
		if err := os.WriteFile(testFile, []byte("Hello"), 0644); err != nil {
			t.Fatalf("Failed to create test file: %v", err)
		}

		result, _ := EditFile(testFile, "NotFound", "Replacement")
		if result.Success {
			t.Error("EditFile should fail when text not found")
		}
		if !strings.Contains(result.Output, "not found") {
			t.Errorf("Expected 'not found' message, got: %s", result.Output)
		}
	})

	t.Run("edit blocked path", func(t *testing.T) {
		result, _ := EditFile("/etc/passwd", "old", "new")
		if result.Success {
			t.Error("EditFile should fail for blocked path")
		}
	})
}

func TestSearchFiles(t *testing.T) {
	tmpDir := createTestDir(t)

	// Create test files
	file1 := filepath.Join(tmpDir, "file1.go")
	file2 := filepath.Join(tmpDir, "file2.go")
	os.WriteFile(file1, []byte("package main\nfunc Hello() {}"), 0644)
	os.WriteFile(file2, []byte("package main\nfunc World() {}"), 0644)

	t.Run("search with matches", func(t *testing.T) {
		result := SearchFiles("func", tmpDir, "")
		if !result.Success {
			t.Errorf("SearchFiles failed: %s", result.Output)
		}
		if !strings.Contains(result.Output, "func") {
			t.Errorf("Expected matches in output, got: %s", result.Output)
		}
	})

	t.Run("search no matches", func(t *testing.T) {
		result := SearchFiles("NOTFOUND", tmpDir, "")
		if !result.Success {
			t.Errorf("SearchFiles failed: %s", result.Output)
		}
		if !strings.Contains(result.Output, "No matches") {
			t.Errorf("Expected 'No matches' message, got: %s", result.Output)
		}
	})
}

func TestListDirectory(t *testing.T) {
	tmpDir := createTestDir(t)

	// Create test files
	os.WriteFile(filepath.Join(tmpDir, "file1.txt"), []byte("test"), 0644)
	os.Mkdir(filepath.Join(tmpDir, "subdir"), 0755)

	t.Run("list directory", func(t *testing.T) {
		result := ListDirectory(tmpDir, false)
		if !result.Success {
			t.Errorf("ListDirectory failed: %s", result.Output)
		}
		if !strings.Contains(result.Output, "file1.txt") {
			t.Errorf("Expected file1.txt in output, got: %s", result.Output)
		}
		if !strings.Contains(result.Output, "subdir") {
			t.Errorf("Expected subdir in output, got: %s", result.Output)
		}
	})

	t.Run("list non-existent directory", func(t *testing.T) {
		result := ListDirectory("/nonexistent/dir", false)
		if result.Success {
			t.Error("ListDirectory should fail for non-existent directory")
		}
	})
}

func TestDeleteFile(t *testing.T) {
	tmpDir := createTestDir(t)

	t.Run("delete existing file", func(t *testing.T) {
		testFile := filepath.Join(tmpDir, "delete.txt")
		os.WriteFile(testFile, []byte("delete me"), 0644)

		result := DeleteFile(testFile)
		if !result.Success {
			t.Errorf("DeleteFile failed: %s", result.Output)
		}

		// Verify file was deleted
		if _, err := os.Stat(testFile); !os.IsNotExist(err) {
			t.Error("File should have been deleted")
		}
	})

	t.Run("delete non-existent file", func(t *testing.T) {
		result := DeleteFile("/nonexistent/file.txt")
		if result.Success {
			t.Error("DeleteFile should fail for non-existent file")
		}
	})

	t.Run("delete directory should fail", func(t *testing.T) {
		subdir := filepath.Join(tmpDir, "subdir_to_delete")
		os.Mkdir(subdir, 0755)

		result := DeleteFile(subdir)
		if result.Success {
			t.Error("DeleteFile should fail for directory")
		}
		if !strings.Contains(result.Output, "cannot delete directories") {
			t.Errorf("Expected directory error, got: %s", result.Output)
		}
	})

	t.Run("delete blocked path", func(t *testing.T) {
		result := DeleteFile("/etc/passwd")
		if result.Success {
			t.Error("DeleteFile should fail for blocked path")
		}
	})
}

func TestGenerateDiff(t *testing.T) {
	old := "line1\nline2"
	new := "line1\nline3"

	diff := GenerateDiff(old, new)

	if !strings.Contains(diff, "--- old") {
		t.Error("Expected '--- old' header in diff")
	}
	if !strings.Contains(diff, "+++ new") {
		t.Error("Expected '+++ new' header in diff")
	}
	if !strings.Contains(diff, "- line1") {
		t.Error("Expected removed lines to start with '- '")
	}
	if !strings.Contains(diff, "+ line1") {
		t.Error("Expected added lines to start with '+ '")
	}
}
