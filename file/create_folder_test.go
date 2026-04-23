package file

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	filev1 "echolist-backend/proto/gen/file/v1"
	"pgregory.net/rapid"
)

func folderNameGen() *rapid.Generator[string] {
	return rapid.StringMatching(`[a-zA-Z0-9][a-zA-Z0-9 _-]{0,49}`)
}

func TestProperty1_CreateFolderRoundTrip(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		name := folderNameGen().Draw(rt, "folderName")
		dataDir := t.TempDir()
		srv := NewFileServer(dataDir, testDB(t), nopLogger())
		resp, err := srv.CreateFolder(context.Background(), &filev1.CreateFolderRequest{
			ParentDir: "",
			Name:       name,
		})
		if err != nil {
			rt.Fatalf("CreateFolder failed: %v", err)
		}
		created := filepath.Join(dataDir, name)
		info, err := os.Stat(created)
		if err != nil {
			rt.Fatalf("folder not found on disk: %v", err)
		}
		if !info.IsDir() {
			rt.Fatal("created path is not a directory")
		}
		if resp.Folder == nil {
			rt.Fatal("response Folder is nil")
		}
		if resp.Folder.Name != name {
			rt.Fatalf("expected Folder.Name %q, got %q", name, resp.Folder.Name)
		}
	})
}

func TestProperty2_ExactDuplicateRejection(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		name := folderNameGen().Draw(rt, "folderName")
		dataDir := t.TempDir()
		srv := NewFileServer(dataDir, testDB(t), nopLogger())
		_, err := srv.CreateFolder(context.Background(), &filev1.CreateFolderRequest{
			Name: name,
		})
		if err != nil {
			rt.Fatalf("first CreateFolder failed: %v", err)
		}
		// Exact same name should be rejected
		_, err = srv.CreateFolder(context.Background(), &filev1.CreateFolderRequest{
			Name: name,
		})
		if err == nil {
			rt.Fatalf("expected AlreadyExists error for duplicate %q", name)
		}
	})
}

func TestProperty2b_CaseVariantAllowed(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		name := folderNameGen().Draw(rt, "folderName")
		variant := swapCase(name)
		if name == variant {
			rt.Skip("name has no alphabetic characters to swap")
		}
		dataDir := t.TempDir()
		srv := NewFileServer(dataDir, testDB(t), nopLogger())
		_, err := srv.CreateFolder(context.Background(), &filev1.CreateFolderRequest{
			Name: name,
		})
		if err != nil {
			rt.Fatalf("first CreateFolder failed: %v", err)
		}
		// Case-variant should be allowed (case-sensitive)
		_, err = srv.CreateFolder(context.Background(), &filev1.CreateFolderRequest{
			Name: variant,
		})
		if err != nil {
			rt.Fatalf("case-variant %q of %q should be allowed: %v", variant, name, err)
		}
	})
}

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

func TestProperty3_InvalidNameRejection(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		invalidName := rapid.OneOf(
			rapid.Just(""),
			rapid.Map(rapid.StringMatching(`[a-z]{1,5}`), func(s string) string { return s + "/" + s }),
			rapid.Map(rapid.StringMatching(`[a-z]{1,5}`), func(s string) string { return s + "\\" + s }),
			rapid.Just("."),
			rapid.Just(".."),
			rapid.Map(rapid.StringMatching(`[a-z]{1,5}`), func(s string) string { return s + "\x00" }),
		).Draw(rt, "invalidName")
		dataDir := t.TempDir()
		srv := NewFileServer(dataDir, testDB(t), nopLogger())
		_, err := srv.CreateFolder(context.Background(), &filev1.CreateFolderRequest{
			Name: invalidName,
		})
		if err == nil {
			rt.Fatalf("expected error for invalid name %q, got nil", invalidName)
		}
	})
}
