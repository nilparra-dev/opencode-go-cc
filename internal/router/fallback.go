// Package router provides fallback handling with circuit breaker pattern.
package router

import (
	"sync"
	"time"
)

// CircuitState represents the state of a circuit breaker.
type CircuitState int

const (
	CircuitClosed   CircuitState = iota // Normal operation
	CircuitOpen                          // Failing, skip requests
	CircuitHalfOpen                      // Testing if recovered
)

const (
	failureThreshold = 3
	openDuration     = 30 * time.Second
)

// CircuitBreaker tracks health per model.
type CircuitBreaker struct {
	mu            sync.RWMutex
	failures      map[string]int
	states        map[string]CircuitState
	lastFailure   map[string]time.Time
}

// NewCircuitBreaker creates a new circuit breaker.
func NewCircuitBreaker() *CircuitBreaker {
	return &CircuitBreaker{
		failures:    make(map[string]int),
		states:      make(map[string]CircuitState),
		lastFailure: make(map[string]time.Time),
	}
}

// CanTry returns true if the model can be used.
func (cb *CircuitBreaker) CanTry(modelID string) bool {
	cb.mu.RLock()
	defer cb.mu.RUnlock()

	state := cb.states[modelID]
	if state == CircuitClosed {
		return true
	}
	if state == CircuitOpen {
		if time.Since(cb.lastFailure[modelID]) > openDuration {
			return true // Allow one test request
		}
		return false
	}
	return true // Half-open allows one request
}

// RecordSuccess marks a successful request.
func (cb *CircuitBreaker) RecordSuccess(modelID string) {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	cb.failures[modelID] = 0
	cb.states[modelID] = CircuitClosed
}

// RecordFailure marks a failed request.
func (cb *CircuitBreaker) RecordFailure(modelID string) {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	cb.failures[modelID]++
	cb.lastFailure[modelID] = time.Now()

	if cb.failures[modelID] >= failureThreshold {
		cb.states[modelID] = CircuitOpen
	}
}
