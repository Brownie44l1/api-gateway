package main

import (
	"fmt"
	"log"
	"net/http"

	"github.com/Brownie44l1/api-gateway/internal/config"
	"github.com/Brownie44l1/api-gateway/internal/server"

	"github.com/Brownie44l1/rate-limiter/ratelimiter"
)

func main() {
	cfg := config.Load()

	// initialize the rate limiter client
	rl, err := ratelimiter.NewClient(cfg.RedisAddr, cfg.RedisPassword)
	if err != nil {
		log.Fatalf("could not connect to redis: %v", err)
	}
	defer rl.Close()

	srv := server.New(cfg, rl)

	fmt.Println("Gateway running on port", cfg.Port)
	http.ListenAndServe(":"+cfg.Port, srv)
}
