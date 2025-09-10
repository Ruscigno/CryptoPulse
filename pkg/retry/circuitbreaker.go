package retry

import (
	"context"
	"fmt"
	"sync"
	"time"

	"go.uber.org/zap"
)

// CircuitBreakerState represents the state of the circuit breaker
type CircuitBreakerState int

const (
	// StateClosed - circuit breaker is closed, requests are allowed
	StateClosed CircuitBreakerState = iota
	// StateOpen - circuit breaker is open, requests are rejected
	StateOpen
	// StateHalfOpen - circuit breaker is half-open, limited requests are allowed
	StateHalfOpen
)

func (s CircuitBreakerState) String() string {
	switch s {
	case StateClosed:
		return "CLOSED"
	case StateOpen:
		return "OPEN"
	case StateHalfOpen:
		return "HALF_OPEN"
	default:
		return "UNKNOWN"
	}
}

// CircuitBreakerConfig holds circuit breaker configuration
type CircuitBreakerConfig struct {
	Name                string        // Name for logging and metrics
	MaxFailures         int           // Maximum failures before opening
	ResetTimeout        time.Duration // Time to wait before transitioning to half-open
	SuccessThreshold    int           // Successful requests needed to close from half-open
	Timeout             time.Duration // Request timeout
	OnStateChange       func(from, to CircuitBreakerState)
	ShouldTrip          func(counts Counts) bool
	Logger              *zap.Logger
}

// DefaultCircuitBreakerConfig returns a default circuit breaker configuration
func DefaultCircuitBreakerConfig(name string) CircuitBreakerConfig {
	return CircuitBreakerConfig{
		Name:             name,
		MaxFailures:      5,
		ResetTimeout:     60 * time.Second,
		SuccessThreshold: 3,
		Timeout:          30 * time.Second,
		Logger:           zap.NewNop(),
	}
}

// Counts holds the statistics for the circuit breaker
type Counts struct {
	Requests             int
	TotalSuccesses       int
	TotalFailures        int
	ConsecutiveSuccesses int
	ConsecutiveFailures  int
}

// CircuitBreaker implements the circuit breaker pattern
type CircuitBreaker struct {
	config CircuitBreakerConfig
	mutex  sync.RWMutex
	state  CircuitBreakerState
	counts Counts
	expiry time.Time
}

// NewCircuitBreaker creates a new circuit breaker
func NewCircuitBreaker(config CircuitBreakerConfig) *CircuitBreaker {
	cb := &CircuitBreaker{
		config: config,
		state:  StateClosed,
		expiry: time.Now(),
	}

	if cb.config.ShouldTrip == nil {
		cb.config.ShouldTrip = cb.defaultShouldTrip
	}

	return cb
}

// Execute executes a function with circuit breaker protection
func (cb *CircuitBreaker) Execute(ctx context.Context, fn func() error) error {
	// Check if circuit breaker allows the request
	if !cb.allowRequest() {
		return fmt.Errorf("circuit breaker %s is OPEN", cb.config.Name)
	}

	// Create a context with timeout
	if cb.config.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, cb.config.Timeout)
		defer cancel()
	}

	// Execute the function
	err := cb.executeWithTimeout(ctx, fn)
	
	// Record the result
	cb.recordResult(err == nil)
	
	return err
}

// ExecuteWithResult executes a function that returns a result with circuit breaker protection
func (cb *CircuitBreaker) ExecuteWithResult[T any](ctx context.Context, fn func() (T, error)) (T, error) {
	var zeroValue T
	
	// Check if circuit breaker allows the request
	if !cb.allowRequest() {
		return zeroValue, fmt.Errorf("circuit breaker %s is OPEN", cb.config.Name)
	}

	// Create a context with timeout
	if cb.config.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, cb.config.Timeout)
		defer cancel()
	}

	// Execute the function
	result, err := cb.executeWithTimeoutAndResult(ctx, fn)
	
	// Record the result
	cb.recordResult(err == nil)
	
	return result, err
}

// allowRequest checks if the circuit breaker allows the request
func (cb *CircuitBreaker) allowRequest() bool {
	cb.mutex.Lock()
	defer cb.mutex.Unlock()

	now := time.Now()

	switch cb.state {
	case StateClosed:
		return true
	case StateOpen:
		if now.After(cb.expiry) {
			cb.setState(StateHalfOpen)
			cb.resetCounts()
			return true
		}
		return false
	case StateHalfOpen:
		return true
	default:
		return false
	}
}

// recordResult records the result of a request
func (cb *CircuitBreaker) recordResult(success bool) {
	cb.mutex.Lock()
	defer cb.mutex.Unlock()

	cb.counts.Requests++

	if success {
		cb.counts.TotalSuccesses++
		cb.counts.ConsecutiveSuccesses++
		cb.counts.ConsecutiveFailures = 0

		if cb.state == StateHalfOpen && cb.counts.ConsecutiveSuccesses >= cb.config.SuccessThreshold {
			cb.setState(StateClosed)
			cb.resetCounts()
		}
	} else {
		cb.counts.TotalFailures++
		cb.counts.ConsecutiveFailures++
		cb.counts.ConsecutiveSuccesses = 0

		if cb.config.ShouldTrip(cb.counts) {
			cb.setState(StateOpen)
			cb.expiry = time.Now().Add(cb.config.ResetTimeout)
		}
	}
}

