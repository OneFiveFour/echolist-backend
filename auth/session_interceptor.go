package auth

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"connectrpc.com/connect"
	"echolist-backend/database"
)

type sessionContextKey struct{}

// NewSessionAuthInterceptor requires persistent sessions, including for access
// tokens. Revocation takes effect on the next request, without waiting for expiry.
func NewSessionAuthInterceptor(tokens *TokenService, db *database.Database, logger *slog.Logger) connect.UnaryInterceptorFunc {
	validateJWT := NewAuthInterceptor(tokens, logger)
	return func(next connect.UnaryFunc) connect.UnaryFunc {
		return validateJWT(func(ctx context.Context, req connect.AnyRequest) (connect.AnyResponse, error) {
			if publicProcedures[req.Spec().Procedure] {
				return next(ctx, req)
			}
			// The outer interceptor already validated the Bearer token and type.
			claims, err := tokens.ValidateToken(req.Header().Get("Authorization")[7:])
			if err != nil {
				return nil, connect.NewError(connect.CodeUnauthenticated, fmt.Errorf("invalid token"))
			}
			_, err = db.GetActiveSession(ctx, claims.SessionID, claims.Username, time.Now().UnixMilli())
			if errors.Is(err, database.ErrInvalidSession) {
				return nil, connect.NewError(connect.CodeUnauthenticated, fmt.Errorf("invalid or expired session"))
			}
			if err != nil {
				logger.Error("session validation failed", "error", err)
				return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("session validation failed"))
			}
			ctx = context.WithValue(ctx, sessionContextKey{}, claims.SessionID)
			return next(ctx, req)
		})
	}
}
