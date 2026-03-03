package common

import (
	"context"
	"log/slog"
	"time"

	"connectrpc.com/connect"

	"echolist-backend/auth"
)

// NewRequestLoggingInterceptor returns a Connect interceptor that logs every
// unary RPC with method, duration, status code, and authenticated user.
func NewRequestLoggingInterceptor(logger *slog.Logger) connect.UnaryInterceptorFunc {
	return connect.UnaryInterceptorFunc(func(next connect.UnaryFunc) connect.UnaryFunc {
		return connect.UnaryFunc(func(ctx context.Context, req connect.AnyRequest) (connect.AnyResponse, error) {
			start := time.Now()
			procedure := req.Spec().Procedure

			resp, err := next(ctx, req)

			duration := time.Since(start)
			code := codeOf(err)
			username, _ := auth.UsernameFromContext(ctx)

			attrs := []slog.Attr{
				slog.String("procedure", procedure),
				slog.Duration("duration", duration),
				slog.String("code", code),
			}
			if username != "" {
				attrs = append(attrs, slog.String("user", username))
			}

			if err != nil {
				attrs = append(attrs, slog.String("error", err.Error()))
				logger.LogAttrs(ctx, slog.LevelError, "rpc error", attrs...)
			} else {
				logger.LogAttrs(ctx, slog.LevelInfo, "rpc ok", attrs...)
			}

			return resp, err
		})
	})
}

func codeOf(err error) string {
	if err == nil {
		return "ok"
	}
	if connectErr, ok := err.(*connect.Error); ok {
		return connectErr.Code().String()
	}
	return "unknown"
}
