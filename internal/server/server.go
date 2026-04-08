package server

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	chimiddleware "github.com/go-chi/chi/v5/middleware"
	"github.com/golang-jwt/jwt/v5"

	"github.com/Brownie44l1/api-gateway/internal/config"
	"github.com/Brownie44l1/api-gateway/internal/middleware"
)

func New(cfg *config.Config) http.Handler {
	r := chi.NewRouter()

	// chi's built-in request logger, just for now
	r.Use(chimiddleware.Logger)
	r.Use(chimiddleware.Recoverer)

	// public routes — no auth needed
	r.Group(func(r chi.Router) {
		r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
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

	// protected routes — auth required
	r.Group(func(r chi.Router) {
		r.Use(middleware.Authenticate(cfg.JWTSecret))

		r.Get("/me", func(w http.ResponseWriter, r *http.Request) {
			user, _ := middleware.UserFromContext(r.Context())
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]string{
				"user_id": user.ID,
			})
		})

		// only admins
		r.Group(func(r chi.Router) {
			r.Use(middleware.RequireRole("admin"))
			r.Get("/admin/dashboard", func(w http.ResponseWriter, r *http.Request) {
				w.Write([]byte(`{"message":"welcome admin"}`))
			})
		})

		// only internal services
		r.Group(func(r chi.Router) {
			r.Use(middleware.RequireRole("service"))
			r.Get("/internal/stats", func(w http.ResponseWriter, r *http.Request) {
				w.Write([]byte(`{"message":"internal stats"}`))
			})
		})
	})

	return r
}
