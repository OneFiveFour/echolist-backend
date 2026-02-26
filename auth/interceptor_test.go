package auth

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"connectrpc.com/connect"
	notesv1 "echolist-backend/proto/gen/notes/v1"
	"echolist-backend/proto/gen/notes/v1/notesv1connect"
	authv1 "echolist-backend/proto/gen/auth/v1"
	"echolist-backend/proto/gen/auth/v1/authv1connect"
	"pgregory.net/rapid"
)

// contextCapturingNotesHandler captures the username injected by the interceptor.
type contextCapturingNotesHandler struct {
	notesv1connect.UnimplementedNoteServiceHandler
	capturedUsername string
	capturedOk      bool
}

func (h *contextCapturingNotesHandler) ListNotes(ctx context.Context, _ *notesv1.ListNotesRequest) (*notesv1.ListNotesResponse, error) {
	h.capturedUsername, h.capturedOk = UsernameFromContext(ctx)
	return &notesv1.ListNotesResponse{}, nil
}

// authHeaderInterceptor is a client-side interceptor that sets the Authorization header.
func authHeaderInterceptor(token string) connect.UnaryInterceptorFunc {
	return connect.UnaryInterceptorFunc(func(next connect.UnaryFunc) connect.UnaryFunc {
		return connect.UnaryFunc(func(ctx context.Context, req connect.AnyRequest) (connect.AnyResponse, error) {
			if token != "" {
				req.Header().Set("Authorization", token)
			}
			return next(ctx, req)
		})
	})
}

// setupTestServer creates an httptest.Server with both auth and notes handlers,
// wired through the auth interceptor.
func setupTestServer(tokenService *TokenService) (*httptest.Server, *contextCapturingNotesHandler) {
	interceptor := NewAuthInterceptor(tokenService)
	interceptors := connect.WithInterceptors(interceptor)

	mux := http.NewServeMux()

	captureHandler := &contextCapturingNotesHandler{}
	notesPath, notesHandler := notesv1connect.NewNoteServiceHandler(captureHandler, interceptors)
	mux.Handle(notesPath, notesHandler)

	authPath, authHandler := authv1connect.NewAuthServiceHandler(
		authv1connect.UnimplementedAuthServiceHandler{},
		interceptors,
	)
	mux.Handle(authPath, authHandler)

	server := httptest.NewServer(mux)
	return server, captureHandler
}

// Property 5: Valid token passes interceptor with correct context
// For any valid access token containing a username, when the Auth_Interceptor
// processes a request to a protected endpoint with that token in the Authorization
// header, the request should proceed and the username extracted into the context
// should match the username in the token.
// **Validates: Requirements 3.1, 3.6**
func TestProperty5_ValidTokenPassesInterceptor(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		username := usernameGen().Draw(rt, "username")
		secret := secretGen().Draw(rt, "secret")

		tokenService := NewTokenService(secret, 15*time.Minute, 7*24*time.Hour)
		server, captureHandler := setupTestServer(tokenService)
		defer server.Close()

		// Generate a valid access token
		token, err := tokenService.GenerateAccessToken(username)
		if err != nil {
			rt.Fatalf("GenerateAccessToken failed: %v", err)
		}

		// Create a client with the auth header set
		client := notesv1connect.NewNoteServiceClient(
			http.DefaultClient,
			server.URL,
			connect.WithInterceptors(authHeaderInterceptor("Bearer "+token)),
		)

		// Reset capture state
		captureHandler.capturedOk = false
		captureHandler.capturedUsername = ""

		_, err = client.ListNotes(context.Background(), &notesv1.ListNotesRequest{})
		if err != nil {
			rt.Fatalf("ListNotes should succeed with valid token, got error: %v", err)
		}

		if !captureHandler.capturedOk {
			rt.Fatal("username was not injected into context")
		}
		if captureHandler.capturedUsername != username {
			rt.Fatalf("context username mismatch: got %q, want %q", captureHandler.capturedUsername, username)
		}
	})
}

// Property 6: Invalid tokens rejected by interceptor
// For any random string that is not a validly signed JWT, when the Auth_Interceptor
// processes a request to a protected endpoint with that string as the Bearer token,
// the interceptor should return an unauthenticated error.
// **Validates: Requirements 3.3**
func TestProperty6_InvalidTokensRejected(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		secret := secretGen().Draw(rt, "secret")
		// Generate a random non-JWT string as the invalid token
		invalidToken := rapid.StringMatching(`[a-zA-Z0-9._\-]{1,100}`).Draw(rt, "invalidToken")

		tokenService := NewTokenService(secret, 15*time.Minute, 7*24*time.Hour)
		server, _ := setupTestServer(tokenService)
		defer server.Close()

		client := notesv1connect.NewNoteServiceClient(
			http.DefaultClient,
			server.URL,
			connect.WithInterceptors(authHeaderInterceptor("Bearer "+invalidToken)),
		)

		_, err := client.ListNotes(context.Background(), &notesv1.ListNotesRequest{})
		if err == nil {
			rt.Fatal("expected error for invalid token, got nil")
		}

		if connect.CodeOf(err) != connect.CodeUnauthenticated {
			rt.Fatalf("expected Unauthenticated error, got code %v: %v", connect.CodeOf(err), err)
		}
	})
}

