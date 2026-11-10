package auth

import (
	"fmt"
	"time"

	"echolist-backend/database"
	"github.com/golang-jwt/jwt/v5"
)

// TokenClaims represents the custom claims embedded in a JWT token.
type TokenClaims struct {
	Username  string `json:"username"`
	SessionID string `json:"sid,omitempty"`
	TokenType string `json:"type"`
	jwt.RegisteredClaims
}

// TokenService handles JWT token generation and validation.
type TokenService struct {
	secret          []byte
	accessTokenTtl  time.Duration
	refreshTokenTtl time.Duration
}

// NewTokenService creates a new TokenService with the given secret and TTLs.
func NewTokenService(secret string, accessTtl, refreshTtl time.Duration) *TokenService {
	return &TokenService{
		secret:          []byte(secret),
		accessTokenTtl:  accessTtl,
		refreshTokenTtl: refreshTtl,
	}
}

// GenerateAccessToken creates a signed JWT with the username claim and
// the configured access token expiry.
func (t *TokenService) GenerateAccessToken(username string) (string, error) {
	return t.generateToken(username, "access", t.accessTokenTtl)
}

// GenerateRefreshToken creates a signed JWT with the username claim and
// the configured refresh token expiry.
func (t *TokenService) GenerateRefreshToken(username string) (string, error) {
	return t.generateToken(username, "refresh", t.refreshTokenTtl)
}

// generateToken creates a signed JWT with the given username, token type, and TTL.
func (t *TokenService) generateToken(username, tokenType string, ttl time.Duration) (string, error) {
	now := time.Now()
	claims := TokenClaims{
		Username:  username,
		TokenType: tokenType,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   username,
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(ttl)),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := token.SignedString(t.secret)
	if err != nil {
		return "", fmt.Errorf("failed to sign token: %w", err)
	}
	return signed, nil
}

// ValidateToken parses and validates a JWT string. Returns the claims if
// the token is valid (correct signature, not expired), or an error.
func (t *TokenService) ValidateToken(tokenStr string) (*TokenClaims, error) {
	token, err := jwt.ParseWithClaims(tokenStr, &TokenClaims{}, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return t.secret, nil
	}, jwt.WithValidMethods([]string{"HS256"}), jwt.WithExpirationRequired())
	if err != nil {
		return nil, fmt.Errorf("invalid token: %w", err)
	}

	claims, ok := token.Claims.(*TokenClaims)
	if !ok || !token.Valid {
		return nil, fmt.Errorf("invalid token claims")
	}

	return claims, nil
}

// SessionTokens binds both tokens to a server-side session. Refresh tokens are
// reproducible so concurrent refreshes and lost responses recover the current token.
func (t *TokenService) SessionTokens(s *database.DeviceSession) (string, string, error) {
	now := time.Now()
	access, err := t.signSessionToken(s, "access", "", now, now.Add(t.accessTokenTtl))
	if err != nil {
		return "", "", err
	}
	refresh, err := t.signSessionToken(s, "refresh", s.RefreshID, time.UnixMilli(s.LastUsedAt), time.UnixMilli(s.ExpiresAt))
	return access, refresh, err
}

func (t *TokenService) signSessionToken(s *database.DeviceSession, kind, id string, issued, expires time.Time) (string, error) {
	claims := TokenClaims{Username: s.Username, SessionID: s.ID, TokenType: kind,
		RegisteredClaims: jwt.RegisteredClaims{Subject: s.Username, ID: id, IssuedAt: jwt.NewNumericDate(issued), ExpiresAt: jwt.NewNumericDate(expires)}}
	return jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(t.secret)
}
