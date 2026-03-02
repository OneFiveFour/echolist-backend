package main

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// ---------------------------------------------------------------------------
// envOrDefault
// ---------------------------------------------------------------------------

func TestEnvOrDefault_Set(t *testing.T) {
	t.Setenv("TEST_ENV_OR_DEFAULT", "custom")
	if got := envOrDefault("TEST_ENV_OR_DEFAULT", "fallback"); got != "custom" {
		t.Errorf("expected 'custom', got %q", got)
	}
}

func TestEnvOrDefault_Unset(t *testing.T) {
	// Ensure the variable is not set (t.Setenv restores after test).
	t.Setenv("TEST_ENV_OR_DEFAULT_UNSET", "")
	if got := envOrDefault("TEST_ENV_OR_DEFAULT_UNSET", "fallback"); got != "fallback" {
		t.Errorf("expected 'fallback', got %q", got)
	}
}

// ---------------------------------------------------------------------------
// parseDurationMinutesEnv
// ---------------------------------------------------------------------------

func TestParseDurationMinutesEnv_Default(t *testing.T) {
	t.Setenv("TEST_PARSE_DUR", "")
	d := parseDurationMinutesEnv("TEST_PARSE_DUR", 30)
	if d.Minutes() != 30 {
		t.Errorf("expected 30m, got %v", d)
	}
}

func TestParseDurationMinutesEnv_CustomValue(t *testing.T) {
	t.Setenv("TEST_PARSE_DUR2", "120")
	d := parseDurationMinutesEnv("TEST_PARSE_DUR2", 15)
	if d.Minutes() != 120 {
		t.Errorf("expected 120m, got %v", d)
	}
}

// ---------------------------------------------------------------------------
// maxBytesMiddleware
// ---------------------------------------------------------------------------

func TestMaxBytesMiddleware_AllowsSmallBody(t *testing.T) {
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			w.WriteHeader(http.StatusRequestEntityTooLarge)
			return
		}
		w.Write(body)
	})

	handler := maxBytesMiddleware(1024, inner)
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader("hello"))
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	if rec.Body.String() != "hello" {
		t.Fatalf("expected body 'hello', got %q", rec.Body.String())
	}
}

func TestMaxBytesMiddleware_RejectsOversizedBody(t *testing.T) {
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, err := io.ReadAll(r.Body)
		if err != nil {
			w.WriteHeader(http.StatusRequestEntityTooLarge)
			return
		}
		w.WriteHeader(http.StatusOK)
	})

	handler := maxBytesMiddleware(5, inner)
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader("this body is way too large"))
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("expected 413, got %d", rec.Code)
	}
}
