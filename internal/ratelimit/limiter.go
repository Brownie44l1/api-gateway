package ratelimit

import (
	"sync"
	"time"
)

// TokenBucket implements the token bucket algorithm for rate limiting
type TokenBucket struct {
	capacity int64
	tokens int64
	refillRate int64 // tokens per second
	lastRefill time.Time
	mu sync.Mutex
}

func NewTokenBucket(capacity, refillRate int64) *TokenBucket {
	return &TokenBucket{
		capacity: capacity,
		tokens: capacity,
		refillRate: refillRate,
		lastRefill: time.Now(),
	}
}

// Allow checks if a request can proceed based on available tokens
func (tb *TokenBucket) Allow() bool {
	tb.mu.Lock()
	defer tb.mu.Unlock()

	tb.refill()

	if tb.tokens > 0 {
		tb.tokens--
		return true
	}

	return false
}

// refill adds tokens based on elapsed time
func (tb *TokenBucket) refill() {
	now := time.Now()
	elapsed := now.Sub(tb.lastRefill)
	
	tokensToAdd := int64(elapsed.Seconds()) * tb.refillRate
	
	if tokensToAdd > 0 {
		tb.tokens += tokensToAdd
		if tb.tokens > tb.capacity {
			tb.tokens = tb.capacity
		}
		tb.lastRefill = now
	}
}

// RateLimiter manages rate limits per API key
type RateLimiter struct {
	mu sync.RWMutex
	buckets map[string]*TokenBucket
	capacity int64
	refillRate int64
}

func NewRateLimiter(requestsPerMinute, burstSize int) *RateLimiter {
	return &RateLimiter{
		buckets: make(map[string]*TokenBucket),
		capacity: int64(burstSize),
		refillRate: int64(requestsPerMinute) / 60, // convert to per-second
	}
}

// Allow checks if a request from the given key is allowed
func (rl *RateLimiter) Allow(key string) bool {
	rl.mu.RLock()
	bucket, exists := rl.buckets[key]
	rl.mu.RUnlock()

	if !exists {
		rl.mu.Lock()
		// Double-check after acquiring write lock
		bucket, exists = rl.buckets[key]
		if !exists {
			bucket = NewTokenBucket(rl.capacity, rl.refillRate)
			rl.buckets[key] = bucket
		}
		rl.mu.Unlock()
	}

	return bucket.Allow()
}

// GetStats returns current token count for a key (useful for debugging)
func (rl *RateLimiter) GetStats(key string) (tokens int64, capacity int64) {
	rl.mu.RLock()
	bucket, exists := rl.buckets[key]
	rl.mu.RUnlock()

	if !exists {
		return 0, rl.capacity
	}

	bucket.mu.Lock()
	bucket.refill()
	tokens = bucket.tokens
	capacity = bucket.capacity
	bucket.mu.Unlock()

	return tokens, capacity
}

// Cleanup removes rate limiters for keys that haven't been used recently
func (rl *RateLimiter) Cleanup(maxAge time.Duration) {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()
	for key, bucket := range rl.buckets {
		bucket.mu.Lock()
		if now.Sub(bucket.lastRefill) > maxAge {
			delete(rl.buckets, key)
		}
		bucket.mu.Unlock()
	}
}