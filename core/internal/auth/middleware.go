package auth

import (
	"context"
	"net/http"
	"strings"
)

type contextKey string

const (
	UserClaimsKey contextKey = "user_claims"
)

type Middleware struct {
	tokenManager *TokenManager
}

func NewMiddleware(tm *TokenManager) *Middleware {
	return &Middleware{tokenManager: tm}
}

func (m *Middleware) Authenticate(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authHeader := r.Header.Get("Authorization")
		if authHeader == "" {
			http.Error(w, `{"success":false,"error":"unauthorized: missing authorization header"}`, http.StatusUnauthorized)
			return
		}

		parts := strings.Split(authHeader, " ")
		if len(parts) != 2 || strings.ToLower(parts[0]) != "bearer" {
			http.Error(w, `{"success":false,"error":"unauthorized: invalid bearer token format"}`, http.StatusUnauthorized)
			return
		}

		claims, err := m.tokenManager.VerifyToken(parts[1])
		if err != nil {
			http.Error(w, `{"success":false,"error":"unauthorized: `+err.Error()+`"}`, http.StatusUnauthorized)
			return
		}

		// Inject authenticated session claims into request context
		ctx := context.WithValue(r.Context(), UserClaimsKey, claims)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// GetClaims retrieves verified caller identity from request context
func GetClaims(ctx context.Context) (*SessionClaims, bool) {
	claims, ok := ctx.Value(UserClaimsKey).(*SessionClaims)
	return claims, ok
}
