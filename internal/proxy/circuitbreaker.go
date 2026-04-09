package proxy

import (
	"sync"
	"time"
)

type state int

const (
	closed state = iota
	open
	halfOpen
)

type CircuitBreaker struct {
	mu           sync.Mutex
	state        state
	failures     int
	maxFailures  int
	resetTimeout time.Duration
	lastFailure  time.Time
}

func NewCircuitBreaker(maxFailures int, resetTimeout time.Duration) *CircuitBreaker {
    return &CircuitBreaker{
        maxFailures:  maxFailures,
        resetTimeout: resetTimeout,
        state:        closed,
    }
}

func (cb *CircuitBreaker) Allow() bool {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	switch cb.state {
	case closed:
		return true
	case open:
		if time.Since(cb.lastFailure) > cb.resetTimeout {
			cb.state = halfOpen
			return true
		} 
		return false
	case halfOpen:
		return false
	}
	return false
}

// Success is called when the upstream responds successfully.
func (cb *CircuitBreaker) Success() {
    cb.mu.Lock()
    defer cb.mu.Unlock()

    // service recovered — reset everything
    cb.failures = 0
    cb.state = closed
}

// Failure is called when the upstream fails or times out.
func (cb *CircuitBreaker) Failure() {
    cb.mu.Lock()
    defer cb.mu.Unlock()

    cb.failures++
    cb.lastFailure = time.Now()

    if cb.failures >= cb.maxFailures {
        cb.state = open
    }
}