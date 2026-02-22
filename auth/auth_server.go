package auth

import (
	"context"
	"fmt"

	"connectrpc.com/connect"
	authv1 "echolist-backend/proto/gen/auth/v1"
	"echolist-backend/proto/gen/auth/v1/authv1connect"
)

// AuthServer implements the AuthService Connect handler.
type AuthServer struct {
	authv1connect.UnimplementedAuthServiceHandler
	userStore    *UserStore
	tokenService *TokenService
}

// NewAuthServer creates a new AuthServer with the given UserStore and TokenService.
func NewAuthServer(userStore *UserStore, tokenService *TokenService) *AuthServer {
	return &AuthServer{
		userStore:    userStore,
		tokenService: tokenService,
	}
}

// Login validates credentials and returns access + refresh tokens.
func (s *AuthServer) Login(_ context.Context, req *authv1.LoginRequest) (*authv1.LoginResponse, error) {
	_, err := s.userStore.Authenticate(req.GetUsername(), req.GetPassword())
	if err != nil {
		return nil, connect.NewError(connect.CodeUnauthenticated, fmt.Errorf("invalid credentials"))
	}

	accessToken, err := s.tokenService.GenerateAccessToken(req.GetUsername())
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to generate access token"))
	}

	refreshToken, err := s.tokenService.GenerateRefreshToken(req.GetUsername())
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to generate refresh token"))
	}

	return &authv1.LoginResponse{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
	}, nil
}

// RefreshToken validates a refresh token and returns a new access token.
func (s *AuthServer) RefreshToken(_ context.Context, req *authv1.RefreshTokenRequest) (*authv1.RefreshTokenResponse, error) {
	claims, err := s.tokenService.ValidateToken(req.GetRefreshToken())
	if err != nil {
		return nil, connect.NewError(connect.CodeUnauthenticated, fmt.Errorf("invalid credentials"))
	}

	accessToken, err := s.tokenService.GenerateAccessToken(claims.Username)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to generate access token"))
	}

	return &authv1.RefreshTokenResponse{
		AccessToken: accessToken,
	}, nil
}
