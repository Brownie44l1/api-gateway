package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/Brownie44l1/api-gateway/internal/auth"
	"github.com/Brownie44l1/api-gateway/internal/config"
	"github.com/Brownie44l1/http/internal/httpserver"
	jwtpkg "github.com/Brownie44l1/api-gateway/internal/jwt"
	"github.com/Brownie44l1/api-gateway/internal/proxy"
	"github.com/Brownie44l1/api-gateway/internal/ratelimit"
	redisclient "github.com/Brownie44l1/api-gateway/internal/redis"
	"github.com/Brownie44l1/api-gateway/internal/registry"
	
	customhttp "github.com/Brownie44l1/http/internal/server"
)

type Gateway struct {
	authStore    *auth.APIKeyStore
	userStore    *auth.InMemoryUserStore
	tokenManager *jwtpkg.TokenManager
	registry     *registry.Registry
	proxy        *proxy.Proxy
	rateLimitMW  *ratelimit.RateLimitMiddleware
	redisClient  *redisclient.Client
	authHandler  *auth.AuthHandler
	router       *httpserver.Router
	server       *customhttp.Server
}

func NewGateway(cfg *config.Config) (*Gateway, error) {
	// Initialize components (same as before)
	authStore := auth.NewAPIKeyStore()
	authStore.LoadSampleKeys()
	
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
		
		distributedLimiter := ratelimit.NewDistributedRateLimiter(
			redisClient,
			cfg.RateLimit.RequestsPerMinute,
			cfg.RateLimit.BurstSize,
		)
		
		rateLimitMW = ratelimit.NewDistributedRateLimitMiddleware(
			distributedLimiter,
			cfg.RateLimit.Strategy,
		)
	} else {
		log.Println("Using in-memory rate limiting")
		
		inMemoryLimiter := ratelimit.NewRateLimiter(
			cfg.RateLimit.RequestsPerMinute,
			cfg.RateLimit.BurstSize,
		)
		
		rateLimitMW = ratelimit.NewRateLimitMiddleware(inMemoryLimiter)
	}

	// Create router
	router := httpserver.NewRouter()
	
	// Create server config
	serverConfig := &customhttp.Config{
		Addr:               fmt.Sprintf("%s:%d", cfg.Gateway.Host, cfg.Gateway.Port),
		ReadTimeout:        15 * time.Second,
		WriteTimeout:       15 * time.Second,
		IdleTimeout:        60 * time.Second,
		MaxHeaderBytes:     1 << 20,
		MaxRequestBodySize: 10 << 20,
	}

	gateway := &Gateway{
		authStore:    authStore,
		userStore:    userStore,
		tokenManager: tokenManager,
		registry:     reg,
		proxy:        prx,
		rateLimitMW:  rateLimitMW,
		redisClient:  redisClient,
		authHandler:  authHandler,
		router:       router,
	}

	// Set up routes
	gateway.setupRoutes(cfg)
	
	// Create custom HTTP server
	gateway.server = customhttp.New(serverConfig, router)

	return gateway, nil
}

func (g *Gateway) setupRoutes(cfg *config.Config) {
	// Public routes
	g.router.GET("/health", g.healthCheck)
	
	// Auth routes (if JWT enabled)
	if g.authHandler != nil {
		g.router.POST("/auth/login", g.handleLogin)
		g.router.POST("/auth/refresh", g.handleRefresh)
		g.router.GET("/auth/me", g.withAuth(g.handleMe))
	}
	
	// Proxy routes (protected)
	// For now, we'll handle all other paths in a catch-all
	// You can enhance this with pattern matching later
}

func (g *Gateway) healthCheck(w *httpserver.ResponseWriter, r *httpserver.Request) {
	services := g.registry.GetAllServices()
	
	redisHealthy := "N/A"
	if g.redisClient != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
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
	
	response := map[string]interface{}{
		"status":    "healthy",
		"services":  len(services),
		"redis":     redisHealthy,
		"jwt":       jwtEnabled,
		"timestamp": time.Now().Format(time.RFC3339),
	}
	
	w.WriteJSON(200, response)
}

