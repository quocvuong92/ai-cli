package logging

import (
	"bytes"
	"encoding/json"
	"errors"
	"strings"
	"testing"
)

func TestLevel_String(t *testing.T) {
	tests := []struct {
		level Level
		want  string
	}{
		{LevelDebug, "DEBUG"},
		{LevelInfo, "INFO"},
		{LevelWarn, "WARN"},
		{LevelError, "ERROR"},
		{Level(99), "UNKNOWN"},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			if got := tt.level.String(); got != tt.want {
				t.Errorf("Level.String() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestParseLevel(t *testing.T) {
	tests := []struct {
		input string
		want  Level
	}{
		{"debug", LevelDebug},
		{"DEBUG", LevelDebug},
		{"info", LevelInfo},
		{"INFO", LevelInfo},
		{"warn", LevelWarn},
		{"WARN", LevelWarn},
		{"warning", LevelWarn},
		{"error", LevelError},
		{"ERROR", LevelError},
		{"none", LevelNone},
		{"off", LevelNone},
		{"invalid", LevelInfo}, // Default to Info
		{"", LevelInfo},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			if got := ParseLevel(tt.input); got != tt.want {
				t.Errorf("ParseLevel(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

func TestLogger_TextFormat(t *testing.T) {
	var buf bytes.Buffer
	logger := New(Options{
		Level:  LevelDebug,
		Format: FormatText,
		Output: &buf,
	})

	logger.Info("test message", Fields{"key": "value"})

	output := buf.String()
	if !strings.Contains(output, "INFO") {
		t.Error("Expected output to contain 'INFO'")
	}
	if !strings.Contains(output, "test message") {
		t.Error("Expected output to contain 'test message'")
	}
	if !strings.Contains(output, "key=value") {
		t.Error("Expected output to contain 'key=value'")
	}
}

func TestLogger_JSONFormat(t *testing.T) {
	var buf bytes.Buffer
	logger := New(Options{
		Level:  LevelDebug,
		Format: FormatJSON,
		Output: &buf,
	})

	logger.Info("test message", Fields{"key": "value"})

	var entry LogEntry
	if err := json.Unmarshal(buf.Bytes(), &entry); err != nil {
		t.Fatalf("Failed to parse JSON output: %v", err)
	}

	if entry.Level != "INFO" {
		t.Errorf("Level = %q, want %q", entry.Level, "INFO")
	}
	if entry.Message != "test message" {
		t.Errorf("Message = %q, want %q", entry.Message, "test message")
	}
	if entry.Fields["key"] != "value" {
		t.Errorf("Fields[key] = %v, want %q", entry.Fields["key"], "value")
	}
}

func TestLogger_LevelFiltering(t *testing.T) {
	var buf bytes.Buffer
	logger := New(Options{
		Level:  LevelWarn,
		Format: FormatText,
		Output: &buf,
	})

	logger.Debug("debug message")
	logger.Info("info message")
	logger.Warn("warn message")
	logger.Error("error message", nil)

	output := buf.String()
	if strings.Contains(output, "debug message") {
		t.Error("Debug message should be filtered out")
	}
	if strings.Contains(output, "info message") {
		t.Error("Info message should be filtered out")
	}
	if !strings.Contains(output, "warn message") {
		t.Error("Warn message should be present")
	}
	if !strings.Contains(output, "error message") {
		t.Error("Error message should be present")
	}
}

func TestLogger_ErrorLogging(t *testing.T) {
	var buf bytes.Buffer
	logger := New(Options{
		Level:  LevelDebug,
		Format: FormatJSON,
		Output: &buf,
	})

	testErr := errors.New("test error")
	logger.Error("something went wrong", testErr)

	var entry LogEntry
	if err := json.Unmarshal(buf.Bytes(), &entry); err != nil {
		t.Fatalf("Failed to parse JSON output: %v", err)
	}

	if entry.Error != "test error" {
		t.Errorf("Error = %q, want %q", entry.Error, "test error")
	}
}

func TestLogger_SetLevel(t *testing.T) {
	var buf bytes.Buffer
	logger := New(Options{
		Level:  LevelError,
		Format: FormatText,
		Output: &buf,
	})

	logger.Info("should not appear")
	if buf.Len() > 0 {
		t.Error("Info should be filtered at Error level")
	}

	logger.SetLevel(LevelInfo)
	logger.Info("should appear")
	if !strings.Contains(buf.String(), "should appear") {
		t.Error("Info should appear after level change")
	}
}

func TestLogger_WithFields(t *testing.T) {
	var buf bytes.Buffer
	logger := New(Options{
		Level:  LevelDebug,
		Format: FormatJSON,
		Output: &buf,
	})

	fieldLogger := logger.WithFields(Fields{"component": "test"})
	fieldLogger.Info("message", Fields{"extra": "field"})

	var entry LogEntry
	if err := json.Unmarshal(buf.Bytes(), &entry); err != nil {
		t.Fatalf("Failed to parse JSON output: %v", err)
	}

	if entry.Fields["component"] != "test" {
		t.Error("Expected preset field 'component'")
	}
	if entry.Fields["extra"] != "field" {
		t.Error("Expected additional field 'extra'")
	}
}

func TestLogger_MultipleFields(t *testing.T) {
	var buf bytes.Buffer
	logger := New(Options{
		Level:  LevelDebug,
		Format: FormatJSON,
		Output: &buf,
	})

	logger.Info("message",
		Fields{"a": 1},
		Fields{"b": 2},
		Fields{"c": 3},
	)

	var entry LogEntry
	if err := json.Unmarshal(buf.Bytes(), &entry); err != nil {
		t.Fatalf("Failed to parse JSON output: %v", err)
	}

	if entry.Fields["a"] != float64(1) {
		t.Error("Expected field 'a'")
	}
	if entry.Fields["b"] != float64(2) {
		t.Error("Expected field 'b'")
	}
	if entry.Fields["c"] != float64(3) {
		t.Error("Expected field 'c'")
	}
}

func TestLogger_NoneLevel(t *testing.T) {
	var buf bytes.Buffer
	logger := New(Options{
		Level:  LevelNone,
		Format: FormatText,
		Output: &buf,
	})

	logger.Debug("debug")
	logger.Info("info")
	logger.Warn("warn")
	logger.Error("error", nil)

	if buf.Len() > 0 {
		t.Error("No messages should be logged at None level")
	}
}

func TestIsSensitiveHeader(t *testing.T) {
	tests := []struct {
		header string
		want   bool
	}{
		{"Authorization", true},
		{"authorization", true},
		{"Api-Key", true},
		{"X-API-KEY", true},
		{"Cookie", true},
		{"Content-Type", false},
		{"Accept", false},
		{"User-Agent", false},
	}

	for _, tt := range tests {
		t.Run(tt.header, func(t *testing.T) {
			if got := isSensitiveHeader(tt.header); got != tt.want {
				t.Errorf("isSensitiveHeader(%q) = %v, want %v", tt.header, got, tt.want)
			}
		})
	}
}

func TestTruncateBody(t *testing.T) {
	tests := []struct {
		name    string
		body    []byte
		maxSize int
		wantLen int
		wantEnd string
	}{
		{
			name:    "small body",
			body:    []byte("hello"),
			maxSize: 100,
			wantLen: 5,
			wantEnd: "hello",
		},
		{
			name:    "large body",
			body:    []byte(strings.Repeat("a", 200)),
			maxSize: 50,
			wantEnd: "...[truncated]",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := truncateBody(tt.body, tt.maxSize)
			if !strings.HasSuffix(got, tt.wantEnd) {
				t.Errorf("truncateBody() should end with %q", tt.wantEnd)
			}
		})
	}
}

func TestRedactSensitiveFields(t *testing.T) {
	input := map[string]interface{}{
		"username": "john",
		"password": "secret123",
		"api_key":  "key123",
		"data": map[string]interface{}{
			"token":   "token123",
			"message": "hello",
		},
	}

	result := redactSensitiveFields(input).(map[string]interface{})

	if result["username"] != "john" {
		t.Error("username should not be redacted")
	}
	if result["password"] != "[REDACTED]" {
		t.Error("password should be redacted")
	}
	if result["api_key"] != "[REDACTED]" {
		t.Error("api_key should be redacted")
	}

	nested := result["data"].(map[string]interface{})
	if nested["token"] != "[REDACTED]" {
		t.Error("nested token should be redacted")
	}
	if nested["message"] != "hello" {
		t.Error("nested message should not be redacted")
	}
}
