package logging

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"time"
)

// HTTPLogger provides request/response logging for HTTP clients
type HTTPLogger struct {
	logger      *Logger
	maxBodySize int
}

// NewHTTPLogger creates a new HTTP logger
func NewHTTPLogger(logger *Logger) *HTTPLogger {
	return &HTTPLogger{
		logger:      logger,
		maxBodySize: 10000, // Default 10KB max body logging
	}
}

// SetMaxBodySize sets the maximum body size to log (in bytes)
func (h *HTTPLogger) SetMaxBodySize(size int) {
	h.maxBodySize = size
}

// LogRequest logs an HTTP request
func (h *HTTPLogger) LogRequest(req *http.Request, body []byte) {
	fields := Fields{
		"method": req.Method,
		"url":    req.URL.String(),
		"host":   req.Host,
	}

	// Log headers (redact sensitive ones)
	headers := make(map[string]string)
	for k, v := range req.Header {
		if isSensitiveHeader(k) {
			headers[k] = "[REDACTED]"
		} else if len(v) > 0 {
			headers[k] = v[0]
		}
	}
	fields["headers"] = headers

	// Log body if present and not too large
	if len(body) > 0 {
		bodyStr := truncateBody(body, h.maxBodySize)
		// Try to parse as JSON for prettier output
		if json.Valid(body) {
			var parsed interface{}
			if err := json.Unmarshal(body, &parsed); err == nil {
				// Redact sensitive fields in JSON body
				redacted := redactSensitiveFields(parsed)
				fields["body"] = redacted
			} else {
				fields["body"] = bodyStr
			}
		} else {
			fields["body"] = bodyStr
		}
		fields["body_size"] = len(body)
	}

	h.logger.Debug("HTTP Request", fields)
}

// LogResponse logs an HTTP response
func (h *HTTPLogger) LogResponse(resp *http.Response, body []byte, duration time.Duration) {
	fields := Fields{
		"status":      resp.StatusCode,
		"status_text": resp.Status,
		"duration_ms": duration.Milliseconds(),
	}

	// Log headers
	headers := make(map[string]string)
	for k, v := range resp.Header {
		if len(v) > 0 {
			headers[k] = v[0]
		}
	}
	fields["headers"] = headers

	// Log body if present and not too large
	if len(body) > 0 {
		bodyStr := truncateBody(body, h.maxBodySize)
		if json.Valid(body) {
			var parsed interface{}
			if err := json.Unmarshal(body, &parsed); err == nil {
				fields["body"] = parsed
			} else {
				fields["body"] = bodyStr
			}
		} else {
			fields["body"] = bodyStr
		}
		fields["body_size"] = len(body)
	}

	h.logger.Debug("HTTP Response", fields)
}

// LogStreamStart logs the start of a streaming response
func (h *HTTPLogger) LogStreamStart(resp *http.Response) {
	fields := Fields{
		"status":      resp.StatusCode,
		"status_text": resp.Status,
		"streaming":   true,
	}

	h.logger.Debug("HTTP Stream Started", fields)
}

// LogStreamChunk logs a streaming chunk (for very verbose debugging)
func (h *HTTPLogger) LogStreamChunk(chunk []byte, chunkNum int) {
	fields := Fields{
		"chunk_num":  chunkNum,
		"chunk_size": len(chunk),
	}

	if len(chunk) <= 500 {
		fields["chunk"] = string(chunk)
	} else {
		fields["chunk"] = string(chunk[:500]) + "...[truncated]"
	}

	h.logger.Debug("HTTP Stream Chunk", fields)
}

// LogStreamEnd logs the end of a streaming response
func (h *HTTPLogger) LogStreamEnd(duration time.Duration, totalBytes int, chunkCount int) {
	fields := Fields{
		"duration_ms": duration.Milliseconds(),
		"total_bytes": totalBytes,
		"chunk_count": chunkCount,
	}

	h.logger.Debug("HTTP Stream Ended", fields)
}

