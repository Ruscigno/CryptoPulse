package middleware

import (
	"context"
	"crypto/subtle"
	"fmt"
	"net/http"
	"strings"
	"time"

	"go.uber.org/zap"
)

// Context key types to avoid collisions
type contextKey string

const (
	userIDKey        contextKey = "user_id"
	apiKeyKey        contextKey = "api_key"
	authenticatedKey contextKey = "authenticated"
	usernameKey      contextKey = "username"
	requestIDKey     contextKey = "request_id"
)

// AuthConfig holds authentication configuration
type AuthConfig struct {
	APIKey string
	Logger *zap.Logger
}

// APIKeyAuth middleware validates API key authentication
func APIKeyAuth(config AuthConfig) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Skip authentication for health check endpoints
			if strings.HasPrefix(r.URL.Path, "/health") {
				next.ServeHTTP(w, r)
				return
			}

			// Extract API key from header
			apiKey := r.Header.Get("X-API-Key")
			if apiKey == "" {
				// Try Authorization header with Bearer token
				authHeader := r.Header.Get("Authorization")
				if strings.HasPrefix(authHeader, "Bearer ") {
					apiKey = strings.TrimPrefix(authHeader, "Bearer ")
				}
			}

			if apiKey == "" {
				config.Logger.Warn("Missing API key",
					zap.String("path", r.URL.Path),
					zap.String("method", r.Method),
					zap.String("remote_addr", r.RemoteAddr))

				http.Error(w, "API key required", http.StatusUnauthorized)
				return
			}

			// Validate API key using constant-time comparison
			if subtle.ConstantTimeCompare([]byte(apiKey), []byte(config.APIKey)) != 1 {
				config.Logger.Warn("Invalid API key",
					zap.String("path", r.URL.Path),
					zap.String("method", r.Method),
					zap.String("remote_addr", r.RemoteAddr))

				http.Error(w, "Invalid API key", http.StatusUnauthorized)
				return
			}

			// Add authenticated context
			ctx := context.WithValue(r.Context(), authenticatedKey, true)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// BasicAuth middleware for simple username/password authentication
func BasicAuth(username, password string, logger *zap.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Skip authentication for health check endpoints
			if strings.HasPrefix(r.URL.Path, "/health") {
				next.ServeHTTP(w, r)
				return
			}

			user, pass, ok := r.BasicAuth()
			if !ok {
				logger.Warn("Missing basic auth credentials",
					zap.String("path", r.URL.Path),
					zap.String("remote_addr", r.RemoteAddr))

				w.Header().Set("WWW-Authenticate", `Basic realm="Restricted"`)
				http.Error(w, "Authentication required", http.StatusUnauthorized)
				return
			}

			// Use constant-time comparison to prevent timing attacks
			userValid := subtle.ConstantTimeCompare([]byte(user), []byte(username)) == 1
			passValid := subtle.ConstantTimeCompare([]byte(pass), []byte(password)) == 1

			if !userValid || !passValid {
				logger.Warn("Invalid basic auth credentials",
					zap.String("path", r.URL.Path),
					zap.String("user", user),
					zap.String("remote_addr", r.RemoteAddr))

				w.Header().Set("WWW-Authenticate", `Basic realm="Restricted"`)
				http.Error(w, "Invalid credentials", http.StatusUnauthorized)
				return
			}

			// Add authenticated context
			ctx := context.WithValue(r.Context(), authenticatedKey, true)
			ctx = context.WithValue(ctx, usernameKey, user)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// CORS middleware for handling cross-origin requests
func CORS(allowedOrigins []string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			origin := r.Header.Get("Origin")

			// Check if origin is allowed
			allowed := false
			for _, allowedOrigin := range allowedOrigins {
				if allowedOrigin == "*" || allowedOrigin == origin {
					allowed = true
					break
				}
			}

			if allowed {
				w.Header().Set("Access-Control-Allow-Origin", origin)
			}

			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-API-Key")
			w.Header().Set("Access-Control-Max-Age", "86400")

			// Handle preflight requests
			if r.Method == "OPTIONS" {
				w.WriteHeader(http.StatusOK)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// SecurityHeaders middleware adds security headers
func SecurityHeaders() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Security headers
			w.Header().Set("X-Content-Type-Options", "nosniff")
			w.Header().Set("X-Frame-Options", "DENY")
			w.Header().Set("X-XSS-Protection", "1; mode=block")
			w.Header().Set("Strict-Transport-Security", "max-age=31536000; includeSubDomains")
			w.Header().Set("Content-Security-Policy", "default-src 'self'")
			w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")

			next.ServeHTTP(w, r)
		})
	}
}

// RequestID middleware adds a unique request ID to each request
func RequestID() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			requestID := r.Header.Get("X-Request-ID")
			if requestID == "" {
				requestID = generateRequestID()
			}

			w.Header().Set("X-Request-ID", requestID)
			ctx := context.WithValue(r.Context(), requestIDKey, requestID)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// generateRequestID creates a simple request ID
func generateRequestID() string {
	// Simple implementation - in production, use a proper UUID library
	return fmt.Sprintf("req_%d", getCurrentTimestamp())
}

// getCurrentTimestamp returns current Unix timestamp in nanoseconds
func getCurrentTimestamp() int64 {
	return time.Now().UnixNano()
}
