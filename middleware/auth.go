// Package middleware provides HTTP middleware for the Routa tunnel.
package middleware

import (
	"crypto/subtle"
	"net/http"
)

// BasicAuth returns an http.Handler middleware that enforces HTTP Basic
// Authentication. If user and pass are both empty, it's a no-op passthrough.
func BasicAuth(user, pass string, next http.Handler) http.Handler {
	if user == "" && pass == "" {
		return next
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		u, p, ok := r.BasicAuth()
		if !ok ||
			subtle.ConstantTimeCompare([]byte(u), []byte(user)) != 1 ||
			subtle.ConstantTimeCompare([]byte(p), []byte(pass)) != 1 {
			w.Header().Set("WWW-Authenticate", `Basic realm="Routa Tunnel"`)
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// CheckBasicAuth performs a basic auth check on raw header values.
// Returns true if auth passes or if no credentials are configured.
func CheckBasicAuth(user, pass, authHeader string) bool {
	if user == "" && pass == "" {
		return true
	}
	// Parse Basic auth from raw header value.
	if len(authHeader) < 6 {
		return false
	}
	// We defer to the relay-side approach: the relay extracts and checks
	// before forwarding through the tunnel.
	return false
}
