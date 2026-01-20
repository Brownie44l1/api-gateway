package ratelimit

import (
	"context"
	"fmt"
	"time"

	redisclient "github.com/Brownie44l1/api-gateway/internal/redis"
)

// DistributedRateLimiter implements rate limiting using Redis
type DistributedRateLimiter struct {
	redis             *redisclient.Client
	requestsPerMinute int
	burstSize         int
	windowSize        time.Duration
	
	// Fallback to in-memory if Redis fails
	fallback *RateLimiter
}

// NewDistributedRateLimiter creates a new distributed rate limiter
func NewDistributedRateLimiter(redis *redisclient.Client, requestsPerMinute, burstSize int) *DistributedRateLimiter {
	return &DistributedRateLimiter{
		redis:             redis,
		requestsPerMinute: requestsPerMinute,
		burstSize:         burstSize,
		windowSize:        time.Minute,
		fallback:          NewRateLimiter(requestsPerMinute, burstSize),
	}
}

// Allow checks if a request is allowed using Redis-based rate limiting
func (drl *DistributedRateLimiter) Allow(ctx context.Context, key string) (bool, error) {
	// Try Redis first
	allowed, err := drl.allowRedis(ctx, key)
	if err != nil {
		// Fallback to in-memory on Redis error
		return drl.fallback.Allow(key), nil
	}
	return allowed, nil
}

// allowRedis implements sliding window rate limiting in Redis
func (drl *DistributedRateLimiter) allowRedis(ctx context.Context, key string) (bool, error) {
	// Use sliding window approach
	// Key: ratelimit:{api_key}:{current_minute}
	now := time.Now()
	currentWindow := now.Truncate(drl.windowSize).Unix()
	redisKey := fmt.Sprintf("ratelimit:%s:%d", key, currentWindow)

	// Lua script for atomic increment and TTL check
	// This ensures we don't have race conditions
	script := `
		local key = KEYS[1]
		local limit = tonumber(ARGV[1])
		local ttl = tonumber(ARGV[2])
		
		local current = redis.call('GET', key)
		if current == false then
			current = 0
		else
			current = tonumber(current)
		end
		
		if current < limit then
			redis.call('INCR', key)
			if current == 0 then
				redis.call('EXPIRE', key, ttl)
			end
			return 1
		else
			return 0
		end
	`

	result, err := drl.redis.Eval(ctx, script, []string{redisKey}, drl.requestsPerMinute, int(drl.windowSize.Seconds()))
	if err != nil {
		return false, err
	}

	allowed := result.(int64) == 1
	return allowed, nil
}

// AllowWithTokenBucket implements token bucket algorithm in Redis (more sophisticated)
func (drl *DistributedRateLimiter) AllowWithTokenBucket(ctx context.Context, key string) (bool, error) {
	redisKey := fmt.Sprintf("ratelimit:bucket:%s", key)
	
	// Lua script for token bucket algorithm
	script := `
		local key = KEYS[1]
		local capacity = tonumber(ARGV[1])
		local refill_rate = tonumber(ARGV[2])
		local now = tonumber(ARGV[3])
		
		local bucket = redis.call('HMGET', key, 'tokens', 'last_refill')
		local tokens = tonumber(bucket[1])
		local last_refill = tonumber(bucket[2])
		
		if tokens == nil then
			tokens = capacity
			last_refill = now
		else
			-- Calculate tokens to add based on time elapsed
			local elapsed = now - last_refill
			local tokens_to_add = elapsed * refill_rate
			tokens = math.min(capacity, tokens + tokens_to_add)
			last_refill = now
		end
		
		if tokens >= 1 then
			tokens = tokens - 1
			redis.call('HMSET', key, 'tokens', tokens, 'last_refill', last_refill)
			redis.call('EXPIRE', key, 3600) -- 1 hour TTL for cleanup
			return {1, tokens, capacity}
		else
			return {0, tokens, capacity}
		end
	`

	refillRate := float64(drl.requestsPerMinute) / 60.0 // per second
	now := float64(time.Now().Unix())

	result, err := drl.redis.Eval(ctx, script, []string{redisKey}, drl.burstSize, refillRate, now)
	if err != nil {
		return false, err
	}

	resultSlice := result.([]interface{})
	allowed := resultSlice[0].(int64) == 1

	return allowed, nil
}

// GetStats returns current rate limit stats
func (drl *DistributedRateLimiter) GetStats(ctx context.Context, key string) (remaining int64, limit int64, err error) {
	now := time.Now()
	currentWindow := now.Truncate(drl.windowSize).Unix()
	redisKey := fmt.Sprintf("ratelimit:%s:%d", key, currentWindow)

	countStr, err := drl.redis.Get(ctx, redisKey)
	if err != nil {
		// Key doesn't exist, full quota available
		return int64(drl.requestsPerMinute), int64(drl.requestsPerMinute), nil
	}

	var count int64
	fmt.Sscanf(countStr, "%d", &count)
	
	remaining = int64(drl.requestsPerMinute) - count
	if remaining < 0 {
		remaining = 0
	}

	return remaining, int64(drl.requestsPerMinute), nil
}

// GetStatsTokenBucket returns token bucket stats
func (drl *DistributedRateLimiter) GetStatsTokenBucket(ctx context.Context, key string) (tokens int64, capacity int64, err error) {
	redisKey := fmt.Sprintf("ratelimit:bucket:%s", key)
	
	script := `
		local key = KEYS[1]
		local capacity = tonumber(ARGV[1])
		local refill_rate = tonumber(ARGV[2])
		local now = tonumber(ARGV[3])
		
		local bucket = redis.call('HMGET', key, 'tokens', 'last_refill')
		local tokens = tonumber(bucket[1])
		local last_refill = tonumber(bucket[2])
		
		if tokens == nil then
			return {capacity, capacity}
		end
		
		local elapsed = now - last_refill
		local tokens_to_add = elapsed * refill_rate
		tokens = math.min(capacity, tokens + tokens_to_add)
		
		return {math.floor(tokens), capacity}
	`

	refillRate := float64(drl.requestsPerMinute) / 60.0
	now := float64(time.Now().Unix())

	result, err := drl.redis.Eval(ctx, script, []string{redisKey}, drl.burstSize, refillRate, now)
	if err != nil {
		return int64(drl.burstSize), int64(drl.burstSize), nil
	}

	resultSlice := result.([]interface{})
	tokens = resultSlice[0].(int64)
	capacity = resultSlice[1].(int64)

	return tokens, capacity, nil
}

// Reset clears rate limit for a key (useful for testing or admin)
func (drl *DistributedRateLimiter) Reset(ctx context.Context, key string) error {
	pattern := fmt.Sprintf("ratelimit:%s:*", key)
	
	// Note: In production, use SCAN instead of KEYS for better performance
	iter := drl.redis.GetClient().Scan(ctx, 0, pattern, 0).Iterator()
	for iter.Next(ctx) {
		if err := drl.redis.GetClient().Del(ctx, iter.Val()).Err(); err != nil {
			return err
		}
	}
	return iter.Err()
}

// HealthCheck verifies Redis connectivity
func (drl *DistributedRateLimiter) HealthCheck(ctx context.Context) error {
	return drl.redis.Ping(ctx)
}