// LogError logs an HTTP error
func (h *HTTPLogger) LogError(err error, req *http.Request) {
	fields := Fields{
		"method": req.Method,
		"url":    req.URL.String(),
	}

	h.logger.Error("HTTP Error", err, fields)
}

// RoundTripperWrapper wraps an http.RoundTripper with logging
type RoundTripperWrapper struct {
	wrapped http.RoundTripper
	logger  *HTTPLogger
	logBody bool
}

// NewLoggingRoundTripper creates a new logging round tripper
func NewLoggingRoundTripper(wrapped http.RoundTripper, logger *HTTPLogger, logBody bool) *RoundTripperWrapper {
	if wrapped == nil {
		wrapped = http.DefaultTransport
	}
	return &RoundTripperWrapper{
		wrapped: wrapped,
		logger:  logger,
		logBody: logBody,
	}
}

// RoundTrip implements http.RoundTripper
func (rt *RoundTripperWrapper) RoundTrip(req *http.Request) (*http.Response, error) {
	start := time.Now()

	// Log request
	var reqBody []byte
	if rt.logBody && req.Body != nil {
		reqBody, _ = io.ReadAll(req.Body)
		req.Body = io.NopCloser(bytes.NewBuffer(reqBody))
		rt.logger.LogRequest(req, reqBody)
	} else {
		rt.logger.LogRequest(req, nil)
	}

	// Execute request
	resp, err := rt.wrapped.RoundTrip(req)
	duration := time.Since(start)

	if err != nil {
		rt.logger.LogError(err, req)
		return nil, err
	}

	// For non-streaming responses, log the full response
	if !isStreamingResponse(resp) && rt.logBody {
		respBody, _ := io.ReadAll(resp.Body)
		resp.Body = io.NopCloser(bytes.NewBuffer(respBody))
		rt.logger.LogResponse(resp, respBody, duration)
	} else {
		rt.logger.LogResponse(resp, nil, duration)
	}

	return resp, nil
}

// Helper functions

// isSensitiveHeader checks if a header should be redacted
func isSensitiveHeader(name string) bool {
	sensitive := []string{
		"authorization",
		"api-key",
		"x-api-key",
		"x-auth-token",
		"cookie",
		"set-cookie",
	}
	nameLower := strings.ToLower(name)
	for _, s := range sensitive {
		if nameLower == s {
			return true
		}
	}
	return false
}

// truncateBody truncates body if too large
func truncateBody(body []byte, maxSize int) string {
	if len(body) <= maxSize {
		return string(body)
	}
	return string(body[:maxSize]) + "...[truncated]"
}

// isStreamingResponse checks if response is a streaming response
func isStreamingResponse(resp *http.Response) bool {
	contentType := resp.Header.Get("Content-Type")
	return strings.Contains(contentType, "text/event-stream") ||
		strings.Contains(contentType, "application/x-ndjson")
}

// redactSensitiveFields redacts sensitive fields in parsed JSON
func redactSensitiveFields(data interface{}) interface{} {
	sensitiveKeys := []string{
		"api_key", "apiKey", "api-key",
		"password", "secret", "token",
		"authorization", "auth",
	}

	switch v := data.(type) {
	case map[string]interface{}:
		result := make(map[string]interface{})
		for k, val := range v {
			keyLower := strings.ToLower(k)
			isSensitive := false
			for _, sensitive := range sensitiveKeys {
				if strings.Contains(keyLower, sensitive) {
					isSensitive = true
					break
				}
			}
			if isSensitive {
				result[k] = "[REDACTED]"
			} else {
				result[k] = redactSensitiveFields(val)
			}
		}
		return result
	case []interface{}:
		result := make([]interface{}, len(v))
		for i, item := range v {
			result[i] = redactSensitiveFields(item)
		}
		return result
	default:
		return data
	}
}
