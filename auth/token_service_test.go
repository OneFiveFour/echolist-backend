package auth

import (
	"testing"
	"time"

	"pgregory.net/rapid"
)

// secretGen generates non-empty JWT secrets (16-64 chars).
func secretGen() *rapid.Generator[string] {
	return rapid.StringMatching(`[a-zA-Z0-9!@#$%^&*]{16,64}`)
}

// Property 3: Token generation round-trip with claims
// For any username string and JWT secret, generating an access token and then
// validating it with the same secret should return claims containing that exact
// username and a valid expiration time in the future.
// **Validates: Requirements 2.5, 2.6**
func TestProperty3_TokenGenerationRoundTrip(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		username := usernameGen().Draw(rt, "username")
		secret := secretGen().Draw(rt, "secret")

		accessTtl := 15 * time.Minute
		refreshTtl := 7 * 24 * time.Hour

		svc := NewTokenService(secret, accessTtl, refreshTtl)

		beforeGenerate := time.Now()

		tokenStr, err := svc.GenerateAccessToken(username)
		if err != nil {
			rt.Fatalf("GenerateAccessToken failed: %v", err)
		}

		claims, err := svc.ValidateToken(tokenStr)
		if err != nil {
			rt.Fatalf("ValidateToken failed: %v", err)
		}

		// Username claim must match
		if claims.Username != username {
			rt.Fatalf("username mismatch: got %q, want %q", claims.Username, username)
		}

		// Subject claim must match username
		if claims.Subject != username {
			rt.Fatalf("subject mismatch: got %q, want %q", claims.Subject, username)
		}

		// Expiration must be in the future
		if claims.ExpiresAt == nil {
			rt.Fatal("ExpiresAt claim is nil")
		}
		if !claims.ExpiresAt.Time.After(beforeGenerate) {
			rt.Fatalf("token expiry %v is not after generation time %v", claims.ExpiresAt.Time, beforeGenerate)
		}

		// IssuedAt must be set
		if claims.IssuedAt == nil {
			rt.Fatal("IssuedAt claim is nil")
		}
	})
}

// Test expired token returns error
// Validates: Requirement 3.2
func TestValidateToken_Expired(t *testing.T) {
	svc := NewTokenService("test-secret-key-1234", 1*time.Nanosecond, 1*time.Nanosecond)

	tokenStr, err := svc.GenerateAccessToken("alice")
	if err != nil {
		t.Fatalf("GenerateAccessToken failed: %v", err)
	}

	// Wait for the token to expire
	time.Sleep(2 * time.Millisecond)

	_, err = svc.ValidateToken(tokenStr)
	if err == nil {
		t.Fatal("expected error for expired token, got nil")
	}
}

// Test token signed with wrong secret returns error
// Validates: Requirement 3.3
func TestValidateToken_WrongSecret(t *testing.T) {
	svc1 := NewTokenService("secret-one-abcdefgh", 15*time.Minute, 7*24*time.Hour)
	svc2 := NewTokenService("secret-two-zyxwvuts", 15*time.Minute, 7*24*time.Hour)

	tokenStr, err := svc1.GenerateAccessToken("bob")
	if err != nil {
		t.Fatalf("GenerateAccessToken failed: %v", err)
	}

	_, err = svc2.ValidateToken(tokenStr)
	if err == nil {
		t.Fatal("expected error for token signed with different secret, got nil")
	}
}
