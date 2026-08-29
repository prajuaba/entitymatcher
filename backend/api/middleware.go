package api

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
)

// Unexported context key type for storing JWT claims
type claimsKey struct{}

// ClaimsFrom retrieves JWTClaims from the request context.
// Returns nil if claims are not present.
func ClaimsFrom(ctx context.Context) *JWTClaims {
	claims, ok := ctx.Value(claimsKey{}).(*JWTClaims)
	if !ok {
		return nil
	}
	return claims
}

// RequireAuth is middleware that validates the Authorization header and ensures
// the request has a valid JWT token. It puts the JWTClaims into the request context.
// Returns 401 if the token is invalid or missing.
// Handles CORS preflight (OPTIONS) by returning 204 before auth check.
func RequireAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Handle CORS preflight - return 204 before auth check
		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusNoContent)
			return
		}

		authHeader := r.Header.Get("Authorization")
		if authHeader == "" || !strings.HasPrefix(authHeader, "Bearer ") {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusUnauthorized)
			json.NewEncoder(w).Encode(map[string]string{"error": "unauthorized"})
			return
		}

		tokenStr := strings.TrimPrefix(authHeader, "Bearer ")
		claims, err := parseToken(tokenStr)
		if err != nil {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusUnauthorized)
			json.NewEncoder(w).Encode(map[string]string{"error": "unauthorized"})
			return
		}

		// Put claims into context
		ctx := context.WithValue(r.Context(), claimsKey{}, claims)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// RequireAuthAllowQueryToken is middleware that validates the Authorization header
// OR accepts a token from the access_token query parameter (for EventSource browser clients).
// It puts the JWTClaims into the request context.
// Returns 401 if the token is invalid or missing.
// Handles CORS preflight (OPTIONS) by returning 204 before auth check.
func RequireAuthAllowQueryToken(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Handle CORS preflight - return 204 before auth check
		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusNoContent)
			return
		}

		var tokenStr string

		// Try Authorization header first
		authHeader := r.Header.Get("Authorization")
		if authHeader != "" && strings.HasPrefix(authHeader, "Bearer ") {
			tokenStr = strings.TrimPrefix(authHeader, "Bearer ")
		} else {
			// Fall back to query parameter for EventSource clients
			tokenStr = r.URL.Query().Get("access_token")
		}

		if tokenStr == "" {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusUnauthorized)
			json.NewEncoder(w).Encode(map[string]string{"error": "unauthorized"})
			return
		}

		claims, err := parseToken(tokenStr)
		if err != nil {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusUnauthorized)
			json.NewEncoder(w).Encode(map[string]string{"error": "unauthorized"})
			return
		}

		// Put claims into context
		ctx := context.WithValue(r.Context(), claimsKey{}, claims)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// RequireRole returns middleware that checks if the authenticated user's role
// is in the allowed set of roles. Returns 403 if not authorized.
// Must be chained after RequireAuth to work properly.
func RequireRole(roles ...Role) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			claims := ClaimsFrom(r.Context())
			if claims == nil {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusUnauthorized)
				json.NewEncoder(w).Encode(map[string]string{"error": "unauthorized"})
				return
			}

			// Check if role is in allowed set
			allowed := false
			for _, role := range roles {
				if claims.Role == role {
					allowed = true
					break
				}
			}

			if !allowed {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusForbidden)
				json.NewEncoder(w).Encode(map[string]string{"error": "forbidden"})
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}
