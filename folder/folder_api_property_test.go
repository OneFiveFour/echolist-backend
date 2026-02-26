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

// Feature: api-unification, Property 3: CreateFolder returns correct Folder
// For any valid folder name and existing parent path, calling CreateFolder should
// return a Folder whose name matches the requested name and whose path is the
// concatenation of the parent path and the name (with trailing /).
// **Validates: Requirements 4.3, 7.2**
func TestProperty3_CreateFolderReturnsCorrectFolder(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		name := folderNameGen().Draw(rt, "folderName")

		dataDir := t.TempDir()
		srv := NewFolderServer(dataDir)

		resp, err := srv.CreateFolder(context.Background(), &folderv1.CreateFolderRequest{
			ParentPath: "",
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

// Feature: api-unification, Property 4: GetFolder returns correct Folder
// For any existing folder, calling GetFolder with its path should return a Folder
// whose path matches the request path and whose name matches the folder's directory name.
// **Validates: Requirements 6.1, 7.3**
func TestProperty4_GetFolderReturnsCorrectFolder(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		name := folderNameGen().Draw(rt, "folderName")

		dataDir := t.TempDir()
		srv := NewFolderServer(dataDir)

		_, err := srv.CreateFolder(context.Background(), &folderv1.CreateFolderRequest{
			Name: name,
		})
		if err != nil {
			rt.Fatalf("CreateFolder failed: %v", err)
		}

		resp, err := srv.GetFolder(context.Background(), &folderv1.GetFolderRequest{
			FolderPath: name,
		})
		if err != nil {
			rt.Fatalf("GetFolder failed: %v", err)
		}

		if resp.Folder == nil {
			rt.Fatal("response Folder is nil")
		}
		if resp.Folder.Name != name {
			rt.Fatalf("expected Folder.Name %q, got %q", name, resp.Folder.Name)
		}
		if resp.Folder.Path != name {
			rt.Fatalf("expected Folder.Path %q, got %q", name, resp.Folder.Path)
		}
	})
}

// Feature: api-unification, Property 5: UpdateFolder returns renamed Folder
// For any existing folder and any valid new name that does not conflict with siblings,
// calling UpdateFolder should return a Folder whose name matches the new name and
// whose path reflects the renamed location.
// **Validates: Requirements 4.4, 6.3, 7.4**
func TestProperty5_UpdateFolderReturnsRenamedFolder(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		oldName := folderNameGen().Draw(rt, "oldName")
		newName := folderNameGen().Draw(rt, "newName")

		if strings.EqualFold(oldName, newName) {
			rt.Skip("old and new names are case-insensitively equal")
		}

		dataDir := t.TempDir()
		srv := NewFolderServer(dataDir)

		_, err := srv.CreateFolder(context.Background(), &folderv1.CreateFolderRequest{
			Name: oldName,
		})
		if err != nil {
			rt.Fatalf("CreateFolder failed: %v", err)
		}

		resp, err := srv.UpdateFolder(context.Background(), &folderv1.UpdateFolderRequest{
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

// Feature: api-unification, Property 6: ListFolders returns immediate children
// For any parent directory containing a known set of child folders, calling ListFolders
// should return exactly one Folder entry per immediate child directory.
// **Validates: Requirements 6.2, 7.5**
func TestProperty6_ListFoldersReturnsImmediateChildren(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		numChildren := rapid.IntRange(0, 5).Draw(rt, "numChildren")

		dataDir := t.TempDir()
		srv := NewFolderServer(dataDir)

		created := make(map[string]bool)
		for i := 0; i < numChildren; i++ {
			name := folderNameGen().Draw(rt, "childName")
			if created[strings.ToLower(name)] {
				continue
			}
			created[strings.ToLower(name)] = true

			_, err := srv.CreateFolder(context.Background(), &folderv1.CreateFolderRequest{
				Name: name,
			})
			if err != nil {
				rt.Fatalf("CreateFolder %q failed: %v", name, err)
			}
		}

		// Also create a file to ensure it's excluded
		os.WriteFile(filepath.Join(dataDir, "note.md"), []byte("x"), 0644)

		resp, err := srv.ListFolders(context.Background(), &folderv1.ListFoldersRequest{
			ParentPath: "",
		})
		if err != nil {
			rt.Fatalf("ListFolders failed: %v", err)
		}

		if len(resp.Folders) != len(created) {
			rt.Fatalf("expected %d folders, got %d", len(created), len(resp.Folders))
		}

		var gotNames []string
		for _, f := range resp.Folders {
			gotNames = append(gotNames, strings.ToLower(f.Name))
		}
		sort.Strings(gotNames)

		var expectedNames []string
		for n := range created {
			expectedNames = append(expectedNames, n)
		}
		sort.Strings(expectedNames)

		for i := range expectedNames {
			if gotNames[i] != expectedNames[i] {
				rt.Fatalf("mismatch at %d: expected %q, got %q", i, expectedNames[i], gotNames[i])
			}
		}
	})
}

// Feature: api-unification, Property 7: Non-existent folder path returns NotFound
// For any folder path that does not exist, calling GetFolder, ListFolders, UpdateFolder,
// or DeleteFolder should return a Connect error with code NotFound.
// **Validates: Requirements 5.4, 6.4, 6.5, 6.6**
func TestProperty7_NonExistentFolderReturnsNotFound(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		name := folderNameGen().Draw(rt, "nonExistentName")

		dataDir := t.TempDir()
		srv := NewFolderServer(dataDir)

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

		_, err := srv.GetFolder(context.Background(), &folderv1.GetFolderRequest{
			FolderPath: name,
		})
		assertNotFound("GetFolder", err)

		_, err = srv.ListFolders(context.Background(), &folderv1.ListFoldersRequest{
			ParentPath: name,
		})
		assertNotFound("ListFolders", err)

		_, err = srv.UpdateFolder(context.Background(), &folderv1.UpdateFolderRequest{
			FolderPath: name,
			NewName:    "anything",
		})
		assertNotFound("UpdateFolder", err)

		_, err = srv.DeleteFolder(context.Background(), &folderv1.DeleteFolderRequest{
			FolderPath: name,
		})
		assertNotFound("DeleteFolder", err)
	})
}

