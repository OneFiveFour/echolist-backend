package notes

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"connectrpc.com/connect"

	pb "echolist-backend/proto/gen/notes/v1"
	"pgregory.net/rapid"
)

// Feature: task-management, Property 4: Created notes use note_ prefix
// For any valid title and content, after CreateNote, the file on disk must be named note_<title>.md.
// **Validates: Requirements 2.3, 2.4**
func TestProperty4_CreatedNotesUseNotePrefix(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		tmp := t.TempDir()

		title := nameGen().Draw(rt, "title")
		content := rapid.StringMatching(`[a-zA-Z0-9 ]{0,100}`).Draw(rt, "content")

		srv := NewNotesServer(tmp, nopLogger())
		resp, err := srv.CreateNote(context.Background(), &pb.CreateNoteRequest{
			Title:   title,
			Content: content,
		})
		if err != nil {
			rt.Fatalf("CreateNote failed: %v", err)
		}

		// Returned title should be the original (without prefix)
		if resp.Note.Title != title {
			rt.Fatalf("expected title %q, got %q", title, resp.Note.Title)
		}

		// File on disk must be note_<title>.md
		expectedFile := "note_" + title + ".md"
		absPath := filepath.Join(tmp, expectedFile)
		if _, err := os.Stat(absPath); os.IsNotExist(err) {
			rt.Fatalf("expected file %q to exist on disk", expectedFile)
		}

		// FilePath in response should reflect the prefix
		if resp.Note.FilePath != expectedFile {
			rt.Fatalf("expected file_path %q, got %q", expectedFile, resp.Note.FilePath)
		}
	})
}

// titleWithPathSepGen generates a random string that contains at least one `/` or `\`.
// Strategy: generate a base string, pick a random separator, and insert it at a random position.
func titleWithPathSepGen() *rapid.Generator[string] {
	return rapid.Custom(func(rt *rapid.T) string {
		base := rapid.StringMatching(`[a-zA-Z0-9]{0,20}`).Draw(rt, "base")
		sep := rapid.SampledFrom([]string{"/", "\\"}).Draw(rt, "sep")
		pos := rapid.IntRange(0, len(base)).Draw(rt, "pos")
		return base[:pos] + sep + base[pos:]
	})
}

// Feature: api-hardening-cleanup, Property 5: Titles containing path separators are rejected
// For any title string containing `/` or `\` characters, calling CreateNote
// should return a Connect error with code CodeInvalidArgument.
// **Validates: Requirements 5.2**
func TestProperty_TitleWithPathSeparatorsRejected(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		title := titleWithPathSepGen().Draw(rt, "title")
		tmpDir := t.TempDir()
		srv := NewNotesServer(tmpDir, nopLogger())
		ctx := context.Background()

		_, err := srv.CreateNote(ctx, &pb.CreateNoteRequest{
			ParentDir: "",
			Title:   title,
			Content: "test content",
		})

		if err == nil {
			rt.Fatalf("CreateNote: expected error for title %q containing path separator, got nil", title)
		}
		var connErr *connect.Error
		if !errors.As(err, &connErr) {
			rt.Fatalf("CreateNote: expected connect.Error for title %q, got %T: %v", title, err, err)
		}
		if connErr.Code() != connect.CodeInvalidArgument {
			rt.Fatalf("CreateNote: expected CodeInvalidArgument for title %q, got %v", title, connErr.Code())
		}
	})
}


// dirSegmentGen generates a single valid directory name segment (1-10 alphanumeric chars).
func dirSegmentGen() *rapid.Generator[string] {
	return rapid.StringMatching(`[a-zA-Z][a-zA-Z0-9]{0,9}`)
}

// cleanDirPathGen generates a clean relative directory path with 1-3 segments (e.g., "Work/2026").
func cleanDirPathGen() *rapid.Generator[string] {
	return rapid.Custom(func(rt *rapid.T) string {
		n := rapid.IntRange(1, 3).Draw(rt, "numSegments")
		segments := make([]string, n)
		for i := range segments {
			segments[i] = dirSegmentGen().Draw(rt, fmt.Sprintf("seg%d", i))
		}
		return filepath.Join(segments...)
	})
}

