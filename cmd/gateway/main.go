package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/Brownie44l1/api-gateway/internal/auth"
	"github.com/Brownie44l1/api-gateway/internal/config"
	"github.com/Brownie44l1/api-gateway/internal/proxy"
	"github.com/Brownie44l1/api-gateway/internal/ratelimit"
	"github.com/Brownie44l1/api-gateway/internal/registry"
)

type Gateway struct {
	authStore    *auth.APIKeyStore
	rateLimiter  *ratelimit.RateLimiter
	registry     *registry.Registry
	proxy        *proxy.Proxy
	authMW       *auth.AuthMiddleware
	rateLimitMW  *ratelimit.RateLimitMiddleware
}

func NewGateway(cfg *config.Config) *Gateway {
	// Initialize components
	authStore := auth.NewAPIKeyStore()
	authStore.LoadSampleKeys() // Load test keys
	
	rateLimiter := ratelimit.NewRateLimiter(
		cfg.RateLimit.RequestsPerMinute,
		cfg.RateLimit.BurstSize,
	)
	
	reg := registry.NewRegistry()
	
	// Register services from config
	for _, svc := range cfg.Services {
		reg.Register(&registry.Service{
			Name:        svc.Name,
			PathPrefix:  svc.PathPrefix,
			UpstreamURL: svc.UpstreamURL,
			Timeout:     svc.Timeout,
			Healthy:     true,
		})
	}
	
	prx := proxy.NewProxy()
	authMW := auth.NewAuthMiddleware(authStore)
	rateLimitMW := ratelimit.NewRateLimitMiddleware(rateLimiter)

	return &Gateway{
		authStore:    authStore,
		rateLimiter:  rateLimiter,
		registry:     reg,
		proxy:        prx,
		authMW:       authMW,
		rateLimitMW:  rateLimitMW,
	}
}

func (g *Gateway) handleRequest(w http.ResponseWriter, r *http.Request) {
	// Find matching service
	service, err := g.registry.FindService(r.URL.Path)
	if err != nil {
		http.Error(w, "Service not found", http.StatusNotFound)
		return
	}

	// Proxy the request
	g.proxy.Forward(w, r, &serviceAdapter{service})
}

func (g *Gateway) healthCheck(w http.ResponseWriter, r *http.Request) {
	services := g.registry.GetAllServices()
	
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	
	fmt.Fprintf(w, `{"status":"healthy","services":%d,"timestamp":"%s"}`,
		len(services), time.Now().Format(time.RFC3339))
}

func (g *Gateway) Run(host string, port int) error {
	// Apply middleware chain: auth -> rate limit -> handler
	handler := g.authMW.Middleware(
		g.rateLimitMW.Middleware(g.handleRequest),
	)

	mux := http.NewServeMux()
	mux.HandleFunc("/health", g.healthCheck)
	mux.HandleFunc("/", handler)

	addr := fmt.Sprintf("%s:%d", host, port)
	server := &http.Server{
		Addr:         addr,
		Handler:      mux,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// Graceful shutdown
	go func() {
		sigint := make(chan os.Signal, 1)
		signal.Notify(sigint, os.Interrupt, syscall.SIGTERM)
		<-sigint

		log.Println("Shutting down gateway...")
		if err := server.Close(); err != nil {
			log.Printf("Error closing server: %v", err)
		}
	}()

	log.Printf("API Gateway starting on %s", addr)
	log.Println("Sample API Keys:")
	log.Println("  - gw_test_key_1 (user_1)")
	log.Println("  - gw_test_key_2 (user_2)")
	
	return server.ListenAndServe()
}

// serviceAdapter adapts registry.Service to proxy.Service interface
type serviceAdapter struct {
	*registry.Service
}

func (sa *serviceAdapter) GetName() string        { return sa.Name }
func (sa *serviceAdapter) GetPathPrefix() string  { return sa.PathPrefix }
func (sa *serviceAdapter) GetUpstreamURL() string { return sa.UpstreamURL }
func (sa *serviceAdapter) GetTimeout() time.Duration { return sa.Timeout }

func main() {
	cfg := config.Default()
	gateway := NewGateway(cfg)
	
	if err := gateway.Run(cfg.Gateway.Host, cfg.Gateway.Port); err != nil {
		log.Fatalf("Gateway error: %v", err)
	}
}