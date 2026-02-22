package folder

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"connectrpc.com/connect"

	folderv1 "echolist-backend/proto/gen/folder/v1"
	"pgregory.net/rapid"
)

// Property 4: Rename preserves contents
// Renaming a folder must not alter its children — the set of child names
// before and after the rename must be identical.
// **Validates: Requirements 2.1, 2.5**
func TestProperty4_RenamePreservesContents(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		oldName := folderNameGen().Draw(rt, "oldName")
		newName := folderNameGen().Draw(rt, "newName")

		// Ensure names differ (case-insensitive) so rename is meaningful
		if strings.EqualFold(oldName, newName) {
			rt.Skip("old and new names are case-insensitively equal")
		}

		dataDir := t.TempDir()
		domain := "notes"
		domainDir := filepath.Join(dataDir, domain)
		if err := os.MkdirAll(domainDir, 0755); err != nil {
			rt.Fatal(err)
		}

		srv := NewFolderServer(dataDir)

		// Create the folder
		_, err := srv.CreateFolder(context.Background(), &folderv1.CreateFolderRequest{
			Domain: domain,
			Name:   oldName,
		})
		if err != nil {
			rt.Fatalf("CreateFolder failed: %v", err)
		}

		// Add some children (files and subdirs)
		childFiles := []string{"a.md", "b.txt", "c.md"}
		childDirs := []string{"sub1", "sub2"}
		for _, f := range childFiles {
			if err := os.WriteFile(filepath.Join(domainDir, oldName, f), []byte("x"), 0644); err != nil {
				rt.Fatal(err)
			}
		}
		for _, d := range childDirs {
			if err := os.Mkdir(filepath.Join(domainDir, oldName, d), 0755); err != nil {
				rt.Fatal(err)
			}
		}

		// Record children before rename
		beforeEntries, _ := os.ReadDir(filepath.Join(domainDir, oldName))
		var beforeNames []string
		for _, e := range beforeEntries {
			beforeNames = append(beforeNames, e.Name())
		}
		sort.Strings(beforeNames)

		// Rename
		_, err = srv.RenameFolder(context.Background(), &folderv1.RenameFolderRequest{
			Domain:     domain,
			FolderPath: oldName,
			NewName:    newName,
		})
		if err != nil {
			rt.Fatalf("RenameFolder failed: %v", err)
		}

		// Record children after rename
		afterEntries, err := os.ReadDir(filepath.Join(domainDir, newName))
		if err != nil {
			rt.Fatalf("failed to read renamed folder: %v", err)
		}
		var afterNames []string
		for _, e := range afterEntries {
			afterNames = append(afterNames, e.Name())
		}
		sort.Strings(afterNames)

		// Children must be identical
		if len(beforeNames) != len(afterNames) {
			rt.Fatalf("child count changed: before=%d after=%d", len(beforeNames), len(afterNames))
		}
		for i := range beforeNames {
			if beforeNames[i] != afterNames[i] {
				rt.Fatalf("child mismatch at %d: before=%q after=%q", i, beforeNames[i], afterNames[i])
			}
		}
	})
}

// Property 5: Case-insensitive duplicate rejection on rename
// Renaming a folder to a name that case-insensitively matches an existing
// sibling must fail with AlreadyExists.
// **Validates: Requirements 2.2**
func TestProperty5_CaseInsensitiveDuplicateRejectionOnRename(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		nameA := folderNameGen().Draw(rt, "nameA")
		nameB := folderNameGen().Draw(rt, "nameB")

		// Ensure A and B are case-insensitively different so both can be created
		if strings.EqualFold(nameA, nameB) {
			rt.Skip("names are case-insensitively equal")
		}

		dataDir := t.TempDir()
		domain := "notes"
		domainDir := filepath.Join(dataDir, domain)
		if err := os.MkdirAll(domainDir, 0755); err != nil {
			rt.Fatal(err)
		}

		srv := NewFolderServer(dataDir)

		// Create folder A
		_, err := srv.CreateFolder(context.Background(), &folderv1.CreateFolderRequest{
			Domain: domain,
			Name:   nameA,
		})
		if err != nil {
			rt.Fatalf("CreateFolder A failed: %v", err)
		}

		// Create folder B
		_, err = srv.CreateFolder(context.Background(), &folderv1.CreateFolderRequest{
			Domain: domain,
			Name:   nameB,
		})
		if err != nil {
			rt.Fatalf("CreateFolder B failed: %v", err)
		}

		// Try to rename A to a case-variant of B — should fail
		variant := swapCase(nameB)
		_, err = srv.RenameFolder(context.Background(), &folderv1.RenameFolderRequest{
			Domain:     domain,
			FolderPath: nameA,
			NewName:    variant,
		})
		if err == nil {
			rt.Fatalf("expected AlreadyExists error renaming to case-variant %q of %q", variant, nameB)
		}
		var connErr *connect.Error
		if errors.As(err, &connErr) {
			if connErr.Code() != connect.CodeAlreadyExists {
				rt.Fatalf("expected AlreadyExists, got %v", connErr.Code())
			}
		}
	})
}

// Property 6: Delete removes folder and contents
// After deleting a folder, it must no longer exist on disk and must not
// appear in the parent listing.
// **Validates: Requirements 3.1, 3.4**
func TestProperty6_DeleteRemovesFolderAndContents(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		name := folderNameGen().Draw(rt, "folderName")

		dataDir := t.TempDir()
		domain := "notes"
		domainDir := filepath.Join(dataDir, domain)
		if err := os.MkdirAll(domainDir, 0755); err != nil {
			rt.Fatal(err)
		}

		srv := NewFolderServer(dataDir)

		// Create the folder
		_, err := srv.CreateFolder(context.Background(), &folderv1.CreateFolderRequest{
			Domain: domain,
			Name:   name,
		})
		if err != nil {
			rt.Fatalf("CreateFolder failed: %v", err)
		}

		// Add some content inside
		if err := os.WriteFile(filepath.Join(domainDir, name, "note.md"), []byte("hello"), 0644); err != nil {
			rt.Fatal(err)
		}
		if err := os.Mkdir(filepath.Join(domainDir, name, "child"), 0755); err != nil {
			rt.Fatal(err)
		}

		// Delete
		resp, err := srv.DeleteFolder(context.Background(), &folderv1.DeleteFolderRequest{
			Domain:     domain,
			FolderPath: name,
		})
		if err != nil {
			rt.Fatalf("DeleteFolder failed: %v", err)
		}

		// Folder must not exist on disk
		if _, err := os.Stat(filepath.Join(domainDir, name)); !os.IsNotExist(err) {
			rt.Fatalf("folder still exists on disk after delete")
		}

		// Folder must not appear in response entries
		for _, e := range resp.Entries {
			if e.Path == name+"/" {
				rt.Fatalf("deleted folder %q still in parent listing", name)
			}
		}
	})
}
