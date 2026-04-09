package middleware

import (
	"net/http"
	"strings"
)

func MaxBodySize(maxBytes int64) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// only request that has body
			if r.ContentLength > maxBytes {
				http.Error(w, `{"error":"request body too large"}`, http.StatusRequestEntityTooLarge)
				return
			}
			// avoid streaming beyoud the limit
			r.Body = http.MaxBytesReader(w, r.Body, maxBytes)

			next.ServeHTTP(w, r)
		})
	}
}

// content type validation on method that has a body.
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

// Stripping Headers to avoid impersonation
func StripHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r.Header.Del("X-User-ID")
        r.Header.Del("X-User-Role")
        r.Header.Del("X-Internal-Token")

        next.ServeHTTP(w, r)
	})
}

// Injecting header after the user has been validated
func InjectHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user, ok := UserFromContext(r.Context())
        if ok {
            // now these headers are trustworthy — gateway set them
            r.Header.Set("X-User-ID", user.ID)
            r.Header.Set("X-User-Role", strings.Join(user.Roles, ","))
        }

        next.ServeHTTP(w, r)
	})
}
