package file

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"sort"
	"testing"

	"connectrpc.com/connect"

	filev1 "echolist-backend/proto/gen/file/v1"
	"pgregory.net/rapid"
)

func TestProperty4_RenamePreservesContents(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		oldName := folderNameGen().Draw(rt, "oldName")
		newName := folderNameGen().Draw(rt, "newName")
		if oldName == newName {
			rt.Skip("old and new names are equal")
		}
		dataDir := t.TempDir()
		srv := NewFileServer(dataDir)
		_, err := srv.CreateFolder(context.Background(), &filev1.CreateFolderRequest{
			Name: oldName,
		})
		if err != nil {
			rt.Fatalf("CreateFolder failed: %v", err)
		}
		childFiles := []string{"a.md", "b.txt", "c.md"}
		childDirs := []string{"sub1", "sub2"}
		for _, f := range childFiles {
			if err := os.WriteFile(filepath.Join(dataDir, oldName, f), []byte("x"), 0644); err != nil {
				rt.Fatal(err)
			}
		}
		for _, d := range childDirs {
			if err := os.Mkdir(filepath.Join(dataDir, oldName, d), 0755); err != nil {
				rt.Fatal(err)
			}
		}
		beforeEntries, _ := os.ReadDir(filepath.Join(dataDir, oldName))
		var beforeNames []string
		for _, e := range beforeEntries {
			beforeNames = append(beforeNames, e.Name())
		}
		sort.Strings(beforeNames)
		_, err = srv.UpdateFolder(context.Background(), &filev1.UpdateFolderRequest{
			FolderPath: oldName,
			NewName:    newName,
		})
		if err != nil {
			rt.Fatalf("UpdateFolder failed: %v", err)
		}
		afterEntries, err := os.ReadDir(filepath.Join(dataDir, newName))
		if err != nil {
			rt.Fatalf("failed to read renamed folder: %v", err)
		}
		var afterNames []string
		for _, e := range afterEntries {
			afterNames = append(afterNames, e.Name())
		}
		sort.Strings(afterNames)
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

func TestProperty5_ExactDuplicateRejectionOnRename(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		nameA := folderNameGen().Draw(rt, "nameA")
		nameB := folderNameGen().Draw(rt, "nameB")
		if nameA == nameB {
			rt.Skip("names are equal")
		}
		dataDir := t.TempDir()
		srv := NewFileServer(dataDir)
		_, err := srv.CreateFolder(context.Background(), &filev1.CreateFolderRequest{
			Name: nameA,
		})
		if err != nil {
			rt.Fatalf("CreateFolder A failed: %v", err)
		}
		_, err = srv.CreateFolder(context.Background(), &filev1.CreateFolderRequest{
			Name: nameB,
		})
		if err != nil {
			rt.Fatalf("CreateFolder B failed: %v", err)
		}
		// Renaming to exact name of sibling should fail
		_, err = srv.UpdateFolder(context.Background(), &filev1.UpdateFolderRequest{
			FolderPath: nameA,
			NewName:    nameB,
		})
		if err == nil {
			rt.Fatalf("expected AlreadyExists error renaming to existing sibling %q", nameB)
		}
		var connErr *connect.Error
		if errors.As(err, &connErr) {
			if connErr.Code() != connect.CodeAlreadyExists {
				rt.Fatalf("expected AlreadyExists, got %v", connErr.Code())
			}
		}
	})
}

func TestProperty5b_RenameToCaseVariantOfSiblingAllowed(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		nameA := folderNameGen().Draw(rt, "nameA")
		nameB := folderNameGen().Draw(rt, "nameB")
		variant := swapCase(nameB)
		if nameA == nameB || nameA == variant || nameB == variant {
			rt.Skip("names collide")
		}
		dataDir := t.TempDir()
		srv := NewFileServer(dataDir)
		_, err := srv.CreateFolder(context.Background(), &filev1.CreateFolderRequest{
			Name: nameA,
		})
		if err != nil {
			rt.Fatalf("CreateFolder A failed: %v", err)
		}
		_, err = srv.CreateFolder(context.Background(), &filev1.CreateFolderRequest{
			Name: nameB,
		})
		if err != nil {
			rt.Fatalf("CreateFolder B failed: %v", err)
		}
		// Renaming to a case-variant of sibling should succeed (case-sensitive)
		_, err = srv.UpdateFolder(context.Background(), &filev1.UpdateFolderRequest{
			FolderPath: nameA,
			NewName:    variant,
		})
		if err != nil {
			rt.Fatalf("rename to case-variant %q of sibling %q should be allowed: %v", variant, nameB, err)
		}
	})
}


func TestProperty6_DeleteRemovesFolderAndContents(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		name := folderNameGen().Draw(rt, "folderName")
		dataDir := t.TempDir()
		srv := NewFileServer(dataDir)
		_, err := srv.CreateFolder(context.Background(), &filev1.CreateFolderRequest{
			Name: name,
		})
		if err != nil {
			rt.Fatalf("CreateFolder failed: %v", err)
		}
		if err := os.WriteFile(filepath.Join(dataDir, name, "note.md"), []byte("hello"), 0644); err != nil {
			rt.Fatal(err)
		}
		if err := os.Mkdir(filepath.Join(dataDir, name, "child"), 0755); err != nil {
			rt.Fatal(err)
		}
		_, err = srv.DeleteFolder(context.Background(), &filev1.DeleteFolderRequest{
			FolderPath: name,
		})
		if err != nil {
			rt.Fatalf("DeleteFolder failed: %v", err)
		}
		if _, err := os.Stat(filepath.Join(dataDir, name)); !os.IsNotExist(err) {
			rt.Fatalf("folder still exists on disk after delete")
		}
	})
}
