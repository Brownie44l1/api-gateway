package server

import (
	"encoding/json"
	"net"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	chimiddleware "github.com/go-chi/chi/v5/middleware"
	"github.com/golang-jwt/jwt/v5"

	"github.com/Brownie44l1/api-gateway/internal/config"
	"github.com/Brownie44l1/api-gateway/internal/middleware"
	"github.com/Brownie44l1/api-gateway/internal/proxy"
	"github.com/Brownie44l1/rate-limiter/ratelimiter"
)

func New(cfg *config.Config, rl *ratelimiter.Client) http.Handler {
	r := chi.NewRouter()

	routesCfg, err := proxy.ParseRoutes(cfg.RoutesRaw)
	if err != nil {
		panic("invalid ROUTES config: " + err.Error())
	}

	p, err := proxy.New(routesCfg, proxy.Options{
		CBMaxFailures:  cfg.CBMaxFailures,
		CBResetTimeout: cfg.CBResetTimeout,
	})
	if err != nil {
		panic(err)
	}

	// request ID first — everything downstream can log it
	r.Use(middleware.RequestID)
	r.Use(middleware.StructuredLogger) // structured slog, replaces chimiddleware.Logger
	r.Use(chimiddleware.Recoverer)
	r.Use(middleware.SecurityHeaders)

	// global middleware — runs on every single request
	r.Use(middleware.StripHeaders)                    // strip lying headers first
	r.Use(middleware.RequireJSON)                     // enforce content type
	r.Use(middleware.MaxBodySize(cfg.MaxBodyBytes))   // configurable body limit
	r.Use(middleware.ValidateBody)                    // validate request body

	// per-IP rate limiter — public routes
	ipLimiter := rl.Middleware(ratelimiter.Config{
		Limit:      cfg.IPRateLimit,
		RefillRate: cfg.IPRateRefill,
		KeyLookup: func(r *http.Request) string {
			ip, _, err := net.SplitHostPort(r.RemoteAddr)
			if err != nil {
				return "ip:" + r.RemoteAddr
			}
			return "ip:" + ip
		},
	})

	// per-user rate limiter — authenticated routes
	userLimiter := rl.Middleware(ratelimiter.Config{
		Limit:      cfg.RateLimit,
		RefillRate: cfg.RateRefill,
		KeyLookup: func(r *http.Request) string {
			user, ok := middleware.UserFromContext(r.Context())
			if !ok {
				ip, _, err := net.SplitHostPort(r.RemoteAddr)
				if err != nil {
					return "ip:" + r.RemoteAddr
				}
				return "ip:" + ip
			}
			return "user:" + user.ID
		},
	})

	// public routes — no auth needed
	r.Group(func(r chi.Router) {
		r.Use(ipLimiter)

		r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(`{"status":"ok"}`))
		})

		r.Post("/auth/login", func(w http.ResponseWriter, r *http.Request) {
			var body struct {
				UserID string `json:"user_id"`
			}
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.UserID == "" {
				http.Error(w, `{"error":"user_id required"}`, http.StatusBadRequest)
				return
			}

			roles := []string{"user"}

			claims := jwt.MapClaims{
				"user_id": body.UserID,
				"roles":   roles,
				"exp":     time.Now().Add(cfg.JWTExpiry).Unix(),
				"iat":     time.Now().Unix(),
			}

			token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
			signed, err := token.SignedString([]byte(cfg.JWTSecret))
			if err != nil {
				http.Error(w, `{"error":"could not generate token"}`, http.StatusInternalServerError)
				return
			}

			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]string{
				"access_token": signed,
			})
		})
	})

	// protected routes
	r.Group(func(r chi.Router) {
		r.Use(middleware.Authenticate(cfg.JWTSecret))
		r.Use(userLimiter)
		r.Use(middleware.InjectHeaders)

		r.Get("/me", func(w http.ResponseWriter, r *http.Request) {
			user, _ := middleware.UserFromContext(r.Context())
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]string{
				"user_id": user.ID,
			})
		})

		r.Group(func(r chi.Router) {
			r.Use(middleware.RequireRole("admin"))
			r.Get("/admin/dashboard", func(w http.ResponseWriter, r *http.Request) {
				w.Write([]byte(`{"message":"welcome admin"}`))
			})
		})

		r.Group(func(r chi.Router) {
			r.Use(middleware.RequireRole("service"))
			r.Get("/internal/stats", func(w http.ResponseWriter, r *http.Request) {
				w.Write([]byte(`{"message":"internal stats"}`))
			})
		})
	})

	r.Handle("/*", p.Handler())

	return r
}
