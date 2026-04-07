package middleware

import (
	"context"
	"net/http"
	"strings"

	"github.com/golang-jwt/jwt/v5"
)

func Authenticate(secret string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Extract the header.
			authHeader := r.Header.Get("Authorization")
			if authHeader == "" {
				http.Error(w, `{"error":"mising authorization header"}`, http.StatusUnauthorized)
				return
			}

			// Strip "Bearer " prefix, get the raw token string
			parts := strings.SplitN(authHeader, " ", 2)
			if len(parts) != 2 || parts[0] != "Bearer" {
				http.Error(w, `{"error":"invalid authorization format"}`, http.StatusUnauthorized)
				return
			}
			tokenString := parts[1]

			// Steps 3,4,5: Parse + verify signature + check expiry
			token, err := jwt.Parse(tokenString, func(t *jwt.Token) (any, error) {
				// Check the algorithm
				if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
					return nil, jwt.ErrSignatureInvalid
				}
				return []byte(secret), nil
			})
			if err != nil || !token.Valid {
				http.Error(w, `{"error":"invalid or expired token"}`, http.StatusUnauthorized)
				return
			}

			// Extract claims — pull user_id and roles out
			claims, ok := token.Claims.(jwt.MapClaims)
			if !ok {
				http.Error(w, `{"error":"invalid token claims"}`, http.StatusUnauthorized)
				return
			}

			// [Authorization] --------------------------------------

			user := &AuthenticatedUser{
				ID:    claims["user_id"].(string),
				Roles: []string{},
			}

			// attach roles if present
			if roles, ok := claims["roles"].([]any); ok {
				for _, r := range roles {
					if role, ok := r.(string); ok {
						user.Roles = append(user.Roles, role)
					}
				}
			}

			// attach user to context so downstream handlers can read it
			ctx := context.WithValue(r.Context(), userContextKey, user)

			// Step 7: Call next — let the request through
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}
