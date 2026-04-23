package config

import (
	"os"
	"strconv"
	"time"
)

// Config holds everything the gateway needs to run.
// Every field is sourced from an environment variable with a safe default.
type Config struct {
	Port string

	// Auth
	JWTSecret string
	JWTExpiry time.Duration // how long tokens live

	// Redis
	RedisAddr     string
	RedisPassword string

	// Rate limiting — authenticated users
	RateLimit  int
	RateRefill int

	// Rate limiting — public/IP routes (stricter)
	IPRateLimit  int
	IPRateRefill int

	// Body size
	MaxBodyBytes int64 // bytes

	// Proxy / circuit breaker
	CBMaxFailures  int           // failures before opening
	CBResetTimeout time.Duration // how long before half-open retry

	// Upstream routes — comma-separated "prefix:upstream:timeout_seconds"
	// e.g. /api/users:http://localhost:3001:5,/api/posts:http://localhost:3002:5
	RoutesRaw string
}

func Load() *Config {
	return &Config{
		Port: getEnv("PORT", "8080"),

		JWTSecret: getEnv("JWT_SECRET", "change-me-in-production"),
		JWTExpiry: getDuration("JWT_EXPIRY", 15*time.Minute),

		RedisAddr:     getEnv("REDIS_ADDR", "localhost:6379"),
		RedisPassword: getEnv("REDIS_PASSWORD", ""),

		RateLimit:  getInt("RATE_LIMIT", 100),
		RateRefill: getInt("RATE_REFILL", 60),

		IPRateLimit:  getInt("IP_RATE_LIMIT", 20),
		IPRateRefill: getInt("IP_RATE_REFILL", 20),

		MaxBodyBytes: getInt64("MAX_BODY_BYTES", 1<<20), // 1MB default

		CBMaxFailures:  getInt("CB_MAX_FAILURES", 5),
		CBResetTimeout: getDuration("CB_RESET_TIMEOUT", 30*time.Second),

		RoutesRaw: getEnv("ROUTES", "/api/users:http://localhost:3001:5,/api/posts:http://localhost:3002:5"),
	}
}

func getEnv(key, fallback string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return fallback
}

func getInt(key string, fallback int) int {
	if val := os.Getenv(key); val != "" {
		if n, err := strconv.Atoi(val); err == nil {
			return n
		}
	}
	return fallback
}

func getInt64(key string, fallback int64) int64 {
	if val := os.Getenv(key); val != "" {
		if n, err := strconv.ParseInt(val, 10, 64); err == nil {
			return n
		}
	}
	return fallback
}

func getDuration(key string, fallback time.Duration) time.Duration {
	if val := os.Getenv(key); val != "" {
		if d, err := time.ParseDuration(val); err == nil {
			return d
		}
	}
	return fallback
}
