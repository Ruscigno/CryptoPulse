package middleware

import (
	"bufio"
	"context"
	"fmt"
	"net"
	"net/http"
	"strings"
	"time"

	"go.uber.org/zap"
)

// Context key types for logging
type loggingContextKey string

const (
	loggerKey        loggingContextKey = "logger"
	requestLoggerKey loggingContextKey = "request_logger"
)

// LoggingConfig holds logging configuration
type LoggingConfig struct {
	Logger           *zap.Logger
	LogRequestBody   bool
	LogResponseBody  bool
	SensitiveHeaders []string // Headers to redact in logs
	SensitiveFields  []string // JSON fields to redact in logs
}

// responseWriter wraps http.ResponseWriter to capture response data
type responseWriter struct {
	http.ResponseWriter
	statusCode int
	size       int
	body       []byte
	logBody    bool
}

func newResponseWriter(w http.ResponseWriter, logBody bool) *responseWriter {
	return &responseWriter{
		ResponseWriter: w,
		statusCode:     http.StatusOK,
		logBody:        logBody,
	}
}

func (rw *responseWriter) WriteHeader(statusCode int) {
	rw.statusCode = statusCode
	rw.ResponseWriter.WriteHeader(statusCode)
}

func (rw *responseWriter) Write(data []byte) (int, error) {
	size, err := rw.ResponseWriter.Write(data)
	rw.size += size

	if rw.logBody {
		rw.body = append(rw.body, data...)
	}

	return size, err
}

// Hijack implements http.Hijacker interface
func (rw *responseWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	if hijacker, ok := rw.ResponseWriter.(http.Hijacker); ok {
		return hijacker.Hijack()
	}
	return nil, nil, fmt.Errorf("hijacking not supported")
}

// RequestLogging middleware logs HTTP requests and responses
func RequestLogging(config LoggingConfig) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()

			// Get request ID from context
			requestID := getRequestIDFromContext(r.Context())

			// Create wrapped response writer
			wrapped := newResponseWriter(w, config.LogResponseBody)

			// Log request
			logRequest(config, r, requestID)

			// Process request
			next.ServeHTTP(wrapped, r)

			// Calculate duration
			duration := time.Since(start)

			// Log response
			logResponse(config, r, wrapped, duration, requestID)
		})
	}
}

// logRequest logs the incoming HTTP request
func logRequest(config LoggingConfig, r *http.Request, requestID string) {
	fields := []zap.Field{
		zap.String("request_id", requestID),
		zap.String("method", r.Method),
		zap.String("path", r.URL.Path),
		zap.String("query", r.URL.RawQuery),
		zap.String("remote_addr", getClientIP(r)),
		zap.String("user_agent", r.UserAgent()),
		zap.String("referer", r.Referer()),
		zap.Int64("content_length", r.ContentLength),
	}

	// Add sanitized headers
	headers := make(map[string]string)
	for name, values := range r.Header {
		if len(values) > 0 {
			if isSensitiveHeader(name, config.SensitiveHeaders) {
				headers[name] = "[REDACTED]"
			} else {
				headers[name] = values[0]
			}
		}
	}
	fields = append(fields, zap.Any("headers", headers))

	config.Logger.Info("HTTP request", fields...)
}

// logResponse logs the HTTP response
func logResponse(config LoggingConfig, r *http.Request, rw *responseWriter, duration time.Duration, requestID string) {
	fields := []zap.Field{
		zap.String("request_id", requestID),
		zap.String("method", r.Method),
		zap.String("path", r.URL.Path),
		zap.Int("status_code", rw.statusCode),
		zap.Int("response_size", rw.size),
		zap.Duration("duration", duration),
		zap.Float64("duration_ms", float64(duration.Nanoseconds())/1e6),
	}

	// Add response body if logging is enabled and body is not too large
	if config.LogResponseBody && len(rw.body) > 0 && len(rw.body) < 10240 { // Max 10KB
		bodyStr := string(rw.body)
		if isJSONContent(rw.Header().Get("Content-Type")) {
			bodyStr = sanitizeJSONBody(bodyStr, config.SensitiveFields)
		}
		fields = append(fields, zap.String("response_body", bodyStr))
	}

	// Log with appropriate level based on status code
	if rw.statusCode >= 500 {
		config.Logger.Error("HTTP response", fields...)
	} else if rw.statusCode >= 400 {
		config.Logger.Warn("HTTP response", fields...)
	} else {
		config.Logger.Info("HTTP response", fields...)
	}
}

// StructuredLogging middleware adds structured logging context
func StructuredLogging(logger *zap.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Add logger to context
			ctx := context.WithValue(r.Context(), loggerKey, logger)

			// Add request metadata to logger
			requestLogger := logger.With(
				zap.String("method", r.Method),
				zap.String("path", r.URL.Path),
				zap.String("remote_addr", getClientIP(r)),
			)

			ctx = context.WithValue(ctx, requestLoggerKey, requestLogger)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// ErrorLogging middleware logs panics and errors
func ErrorLogging(logger *zap.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				if err := recover(); err != nil {
					requestID := getRequestIDFromContext(r.Context())

					logger.Error("HTTP handler panic",
						zap.String("request_id", requestID),
						zap.String("method", r.Method),
						zap.String("path", r.URL.Path),
						zap.String("remote_addr", getClientIP(r)),
						zap.Any("panic", err),
					)

					http.Error(w, "Internal Server Error", http.StatusInternalServerError)
				}
			}()

			next.ServeHTTP(w, r)
		})
	}
}

// getRequestIDFromContext extracts request ID from context
func getRequestIDFromContext(ctx context.Context) string {
	if requestID, ok := ctx.Value("request_id").(string); ok {
		return requestID
	}
	return "unknown"
}

// isSensitiveHeader checks if a header should be redacted
func isSensitiveHeader(headerName string, sensitiveHeaders []string) bool {
	headerLower := strings.ToLower(headerName)

	// Default sensitive headers
	defaultSensitive := []string{
		"authorization",
		"x-api-key",
		"cookie",
		"set-cookie",
		"x-auth-token",
		"x-access-token",
	}

	// Check default sensitive headers
	for _, sensitive := range defaultSensitive {
		if headerLower == sensitive {
			return true
		}
	}

	// Check custom sensitive headers
	for _, sensitive := range sensitiveHeaders {
		if strings.ToLower(sensitive) == headerLower {
			return true
		}
	}

	return false
}

// isJSONContent checks if content type is JSON
func isJSONContent(contentType string) bool {
	return strings.Contains(strings.ToLower(contentType), "application/json")
}

// sanitizeJSONBody redacts sensitive fields from JSON body
func sanitizeJSONBody(body string, sensitiveFields []string) string {
	// Simple implementation - in production, use proper JSON parsing
	result := body

	for _, field := range sensitiveFields {
		// Simple string replacement - in production, use proper JSON manipulation
		// This is a simplified approach - use regex in production
		if strings.Contains(result, fmt.Sprintf(`"%s":`, field)) {
			result = strings.ReplaceAll(result,
				fmt.Sprintf(`"%s":`, field),
				fmt.Sprintf(`"%s":"[REDACTED]"`, field))
		}
	}

	return result
}