// Property 7: Public endpoints bypass authentication
// For any request to a public endpoint procedure (Login, RefreshToken), the
// Auth_Interceptor should allow the request to proceed regardless of whether
// an Authorization header is present or what value it contains.
// **Validates: Requirements 3.5**
func TestProperty7_PublicEndpointsBypassAuth(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		secret := secretGen().Draw(rt, "secret")
		// Generate a random auth header value — could be empty, garbage, or anything
		authValue := rapid.OneOf(
			rapid.Just(""),
			rapid.StringMatching(`[a-zA-Z0-9 ._\-]{0,100}`),
			rapid.Just("Bearer invalid-token"),
			rapid.Just("NotBearer something"),
		).Draw(rt, "authHeaderValue")

		tokenService := NewTokenService(secret, 15*time.Minute, 7*24*time.Hour)
		server, _ := setupTestServer(tokenService)
		defer server.Close()

		// Call Login (a public endpoint) — it should pass through the interceptor
		// regardless of auth header. The handler returns Unimplemented, which proves
		// the interceptor didn't block it.
		authClient := authv1connect.NewAuthServiceClient(
			http.DefaultClient,
			server.URL,
			connect.WithInterceptors(authHeaderInterceptor(authValue)),
		)

		_, err := authClient.Login(context.Background(), &authv1.LoginRequest{
			Username: "test",
			Password: "test",
		})

		// The error should be Unimplemented (from the stub handler), NOT Unauthenticated.
		// This proves the interceptor allowed the request through.
		if err == nil {
			rt.Fatal("expected Unimplemented error from stub handler, got nil")
		}
		if connect.CodeOf(err) == connect.CodeUnauthenticated {
			rt.Fatalf("public endpoint should not return Unauthenticated, got: %v", err)
		}
		if connect.CodeOf(err) != connect.CodeUnimplemented {
			rt.Fatalf("expected Unimplemented from stub handler, got code %v: %v", connect.CodeOf(err), err)
		}
	})
}

// --- Unit Tests for Auth Interceptor Edge Cases ---

// Test missing Authorization header returns Unauthenticated
// Validates: Requirement 3.4
func TestInterceptor_MissingAuthHeader(t *testing.T) {
	tokenService := NewTokenService("test-secret-1234abcd", 15*time.Minute, 7*24*time.Hour)
	server, _ := setupTestServer(tokenService)
	defer server.Close()

	// No auth header interceptor — sends request without Authorization
	client := notesv1connect.NewNoteServiceClient(http.DefaultClient, server.URL)

	_, err := client.ListNotes(context.Background(), &notesv1.ListNotesRequest{})
	if err == nil {
		t.Fatal("expected error for missing auth header, got nil")
	}
	if connect.CodeOf(err) != connect.CodeUnauthenticated {
		t.Fatalf("expected Unauthenticated, got %v: %v", connect.CodeOf(err), err)
	}
}

// Test malformed header (no "Bearer " prefix) returns InvalidArgument
// Validates: Requirement 3.3
func TestInterceptor_MalformedAuthHeader(t *testing.T) {
	tokenService := NewTokenService("test-secret-1234abcd", 15*time.Minute, 7*24*time.Hour)
	server, _ := setupTestServer(tokenService)
	defer server.Close()

	client := notesv1connect.NewNoteServiceClient(
		http.DefaultClient,
		server.URL,
		connect.WithInterceptors(authHeaderInterceptor("Token some-value")),
	)

	_, err := client.ListNotes(context.Background(), &notesv1.ListNotesRequest{})
	if err == nil {
		t.Fatal("expected error for malformed auth header, got nil")
	}
	if connect.CodeOf(err) != connect.CodeInvalidArgument {
		t.Fatalf("expected InvalidArgument, got %v: %v", connect.CodeOf(err), err)
	}
}

// Test expired token returns Unauthenticated
// Validates: Requirement 3.2
func TestInterceptor_ExpiredToken(t *testing.T) {
	tokenService := NewTokenService("test-secret-1234abcd", 1*time.Nanosecond, 7*24*time.Hour)

	token, err := tokenService.GenerateAccessToken("alice")
	if err != nil {
		t.Fatalf("GenerateAccessToken failed: %v", err)
	}

	// Wait for the token to expire
	time.Sleep(2 * time.Millisecond)

	server, _ := setupTestServer(tokenService)
	defer server.Close()

	client := notesv1connect.NewNoteServiceClient(
		http.DefaultClient,
		server.URL,
		connect.WithInterceptors(authHeaderInterceptor("Bearer "+token)),
	)

	_, err = client.ListNotes(context.Background(), &notesv1.ListNotesRequest{})
	if err == nil {
		t.Fatal("expected error for expired token, got nil")
	}
	if connect.CodeOf(err) != connect.CodeUnauthenticated {
		t.Fatalf("expected Unauthenticated, got %v: %v", connect.CodeOf(err), err)
	}
}

