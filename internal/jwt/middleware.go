package jwt

import (
	"context"
	"net/http"
	"strings"
)

type contextKey string

const (
	ClaimsContextKey contextKey = "jwt_claims"
	UserIDContextKey contextKey = "user_id"
	RolesContextKey  contextKey = "roles"
)

// JWTMiddleware handles JWT authentication
type JWTMiddleware struct {
	tokenManager *TokenManager
	// Optional: also allow API keys for backwards compatibility
	allowAPIKeys bool
}

// NewJWTMiddleware creates a new JWT middleware
func NewJWTMiddleware(tokenManager *TokenManager) *JWTMiddleware {
	return &JWTMiddleware{
		tokenManager: tokenManager,
		allowAPIKeys: false,
	}
}

// Middleware validates JWT tokens from Authorization header
func (jm *JWTMiddleware) Middleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Extract token from Authorization header
		authHeader := r.Header.Get("Authorization")
		if authHeader == "" {
			http.Error(w, "Missing authorization header", http.StatusUnauthorized)
			return
		}

		// Check for Bearer token
		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) != 2 || parts[0] != "Bearer" {
			http.Error(w, "Invalid authorization header format. Expected: Bearer <token>", http.StatusUnauthorized)
			return
		}

		tokenString := parts[1]

		// Validate token
		claims, err := jm.tokenManager.ValidateToken(tokenString)
		if err != nil {
			switch err {
			case ErrExpiredToken:
				http.Error(w, "Token has expired", http.StatusUnauthorized)
			case ErrInvalidToken:
				http.Error(w, "Invalid token", http.StatusUnauthorized)
			case ErrInvalidSignature:
				http.Error(w, "Invalid token signature", http.StatusUnauthorized)
			case ErrMissingClaims:
				http.Error(w, "Token missing required claims", http.StatusUnauthorized)
			default:
				http.Error(w, "Authentication failed", http.StatusUnauthorized)
			}
			return
		}

		// Add claims to request context
		ctx := r.Context()
		ctx = context.WithValue(ctx, ClaimsContextKey, claims)
		ctx = context.WithValue(ctx, UserIDContextKey, claims.UserID)
		ctx = context.WithValue(ctx, RolesContextKey, claims.Roles)

		next(w, r.WithContext(ctx))
	}
}

// RequireRole middleware checks if user has a specific role
func (jm *JWTMiddleware) RequireRole(role string) func(http.HandlerFunc) http.HandlerFunc {
	return func(next http.HandlerFunc) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			claims, ok := GetClaimsFromContext(r.Context())
			if !ok {
				http.Error(w, "Unauthorized", http.StatusUnauthorized)
				return
			}

			if !claims.HasRole(role) {
				http.Error(w, "Forbidden: insufficient permissions", http.StatusForbidden)
				return
			}

			next(w, r)
		}
	}
}

// RequireAnyRole middleware checks if user has any of the specified roles
func (jm *JWTMiddleware) RequireAnyRole(roles ...string) func(http.HandlerFunc) http.HandlerFunc {
	return func(next http.HandlerFunc) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			claims, ok := GetClaimsFromContext(r.Context())
			if !ok {
				http.Error(w, "Unauthorized", http.StatusUnauthorized)
				return
			}

			if !claims.HasAnyRole(roles...) {
				http.Error(w, "Forbidden: insufficient permissions", http.StatusForbidden)
				return
			}

			next(w, r)
		}
	}
}

// RequirePermission middleware checks if user has a specific permission
func (jm *JWTMiddleware) RequirePermission(permission string) func(http.HandlerFunc) http.HandlerFunc {
	return func(next http.HandlerFunc) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			claims, ok := GetClaimsFromContext(r.Context())
			if !ok {
				http.Error(w, "Unauthorized", http.StatusUnauthorized)
				return
			}

			if !claims.HasPermission(permission) {
				http.Error(w, "Forbidden: insufficient permissions", http.StatusForbidden)
				return
			}

			next(w, r)
		}
	}
}

// GetClaimsFromContext retrieves JWT claims from request context
func GetClaimsFromContext(ctx context.Context) (*Claims, bool) {
	claims, ok := ctx.Value(ClaimsContextKey).(*Claims)
	return claims, ok
}

// GetUserIDFromContext retrieves user ID from request context
func GetUserIDFromContext(ctx context.Context) (string, bool) {
	userID, ok := ctx.Value(UserIDContextKey).(string)
	return userID, ok
}

// GetRolesFromContext retrieves roles from request context
func GetRolesFromContext(ctx context.Context) ([]string, bool) {
	roles, ok := ctx.Value(RolesContextKey).([]string)
	return roles, ok
}