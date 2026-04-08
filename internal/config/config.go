package config

import "os"

//Everything we need to know

type Config struct {
	Port string
	JWTSecret string

	// Redis
    RedisAddr     string
    RedisPassword string

    // Rate limiting
    RateLimit     int 
    RateRefill    int 
}

func Load() *Config {
    return &Config{
        Port:          getEnv("PORT", "8080"),
        JWTSecret:     getEnv("JWT_SECRET", "change-me-in-production"),
        RedisAddr:     getEnv("REDIS_ADDR", "localhost:6379"),
        RedisPassword: getEnv("REDIS_PASSWORD", ""),
        RateLimit:     100, // 100 requests
        RateRefill:    60,  // 1 per second steady state
    }
}

func getEnv(key, fallback string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return fallback
}