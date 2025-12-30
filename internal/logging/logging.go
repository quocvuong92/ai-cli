// Package logging provides structured logging with multiple levels and output formats.
//
// # Features
//
//   - Multiple log levels: Debug, Info, Warn, Error
//   - JSON output format for machine parsing
//   - Text output format for human readability
//   - Request/response logging for API debugging
//   - File and stderr output support
//   - Thread-safe operations
//
// # Usage
//
//	logger := logging.New(logging.Options{
//	    Level:  logging.LevelDebug,
//	    Format: logging.FormatJSON,
//	    Output: os.Stderr,
//	})
//
//	logger.Info("Starting application", logging.Fields{
//	    "version": "1.0.0",
//	    "model":   "gpt-4",
//	})
//
//	logger.Debug("API request", logging.Fields{
//	    "method": "POST",
//	    "url":    "https://api.example.com",
//	    "body":   requestBody,
//	})
package logging

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
	"sync"
	"time"
)

// Level represents a logging level
type Level int

const (
	// LevelDebug is for detailed debugging information
	LevelDebug Level = iota
	// LevelInfo is for general informational messages
	LevelInfo
	// LevelWarn is for warning messages
	LevelWarn
	// LevelError is for error messages
	LevelError
	// LevelNone disables all logging
	LevelNone
)

// String returns the string representation of the log level
func (l Level) String() string {
	switch l {
	case LevelDebug:
		return "DEBUG"
	case LevelInfo:
		return "INFO"
	case LevelWarn:
		return "WARN"
	case LevelError:
		return "ERROR"
	default:
		return "UNKNOWN"
	}
}

// ParseLevel parses a string into a Level
func ParseLevel(s string) Level {
	switch strings.ToUpper(s) {
	case "DEBUG":
		return LevelDebug
	case "INFO":
		return LevelInfo
	case "WARN", "WARNING":
		return LevelWarn
	case "ERROR":
		return LevelError
	case "NONE", "OFF":
		return LevelNone
	default:
		return LevelInfo
	}
}

// Format represents the output format
type Format int

const (
	// FormatText outputs human-readable text
	FormatText Format = iota
	// FormatJSON outputs machine-readable JSON
	FormatJSON
)

// Fields is a map of structured log fields
type Fields map[string]interface{}

// LogEntry represents a single log entry
type LogEntry struct {
	Timestamp time.Time `json:"timestamp"`
	Level     string    `json:"level"`
	Message   string    `json:"message"`
	Fields    Fields    `json:"fields,omitempty"`
	Error     string    `json:"error,omitempty"`
}

// Options configures the logger
type Options struct {
	Level  Level
	Format Format
	Output io.Writer
}

// Logger provides structured logging capabilities
type Logger struct {
	mu     sync.Mutex
	level  Level
	format Format
	output io.Writer
}

// DefaultLogger is a package-level logger for convenience
var DefaultLogger = New(Options{
	Level:  LevelInfo,
	Format: FormatText,
	Output: os.Stderr,
})

// New creates a new Logger with the given options
func New(opts Options) *Logger {
	if opts.Output == nil {
		opts.Output = os.Stderr
	}
	return &Logger{
		level:  opts.Level,
		format: opts.Format,
		output: opts.Output,
	}
}

// SetLevel changes the log level
func (l *Logger) SetLevel(level Level) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.level = level
}

// SetFormat changes the output format
func (l *Logger) SetFormat(format Format) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.format = format
}

// SetOutput changes the output writer
func (l *Logger) SetOutput(w io.Writer) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.output = w
}

// Debug logs a debug message
func (l *Logger) Debug(msg string, fields ...Fields) {
	l.log(LevelDebug, msg, nil, fields...)
}

// Info logs an info message
func (l *Logger) Info(msg string, fields ...Fields) {
	l.log(LevelInfo, msg, nil, fields...)
}

// Warn logs a warning message
func (l *Logger) Warn(msg string, fields ...Fields) {
	l.log(LevelWarn, msg, nil, fields...)
}

