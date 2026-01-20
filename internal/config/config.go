package config

import (
	"time"
)

type Config struct {
	Gateway   GatewayConfig
	Services  []ServiceConfig
	RateLimit RateLimitConfig
	Redis     RedisConfig
	JWT       JWTConfig
	Auth      AuthConfig
}

type GatewayConfig struct {
	Host string
	Port int
}

type ServiceConfig struct {
	Name        string
	PathPrefix  string
	UpstreamURL string
	Timeout     time.Duration
}

type RateLimitConfig struct {
	RequestsPerMinute int
	BurstSize         int
	UseDistributed    bool
	Strategy          string
}

type RedisConfig struct {
	Enabled  bool
	Host     string
	Port     int
	Password string
	DB       int
}

type JWTConfig struct {
	Enabled         bool
	SecretKey       string
	Issuer          string
	AccessTokenTTL  time.Duration
	RefreshTokenTTL time.Duration
}

type AuthConfig struct {
	// Which auth methods to support
	AllowAPIKeys bool
	AllowJWT     bool
	// If both enabled, which takes precedence
	PreferJWT bool
}

func Default() *Config {
	return &Config{
		Gateway: GatewayConfig{
			Host: "localhost",
			Port: 8080,
		},
		Services: []ServiceConfig{
			{
				Name:        "test-backend",
				PathPrefix:  "/api/test",
				UpstreamURL: "http://localhost:8081",
				Timeout:     30 * time.Second,
			},
			{
				Name:        "test-backend-root",
				PathPrefix:  "/",
				UpstreamURL: "http://localhost:8081",
				Timeout:     30 * time.Second,
			},
		},
		RateLimit: RateLimitConfig{
			RequestsPerMinute: 60,
			BurstSize:         10,
			UseDistributed:    false,
			Strategy:          "sliding-window",
		},
		Redis: RedisConfig{
			Enabled:  false,
			Host:     "localhost",
			Port:     6379,
			Password: "",
			DB:       0,
		},
		JWT: JWTConfig{
			Enabled:         false,
			SecretKey:       "your-256-bit-secret-change-this-in-production",
			Issuer:          "api-gateway",
			AccessTokenTTL:  15 * time.Minute,
			RefreshTokenTTL: 7 * 24 * time.Hour, // 7 days
		},
		Auth: AuthConfig{
			AllowAPIKeys: true,
			AllowJWT:     false,
			PreferJWT:    false,
		},
	}
}

// WithRedis returns config with Redis enabled
func WithRedis() *Config {
	cfg := Default()
	cfg.Redis.Enabled = true
	cfg.RateLimit.UseDistributed = true
	return cfg
}

// WithJWT returns config with JWT enabled
func WithJWT() *Config {
	cfg := Default()
	cfg.JWT.Enabled = true
	cfg.Auth.AllowJWT = true
	return cfg
}

// WithBothAuth returns config with both API keys and JWT
func WithBothAuth() *Config {
	cfg := Default()
	cfg.JWT.Enabled = true
	cfg.Auth.AllowAPIKeys = true
	cfg.Auth.AllowJWT = true
	cfg.Auth.PreferJWT = true
	return cfg
}