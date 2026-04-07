package server

import (
    "net/http"

    "github.com/go-chi/chi/v5"
    chimiddleware "github.com/go-chi/chi/v5/middleware"

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
    })

    // protected routes — auth required
    r.Group(func(r chi.Router) {
        r.Use(middleware.Authenticate(cfg.JWTSecret))

        r.Get("/me", func(w http.ResponseWriter, r *http.Request) {
            user, _ := middleware.UserFromContext(r.Context())
            w.Write([]byte(`{"user_id":"` + user.ID + `"}`))
        })
    })

    return r
}