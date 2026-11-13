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

func TestDeviceSessionsHTTPRevocationAndLegacyRejection(t *testing.T) {
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
	defer ts.Close()
	ctx := context.Background()
	login := func(name string) *authv1.LoginResponse {
		t.Helper()
		response, err := server.Login(ctx, &authv1.LoginRequest{Username: "alice", Password: "password123", DeviceName: name})
		if err != nil {
			t.Fatal(err)
		}
		return response
	}
	web := login("Web browser")
	android := login("Android")
	clientFor := func(token string) authv1connect.AuthServiceClient {
		return authv1connect.NewAuthServiceClient(ts.Client(), ts.URL, connect.WithInterceptors(connect.UnaryInterceptorFunc(func(next connect.UnaryFunc) connect.UnaryFunc {
			return func(ctx context.Context, req connect.AnyRequest) (connect.AnyResponse, error) {
				if token != "" {
					req.Header().Set("Authorization", "Bearer "+token)
				}
				return next(ctx, req)
			}
		})))
	}
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
			"--url", ts.URL, "--username", "alice", "list")
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
