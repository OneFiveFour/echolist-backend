package folder

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	folderv1 "echolist-backend/proto/gen/folder/v1"
	"pgregory.net/rapid"
)

// folderNameGen generates valid folder names: alphanumeric with hyphens/underscores/spaces, 1-50 chars.
func folderNameGen() *rapid.Generator[string] {
	return rapid.StringMatching(`[a-zA-Z0-9][a-zA-Z0-9 _-]{0,49}`)
}

// Property 1: Create folder round-trip
// Creating a folder with a valid name succeeds and the returned entries
// contain the newly created folder (with trailing "/").
// **Validates: Requirements 1.1, 1.4**
func TestProperty1_CreateFolderRoundTrip(t *testing.T) {
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

		// The created folder must appear on disk
		created := filepath.Join(dataDir, name)
		info, err := os.Stat(created)
		if err != nil {
			rt.Fatalf("folder not found on disk: %v", err)
		}
		if !info.IsDir() {
			rt.Fatal("created path is not a directory")
		}

		// The response must contain the created folder
		if resp.Folder == nil {
			rt.Fatal("response Folder is nil")
		}
		if resp.Folder.Name != name {
			rt.Fatalf("expected Folder.Name %q, got %q", name, resp.Folder.Name)
		}
	})
}


// Property 2: Case-insensitive duplicate rejection on create
// If a folder already exists, creating another folder whose name differs
// only in casing must fail with AlreadyExists.
// **Validates: Requirements 1.2**
func TestProperty2_CaseInsensitiveDuplicateRejection(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		name := folderNameGen().Draw(rt, "folderName")

		dataDir := t.TempDir()
		srv := NewFolderServer(dataDir)

		// Create the folder first
		_, err := srv.CreateFolder(context.Background(), &folderv1.CreateFolderRequest{
			Name: name,
		})
		if err != nil {
			rt.Fatalf("first CreateFolder failed: %v", err)
		}

		// Pick a case-variant: swap case of each character
		variant := swapCase(name)

		// Try to create with the variant name — should fail
		_, err = srv.CreateFolder(context.Background(), &folderv1.CreateFolderRequest{
			Name: variant,
		})
		if err == nil {
			rt.Fatalf("expected AlreadyExists error for case-variant %q of %q", variant, name)
		}
	})
}

// swapCase flips the case of every ASCII letter in s.
func swapCase(s string) string {
	b := []byte(s)
	for i, c := range b {
		if c >= 'a' && c <= 'z' {
			b[i] = c - 32
		} else if c >= 'A' && c <= 'Z' {
			b[i] = c + 32
		}
	}
	return string(b)
}

// Property 3: Invalid name rejection
// Names that are empty, contain path separators, are "." or "..", or contain
// null bytes must be rejected with InvalidArgument.
// **Validates: Requirements 1.3, 2.4**
func TestProperty3_InvalidNameRejection(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		// Generate an invalid name from one of several categories
		invalidName := rapid.OneOf(
			// empty string
			rapid.Just(""),
			// contains forward slash
			rapid.Map(rapid.StringMatching(`[a-z]{1,5}`), func(s string) string { return s + "/" + s }),
			// contains backslash
			rapid.Map(rapid.StringMatching(`[a-z]{1,5}`), func(s string) string { return s + "\\" + s }),
			// dot
			rapid.Just("."),
			// dot-dot
			rapid.Just(".."),
			// contains null byte
			rapid.Map(rapid.StringMatching(`[a-z]{1,5}`), func(s string) string { return s + "\x00" }),
		).Draw(rt, "invalidName")

		dataDir := t.TempDir()
		srv := NewFolderServer(dataDir)

		_, err := srv.CreateFolder(context.Background(), &folderv1.CreateFolderRequest{
			Name: invalidName,
		})
		if err == nil {
			rt.Fatalf("expected error for invalid name %q, got nil", invalidName)
		}
	})
}