// setState changes the circuit breaker state
func (cb *CircuitBreaker) setState(newState CircuitBreakerState) {
	if cb.state == newState {
		return
	}

	oldState := cb.state
	cb.state = newState

	cb.config.Logger.Info("Circuit breaker state changed",
		zap.String("name", cb.config.Name),
		zap.String("from", oldState.String()),
		zap.String("to", newState.String()),
		zap.Any("counts", cb.counts))

	if cb.config.OnStateChange != nil {
		cb.config.OnStateChange(oldState, newState)
	}
}

// resetCounts resets the circuit breaker counts
func (cb *CircuitBreaker) resetCounts() {
	cb.counts = Counts{}
}

// defaultShouldTrip is the default function to determine if the circuit should trip
func (cb *CircuitBreaker) defaultShouldTrip(counts Counts) bool {
	return counts.ConsecutiveFailures >= cb.config.MaxFailures
}

// executeWithTimeout executes a function with timeout handling
func (cb *CircuitBreaker) executeWithTimeout(ctx context.Context, fn func() error) error {
	done := make(chan error, 1)
	
	go func() {
		done <- fn()
	}()

	select {
	case err := <-done:
		return err
	case <-ctx.Done():
		return ctx.Err()
	}
}

// executeWithTimeoutAndResult executes a function that returns a result with timeout handling
func (cb *CircuitBreaker) executeWithTimeoutAndResult[T any](ctx context.Context, fn func() (T, error)) (T, error) {
	type result struct {
		value T
		err   error
	}
	
	done := make(chan result, 1)
	
	go func() {
		value, err := fn()
		done <- result{value: value, err: err}
	}()

	select {
	case res := <-done:
		return res.value, res.err
	case <-ctx.Done():
		var zeroValue T
		return zeroValue, ctx.Err()
	}
}

// GetState returns the current state of the circuit breaker
func (cb *CircuitBreaker) GetState() CircuitBreakerState {
	cb.mutex.RLock()
	defer cb.mutex.RUnlock()
	return cb.state
}

// GetCounts returns the current counts of the circuit breaker
func (cb *CircuitBreaker) GetCounts() Counts {
	cb.mutex.RLock()
	defer cb.mutex.RUnlock()
	return cb.counts
}

// Reset manually resets the circuit breaker to closed state
func (cb *CircuitBreaker) Reset() {
	cb.mutex.Lock()
	defer cb.mutex.Unlock()
	
	cb.setState(StateClosed)
	cb.resetCounts()
	cb.expiry = time.Now()
}

// CircuitBreakerManager manages multiple circuit breakers
type CircuitBreakerManager struct {
	breakers map[string]*CircuitBreaker
	mutex    sync.RWMutex
}

// NewCircuitBreakerManager creates a new circuit breaker manager
func NewCircuitBreakerManager() *CircuitBreakerManager {
	return &CircuitBreakerManager{
		breakers: make(map[string]*CircuitBreaker),
	}
}

// GetOrCreate gets an existing circuit breaker or creates a new one
func (cbm *CircuitBreakerManager) GetOrCreate(name string, config CircuitBreakerConfig) *CircuitBreaker {
	cbm.mutex.RLock()
	if cb, exists := cbm.breakers[name]; exists {
		cbm.mutex.RUnlock()
		return cb
	}
	cbm.mutex.RUnlock()

	cbm.mutex.Lock()
	defer cbm.mutex.Unlock()

	// Double-check after acquiring write lock
	if cb, exists := cbm.breakers[name]; exists {
		return cb
	}

	config.Name = name
	cb := NewCircuitBreaker(config)
	cbm.breakers[name] = cb
	return cb
}

// Get gets an existing circuit breaker
func (cbm *CircuitBreakerManager) Get(name string) (*CircuitBreaker, bool) {
	cbm.mutex.RLock()
	defer cbm.mutex.RUnlock()
	
	cb, exists := cbm.breakers[name]
	return cb, exists
}

// Remove removes a circuit breaker
func (cbm *CircuitBreakerManager) Remove(name string) {
	cbm.mutex.Lock()
	defer cbm.mutex.Unlock()
	
	delete(cbm.breakers, name)
}

// GetAll returns all circuit breakers
func (cbm *CircuitBreakerManager) GetAll() map[string]*CircuitBreaker {
	cbm.mutex.RLock()
	defer cbm.mutex.RUnlock()
	
	result := make(map[string]*CircuitBreaker)
	for name, cb := range cbm.breakers {
		result[name] = cb
	}
	return result
}
