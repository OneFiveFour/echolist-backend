package auth

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"connectrpc.com/connect"
	authv1 "echolist-backend/proto/gen/auth/v1"
	"echolist-backend/proto/gen/auth/v1/authv1connect"
)

// sessionTestEnv is an HTTP test server running the AuthService behind the
// session interceptor, with a single user "alice".
type sessionTestEnv struct {
	t      *testing.T
	server *AuthServer
	tokens *TokenService
	url    string
	client *http.Client
}

func newSessionTestEnv(t *testing.T) *sessionTestEnv {
	t.Helper()
	users := NewUserStore(filepath.Join(t.TempDir(), "users.json"))
	if err := users.LoadOrInitialize("alice", "password123"); err != nil {
		t.Fatal(err)
	}
	tokens := NewTokenService("a-signing-key-with-at-least-32-bytes", 15*time.Minute, 90*24*time.Hour)
	server := newTestAuthServer(t, users, tokens)
	path, handler := authv1connect.NewAuthServiceHandler(server, connect.WithInterceptors(NewSessionAuthInterceptor(tokens, server.db, nopLogger())))
	mux := http.NewServeMux()
	mux.Handle(path, handler)
	ts := httptest.NewServer(mux)
	t.Cleanup(ts.Close)
	return &sessionTestEnv{t: t, server: server, tokens: tokens, url: ts.URL, client: ts.Client()}
}

func (e *sessionTestEnv) login(name string) *authv1.LoginResponse {
	e.t.Helper()
	response, err := e.server.Login(context.Background(), &authv1.LoginRequest{Username: "alice", Password: "password123", DeviceName: name})
	if err != nil {
		e.t.Fatal(err)
	}
	return response
}

// clientFor returns an AuthService client sending the given access token.
// An empty token sends no Authorization header.
func (e *sessionTestEnv) clientFor(token string) authv1connect.AuthServiceClient {
	return authv1connect.NewAuthServiceClient(e.client, e.url, connect.WithInterceptors(connect.UnaryInterceptorFunc(func(next connect.UnaryFunc) connect.UnaryFunc {
		return func(ctx context.Context, req connect.AnyRequest) (connect.AnyResponse, error) {
			if token != "" {
				req.Header().Set("Authorization", "Bearer "+token)
			}
			return next(ctx, req)
		}
	})))
}

func TestDeviceSessionsHTTPRevocationAndLegacyRejection(t *testing.T) {
	env := newSessionTestEnv(t)
	server, tokens, clientFor, login := env.server, env.tokens, env.clientFor, env.login
	ctx := context.Background()
	web := login("Web browser")
	android := login("Android")
	client := clientFor(web.AccessToken)
	list := func(token string) (*authv1.ListSessionsResponse, error) {
		return clientFor(token).ListSessions(ctx, &authv1.ListSessionsRequest{})
	}
	response, err := list(web.AccessToken)
	if err != nil {
		t.Fatal(err)
	}
	if len(response.Sessions) != 2 {
		t.Fatalf("expected two devices: %+v", response)
	}
	var androidID string
	current := 0
	for _, s := range response.Sessions {
		if s.IsCurrent {
			current++
		}
		if s.DeviceName == "Android" {
			androidID = s.Id
		}
		if delta := s.ExpiresAt - s.CreatedAt; delta != int64((90*24*time.Hour)/time.Millisecond) {
			t.Fatalf("TTL: %d", delta)
		}
	}
	if androidID == "" || current != 1 {
		t.Fatal("missing session metadata")
	}
	if python, err := exec.LookPath("python3"); err == nil {
		commandCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
		defer cancel()
		// Stub the prompt so tests never read a developer's controlling terminal.
		command := exec.CommandContext(commandCtx, python, "-c",
			"import getpass, runpy; getpass.getpass = lambda _: 'password123'; runpy.run_path('../scripts/sessions.py', run_name='__main__')",
			"--url", env.url, "--username", "alice", "list")
		output, err := command.CombinedOutput()
		if err != nil {
			t.Fatalf("management CLI: %v, %s", err, output)
		}
		if !strings.Contains(string(output), "Android") || !strings.Contains(string(output), "Web browser") {
			t.Fatalf("CLI missing devices: %s", output)
		}
		active, err := server.db.ListSessions(ctx, "alice", time.Now().UnixMilli())
		if err != nil || len(active) != 2 {
			t.Fatalf("management CLI left a live session: %v", err)
		}
	}
	if _, err := client.RevokeSession(ctx, &authv1.RevokeSessionRequest{SessionId: androidID}); err != nil {
		t.Fatal(err)
	}
	if _, err := list(android.AccessToken); connect.CodeOf(err) != connect.CodeUnauthenticated {
		t.Fatalf("revoked access still accepted: %v", err)
	}
	if _, err := server.RefreshToken(ctx, refreshTokenRequest(android.RefreshToken)); connect.CodeOf(err) != connect.CodeUnauthenticated {
		t.Fatalf("revoked refresh still accepted: %v", err)
	}
	response, err = list(web.AccessToken)
	if err != nil || len(response.Sessions) != 1 {
		t.Fatalf("other device affected: %v", err)
	}
	refreshed, err := server.RefreshToken(ctx, refreshTokenRequest(web.RefreshToken))
	if err != nil {
		t.Fatal(err)
	}
	if refreshed.RefreshToken == web.RefreshToken || refreshed.RefreshToken == "" {
		t.Fatal("refresh token was not rotated")
	}
	if _, err := list(refreshed.AccessToken); err != nil {
		t.Fatal(err)
	}
	recovered, err := server.RefreshToken(ctx, refreshTokenRequest(web.RefreshToken))
	if err != nil || recovered.RefreshToken != refreshed.RefreshToken {
		t.Fatalf("retry did not recover rotation: %v", err)
	}
	legacy, _ := tokens.GenerateAccessToken("alice")
	if _, err := list(legacy); connect.CodeOf(err) != connect.CodeUnauthenticated {
		t.Fatalf("sessionless legacy access accepted: %v", err)
	}
	legacyRefresh, _ := tokens.GenerateRefreshToken("alice")
	if _, err := server.RefreshToken(ctx, refreshTokenRequest(legacyRefresh)); connect.CodeOf(err) != connect.CodeUnauthenticated {
		t.Fatalf("legacy refresh accepted: %v", err)
	}
	if _, err := clientFor("").ListSessions(ctx, &authv1.ListSessionsRequest{}); connect.CodeOf(err) != connect.CodeUnauthenticated {
		t.Fatalf("anonymous list accepted: %v", err)
	}
}

