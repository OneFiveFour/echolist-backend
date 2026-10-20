package main

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestCorsMiddlewareAllowsConfiguredOriginAndPreflight(t *testing.T) {
	nextCalled := false
	handler := corsMiddleware([]string{"https://echolist.onefivefour.net"}, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		nextCalled = true
		w.WriteHeader(http.StatusOK)
	}))

	request := httptest.NewRequest(http.MethodOptions, "/auth.v1.AuthService/Login", nil)
	request.Header.Set("Origin", "https://echolist.onefivefour.net")
	response := httptest.NewRecorder()

	handler.ServeHTTP(response, request)

	if response.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusNoContent)
	}
	if nextCalled {
		t.Fatal("preflight reached downstream handler")
	}
	if got := response.Header().Get("Access-Control-Allow-Origin"); got != "https://echolist.onefivefour.net" {
		t.Fatalf("Access-Control-Allow-Origin = %q", got)
	}
}

func TestCorsMiddlewareDoesNotAllowUnknownOrigin(t *testing.T) {
	handler := corsMiddleware([]string{"https://echolist.onefivefour.net"}, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	request := httptest.NewRequest(http.MethodPost, "/auth.v1.AuthService/Login", nil)
	request.Header.Set("Origin", "https://untrusted.example")
	response := httptest.NewRecorder()

	handler.ServeHTTP(response, request)

	if got := response.Header().Get("Access-Control-Allow-Origin"); got != "" {
		t.Fatalf("unexpected Access-Control-Allow-Origin = %q", got)
	}
}