func (g *Gateway) handleLogin(w *httpserver.ResponseWriter, r *httpserver.Request) {
	var req auth.LoginRequest
	if err := json.Unmarshal(r.Body(), &req); err != nil {
		w.Error("Invalid request body", 400)
		return
	}

	// Validate credentials
	user, err := g.userStore.ValidateCredentials(req.Username, req.Password)
	if err != nil {
		w.Error("Invalid credentials", 401)
		return
	}

	// Generate tokens
	accessToken, err := g.tokenManager.GenerateAccessToken(
		user.ID, user.Email, user.Roles, user.Permissions,
	)
	if err != nil {
		w.Error("Failed to generate access token", 500)
		return
	}

	refreshToken, err := g.tokenManager.GenerateRefreshToken(user.ID)
	if err != nil {
		w.Error("Failed to generate refresh token", 500)
		return
	}

	response := auth.LoginResponse{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		TokenType:    "Bearer",
		ExpiresIn:    int(g.tokenManager.GetAccessTokenTTL().Seconds()),
	}

	w.WriteJSON(200, response)
}

func (g *Gateway) handleRefresh(w *httpserver.ResponseWriter, r *httpserver.Request) {
	var req auth.RefreshRequest
	if err := json.Unmarshal(r.Body(), &req); err != nil {
		w.Error("Invalid request body", 400)
		return
	}

	userID, err := g.tokenManager.ValidateRefreshToken(req.RefreshToken)
	if err != nil {
		w.Error("Invalid refresh token", 401)
		return
	}

	user, err := g.userStore.GetUser(userID)
	if err != nil {
		w.Error("User not found", 401)
		return
	}

	accessToken, err := g.tokenManager.GenerateAccessToken(
		user.ID, user.Email, user.Roles, user.Permissions,
	)
	if err != nil {
		w.Error("Failed to generate access token", 500)
		return
	}

	response := auth.RefreshResponse{
		AccessToken: accessToken,
		TokenType:   "Bearer",
		ExpiresIn:   int(g.tokenManager.GetAccessTokenTTL().Seconds()),
	}

	w.WriteJSON(200, response)
}

func (g *Gateway) handleMe(w *httpserver.ResponseWriter, r *httpserver.Request) {
	// TODO: Extract user from context set by auth middleware
	w.WriteJSON(200, map[string]string{"message": "User info"})
}

// withAuth is a middleware that requires authentication
func (g *Gateway) withAuth(handler httpserver.HandlerFunc) httpserver.HandlerFunc {
	return func(w *httpserver.ResponseWriter, r *httpserver.Request) {
		// Extract Bearer token
		authHeader := r.Header("Authorization")
		if authHeader == "" {
			w.Error("Missing authorization header", 401)
			return
		}

		// TODO: Validate token and add to context
		
		handler(w, r)
	}
}

func (g *Gateway) Run() error {
	log.Printf("API Gateway starting on %s", g.server.config.Addr)
	
	if g.authHandler != nil {
		log.Println("\nJWT Authentication enabled!")
		log.Println("Sample users:")
		log.Println("  - username: admin,    password: admin123    (roles: admin, user)")
		log.Println("  - username: user1,    password: password123 (roles: user)")
		log.Println("  - username: readonly, password: readonly123 (roles: viewer)")
	}

	// Graceful shutdown
	go func() {
		sigint := make(chan os.Signal, 1)
		signal.Notify(sigint, os.Interrupt, syscall.SIGTERM)
		<-sigint

		log.Println("Shutting down gateway...")
		
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		
		if err := g.server.Shutdown(ctx); err != nil {
			log.Printf("Server shutdown error: %v", err)
		}
		
		if g.redisClient != nil {
			g.redisClient.Close()
		}
	}()

	return g.server.ListenAndServe()
}

func main() {
	useRedis := flag.Bool("redis", false, "Use Redis for distributed rate limiting")
	useJWT := flag.Bool("jwt", false, "Use JWT authentication")
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
	}
	
	cfg.Gateway.Port = *port
	
	gateway, err := NewGateway(cfg)
	if err != nil {
		log.Fatalf("Failed to create gateway: %v", err)
	}
	
	if err := gateway.Run(); err != nil {
		log.Fatalf("Gateway error: %v", err)
	}
}