package ratelimit

import (
	"fmt"
	"net/http"

	"github.com/Brownie44l1/api-gateway/internal/auth"
)

type RateLimitMiddleware struct {
	limiter *RateLimiter
}

func NewRateLimitMiddleware(limiter *RateLimiter) *RateLimitMiddleware {
	return &RateLimitMiddleware{
		limiter: limiter,
	}
}

// Middleware enforces rate limits per API key
func (rlm *RateLimitMiddleware) Middleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Get API key from context (set by auth middleware)
		apiKey, ok := auth.GetAPIKeyFromContext(r.Context())
		if !ok {
			http.Error(w, "Internal error: API key not found in context", http.StatusInternalServerError)
			return
		}

		// Check rate limit
		if !rlm.limiter.Allow(apiKey) {
			_, capacity := rlm.limiter.GetStats(apiKey)
			w.Header().Set("X-RateLimit-Limit", fmt.Sprintf("%d", capacity))
			w.Header().Set("X-RateLimit-Remaining", "0")
			w.Header().Set("Retry-After", "60")
			http.Error(w, "Rate limit exceeded", http.StatusTooManyRequests)
			return
		}

		// Add rate limit headers
		tokens, capacity := rlm.limiter.GetStats(apiKey)
		w.Header().Set("X-RateLimit-Limit", fmt.Sprintf("%d", capacity))
		w.Header().Set("X-RateLimit-Remaining", fmt.Sprintf("%d", tokens))

		next(w, r)
	}
}