// uncleanPathGen takes a clean path and returns an equivalent unclean form by inserting
// redundant "./" or "foo/../" segments at random positions between path components.
func uncleanPathGen(cleanPath string) *rapid.Generator[string] {
	return rapid.Custom(func(rt *rapid.T) string {
		segments := strings.Split(cleanPath, string(filepath.Separator))
		var result []string
		for i, seg := range segments {
			// Optionally insert noise before this segment
			if rapid.Bool().Draw(rt, fmt.Sprintf("insertNoise%d", i)) {
				noiseKind := rapid.IntRange(0, 1).Draw(rt, fmt.Sprintf("noiseKind%d", i))
				if noiseKind == 0 {
					// Insert "./" (current directory reference)
					result = append(result, ".")
				} else {
					// Insert "foo/../" (up-and-back reference)
					junk := dirSegmentGen().Draw(rt, fmt.Sprintf("junk%d", i))
					result = append(result, junk, "..")
				}
			}
			result = append(result, seg)
		}
		return filepath.Join(result...)
	})
}

// Feature: code-review-hardening, Property 1: CreateNote path canonicalization
// For any valid directory path and any equivalent unclean form of that path,
// calling CreateNote with either form should produce a file at the same absolute
// location and return the same relative file path.
// **Validates: Requirements 1.1, 1.3**
// Feature: code-review-hardening, Property 1: CreateNote path canonicalization
// For any valid directory path and any equivalent unclean form of that path,
// calling CreateNote with either form should produce a file at the same absolute
// location and return the same relative file path.
// **Validates: Requirements 1.1, 1.3**
func TestProperty_CreateNotePathCanonicalization(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		tmp := t.TempDir()
		srv := NewNotesServer(tmp, nopLogger())
		ctx := context.Background()

		cleanDir := cleanDirPathGen().Draw(rt, "cleanDir")
		uncleanDir := uncleanPathGen(cleanDir).Draw(rt, "uncleanDir")
		title := nameGen().Draw(rt, "title")
		content := rapid.StringMatching(`[a-zA-Z0-9 ]{0,50}`).Draw(rt, "content")

		// Pre-create the parent directory (CreateNote no longer auto-creates intermediates)
		if err := os.MkdirAll(filepath.Join(tmp, cleanDir), 0755); err != nil {
			rt.Fatalf("failed to pre-create directory: %v", err)
		}

		// Create note with clean path
		respClean, err := srv.CreateNote(ctx, &pb.CreateNoteRequest{
			ParentDir: cleanDir,
			Title:     title,
			Content:   content,
		})
		if err != nil {
			rt.Fatalf("CreateNote with clean path %q failed: %v", cleanDir, err)
		}

		// Record the absolute file location and relative path from the clean call
		cleanRelPath := respClean.Note.FilePath
		cleanAbsPath := filepath.Join(tmp, cleanRelPath)

		// Verify the file exists on disk
		if _, err := os.Stat(cleanAbsPath); err != nil {
			rt.Fatalf("file from clean path does not exist at %q: %v", cleanAbsPath, err)
		}

		// Remove the file so we can create it again with the unclean path
		if err := os.Remove(cleanAbsPath); err != nil {
			rt.Fatalf("failed to remove file %q: %v", cleanAbsPath, err)
		}

		// Create note with unclean path (equivalent to clean path)
		respUnclean, err := srv.CreateNote(ctx, &pb.CreateNoteRequest{
			ParentDir: uncleanDir,
			Title:     title,
			Content:   content,
		})
		if err != nil {
			rt.Fatalf("CreateNote with unclean path %q failed: %v", uncleanDir, err)
		}

		uncleanRelPath := respUnclean.Note.FilePath
		uncleanAbsPath := filepath.Join(tmp, uncleanRelPath)

		// Both should produce the same relative file path
		if cleanRelPath != uncleanRelPath {
			rt.Fatalf("relative paths differ: clean=%q unclean=%q", cleanRelPath, uncleanRelPath)
		}

		// Both should resolve to the same absolute location
		if cleanAbsPath != uncleanAbsPath {
			rt.Fatalf("absolute paths differ: clean=%q unclean=%q", cleanAbsPath, uncleanAbsPath)
		}

		// Verify the file exists at the expected location
		if _, err := os.Stat(uncleanAbsPath); err != nil {
			rt.Fatalf("file from unclean path does not exist at %q: %v", uncleanAbsPath, err)
		}
	})
}


