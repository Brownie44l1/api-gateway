package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/Brownie44l1/api-gateway/internal/auth"
	"github.com/Brownie44l1/api-gateway/internal/config"
	jwtpkg "github.com/Brownie44l1/api-gateway/internal/jwt"
	"github.com/Brownie44l1/api-gateway/internal/proxy"
	"github.com/Brownie44l1/api-gateway/internal/ratelimit"
	redisclient "github.com/Brownie44l1/api-gateway/internal/redis"
	"github.com/Brownie44l1/api-gateway/internal/registry"
)

type Gateway struct {
	authStore    *auth.APIKeyStore
	userStore    *auth.InMemoryUserStore
	tokenManager *jwtpkg.TokenManager
	registry     *registry.Registry
	proxy        *proxy.Proxy
	authMW       http.HandlerFunc
	rateLimitMW  *ratelimit.RateLimitMiddleware
	redisClient  *redisclient.Client
	authHandler  *auth.AuthHandler
}

func NewGateway(cfg *config.Config) (*Gateway, error) {
	// Initialize API key store (for backwards compatibility)
	authStore := auth.NewAPIKeyStore()
	authStore.LoadSampleKeys()
	
	// Initialize user store and JWT manager
	var userStore *auth.InMemoryUserStore
	var tokenManager *jwtpkg.TokenManager
	var authHandler *auth.AuthHandler
	
	if cfg.JWT.Enabled {
		userStore = auth.NewInMemoryUserStore()
		if err := userStore.LoadSampleUsers(); err != nil {
			return nil, fmt.Errorf("failed to load sample users: %w", err)
		}
		
		tokenManager = jwtpkg.NewTokenManager(jwtpkg.Config{
			SecretKey:       cfg.JWT.SecretKey,
			Issuer:          cfg.JWT.Issuer,
			AccessTokenTTL:  cfg.JWT.AccessTokenTTL,
			RefreshTokenTTL: cfg.JWT.RefreshTokenTTL,
		})
		
		authHandler = auth.NewAuthHandler(tokenManager, userStore)
	}
	
	reg := registry.NewRegistry()
	
	// Register services
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
	
	// Set up authentication middleware
	var authMW http.HandlerFunc
	if cfg.Auth.AllowJWT && cfg.Auth.AllowAPIKeys {
		// Dual auth: support both JWT and API keys
		log.Println("Using dual authentication (JWT + API keys)")
		dualAuth := auth.NewDualAuthMiddleware(authStore, tokenManager)
		authMW = func(w http.ResponseWriter, r *http.Request) {
			dualAuth.Middleware(func(w http.ResponseWriter, r *http.Request) {})(w, r)
		}
	} else if cfg.Auth.AllowJWT {
		// JWT only
		log.Println("Using JWT authentication only")
		jwtMW := jwtpkg.NewJWTMiddleware(tokenManager)
		authMW = func(w http.ResponseWriter, r *http.Request) {
			jwtMW.Middleware(func(w http.ResponseWriter, r *http.Request) {})(w, r)
		}
	} else {
		// API keys only (default)
		log.Println("Using API key authentication only")
		apiKeyMW := auth.NewAuthMiddleware(authStore)
		authMW = func(w http.ResponseWriter, r *http.Request) {
			apiKeyMW.Middleware(func(w http.ResponseWriter, r *http.Request) {})(w, r)
		}
	}
	
	// Set up rate limiting
	var rateLimitMW *ratelimit.RateLimitMiddleware
	var redisClient *redisclient.Client
	
	if cfg.RateLimit.UseDistributed && cfg.Redis.Enabled {
		log.Println("Using distributed rate limiting with Redis")
		
		var err error
		redisClient, err = redisclient.NewClient(redisclient.Config{
			Host:     cfg.Redis.Host,
			Port:     cfg.Redis.Port,
			Password: cfg.Redis.Password,
			DB:       cfg.Redis.DB,
		})
		if err != nil {
			return nil, fmt.Errorf("failed to connect to Redis: %w", err)
		}
		
		log.Printf("Connected to Redis at %s:%d", cfg.Redis.Host, cfg.Redis.Port)
		
		distributedLimiter := ratelimit.NewDistributedRateLimiter(
			redisClient,
			cfg.RateLimit.RequestsPerMinute,
			cfg.RateLimit.BurstSize,
		)
		
		rateLimitMW = ratelimit.NewDistributedRateLimitMiddleware(
			distributedLimiter,
			cfg.RateLimit.Strategy,
		)
		
		log.Printf("Rate limiting: %d req/min, burst: %d, strategy: %s",
			cfg.RateLimit.RequestsPerMinute,
			cfg.RateLimit.BurstSize,
			cfg.RateLimit.Strategy)
	} else {
		log.Println("Using in-memory rate limiting")
		
		inMemoryLimiter := ratelimit.NewRateLimiter(
			cfg.RateLimit.RequestsPerMinute,
			cfg.RateLimit.BurstSize,
		)
		
		rateLimitMW = ratelimit.NewRateLimitMiddleware(inMemoryLimiter)
		
		log.Printf("Rate limiting: %d req/min, burst: %d (in-memory)",
			cfg.RateLimit.RequestsPerMinute,
			cfg.RateLimit.BurstSize)
	}

	return &Gateway{
		authStore:    authStore,
		userStore:    userStore,
		tokenManager: tokenManager,
		registry:     reg,
		proxy:        prx,
		authMW:       authMW,
		rateLimitMW:  rateLimitMW,
		redisClient:  redisClient,
		authHandler:  authHandler,
	}, nil
}

