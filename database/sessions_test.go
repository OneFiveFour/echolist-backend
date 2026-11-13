package database

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"sync"
	"testing"
)

func TestSessionRotationExpiryRecoveryAndReplay(t *testing.T) {
	db, err := New(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	ctx := context.Background()
	const now int64 = 1000000
	session := &DeviceSession{ID: "device", Username: "alice", DeviceName: "Android", CreatedAt: now, LastUsedAt: now, ExpiresAt: now + 100000, RefreshID: "original"}
	if err := db.CreateSession(ctx, session); err != nil {
		t.Fatal(err)
	}
	rotated, err := db.RotateSession(ctx, "device", "alice", "original", "new", now+1000, now+200000)
	if err != nil {
		t.Fatal(err)
	}
	if rotated.RefreshID != "new" || rotated.ExpiresAt != now+200000 {
		t.Fatalf("not rotated: %+v", rotated)
	}
	recovered, err := db.RotateSession(ctx, "device", "alice", "original", "unused", now+2000, now+300000)
	if err != nil {
		t.Fatal(err)
	}
	if *recovered != *rotated {
		t.Fatalf("retry modified session: %+v", recovered)
	}
	current, err := db.RotateSession(ctx, "device", "alice", "new", "unused", now+3000, now+300000)
	if err != nil || *current != *rotated {
		t.Fatalf("current token changed during grace: %+v, %v", current, err)
	}
	_, err = db.RotateSession(ctx, "device", "alice", "original", "unused", now+62000, now+400000)
	if !errors.Is(err, ErrInvalidSession) {
		t.Fatalf("expected replay rejection: %v", err)
	}
	if _, err = db.GetActiveSession(ctx, "device", "alice", now+62001); !errors.Is(err, ErrInvalidSession) {
		t.Fatalf("replay did not revoke access: %v", err)
	}
}

func TestSessionsPersistenceIsolationRevocationAndInactivity(t *testing.T) {
	path := filepath.Join(t.TempDir(), "test.db")
	db, err := New(path)
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	for _, s := range []*DeviceSession{
		{ID: "a1", Username: "alice", DeviceName: "Web", CreatedAt: 1, LastUsedAt: 1, ExpiresAt: 100, RefreshID: "one"},
		{ID: "a2", Username: "alice", DeviceName: "Android", CreatedAt: 1, LastUsedAt: 1, ExpiresAt: 100, RefreshID: "two"},
		{ID: "b1", Username: "bob", DeviceName: "Web", CreatedAt: 1, LastUsedAt: 1, ExpiresAt: 100, RefreshID: "three"},
	} {
		if err = db.CreateSession(ctx, s); err != nil {
			t.Fatal(err)
		}
	}
	if err = db.Close(); err != nil {
		t.Fatal(err)
	}
	db, err = New(path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err = db.RevokeSession(ctx, "b1", "alice", 2); err != nil {
		t.Fatal(err)
	}
	if _, err = db.GetActiveSession(ctx, "b1", "bob", 3); err != nil {
		t.Fatalf("cross-account revocation: %v", err)
	}
	if _, err = db.GetActiveSession(ctx, "b1", "alice", 3); !errors.Is(err, ErrInvalidSession) {
		t.Fatal("cross-account access")
	}
	if err = db.RevokeSession(ctx, "a1", "alice", 2); err != nil {
		t.Fatal(err)
	}
	if err = db.RevokeSession(ctx, "a1", "alice", 3); err != nil {
		t.Fatal(err)
	}
	sessions, err := db.ListSessions(ctx, "alice", 4)
	if err != nil || len(sessions) != 1 || sessions[0].ID != "a2" {
		t.Fatalf("list: %+v %v", sessions, err)
	}
	if _, err = db.GetActiveSession(ctx, "a2", "alice", 100); !errors.Is(err, ErrInvalidSession) {
		t.Fatal("expired session accepted")
	}
	if _, err = db.RotateSession(ctx, "a2", "alice", "two", "new", 100, 200); !errors.Is(err, ErrInvalidSession) {
		t.Fatal("expired session renewed")
	}
}

func TestSessionCanRotateAgainAfterGrace(t *testing.T) {
	db, err := New(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	ctx := context.Background()
	s := &DeviceSession{ID: "a", Username: "alice", DeviceName: "Web", CreatedAt: 1, LastUsedAt: 1, ExpiresAt: 1000000, RefreshID: "one"}
	if err = db.CreateSession(ctx, s); err != nil {
		t.Fatal(err)
	}
	if _, err = db.RotateSession(ctx, "a", "alice", "one", "two", 2, 2000000); err != nil {
		t.Fatal(err)
	}
	next, err := db.RotateSession(ctx, "a", "alice", "two", "three", 60002, 3000000)
	if err != nil || next.RefreshID != "three" || next.ExpiresAt != 3000000 {
		t.Fatalf("second rotation: %+v %v", next, err)
	}
}

func TestConcurrentSessionRefreshesRecoverSameToken(t *testing.T) {
	db, err := New(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	ctx := context.Background()
	s := &DeviceSession{ID: "a", Username: "alice", DeviceName: "Web", CreatedAt: 1, LastUsedAt: 1, ExpiresAt: 1000000, RefreshID: "one"}
	if err = db.CreateSession(ctx, s); err != nil {
		t.Fatal(err)
	}
	var group sync.WaitGroup
	ids := make(chan string, 12)
	for i := 0; i < 12; i++ {
		group.Add(1)
		go func(i int) {
			defer group.Done()
			rotated, err := db.RotateSession(ctx, "a", "alice", "one", fmt.Sprintf("next-%d", i), 2, 2000000)
			if err != nil {
				t.Errorf("concurrent refresh: %v", err)
				return
			}
			ids <- rotated.RefreshID
		}(i)
	}
	group.Wait()
	close(ids)
	var expected string
	for id := range ids {
		if expected == "" {
			expected = id
		}
		if expected != id {
			t.Errorf("rotation race: %s != %s", id, expected)
		}
	}
}
