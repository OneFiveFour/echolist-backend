package auth

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"connectrpc.com/connect"
	authv1 "echolist-backend/proto/gen/auth/v1"
	"pgregory.net/rapid"
)

// Property 4: Valid credentials produce valid tokens
// For any user in the UserStore with a known password, calling Login with
// the correct username and password should return an access token and a
// refresh token that are both non-empty and valid (parseable with correct claims).
// **Validates: Requirements 2.1**
func TestProperty4_ValidCredentialsProduceValidTokens(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		username := usernameGen().Draw(rt, "username")
		password := passwordGen().Draw(rt, "password")
		secret := rapid.StringMatching(`[a-zA-Z0-9]{16,32}`).Draw(rt, "secret")

		// Set up a UserStore with the generated user
		dir := t.TempDir()
		filePath := filepath.Join(dir, "users.json")

		store := NewUserStore(filePath)
		if err := store.LoadOrInitialize(username, password); err != nil {
			rt.Fatal(err)
		}

		tokenService := NewTokenService(secret, 15*time.Minute, 7*24*time.Hour)
		server := NewAuthServer(store, tokenService)

		resp, err := server.Login(nil, loginRequest(username, password))
		if err != nil {
			rt.Fatalf("Login failed for valid credentials: %v", err)
		}

		if resp.GetAccessToken() == "" {
			rt.Fatal("access token is empty")
		}
		if resp.GetRefreshToken() == "" {
			rt.Fatal("refresh token is empty")
		}

		// Validate access token has correct claims
		accessClaims, err := tokenService.ValidateToken(resp.GetAccessToken())
		if err != nil {
			rt.Fatalf("access token is not valid: %v", err)
		}
		if accessClaims.Username != username {
			rt.Fatalf("access token username: got %q, want %q", accessClaims.Username, username)
		}

		// Validate refresh token has correct claims
		refreshClaims, err := tokenService.ValidateToken(resp.GetRefreshToken())
		if err != nil {
			rt.Fatalf("refresh token is not valid: %v", err)
		}
		if refreshClaims.Username != username {
			rt.Fatalf("refresh token username: got %q, want %q", refreshClaims.Username, username)
		}
	})
}

// Property 8: Valid refresh token produces new access token
// For any valid refresh token containing a username, calling RefreshToken
// should return a new non-empty access token that, when validated, contains
// the same username.
// **Validates: Requirements 4.1**
func TestProperty8_ValidRefreshTokenProducesNewAccessToken(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		username := usernameGen().Draw(rt, "username")
		secret := rapid.StringMatching(`[a-zA-Z0-9]{16,32}`).Draw(rt, "secret")

		tokenService := NewTokenService(secret, 15*time.Minute, 7*24*time.Hour)

		// Generate a valid refresh token
		refreshToken, err := tokenService.GenerateRefreshToken(username)
		if err != nil {
			rt.Fatal(err)
		}

		// UserStore not needed for RefreshToken, but AuthServer requires it
		dir := t.TempDir()
		filePath := filepath.Join(dir, "users.json")
		store := NewUserStore(filePath)
		// Write an empty users file so LoadOrInitialize doesn't need a password
		if err := os.WriteFile(filePath, []byte("[]"), 0600); err != nil {
			rt.Fatal(err)
		}
		if err := store.LoadOrInitialize("", ""); err != nil {
			rt.Fatal(err)
		}

		server := NewAuthServer(store, tokenService)

		resp, err := server.RefreshToken(nil, refreshTokenRequest(refreshToken))
		if err != nil {
			rt.Fatalf("RefreshToken failed: %v", err)
		}

		if resp.GetAccessToken() == "" {
			rt.Fatal("new access token is empty")
		}

		claims, err := tokenService.ValidateToken(resp.GetAccessToken())
		if err != nil {
			rt.Fatalf("new access token is not valid: %v", err)
		}
		if claims.Username != username {
			rt.Fatalf("new access token username: got %q, want %q", claims.Username, username)
		}
	})
}


// --- Unit Tests ---