func (g *Gateway) handleRequest(w http.ResponseWriter, r *http.Request) {
	service, err := g.registry.FindService(r.URL.Path)
	if err != nil {
		http.Error(w, "Service not found", http.StatusNotFound)
		return
	}

	g.proxy.Forward(w, r, &serviceAdapter{service})
}

func (g *Gateway) healthCheck(w http.ResponseWriter, r *http.Request) {
	services := g.registry.GetAllServices()
	
	redisHealthy := "N/A"
	if g.redisClient != nil {
		ctx, cancel := context.WithTimeout(r.Context(), 1*time.Second)
		defer cancel()
		
		if err := g.redisClient.Ping(ctx); err != nil {
			redisHealthy = "unhealthy"
		} else {
			redisHealthy = "healthy"
		}
	}
	
	jwtEnabled := "disabled"
	if g.tokenManager != nil {
		jwtEnabled = "enabled"
	}
	
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	
	fmt.Fprintf(w, `{"status":"healthy","services":%d,"redis":"%s","jwt":"%s","timestamp":"%s"}`,
		len(services), redisHealthy, jwtEnabled, time.Now().Format(time.RFC3339))
}

func (g *Gateway) Run(host string, port int) error {
	// Protected handler with auth + rate limiting
	protectedHandler := func(w http.ResponseWriter, r *http.Request) {
		g.authMW(w, r)
		if w.Header().Get("X-Auth-Failed") != "" {
			return
		}
		g.rateLimitMW.Middleware(g.handleRequest)(w, r)
	}

	mux := http.NewServeMux()
	
	// Public endpoints
	mux.HandleFunc("/health", g.healthCheck)
	
	// Auth endpoints (if JWT enabled)
	if g.authHandler != nil {
		mux.HandleFunc("/auth/login", g.authHandler.Login)
		mux.HandleFunc("/auth/refresh", g.authHandler.Refresh)
		
		// Protected: get current user info
		mux.HandleFunc("/auth/me", func(w http.ResponseWriter, r *http.Request) {
			g.authMW(w, r)
			if w.Header().Get("X-Auth-Failed") != "" {
				return
			}
			g.authHandler.Me(w, r)
		})
	}
	
	// All other routes require auth
	mux.HandleFunc("/", protectedHandler)

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
		
		if g.redisClient != nil {
			if err := g.redisClient.Close(); err != nil {
				log.Printf("Error closing Redis: %v", err)
			}
		}
		
		if err := server.Close(); err != nil {
			log.Printf("Error closing server: %v", err)
		}
	}()

	log.Printf("API Gateway starting on %s", addr)
	
	if g.authHandler != nil {
		log.Println("\nJWT Authentication enabled!")
		log.Println("Sample users:")
		log.Println("  - username: admin,    password: admin123    (roles: admin, user)")
		log.Println("  - username: user1,    password: password123 (roles: user)")
		log.Println("  - username: readonly, password: readonly123 (roles: viewer)")
		log.Println("\nTo login: POST /auth/login with {\"username\":\"admin\",\"password\":\"admin123\"}")
	} else {
		log.Println("\nAPI Key Authentication:")
		log.Println("  - gw_test_key_1 (user_1)")
		log.Println("  - gw_test_key_2 (user_2)")
	}
	
	return server.ListenAndServe()
}

type serviceAdapter struct {
	*registry.Service
}

func (sa *serviceAdapter) GetName() string              { return sa.Name }
func (sa *serviceAdapter) GetPathPrefix() string        { return sa.PathPrefix }
func (sa *serviceAdapter) GetUpstreamURL() string       { return sa.UpstreamURL }
func (sa *serviceAdapter) GetTimeout() time.Duration    { return sa.Timeout }

func main() {
	useRedis := flag.Bool("redis", false, "Use Redis for distributed rate limiting")
	useJWT := flag.Bool("jwt", false, "Use JWT authentication")
	strategy := flag.String("strategy", "sliding-window", "Rate limit strategy: sliding-window or token-bucket")
	port := flag.Int("port", 8080, "Port to run gateway on")
	flag.Parse()

	var cfg *config.Config
	
	if *useJWT {
		cfg = config.WithJWT()
	} else {
		cfg = config.Default()
	}
	
	if *useRedis {
		cfg.Redis.Enabled = true
		cfg.RateLimit.UseDistributed = true
		cfg.RateLimit.Strategy = *strategy
	}
	
	cfg.Gateway.Port = *port
	
	gateway, err := NewGateway(cfg)
	if err != nil {
		log.Fatalf("Failed to create gateway: %v", err)
	}
	
	if err := gateway.Run(cfg.Gateway.Host, cfg.Gateway.Port); err != nil {
		log.Fatalf("Gateway error: %v", err)
	}
}