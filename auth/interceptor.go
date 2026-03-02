package auth

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"connectrpc.com/connect"
)

// contextKey is an unexported type for context keys in this package.
type contextKey struct{}

// usernameKey is the context key for the authenticated username.
var usernameKey = contextKey{}

// UsernameFromContext extracts the authenticated username from the context.
func UsernameFromContext(ctx context.Context) (string, bool) {
	username, ok := ctx.Value(usernameKey).(string)
	return username, ok
}

// publicProcedures is the set of fully-qualified procedure names that
// do not require authentication.
var publicProcedures = map[string]bool{
	"/auth.v1.AuthService/Login":        true,
	"/auth.v1.AuthService/RefreshToken": true,
}

// NewAuthInterceptor returns a connect.UnaryInterceptorFunc that validates
// JWT tokens for all non-public procedures.
func NewAuthInterceptor(tokenService *TokenService, logger *slog.Logger) connect.UnaryInterceptorFunc {
	return connect.UnaryInterceptorFunc(func(next connect.UnaryFunc) connect.UnaryFunc {
		return connect.UnaryFunc(func(ctx context.Context, req connect.AnyRequest) (connect.AnyResponse, error) {
			// Allow public procedures without authentication.
			if publicProcedures[req.Spec().Procedure] {
				return next(ctx, req)
			}

			// Extract Authorization header.
			authHeader := req.Header().Get("Authorization")
			if authHeader == "" {
				logger.Warn("missing authorization header", "procedure", req.Spec().Procedure)
				return nil, connect.NewError(connect.CodeUnauthenticated, fmt.Errorf("missing authorization header"))
			}

			// Validate Bearer prefix.
			if !strings.HasPrefix(authHeader, "Bearer ") {
				logger.Warn("malformed authorization header", "procedure", req.Spec().Procedure)
				return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("malformed authorization header, expected: Bearer <token>"))
			}

			tokenStr := strings.TrimPrefix(authHeader, "Bearer ")

			// Validate the token.
			claims, err := tokenService.ValidateToken(tokenStr)
			if err != nil {
				if strings.Contains(err.Error(), "expired") {
					logger.Debug("expired token", "procedure", req.Spec().Procedure)
					return nil, connect.NewError(connect.CodeUnauthenticated, fmt.Errorf("token expired"))
				}
				logger.Warn("invalid token", "procedure", req.Spec().Procedure, "error", err)
				return nil, connect.NewError(connect.CodeUnauthenticated, fmt.Errorf("invalid token"))
			}

			// Reject non-access tokens.
			if claims.TokenType != "access" {
				logger.Warn("invalid token type", "procedure", req.Spec().Procedure, "tokenType", claims.TokenType)
				return nil, connect.NewError(connect.CodeUnauthenticated, fmt.Errorf("invalid token type"))
			}

			// Inject username into context.
			ctx = context.WithValue(ctx, usernameKey, claims.Username)
			return next(ctx, req)
		})
	})
}