func TestLogin_InvalidUsername(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "users.json")

	store := NewUserStore(filePath)
	if err := store.LoadOrInitialize("admin", "password123"); err != nil {
		t.Fatal(err)
	}

	tokenService := NewTokenService("testsecret123456", 15*time.Minute, 7*24*time.Hour)
	server := NewAuthServer(store, tokenService)

	_, err := server.Login(nil, loginRequest("nonexistent", "password123"))
	if err == nil {
		t.Fatal("expected error for invalid username")
	}
	assertUnauthenticated(t, err)
}

func TestLogin_WrongPassword(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "users.json")

	store := NewUserStore(filePath)
	if err := store.LoadOrInitialize("admin", "password123"); err != nil {
		t.Fatal(err)
	}

	tokenService := NewTokenService("testsecret123456", 15*time.Minute, 7*24*time.Hour)
	server := NewAuthServer(store, tokenService)

	_, err := server.Login(nil, loginRequest("admin", "wrongpassword"))
	if err == nil {
		t.Fatal("expected error for wrong password")
	}
	assertUnauthenticated(t, err)
}

func TestLogin_ErrorMessagesIdentical(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "users.json")

	store := NewUserStore(filePath)
	if err := store.LoadOrInitialize("admin", "password123"); err != nil {
		t.Fatal(err)
	}

	tokenService := NewTokenService("testsecret123456", 15*time.Minute, 7*24*time.Hour)
	server := NewAuthServer(store, tokenService)

	_, errBadUser := server.Login(nil, loginRequest("nonexistent", "password123"))
	_, errBadPass := server.Login(nil, loginRequest("admin", "wrongpassword"))

	if errBadUser == nil || errBadPass == nil {
		t.Fatal("expected errors for both cases")
	}

	if errBadUser.Error() != errBadPass.Error() {
		t.Fatalf("error messages differ: bad user=%q, bad pass=%q", errBadUser.Error(), errBadPass.Error())
	}
}

func TestRefreshToken_ExpiredToken(t *testing.T) {
	// Create a token service with 0 TTL to produce already-expired tokens
	tokenService := NewTokenService("testsecret123456", 0, 0)
	refreshToken, err := tokenService.GenerateRefreshToken("admin")
	if err != nil {
		t.Fatal(err)
	}

	// Use a normal token service for the server (so it can validate properly)
	serverTokenService := NewTokenService("testsecret123456", 15*time.Minute, 7*24*time.Hour)

	dir := t.TempDir()
	filePath := filepath.Join(dir, "users.json")
	store := NewUserStore(filePath)
	if err := os.WriteFile(filePath, []byte("[]"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := store.LoadOrInitialize("", ""); err != nil {
		t.Fatal(err)
	}

	server := NewAuthServer(store, serverTokenService)

	// Wait briefly to ensure the token is expired
	time.Sleep(10 * time.Millisecond)

	_, err = server.RefreshToken(nil, refreshTokenRequest(refreshToken))
	if err == nil {
		t.Fatal("expected error for expired refresh token")
	}
	assertUnauthenticated(t, err)
}

func TestRefreshToken_MalformedToken(t *testing.T) {
	tokenService := NewTokenService("testsecret123456", 15*time.Minute, 7*24*time.Hour)

	dir := t.TempDir()
	filePath := filepath.Join(dir, "users.json")
	store := NewUserStore(filePath)
	if err := os.WriteFile(filePath, []byte("[]"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := store.LoadOrInitialize("", ""); err != nil {
		t.Fatal(err)
	}

	server := NewAuthServer(store, tokenService)

	_, err := server.RefreshToken(nil, refreshTokenRequest("not-a-valid-jwt"))
	if err == nil {
		t.Fatal("expected error for malformed refresh token")
	}
	assertUnauthenticated(t, err)
}

// --- Helpers ---

func loginRequest(username, password string) *authv1.LoginRequest {
	return &authv1.LoginRequest{
		Username: username,
		Password: password,
	}
}

func refreshTokenRequest(token string) *authv1.RefreshTokenRequest {
	return &authv1.RefreshTokenRequest{
		RefreshToken: token,
	}
}

func assertUnauthenticated(t *testing.T, err error) {
	t.Helper()
	connectErr, ok := err.(*connect.Error)
	if !ok {
		t.Fatalf("expected connect.Error, got %T: %v", err, err)
	}
	if connectErr.Code() != connect.CodeUnauthenticated {
		t.Fatalf("expected CodeUnauthenticated, got %v", connectErr.Code())
	}
}
