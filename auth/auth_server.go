package auth

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"
	"unicode/utf8"

	"connectrpc.com/connect"
	"echolist-backend/database"
	authv1 "echolist-backend/proto/gen/auth/v1"
	"echolist-backend/proto/gen/auth/v1/authv1connect"
	"github.com/google/uuid"
)

type AuthServer struct {
	authv1connect.UnimplementedAuthServiceHandler
	userStore    *UserStore
	tokenService *TokenService
	db           *database.Database
	logger       *slog.Logger
}

func NewAuthServer(users *UserStore, tokens *TokenService, db *database.Database, logger *slog.Logger) *AuthServer {
	return &AuthServer{userStore: users, tokenService: tokens, db: db, logger: logger.With("service", "auth")}
}

func (s *AuthServer) internal(message string, err error) error {
	s.logger.Error(message, "error", err)
	return connect.NewError(connect.CodeInternal, fmt.Errorf("%s", message))
}

func invalidSession() error {
	return connect.NewError(connect.CodeUnauthenticated, fmt.Errorf("invalid or expired session"))
}

func (s *AuthServer) Login(ctx context.Context, req *authv1.LoginRequest) (*authv1.LoginResponse, error) {
	if _, err := s.userStore.Authenticate(req.GetUsername(), req.GetPassword()); err != nil {
		return nil, connect.NewError(connect.CodeUnauthenticated, fmt.Errorf("invalid credentials"))
	}
	name := strings.TrimSpace(req.GetDeviceName())
	if !utf8.ValidString(name) || len(name) > 200 || strings.ContainsAny(name, "\r\n\x00") {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("invalid device name (maximum 200 bytes)"))
	}
	if name == "" {
		name = "Unknown device"
	}
	now := time.Now()
	session := &database.DeviceSession{ID: uuid.NewString(), Username: req.GetUsername(), DeviceName: name,
		CreatedAt: now.UnixMilli(), LastUsedAt: now.UnixMilli(), ExpiresAt: now.Add(s.tokenService.refreshTokenTtl).UnixMilli(), RefreshID: uuid.NewString()}
	access, refresh, err := s.tokenService.SessionTokens(session)
	if err != nil {
		return nil, s.internal("failed to generate tokens", err)
	}
	if err := s.db.CreateSession(ctx, session); err != nil {
		return nil, s.internal("failed to create session", err)
	}
	return &authv1.LoginResponse{AccessToken: access, RefreshToken: refresh}, nil
}

func (s *AuthServer) RefreshToken(ctx context.Context, req *authv1.RefreshTokenRequest) (*authv1.RefreshTokenResponse, error) {
	claims, err := s.tokenService.ValidateToken(req.GetRefreshToken())
	if err != nil || claims.TokenType != "refresh" || claims.SessionID == "" || claims.ID == "" {
		return nil, invalidSession()
	}
	if _, err := s.userStore.getUser(claims.Username); err != nil {
		return nil, invalidSession()
	}
	now := time.Now()
	session, err := s.db.RotateSession(ctx, claims.SessionID, claims.Username, claims.ID, uuid.NewString(), now.UnixMilli(), now.Add(s.tokenService.refreshTokenTtl).UnixMilli())
	if errors.Is(err, database.ErrInvalidSession) {
		return nil, invalidSession()
	}
	if err != nil {
		return nil, s.internal("failed to refresh session", err)
	}
	access, refresh, err := s.tokenService.SessionTokens(session)
	if err != nil {
		return nil, s.internal("failed to generate tokens", err)
	}
	return &authv1.RefreshTokenResponse{AccessToken: access, RefreshToken: refresh}, nil
}

func (s *AuthServer) ListSessions(ctx context.Context, _ *authv1.ListSessionsRequest) (*authv1.ListSessionsResponse, error) {
	username, ok := UsernameFromContext(ctx)
	if !ok {
		return nil, invalidSession()
	}
	sessions, err := s.db.ListSessions(ctx, username, time.Now().UnixMilli())
	if err != nil {
		return nil, s.internal("failed to list sessions", err)
	}
	current, _ := ctx.Value(sessionContextKey{}).(string)
	response := &authv1.ListSessionsResponse{}
	for _, v := range sessions {
		response.Sessions = append(response.Sessions, &authv1.DeviceSession{Id: v.ID, DeviceName: v.DeviceName,
			CreatedAt: v.CreatedAt, LastUsedAt: v.LastUsedAt, ExpiresAt: v.ExpiresAt, IsCurrent: v.ID == current})
	}
	return response, nil
}

func (s *AuthServer) RevokeSession(ctx context.Context, req *authv1.RevokeSessionRequest) (*authv1.RevokeSessionResponse, error) {
	username, ok := UsernameFromContext(ctx)
	if !ok {
		return nil, invalidSession()
	}
	if _, err := uuid.Parse(req.GetSessionId()); err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("invalid session id"))
	}
	if err := s.db.RevokeSession(ctx, req.GetSessionId(), username, time.Now().UnixMilli()); err != nil {
		return nil, s.internal("failed to revoke session", err)
	}
	return &authv1.RevokeSessionResponse{}, nil
}