// Error logs an error message
func (l *Logger) Error(msg string, err error, fields ...Fields) {
	l.log(LevelError, msg, err, fields...)
}

// log is the internal logging function
func (l *Logger) log(level Level, msg string, err error, fields ...Fields) {
	l.mu.Lock()
	defer l.mu.Unlock()

	if level < l.level {
		return
	}

	entry := LogEntry{
		Timestamp: time.Now(),
		Level:     level.String(),
		Message:   msg,
	}

	// Merge all fields
	if len(fields) > 0 {
		merged := make(Fields)
		for _, f := range fields {
			for k, v := range f {
				merged[k] = v
			}
		}
		entry.Fields = merged
	}

	if err != nil {
		entry.Error = err.Error()
	}

	var output string
	if l.format == FormatJSON {
		output = l.formatJSON(entry)
	} else {
		output = l.formatText(entry)
	}

	fmt.Fprintln(l.output, output)
}

// formatJSON formats the entry as JSON
func (l *Logger) formatJSON(entry LogEntry) string {
	data, err := json.Marshal(entry)
	if err != nil {
		return fmt.Sprintf(`{"error":"failed to marshal log entry: %s"}`, err.Error())
	}
	return string(data)
}

// formatText formats the entry as human-readable text
func (l *Logger) formatText(entry LogEntry) string {
	timestamp := entry.Timestamp.Format("2006-01-02 15:04:05.000")

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("[%s] %s: %s", timestamp, entry.Level, entry.Message))

	if entry.Error != "" {
		sb.WriteString(fmt.Sprintf(" error=%q", entry.Error))
	}

	if len(entry.Fields) > 0 {
		for k, v := range entry.Fields {
			sb.WriteString(fmt.Sprintf(" %s=%v", k, v))
		}
	}

	return sb.String()
}

// WithFields creates a child logger with preset fields
func (l *Logger) WithFields(fields Fields) *FieldLogger {
	return &FieldLogger{
		logger: l,
		fields: fields,
	}
}

// FieldLogger is a logger with preset fields
type FieldLogger struct {
	logger *Logger
	fields Fields
}

// Debug logs a debug message with preset fields
func (fl *FieldLogger) Debug(msg string, fields ...Fields) {
	fl.logger.Debug(msg, fl.mergeFields(fields...)...)
}

// Info logs an info message with preset fields
func (fl *FieldLogger) Info(msg string, fields ...Fields) {
	fl.logger.Info(msg, fl.mergeFields(fields...)...)
}

// Warn logs a warning message with preset fields
func (fl *FieldLogger) Warn(msg string, fields ...Fields) {
	fl.logger.Warn(msg, fl.mergeFields(fields...)...)
}

// Error logs an error message with preset fields
func (fl *FieldLogger) Error(msg string, err error, fields ...Fields) {
	fl.logger.Error(msg, err, fl.mergeFields(fields...)...)
}

// mergeFields merges preset fields with additional fields
func (fl *FieldLogger) mergeFields(fields ...Fields) []Fields {
	result := make([]Fields, 0, len(fields)+1)
	result = append(result, fl.fields)
	result = append(result, fields...)
	return result
}

// Package-level convenience functions using DefaultLogger

// Debug logs a debug message using the default logger
func Debug(msg string, fields ...Fields) {
	DefaultLogger.Debug(msg, fields...)
}

// Info logs an info message using the default logger
func Info(msg string, fields ...Fields) {
	DefaultLogger.Info(msg, fields...)
}

// Warn logs a warning message using the default logger
func Warn(msg string, fields ...Fields) {
	DefaultLogger.Warn(msg, fields...)
}

// Error logs an error message using the default logger
func Error(msg string, err error, fields ...Fields) {
	DefaultLogger.Error(msg, err, fields...)
}

// SetLevel sets the level of the default logger
func SetLevel(level Level) {
	DefaultLogger.SetLevel(level)
}

// SetFormat sets the format of the default logger
func SetFormat(format Format) {
	DefaultLogger.SetFormat(format)
}
