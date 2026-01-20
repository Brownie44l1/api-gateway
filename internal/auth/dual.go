package auth

import (
	"context"
	"net/http"
	"strings"

	jwtpkg "github.com/Brownie44l1/api-gateway/internal/jwt"
)

// DualAuthMiddleware supports both API keys and JWT tokens
type DualAuthMiddleware struct {
	apiKeyStore  *APIKeyStore
	tokenManager *jwtpkg.TokenManager
}

// NewDualAuthMiddleware creates middleware that accepts both auth methods
func NewDualAuthMiddleware(apiKeyStore *APIKeyStore, tokenManager *jwtpkg.TokenManager) *DualAuthMiddleware {
	return &DualAuthMiddleware{
		apiKeyStore:  apiKeyStore,
		tokenManager: tokenManager,
	}
}

// Middleware tries JWT first, falls back to API key
func (dam *DualAuthMiddleware) Middleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Try JWT first (Authorization: Bearer <token>)
		authHeader := r.Header.Get("Authorization")
		if authHeader != "" && strings.HasPrefix(authHeader, "Bearer ") {
			if dam.handleJWT(w, r, next) {
				return
			}
			// JWT auth failed, don't try API key
			return
		}

		// Try API key (X-API-Key header)
		apiKey := r.Header.Get("X-API-Key")
		if apiKey != "" {
			if dam.handleAPIKey(w, r, next, apiKey) {
				return
			}
			// API key auth failed
			return
		}

		// No auth provided
		http.Error(w, "Missing authentication: provide either Authorization header (Bearer token) or X-API-Key header", http.StatusUnauthorized)
	}
}

// handleJWT processes JWT authentication
func (dam *DualAuthMiddleware) handleJWT(w http.ResponseWriter, r *http.Request, next http.HandlerFunc) bool {
	authHeader := r.Header.Get("Authorization")
	parts := strings.SplitN(authHeader, " ", 2)
	if len(parts) != 2 {
		http.Error(w, "Invalid authorization header format", http.StatusUnauthorized)
		return false
	}

	tokenString := parts[1]
	claims, err := dam.tokenManager.ValidateToken(tokenString)
	if err != nil {
		switch err {
		case jwtpkg.ErrExpiredToken:
			http.Error(w, "Token has expired", http.StatusUnauthorized)
		case jwtpkg.ErrInvalidToken:
			http.Error(w, "Invalid token", http.StatusUnauthorized)
		case jwtpkg.ErrInvalidSignature:
			http.Error(w, "Invalid token signature", http.StatusUnauthorized)
		default:
			http.Error(w, "Authentication failed", http.StatusUnauthorized)
		}
		return false
	}

	// Add both JWT claims and backwards-compatible context values
	ctx := r.Context()
	ctx = context.WithValue(ctx, jwtpkg.ClaimsContextKey, claims)
	ctx = context.WithValue(ctx, APIKeyContextKey, "jwt:"+claims.UserID) // For rate limiting
	ctx = context.WithValue(ctx, UserIDContextKey, claims.UserID)

	next(w, r.WithContext(ctx))
	return true
}

// handleAPIKey processes API key authentication
func (dam *DualAuthMiddleware) handleAPIKey(w http.ResponseWriter, r *http.Request, next http.HandlerFunc, apiKey string) bool {
	key, err := dam.apiKeyStore.ValidateAPIKey(apiKey)
	if err != nil {
		switch err {
		case ErrInvalidAPIKey:
			http.Error(w, "Invalid API key", http.StatusUnauthorized)
		case ErrExpiredAPIKey:
			http.Error(w, "API key has expired", http.StatusUnauthorized)
		default:
			http.Error(w, "Authentication failed", http.StatusInternalServerError)
		}
		return false
	}

	// Add API key info to context
	ctx := r.Context()
	ctx = context.WithValue(ctx, APIKeyContextKey, key.Key)
	ctx = context.WithValue(ctx, UserIDContextKey, key.UserID)

	next(w, r.WithContext(ctx))
	return true
}