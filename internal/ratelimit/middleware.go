package ratelimit

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/Brownie44l1/api-gateway/internal/auth"
)

type RateLimitMiddleware struct {
	limiter            *RateLimiter
	distributedLimiter *DistributedRateLimiter
	useDistributed     bool
	strategy           string // "sliding-window" or "token-bucket"
}

func NewRateLimitMiddleware(limiter *RateLimiter) *RateLimitMiddleware {
	return &RateLimitMiddleware{
		limiter:        limiter,
		useDistributed: false,
	}
}

func NewDistributedRateLimitMiddleware(distributedLimiter *DistributedRateLimiter, strategy string) *RateLimitMiddleware {
	return &RateLimitMiddleware{
		distributedLimiter: distributedLimiter,
		useDistributed:     true,
		strategy:          strategy,
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

		var allowed bool
		var remaining, capacity int64
		var err error

		if rlm.useDistributed {
			// Use distributed rate limiting
			ctx, cancel := context.WithTimeout(r.Context(), 100*time.Millisecond)
			defer cancel()

			if rlm.strategy == "token-bucket" {
				allowed, err = rlm.distributedLimiter.AllowWithTokenBucket(ctx, apiKey)
				if err == nil {
					remaining, capacity, _ = rlm.distributedLimiter.GetStatsTokenBucket(ctx, apiKey)
				}
			} else {
				// Default: sliding-window
				allowed, err = rlm.distributedLimiter.Allow(ctx, apiKey)
				if err == nil {
					remaining, capacity, _ = rlm.distributedLimiter.GetStats(ctx, apiKey)
				}
			}

			if err != nil {
				// Redis error, fallback handled internally by DistributedRateLimiter
				allowed = rlm.distributedLimiter.fallback.Allow(apiKey)
				tokens, cap := rlm.distributedLimiter.fallback.GetStats(apiKey)
				remaining = tokens
				capacity = cap
			}
		} else {
			// Use in-memory rate limiting
			allowed = rlm.limiter.Allow(apiKey)
			tokens, cap := rlm.limiter.GetStats(apiKey)
			remaining = tokens
			capacity = cap
		}

		// Add rate limit headers
		w.Header().Set("X-RateLimit-Limit", fmt.Sprintf("%d", capacity))
		w.Header().Set("X-RateLimit-Remaining", fmt.Sprintf("%d", remaining))

		if !allowed {
			w.Header().Set("Retry-After", "60")
			http.Error(w, "Rate limit exceeded", http.StatusTooManyRequests)
			return
		}

		next(w, r)
	}
}