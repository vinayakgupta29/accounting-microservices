package middleware

import (
	"context"
	"net/http"
	"strings"

	authv1 "github.com/accounting-microservices/gen/go/auth/v1"
)

type contextKey string

const (
	TenantIDContextKey contextKey = "tenant_id"
	UsernameContextKey contextKey = "username"
)

// AuthMiddleware creates an HTTP middleware that verifies bearer JWT tokens via AuthServiceClient.
func AuthMiddleware(authClient authv1.AuthServiceClient) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			authHeader := r.Header.Get("Authorization")
			if authHeader == "" {
				http.Error(w, `{"error":"Authorization header missing"}`, http.StatusUnauthorized)
				return
			}

			parts := strings.SplitN(authHeader, " ", 2)
			if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
				http.Error(w, `{"error":"Invalid authorization header format"}`, http.StatusUnauthorized)
				return
			}

			tokenString := strings.TrimSpace(parts[1])

			// Validate with Auth Service
			resp, err := authClient.ValidateToken(r.Context(), &authv1.ValidateTokenRequest{
				Token: tokenString,
			})
			if err != nil || resp == nil || !resp.GetValid() {
				http.Error(w, `{"error":"Unauthorized: invalid or expired token"}`, http.StatusUnauthorized)
				return
			}

			// Tenant ID is bound to the authenticated User ID
			ctx := context.WithValue(r.Context(), TenantIDContextKey, resp.GetUserId())
			ctx = context.WithValue(ctx, UsernameContextKey, resp.GetUsername())

			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// GetTenantID extracts tenant ID from request context.
func GetTenantID(ctx context.Context) string {
	if val, ok := ctx.Value(TenantIDContextKey).(string); ok {
		return val
	}
	return ""
}

// GetUsername extracts username from request context.
func GetUsername(ctx context.Context) string {
	if val, ok := ctx.Value(UsernameContextKey).(string); ok {
		return val
	}
	return ""
}
