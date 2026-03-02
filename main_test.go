package main

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"echolist-backend/auth"
)

func TestLivez(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/livez", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	})

	req := httptest.NewRequest(http.MethodGet, "/livez", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	if rec.Body.String() != "ok" {
		t.Fatalf("expected body 'ok', got %q", rec.Body.String())
	}
}

func TestHealthz_AllHealthy(t *testing.T) {
	dataDir := t.TempDir()
	store := loadedUserStore(t)

	handler := healthzHandler(dataDir, store)
	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d; body: %s", rec.Code, rec.Body.String())
	}
	if rec.Body.String() != "ok" {
		t.Fatalf("expected body 'ok', got %q", rec.Body.String())
	}

	// Probe file should have been cleaned up
	if _, err := os.Stat(filepath.Join(dataDir, ".healthz_probe")); !os.IsNotExist(err) {
		t.Fatal("probe file was not cleaned up")
	}
}

func TestHealthz_MissingDataDir(t *testing.T) {
	dataDir := filepath.Join(t.TempDir(), "nonexistent")
	store := loadedUserStore(t)

	handler := healthzHandler(dataDir, store)
	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "data_dir:") {
		t.Fatalf("expected data_dir failure in body, got %q", rec.Body.String())
	}
}

func TestHealthz_ReadOnlyDataDir(t *testing.T) {
	dataDir := t.TempDir()
	if err := os.Chmod(dataDir, 0555); err != nil {
		t.Skipf("cannot set read-only permissions: %v", err)
	}
	t.Cleanup(func() { os.Chmod(dataDir, 0755) })

	store := loadedUserStore(t)

	handler := healthzHandler(dataDir, store)
	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "data_dir_write:") {
		t.Fatalf("expected data_dir_write failure in body, got %q", rec.Body.String())
	}
}

func TestHealthz_EmptyUserStore(t *testing.T) {
	dataDir := t.TempDir()
	// UserStore with no users loaded
	store := auth.NewUserStore(filepath.Join(t.TempDir(), "empty.json"))

	handler := healthzHandler(dataDir, store)
	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "user_store:") {
		t.Fatalf("expected user_store failure in body, got %q", rec.Body.String())
	}
}

// loadedUserStore creates a UserStore with one user for testing.
func loadedUserStore(t *testing.T) *auth.UserStore {
	t.Helper()
	dir := t.TempDir()
	store := auth.NewUserStore(filepath.Join(dir, "users.json"))
	if err := store.LoadOrInitialize("testuser", "testpass"); err != nil {
		t.Fatalf("failed to initialize user store: %v", err)
	}
	return store
}
