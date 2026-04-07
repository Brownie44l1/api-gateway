package config

import "os"

//Everything we need to know/ use.

type Config struct {
	Port string
	JWTSecret string
}

func Load() *Config {
	return &Config{
		Port: getEnv("PORT", "8080"),
		JWTSecret: getEnv("JWT_SECRET", "change-me-in-production"),
	}
}

func getEnv(key, fallback string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return fallback
}