// Feature: api-unification, Property 8: UpdateFolder case-insensitive sibling conflict returns AlreadyExists
// For any existing folder and any new name that case-insensitively matches an existing
// sibling folder's name, calling UpdateFolder should return AlreadyExists.
// **Validates: Requirements 6.7**
func TestProperty8_UpdateFolderCaseInsensitiveConflict(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		nameA := folderNameGen().Draw(rt, "nameA")
		nameB := folderNameGen().Draw(rt, "nameB")

		if strings.EqualFold(nameA, nameB) {
			rt.Skip("names are case-insensitively equal")
		}

		dataDir := t.TempDir()
		srv := NewFolderServer(dataDir)

		_, err := srv.CreateFolder(context.Background(), &folderv1.CreateFolderRequest{
			Name: nameA,
		})
		if err != nil {
			rt.Fatalf("CreateFolder A failed: %v", err)
		}

		_, err = srv.CreateFolder(context.Background(), &folderv1.CreateFolderRequest{
			Name: nameB,
		})
		if err != nil {
			rt.Fatalf("CreateFolder B failed: %v", err)
		}

		// Try to rename A to a case-variant of B
		variant := swapCase(nameB)
		_, err = srv.UpdateFolder(context.Background(), &folderv1.UpdateFolderRequest{
			FolderPath: nameA,
			NewName:    variant,
		})
		if err == nil {
			rt.Fatalf("expected AlreadyExists error for case-variant %q of %q", variant, nameB)
		}
		var connErr *connect.Error
		if errors.As(err, &connErr) {
			if connErr.Code() != connect.CodeAlreadyExists {
				rt.Fatalf("expected AlreadyExists, got %v", connErr.Code())
			}
		}
	})
}
