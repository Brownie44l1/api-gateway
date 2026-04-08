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
	"github.com/Brownie44l1/rate-limiter/ratelimiter"
)

func New(cfg *config.Config, rl *ratelimiter.Client) http.Handler {
	r := chi.NewRouter()

	r.Use(chimiddleware.Logger)
	r.Use(chimiddleware.Recoverer)

	// per-IP rate limiter — for public routes
	// we don't know who they are yet, so IP is the best we have
	ipLimiter := rl.Middleware(ratelimiter.Config{
		Limit:      20, // stricter on public routes
		RefillRate: 20, // 1 per 3 seconds steady state
		KeyLookup: func(r *http.Request) string {
			ip, _, err := net.SplitHostPort(r.RemoteAddr)
			if err != nil {
				return "ip:" + r.RemoteAddr // fallback
			}
			return "ip:" + ip
		},
	})

	// per-user rate limiter — for authenticated routes
	// we know who they are, so we limit by user ID
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
			// parse the request body
			var body struct {
				UserID string `json:"user_id"`
			}
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.UserID == "" {
				http.Error(w, `{"error":"user_id required"}`, http.StatusBadRequest)
				return
			}

			roles := []string{"user"}
			if body.UserID == "7" {
				roles = []string{"admin"}
			}

			// build the claims — this is the payload that goes inside the token
			claims := jwt.MapClaims{
				"user_id": body.UserID,
				"roles":   roles,
				"exp":     time.Now().Add(15 * time.Minute).Unix(), // expires in 15min
				"iat":     time.Now().Unix(),                       // issued at
			}

			// sign the token with your secret
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
		r.Use(userLimiter) // ← runs after auth, so user is on context

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

	return r
}
