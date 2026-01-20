package auth

import (
	"context"
	"net/http"
)

type contextKey string

const (
	APIKeyContextKey contextKey = "api_key"
	UserIDContextKey contextKey = "user_id"
)

type AuthMiddleware struct {
	store *APIKeyStore
}

func NewAuthMiddleware(store *APIKeyStore) *AuthMiddleware {
	return &AuthMiddleware{
		store: store,
	}
}

// Middleware validates API key from request header
func (am *AuthMiddleware) Middleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		apiKey := r.Header.Get("X-API-Key")
		if apiKey == "" {
			http.Error(w, "Missing API key", http.StatusUnauthorized)
			return
		}

		key, err := am.store.ValidateAPIKey(apiKey)
		if err != nil {
			switch err {
			case ErrInvalidAPIKey:
				http.Error(w, "Invalid API key", http.StatusUnauthorized)
			case ErrExpiredAPIKey:
				http.Error(w, "API key has expired", http.StatusUnauthorized)
			default:
				http.Error(w, "Authentication failed", http.StatusInternalServerError)
			}
			return
		}

		// Add API key info to request context
		ctx := r.Context()
		ctx = context.WithValue(ctx, APIKeyContextKey, key.Key)
		ctx = context.WithValue(ctx, UserIDContextKey, key.UserID)

		next(w, r.WithContext(ctx))
	}
}

// GetAPIKeyFromContext retrieves the API key from request context
func GetAPIKeyFromContext(ctx context.Context) (string, bool) {
	key, ok := ctx.Value(APIKeyContextKey).(string)
	return key, ok
}

// GetUserIDFromContext retrieves the user ID from request context
func GetUserIDFromContext(ctx context.Context) (string, bool) {
	userID, ok := ctx.Value(UserIDContextKey).(string)
	return userID, ok
}