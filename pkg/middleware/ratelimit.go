package middleware

import (
	"net"
	"net/http"
	"strconv"
	"sync"
	"time"

	"go.uber.org/zap"
)

// RateLimiter interface for different rate limiting strategies
type RateLimiter interface {
	Allow(key string) bool
	Reset(key string)
}

// TokenBucketLimiter implements token bucket rate limiting
type TokenBucketLimiter struct {
	mu              sync.RWMutex
	buckets         map[string]*TokenBucket
	rate            int           // tokens per second
	capacity        int           // bucket capacity
	cleanupInterval time.Duration // cleanup interval
	logger          *zap.Logger
}

// TokenBucket represents a single token bucket
type TokenBucket struct {
	tokens     int
	capacity   int
	rate       int
	lastRefill time.Time
	mu         sync.Mutex
}

// NewTokenBucketLimiter creates a new token bucket rate limiter
func NewTokenBucketLimiter(rate, capacity int, logger *zap.Logger) *TokenBucketLimiter {
	limiter := &TokenBucketLimiter{
		buckets:         make(map[string]*TokenBucket),
		rate:            rate,
		capacity:        capacity,
		cleanupInterval: 5 * time.Minute,
		logger:          logger,
	}

	// Start cleanup goroutine
	go limiter.cleanup()

	return limiter
}

// Allow checks if a request is allowed for the given key
func (tbl *TokenBucketLimiter) Allow(key string) bool {
	tbl.mu.RLock()
	bucket, exists := tbl.buckets[key]
	tbl.mu.RUnlock()

	if !exists {
		tbl.mu.Lock()
		// Double-check after acquiring write lock
		if bucket, exists = tbl.buckets[key]; !exists {
			bucket = &TokenBucket{
				tokens:     tbl.capacity,
				capacity:   tbl.capacity,
				rate:       tbl.rate,
				lastRefill: time.Now(),
			}
			tbl.buckets[key] = bucket
		}
		tbl.mu.Unlock()
	}

	return bucket.consume()
}

// Reset removes the bucket for the given key
func (tbl *TokenBucketLimiter) Reset(key string) {
	tbl.mu.Lock()
	delete(tbl.buckets, key)
	tbl.mu.Unlock()
}

// consume attempts to consume a token from the bucket
func (tb *TokenBucket) consume() bool {
	tb.mu.Lock()
	defer tb.mu.Unlock()

	now := time.Now()
	elapsed := now.Sub(tb.lastRefill)

	// Refill tokens based on elapsed time
	tokensToAdd := int(elapsed.Seconds()) * tb.rate
	if tokensToAdd > 0 {
		tb.tokens = min(tb.capacity, tb.tokens+tokensToAdd)
		tb.lastRefill = now
	}

	// Check if we can consume a token
	if tb.tokens > 0 {
		tb.tokens--
		return true
	}

	return false
}

// cleanup removes old buckets periodically
func (tbl *TokenBucketLimiter) cleanup() {
	ticker := time.NewTicker(tbl.cleanupInterval)
	defer ticker.Stop()

	for range ticker.C {
		tbl.mu.Lock()
		now := time.Now()
		for key, bucket := range tbl.buckets {
			bucket.mu.Lock()
			if now.Sub(bucket.lastRefill) > tbl.cleanupInterval {
				delete(tbl.buckets, key)
			}
			bucket.mu.Unlock()
		}
		tbl.mu.Unlock()
	}
}

// RateLimitConfig holds rate limiting configuration
type RateLimitConfig struct {
	RequestsPerSecond int
	BurstSize         int
	Logger            *zap.Logger
}

// RateLimit middleware implements rate limiting
func RateLimit(config RateLimitConfig) func(http.Handler) http.Handler {
	limiter := NewTokenBucketLimiter(config.RequestsPerSecond, config.BurstSize, config.Logger)

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Get client IP for rate limiting key
			clientIP := getClientIP(r)

			if !limiter.Allow(clientIP) {
				config.Logger.Warn("Rate limit exceeded",
					zap.String("client_ip", clientIP),
					zap.String("path", r.URL.Path),
					zap.String("method", r.Method))

				// Set rate limit headers
				w.Header().Set("X-RateLimit-Limit", strconv.Itoa(config.RequestsPerSecond))
				w.Header().Set("X-RateLimit-Remaining", "0")
				w.Header().Set("X-RateLimit-Reset", strconv.FormatInt(time.Now().Add(time.Second).Unix(), 10))

				http.Error(w, "Rate limit exceeded", http.StatusTooManyRequests)
				return
			}

			// Set rate limit headers for successful requests
			w.Header().Set("X-RateLimit-Limit", strconv.Itoa(config.RequestsPerSecond))
			// Note: In a real implementation, you'd track remaining tokens
			w.Header().Set("X-RateLimit-Remaining", strconv.Itoa(config.BurstSize-1))

			next.ServeHTTP(w, r)
		})
	}
}

// getClientIP extracts the client IP address from the request
func getClientIP(r *http.Request) string {
	// Check X-Forwarded-For header first (for proxies)
	xff := r.Header.Get("X-Forwarded-For")
	if xff != "" {
		// Take the first IP in the list
		ips := parseXForwardedFor(xff)
		if len(ips) > 0 {
			return ips[0]
		}
	}

	// Check X-Real-IP header
	xri := r.Header.Get("X-Real-IP")
	if xri != "" {
		return xri
	}

	// Fall back to RemoteAddr
	ip, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return ip
}

// parseXForwardedFor parses the X-Forwarded-For header
func parseXForwardedFor(xff string) []string {
	var ips []string
	for _, ip := range splitAndTrim(xff, ",") {
		if net.ParseIP(ip) != nil {
			ips = append(ips, ip)
		}
	}
	return ips
}

// splitAndTrim splits a string by delimiter and trims whitespace
func splitAndTrim(s, delimiter string) []string {
	var result []string
	for _, part := range splitString(s, delimiter) {
		trimmed := trimWhitespace(part)
		if trimmed != "" {
			result = append(result, trimmed)
		}
	}
	return result
}

// splitString splits a string by delimiter
func splitString(s, delimiter string) []string {
	// Simple implementation - in production, use strings.Split
	var result []string
	current := ""
	for _, char := range s {
		if string(char) == delimiter {
			result = append(result, current)
			current = ""
		} else {
			current += string(char)
		}
	}
	if current != "" {
		result = append(result, current)
	}
	return result
}

// trimWhitespace removes leading and trailing whitespace
func trimWhitespace(s string) string {
	// Simple implementation - in production, use strings.TrimSpace
	start := 0
	end := len(s)

	for start < end && (s[start] == ' ' || s[start] == '\t' || s[start] == '\n' || s[start] == '\r') {
		start++
	}

	for end > start && (s[end-1] == ' ' || s[end-1] == '\t' || s[end-1] == '\n' || s[end-1] == '\r') {
		end--
	}

	return s[start:end]
}

// min returns the minimum of two integers
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