func TestLogoutRevokesSessionByRefreshToken(t *testing.T) {
	env := newSessionTestEnv(t)
	ctx := context.Background()
	web := env.login("Web browser")
	android := env.login("Android")
	// Logout must work without any valid access token.
	anonymous := env.clientFor("")
	if _, err := anonymous.Logout(ctx, &authv1.LogoutRequest{RefreshToken: web.RefreshToken}); err != nil {
		t.Fatal(err)
	}
	if _, err := env.clientFor(web.AccessToken).ListSessions(ctx, &authv1.ListSessionsRequest{}); connect.CodeOf(err) != connect.CodeUnauthenticated {
		t.Fatalf("access token still accepted after logout: %v", err)
	}
	if _, err := env.server.RefreshToken(ctx, refreshTokenRequest(web.RefreshToken)); connect.CodeOf(err) != connect.CodeUnauthenticated {
		t.Fatalf("refresh token still accepted after logout: %v", err)
	}
	if _, err := env.clientFor(android.AccessToken).ListSessions(ctx, &authv1.ListSessionsRequest{}); err != nil {
		t.Fatalf("other device affected by logout: %v", err)
	}
	// Repeating the logout is idempotent: success still means "session is revoked".
	if _, err := anonymous.Logout(ctx, &authv1.LogoutRequest{RefreshToken: web.RefreshToken}); err != nil {
		t.Fatalf("repeated logout failed: %v", err)
	}
}

func TestLogoutRejectsInvalidTokens(t *testing.T) {
	env := newSessionTestEnv(t)
	ctx := context.Background()
	session := env.login("Web browser")
	legacyRefresh, _ := env.tokens.GenerateRefreshToken("alice")
	for name, token := range map[string]string{
		"empty":        "",
		"garbage":      "not-a-token",
		"access token": session.AccessToken,
		"legacy":       legacyRefresh,
	} {
		if _, err := env.clientFor("").Logout(ctx, &authv1.LogoutRequest{RefreshToken: token}); connect.CodeOf(err) != connect.CodeUnauthenticated {
			t.Fatalf("%s: logout accepted: %v", name, err)
		}
	}
	// None of the rejected attempts may have revoked the session.
	if _, err := env.clientFor(session.AccessToken).ListSessions(ctx, &authv1.ListSessionsRequest{}); err != nil {
		t.Fatalf("session revoked by invalid logout: %v", err)
	}
}

func TestLogoutWithRotatedRefreshToken(t *testing.T) {
	env := newSessionTestEnv(t)
	ctx := context.Background()
	session := env.login("Web browser")
	rotated, err := env.server.RefreshToken(ctx, refreshTokenRequest(session.RefreshToken))
	if err != nil {
		t.Fatal(err)
	}
	// The superseded refresh token still identifies the session and revokes it.
	if _, err := env.clientFor("").Logout(ctx, &authv1.LogoutRequest{RefreshToken: session.RefreshToken}); err != nil {
		t.Fatalf("logout with superseded refresh token failed: %v", err)
	}
	if _, err := env.clientFor(rotated.AccessToken).ListSessions(ctx, &authv1.ListSessionsRequest{}); connect.CodeOf(err) != connect.CodeUnauthenticated {
		t.Fatalf("session still active after logout: %v", err)
	}
	if _, err := env.server.RefreshToken(ctx, refreshTokenRequest(rotated.RefreshToken)); connect.CodeOf(err) != connect.CodeUnauthenticated {
		t.Fatalf("current refresh token still accepted after logout: %v", err)
	}
}

func TestLogoutRejectsExpiredRefreshToken(t *testing.T) {
	env := newSessionTestEnv(t)
	ctx := context.Background()
	session := env.login("Web browser")
	claims, err := env.tokens.ValidateToken(session.RefreshToken)
	if err != nil {
		t.Fatal(err)
	}
	stored, err := env.server.db.GetActiveSession(ctx, claims.SessionID, "alice", time.Now().UnixMilli())
	if err != nil {
		t.Fatal(err)
	}
	past := time.Now().Add(-time.Hour)
	expired, err := env.tokens.signSessionToken(stored, "refresh", stored.RefreshID, past.Add(-time.Hour), past)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := env.clientFor("").Logout(ctx, &authv1.LogoutRequest{RefreshToken: expired}); connect.CodeOf(err) != connect.CodeUnauthenticated {
		t.Fatalf("expired refresh token accepted: %v", err)
	}
	// The rejected attempt must not have revoked the session.
	if _, err := env.clientFor(session.AccessToken).ListSessions(ctx, &authv1.ListSessionsRequest{}); err != nil {
		t.Fatalf("session revoked by expired-token logout: %v", err)
	}
}
