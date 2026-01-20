package config

import (
	"time"
)

type Config struct {
	Gateway GatewayConfig
	Services []ServiceConfig
	RateLimit RateLimitConfig
}

type GatewayConfig struct {
	Host string
	Port int
}

type ServiceConfig struct {
	Name string
	PathPrefix string
	UpstreamURL string
	Timeout time.Duration
}

type RateLimitConfig struct {
	RequestsPerMinute int
	BurstSize int
}

func Default() *Config {
	return &Config{
		Gateway: GatewayConfig{
			Host: "localhost",
			Port: 8080,
		},
		Services: []ServiceConfig{
			{
				Name: "user-service",
				PathPrefix: "/api/users",
				UpstreamURL: "http://localhost:8081",
				Timeout: 30 * time.Second,
			},
			{
				Name: "order-service",
				PathPrefix: "/api/orders",
				UpstreamURL: "http://localhost:8082",
				Timeout: 30 * time.Second,
			},
		},
		RateLimit: RateLimitConfig{
			RequestsPerMinute: 60,
			BurstSize: 10,
		},
	}
}