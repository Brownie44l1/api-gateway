package main

import (
	"context"
	"fmt"
	"log"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/Brownie44l1/api-gateway/internal/config"
	"github.com/Brownie44l1/api-gateway/internal/server"
	"github.com/Brownie44l1/rate-limiter/ratelimiter"
)

func main() {
	cfg := config.Load()
	fmt.Printf("AdminUsers: %v\n", cfg.AdminUsers)

	rl, err := ratelimiter.NewClient(cfg.RedisAddr, cfg.RedisPassword)
	if err != nil {
		log.Fatalf("could not connect to redis: %v", err)
	}
	defer rl.Close()

	srv := &http.Server{
		Addr:         ":" + cfg.Port,
		Handler:      server.New(cfg, rl),
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// start server in a goroutine so we can listen for shutdown signals
	go func() {
		fmt.Println("Gateway running on port", cfg.Port)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("server error: %v", err)
		}
	}()

	// block until we receive SIGINT or SIGTERM
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	slog.Info("shutting down — draining in-flight requests")

	// give in-flight requests 15 seconds to finish
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		log.Fatalf("forced shutdown: %v", err)
	}

	slog.Info("shutdown complete")
}
