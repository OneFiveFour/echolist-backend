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

func TestProperty3_CreateFolderReturnsCorrectFolder(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		name := folderNameGen().Draw(rt, "folderName")
		dataDir := t.TempDir()
		srv := NewFileServer(dataDir, nopLogger())
		resp, err := srv.CreateFolder(context.Background(), &filev1.CreateFolderRequest{
			ParentDir: "",
			Name:       name,
		})
		if err != nil {
			rt.Fatalf("CreateFolder failed: %v", err)
		}
		if resp.Folder == nil {
			rt.Fatal("response Folder is nil")
		}
		if resp.Folder.Name != name {
			rt.Fatalf("expected Folder.Name %q, got %q", name, resp.Folder.Name)
		}
		expectedPath := name + "/"
		if resp.Folder.Path != expectedPath {
			rt.Fatalf("expected Folder.Path %q, got %q", expectedPath, resp.Folder.Path)
		}
	})
}

func TestProperty5_UpdateFolderReturnsRenamedFolder(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		oldName := folderNameGen().Draw(rt, "oldName")
		newName := folderNameGen().Draw(rt, "newName")
		if oldName == newName {
			rt.Skip("old and new names are equal")
		}
		dataDir := t.TempDir()
		srv := NewFileServer(dataDir, nopLogger())
		_, err := srv.CreateFolder(context.Background(), &filev1.CreateFolderRequest{
			Name: oldName,
		})
		if err != nil {
			rt.Fatalf("CreateFolder failed: %v", err)
		}
		resp, err := srv.UpdateFolder(context.Background(), &filev1.UpdateFolderRequest{
			FolderPath: oldName,
			NewName:    newName,
		})
		if err != nil {
			rt.Fatalf("UpdateFolder failed: %v", err)
		}
		if resp.Folder == nil {
			rt.Fatal("response Folder is nil")
		}
		if resp.Folder.Name != newName {
			rt.Fatalf("expected Folder.Name %q, got %q", newName, resp.Folder.Name)
		}
		expectedPath := newName + "/"
		if resp.Folder.Path != expectedPath {
			rt.Fatalf("expected Folder.Path %q, got %q", expectedPath, resp.Folder.Path)
		}
	})
}

func TestProperty6_ListFilesReturnsImmediateChildren(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		numChildren := rapid.IntRange(0, 5).Draw(rt, "numChildren")
		dataDir := t.TempDir()
		srv := NewFileServer(dataDir, nopLogger())
		created := make(map[string]bool)
		for i := 0; i < numChildren; i++ {
			name := folderNameGen().Draw(rt, "childName")
			if created[name] {
				continue
			}
			created[name] = true
			_, err := srv.CreateFolder(context.Background(), &filev1.CreateFolderRequest{
				Name: name,
			})
			if err != nil {
				rt.Fatalf("CreateFolder %q failed: %v", name, err)
			}
		}
		os.WriteFile(filepath.Join(dataDir, "note_test.md"), []byte("x"), 0644)
		resp, err := srv.ListFiles(context.Background(), &filev1.ListFilesRequest{
			ParentDir: "",
		})
		if err != nil {
			rt.Fatalf("ListFiles failed: %v", err)
		}
		// Count directory entries (those with FOLDER type) — should match created folders
		var dirEntries []string
		for _, e := range resp.Entries {
			if e.ItemType == filev1.ItemType_ITEM_TYPE_FOLDER {
				dirEntries = append(dirEntries, e.Title)
			}
		}
		sort.Strings(dirEntries)
		var expectedNames []string
		for n := range created {
			expectedNames = append(expectedNames, n)
		}
		sort.Strings(expectedNames)
		if len(dirEntries) != len(expectedNames) {
			rt.Fatalf("expected %d directory entries, got %d", len(expectedNames), len(dirEntries))
		}
		for i := range expectedNames {
			if dirEntries[i] != expectedNames[i] {
				rt.Fatalf("mismatch at %d: expected %q, got %q", i, expectedNames[i], dirEntries[i])
			}
		}
		// The "note_test.md" file should also appear as a NOTE entry
		foundFile := false
		for _, e := range resp.Entries {
			if e.Path == "note_test.md" && e.ItemType == filev1.ItemType_ITEM_TYPE_NOTE {
				foundFile = true
				break
			}
		}
		if !foundFile {
			rt.Fatal("expected 'note_test.md' with NOTE type in entries but not found")
		}
	})
}

func TestProperty7_NonExistentFolderReturnsNotFound(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		name := folderNameGen().Draw(rt, "nonExistentName")
		dataDir := t.TempDir()
		srv := NewFileServer(dataDir, nopLogger())
		assertNotFound := func(label string, err error) {
			if err == nil {
				rt.Fatalf("%s: expected error, got nil", label)
			}
			var connErr *connect.Error
			if errors.As(err, &connErr) {
				if connErr.Code() != connect.CodeNotFound {
					rt.Fatalf("%s: expected NotFound, got %v", label, connErr.Code())
				}
			}
		}

		_, err := srv.ListFiles(context.Background(), &filev1.ListFilesRequest{
			ParentDir: name,
		})
		assertNotFound("ListFiles", err)

		_, err = srv.UpdateFolder(context.Background(), &filev1.UpdateFolderRequest{
			FolderPath: name,
			NewName:    "anything",
		})
		assertNotFound("UpdateFolder", err)

		_, err = srv.DeleteFolder(context.Background(), &filev1.DeleteFolderRequest{
			FolderPath: name,
		})
		assertNotFound("DeleteFolder", err)
	})
}

func TestProperty8_UpdateFolderExactConflict(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		nameA := folderNameGen().Draw(rt, "nameA")
		nameB := folderNameGen().Draw(rt, "nameB")
		if nameA == nameB {
			rt.Skip("names are equal")
		}
		dataDir := t.TempDir()
		srv := NewFileServer(dataDir, nopLogger())
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
			rt.Fatalf("expected AlreadyExists error for exact duplicate %q", nameB)
		}
		var connErr *connect.Error
		if errors.As(err, &connErr) {
			if connErr.Code() != connect.CodeAlreadyExists {
				rt.Fatalf("expected AlreadyExists, got %v", connErr.Code())
			}
		}
	})
}
