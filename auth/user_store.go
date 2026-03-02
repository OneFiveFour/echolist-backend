package auth

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"golang.org/x/crypto/bcrypt"
)

// bcryptCost is the bcrypt cost factor used for password hashing.
// It is a variable (not a constant) so that tests can lower it for speed.
var bcryptCost = 10

// User represents a stored user credential.
type User struct {
	Username     string `json:"username"`
	PasswordHash string `json:"password_hash"`
}

// UserStore manages file-based user credential storage.
type UserStore struct {
	filePath string
	mu       sync.RWMutex
	users    []User
}

// NewUserStore creates a new UserStore that reads/writes to the given file path.
func NewUserStore(filePath string) *UserStore {
	return &UserStore{filePath: filePath}
}

// LoadOrInitialize reads the user store file, or creates it with a default
// admin user if it doesn't exist. Returns an error if the file exists but
// is malformed, or if the default password is empty when initialization is needed.
func (s *UserStore) LoadOrInitialize(defaultUser, defaultPassword string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	data, err := os.ReadFile(s.filePath)
	if err == nil {
		// File exists — parse it
		var users []User
		if err := json.Unmarshal(data, &users); err != nil {
			return fmt.Errorf("failed to parse user store %s: %w", s.filePath, err)
		}
		s.users = users
		return nil
	}

	if !os.IsNotExist(err) {
		return fmt.Errorf("failed to read user store %s: %w", s.filePath, err)
	}

	// File doesn't exist — create default user
	if defaultPassword == "" {
		return fmt.Errorf("AUTH_DEFAULT_PASSWORD is required for first-run user creation")
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(defaultPassword), bcryptCost)
	if err != nil {
		return fmt.Errorf("failed to hash default password: %w", err)
	}

	s.users = []User{{Username: defaultUser, PasswordHash: string(hash)}}

	data, err = json.MarshalIndent(s.users, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal user store: %w", err)
	}

	// Ensure parent directory exists
	if dir := filepath.Dir(s.filePath); dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0700); err != nil {
			return fmt.Errorf("failed to create directory for user store: %w", err)
		}
	}

	if err := os.WriteFile(s.filePath, data, 0600); err != nil {
		return fmt.Errorf("failed to write user store %s: %w", s.filePath, err)
	}

	return nil
}

// Authenticate checks the provided password against the stored bcrypt hash
// for the given username. Returns the User if valid, or an error.
func (s *UserStore) Authenticate(username, password string) (*User, error) {
	user, err := s.getUser(username)
	if err != nil {
		return nil, err
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password)); err != nil {
		return nil, fmt.Errorf("invalid credentials")
	}

	return user, nil
}

// HasUsers reports whether the store contains at least one user.
// Intended for health-check use.
func (s *UserStore) HasUsers() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.users) > 0
}

// getUser returns the user record for the given username, or an error if not found.
func (s *UserStore) getUser(username string) (*User, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	for _, u := range s.users {
		if u.Username == username {
			return &u, nil
		}
	}

	return nil, fmt.Errorf("invalid credentials")
}