// titleWithNullByteGen generates a random string that contains at least one null byte.
// Strategy: generate a base string and insert \x00 at a random position.
func titleWithNullByteGen() *rapid.Generator[string] {
	return rapid.Custom(func(rt *rapid.T) string {
		base := rapid.StringMatching(`[a-zA-Z0-9]{1,20}`).Draw(rt, "base")
		pos := rapid.IntRange(0, len(base)).Draw(rt, "pos")
		return base[:pos] + "\x00" + base[pos:]
	})
}

// Feature: code-review-hardening, Property 6: Null byte titles are rejected
// For any string containing one or more null bytes, calling CreateNote with that
// string as the title should return a Connect error with code CodeInvalidArgument.
// **Validates: Requirements 5.1**
func TestProperty_NullByteTitlesRejected(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		title := titleWithNullByteGen().Draw(rt, "title")
		tmpDir := t.TempDir()
		srv := NewNotesServer(tmpDir, nopLogger())
		ctx := context.Background()

		_, err := srv.CreateNote(ctx, &pb.CreateNoteRequest{
			ParentDir: "",
			Title:     title,
			Content:   "test content",
		})

		if err == nil {
			rt.Fatalf("CreateNote: expected error for title %q containing null byte, got nil", title)
		}
		var connErr *connect.Error
		if !errors.As(err, &connErr) {
			rt.Fatalf("CreateNote: expected connect.Error for title %q, got %T: %v", title, err, err)
		}
		if connErr.Code() != connect.CodeInvalidArgument {
			rt.Fatalf("CreateNote: expected CodeInvalidArgument for title %q, got %v", title, connErr.Code())
		}
	})
}

// Feature: code-review-hardening, Property 5: CreateNote duplicate detection
// For any valid note title, calling CreateNote twice with the same title in the
// same directory should succeed the first time and return CodeAlreadyExists the
// second time, with the original file content unchanged.
// **Validates: Requirements 4.1, 4.2**
func TestProperty_CreateNoteDuplicateDetection(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		tmp := t.TempDir()
		srv := NewNotesServer(tmp, nopLogger())
		ctx := context.Background()

		title := nameGen().Draw(rt, "title")
		content := rapid.StringMatching(`[a-zA-Z0-9 ]{1,50}`).Draw(rt, "content")

		// First call should succeed
		resp, err := srv.CreateNote(ctx, &pb.CreateNoteRequest{
			ParentDir: "",
			Title:     title,
			Content:   content,
		})
		if err != nil {
			rt.Fatalf("first CreateNote failed: %v", err)
		}

		// Record the file path and read original content from disk
		absPath := filepath.Join(tmp, resp.Note.FilePath)
		originalData, err := os.ReadFile(absPath)
		if err != nil {
			rt.Fatalf("failed to read created file: %v", err)
		}

		// Second call with the same title should return CodeAlreadyExists
		_, err = srv.CreateNote(ctx, &pb.CreateNoteRequest{
			ParentDir: "",
			Title:     title,
			Content:   "different content",
		})
		if err == nil {
			rt.Fatalf("second CreateNote: expected CodeAlreadyExists for title %q, got nil", title)
		}
		var connErr *connect.Error
		if !errors.As(err, &connErr) {
			rt.Fatalf("second CreateNote: expected connect.Error, got %T: %v", err, err)
		}
		if connErr.Code() != connect.CodeAlreadyExists {
			rt.Fatalf("second CreateNote: expected CodeAlreadyExists, got %v", connErr.Code())
		}

		// Verify original file content was not overwritten
		afterData, err := os.ReadFile(absPath)
		if err != nil {
			rt.Fatalf("failed to read file after duplicate attempt: %v", err)
		}
		if string(afterData) != string(originalData) {
			rt.Fatalf("file content changed after duplicate CreateNote: before=%q after=%q", originalData, afterData)
		}
	})
}

