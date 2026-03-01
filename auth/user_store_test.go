package auth

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"golang.org/x/crypto/bcrypt"
	"pgregory.net/rapid"
)

// usernameGen generates non-empty alphanumeric usernames.
func usernameGen() *rapid.Generator[string] {
	return rapid.StringMatching(`[a-zA-Z][a-zA-Z0-9]{0,19}`)
}

// passwordGen generates passwords (8-50 chars, avoiding false positives in bcrypt hash matching).
func passwordGen() *rapid.Generator[string] {
	return rapid.StringMatching(`[a-zA-Z0-9!@#$%^&*]{8,50}`)
}

// Property 1: User store round-trip
// For any list of users with valid usernames and bcrypt-hashed passwords,
// writing them to the UserStore file and then looking up each username
// should return the same user record (username and password hash).
// **Validates: Requirements 1.1, 1.5**
func TestProperty1_UserStoreRoundTrip(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		// Generate 1-5 unique users
		count := rapid.IntRange(1, 5).Draw(rt, "userCount")
		users := make([]User, 0, count)
		seen := make(map[string]bool)

		for i := 0; i < count; i++ {
			username := usernameGen().Draw(rt, "username")
			if seen[username] {
				continue
			}
			seen[username] = true

			password := passwordGen().Draw(rt, "password")
			hash, err := bcrypt.GenerateFromPassword([]byte(password), bcryptCost)
			if err != nil {
				rt.Fatal(err)
			}
			users = append(users, User{Username: username, PasswordHash: string(hash)})
		}

		if len(users) == 0 {
			return
		}

		// Write users to a temp file via UserStore
		dir := t.TempDir()
		filePath := filepath.Join(dir, "users.json")

		// Write the users JSON directly to simulate a pre-existing store
		data, err := json.MarshalIndent(users, "", "  ")
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filePath, data, 0600); err != nil {
			rt.Fatal(err)
		}

		// Load the store and verify round-trip
		store := NewUserStore(filePath)
		if err := store.LoadOrInitialize("", ""); err != nil {
			rt.Fatal(err)
		}

		for _, u := range users {
			got, err := store.getUser(u.Username)
			if err != nil {
				rt.Fatalf("getUser(%q) failed: %v", u.Username, err)
			}
			if got.Username != u.Username {
				rt.Fatalf("username mismatch: got %q, want %q", got.Username, u.Username)
			}
			if got.PasswordHash != u.PasswordHash {
				rt.Fatalf("password hash mismatch for user %q", u.Username)
			}
		}
	})
}

// Property 2: Bcrypt hashing invariant
// For any password string provided during user creation, the stored
// password_hash field must be a valid bcrypt hash with a cost factor
// of at least 10, and the original plaintext password must not appear
// anywhere in the stored data.
// **Validates: Requirements 1.2, 1.4**
func TestProperty2_BcryptHashingInvariant(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		password := passwordGen().Draw(rt, "password")
		username := usernameGen().Draw(rt, "username")

		dir := t.TempDir()
		filePath := filepath.Join(dir, "users.json")

		store := NewUserStore(filePath)
		if err := store.LoadOrInitialize(username, password); err != nil {
			rt.Fatal(err)
		}

		// Read the raw file and verify no plaintext password
		data, err := os.ReadFile(filePath)
		if err != nil {
			rt.Fatal(err)
		}

		dataStr := string(data)
		if len(password) > 0 && contains(dataStr, password) {
			rt.Fatalf("plaintext password %q found in stored data", password)
		}

		// Verify the stored hash is valid bcrypt with cost >= bcryptCost
		user, err := store.getUser(username)
		if err != nil {
			rt.Fatal(err)
		}

		cost, err := bcrypt.Cost([]byte(user.PasswordHash))
		if err != nil {
			rt.Fatalf("stored hash is not valid bcrypt: %v", err)
		}
		if cost < bcryptCost {
			rt.Fatalf("bcrypt cost %d is less than minimum %d", cost, bcryptCost)
		}

		// Verify the password actually matches the hash
		if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password)); err != nil {
			rt.Fatalf("password does not match stored hash: %v", err)
		}
	})
}

// contains checks if substr appears in s.
func contains(s, substr string) bool {
	return len(substr) > 0 && len(s) >= len(substr) && stringContains(s, substr)
}

func stringContains(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// --- Unit Tests ---

func TestLoadOrInitialize_CreatesDefaultUser(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "users.json")

	store := NewUserStore(filePath)
	if err := store.LoadOrInitialize("admin", "secret123"); err != nil {
		t.Fatalf("LoadOrInitialize failed: %v", err)
	}

	user, err := store.getUser("admin")
	if err != nil {
		t.Fatalf("getUser failed: %v", err)
	}
	if user.Username != "admin" {
		t.Fatalf("expected username 'admin', got %q", user.Username)
	}

	// Verify file was created
	if _, err := os.Stat(filePath); err != nil {
		t.Fatalf("users.json was not created: %v", err)
	}
}

func TestLoadOrInitialize_LoadsExistingFile(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "users.json")

	// Write a valid users file
	data := []byte(`[{"username":"testuser","password_hash":"$2a$10$abcdefghijklmnopqrstuuABCDEFGHIJKLMNOPQRSTUVWXYZ012"}]`)
	if err := os.WriteFile(filePath, data, 0600); err != nil {
		t.Fatal(err)
	}

	store := NewUserStore(filePath)
	if err := store.LoadOrInitialize("", ""); err != nil {
		t.Fatalf("LoadOrInitialize failed: %v", err)
	}

	user, err := store.getUser("testuser")
	if err != nil {
		t.Fatalf("getUser failed: %v", err)
	}
	if user.Username != "testuser" {
		t.Fatalf("expected 'testuser', got %q", user.Username)
	}
}

func TestLoadOrInitialize_ErrorOnMalformedJSON(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "users.json")

	if err := os.WriteFile(filePath, []byte(`{not valid json`), 0600); err != nil {
		t.Fatal(err)
	}

	store := NewUserStore(filePath)
	err := store.LoadOrInitialize("", "")
	if err == nil {
		t.Fatal("expected error for malformed JSON, got nil")
	}
}

func TestAuthenticate_WrongPassword(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "users.json")

	store := NewUserStore(filePath)
	if err := store.LoadOrInitialize("admin", "correctpassword"); err != nil {
		t.Fatal(err)
	}

	_, err := store.Authenticate("admin", "wrongpassword")
	if err == nil {
		t.Fatal("expected error for wrong password, got nil")
	}
}

func TestGetUser_NonExistent(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "users.json")

	store := NewUserStore(filePath)
	if err := store.LoadOrInitialize("admin", "password"); err != nil {
		t.Fatal(err)
	}

	_, err := store.getUser("nonexistent")
	if err == nil {
		t.Fatal("expected error for non-existent user, got nil")
	}
}
