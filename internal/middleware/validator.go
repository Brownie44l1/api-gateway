package middleware

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"strings"
)

func MaxBodySize(maxBytes int64) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.ContentLength > maxBytes {
				http.Error(w, `{"error":"request body too large"}`, http.StatusRequestEntityTooLarge)
				return
			}
			r.Body = http.MaxBytesReader(w, r.Body, maxBytes)
			next.ServeHTTP(w, r)
		})
	}
}

// RequireJSON enforces Content-Type on request methods that carry a body.
func RequireJSON(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodPost, http.MethodPut, http.MethodPatch:
			ct := r.Header.Get("Content-Type")
			if !strings.Contains(ct, "application/json") {
				http.Error(w, `{"error":"Content-Type must be application/json"}`, http.StatusUnsupportedMediaType)
				return
			}
		}
		next.ServeHTTP(w, r)
	})
}

// StripHeaders removes headers a client could use to impersonate the gateway.
func StripHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r.Header.Del("X-User-ID")
		r.Header.Del("X-User-Role")
		r.Header.Del("X-Internal-Token")
		next.ServeHTTP(w, r)
	})
}

// InjectHeaders stamps trusted identity headers after the user has been verified.
func InjectHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user, ok := UserFromContext(r.Context())
		if ok {
			r.Header.Set("X-User-ID", user.ID)
			r.Header.Set("X-User-Role", strings.Join(user.Roles, ","))
		}
		next.ServeHTTP(w, r)
	})
}

// ValidateBody ensures the request body is valid JSON and rewinds it
// for downstream handlers.
func ValidateBody(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodPost, http.MethodPut, http.MethodPatch:
			var data map[string]any
			if err := json.NewDecoder(r.Body).Decode(&data); err != nil {
				http.Error(w, `{"error":"invalid JSON"}`, http.StatusBadRequest)
				return
			}
			buf, _ := json.Marshal(data)
			r.Body = io.NopCloser(bytes.NewReader(buf))
		}
		next.ServeHTTP(w, r)
	})
}

// SecurityHeaders sets standard defensive response headers.
func SecurityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("X-XSS-Protection", "1; mode=block")
		w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")
		w.Header().Set("Content-Security-Policy", "default-src 'none'")
		next.ServeHTTP(w, r)
	})
}
